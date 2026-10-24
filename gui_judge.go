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
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

	runBtn  *widget.Button
	stopBtn *widget.Button
	status  *widget.Label
	list    *widget.List
	detail  *widget.Label
	score   *widget.Label
	note    *widget.Label
	target  *widget.Label
	bar     *widget.ProgressBar

	points []pointResult
	limit  float64
	memCap float64

	running atomic.Bool
	cancel  atomic.Bool
}

func newJudgePage(pg *projectPage) *judgePage {
	j := &judgePage{pg: pg}

	j.status = caption("点“编译并评测”开始，默认跟随项目配置的时限与内存。")
	j.detail = hint("选择左侧测试点查看详情。")
	j.detail.Wrapping = fyne.TextWrapWord

	// 评分用大号等宽数字，一眼能看到。
	j.score = widget.NewLabel("—")
	j.score.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	j.score.SizeName = theme.SizeNameHeadingText
	j.note = caption("尚未评测")
	j.target = hint("")
	j.bar = widget.NewProgressBar()
	j.bar.Hide()

	j.runBtn = widget.NewButtonWithIcon("编译并评测", theme.MediaPlayIcon(), j.run)
	j.runBtn.Importance = widget.HighImportance
	j.stopBtn = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), j.stop)
	j.stopBtn.Disable()

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
		j.showDetail(j.points[id])
	}

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			container.NewHBox(j.runBtn, j.stopBtn),
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
	title := caption("评分")
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
	j.detail.SetText("正在编译 ...")
	j.score.SetText("—")
	j.note.SetText("评测中 ...")
	j.status.SetText("编译 " + name + ".cpp ...")
	j.runBtn.Disable()
	j.stopBtn.Enable()
	j.target.SetText(j.targetText(p))

	gen := j.pg.gen
	go func() {
		props := p.cfg()
		limit, memCap := judgeLimits(props)
		threads := judgeThreads(props)
		j.setLimits(limit, memCap)

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
				j.note.SetText("编译失败")
				j.status.SetText("[CE] 编译失败")
				j.detail.SetText(log)
				j.pg.g.busy(false, "编译失败")
			})
			return
		}

		ui(func() {
			if !j.alive(gen) {
				return
			}
			j.status.SetText(fmt.Sprintf("正在评测 %s（%d 线程）", name, threads))
			j.detail.SetText("正在运行测试点 ...")
			j.bar.SetValue(0)
			j.bar.Show()
			j.pg.g.busy(true, "评测中 ...")
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
		}
		total := len(index)

		// 并发回调来自多个 goroutine：这里只做计数与结果入表，界面
		// 更新全部经由 ui() 回到主线程，避免数据竞争。
		var mu sync.Mutex
		rep := evalSamples(job, func(pt pointResult) {
			mu.Lock()
			j.points = append(j.points, pt)
			done := len(j.points)
			mu.Unlock()
			ui(func() {
				if !j.alive(gen) {
					return
				}
				j.limit = limit
				j.list.Refresh()
				if total > 0 {
					j.bar.SetValue(float64(done) / float64(total))
				}
				j.refreshScore()
			})
		}, j.cancel.Load)

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
				j.note.SetText("没有可评测的样例")
				j.detail.SetText("项目里还没有样例：放入 .in 与 .out/.ans 后到“样例”页同步。")
				j.pg.g.busy(false, "没有样例")
				return
			}

			// 评测结束后自动选中第一个测试点，右侧立刻显示它的详情。
			j.list.Select(0)

			if j.cancel.Load() {
				j.status.SetText(fmt.Sprintf("已停止（完成 %d/%d）", len(rep.Points), total))
				j.pg.g.busy(false, "评测已停止")
				return
			}
			if rep.allAC() {
				j.status.SetText(fmt.Sprintf("评测完成，全部通过（%d 线程）", threads))
			} else {
				j.status.SetText("评测完成，存在未通过的测试点")
			}
			j.pg.g.busy(false, fmt.Sprintf("评测完成：%d/%d AC，%s",
				rep.passed(), len(rep.Points), scoreTextOf(rep)))
		})
	}()
}

// targetText 是页面顶部的目标说明。
func (j *judgePage) targetText(p project) string {
	return p.codeName() + ".cpp · 标准输入输出"
}

// reload 在切换项目后重置页面。
func (j *judgePage) reload() {
	if j.running.Load() {
		return
	}
	p := j.pg.p
	j.target.SetText(j.targetText(p))
	j.points = nil
	j.list.UnselectAll()
	j.list.Refresh()
	j.score.SetText("—")
	j.note.SetText("尚未评测")
	j.detail.SetText("选择左侧测试点查看详情。")
	j.status.SetText("点“编译并评测”开始，默认跟随项目配置的时限与内存。")
}

// setLimits 记录本次评测使用的时限与内存限制（后台线程安全，仅写数值）。
func (j *judgePage) setLimits(timeLimit, memLimit float64) {
	j.limit, j.memCap = timeLimit, memLimit
}

// refreshScore 依据当前结果刷新评分卡片。
func (j *judgePage) refreshScore() {
	total := len(j.points)
	if total == 0 {
		j.score.SetText("—")
		j.note.SetText("尚未评测")
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
		j.note.SetText(fmt.Sprintf("全部通过 · %d 分", 100))
		j.score.Importance = widget.SuccessImportance
	case worst == verdictMissing:
		j.note.SetText("有样例文件缺失")
		j.score.Importance = widget.WarningImportance
	default:
		j.note.SetText(fmt.Sprintf("首次未通过：#%s %s", firstFailedID(j.points), worst))
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
		return "无样例"
	}
	return fmt.Sprintf("%d 分", rep.passed()*100/len(rep.Points))
}

// stop 请求中断评测：当前测试点跑完后不再继续。
func (j *judgePage) stop() {
	j.cancel.Store(true)
	j.status.SetText("正在停止 ...")
}

// alive 判断后台任务是否仍应更新界面。
func (j *judgePage) alive(gen int64) bool {
	return j.pg.g.generation.Load() == gen
}

// showDetail 在右侧展示单个测试点的详情。
func (j *judgePage) showDetail(pt pointResult) {
	switch pt.Verdict {
	case verdictAC:
		j.detail.SetText(fmt.Sprintf("#%s 通过\n\n用时 %dms，峰值内存 %s。", pt.ID, pt.WallMS, memStr(pt)))
	case verdictWA:
		j.detail.SetText(fmt.Sprintf("#%s 输出不一致\n\n期望：\n%s\n\n实际：\n%s", pt.ID, pt.Expected, pt.Actual))
	case verdictRE:
		detail := pt.Detail
		if strings.TrimSpace(detail) == "" {
			detail = "（程序没有输出 stderr）"
		}
		j.detail.SetText(fmt.Sprintf("#%s 运行时错误\n\n用时 %dms，峰值内存 %s\n\n%s",
			pt.ID, pt.WallMS, memStr(pt), detail))
	case verdictTLE:
		j.detail.SetText(fmt.Sprintf("#%s 超出时限（%.2fs）", pt.ID, j.limit))
	case verdictMLE:
		j.detail.SetText(fmt.Sprintf("#%s 超出内存限制（%.2fMB / %.0fMB）", pt.ID, pt.MemMB, j.memCap))
	case verdictMissing:
		j.detail.SetText(fmt.Sprintf("#%s 样例文件缺失：需要同时存在 %s.in 与 %s.out/.ans。", pt.ID, pt.ID, pt.ID))
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
		r.info.Text = "文件缺失"
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
	line := container.NewHBox(r.badge, r.id, layout.NewSpacer(), r.info)
	return widget.NewSimpleRenderer(container.NewPadded(line))
}
