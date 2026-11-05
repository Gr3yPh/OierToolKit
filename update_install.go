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
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const updateRepo = "Gr3yPh/OierToolKit"

// updatePackage 是下载并解压后的发行包；临时目录在安装完成前不能删。
type updatePackage struct {
	dir   string
	files map[string]string // 文件名（otk / otk.exe ...）→ 临时路径
}

func (p *updatePackage) cleanup() {
	if p != nil && p.dir != "" {
		_ = os.RemoveAll(p.dir)
	}
}

// updateAssetName 返回当前平台对应的 release 包名。
func updateAssetName() (string, bool) {
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "linux", "darwin":
		if arch != "amd64" && arch != "arm64" {
			return "", false
		}
		return fmt.Sprintf("otk-%s-%s.tar.gz", runtime.GOOS, arch), true
	case "windows":
		// 发布侧只有 windows/amd64；ARM64 Windows 走 x64 模拟层。
		return "otk-windows-amd64.zip", true
	default:
		return "", false
	}
}

func updateAssetURLs(info updateInfo, asset string) []string {
	urls := []string{"https://otk.gr3yph4ntom.cn/releases/" + asset}
	if info.Version != "" {
		urls = append(urls, "https://github.com/"+updateRepo+"/releases/download/"+info.Version+"/"+asset)
	}
	return urls
}

// currentUpdateExecutable 返回当前正在运行的、可更新的程序路径。
// go run / 临时构建出来的 otk 不参与更新，避免把临时目录当成安装目录。
func currentUpdateExecutable() (exe, base, dir string, err error) {
	exe, err = os.Executable()
	if err != nil {
		return "", "", "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
		exe = resolved
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", "", "", err
	}
	base = filepath.Base(exe)
	dir = filepath.Dir(exe)

	if !isUpdateBinaryName(base) {
		return "", "", "", fmt.Errorf("%s", tr("当前程序不是发布版入口，无法在软件内更新。"))
	}
	tempRoot := filepath.Clean(os.TempDir()) + string(os.PathSeparator)
	if strings.HasPrefix(exe, tempRoot) || strings.Contains(exe, string(os.PathSeparator)+"go-build"+string(os.PathSeparator)) {
		return "", "", "", fmt.Errorf("%s", tr("当前运行的是开发构建，无法在软件内更新，请使用发布版。"))
	}
	return exe, base, dir, nil
}

func isUpdateBinaryName(name string) bool {
	switch strings.ToLower(name) {
	case "otk", "otk-gui", "otk.exe", "otk-gui.exe":
		return true
	default:
		return false
	}
}

// downloadAndExtractUpdate 从镜像优先下载当前平台包，并解压到临时目录。
func downloadAndExtractUpdate(info updateInfo, progress func(done, total int64)) (*updatePackage, error) {
	asset, ok := updateAssetName()
	if !ok {
		return nil, fmt.Errorf("%s", tr("当前平台没有可用的更新包。"))
	}
	dir, err := os.MkdirTemp("", "otk-update-*")
	if err != nil {
		return nil, err
	}
	pkg := &updatePackage{dir: dir}
	fail := func(err error) (*updatePackage, error) {
		pkg.cleanup()
		return nil, err
	}

	archive := filepath.Join(dir, asset)
	if err := downloadUpdateArchive(updateAssetURLs(info, asset), archive, progress); err != nil {
		return fail(err)
	}
	files, err := extractUpdateArchive(archive, dir)
	if err != nil {
		return fail(err)
	}
	pkg.files = files
	return pkg, nil
}

func downloadUpdateArchive(urls []string, dst string, progress func(done, total int64)) error {
	var lastErr error
	for _, url := range urls {
		if err := downloadOneUpdate(url, dst, progress); err == nil {
			return nil
		} else {
			lastErr = err
			_ = os.Remove(dst)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s", tr("没有可用的下载源。"))
	}
	return lastErr
}

func downloadOneUpdate(url, dst string, progress func(done, total int64)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "OierToolKit/"+otkVersion)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	reader := &updateProgressReader{
		reader:   resp.Body,
		total:    resp.ContentLength,
		progress: progress,
	}
	if _, err := io.Copy(out, reader); err != nil {
		return err
	}
	return out.Close()
}

type updateProgressReader struct {
	reader   io.Reader
	total    int64
	done     int64
	progress func(done, total int64)
}

func (r *updateProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.done += int64(n)
		if r.progress != nil {
			r.progress(r.done, r.total)
		}
	}
	return n, err
}

func extractUpdateArchive(archive, dest string) (map[string]string, error) {
	if strings.HasSuffix(strings.ToLower(archive), ".zip") {
		return extractUpdateZip(archive, dest)
	}
	return extractUpdateTarGz(archive, dest)
}

func extractUpdateTarGz(archive, dest string) (map[string]string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	out := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(h.Name)
		if !isUpdateBinaryName(name) || !h.FileInfo().Mode().IsRegular() {
			continue
		}
		if err := writeUpdateBinary(dest, name, tr); err != nil {
			return nil, err
		}
		out[name] = filepath.Join(dest, name)
	}
	return out, nil
}

func extractUpdateZip(archive, dest string) (map[string]string, error) {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	out := map[string]string{}
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !isUpdateBinaryName(name) || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		err = writeUpdateBinary(dest, name, rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		out[name] = filepath.Join(dest, name)
	}
	return out, nil
}

func writeUpdateBinary(dest, name string, r io.Reader) error {
	path := filepath.Join(dest, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// shellQuote 把路径安全地放进 /bin/sh 脚本。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// installUpdatePackage 替换程序文件并启动新进程。Windows 会交给延时批处理。
func installUpdatePackage(pkg *updatePackage) error {
	_, base, dir, err := currentUpdateExecutable()
	if err != nil {
		return err
	}
	if _, ok := pkg.files[base]; !ok {
		return fmt.Errorf(tr("更新包缺少当前入口 %s。"), base)
	}
	if err := finishUpdateInstall(dir, base, pkg.dir, pkg.files); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		pkg.cleanup()
	}
	return nil
}
