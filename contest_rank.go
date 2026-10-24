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
	"sort"
)

// 排名计算：总分降序，同分按"最后得分时间"升序（越早越好），再按姓名。

// rankRow 是榜单中的一行。
type rankRow struct {
	Rank    int
	Player  string
	Scores  []float64 // 与 contestResult.Problems 同序
	Total   float64
	Solved  int // 满分题数
	Missing int // 没有提交的题目数
}

// buildRanking 依据评测结果生成榜单（已排序）。
func buildRanking(c contest, res contestResult) []rankRow {
	problems := res.Problems
	players := make([]string, 0, len(c.Cfg.Players))
	seen := map[string]bool{}
	for _, p := range c.Cfg.Players {
		if !seen[p.Name] {
			seen[p.Name] = true
			players = append(players, p.Name)
		}
	}
	// 结果里出现但玩家表里没有的选手（手改配置造成）也要列出来。
	for _, r := range res.Results {
		if !seen[r.Player] {
			seen[r.Player] = true
			players = append(players, r.Player)
		}
	}

	rows := make([]rankRow, 0, len(players))
	for _, name := range players {
		row := rankRow{Player: name}
		for _, prob := range problems {
			score := res.scoreOf(name, prob)
			row.Scores = append(row.Scores, score)
			row.Total += score
			if full := fullScoreOf(c, prob); full > 0 && score >= full-1e-9 {
				row.Solved++
			}
			if _, ok := res.resultOf(name, prob); !ok {
				row.Missing++
			}
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].Player < rows[j].Player
	})

	// 同分同名次（标准 OI 榜单的并列规则）。
	for i := range rows {
		if i > 0 && rows[i].Total == rows[i-1].Total {
			rows[i].Rank = rows[i-1].Rank
		} else {
			rows[i].Rank = i + 1
		}
	}
	return rows
}

// fullScoreOf 返回一道题的满分。
func fullScoreOf(c contest, problem string) float64 {
	for _, p := range c.Cfg.Problems {
		if p.Name != problem {
			continue
		}
		if len(p.Subtasks) == 0 {
			var sum float64
			for _, t := range p.Tests {
				sum += t.Score
			}
			return sum
		}
		var sum float64
		for _, st := range p.Subtasks {
			sum += st.Score
		}
		return sum
	}
	return 0
}

// totalFullScore 返回整场比赛的总满分。
func totalFullScore(c contest) float64 {
	var sum float64
	for _, p := range c.Cfg.Problems {
		sum += fullScoreOf(c, p.Name)
	}
	return sum
}

// problemOf 按名字取题目配置。
func (c contest) problemOf(name string) (contestProblem, bool) {
	for _, p := range c.Cfg.Problems {
		if p.Name == name {
			return p, true
		}
	}
	return contestProblem{}, false
}

// validateContest 检查配置里容易写错的地方，返回告警文本列表。
func validateContest(c contest) []string {
	var warns []string
	if len(c.Cfg.Problems) == 0 {
		warns = append(warns, "比赛里没有任何题目")
	}
	for _, p := range c.Cfg.Problems {
		testIDs := map[string]bool{}
		for _, t := range p.Tests {
			testIDs[t.ID] = true
		}
		for _, st := range p.Subtasks {
			if len(st.Tests) == 0 {
				warns = append(warns, p.Name+" 的子任务 "+st.Name+" 没有测试点")
			}
			for _, id := range st.Tests {
				if !testIDs[id] {
					warns = append(warns, p.Name+" 的子任务 "+st.Name+" 引用了不存在的测试点 "+id)
				}
			}
		}
		if len(p.Subtasks) == 0 {
			var sum float64
			for _, t := range p.Tests {
				sum += t.Score
			}
			if sum == 0 {
				warns = append(warns, p.Name+" 的测试点总分为 0")
			}
		}
	}
	if len(c.Cfg.Sources) == 0 {
		warns = append(warns, "没有任何选手提交")
	}
	return warns
}
