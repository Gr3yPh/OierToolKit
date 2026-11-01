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
	"image/color"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// verdictIdle 是等待判题的中性色。这里必须问主题要，不能写死：
// 深色主题下浅色版的中性灰会偏亮、浅色主题下深色版的会偏暗。
func verdictIdle() color.Color {
	return theme.Color(theme.ColorNameDisabled)
}

// judgePage 是一键评测页：编译、逐点运行、展示判定、得分与差异对比。
type judgePage struct {
	pg      *projectPage
	content fyne.CanvasObject

	runBtn    *widget.Button
	stopBtn   *widget.Button
	retestBtn *widget.Button
	histBtn   *widget.Button
	status    *widget.Label
	list      *widget.List
	detail    *widget.Label
	score     *widget.Label
	note      *widget.Label
	target    *widget.Label
	bar       *widget.ProgressBar

	points     []pointResult
	limit      float64
	memCap     float64
	selectedID string

	running atomic.Bool
	cancel  atomic.Bool
}

func newJudgePage(pg *projectPage) *judgePage {
	j := &judgePage{pg: pg}

	j.status = caption(tr("点“编译并评测”开始，默认跟随项目配置的时限与内存。"))
	j.detail = hint(tr("选择左侧测试点查看详情。"))
	j.detail.Wrapping = fyne.TextWrapWord

	// 评分用大号等宽数字，一眼能看到。
	j.score = widget.NewLabel("—")
	j.score.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	j.score.SizeName = theme.SizeNameHeadingText
	j.note = caption(tr("尚未评测"))
	j.target = hint("")
	j.bar = widget.NewProgressBar()
	j.bar.Hide()

	j.runBtn = widget.NewButtonWithIcon(tr("编译并评测"), theme.MediaPlayIcon(), j.run)
	j.runBtn.Importance = widget.HighImportance
	j.stopBtn = widget.NewButtonWithIcon(tr("停止"), theme.MediaStopIcon(), j.stop)
	j.stopBtn.Disable()
	j.retestBtn = widget.NewButtonWithIcon(tr("重测此点"), theme.ViewRefreshIcon(), j.retest)
	j.retestBtn.Disable()
	j.histBtn = widget.NewButtonWithIcon(tr("历史"), theme.HistoryIcon(), j.showHistory)

	j.list = widget.NewList(
		func() int { return len(j.points) },
		func() fyne.CanvasObject { return newPointRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(j.points) {
				return
			}
			obj.(*pointRow).update(j.points[id], j.limit)
		},
	)
	j.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(j.points) {
			return
		}
		j.selectedID = j.points[id].ID
		j.retestBtn.Enable()
		j.showDetail(j.points[id])
	}

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			container.NewHBox(j.runBtn, j.stopBtn, j.retestBtn, j.histBtn),
			nil,
			j.status,
		)),
		container.NewPadded(j.target),
		container.NewPadded(j.bar),
		widget.NewSeparator(),
	)

	left := container.NewBorder(j.buildScoreCard(), nil, nil, nil, j.list)

	body := container.NewHSplit(left, container.NewVScroll(container.NewPadded(j.detail)))
	body.SetOffset(0.36)

	j.content = container.NewBorder(head, nil, nil, nil, body)
	j.reload()
	return j
}

// buildScoreCard 是评测页左下角的评分卡片：大号分数 + 一句话结论。
func (j *judgePage) buildScoreCard() fyne.CanvasObject {
	title := caption(tr("评分"))
	right := container.NewVBox(j.score, j.note)
	card := container.NewVBox(
		container.NewBorder(nil, nil, title, nil, right),
		widget.NewSeparator(),
	)
	return card
}

// run 在后台编译并评测，逐点回写界面。
func (j *judgePage) run() {
	if j.running.Load() {
		return
	}
	p := j.pg.p
	srcPath, exePath, name := p.sourcePath(), p.exePath(), p.codeName()

	j.running.Store(true)
	j.cancel.Store(false)
	j.points = nil
	j.list.UnselectAll()
	j.list.Refresh()
	j.detail.SetText(tr("正在编译 ..."))
	j.score.SetText("—")
	j.note.SetText(tr("评测中 ..."))
	j.status.SetText(fmt.Sprintf(tr("编译 %s.cpp ..."), name))
	j.runBtn.Disable()
	j.stopBtn.Enable()
	j.target.SetText(j.targetText(p))

	gen := j.pg.gen
	go func() {
		props := p.cfg()
		limit, memCap := judgeLimits(props)
		threads := judgeThreads(props)
		// limit 会被 UI 线程的列表模板与详情面板读取，写入必须回 UI 线程。
		ui(func() {
			if j.alive(gen) {
				j.setLimits(limit, memCap)
			}
		})

		log, ok := compileProjectTo(srcPath, exePath, props)
		if !ok {
			ui(func() {
				if !j.alive(gen) {
					return
				}
				j.running.Store(false)
				j.runBtn.Enable()
				j.stopBtn.Disable()
				j.bar.Hide()
				j.note.SetText(tr("编译失败"))
				j.status.SetText("[CE] " + tr("编译失败"))
				j.detail.SetText(log)
				j.pg.g.busy(false, tr("编译失败"))
			})
			return
		}
		warnings := parseWarnings(log)

		checkerExe, ceLog, checkerOK := prepareChecker(p, props)
		if !checkerOK {
			ui(func() {
				if !j.alive(gen) {
					return
				}
				j.running.Store(false)
				j.runBtn.Enable()
				j.stopBtn.Disable()
				j.bar.Hide()
				j.note.SetText(tr("checker 编译失败"))
				j.status.SetText("[CE] " + tr("checker 编译失败"))
				j.detail.SetText(ceLog)
				j.pg.g.busy(false, tr("checker 编译失败"))
			})
			return
		}

		ui(func() {
			if !j.alive(gen) {
				return
			}
			j.status.SetText(fmt.Sprintf(tr("正在评测 %s（%d 线程）"), name, threads))
			j.detail.SetText(tr("正在运行测试点 ..."))
			j.bar.SetValue(0)
			j.bar.Show()
			j.pg.g.busy(true, tr("评测中 ..."))
		})

		index := readIndex(p)
		job := judgeJob{
			ExePath:   exePath,
			Dir:       p.Dir,
			Index:     index,
			Threads:   threads,
			TimeLimit: limit,
			MemLimit:  memCap,
			FileIO:    fileIOFromProps(p.cfg(), p.codeName()),
			Checker:   checkerExe,
		}
		total := len(index)

		// 并发回调来自多个 goroutine：计数用原子量，j.points 的追加也放到
		// ui() 里由主线程串行执行——列表模板随时可能在读它，后台写是竞争。
		var doneCount int64
		rep := evalSamples(job, func(pt pointResult) {
			done := atomic.AddInt64(&doneCount, 1)
			ui(func() {
				if !j.alive(gen) {
					return
				}
				j.limit = limit
				j.points = append(j.points, pt)
				j.list.Refresh()
				if total > 0 {
					j.bar.SetValue(float64(done) / float64(total))
				}
				j.refreshScore()
			})
		}, j.cancel.Load)
		rep.Warnings = warnings

		// 完整评测记入历史（中途取消的半截结果不记）。
		if !j.cancel.Load() && len(rep.Points) > 0 {
			appendHistory(p, rep)
		}

		ui(func() {
			if !j.alive(gen) {
				return
			}
			j.points = rep.Points
			j.limit = rep.TimeLimit
			j.bar.Hide()
			j.list.Refresh()
			j.running.Store(false)
			j.runBtn.Enable()
			j.stopBtn.Disable()
			j.refreshScore()

			if len(rep.Points) == 0 {
				j.note.SetText(tr("没有可评测的样例"))
				j.detail.SetText(tr("项目里还没有样例：放入 .in 与 .out/.ans 后到“样例”页同步。"))
				j.pg.g.busy(false, tr("没有样例"))
				return
			}

			// 评测结束后自动选中第一个测试点，右侧立刻显示它的详情。
			j.list.Select(0)

			if j.cancel.Load() {
				j.status.SetText(fmt.Sprintf(tr("已停止（完成 %d/%d）"), len(rep.Points), total))
				j.pg.g.busy(false, tr("评测已停止"))
				return
			}
			if rep.allAC() {
				if len(rep.Warnings) > 0 {
					j.status.SetText(fmt.Sprintf(tr("评测完成，全部通过（%d 条编译警告）"), len(rep.Warnings)))
				} else {
					j.status.SetText(fmt.Sprintf(tr("评测完成，全部通过（%d 线程）"), threads))
				}
			} else {
				j.status.SetText(tr("评测完成，存在未通过的测试点"))
			}
			if len(rep.Warnings) > 0 {
				// 全部通过时右侧详情正好空闲，用来展示警告全文。
				if rep.allAC() {
					j.detail.SetText(tr("编译警告（不影响判定）：") + "\n" + strings.Join(rep.Warnings, "\n"))
				} else {
					j.detail.SetText(j.detail.Text + "\n\n" + tr("编译警告（不影响判定）：") + "\n" + strings.Join(rep.Warnings, "\n"))
				}
			}
			j.pg.g.busy(false, fmt.Sprintf(tr("评测完成：%d/%d AC，%s"),
				rep.passed(), len(rep.Points), scoreTextOf(rep)))
		})
	}()
}

// targetText 是页面顶部的目标说明。
func (j *judgePage) targetText(p project) string {
	props := p.cfg()
	ioMode := tr("标准输入输出")
	if fileIOFromProps(props, p.codeName()) != nil {
		ioMode = tr("文件读写")
	}
	if spjEnabled(p, props) {
		return p.codeName() + ".cpp · " + ioMode + " · " + tr("特殊判题（checker.cpp）")
	}
	return p.codeName() + ".cpp · " + ioMode
}

// retest 只重测当前选中的测试点：复用已编译的可执行文件与 checker
// （都不存在时先编译），结果原地替换，不写历史。评测进行中忽略。
func (j *judgePage) retest() {
	if j.running.Load() || j.selectedID == "" {
		return
	}
	id := j.selectedID
	p := j.pg.p
	gen := j.pg.gen

	j.running.Store(true)
	j.cancel.Store(false)
	j.runBtn.Disable()
	j.stopBtn.Enable()
	j.retestBtn.Disable()
	j.status.SetText(fmt.Sprintf(tr("重测 #%s ..."), id))
	j.pg.g.busy(true, fmt.Sprintf(tr("重测 #%s ..."), id))

	go func() {
		props := p.cfg()

		// 可执行文件缺失（比如被清理过）就现编译一次。
		if !fileExists(p.exePath()) {
			log, ok := compileProjectTo(p.sourcePath(), p.exePath(), props)
			if !ok {
				ui(func() {
					if !j.alive(gen) {
						return
					}
					j.running.Store(false)
					j.runBtn.Enable()
					j.stopBtn.Disable()
					j.status.SetText("[CE] " + tr("编译失败"))
					j.detail.SetText(log)
					j.pg.g.busy(false, tr("编译失败"))
				})
				return
			}
		}
		checkerExe, ceLog, checkerOK := prepareChecker(p, props)
		if !checkerOK {
			ui(func() {
				if !j.alive(gen) {
					return
				}
				j.running.Store(false)
				j.runBtn.Enable()
				j.stopBtn.Disable()
				j.status.SetText("[CE] " + tr("checker 编译失败"))
				j.detail.SetText(ceLog)
				j.pg.g.busy(false, tr("checker 编译失败"))
			})
			return
		}

		limit, memCap := judgeLimits(props)
		job := judgeJob{
			ExePath:   p.exePath(),
			Dir:       p.Dir,
			Index:     []string{id},
			Threads:   1,
			TimeLimit: limit,
			MemLimit:  memCap,
			FileIO:    fileIOFromProps(props, p.codeName()),
			Checker:   checkerExe,
		}
		rep := evalSamples(job, func(pt pointResult) {
			ui(func() {
				if !j.alive(gen) {
					return
				}
				// 结果按编号原地替换，保持列表顺序稳定。
				for i := range j.points {
					if j.points[i].ID == pt.ID {
						j.points[i] = pt
						break
					}
				}
				j.limit = limit
				j.memCap = memCap
				j.list.Refresh()
				j.refreshScore()
				j.showDetail(pt)
			})
		}, j.cancel.Load)

		ui(func() {
			if !j.alive(gen) {
				return
			}
			j.running.Store(false)
			j.runBtn.Enable()
			j.stopBtn.Disable()
			if j.selectedID == id {
				j.retestBtn.Enable()
			}
			if len(rep.Points) == 0 {
				j.status.SetText(tr("重测失败：样例文件缺失"))
				j.pg.g.busy(false, tr("重测失败"))
				return
			}
			j.status.SetText(fmt.Sprintf(tr("重测 #%s 完成（%s）"), id, rep.Points[0].Verdict))
			j.pg.g.busy(false, fmt.Sprintf(tr("重测 #%s：%s"), id, rep.Points[0].Verdict))
		})
	}()
}

// showHistory 弹出评测历史：左侧记录列表（最新在上），右侧选中记录的
// 逐点明细；顶部一行方块是耗时趋势速览。
func (j *judgePage) showHistory() {
	entries := readHistory(j.pg.p, 50)
	if len(entries) == 0 {
		dialog.ShowInformation(tr("评测历史"), tr("还没有评测记录。\n每次“编译并评测”都会自动记录到这里。"), j.pg.g.win)
		return
	}

	// 列表展示最新在上，明细按下标反查。
	type histRow struct {
		e     historyEntry
		label string
	}
	rows := make([]histRow, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		status := fmt.Sprintf(tr("%d/%d 全部通过"), e.Passed, e.Total)
		if e.Passed < e.Total {
			status = fmt.Sprintf("%d/%d", e.Passed, e.Total)
			if w := firstWorstVerdict(e.Points); w != "" {
				status += " · " + w
			}
		}
		rows[len(entries)-1-i] = histRow{e: e, label: e.Time[5:] + "  " + status}
	}

	var walls []int64
	for _, e := range entries {
		walls = append(walls, maxWallOf(e))
	}
	trend := caption("")
	if spark := sparkline(walls); spark != "" {
		trend.SetText(tr("最慢点耗时趋势（旧 → 新）  ") + spark)
	}

	detail := widget.NewLabel("")
	detail.TextStyle = fyne.TextStyle{Monospace: true}
	detail.Wrapping = fyne.TextWrapWord

	list := widget.NewList(
		func() int { return len(rows) },
		func() fyne.CanvasObject { return widget.NewLabel("2026-01-02 15:04") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(rows) {
				return
			}
			obj.(*widget.Label).SetText(rows[id].label)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(rows) {
			return
		}
		e := rows[id].e
		var b strings.Builder
		fmt.Fprintf(&b, tr("%s    %d/%d 通过    限制 %.2fs / %.0fMB\n\n"),
			e.Time, e.Passed, e.Total, e.TimeLimit, e.MemLimit)
		for _, pt := range e.Points {
			switch pt.Verdict {
			case verdictTLE:
				fmt.Fprintf(&b, "#%s  TLE   >%.0fms\n", pt.ID, e.TimeLimit*1000)
			case verdictMissing:
				fmt.Fprintf(&b, "#%s  %s\n", pt.ID, tr("缺失"))
			default:
				fmt.Fprintf(&b, "#%s  %-4s  %dms  %s\n", pt.ID, pt.Verdict, pt.WallMS, memStr(pt))
			}
			if s := strings.TrimSpace(pt.Detail); s != "" {
				fmt.Fprintf(&b, "      %s\n", firstLines(s, 3))
			}
		}
		detail.SetText(b.String())
	}

	var d dialog.Dialog
	clear := widget.NewButton(tr("清空历史"), func() {
		d.Hide()
		dialog.ShowConfirm(tr("清空评测历史"), tr("将删除这个项目的全部评测记录，确定吗？"), func(ok bool) {
			if ok {
				clearHistory(j.pg.p)
				j.pg.g.setStatus(tr("评测历史已清空"))
			}
		}, j.pg.g.win)
	})

	export := func(format string) {
		entries := readHistory(j.pg.p, 0)
		if len(entries) == 0 {
			return
		}
		var data []byte
		var err error
		if format == "json" {
			data, err = historyJSON(entries)
		} else {
			data = historyCSV(entries)
		}
		if err != nil {
			j.pg.g.showError(err)
			return
		}
		save := dialog.NewFileSave(func(f fyne.URIWriteCloser, werr error) {
			if werr != nil || f == nil {
				return
			}
			if _, werr = f.Write(data); werr != nil {
				j.pg.g.showError(werr)
				return
			}
			name := f.URI().Name()
			_ = f.Close()
			j.pg.g.setStatus(fmt.Sprintf(tr("已导出 %d 条记录到 %s"), len(entries), name))
		}, j.pg.g.win)
		save.SetFileName(fmt.Sprintf("history-%s.%s", j.pg.p.codeName(), format))
		save.Show()
	}
	exportCsvBtn := widget.NewButton(tr("导出 CSV"), func() { export("csv") })
	exportJsonBtn := widget.NewButton(tr("导出 JSON"), func() { export("json") })

	body := container.NewHSplit(list, container.NewVScroll(detail))
	body.SetOffset(0.34)

	content := container.NewBorder(
		container.NewVBox(container.NewPadded(trend), widget.NewSeparator()),
		container.NewHBox(clear, exportCsvBtn, exportJsonBtn),
		nil, nil,
		body,
	)
	d = dialog.NewCustom(fmt.Sprintf(tr("评测历史（最近 %d 次）"), len(entries)), tr("关闭"), content, j.pg.g.win)
	d.Resize(fyne.NewSize(640, 460))
	d.Show()
}

// compileOnly 只编译不评测（Ctrl+B）：结果写进状态栏与详情区，
// 供快速检查语法错误。评测进行中忽略。
func (j *judgePage) compileOnly() {
	if j.running.Load() {
		return
	}
	p := j.pg.p
	j.pg.g.busy(true, fmt.Sprintf(tr("编译 %s ..."), p.codeName()))
	j.note.SetText(tr("编译中 ..."))
	gen := j.pg.gen
	go func() {
		log, ok := compileProjectTo(p.sourcePath(), p.exePath(), p.cfg())
		ui(func() {
			if !j.alive(gen) {
				return
			}
			if ok {
				j.pg.g.busy(false, tr("编译成功"))
				j.note.SetText(tr("编译成功，可开始评测"))
			} else {
				j.pg.g.busy(false, tr("编译失败"))
				j.note.SetText(tr("编译失败"))
				j.detail.SetText(log)
			}
		})
	}()
}

// reload 在切换项目后重置页面。
func (j *judgePage) reload() {
	if j.running.Load() {
		return
	}
	p := j.pg.p
	j.target.SetText(j.targetText(p))
	j.points = nil
	j.selectedID = ""
	j.retestBtn.Disable()
	j.list.UnselectAll()
	j.list.Refresh()
	j.score.SetText("—")
	j.note.SetText(tr("尚未评测"))
	j.detail.SetText(tr("选择左侧测试点查看详情。"))
	j.status.SetText(tr("点“编译并评测”开始，默认跟随项目配置的时限与内存。"))
}

// setLimits 记录本次评测使用的时限与内存限制。只能在 UI 线程调用：
// 列表模板与详情面板会无锁读取这两个值。
func (j *judgePage) setLimits(timeLimit, memLimit float64) {
	j.limit, j.memCap = timeLimit, memLimit
}

// refreshScore 依据当前结果刷新评分卡片。
func (j *judgePage) refreshScore() {
	total := len(j.points)
	if total == 0 {
		j.score.SetText("—")
		j.note.SetText(tr("尚未评测"))
		return
	}
	passed := 0
	worst := ""
	for _, pt := range j.points {
		if pt.Verdict == verdictAC {
			passed++
		} else if worst == "" || pt.Verdict == verdictMissing {
			worst = pt.Verdict
		}
	}
	j.score.SetText(fmt.Sprintf("%d / %d", passed, total))

	switch {
	case passed == total:
		j.note.SetText(fmt.Sprintf(tr("全部通过 · %d 分"), 100))
		j.score.Importance = widget.SuccessImportance
	case worst == verdictMissing:
		j.note.SetText(tr("有样例文件缺失"))
		j.score.Importance = widget.WarningImportance
	default:
		j.note.SetText(fmt.Sprintf(tr("首次未通过：#%s %s"), firstFailedID(j.points), worst))
		j.score.Importance = widget.DangerImportance
	}
	j.score.Refresh()
}

// firstFailedID 返回第一个未通过测试点的编号。
func firstFailedID(points []pointResult) string {
	for _, pt := range points {
		if pt.Verdict != verdictAC {
			return pt.ID
		}
	}
	return "-"
}

// scoreTextOf 生成「通过 x/y，得分 z」形式的短句。
func scoreTextOf(rep judgeReport) string {
	if len(rep.Points) == 0 {
		return tr("无样例")
	}
	return fmt.Sprintf(tr("%d 分"), rep.passed()*100/len(rep.Points))
}

// stop 请求中断评测：当前测试点跑完后不再继续。
func (j *judgePage) stop() {
	j.cancel.Store(true)
	j.status.SetText(tr("正在停止 ..."))
}

// alive 判断后台任务是否仍应更新界面。
func (j *judgePage) alive(gen int64) bool {
	return j.pg.g.generation.Load() == gen
}

// showDetail 在右侧展示单个测试点的详情。
func (j *judgePage) showDetail(pt pointResult) {
	switch pt.Verdict {
	case verdictAC:
		detail := fmt.Sprintf(tr("#%s 通过\n\n用时 %dms，峰值内存 %s。"), pt.ID, pt.WallMS, memStr(pt))
		if strings.TrimSpace(pt.Detail) != "" {
			detail += "\n\nchecker：" + pt.Detail
		}
		j.detail.SetText(detail)
	case verdictWA:
		if strings.TrimSpace(pt.Detail) != "" && pt.Expected == "" && pt.Actual == "" {
			// 特殊判题的反馈：没有期望/实际对比，只有 checker 的意见。
			j.detail.SetText(fmt.Sprintf(tr("#%s 未通过\n\n用时 %dms，峰值内存 %s\n\n%s"),
				pt.ID, pt.WallMS, memStr(pt), pt.Detail))
			return
		}
		j.detail.SetText(fmt.Sprintf(tr("#%s 输出不一致\n\n期望：\n%s\n\n实际：\n%s"), pt.ID, pt.Expected, pt.Actual))
	case verdictRE:
		detail := pt.Detail
		if strings.TrimSpace(detail) == "" {
			detail = tr("（程序没有输出 stderr）")
		}
		j.detail.SetText(fmt.Sprintf(tr("#%s 运行时错误\n\n用时 %dms，峰值内存 %s\n\n%s"),
			pt.ID, pt.WallMS, memStr(pt), detail))
	case verdictTLE:
		j.detail.SetText(fmt.Sprintf(tr("#%s 超出时限（%.2fs）"), pt.ID, j.limit))
	case verdictMLE:
		j.detail.SetText(fmt.Sprintf(tr("#%s 超出内存限制（%.2fMB / %.0fMB）"), pt.ID, pt.MemMB, j.memCap))
	case verdictMissing:
		j.detail.SetText(fmt.Sprintf(tr("#%s 样例文件缺失：需要同时存在 %s.in 与 %s.out/.ans。"), pt.ID, pt.ID, pt.ID))
	default:
		j.detail.SetText(fmt.Sprintf("#%s %s", pt.ID, pt.Verdict))
	}
}

// pointRow 是评测结果列表的一行：判定徽标 + 编号 + 时空。
// 文字用 canvas.Text 绘制并居中，避免混用 Label 时行高不一致导致
// 上下不齐。
type pointRow struct {
	widget.BaseWidget

	badge *verdictBadge
	id    *canvas.Text
	info  *canvas.Text
}

func newPointRow() *pointRow {
	r := &pointRow{
		badge: newVerdictBadge(),
		id:    canvas.NewText("#1", verdictIdle()),
		info:  canvas.NewText("", verdictIdle()),
	}
	r.id.TextSize = 13
	r.id.TextStyle = fyne.TextStyle{Bold: true}
	r.info.TextSize = 12
	r.ExtendBaseWidget(r)
	return r
}

func (r *pointRow) update(pt pointResult, limit float64) {
	r.badge.set(pt.Verdict)
	r.id.Text = "#" + pt.ID
	r.id.Refresh()

	switch pt.Verdict {
	case verdictMissing:
		r.info.Text = tr("文件缺失")
	case verdictTLE:
		r.info.Text = fmt.Sprintf(">%.0fms", limit*1000)
	case verdictRE, verdictMLE, verdictAC, verdictWA:
		r.info.Text = fmt.Sprintf("%dms · %s", pt.WallMS, memStr(pt))
	default:
		r.info.Text = pt.Verdict
	}
	r.info.Refresh()
}

func (r *pointRow) CreateRenderer() fyne.WidgetRenderer {
	// canvas.Text 在 HBox 里会贴底绘制，套 Center 才能与徽标垂直对齐。
	centerText := func(t *canvas.Text) fyne.CanvasObject {
		return container.NewStack(container.NewCenter(t))
	}
	line := container.NewHBox(r.badge, centerText(r.id), layout.NewSpacer(), centerText(r.info))
	return widget.NewSimpleRenderer(container.NewPadded(line))
}
