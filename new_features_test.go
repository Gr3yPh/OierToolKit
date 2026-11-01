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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThemePrefRoundTrip 验证主题偏好值与下拉框下标能对上，
// 且 applyThemeEnv 只在明确选了浅色/深色时才动 FYNE_THEME。
func TestThemePrefRoundTrip(t *testing.T) {
	useTempBaseDir(t)

	if len(themeLabels()) != len(themeValues) {
		t.Fatalf("主题选项与取值数量不一致: %d vs %d", len(themeLabels()), len(themeValues))
	}
	for i, v := range themeValues {
		if got := themeIndexFromValue(v); got != i {
			t.Fatalf("%s 应映射到下标 %d，实际 %d", v, i, got)
		}
	}
	// 手改过 ini 的怪值按「跟随系统」处理，不能让下拉框空着。
	if got := themeIndexFromValue("solarized"); got != 0 {
		t.Fatalf("未知值应回到「跟随系统」，实际 %d", got)
	}

	old, had := os.LookupEnv("FYNE_THEME")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("FYNE_THEME", old)
		} else {
			_ = os.Unsetenv("FYNE_THEME")
		}
	})

	// 用户明确选了深色：环境变量必须写进去，Fyne 建 App 时才会用深色。
	_ = os.Unsetenv("FYNE_THEME")
	if err := setGuiPref(prefTheme, "dark"); err != nil {
		t.Fatal(err)
	}
	applyThemeEnv()
	if got := os.Getenv("FYNE_THEME"); got != "dark" {
		t.Fatalf("选深色后 FYNE_THEME=%q", got)
	}

	_ = setGuiPref(prefTheme, "light")
	applyThemeEnv()
	if got := os.Getenv("FYNE_THEME"); got != "light" {
		t.Fatalf("选浅色后 FYNE_THEME=%q", got)
	}

	// 跟随系统：不覆盖外部已经设好的变量，交给 Fyne 自己判断。
	_ = setGuiPref(prefTheme, "auto")
	_ = os.Setenv("FYNE_THEME", "dark")
	applyThemeEnv()
	if got := os.Getenv("FYNE_THEME"); got != "dark" {
		t.Fatalf("跟随系统不应改动已存在的 FYNE_THEME，实际 %q", got)
	}
}

// TestRankingMarkdown 验证 Markdown 榜单的结构与转义。
func TestRankingMarkdown(t *testing.T) {
	c := testContestForRank()
	res := testResultForRank()
	md := rankingMarkdown(c, res)

	want := []string{
		"## 测试赛",
		"| 名次 | 选手 | A | B | 总分 | 满分题数 |",
		"| ---: | :--- | ---: | ---: | ---: | ---: |",
		"| 1 | 甲 | 100 | 100 | 200 | 2 |",
		"| 2 | 乙 | 50 | 0 | 50 | 0 |",
		"满分率：",
		"50.0%",
	}
	for _, w := range want {
		if !strings.Contains(md, w) {
			t.Fatalf("Markdown 缺少 %q:\n%s", w, md)
		}
	}

	// 选手名里的竖线会把表格撑破，必须转义。
	c.Cfg.Players = []contestPlayer{{Name: "甲|乙"}}
	c.Cfg.Sources = nil
	res2 := contestResult{
		Problems: []string{"A"},
		Results:  []problemResult{{Player: "甲|乙", Problem: "A", Score: 100, CompileOK: true}},
	}
	if md2 := rankingMarkdown(c, res2); !strings.Contains(md2, `甲\|乙`) {
		t.Fatalf("竖线未转义:\n%s", md2)
	}
}

// TestExportProjectArchive 验证项目打包带上源码/样例/.otk，且不带编译产物。
func TestExportProjectArchive(t *testing.T) {
	// initProjectAt 会把项目登记进项目库，必须把配置目录指到临时位置，
	// 否则 baseDir 为空时会往当前目录写一个 registry 文件。
	useTempBaseDir(t)
	dir := t.TempDir()
	p, _, _, err := initProjectAt(dir, "打包测试")
	if err != nil {
		t.Fatal(err)
	}
	// 两组样例 + 索引。
	for _, id := range []string{"1", "2"} {
		if err := os.WriteFile(filepath.Join(dir, id+".in"), []byte("1 2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".out"), []byte("3\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeIndex(p, []string{"1", "2"})

	// 编译产物：项目可执行文件与 checker，都不该进包。
	if err := os.WriteFile(p.exePath(), []byte("MZ"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checkerExePath(p), []byte("MZ"), 0644); err != nil {
		t.Fatal(err)
	}
	// 上一次导出的包也不该被套娃打进去。
	if err := os.WriteFile(filepath.Join(dir, "old.zip"), []byte("PK"), 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "proj")
	path, err := exportProjectArchive(p, out)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".zip" {
		t.Fatalf("应自动补 .zip 后缀: %s", path)
	}

	names := zipNameSet(t, path)
	for _, want := range []string{
		p.codeName() + ".cpp",
		"1.in", "1.out", "2.in", "2.out",
		".otk/config", ".otk/samples",
	} {
		if !names[want] {
			t.Fatalf("包里缺少 %s，实际条目: %v", want, names)
		}
	}
	for _, bad := range []string{"old.zip"} {
		if names[bad] {
			t.Fatalf("包里不该有 %s", bad)
		}
	}
	for name := range names {
		if strings.EqualFold(filepath.Ext(name), ".exe") {
			t.Fatalf("编译产物不该进包: %s", name)
		}
	}
}

// TestDuplicateContest 验证比赛复制的命名规则与「不带评测结果」。
func TestDuplicateContest(t *testing.T) {
	useTempBaseDir(t)
	c, err := importContest(makeUploadDir(t), "原赛")
	if err != nil {
		t.Fatal(err)
	}
	// 原比赛里的评测结果与编译产物。
	if err := os.WriteFile(c.resultPath(), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.Dir, ".build"), 0755); err != nil {
		t.Fatal(err)
	}

	dup, err := duplicateContest(c, "")
	if err != nil {
		t.Fatal(err)
	}
	if dup.Cfg.Name != "原赛 "+tr("副本") {
		t.Fatalf("自动命名错误: %q", dup.Cfg.Name)
	}
	if dup.Dir == c.Dir {
		t.Fatal("副本不能覆盖原目录")
	}
	if !fileExists(filepath.Join(dup.Dir, "data", "aplusb", "1.in")) {
		t.Fatal("测试数据没有复制过来")
	}
	if !fileExists(filepath.Join(dup.Dir, "source", "张三", "aplusb.cpp")) {
		t.Fatal("选手源码没有复制过来")
	}
	if fileExists(filepath.Join(dup.Dir, "result.json")) {
		t.Fatal("评测结果不该被复制")
	}
	if dirExists(filepath.Join(dup.Dir, ".build")) {
		t.Fatal("编译产物不该被复制")
	}

	// 再复制一次：自动命名要接着往后排，不能撞车。
	dup2, err := duplicateContest(c, "")
	if err != nil {
		t.Fatal(err)
	}
	if dup2.Cfg.Name != "原赛 "+tr("副本")+" 2" {
		t.Fatalf("第二次自动命名错误: %q", dup2.Cfg.Name)
	}

	// 手填的名字撞了要报错，不能悄悄改名。
	if _, err := duplicateContest(c, "原赛"); err == nil {
		t.Fatal("同名复制应报错")
	}
}

// TestJudgeProjectHeadlessNoSamples 验证批量评测对「没样例」的项目给原因而不是崩。
func TestJudgeProjectHeadlessNoSamples(t *testing.T) {
	useTempBaseDir(t)
	dir := t.TempDir()
	p, _, _, err := initProjectAt(dir, "空项目")
	if err != nil {
		t.Fatal(err)
	}
	res := judgeProjectHeadless(p, nil)
	if res.OK() {
		t.Fatal("没有样例的项目不该算跑完")
	}
	if res.Err != tr("没有登记样例") {
		t.Fatalf("原因不对: %q", res.Err)
	}
	if res.Name != "空项目" {
		t.Fatalf("结果里应带项目名: %q", res.Name)
	}
}

// zipNameSet 读出 zip 里的条目集合，便于做包含判断。
func zipNameSet(t *testing.T, path string) map[string]bool {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]bool{}
	for _, f := range zr.File {
		out[f.Name] = true
	}
	return out
}
