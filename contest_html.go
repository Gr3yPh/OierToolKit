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
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

// 榜单导出：单文件 HTML，样式内联，双击即可看、可直接发出去或打印。

// exportContestHTML 把榜单写成 <dir>/<比赛名>-排名.html，返回文件路径。
func exportContestHTML(c contest, res contestResult, outPath string) (string, error) {
	rows := buildRanking(c, res)
	full := totalFullScore(c)
	page := renderContestHTML(c, res, rows, full)

	if outPath == "" {
		outPath = filepath.Join(c.Dir, sanitizeFileName(c.Cfg.Name)+"-排名.html")
	}
	if err := os.WriteFile(outPath, []byte(page), 0644); err != nil {
		return "", err
	}
	return outPath, nil
}

// renderContestHTML 生成完整榜单页面。
func renderContestHTML(c contest, res contestResult, rows []rankRow, full float64) string {
	var b strings.Builder

	title := html.EscapeString(c.Cfg.Name)
	b.WriteString("<!DOCTYPE html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s · 排名</title>\n", title)
	b.WriteString("<style>\n" + contestCSS + "</style>\n</head>\n<body>\n")

	b.WriteString("<div class=\"wrap\">\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", title)
	fmt.Fprintf(&b, "<p class=\"meta\">共 %d 位选手 · %d 道题 · 满分 %.0f 分 · 评测于 %s</p>\n",
		len(rows), len(res.Problems), full, html.EscapeString(res.JudgedAt))

	// 榜单表格：名次 / 选手 / 每题 / 总分。
	b.WriteString("<table class=\"board\">\n<thead><tr>")
	b.WriteString("<th class=\"rank\">#</th><th class=\"name\">选手</th>")
	for _, p := range res.Problems {
		fmt.Fprintf(&b, "<th class=\"prob\" title=\"满分 %.0f\">%s</th>", fullScoreOf(c, p), html.EscapeString(p))
	}
	b.WriteString("<th class=\"total\">总分</th></tr></thead>\n<tbody>\n")

	for _, row := range rows {
		cls := "row"
		if row.Rank <= 3 {
			cls += fmt.Sprintf(" top%d", row.Rank)
		}
		fmt.Fprintf(&b, "<tr class=\"%s\">", cls)
		fmt.Fprintf(&b, "<td class=\"rank\">%d</td>", row.Rank)
		fmt.Fprintf(&b, "<td class=\"name\">%s</td>", html.EscapeString(row.Player))
		for i, score := range row.Scores {
			fmt.Fprintf(&b, "<td class=\"score%s\">%s</td>", scoreClass(c, res.Problems[i], score), fmtScore(score))
		}
		fmt.Fprintf(&b, "<td class=\"total\">%s</td>", fmtScore(row.Total))
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n")

	// 每题的子任务明细。
	b.WriteString("<h2>题目明细</h2>\n")
	for _, probName := range res.Problems {
		prob, ok := c.problemOf(probName)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "<h3>%s <span class=\"muted\">满分 %.0f 分 · %d 个测试点</span></h3>\n",
			html.EscapeString(prob.Name), fullScoreOf(c, prob.Name), len(prob.Tests))

		if len(prob.Subtasks) > 0 {
			b.WriteString("<table class=\"sub\">\n<thead><tr><th>子任务</th><th>判定</th><th>分值</th>")
			for _, row := range rows {
				fmt.Fprintf(&b, "<th>%s</th>", html.EscapeString(row.Player))
			}
			b.WriteString("</tr></thead>\n<tbody>\n")
			for _, st := range prob.Subtasks {
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%.0f</td>",
					html.EscapeString(st.Name), html.EscapeString(subtaskModeLabel(st.Mode)), st.Score)
				for _, row := range rows {
					pr, ok := res.resultOf(row.Player, prob.Name)
					awarded := 0.0
					if ok {
						for _, so := range pr.Subtasks {
							if so.Name == st.Name {
								awarded = so.Awarded
							}
						}
					}
					fmt.Fprintf(&b, "<td class=\"%s\">%s</td>", subtaskScoreClass(awarded, st.Score), fmtScore(awarded))
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</tbody>\n</table>\n")
		}

		// 测试点状态矩阵，一眼看出谁挂在哪个点。
		if len(prob.Tests) > 0 {
			b.WriteString("<table class=\"sub compact\">\n<thead><tr><th>测试点</th><th>分值</th>")
			for _, row := range rows {
				fmt.Fprintf(&b, "<th>%s</th>", html.EscapeString(row.Player))
			}
			b.WriteString("</tr></thead>\n<tbody>\n")
			for _, t := range prob.Tests {
				fmt.Fprintf(&b, "<tr><td>%s</td><td>%.0f</td>", html.EscapeString(t.ID), t.Score)
				for _, row := range rows {
					pr, ok := res.resultOf(row.Player, prob.Name)
					verdict := ""
					switch {
					case !ok:
						verdict = "" // 没有提交
					case !pr.CompileOK:
						verdict = "CE" // 编译失败，整题 0 分
					default:
						for _, to := range pr.Tests {
							if to.ID == t.ID {
								verdict = to.Verdict
							}
						}
					}
					if verdict == "" {
						fmt.Fprint(&b, "<td class=\"vc-none\">—</td>")
					} else {
						title := verdict
						if verdict == "CE" && strings.TrimSpace(pr.CompileLog) != "" {
							title = firstLine(pr.CompileLog)
						}
						fmt.Fprintf(&b, "<td class=\"vc-%s\" title=\"%s\">%s</td>",
							strings.ToLower(verdict), html.EscapeString(title), html.EscapeString(verdict))
					}
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</tbody>\n</table>\n")
		}
	}

	b.WriteString("<p class=\"foot\">由 OierToolKit " + html.EscapeString(otkVersion) + " 生成</p>\n")
	b.WriteString("</div>\n</body>\n</html>\n")
	return b.String()
}

// firstLine 提取编译错误摘要：优先第一条含 error: 的行，
// 没有就退回第一条非空内容。
func firstLine(s string) string {
	fallback := ""
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if fallback == "" {
			fallback = t
		}
		if strings.Contains(t, "error:") || strings.Contains(t, "错误：") {
			return t
		}
	}
	return fallback
}

// scoreClass 给分数单元格上色：满分绿、0 分红、其余黄。
func scoreClass(c contest, problem string, score float64) string {
	full := fullScoreOf(c, problem)
	switch {
	case score >= full-1e-9 && full > 0:
		return " full"
	case score <= 1e-9:
		return " zero"
	default:
		return " part"
	}
}

// subtaskScoreClass 给子任务得分上色。
func subtaskScoreClass(awarded, full float64) string {
	switch {
	case full > 0 && awarded >= full-1e-9:
		return "full"
	case awarded <= 1e-9:
		return "zero"
	default:
		return "part"
	}
}

// subtaskModeLabel 返回子任务判定方式的中文说明。
func subtaskModeLabel(mode string) string {
	switch mode {
	case subtaskAny:
		return "任意通过"
	case subtaskSum:
		return "按比例"
	default:
		return "捆绑"
	}
}

// fmtScore 去掉小数末尾多余的 0，整数原样显示。
func fmtScore(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

// contestCSS 是导出页面的样式（深浅色都看得清，打印友好）。
const contestCSS = `
:root{--fg:#1f2328;--muted:#656d76;--line:#d0d7de;--bg:#fff;--panel:#f6f8fa;
--full:#1a7f37;--zero:#cf222e;--part:#9a6700;--accent:#0969da}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);
font:15px/1.5 -apple-system,"Segoe UI","Noto Sans SC","Microsoft YaHei",sans-serif}
.wrap{max-width:1100px;margin:0 auto;padding:32px 20px 60px}
h1{font-size:26px;margin:0 0 6px}
h2{font-size:20px;margin:36px 0 12px;padding-bottom:6px;border-bottom:1px solid var(--line)}
h3{font-size:16px;margin:22px 0 8px}
.meta,.muted,.foot{color:var(--muted);font-size:13px}
.foot{margin-top:40px;text-align:center}
table{width:100%;border-collapse:collapse;font-variant-numeric:tabular-nums;
background:var(--bg);border:1px solid var(--line);border-radius:8px;overflow:hidden}
th,td{padding:8px 10px;text-align:center;border-bottom:1px solid var(--line)}
th{background:var(--panel);font-weight:600;font-size:13px;color:var(--muted)}
tbody tr:last-child td{border-bottom:none}
tbody tr:hover{background:#f6f8fa}
td.name{text-align:left;font-weight:600}
th.name{text-align:left}
th.prob{font-family:ui-monospace,Consolas,monospace}
td.total{font-weight:700}
td.full,.tc-full{color:var(--full);font-weight:600}
td.zero{color:var(--zero)}
td.part{color:var(--part)}
tr.top1 td.rank{color:#bf8700;font-weight:700}
tr.top2 td.rank{color:#6e7781;font-weight:700}
tr.top3 td.rank{color:#a1571a;font-weight:700}
table.sub{margin:6px 0 18px;font-size:13px}
table.compact th,table.compact td{padding:5px 6px}
td.vc-ac{color:var(--full);font-weight:600}
td.vc-wa{color:var(--zero)}
td.vc-ce{color:#8250df;font-weight:600}
td.vc-tle,td.vc-mle,td.vc-re{color:var(--zero);font-weight:600}
td.vc-missing{color:var(--muted)}
td.vc-none{color:#d0d7de}
@media print{
 body{font-size:12px}
 .wrap{max-width:none;padding:0}
 table{page-break-inside:auto}
 tr{page-break-inside:avoid}
 h2{page-break-before:auto}
}
`
