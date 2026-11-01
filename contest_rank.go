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
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// rankingCSV 把榜单导出为 CSV 文件（UTF-8，含 BOM 便于 Excel 识别中文）。
// 用 encoding/csv 写出：选手名或题目名里出现逗号、引号、换行时自动加引号
// 转义，Excel 打开才不会错列。
func rankingCSV(c contest, res contestResult) []byte {
	rows := buildRanking(c, res)
	var b strings.Builder
	b.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&b)
	header := []string{tr("名次"), tr("选手")}
	for _, prob := range res.Problems {
		header = append(header, prob)
	}
	header = append(header, tr("总分"), tr("满分题数"))
	_ = w.Write(header)
	for _, row := range rows {
		rec := []string{strconv.Itoa(row.Rank), row.Player}
		for _, s := range row.Scores {
			rec = append(rec, fmtScore(s))
		}
		rec = append(rec, fmtScore(row.Total), strconv.Itoa(row.Solved))
		_ = w.Write(rec)
	}
	w.Flush()
	return []byte(b.String())
}

// rankingMarkdown 把榜单渲染成 Markdown：直接贴进群聊、issue 或文档
// 都能正常显示成表格，比 CSV 更适合人读。
//
// 名次列右对齐、选手列左对齐，分数列一律右对齐——贴到聊天软件里
// 数字对得齐，一眼能看出谁差在哪道题上。
func rankingMarkdown(c contest, res contestResult) string {
	rows := buildRanking(c, res)
	var b strings.Builder

	if name := strings.TrimSpace(c.Cfg.Name); name != "" {
		b.WriteString("## " + name + "\n\n")
	}

	head := []string{tr("名次"), tr("选手")}
	align := []string{"---:", ":---"}
	for _, prob := range res.Problems {
		head = append(head, mdCell(prob))
		align = append(align, "---:")
	}
	head = append(head, tr("总分"), tr("满分题数"))
	align = append(align, "---:", "---:")

	b.WriteString("| " + strings.Join(head, " | ") + " |\n")
	b.WriteString("| " + strings.Join(align, " | ") + " |\n")
	for _, row := range rows {
		cells := []string{strconv.Itoa(row.Rank), mdCell(row.Player)}
		for _, s := range row.Scores {
			cells = append(cells, fmtScore(s))
		}
		cells = append(cells, fmtScore(row.Total), strconv.Itoa(row.Solved))
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}

	// 每题的满分率单独一行：表格里只有分数，看不出"这题挂了多少人"。
	var rates []string
	for _, pr := range passRates(c, res) {
		if pr.Players == 0 {
			continue
		}
		rates = append(rates, fmt.Sprintf("%s %s（%d/%d）",
			mdCell(pr.Problem), fmtPercent(pr.Rate()), pr.Solved, pr.Players))
	}
	if len(rates) > 0 {
		b.WriteString("\n" + tr("满分率：") + strings.Join(rates, " · ") + "\n")
	}
	return b.String()
}

// mdCell 转义 Markdown 表格里的竖线与换行，避免选手名把表格撑破。
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// fmtPercent 把 0~1 的比例写成一位小数的百分比。
func fmtPercent(v float64) string {
	return strconv.FormatFloat(v*100, 'f', 1, 64) + "%"
}

// passRate 是一道题的通过情况。
type passRate struct {
	Problem string
	Solved  int // 满分人数
	Tried   int // 至少提交过一次的人数
	Players int // 总人数
}

// Rate 满分率（0~1）。
func (pr passRate) Rate() float64 {
	if pr.Players == 0 {
		return 0
	}
	return float64(pr.Solved) / float64(pr.Players)
}

// passRates 统计每道题的满分通过率，与 res.Problems 同序。
func passRates(c contest, res contestResult) []passRate {
	players := map[string]bool{}
	for _, r := range res.Results {
		players[r.Player] = true
	}
	out := make([]passRate, 0, len(res.Problems))
	for _, prob := range res.Problems {
		pr := passRate{Problem: prob, Players: len(players)}
		for _, r := range res.Results {
			if r.Problem != prob {
				continue
			}
			pr.Tried++
			if full := fullScoreOf(c, prob); full > 0 && r.Score >= full-1e-9 {
				pr.Solved++
			}
		}
		out = append(out, pr)
	}
	return out
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
		warns = append(warns, tr("比赛里没有任何题目"))
	}
	for _, p := range c.Cfg.Problems {
		testIDs := map[string]bool{}
		for _, t := range p.Tests {
			testIDs[t.ID] = true
		}
		for _, st := range p.Subtasks {
			if len(st.Tests) == 0 {
				warns = append(warns, p.Name+tr("的子任务")+" "+st.Name+" "+tr("没有测试点"))
			}
			for _, id := range st.Tests {
				if !testIDs[id] {
					warns = append(warns, p.Name+tr("的子任务")+" "+st.Name+" "+tr("引用了不存在的测试点")+" "+id)
				}
			}
		}
		if len(p.Subtasks) == 0 {
			var sum float64
			for _, t := range p.Tests {
				sum += t.Score
			}
			if sum == 0 {
				warns = append(warns, p.Name+tr(" 的测试点总分为 0"))
			}
		}
	}
	if len(c.Cfg.Sources) == 0 {
		warns = append(warns, tr("没有任何选手提交"))
	}
	return warns
}
