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
	"path/filepath"
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

// contestPage 是一整场模拟赛的工作台：题目/子任务配置、评测、榜单、导出。
type contestPage struct {
	g   *guiApp
	c   contest
	gen int64

	content fyne.CanvasObject
	title   *widget.Label
	meta    *widget.Label
	status  *widget.Label
	bar     *widget.ProgressBar
	detail  *widget.Label

	judgeBtn  *widget.Button
	stopBtn   *widget.Button
	exportBtn *widget.Button

	editor      *contestEditor
	boardHolder *fyne.Container
	boardScroll *container.Scroll

	res       contestResult
	hasResult bool

	running atomic.Bool
	cancel  atomic.Bool
	// savePending 用于合并连续输入产生的写盘请求。
	savePending atomic.Bool
	warns       []string
}

func newContestPage(g *guiApp, c contest) *contestPage {
	p := &contestPage{g: g, c: c, gen: g.generation.Load()}
	p.res, p.hasResult = loadContestResult(c)
	p.warns = validateContest(c)

	btns := container.NewHBox(
		widget.NewButtonWithIcon("返回比赛列表", theme.NavigateBackIcon(), g.showContestList),
		layout.NewSpacer(),
	)

	p.title = widget.NewLabelWithStyle(c.Cfg.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.title.SizeName = theme.SizeNameSubHeadingText
	p.meta = caption("")
	p.meta.Wrapping = fyne.TextWrapWord
	p.status = caption("准备就绪。")
	// 进度文字会带选手名与题目名，必须允许换行，否则长文本会把整页撑宽。
	p.status.Wrapping = fyne.TextWrapWord
	p.status.Truncation = fyne.TextTruncateEllipsis
	p.bar = widget.NewProgressBar()
	p.bar.Hide()
	p.detail = hint("")
	p.detail.Hide()
	p.boardHolder = container.NewVBox(layout.NewSpacer())
	// 整个榜单挂在一个双向滚动容器里：内容有多宽多高就滚多远，不撑窗口。
	p.boardScroll = container.NewVScroll(container.NewPadded(p.boardHolder))
	p.boardScroll.Direction = container.ScrollBoth
	// 榜单内部是 canvas 画的表格，本身不报最小尺寸；Fyne 的 Scroll 只有在
	// 内容 MinSize 大于可视区域时才出现滚动条，所以给一个"至少一屏"的下限，
	// 否则会出现内容溢出却滚不动的现象。
	p.boardScroll.SetMinSize(fyne.NewSize(560, 320))

	p.judgeBtn = widget.NewButtonWithIcon("开始评测", theme.MediaPlayIcon(), p.startJudge)
	p.judgeBtn.Importance = widget.HighImportance
	p.stopBtn = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), p.stopJudge)
	p.stopBtn.Disable()
	p.exportBtn = widget.NewButtonWithIcon("导出 HTML 榜单", theme.DownloadIcon(), p.exportHTML)
	p.exportBtn.Disable()

	head := container.NewVBox(
		container.NewPadded(btns),
		container.NewPadded(container.NewVBox(p.title, p.meta)),
		// 按钮行只放按钮：状态文字单独占一行，否则进度信息会把整行
		// 撑宽，连带挤压左侧的模拟赛入口。
		container.NewPadded(container.NewHBox(p.judgeBtn, p.stopBtn, p.exportBtn)),
		container.NewPadded(p.status),
		container.NewPadded(p.bar),
	)

	ed := newContestEditor(p)
	p.editor = ed
	configTab := ed.content

	tabs := container.NewAppTabs(
		// 榜单用双向 Scroll：嵌套的 HScroll(VScroll(...)) 在 Fyne 里会互相
		// 算错 MinSize（内层把高度撑满，外层再无余量），结果纵向根本滚不动。
		// NewScroll 一个控件同时处理横向与纵向，事件也不会被内层吃掉。
		container.NewTabItemWithIcon("榜单", theme.ListIcon(), p.boardScroll),
		container.NewTabItemWithIcon("题目配置", theme.SettingsIcon(), configTab),
		container.NewTabItemWithIcon("选手与提交", theme.AccountIcon(), newContestRoster(p).content),
		container.NewTabItemWithIcon("评测日志", theme.InfoIcon(), container.NewVScroll(container.NewPadded(p.detail))),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	p.content = container.NewBorder(head, nil, nil, nil, tabs)
	p.reload()
	return p
}

// reload 刷新标题、概况与榜单。
func (p *contestPage) reload() {
	// 界面可能尚未构建（单元测试或后台调用），逐个判断而不是整体跳过。
	if p.title != nil {
		p.title.SetText(p.c.Cfg.Name)
	}
	p.refreshMeta()
	if p.editor != nil {
		p.editor.reload()
	}
	if p.boardHolder != nil {
		p.refreshBoard()
	}
}

// rescan 重新扫描 data/ 目录，补上新增的测试点（保留已有分值）。
func (p *contestPage) rescan() {
	c := p.c
	problems, err := scanContestProblems(c.dataDir())
	if err != nil {
		p.g.showError(err)
		return
	}

	old := map[string]contestProblem{}
	for _, prob := range c.Cfg.Problems {
		old[prob.Name] = prob
	}
	for i := range problems {
		prev, ok := old[problems[i].Name]
		if !ok {
			continue
		}
		// 保留用户设置过的标题、时限与已有测试点的分值。
		problems[i].Title = prev.Title
		if prev.TimeMS > 0 {
			problems[i].TimeMS = prev.TimeMS
		}
		if prev.MemMB > 0 {
			problems[i].MemMB = prev.MemMB
		}
		problems[i].Subtasks = prev.Subtasks
		score := map[string]testPoint{}
		for _, t := range prev.Tests {
			score[t.ID] = t
		}
		for j := range problems[i].Tests {
			if t, ok := score[problems[i].Tests[j].ID]; ok {
				problems[i].Tests[j].Score = t.Score
				problems[i].Tests[j].Subtask = t.Subtask
			}
		}
	}
	c.Cfg.Problems = problems
	if err := c.save(); err != nil {
		p.g.showError(fmt.Errorf("写入配置失败：%v", err))
		return
	}
	p.c = c
	p.warns = validateContest(c)
	p.reload()
	p.g.setStatus(fmt.Sprintf("已重新扫描：%d 道题", len(problems)))
}

// autoSubtasks 按测试点自动生成子任务：每个测试点一个 all 型子任务。
func (p *contestPage) autoSubtasks() {
	c := p.c
	for i := range c.Cfg.Problems {
		prob := &c.Cfg.Problems[i]
		if len(prob.Subtasks) > 0 {
			continue // 已经配了子任务的题不动
		}
		var subs []contestSubtask
		for j, t := range prob.Tests {
			subs = append(subs, contestSubtask{
				Name:  fmt.Sprintf("测试点 %s", t.ID),
				Score: t.Score,
				Mode:  subtaskAll,
				Tests: []string{t.ID},
			})
			prob.Tests[j].Subtask = j
		}
		prob.Subtasks = subs
	}
	if err := c.save(); err != nil {
		p.g.showError(fmt.Errorf("写入配置失败：%v", err))
		return
	}
	p.c = c
	p.reload()
	p.g.setStatus("已按测试点生成子任务")
}

// fillScores 把每道题的满分补到 100（没有子任务时按测试点均分）。
func (p *contestPage) fillScores() {
	c := p.c
	for i := range c.Cfg.Problems {
		prob := &c.Cfg.Problems[i]
		if len(prob.Subtasks) > 0 {
			var sum float64
			for _, st := range prob.Subtasks {
				sum += st.Score
			}
			if sum > 0 {
				for j := range prob.Subtasks {
					prob.Subtasks[j].Score = prob.Subtasks[j].Score / sum * 100
				}
			}
			continue
		}
		if len(prob.Tests) == 0 {
			continue
		}
		each := 100.0 / float64(len(prob.Tests))
		for j := range prob.Tests {
			prob.Tests[j].Score = each
		}
	}
	if err := c.save(); err != nil {
		p.g.showError(fmt.Errorf("写入配置失败：%v", err))
		return
	}
	p.c = c
	p.reload()
	p.g.setStatus("每道题的满分已归一化到 100 分")
}

// alive 判断页面的后台任务是否仍应更新界面。
func (p *contestPage) alive(gen int64) bool {
	return p.g.generation.Load() == gen
}

// stop 在切走页面时中断评测。
func (p *contestPage) stop() {
	if p.running.Load() {
		p.cancel.Store(true)
	}
}

// startJudge 在后台评测整场比赛。
func (p *contestPage) startJudge() {
	if p.running.Load() {
		return
	}
	// 编辑框里的改动可能还在等待延迟落盘，先确保评测用的是最新配置。
	p.flushSave()
	c := p.c
	if len(c.Cfg.Sources) == 0 {
		p.g.showError(fmt.Errorf("这场比赛没有任何选手提交"))
		return
	}
	for _, w := range p.warns {
		if strings.Contains(w, "引用了不存在的测试点") {
			p.g.showError(fmt.Errorf("配置里有问题，先修好子任务：%s", w))
			return
		}
	}

	p.running.Store(true)
	p.cancel.Store(false)
	p.judgeBtn.Disable()
	p.stopBtn.Enable()
	p.exportBtn.Disable()
	p.bar.SetValue(0)
	p.bar.Show()
	p.detail.SetText("")
	p.detail.SetText(fmt.Sprintf("开始评测：%d 份提交，%d 并发\n", len(c.Cfg.Sources), contestParallelism()))
	p.status.SetText("正在编译并评测 ...")
	p.g.busy(true, "模拟赛评测中 ...")

	gen := p.gen
	// isCancelled 是传给引擎的停止信号：编译与运行都会每 50ms 轮询它，
	// 一旦为真立刻杀进程，所以点“停止”能马上生效，而不是等整场跑完。
	isCancelled := func() bool { return p.cancel.Load() }
	cancelled := false

	go func() {
		res, err := judgeContest(c, func(ev judgeEvent) {
			if ev.Kind != judgeDone {
				return
			}
			ui(func() {
				if !p.alive(gen) {
					return
				}
				p.bar.SetValue(ev.Percent)
				// 状态行只放百分比与当前进度，完整的每份结果留在评测日志页，
				// 避免一条长消息把这一行撑到窗口外面去。
				p.status.SetText(fmt.Sprintf("%.0f%% · %s", ev.Percent*100, shorten(ev.Message, 60)))
				p.detail.SetText(p.detail.Text + ev.Message + "\n")
			})
		}, isCancelled)
		cancelled = p.cancel.Load()

		ui(func() {
			// 先复位运行状态：即使页面已经被切走（alive 为假），
			// 也不能让 running 卡在 true 导致再也点不动“开始评测”。
			p.running.Store(false)
			p.stopBtn.Disable()
			p.bar.Hide()

			if !p.alive(gen) {
				p.g.busy(false, "")
				return
			}
			p.judgeBtn.Enable()

			if cancelled {
				p.status.SetText("已中断评测，本次结果未保存")
				p.g.busy(false, "评测已中断")
				return
			}
			if err != nil {
				p.status.SetText("评测失败")
				p.g.busy(false, "评测失败")
				p.g.showError(err)
				return
			}
			if err := saveContestResult(c, res); err != nil {
				p.g.showError(fmt.Errorf("保存结果失败：%v", err))
			}
			p.res, p.hasResult = res, true
			p.bar.SetValue(1)
			p.refreshBoard()
			p.exportBtn.Enable()
			rows := buildRanking(c, res)
			top := ""
			if len(rows) > 0 {
				top = fmt.Sprintf("，第一名 %s（%s 分）", rows[0].Player, fmtScore(rows[0].Total))
			}
			p.status.SetText(fmt.Sprintf("评测完成：%d 位选手%s", len(rows), top))
			p.g.busy(false, "模拟赛评测完成")
		})
	}()
}

// stopJudge 请求中断评测。引擎每 50ms 轮询取消标志，编译与运行中的进程
// 都会被立即杀掉，因此这里只置位并给界面反馈，真正的收尾在评测 goroutine 里。
func (p *contestPage) stopJudge() {
	if !p.running.Load() {
		return
	}
	p.cancel.Store(true)
	p.stopBtn.Disable()
	p.status.SetText("正在停止（结束当前进程）...")
}

// refreshBoard 重建榜单表格，并回到顶部。
func (p *contestPage) refreshBoard() {
	p.boardHolder.Objects = []fyne.CanvasObject{p.buildBoard()}
	p.boardHolder.Refresh()
	if p.boardScroll != nil {
		p.boardScroll.ScrollToTop()
		p.boardScroll.Refresh()
	}
}

// buildBoard 生成榜单区域：没有结果时给提示，有结果时给表格 + 明细。
func (p *contestPage) buildBoard() fyne.CanvasObject {
	if !p.hasResult {
		tip := caption("还没有评测结果。点上方“开始评测”，或到“题目配置”检查子任务。")
		tip.Alignment = fyne.TextAlignCenter
		tip.Wrapping = fyne.TextWrapWord
		return container.NewVBox(
			container.NewPadded(container.NewCenter(
				widget.NewLabelWithStyle("尚未评测", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))),
			container.NewPadded(tip),
		)
	}

	c := p.c
	rows := buildRanking(c, p.res)
	full := totalFullScore(c)

	items := []fyne.CanvasObject{
		// 概览文字允许换行，否则选手多时这一行自己就把内容顶得很宽。
		wrappedCenter(fmt.Sprintf("共 %d 位选手 · %d 道题 · 满分 %s 分 · 评测于 %s",
			len(rows), len(p.res.Problems), fmtScore(full), p.res.JudgedAt)),
		// 表格不设宽高约束，统一交给外层 ScrollBoth 滚动。
		p.rankingTable(rows, full),
	}
	if len(p.warns) > 0 {
		var lines []string
		for _, w := range p.warns {
			lines = append(lines, "· "+w)
		}
		items = append(items, wrappedCenter("配置提醒：\n"+strings.Join(lines, "\n")))
	}
	items = append(items, widget.NewSeparator(), section("题目明细"))
	for _, name := range p.res.Problems {
		if prob, ok := c.problemOf(name); ok {
			items = append(items, p.problemDetail(prob, rows))
		}
	}
	p.exportBtn.Enable()
	return container.NewVBox(items...)
}

// wrappedCenter 生成一段居中的可换行说明文字。
func wrappedCenter(text string) fyne.CanvasObject {
	l := caption(text)
	l.Alignment = fyne.TextAlignCenter
	l.Wrapping = fyne.TextWrapWord
	return container.NewPadded(l)
}

// rankingTable 用 canvas 画一张对齐的榜单（Fyne 没有表格控件）。
func (p *contestPage) rankingTable(rows []rankRow, full float64) fyne.CanvasObject {
	c := p.c
	headers := []string{"#", "选手"}
	headers = append(headers, p.res.Problems...)
	headers = append(headers, "总分")

	widths := []float32{44, 130}
	for range p.res.Problems {
		widths = append(widths, 78)
	}
	widths = append(widths, 78)

	header := cellRow(headers, widths, true)
	objects := []fyne.CanvasObject{header, widget.NewSeparator()}
	for _, row := range rows {
		cells := []string{fmt.Sprintf("%d", row.Rank), row.Player}
		colors := []interface{}{toneNone, toneNone}
		for i, s := range row.Scores {
			cells = append(cells, fmtScore(s))
			colors = append(colors, scoreToneOf(c, p.res.Problems[i], s))
		}
		cells = append(cells, fmtScore(row.Total))
		colors = append(colors, toneNone)
		objects = append(objects, cellRowWithColors(cells, colors, widths, false))
	}

	_ = full
	return container.NewVBox(objects...)
}

// problemDetail 展示一道题的子任务与测试点状态矩阵。
func (p *contestPage) problemDetail(prob contestProblem, rows []rankRow) fyne.CanvasObject {
	full := fullScoreOf(p.c, prob.Name)
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("%s  满分 %s 分 · %d 个测试点", prob.Name, fmtScore(full), len(prob.Tests)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	items := []fyne.CanvasObject{container.NewPadded(title)}

	// 子任务矩阵
	if len(prob.Subtasks) > 0 {
		headers := []string{"子任务", "判定", "分值"}
		widths := []float32{150, 84, 56}
		for _, row := range rows {
			headers = append(headers, row.Player)
			widths = append(widths, 78)
		}

		objs := []fyne.CanvasObject{cellRow(headers, widths, true), widget.NewSeparator()}
		for _, st := range prob.Subtasks {
			cells := []string{st.Name, subtaskModeLabel(st.Mode), fmtScore(st.Score)}
			colors := []interface{}{nil, nil, nil}
			for _, row := range rows {
				awarded := 0.0
				if pr, ok := p.res.resultOf(row.Player, prob.Name); ok {
					for _, so := range pr.Subtasks {
						if so.Name == st.Name {
							awarded = so.Awarded
						}
					}
				}
				cells = append(cells, fmtScore(awarded))
				switch {
				case st.Score > 0 && awarded >= st.Score-1e-9:
					colors = append(colors, toneAC)
				case awarded <= 1e-9:
					colors = append(colors, toneFail)
				default:
					colors = append(colors, toneMissing)
				}
			}
			objs = append(objs, cellRowWithColors(cells, colors, widths, false))
		}
		items = append(items, container.NewPadded(container.NewVBox(objs...)))
	}

	// 测试点矩阵
	if len(prob.Tests) > 0 {
		headers := []string{"测试点", "分值"}
		widths := []float32{120, 56}
		for _, row := range rows {
			headers = append(headers, row.Player)
			widths = append(widths, 78)
		}
		objs := []fyne.CanvasObject{cellRow(headers, widths, true), widget.NewSeparator()}
		for _, t := range prob.Tests {
			cells := []string{t.ID, fmtScore(t.Score)}
			colors := []interface{}{nil, nil}
			for _, row := range rows {
				verdict := ""
				if pr, ok := p.res.resultOf(row.Player, prob.Name); ok {
					if !pr.CompileOK {
						verdict = "CE"
					} else {
						for _, to := range pr.Tests {
							if to.ID == t.ID {
								verdict = to.Verdict
							}
						}
					}
				}
				if verdict == "" {
					cells = append(cells, "—")
					colors = append(colors, nil)
					continue
				}
				cells = append(cells, verdict)
				colors = append(colors, verdictToneFor(verdict,
					fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark))
			}
			objs = append(objs, cellRowWithColors(cells, colors, widths, false))
		}
		items = append(items, container.NewPadded(container.NewVBox(objs...)))
	}

	return container.NewVBox(items...)
}

// exportHTML 弹保存框导出榜单页面。
func (p *contestPage) exportHTML() {
	if !p.hasResult {
		p.g.showError(fmt.Errorf("还没有评测结果"))
		return
	}
	def := sanitizeFileName(p.c.Cfg.Name) + "-排名.html"
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		outPath := w.URI().Path()
		_ = w.Close()
		path, err := exportContestHTML(p.c, p.res, outPath)
		if err != nil {
			p.g.showError(fmt.Errorf("导出失败：%v", err))
			return
		}
		p.g.setStatus("榜单已导出：" + path)
		openTip := dialog.NewConfirm("导出成功", "榜单已写入：\n"+path+"\n\n要现在打开吗？", func(ok bool) {
			if ok {
				if err := openPath(filepath.Dir(path)); err != nil {
					p.g.showError(err)
				}
			}
		}, p.g.win)
		openTip.SetConfirmText("打开所在目录")
		openTip.Show()
	}, p.g.win)
	save.SetFileName(def)
	save.Show()
}

// cellRow 生成一行表头/数据（纯文本，单一颜色）。
func cellRow(cells []string, widths []float32, header bool) fyne.CanvasObject {
	colors := make([]interface{}, len(cells))
	return cellRowWithColors(cells, colors, widths, header)
}

// cellRowWithColors 生成一行等宽单元格，颜色为空时用主题的文字色。
//
// 这里必须问主题要颜色，不能写死某一种灰：界面支持浅色/深色两套，
// 写死的灰在另一套背景上会糊成一片。
func cellRowWithColors(cells []string, colors []interface{}, widths []float32, header bool) fyne.CanvasObject {
	dark := fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark
	// 表头用次要文字色（比正文弱），普通单元格用正文色。
	base := theme.Color(theme.ColorNameForeground)
	if header {
		base = theme.Color(theme.ColorNameDisabled)
	}
	objs := make([]fyne.CanvasObject, 0, len(cells))
	for i, text := range cells {
		w := float32(90)
		if i < len(widths) {
			w = widths[i]
		}
		align := fyne.TextAlignCenter
		if i == 1 {
			align = fyne.TextAlignLeading
		}
		col := base
		if i < len(colors) && !header {
			if c, ok := colors[i].(color.Color); ok && c != nil {
				col = c
			} else if sc, ok := colors[i].(scoreTone); ok {
				col = scoreToneColor(sc, dark)
			}
		}
		t := canvas.NewText(text, col)
		t.TextSize = 13
		t.Alignment = align
		t.TextStyle = fyne.TextStyle{Bold: header}
		objs = append(objs, container.NewGridWrap(fyne.NewSize(w, 26), t))
	}
	return container.NewHBox(objs...)
}

// shorten 把长文本截断到 n 个字符（按 rune 计，避免切坏中文）。
func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// scoreToneOf 返回分数单元格的语义色：满分绿、0 分红、部分分黄。
func scoreToneOf(c contest, problem string, score float64) scoreTone {
	full := fullScoreOf(c, problem)
	switch {
	case full > 0 && score >= full-1e-9:
		return toneAC
	case score <= 1e-9:
		return toneFail
	default:
		return toneMissing
	}
}

// showContestList 把右侧详情切回模拟赛列表。
func (g *guiApp) showContestList() {
	g.generation.Add(1)
	g.page = nil
	g.selected = ""
	g.mode = modeContest
	if g.list != nil {
		g.list.UnselectAll()
		g.list.Refresh()
	}
	g.detail.Objects = []fyne.CanvasObject{g.buildContestList()}
	g.detail.Refresh()
	g.setStatus("模拟赛")
}
