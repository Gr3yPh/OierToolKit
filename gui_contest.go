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
	"sort"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
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
	copyBtn   *widget.Button

	editor      *contestEditor
	boardHolder *fyne.Container
	boardScroll *container.Scroll

	res       contestResult
	hasResult bool

	// 榜单排序状态：0 = 默认按总分；2..n+1 = 题目列；n+2 = 总分列。
	// 点击表头切换，再点同一列反转方向。
	rankSortCol  int
	rankSortDesc bool
	// rankFilter 是榜单的选手名筛选串（小写），空 = 不过滤。
	rankFilter string

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
		widget.NewButtonWithIcon(tr("返回比赛列表"), theme.NavigateBackIcon(), g.showContestList),
		layout.NewSpacer(),
	)

	p.title = widget.NewLabelWithStyle(c.Cfg.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.title.SizeName = theme.SizeNameSubHeadingText
	p.meta = caption("")
	p.meta.Wrapping = fyne.TextWrapWord
	p.status = caption(tr("准备就绪。"))
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

	p.judgeBtn = widget.NewButtonWithIcon(tr("开始评测"), theme.MediaPlayIcon(), p.startJudge)
	p.judgeBtn.Importance = widget.HighImportance
	p.stopBtn = widget.NewButtonWithIcon(tr("停止"), theme.MediaStopIcon(), p.stopJudge)
	p.stopBtn.Disable()
	p.copyBtn = widget.NewButtonWithIcon(tr("复制榜单"), theme.ContentCopyIcon(), p.copyRanking)
	p.copyBtn.Disable()
	// 导出合成一个菜单：HTML / CSV / Markdown 三个按钮横着排会把标题挤掉。
	p.exportBtn = widget.NewButtonWithIcon(tr("导出…"), theme.DownloadIcon(), nil)
	p.exportBtn.OnTapped = p.showExportMenu
	p.exportBtn.Disable()

	// 标题与操作放同一行：评测状态紧跟按钮，不再被标题独占一行挤到下面。
	titleRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(p.judgeBtn, p.stopBtn, p.copyBtn, p.exportBtn),
		container.NewVBox(p.title, p.meta),
	)

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			btns, nil,
			titleRow,
		)),
		// 状态文字与进度条紧贴按钮行，评测时视线不用往下找。
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
		container.NewTabItemWithIcon(tr("榜单"), theme.ListIcon(), p.boardScroll),
		container.NewTabItemWithIcon(tr("题目配置"), theme.SettingsIcon(), configTab),
		container.NewTabItemWithIcon(tr("选手与提交"), theme.AccountIcon(), newContestRoster(p).content),
		container.NewTabItemWithIcon(tr("评测日志"), theme.InfoIcon(), container.NewVScroll(container.NewPadded(p.detail))),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	p.content = container.NewBorder(container.NewPadded(head), nil, nil, nil, tabs)
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
		p.g.showError(fmt.Errorf(tr("写入配置失败：%v"), err))
		return
	}
	p.c = c
	p.warns = validateContest(c)
	p.reload()
	p.g.setStatus(fmt.Sprintf(tr("已重新扫描：%d 道题"), len(problems)))
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
				Name:  fmt.Sprintf(tr("测试点 %s"), t.ID),
				Score: t.Score,
				Mode:  subtaskAll,
				Tests: []string{t.ID},
			})
			prob.Tests[j].Subtask = j
		}
		prob.Subtasks = subs
	}
	if err := c.save(); err != nil {
		p.g.showError(fmt.Errorf(tr("写入配置失败：%v"), err))
		return
	}
	p.c = c
	p.reload()
	p.g.setStatus(tr("已按测试点生成子任务"))
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
		p.g.showError(fmt.Errorf(tr("写入配置失败：%v"), err))
		return
	}
	p.c = c
	p.reload()
	p.g.setStatus(tr("每道题的满分已归一化到 100 分"))
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
		p.g.showError(fmt.Errorf("%s", tr("这场比赛没有任何选手提交")))
		return
	}
	for _, w := range p.warns {
		if strings.Contains(w, tr("引用了不存在的测试点")) {
			p.g.showError(fmt.Errorf(tr("配置里有问题，先修好子任务：%s"), w))
			return
		}
	}

	p.running.Store(true)
	p.cancel.Store(false)
	p.judgeBtn.Disable()
	p.stopBtn.Enable()
	p.exportBtn.Disable()
	p.copyBtn.Disable()
	p.bar.SetValue(0)
	p.bar.Show()
	p.detail.SetText("")
	p.detail.SetText(fmt.Sprintf(tr("开始评测：%d 份提交，%d 并发")+"\n", len(c.Cfg.Sources), contestParallelism()))
	p.status.SetText(tr("正在编译并评测 ..."))
	p.g.busy(true, tr("模拟赛评测中 ..."))

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
				p.status.SetText(tr("已中断评测，本次结果未保存"))
				p.g.busy(false, tr("评测已中断"))
				return
			}
			if err != nil {
				p.status.SetText(tr("评测失败"))
				p.g.busy(false, tr("评测失败"))
				p.g.showError(err)
				return
			}
			if err := saveContestResult(c, res); err != nil {
				p.g.showError(fmt.Errorf(tr("保存结果失败：%v"), err))
			}
			p.res, p.hasResult = res, true
			p.bar.SetValue(1)
			p.refreshBoard()
			p.exportBtn.Enable()
			p.copyBtn.Enable()
			rows := buildRanking(c, res)
			top := ""
			if len(rows) > 0 {
				top = fmt.Sprintf(tr("，第一名 %s（%s 分）"), rows[0].Player, fmtScore(rows[0].Total))
			}
			p.status.SetText(fmt.Sprintf(tr("评测完成：%d 位选手%s"), len(rows), top))
			p.g.busy(false, tr("模拟赛评测完成"))
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
	p.status.SetText(tr("正在停止（结束当前进程）..."))
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
		tip := caption(tr("还没有评测结果。点上方“开始评测”，或到“题目配置”检查子任务。"))
		tip.Alignment = fyne.TextAlignCenter
		tip.Wrapping = fyne.TextWrapWord
		return container.NewVBox(
			container.NewGridWrap(fyne.NewSize(1, theme.Padding()), layout.NewSpacer()),
			container.NewPadded(container.NewCenter(
				widget.NewLabelWithStyle(tr("尚未评测"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))),
			container.NewPadded(tip),
		)
	}

	c := p.c
	rows := buildRanking(c, p.res)
	full := totalFullScore(c)

	items := []fyne.CanvasObject{
		// 概览文字允许换行，否则选手多时这一行自己就把内容顶得很宽。
		// 顶部补一个 Padding()，和榜单页外层 Padded 的边距一致。
		container.NewGridWrap(fyne.NewSize(1, theme.Padding()), layout.NewSpacer()),
		textRow(fmt.Sprintf(tr("共 %d 位选手 · %d 道题 · 满分 %s 分 · 评测于 %s"),
			len(rows), len(p.res.Problems), fmtScore(full), p.res.JudgedAt),
			theme.Color(theme.ColorNameDisabled), theme.SizeNameCaptionText, false),
		passRateBars(passRates(c, p.res)),
		// 表格不设宽高约束，统一交给外层 ScrollBoth 滚动。
		p.rankingTable(rows, full),
	}
	if len(p.warns) > 0 {
		var lines []string
		for _, w := range p.warns {
			lines = append(lines, "· "+w)
		}
		items = append(items, wrappedCenter(tr("配置提醒：")+"\n"+strings.Join(lines, "\n")))
	}
	items = append(items, widget.NewSeparator(), section(tr("题目明细")))
	for i, name := range p.res.Problems {
		if prob, ok := c.problemOf(name); ok {
			if i > 0 {
				// 每道题之间留一个空行，长榜单里才好分辨分界。
				items = append(items, widget.NewSeparator())
			}
			items = append(items, p.problemDetail(prob, rows))
		}
	}
	p.exportBtn.Enable()
	p.copyBtn.Enable()
	return container.NewVBox(items...)
}

// passRateBars 生成每道题的满分率进度条行（题名 + 条 + 百分比）。
// 外层刻意不套任何 GridWrap/Padded：GridWrap 会按格子高度顶部对齐，
// 一格高度对不上就把落差全变成「通过率」标题上方的大片空白；VBox 只
// 按各子项 MinSize 自上而下排，是最没有惊喜的布局方式。
func passRateBars(rates []passRate) fyne.CanvasObject {
	if len(rates) == 0 {
		return nil
	}
	// 标题用 canvas.Text 直接画：widget.Label 的最小高度含文本框上下留白，
	// 会凭空在「通过率」上方撑出十几像素的空白。
	title := canvas.NewText(tr("通过率"), theme.Color(theme.ColorNameForeground))
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = theme.TextSize()
	// 标题上方不留 spacer（0px）：概览行和标题都是紧凑的 singleText（高度只报
	// 字号，见 textLineH），两行行盒之间没有布局空隙。剩下的视觉间距全部来自
	// canvas.Text 把字形画在行盒顶下方约 10px 的字体 ascent，布局减不掉，
	// 所以这里把标题字形额外上移 passRateTitleNudgeY，把这段 ascent 抵消掉一部分。
	out := []fyne.CanvasObject{textLineNudge(title, passRateTitleNudgeY)}
	for _, pr := range rates {
		bar := newSlimBar(pr.Rate())
		info := caption(fmt.Sprintf("%d/%d（%.0f%%）", pr.Solved, pr.Players, pr.Rate()*100))
		name := widget.NewLabel(pr.Problem)
		// 题名在左、数字在右，都套 Center 让文字在整行高度里垂直居中。
		// 右区放百分比文字（定宽），中间进度条吃掉剩余宽度。
		right := container.NewHBox(
			container.NewStack(container.NewCenter(container.NewHBox(info))),
		)
		row := container.NewBorder(nil, nil,
			container.NewStack(container.NewCenter(container.NewHBox(name))),
			right,
			bar,
		)
		out = append(out, row)
	}
	// 末尾留一段纯空白（不是分隔线），与下方榜单表格分开。
	out = append(out, container.NewGridWrap(fyne.NewSize(1, theme.Padding()), layout.NewSpacer()))
	// 整块限宽不能用 GridWrap：它把子项塞进固定格子并顶部对齐，内容比格子
	// 矮时落差全变成空白（就是「通过率」上方那片怎么也调不掉的空）。
	return maxWidthBars(container.NewVBox(out...), passRateBlockW)
}

// passRateBlockW 是通过率区块的最大宽度，与评测进度条一致。
const passRateBlockW = 560

// passRateTitleNudgeY 是「通过率」标题字形的额外纵向偏移（负值上移，0 = 居中）。
// 行盒之间本身是 0 间距，剩下的视觉间距来自 canvas.Text 的行距留白。
// 注意：修好 singleText 的 Layout 之后，canvas.Text 会被正确居中，每行自动比
// 之前上提约 2.5px，两行合计让标题行整体上移了约 5px——此时再叠 -4px 只剩
// 3px 净空，两行会黏在一起，所以这里归零，实测净空 7px（原来 12px）。
// 想更紧就调成负数（-1 → 6px，-2 → 5px），别超过 -6，否则字形会压到上一行。
const passRateTitleNudgeY = 0

// maxWidthBars 只压缩内容的横向宽度，纵向完全跟着内容走。
// 做成 wrapper 而不是 GridWrap：GridWrap 用固定的格子尺寸排版，格子比
// 内容高时会把子项顶部对齐并留下空白；这里 Layout 只在宽度上做 min()。
func maxWidthBars(content fyne.CanvasObject, max float32) fyne.CanvasObject {
	b := &barsWrapper{content: content, max: max}
	b.ExtendBaseWidget(b)
	return b
}

type barsWrapper struct {
	widget.BaseWidget
	content fyne.CanvasObject
	max     float32
}

func (b *barsWrapper) CreateRenderer() fyne.WidgetRenderer { return &barsRenderer{b: b} }

type barsRenderer struct{ b *barsWrapper }

func (r *barsRenderer) Layout(size fyne.Size) {
	w := r.b.max
	if size.Width < w {
		w = size.Width
	}
	// 高度固定取内容 MinSize：内容是一列固定高的行，不应被父级拉长
	// （拉长后 VBox 会把多出来的高度平均分给各行，又变回空白）。
	r.b.content.Resize(fyne.NewSize(w, r.b.content.MinSize().Height))
	r.b.content.Move(fyne.NewPos(0, 0))
}

func (r *barsRenderer) MinSize() fyne.Size {
	ms := r.b.content.MinSize()
	if ms.Width > r.b.max {
		ms.Width = r.b.max
	}
	return ms
}

func (r *barsRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.b.content} }

func (r *barsRenderer) Refresh() { canvas.Refresh(r.b.content) }

func (r *barsRenderer) Destroy() {}

// textLine 把一段 canvas.Text 包成只占文字实际高度的组件。
// 用它代替 widget.Label 当小标题，可以避免 Label 自带的行内留白。
func textLine(t *canvas.Text) fyne.CanvasObject {
	return newSingleText(t, 0)
}

// textLineNudge 同上，但把字形在行盒内额外上移/下移 dy 像素。
// 负值 = 上移，用于压缩和上一行之间的视觉间距（字体 ascent 是布局算不掉的）。
func textLineNudge(t *canvas.Text, dy float32) fyne.CanvasObject {
	return newSingleText(t, dy)
}

func newSingleText(t *canvas.Text, nudge float32) *singleText {
	s := &singleText{t: t, nudge: nudge}
	// BaseWidget 必须绑定自身实现，否则 Resize 不会触发渲染器的 Layout，
	// 内部 canvas.Text 的尺寸和位置就永远停在初始值，nudge 也就无从生效。
	s.ExtendBaseWidget(s)
	return s
}

type singleText struct {
	widget.BaseWidget
	t *canvas.Text
	// nudge 是字形在行盒内的额外纵向偏移，0 表示按行盒居中。
	nudge float32
}

func (s *singleText) CreateRenderer() fyne.WidgetRenderer {
	return &singleTextRenderer{s: s}
}

func (s *singleText) MinSize() fyne.Size {
	// 宽度取文字实际宽度：canvas.Text 不换行，报 0 会被父容器裁掉。
	return fyne.NewSize(s.t.MinSize().Width, textLineH(s.t))
}

// textLineH 是 singleText 占用的高度，直接取字号（14→14，11→11），
// 而不是 canvas.Text.MinSize（字号 14 报 19.08，字号 11 报 14.98）。
//
// 两者之差是字体行距：canvas.Text 在行盒内贴顶绘制，字形本身只有约
// 0.78 倍字号高，所以 MinSize 里多出来的 5~6px 会全部落在文字下方，
// 白送给下一个组件。实测在概览行上就是：字号 11 的文字从行盒顶部往下
// 约 10px 才出现第一个像素行，这 10px 会整个叠进「通过率」上方的间距，
// 让 5px 的 spacer 量出来变成 17px。
//
// 行盒压到字号本身后，字形（0.78 倍）仍然完整落在行盒内，只是行距不再
// 由字体决定、而由调用方插的 spacer 决定——要求 5px 就能精确落在 5px。
func textLineH(t *canvas.Text) float32 {
	return t.TextSize
}

// textRow 把一行只读文字包成紧凑组件：用 canvas.Text 而不是 widget.Label，
// 因为 Label 的行盒上下各留约 7px 空白，叠在「通过率」标题上方会变成
// 十几像素的空档。样式（颜色、字号、加粗、居中）全部在构造时定好，
// 之后不再变化，所以渲染器只负责排版。
func textRow(text string, col color.Color, sizeName fyne.ThemeSizeName, bold bool) fyne.CanvasObject {
	t := canvas.NewText(text, col)
	t.TextSize = theme.Size(sizeName)
	t.TextStyle = fyne.TextStyle{Bold: bold}
	t.Alignment = fyne.TextAlignCenter
	return newSingleText(t, 0)
}

type singleTextRenderer struct{ s *singleText }

func (r *singleTextRenderer) Layout(size fyne.Size) {
	// 宽度撑满行盒，让 canvas.Text 自己做水平居中（它按 size.Width 算偏移）；
	// 高度取文字本身的行高，再叠加调用方要求的 nudge。
	h := r.s.t.MinSize().Height
	r.s.t.Resize(fyne.NewSize(size.Width, h))
	r.s.t.Move(fyne.NewPos(0, (size.Height-h)/2+r.s.nudge))
}

func (r *singleTextRenderer) MinSize() fyne.Size { return r.s.MinSize() }

func (r *singleTextRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.s.t} }

func (r *singleTextRenderer) Refresh() {
	// 只重绘，不要在这里改颜色：颜色（比如概览行的灰色）是构造时定死的，
	// 覆盖成前景色会让灰字变黑。
	canvas.Refresh(r.s.t)
}

func (r *singleTextRenderer) Destroy() {}

// slimBar 是自绘的细进度条：Fyne 的 widget.ProgressBar 最小高度约 36，
// 强行压矮会留下错位空白，所以这里自己画背景、填充和居中文字。
type slimBar struct {
	widget.BaseWidget
	ratio float64
}

func newSlimBar(ratio float64) *slimBar {
	b := &slimBar{ratio: ratio}
	b.ExtendBaseWidget(b)
	return b
}

func (b *slimBar) MinSize() fyne.Size {
	// 只画一条细条，纵向高度就是条本身。宽度给一个较小的下限，
	// 实际宽度由父级 Border 拉伸，但整行有 passRateBlockW 的上限兜底。
	return fyne.NewSize(120, theme.Padding()*2)
}

func (b *slimBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	bg.CornerRadius = theme.Padding() / 2
	fill := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	fill.CornerRadius = theme.Padding() / 2
	r := &slimBarRenderer{bar: b, bg: bg, fill: fill}
	bg.Resize(fyne.NewSize(220, slimBarH))
	fill.Resize(fyne.NewSize(220*float32(b.ratio), slimBarH))
	return r
}

// slimBarH 是细条本身的视觉高度。
const slimBarH = 8

type slimBarRenderer struct {
	bar      *slimBar
	bg, fill *canvas.Rectangle
}

func (r *slimBarRenderer) Layout(size fyne.Size) {
	top := (size.Height - slimBarH) / 2
	r.bg.Move(fyne.NewPos(0, top))
	r.bg.Resize(fyne.NewSize(size.Width, slimBarH))
	w := size.Width * float32(r.bar.ratio)
	if w < 0 {
		w = 0
	}
	if w > size.Width {
		w = size.Width
	}
	r.fill.Move(fyne.NewPos(0, top))
	r.fill.Resize(fyne.NewSize(w, slimBarH))
}

func (r *slimBarRenderer) MinSize() fyne.Size {
	return r.bar.MinSize()
}

func (r *slimBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill}
}

func (r *slimBarRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.fill.FillColor = theme.Color(theme.ColorNamePrimary)
	canvas.Refresh(r.bar)
}

func (r *slimBarRenderer) Destroy() {}

// wrappedCenter 生成一段居中的可换行说明文字。
func wrappedCenter(text string) fyne.CanvasObject {
	l := caption(text)
	l.Alignment = fyne.TextAlignCenter
	l.Wrapping = fyne.TextWrapWord
	// 不要套 NewPadded：本主题 Padding()=5，上下各加 5 后这一行会从 32 长到
	// 42，视觉上就像「通过率」上方突然空了一行。
	return l
}

// rankingTable 用 canvas 画一张对齐的榜单（Fyne 没有表格控件）。
// 题目列和总分列的表头可以点击：点一下按该列降序，再点反转；
// 名次列始终按原始总分的名次显示，不受当前排序影响。
func (p *contestPage) rankingTable(rows []rankRow, full float64) fyne.CanvasObject {
	c := p.c
	problemCols := len(p.res.Problems)
	headers := []string{"#", tr("选手")}
	headers = append(headers, p.res.Problems...)
	headers = append(headers, tr("总分"))

	widths := []float32{44, 130}
	for range p.res.Problems {
		widths = append(widths, 78)
	}
	widths = append(widths, 78)

	_ = full
	box := container.NewVBox()
	// 选手名筛选框放在表格上方；只重建表格部分，输入框本身不动，
	// 否则每敲一个字焦点就丢。
	search := widget.NewEntry()
	search.SetPlaceHolder(tr("输入选手名筛选…"))
	// 先声明再赋值：:= 的作用域从语句之后才开始，闭包里引用不到。
	var rebuild func()
	rebuild = func() {
		// 表头：# 和选手名不可排，其余列可点击排序。
		head := make([]fyne.CanvasObject, 0, len(headers))
		for i, h := range headers {
			if i < 2 {
				head = append(head, plainHeaderCell(h, widths[i]))
				continue
			}
			title := h
			if i == p.rankSortCol {
				if p.rankSortDesc {
					title += " ▼"
				} else {
					title += " ▲"
				}
			}
			idx, desc := i, p.rankSortDesc
			head = append(head, newTapHeaderCell(title, widths[i], i == p.rankSortCol, func() {
				if p.rankSortCol == idx {
					p.rankSortDesc = !desc
				} else {
					p.rankSortCol = idx
					p.rankSortDesc = true
				}
				rebuild()
			}))
		}
		objs := []fyne.CanvasObject{container.NewHBox(head...), widget.NewSeparator()}

		// 视图行：先按选手名筛选，再按当前排序列重排。
		// Rank 保留原值——它是按总分算出来的名次，筛选和重排都不改。
		view := make([]rankRow, 0, len(rows))
		for _, r := range rows {
			if p.rankFilter == "" || strings.Contains(strings.ToLower(r.Player), p.rankFilter) {
				view = append(view, r)
			}
		}
		if p.rankFilter != "" {
			objs = append(objs, container.NewPadded(
				caption(fmt.Sprintf(tr("筛出 %d 位选手"), len(view)))))
		}
		if p.rankSortCol >= 2 {
			col, desc := p.rankSortCol, p.rankSortDesc
			sort.SliceStable(view, func(a, b int) bool {
				x, y := view[a].Total, view[b].Total
				if col < 2+problemCols {
					x, y = view[a].Scores[col-2], view[b].Scores[col-2]
				}
				if x != y {
					if desc {
						return x > y
					}
					return x < y
				}
				// 同分：总分高在前，再按姓名。
				if view[a].Total != view[b].Total {
					return view[a].Total > view[b].Total
				}
				return view[a].Player < view[b].Player
			})
		}
		dark := fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark
		for _, row := range view {
			// 名次和选手名两格不可点；题目分数格可点，弹测试点明细。
			player := row.Player
			cells := []fyne.CanvasObject{
				plainCell(fmt.Sprintf("%d", row.Rank), widths[0], nil, false),
				plainCell(row.Player, widths[1], nil, true),
			}
			for i, s := range row.Scores {
				prob := p.res.Problems[i]
				col := scoreToneColor(scoreToneOf(c, prob, s), dark)
				cells = append(cells, newTapCell(fmtScore(s), widths[i+2], col, func() {
					p.showPointDetail(player, prob)
				}))
			}
			cells = append(cells, plainCell(fmtScore(row.Total), widths[len(widths)-1], nil, false))
			objs = append(objs, container.NewHBox(cells...))
		}
		box.Objects = objs
		box.Refresh()
	}
	search.OnChanged = func(s string) {
		p.rankFilter = strings.ToLower(strings.TrimSpace(s))
		rebuild()
	}
	rebuild()
	return container.NewVBox(search, box)
}

// plainCell 是榜单数据行里不可点击的格子，样式与 cellRowWithColors 一致。
func plainCell(text string, w float32, col color.Color, leading bool) fyne.CanvasObject {
	if col == nil {
		col = theme.Color(theme.ColorNameForeground)
	}
	t := canvas.NewText(text, col)
	t.TextSize = 13
	t.Alignment = fyne.TextAlignCenter
	if leading {
		t.Alignment = fyne.TextAlignLeading
	}
	return container.NewGridWrap(fyne.NewSize(w, 26), container.NewStack(container.NewCenter(t)))
}

// plainHeaderCell 是不可点击的表头格子，样式与 cellRow(header) 一致。
func plainHeaderCell(text string, w float32) fyne.CanvasObject {
	t := canvas.NewText(text, theme.Color(theme.ColorNameDisabled))
	t.TextSize = 13
	t.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewGridWrap(fyne.NewSize(w, 26), container.NewStack(container.NewCenter(t)))
}

// tapHeaderCell 是榜单表头里可以点击的格子：文字用主题强调色，
// 当前排序列高亮显示。实现 fyne.Tappable 即可接收点击。
type tapHeaderCell struct {
	widget.BaseWidget
	t     *canvas.Text
	w     float32
	onTap func()
}

func newTapHeaderCell(text string, w float32, active bool, onTap func()) *tapHeaderCell {
	col := theme.Color(theme.ColorNameDisabled)
	if active {
		col = theme.Color(theme.ColorNamePrimary)
	}
	t := canvas.NewText(text, col)
	t.TextSize = 13
	t.TextStyle = fyne.TextStyle{Bold: true}
	c := &tapHeaderCell{t: t, w: w, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapHeaderCell) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapHeaderCell) CreateRenderer() fyne.WidgetRenderer {
	return &tapHeaderCellRenderer{c: c}
}

type tapHeaderCellRenderer struct{ c *tapHeaderCell }

func (r *tapHeaderCellRenderer) Layout(size fyne.Size) {
	// 横向撑满格子让 TextAlignCenter 生效，纵向居中。
	h := r.c.t.MinSize().Height
	r.c.t.Resize(fyne.NewSize(size.Width, h))
	r.c.t.Move(fyne.NewPos(0, (size.Height-h)/2))
}

func (r *tapHeaderCellRenderer) MinSize() fyne.Size { return fyne.NewSize(r.c.w, 26) }

func (r *tapHeaderCellRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c.t} }

func (r *tapHeaderCellRenderer) Refresh() { canvas.Refresh(r.c.t) }

func (r *tapHeaderCellRenderer) Destroy() {}

// tapCell 是榜单数据行里可以点击的分数格：点开该选手该题的测试点明细。
// 外观与普通单元格一致，只有点击行为不同。
type tapCell struct {
	widget.BaseWidget
	t     *canvas.Text
	w     float32
	onTap func()
}

func newTapCell(text string, w float32, col color.Color, onTap func()) *tapCell {
	t := canvas.NewText(text, col)
	t.TextSize = 13
	t.Alignment = fyne.TextAlignCenter
	c := &tapCell{t: t, w: w, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapCell) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapCell) CreateRenderer() fyne.WidgetRenderer {
	return &tapCellRenderer{c: c}
}

type tapCellRenderer struct{ c *tapCell }

func (r *tapCellRenderer) Layout(size fyne.Size) {
	h := r.c.t.MinSize().Height
	r.c.t.Resize(fyne.NewSize(size.Width, h))
	r.c.t.Move(fyne.NewPos(0, (size.Height-h)/2))
}

func (r *tapCellRenderer) MinSize() fyne.Size { return fyne.NewSize(r.c.w, 26) }

func (r *tapCellRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c.t} }

func (r *tapCellRenderer) Refresh() { canvas.Refresh(r.c.t) }

func (r *tapCellRenderer) Destroy() {}

// showPointDetail 弹出某位选手某道题的评测明细：编译日志（编译失败时）、
// 子任务矩阵或逐测试点的判定/耗时/内存。
func (p *contestPage) showPointDetail(player, problem string) {
	pr, ok := p.res.resultOf(player, problem)
	if !ok {
		dialog.NewInformation(tr("评测明细"),
			tr("该选手此题没有评测记录。"), p.g.win).Show()
		return
	}
	full := fullScoreOf(p.c, problem)

	items := []fyne.CanvasObject{container.NewPadded(widget.NewLabelWithStyle(
		fmt.Sprintf(tr("得分 %s / %s"), fmtScore(pr.Score), fmtScore(full)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))}

	if !pr.CompileOK {
		items = append(items,
			container.NewPadded(caption(tr("编译失败："))),
			widget.NewLabelWithStyle(strings.TrimSpace(pr.CompileLog),
				fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
	} else if len(pr.Subtasks) > 0 {
		// 子任务模式：每个子任务一行小结，下面缩进列它的测试点。
		for _, st := range pr.Subtasks {
			state := tr("未通过")
			if st.Passed {
				state = tr("通过")
			}
			items = append(items, widget.NewLabelWithStyle(
				fmt.Sprintf(tr("子任务 %s · %s / %s 分 · %s"),
					st.Name, fmtScore(st.Awarded), fmtScore(st.Score), state),
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			items = append(items, pointRows(st.Tests, "  ")...)
		}
	} else {
		items = append(items, pointRows(pr.Tests, "")...)
	}

	scroll := container.NewVScroll(container.NewVBox(items...))
	scroll.SetMinSize(fyne.NewSize(520, 380))
	d := dialog.NewCustom(tr("评测明细"), tr("关闭"), scroll, p.g.win)
	d.Resize(fyne.NewSize(560, 430))
	d.Show()
}

// pointRows 把一组测试点画成紧凑的文本行，前缀用于子任务缩进。
func pointRows(tests []testOutcome, indent string) []fyne.CanvasObject {
	if len(tests) == 0 {
		return []fyne.CanvasObject{caption(indent + tr("（没有测试点记录）"))}
	}
	rows := make([]fyne.CanvasObject, 0, len(tests))
	for _, t := range tests {
		line := fmt.Sprintf("%s%s · %s · %dms · %.1fMB",
			indent, tr("测试点")+" "+t.ID, t.Verdict, t.TimeMS, t.MemMB)
		rows = append(rows, widget.NewLabelWithStyle(line,
			fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
		if t.Message != "" {
			rows = append(rows, caption(indent+"  "+shorten(t.Message, 120)))
		}
	}
	return rows
}

// problemDetail 展示一道题的子任务与测试点状态矩阵。
func (p *contestPage) problemDetail(prob contestProblem, rows []rankRow) fyne.CanvasObject {
	full := fullScoreOf(p.c, prob.Name)
	title := widget.NewLabelWithStyle(
		fmt.Sprintf(tr("%s  满分 %s 分 · %d 个测试点"), prob.Name, fmtScore(full), len(prob.Tests)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	items := []fyne.CanvasObject{container.NewPadded(title)}

	// 题面（Markdown）
	if strings.TrimSpace(prob.Statement) != "" {
		rich := widget.NewRichText()
		rich.ParseMarkdown(prob.Statement)
		rich.Wrapping = fyne.TextWrapWord
		items = append(items, container.NewPadded(rich), widget.NewSeparator())
	}

	// 子任务矩阵
	if len(prob.Subtasks) > 0 {
		headers := []string{tr("子任务"), tr("判定"), tr("分值")}
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
		headers := []string{tr("测试点"), tr("分值")}
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
		p.g.showError(fmt.Errorf("%s", tr("还没有评测结果")))
		return
	}
	def := sanitizeFileName(p.c.Cfg.Name) + tr("-排名.html")
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		outPath := w.URI().Path()
		_ = w.Close()
		path, err := exportContestHTML(p.c, p.res, outPath)
		if err != nil {
			p.g.showError(fmt.Errorf(tr("导出失败：%v"), err))
			return
		}
		p.g.setStatus(tr("榜单已导出：") + path)
		openTip := dialog.NewConfirm(tr("导出成功"), tr("榜单已写入：")+"\n"+path+"\n\n"+tr("要现在打开吗？"), func(ok bool) {
			if ok {
				if err := openPath(filepath.Dir(path)); err != nil {
					p.g.showError(err)
				}
			}
		}, p.g.win)
		openTip.SetConfirmText(tr("打开所在目录"))
		openTip.Show()
	}, p.g.win)
	save.SetFileName(def)
	save.Show()
}

// showExportMenu 弹出导出菜单：HTML / CSV / Markdown 三选一。
// 三个按钮横排会把标题挤掉，收进菜单后按钮行只剩「复制榜单」与「导出…」。
func (p *contestPage) showExportMenu() {
	if p.g == nil || p.g.win == nil || p.g.win.Canvas() == nil {
		return
	}
	menu := widget.NewPopUpMenu(fyne.NewMenu("",
		fyne.NewMenuItem(tr("HTML 榜单"), p.exportHTML),
		fyne.NewMenuItem(tr("CSV 表格"), p.exportCSV),
		fyne.NewMenuItem(tr("Markdown 文档"), p.exportMarkdown),
	), p.g.win.Canvas())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(p.exportBtn)
	x := pos.X
	// 按钮贴着右边缘，菜单往右展开会被窗口裁掉：越界时整块左移回来。
	// 用 MinSize 估算宽度——刚创建时 Size() 还是零，拿它判断等于没判。
	if w := menu.MinSize().Width; x+w > p.g.win.Canvas().Size().Width {
		x = p.g.win.Canvas().Size().Width - w - 8
	}
	menu.ShowAtPosition(fyne.NewPos(x, pos.Y+p.exportBtn.Size().Height))
}

// copyRanking 把榜单以 Markdown 文本放进剪贴板，方便直接贴到群里。
func (p *contestPage) copyRanking() {
	if !p.hasResult {
		p.g.showError(fmt.Errorf("%s", tr("还没有评测结果")))
		return
	}
	if p.g.win == nil || p.g.win.Clipboard() == nil {
		return
	}
	p.g.win.Clipboard().SetContent(rankingMarkdown(p.c, p.res))
	p.g.setStatus(tr("榜单已复制到剪贴板"))
}

// exportMarkdown 弹保存框导出 Markdown 榜单。
func (p *contestPage) exportMarkdown() {
	if !p.hasResult {
		p.g.showError(fmt.Errorf("%s", tr("还没有评测结果")))
		return
	}
	def := sanitizeFileName(p.c.Cfg.Name) + tr("-排名.md")
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		_, werr := w.Write([]byte(rankingMarkdown(p.c, p.res)))
		_ = w.Close()
		if werr != nil {
			p.g.showError(fmt.Errorf(tr("导出失败：%v"), werr))
			return
		}
		p.g.setStatus(tr("榜单 Markdown 已导出：") + w.URI().Path())
	}, p.g.win)
	save.SetFileName(def)
	save.SetFilter(storage.NewExtensionFileFilter([]string{".md"}))
	save.Show()
}

// exportCSV 弹保存框导出榜单 CSV。
func (p *contestPage) exportCSV() {
	if !p.hasResult {
		p.g.showError(fmt.Errorf("%s", tr("还没有评测结果")))
		return
	}
	def := sanitizeFileName(p.c.Cfg.Name) + tr("-排名.csv")
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		_, werr := w.Write(rankingCSV(p.c, p.res))
		_ = w.Close()
		if werr != nil {
			p.g.showError(fmt.Errorf(tr("导出失败：%v"), werr))
			return
		}
		p.g.setStatus(tr("榜单 CSV 已导出：") + w.URI().Path())
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
		// 用 Center 包一层：GridWrap 只给固定宽高，canvas.Text 自己不会
		// 垂直居中，直接用会让判定文字贴在下边缘。
		cell := container.NewStack(container.NewCenter(t))
		objs = append(objs, container.NewGridWrap(fyne.NewSize(w, 26), cell))
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
// showProjectList 打开项目库总览页（表格视图）。
func (g *guiApp) showProjectList() {
	g.generation.Add(1)
	if g.page != nil {
		g.page.stop()
		g.page = nil
	}
	if g.contest != nil {
		g.contest.stop()
		g.contest = nil
	}
	g.selected = ""
	g.mode = modeProjectList
	if g.list != nil {
		g.list.UnselectAll()
		g.list.Refresh()
	}
	g.detail.Objects = []fyne.CanvasObject{g.buildProjectList()}
	g.detail.Refresh()
	g.setStatus(tr("项目库"))
}

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
	g.setStatus(tr("模拟赛"))
}
