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

import "testing"

// 评测并发度：默认自动（1~8），设置里写固定值后按固定值走，越界收敛到 32。
func TestContestParallelismPref(t *testing.T) {
	old := baseDir
	baseDir = t.TempDir()
	defer func() { baseDir = old }()

	auto := contestParallelism()
	if auto < 1 || auto > 8 {
		t.Fatalf("自动并发度应在 1~8，得到 %d", auto)
	}

	if err := setGuiPref(prefJudgeParallel, "4"); err != nil {
		t.Fatal(err)
	}
	if got := contestParallelism(); got != 4 {
		t.Fatalf("固定值 4 应原样返回，得到 %d", got)
	}

	if err := setGuiPref(prefJudgeParallel, "999"); err != nil {
		t.Fatal(err)
	}
	if got := contestParallelism(); got != 32 {
		t.Fatalf("越界值应收敛到 32，得到 %d", got)
	}

	if err := setGuiPref(prefJudgeParallel, "0"); err != nil {
		t.Fatal(err)
	}
	if got := contestParallelism(); got != auto {
		t.Fatalf("写回 0 应恢复自动值 %d，得到 %d", auto, got)
	}
}
