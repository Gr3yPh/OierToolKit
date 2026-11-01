// Copyright (C) 2026 魇珩Gr3yPh4ntom
//
// This file is part of OierToolKit.
//
// OierToolKit is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// OierToolKit is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with OierToolKit.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 配置导出/导入：把 ~/.otkprofile 下的 config（以及可选的 registry）打成
// 一个几十 KB 的 zip，换机器时整个搬过去。
//
// 这是"配置跟着人走"的最省事形态：不用服务器、不用账号、不用凭据，
// 一个文件拷贝就行。真正的云同步（WebDAV / Git）另行讨论过，暂不做。
//
// 两条硬规矩：
//  1. 令牌默认不导出。token.* 是 OJ 登录凭据，明文躺在配置里，跟着 zip
//     传到网盘等于泄露账号。要带必须显式勾选。
//  2. 导入按 key 合并，不整文件覆盖。编辑器路径这类键跟机器绑定，
//     直接覆盖会把新机器上刚配好的值冲掉，而且很难发现。

const (
	configEntryName        = "config"
	registryEntryName      = "registry"
	configManifestName     = "otk-config.json"
	configArchiveMaxBytes  = 8 << 20 // 解压总量上限：配置文件不该有 8MB
	configArchiveMaxItems  = 2000
	configArchiveAppMarker = "OierToolKit"
)

// configArchiveInfo 是包内的清单，记录这份配置从哪台机器、什么时候导出。
type configArchiveInfo struct {
	App          string `json:"app"`
	Version      string `json:"version"`
	ExportedAt   string `json:"exported_at"`
	Host         string `json:"host,omitempty"`
	WithRegistry bool   `json:"with_registry"`
	WithTokens   bool   `json:"with_tokens"`
	Keys         int    `json:"keys"`
}

// configExportOptions 控制导出内容。
type configExportOptions struct {
	IncludeRegistry bool
	IncludeTokens   bool
}

// configImportOptions 控制导入行为。
type configImportOptions struct {
	// Overwrite 为真时用包里的值覆盖本地同名项；为假时只补本地没有的项。
	Overwrite       bool
	IncludeTokens   bool
	IncludeRegistry bool
}

// configImportResult 汇总导入结果，供 GUI 与命令行显示。
type configImportResult struct {
	From          string
	Added         []string
	Updated       []string
	Skipped       []string
	TokensSkipped int
	RegistryAdded int
	RegistryTotal int
}

// isTokenKey 判断一个配置键是不是凭据。
func isTokenKey(k string) bool {
	return k == "remote_token" || strings.HasPrefix(k, "token.")
}

// exportConfigArchive 把全局配置打包到 zipPath；不以 .zip 结尾会自动补。
func exportConfigArchive(zipPath string, opts configExportOptions) (configArchiveInfo, error) {
	props := readIni(globalConfigPath())
	if !opts.IncludeTokens {
		for k := range props {
			if isTokenKey(k) {
				delete(props, k)
			}
		}
	}
	if len(props) == 0 && !opts.IncludeRegistry {
		return configArchiveInfo{}, fmt.Errorf("%s", tr("没有可导出的配置"))
	}

	if !strings.HasSuffix(strings.ToLower(zipPath), ".zip") {
		zipPath += ".zip"
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return configArchiveInfo{}, err
	}

	info := configArchiveInfo{
		App:          configArchiveAppMarker,
		Version:      otkVersion,
		ExportedAt:   time.Now().Format("2006-01-02 15:04:05"),
		WithRegistry: opts.IncludeRegistry,
		WithTokens:   opts.IncludeTokens,
		Keys:         len(props),
	}
	if host, err := os.Hostname(); err == nil {
		info.Host = host
	}

	// 先写 .part 再改名：中途失败不会留下半个包被当成完整备份。
	tmp := zipPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return configArchiveInfo{}, err
	}
	zw := zip.NewWriter(f)
	writeEntry := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, content)
		return err
	}
	if err := writeEntry(configEntryName, iniText(props)); err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return configArchiveInfo{}, err
	}
	if opts.IncludeRegistry {
		paths := registryRead()
		if err := writeEntry(registryEntryName, strings.Join(paths, "\n")); err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return configArchiveInfo{}, err
		}
	}
	manifest, err := json.MarshalIndent(info, "", "  ")
	if err == nil {
		err = writeEntry(configManifestName, string(append(manifest, '\n')))
	}
	if err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return configArchiveInfo{}, err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		return configArchiveInfo{}, err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return configArchiveInfo{}, err
	}
	if err := os.Rename(tmp, zipPath); err != nil {
		os.Remove(tmp)
		return configArchiveInfo{}, err
	}
	return info, nil
}

// importConfigArchive 从 zipPath 读入配置并按 key 合并到本地。
func importConfigArchive(zipPath string, opts configImportOptions) (configImportResult, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return configImportResult{}, fmt.Errorf(tr("打不开数据包: %v"), err)
	}
	defer zr.Close()

	if len(zr.File) > configArchiveMaxItems {
		return configImportResult{}, fmt.Errorf(tr("数据包条目过多（%d 条），不像是配置包"), len(zr.File))
	}

	var configText, registryText, manifestText string
	var total uint64
	for _, f := range zr.File {
		// 配置包里不会有子目录；名字一律先过一遍路径检查，
		// 免得将来加了别的内容时忘了防 zip slip。
		rel, ok := archiveEntryPath(f.Name)
		if !ok {
			return configImportResult{}, fmt.Errorf(tr("数据包里有非法路径: %s"), f.Name)
		}
		total += f.UncompressedSize64
		if total > configArchiveMaxBytes {
			return configImportResult{}, fmt.Errorf("%s", tr("数据包解压后过大，已中止"))
		}
		switch rel {
		case configEntryName, registryEntryName, configManifestName:
		default:
			continue // 包里多出来的东西一律忽略
		}
		rc, err := f.Open()
		if err != nil {
			return configImportResult{}, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, configArchiveMaxBytes+1))
		rc.Close()
		if err != nil {
			return configImportResult{}, err
		}
		if uint64(len(b)) > configArchiveMaxBytes {
			return configImportResult{}, fmt.Errorf("%s", tr("数据包解压后过大，已中止"))
		}
		switch rel {
		case configEntryName:
			configText = string(b)
		case registryEntryName:
			registryText = string(b)
		case configManifestName:
			manifestText = string(b)
		}
	}
	if configText == "" && registryText == "" {
		return configImportResult{}, fmt.Errorf("%s", tr("这个包里没有配置"))
	}

	res := configImportResult{From: zipPath}
	if manifestText != "" {
		var info configArchiveInfo
		if err := json.Unmarshal([]byte(manifestText), &info); err == nil && info.ExportedAt != "" {
			res.From = info.ExportedAt
			if info.Host != "" {
				res.From += " · " + info.Host
			}
		}
	}

	imported := parseIni([]byte(configText))
	if len(imported) > 0 {
		local := readIni(globalConfigPath())
		for k, v := range imported {
			if isTokenKey(k) && !opts.IncludeTokens {
				res.TokensSkipped++
				continue
			}
			cur, exists := local[k]
			switch {
			case exists && cur == v:
				res.Skipped = append(res.Skipped, k)
			case exists && !opts.Overwrite:
				res.Skipped = append(res.Skipped, k)
			case exists:
				local[k] = v
				res.Updated = append(res.Updated, k)
			default:
				local[k] = v
				res.Added = append(res.Added, k)
			}
		}
		writeIni(globalConfigPath(), local)
		sort.Strings(res.Added)
		sort.Strings(res.Updated)
		sort.Strings(res.Skipped)
	}

	if registryText != "" && opts.IncludeRegistry {
		for _, line := range strings.Split(registryText, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			res.RegistryTotal++
			if registryAdd(line) {
				res.RegistryAdded++
			}
		}
	}
	return res, nil
}

// configArchiveSummary 把导入结果拼成一句中文摘要。
func configArchiveSummary(res configImportResult) string {
	parts := []string{
		fmt.Sprintf(tr("新增 %d 项"), len(res.Added)),
		fmt.Sprintf(tr("覆盖 %d 项"), len(res.Updated)),
		fmt.Sprintf(tr("保留本地 %d 项"), len(res.Skipped)),
	}
	if res.TokensSkipped > 0 {
		parts = append(parts, fmt.Sprintf(tr("跳过令牌 %d 项"), res.TokensSkipped))
	}
	if res.RegistryTotal > 0 {
		parts = append(parts, fmt.Sprintf(tr("登记项目 %d 个"), res.RegistryAdded))
	}
	return strings.Join(parts, " · ")
}
