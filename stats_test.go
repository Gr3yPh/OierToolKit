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
	"strings"
	"testing"
	"time"
)

func TestCodeStatsBalanced(t *testing.T) {
	src := "int main() {\n  // 中文注释：分号；不算\n  string s = \"(){[}\";\n  return 0;\n}\n"
	cs := codeStatsOf(src)
	if cs.Lines != 5 || cs.OpenBrace != 0 || cs.OpenParen != 0 || cs.OpenBrack != 0 {
		t.Fatalf("配对统计错误: %+v", cs)
	}
	if cs.FullWidth != "" {
		t.Fatalf("注释里的全角标点不应告警: %q", cs.FullWidth)
	}
	if cs.NonEmpty < 4 {
		t.Fatalf("非空行统计错误: %d", cs.NonEmpty)
	}
}

func TestCodeStatsUnbalancedAndFullWidth(t *testing.T) {
	src := "int main() {\n  printf（\"hi\"）；\n"
	cs := codeStatsOf(src)
	if cs.OpenBrace != 1 {
		t.Fatalf("应报大括号未闭合: %+v", cs)
	}
	if cs.OpenParen != 0 {
		t.Fatalf("全角（不应按圆括号计数: %+v", cs)
	}
	if cs.FullWidth == "" || !strings.Contains(cs.FullWidth, "2") {
		t.Fatalf("应报第 2 行全角标点: %q", cs.FullWidth)
	}
	if len(cs.Warnings) < 2 {
		t.Fatalf("应有至少两条警告: %v", cs.Warnings)
	}
}

func TestHistoryCSV(t *testing.T) {
	entries := []historyEntry{{
		Time: "2026-10-26 12:00:00", Passed: 2, Total: 3,
		TimeLimit: 1, MemLimit: 256,
		Points: []pointResult{{ID: "1", Verdict: "AC", WallMS: 12}},
	}}
	got := string(historyCSV(entries))
	want := "时间,通过,总数,时限(s),内存限制(MB),最慢点(ms)\n2026-10-26 12:00:00,2,3,1.00,256,12\n"
	if got != want {
		t.Fatalf("CSV 不匹配:\n got %q\nwant %q", got, want)
	}
}

func TestTimingSummary(t *testing.T) {
	res := stressResult{Passed: 4, GenMs: 100, BruteMs: 200, ProjMs: 700, SlowRound: 3, SlowRoundMs: 1500}
	got := timingSummary(res)
	if !strings.Contains(got, "0.7s") || !strings.Contains(got, "第 3 轮") ||
		!strings.Contains(got, "1500ms") || !strings.Contains(got, "250ms/轮") {
		t.Fatalf("耗时摘要不正确: %q", got)
	}
}

func TestSecs(t *testing.T) {
	if secs(1500) != 1.5 {
		t.Fatalf("secs(1500) = %v", secs(1500))
	}
	if time.Now().IsZero() { // 保持 time 导入（轮计时不直接单测）
		t.Fatal("unexpected")
	}
}
