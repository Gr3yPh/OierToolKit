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
	"testing"
)

// 比赛列表的筛选与排序：子串匹配（忽略大小写）与四种排序方式。
func TestFilterSortContests(t *testing.T) {
	mk := func(name, at string) contest {
		c := contest{}
		c.Cfg.Name = name
		c.Cfg.ImportedAt = at
		return c
	}
	list := []contest{
		mk("2026 暑假集训 Round 1", "2026-07-01 10:00"),
		mk("CSP-S 模拟", "2026-09-15 14:00"),
		mk("csp-j 模拟", "2026-09-20 09:00"),
		mk("NOIP 复赛模拟", "2026-10-01 08:00"),
	}

	names := func(cs []contest) []string {
		out := make([]string, len(cs))
		for i, c := range cs {
			out[i] = c.Cfg.Name
		}
		return out
	}

	// 无筛选 + 按导入时间新→旧
	got := names(filterSortContests(list, "", 0))
	want := []string{"NOIP 复赛模拟", "csp-j 模拟", "CSP-S 模拟", "2026 暑假集训 Round 1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("时间倒序第 %d 项应为 %q，实际 %q", i, want[i], got[i])
		}
	}

	// 旧→新
	got = names(filterSortContests(list, "", 1))
	if got[0] != "2026 暑假集训 Round 1" || got[len(got)-1] != "NOIP 复赛模拟" {
		t.Fatalf("时间正序不对: %v", got)
	}

	// 名称 A→Z / Z→A
	got = names(filterSortContests(list, "", 2))
	if got[0] != "2026 暑假集训 Round 1" {
		t.Fatalf("名称正序首项不对: %v", got)
	}
	got = names(filterSortContests(list, "", 3))
	if got[0] != "csp-j 模拟" {
		t.Fatalf("名称倒序首项不对: %v", got)
	}

	// 筛选忽略大小写，且能命中两条 csp
	got = names(filterSortContests(list, "  CSP ", 0))
	if len(got) != 2 {
		t.Fatalf("筛选 csp 应命中 2 条，实际 %d: %v", len(got), got)
	}
	// 排序方式不影响筛选结果集
	if len(filterSortContests(list, "csp", 3)) != 2 {
		t.Fatalf("筛选结果不应随排序方式改变")
	}
	// 空结果
	if len(filterSortContests(list, "不存在的比赛", 0)) != 0 {
		t.Fatalf("不匹配时应返回空")
	}
}
