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
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixtureMetas 造三条元数据：甲（新、样例多、时限长）、乙（旧、无样例）、
// 丙（目录缺失）。
func fixtureMetas() []projectMeta {
	t1 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local)
	t2 := time.Date(2026, 10, 27, 21, 30, 0, 0, time.Local)
	return []projectMeta{
		{Dir: `D:\OI\alpha`, Name: "甲", Samples: 8, TimeLimit: "2.00", MemLimit: "512", LastAt: t2, LastScore: "5/5"},
		{Dir: `D:\OI\beta`, Name: "乙", Samples: 0, TimeLimit: "1.00", MemLimit: "125", LastAt: t1, LastScore: "2/5"},
		{Dir: `D:\OI\gone`, Name: "丙", TimeLimit: "-", MemLimit: "-", Missing: true},
	}
}

func namesOf(list []projectMeta) []string {
	out := make([]string, len(list))
	for i, m := range list {
		out[i] = m.Name
	}
	return out
}

// TestSortProjectMetas 覆盖每一列的正序与倒序。
func TestSortProjectMetas(t *testing.T) {
	cases := []struct {
		col  int
		desc bool
		want []string
	}{
		// 中文按 UTF-8 码位比较：丙 < 乙 < 甲。
		{projSortName, false, []string{"丙", "乙", "甲"}},
		{projSortName, true, []string{"甲", "乙", "丙"}},
		{projSortSamples, false, []string{"乙", "丙", "甲"}}, // 乙、丙 都是 0 组样例，稳定排序保持原序
		{projSortSamples, true, []string{"甲", "乙", "丙"}},
		{projSortLimit, false, []string{"丙", "乙", "甲"}},
		{projSortLimit, true, []string{"甲", "乙", "丙"}},
		// 没评测过的永远沉底，两种顺序都不占前排。
		{projSortRecent, false, []string{"乙", "甲", "丙"}},
		{projSortRecent, true, []string{"甲", "乙", "丙"}},
		// 缺失的固定排前面，正序倒序都一样，方便一眼看到该清理的记录。
		{projSortStatus, false, []string{"丙", "乙", "甲"}},
		{projSortStatus, true, []string{"丙", "甲", "乙"}},
	}
	for _, c := range cases {
		list := fixtureMetas()
		sortProjectMetas(list, c.col, c.desc)
		got := namesOf(list)
		if len(got) != len(c.want) {
			t.Fatalf("col=%d desc=%v 长度不对: %v", c.col, c.desc, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("col=%d desc=%v 得到 %v，期望 %v", c.col, c.desc, got, c.want)
				break
			}
		}
	}
}

// TestFilterProjectMetas 按名称或路径过滤，忽略大小写。
func TestFilterProjectMetas(t *testing.T) {
	list := fixtureMetas()
	for _, c := range []struct {
		keyword string
		want    int
	}{
		{"", 3},
		{"甲", 1},
		{"ALPHA", 1},  // 路径大小写不敏感
		{"d:\\oi", 3}, // 全部都在 D:\OI 下
		{"不存在的", 0},
	} {
		got := filterProjectMetas(list, c.keyword)
		if len(got) != c.want {
			t.Errorf("keyword=%q 得到 %d 条，期望 %d", c.keyword, len(got), c.want)
		}
	}
}

// TestScanProjectMeta 从项目目录读出样例数、时限与最近评测。
func TestScanProjectMeta(t *testing.T) {
	useTempBaseDir(t)
	dir := t.TempDir()
	p, _, _, err := initProjectAt(dir, "扫描题")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if err := os.WriteFile(filepath.Join(dir, string(rune('0'+i))+".in"), []byte("1\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, string(rune('0'+i))+".out"), []byte("1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	syncSamples(p, true)

	props := p.cfg()
	props["time_limit"] = "3.00"
	props["memory_limit"] = "256"
	writeIni(p.configPath(), props)

	m := scanProjectMeta(dir)
	if m.Missing {
		t.Fatal("目录存在，不该标记缺失")
	}
	if m.Samples != 2 {
		t.Errorf("样例数 %d，期望 2", m.Samples)
	}
	if m.TimeLimit != "3.00" || m.MemLimit != "256" {
		t.Errorf("时限内存读错: %s / %s", m.TimeLimit, m.MemLimit)
	}
	if !m.LastAt.IsZero() {
		t.Error("还没评测过，最近评测应为空")
	}
	if m.statusText() != tr("未评测") {
		t.Errorf("状态 %q", m.statusText())
	}

	// 缺目录：只保留名字与路径，其余留空并标记缺失。
	gone := scanProjectMeta(filepath.Join(dir, "does-not-exist"))
	if !gone.Missing {
		t.Error("不存在的目录应标记缺失")
	}
	if gone.lastText() != "" || gone.statusText() != tr("目录缺失") {
		t.Errorf("缺失行的文本不对: %q / %q", gone.lastText(), gone.statusText())
	}
}
