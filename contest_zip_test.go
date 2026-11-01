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

// useTempBaseDir 把配置目录指到临时目录，测试结束后恢复。
func useTempBaseDir(t *testing.T) string {
	t.Helper()
	old := baseDir
	dir := t.TempDir()
	baseDir = dir
	t.Cleanup(func() { baseDir = old })
	return dir
}

// makeUploadDir 造一个最小上传目录：1 道题、1 位选手。
func makeUploadDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data", "aplusb")
	if err := os.MkdirAll(data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "1.in"), []byte("1 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "1.ans"), []byte("3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "source", "张三")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "aplusb.cpp"), []byte("int main(){return 0;}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeZip 按条目表造一个 zip。
func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// zipNames 列出包内条目名。
func zipNames(t *testing.T, path string) []string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// TestContestArchiveRoundTrip 导出再导入，配置与文件都要原样回来。
func TestContestArchiveRoundTrip(t *testing.T) {
	useTempBaseDir(t)

	c, err := importContest(makeUploadDir(t), "往返赛")
	if err != nil {
		t.Fatalf("importContest: %v", err)
	}
	// 手改一些"只有配置里才有"的信息，验证它们跟着包走。
	p := &c.Cfg.Problems[0]
	p.TimeMS = 2500
	p.MemMB = 512
	p.Title = "A+B Problem"
	p.Statement = "# 题面"
	p.Subtasks = []contestSubtask{{Name: "Sub", Score: 100, Mode: subtaskAll, Tests: []string{"1"}}}
	p.Tests[0].Score = 100
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	// result.json 与 .build 都不该进包。
	if err := os.WriteFile(c.resultPath(), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.Dir, ".build"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Dir, ".build", "a.out"), []byte("bin"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgPath, err := exportContestArchive(c, filepath.Join(t.TempDir(), "pack"))
	if err != nil {
		t.Fatalf("exportContestArchive: %v", err)
	}
	if filepath.Ext(pkgPath) != ".zip" {
		t.Fatalf("应自动补 .zip 后缀: %s", pkgPath)
	}
	for _, name := range zipNames(t, pkgPath) {
		if name == "result.json" || strings.HasPrefix(name, ".build/") {
			t.Fatalf("不该打包 %s", name)
		}
	}

	// 换一个空的配置目录，相当于另一台机器。
	useTempBaseDir(t)
	got, err := importContestArchive(pkgPath, "")
	if err != nil {
		t.Fatalf("importContestArchive: %v", err)
	}
	if got.Cfg.Name != "往返赛" {
		t.Fatalf("名字应取自 contest.json: %q", got.Cfg.Name)
	}
	gp := got.Cfg.Problems[0]
	if gp.TimeMS != 2500 || gp.MemMB != 512 || gp.Title != "A+B Problem" || gp.Statement != "# 题面" {
		t.Fatalf("题目配置没还原: %+v", gp)
	}
	if len(gp.Subtasks) != 1 || gp.Subtasks[0].Mode != subtaskAll || gp.Tests[0].Score != 100 {
		t.Fatalf("子任务与分值没还原: %+v", gp)
	}
	if len(got.Cfg.Players) != 1 || got.Cfg.Players[0].Name != "张三" {
		t.Fatalf("选手没还原: %+v", got.Cfg.Players)
	}
	in := filepath.Join(got.dataDir(), "aplusb", "1.in")
	if body, err := os.ReadFile(in); err != nil || string(body) != "1 2\n" {
		t.Fatalf("测试点没还原: %q %v", body, err)
	}
	if !fileExists(filepath.Join(got.sourceDir(), "张三", "aplusb.cpp")) {
		t.Fatal("源码没还原")
	}
	if fileExists(got.resultPath()) {
		t.Fatal("评测结果不该被带过来")
	}
}

// TestContestArchiveWrappedRoot 兼容「整包被包进一个顶层目录」的打包方式。
func TestContestArchiveWrappedRoot(t *testing.T) {
	useTempBaseDir(t)
	pkg := filepath.Join(t.TempDir(), "wrapped.zip")
	writeZip(t, pkg, map[string]string{
		"__MACOSX/._x": "junk",
		"pkg/contest.json": `{"name":"包了一层","problems":[{"name":"aplusb","time_ms":1500,` +
			`"mem_mb":128,"tests":[{"id":"1","subtask":-1,"score":100}]}],` +
			`"players":[{"name":"李四"}],"sources":[{"player":"李四","problem":"aplusb","path":"source/李四/aplusb.cpp"}]}`,
		"pkg/data/aplusb/1.in":     "1 2\n",
		"pkg/data/aplusb/1.ans":    "3\n",
		"pkg/source/李四/aplusb.cpp": "int main(){return 0;}\n",
		"pkg/readme.txt":           "ignore me",
	})

	c, err := importContestArchive(pkg, "")
	if err != nil {
		t.Fatalf("importContestArchive: %v", err)
	}
	if c.Cfg.Name != "包了一层" {
		t.Fatalf("名字错误: %q", c.Cfg.Name)
	}
	if c.Cfg.Problems[0].TimeMS != 1500 {
		t.Fatalf("时限没还原: %d", c.Cfg.Problems[0].TimeMS)
	}
	if !fileExists(filepath.Join(c.sourceDir(), "李四", "aplusb.cpp")) {
		t.Fatal("源码应在 source/李四/ 下")
	}
	if fileExists(filepath.Join(c.Dir, "readme.txt")) || dirExists(filepath.Join(c.Dir, "__MACOSX")) {
		t.Fatal("无关文件不该解出来")
	}
}

// TestContestArchiveWithoutConfig 没有 contest.json 时按上传目录重新扫描。
func TestContestArchiveWithoutConfig(t *testing.T) {
	useTempBaseDir(t)
	pkg := filepath.Join(t.TempDir(), "plain.zip")
	writeZip(t, pkg, map[string]string{
		"data/aplusb/1.in":     "1 2\n",
		"data/aplusb/1.ans":    "3\n",
		"source/王五/aplusb.cpp": "int main(){return 0;}\n",
	})

	c, err := importContestArchive(pkg, "")
	if err != nil {
		t.Fatalf("importContestArchive: %v", err)
	}
	if c.Cfg.Name != "plain" {
		t.Fatalf("名字应取包名: %q", c.Cfg.Name)
	}
	if len(c.Cfg.Problems) != 1 || len(c.Cfg.Problems[0].Tests) != 1 {
		t.Fatalf("应扫出 1 题 1 点: %+v", c.Cfg.Problems)
	}
	if len(c.Cfg.Sources) != 1 {
		t.Fatalf("应扫出 1 份提交: %+v", c.Cfg.Sources)
	}
	if c.Cfg.SourceDir != pkg {
		t.Fatalf("来源应记为数据包路径: %q", c.Cfg.SourceDir)
	}
}

// TestContestArchiveRejectsSlip 带 .. 的条目必须被拒绝，不能写到目录外面。
func TestContestArchiveRejectsSlip(t *testing.T) {
	useTempBaseDir(t)
	dir := t.TempDir()
	pkg := filepath.Join(dir, "evil.zip")
	writeZip(t, pkg, map[string]string{
		"contest.json":      `{"name":"滑出去"}`,
		"data/aplusb/1.in":  "1\n",
		"data/aplusb/1.ans": "2\n",
		"../../escaped.txt": "pwned",
		"source/张三/a.cpp":   "int main(){}\n",
	})

	if _, err := importContestArchive(pkg, ""); err == nil {
		t.Fatal("越界路径应当报错")
	}
	escaped := filepath.Join(dir, "..", "..", "escaped.txt")
	if fileExists(escaped) {
		t.Fatalf("文件被写到了包外: %s", escaped)
	}
	if dirExists(filepath.Join(contestsDir(), "滑出去")) {
		t.Fatal("失败的导入不该留下比赛目录")
	}
}

// TestContestArchiveDuplicate 同名比赛已存在时报错，不覆盖。
func TestContestArchiveDuplicate(t *testing.T) {
	useTempBaseDir(t)
	c, err := importContest(makeUploadDir(t), "重名赛")
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := exportContestArchive(c, filepath.Join(t.TempDir(), "dup.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importContestArchive(pkg, ""); err == nil {
		t.Fatal("同名比赛应报错")
	}
	if _, err := importContestArchive(pkg, "改了名"); err != nil {
		t.Fatalf("改名后应能导入: %v", err)
	}
}

// TestArchiveEntryPath 单元测试路径规范化。
func TestArchiveEntryPath(t *testing.T) {
	ok := map[string]string{
		"data/a/1.in":     "data/a/1.in",
		"./data/a/1.in":   "data/a/1.in",
		"pkg\\data\\1.in": "pkg/data/1.in",
	}
	for in, want := range ok {
		got, valid := archiveEntryPath(in)
		if !valid || got != want {
			t.Fatalf("%q -> %q(%v)，应为 %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"../x", "a/../../x", "/etc/passwd", "C:/Windows/x", "..\\..\\x"} {
		if _, valid := archiveEntryPath(in); valid {
			t.Fatalf("%q 应被拒绝", in)
		}
	}
}
