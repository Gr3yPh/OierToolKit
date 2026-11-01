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
	"strings"
	"testing"
)

func testContestForRank() contest {
	c := contest{}
	c.Cfg.Name = "测试赛"
	c.Cfg.Players = []contestPlayer{{Name: "甲"}, {Name: "乙"}}
	c.Cfg.Problems = []contestProblem{
		{Name: "A", Tests: []testPoint{{ID: "1", Score: 50}, {ID: "2", Score: 50}}},
		{Name: "B", Tests: []testPoint{{ID: "1", Score: 100}}},
	}
	return c
}

func testResultForRank() contestResult {
	return contestResult{
		Problems: []string{"A", "B"},
		Results: []problemResult{
			{Player: "甲", Problem: "A", Score: 100, CompileOK: true},
			{Player: "甲", Problem: "B", Score: 100, CompileOK: true},
			{Player: "乙", Problem: "A", Score: 50, CompileOK: true},
		},
	}
}

func TestPassRates(t *testing.T) {
	rates := passRates(testContestForRank(), testResultForRank())
	if len(rates) != 2 {
		t.Fatalf("应有 2 道题: %d", len(rates))
	}
	if rates[0].Solved != 1 || rates[0].Tried != 2 || rates[0].Players != 2 {
		t.Fatalf("A 题通过率数据错误: %+v", rates[0])
	}
	if rates[1].Solved != 1 || rates[1].Tried != 1 {
		t.Fatalf("B 题通过率数据错误: %+v", rates[1])
	}
	if rates[0].Rate() != 0.5 {
		t.Fatalf("A 题满分率应为 0.5: %v", rates[0].Rate())
	}
}

func TestRankingCSV(t *testing.T) {
	got := string(rankingCSV(testContestForRank(), testResultForRank()))
	if !strings.HasPrefix(got, "\xEF\xBB\xBF名次,选手,A,B,总分,满分题数\n") {
		t.Fatalf("CSV 表头错误: %q", got)
	}
	// 甲 200 分排第 1，乙 50 分排第 2
	if !strings.Contains(got, "1,甲,100,100,200,2") {
		t.Fatalf("缺少甲的行: %q", got)
	}
	if !strings.Contains(got, "2,乙,50,0,50,0") {
		t.Fatalf("缺少乙的行: %q", got)
	}
}

func TestTr(t *testing.T) {
	if tr("概览") != "概览" {
		t.Fatalf("默认语言不应翻译: %q", tr("概览"))
	}
}

func TestLanguageValueRoundTrip(t *testing.T) {
	for display, value := range map[string]string{"中文": "zh", "English": "en", "跟随系统": "auto"} {
		if languageValueFromChoice(display) != value {
			t.Fatalf("%s -> %s 错误", display, languageValueFromChoice(display))
		}
	}
	for value, display := range map[string]string{"zh": "中文", "en": "English", "auto": "跟随系统"} {
		if languageChoiceFromValue(value) != display {
			t.Fatalf("%s -> %s 错误", value, languageChoiceFromValue(value))
		}
	}
}

func TestFindStatement(t *testing.T) {
	dir := t.TempDir()
	p := project{Dir: dir}
	if findStatement(p) != "" {
		t.Fatal("空目录不应找到题面")
	}
	if err := os.WriteFile(filepath.Join(dir, "problem.md"), []byte("# 题"), 0644); err != nil {
		t.Fatal(err)
	}
	if findStatement(p) == "" {
		t.Fatal("应找到 problem.md")
	}
}
