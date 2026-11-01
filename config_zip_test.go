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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setBaseDir 把配置目录指到 dir，返回恢复函数。
func setBaseDir(t *testing.T, dir string) {
	t.Helper()
	old := baseDir
	baseDir = dir
	t.Cleanup(func() { baseDir = old })
}

// TestConfigExportExcludesTokensByDefault 令牌默认不能进包。
func TestConfigExportExcludesTokensByDefault(t *testing.T) {
	base := t.TempDir()
	setBaseDir(t, base)
	writeIni(globalConfigPath(), map[string]string{
		"editor":       "code -g %s",
		"remote.洛谷":    "https://example.com",
		"token.洛谷":     "SECRET-TOKEN",
		"remote_token": "LEGACY-TOKEN",
	})

	out := filepath.Join(t.TempDir(), "cfg.zip")
	info, err := exportConfigArchive(out, configExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Keys != 2 {
		t.Errorf("导出 %d 项，期望 2 项（editor 与 remote.洛谷）", info.Keys)
	}

	body := readZipEntry(t, out, configEntryName)
	if strings.Contains(body, "SECRET-TOKEN") || strings.Contains(body, "LEGACY-TOKEN") {
		t.Errorf("令牌泄漏进包里了:\n%s", body)
	}
	if !strings.Contains(body, "editor=") {
		t.Errorf("编者配置没进包:\n%s", body)
	}

	// 显式要求时才带上。
	out2 := filepath.Join(t.TempDir(), "cfg-with-token.zip")
	if _, err := exportConfigArchive(out2, configExportOptions{IncludeTokens: true}); err != nil {
		t.Fatal(err)
	}
	if body := readZipEntry(t, out2, configEntryName); !strings.Contains(body, "SECRET-TOKEN") {
		t.Error("勾选后令牌应当被导出")
	}
}

// TestConfigImportMerge 默认只补空缺，覆盖要显式要求。
func TestConfigImportMerge(t *testing.T) {
	src := t.TempDir()
	setBaseDir(t, src)
	writeIni(globalConfigPath(), map[string]string{
		"editor":    "code -g %s",
		"gui_theme": "dark",
	})
	pack := filepath.Join(t.TempDir(), "cfg.zip")
	if _, err := exportConfigArchive(pack, configExportOptions{}); err != nil {
		t.Fatal(err)
	}

	// 换一台机器：本地已经配了自己的编辑器。
	dst := t.TempDir()
	setBaseDir(t, dst)
	writeIni(globalConfigPath(), map[string]string{"editor": "notepad %s"})

	res, err := importConfigArchive(pack, configImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := readIni(globalConfigPath())["editor"]; got != "notepad %s" {
		t.Errorf("默认不该覆盖本地 editor，得到 %q", got)
	}
	if got := readIni(globalConfigPath())["gui_theme"]; got != "dark" {
		t.Errorf("缺失项应当补上，gui_theme = %q", got)
	}
	if len(res.Added) != 1 || len(res.Skipped) != 1 {
		t.Errorf("统计不对: added=%v skipped=%v", res.Added, res.Skipped)
	}

	// 明确要求覆盖时，同名项要更新。
	res, err = importConfigArchive(pack, configImportOptions{Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := readIni(globalConfigPath())["editor"]; got != "code -g %s" {
		t.Errorf("覆盖模式下 editor 应变成包里的值，得到 %q", got)
	}
	if len(res.Updated) != 1 {
		t.Errorf("updated = %v，期望 1 项", res.Updated)
	}
}

// TestConfigImportRegistry 项目库按行去重追加。
func TestConfigImportRegistry(t *testing.T) {
	src := t.TempDir()
	setBaseDir(t, src)
	a, b := filepath.Join(src, "proj-a"), filepath.Join(src, "proj-b")
	registryWrite([]string{a, b})
	writeIni(globalConfigPath(), map[string]string{"editor": "vim %s"})

	pack := filepath.Join(t.TempDir(), "with-registry.zip")
	if _, err := exportConfigArchive(pack, configExportOptions{IncludeRegistry: true}); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	setBaseDir(t, dst)
	registryWrite([]string{a}) // 已有一条相同的

	res, err := importConfigArchive(pack, configImportOptions{IncludeRegistry: true})
	if err != nil {
		t.Fatal(err)
	}
	got := registryRead()
	if len(got) != 2 {
		t.Errorf("导入后应有 2 条（去重），实际 %v", got)
	}
	if res.RegistryAdded != 1 {
		t.Errorf("新增 %d 条，期望 1 条", res.RegistryAdded)
	}
}

// TestConfigArchiveBadInput 坏包与陌生包要报错，而不是静默成功。
func TestConfigArchiveBadInput(t *testing.T) {
	setBaseDir(t, t.TempDir())

	missing := filepath.Join(t.TempDir(), "nope.zip")
	if _, err := importConfigArchive(missing, configImportOptions{}); err == nil {
		t.Error("不存在的包应当报错")
	}

	// 只有个无关文件，没有 config 也没有 registry。
	foreign := filepath.Join(t.TempDir(), "foreign.zip")
	f, err := os.Create(foreign)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("hello"))
	zw.Close()
	f.Close()
	if _, err := importConfigArchive(foreign, configImportOptions{}); err == nil {
		t.Error("没有配置的包应当报错")
	}
}

// TestConfigExportAddsZipSuffix 路径没写 .zip 时自动补。
func TestConfigExportAddsZipSuffix(t *testing.T) {
	setBaseDir(t, t.TempDir())
	writeIni(globalConfigPath(), map[string]string{"editor": "vim %s"})

	out := filepath.Join(t.TempDir(), "cfgwithoutsuffix")
	zpath := out
	if !strings.HasSuffix(zpath, ".zip") {
		zpath += ".zip"
	}
	if _, err := exportConfigArchive(out, configExportOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zpath); err != nil {
		t.Errorf("应当生成 %s: %v", zpath, err)
	}
}

// readZipEntry 读出 zip 里某个条目的文本。
func readZipEntry(t *testing.T, path, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	t.Fatalf("包里没有 %s", name)
	return ""
}
