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
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 模拟赛配置编辑器：整套比赛中每道题的时限、内存、测试点分值与子任务
// 都在这里用控件改，改完写回 contest.json（不再需要手改 JSON）。

// contestEditor 是「题目配置」页的状态。
type contestEditor struct {
	page    *contestPage
	content fyne.CanvasObject

	probList  *widget.List
	probNames []string
	selected  int // 当前选中的题目下标，-1 表示未选

	detail     *fyne.Container
	titleLabel *widget.Label
	subtaskBox *fyne.Container
	testBox    *fyne.Container
	scoreSum   *widget.Label
	warnLabel  *widget.Label

	subtaskRows []*subtaskRow
	testRows    []*testRow
}

// subtaskRow 是一个子任务的编辑行。
type subtaskRow struct {
	name    *widget.Entry
	score   *widget.Entry
	mode    *widget.Select
	tests   *widget.Entry
	summary *widget.Label
	obj     fyne.CanvasObject
}

// testRow 是一个测试点的编辑行。
type testRow struct {
	id      string
	score   *widget.Entry
	subtask *widget.Label
	obj     fyne.CanvasObject
}

// newContestEditor 构建配置编辑器。
func newContestEditor(p *contestPage) *contestEditor {
	e := &contestEditor{page: p, selected: -1}

	e.probList = widget.NewList(
		func() int { return len(e.probNames) },
		func() fyne.CanvasObject { return newSimpleRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(e.probNames) {
				return
			}
			name := e.probNames[id]
			sub := ""
			if prob, ok := p.c.problemOf(name); ok {
				sub = fmt.Sprintf("%d 测试点 · 满分 %s 分", len(prob.Tests), fmtScore(fullScoreOf(p.c, name)))
			}
			obj.(*simpleRow).update(name, sub, id == e.selected)
		},
	)
	e.probList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(e.probNames) {
			return
		}
		e.selected = id
		e.showProblem()
	}

	e.titleLabel = widget.NewLabelWithStyle("选择左侧的题目", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	e.titleLabel.SizeName = theme.SizeNameSubHeadingText
	e.subtaskBox = container.NewVBox()
	e.testBox = container.NewVBox()
	e.scoreSum = caption("")
	e.warnLabel = hint("")
	e.warnLabel.Hide()

	e.detail = container.NewStack()
	e.content = container.NewHSplit(
		e.buildLeftPane(),
		container.NewVScroll(container.NewPadded(e.detail)),
	)
	e.content.(*container.Split).SetOffset(0.24)
	e.reload()
	return e
}

// buildLeftPane 是左侧的题目列表与全局操作。
func (e *contestEditor) buildLeftPane() fyne.CanvasObject {
	scanBtn := widget.NewButtonWithIcon("重新扫描测试点", theme.ViewRefreshIcon(), e.page.rescan)
	regenBtn := widget.NewButtonWithIcon("自动生成子任务", theme.ContentAddIcon(), e.page.autoSubtasks)
	fullBtn := widget.NewButtonWithIcon("每题补足 100 分", theme.ConfirmIcon(), e.page.fillScores)

	head := container.NewVBox(
		container.NewPadded(caption("题目")),
		widget.NewSeparator(),
	)
	foot := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(scanBtn, regenBtn, fullBtn)),
	)
	return container.NewBorder(head, foot, nil, nil, e.probList)
}

// reload 从比赛配置重新构建整页。
func (e *contestEditor) reload() {
	e.probNames = e.probNames[:0]
	for _, prob := range e.page.c.Cfg.Problems {
		e.probNames = append(e.probNames, prob.Name)
	}
	if e.selected >= len(e.probNames) {
		e.selected = len(e.probNames) - 1
	}
	if e.probList != nil {
		e.probList.UnselectAll()
		e.probList.Refresh()
		if e.selected >= 0 {
			e.probList.Select(e.selected)
		}
	}
	if e.selected < 0 {
		e.titleLabel.SetText("选择左侧的题目")
		e.detail.Objects = []fyne.CanvasObject{
			container.NewPadded(caption("这场比赛没有可配置的题目。点左下角“重新扫描测试点”从 data/ 目录重新读取。")),
		}
		e.detail.Refresh()
		return
	}
	e.showProblem()
}

// showProblem 渲染当前选中题目的编辑器。
func (e *contestEditor) showProblem() {
	p := e.page
	if e.selected < 0 || e.selected >= len(p.c.Cfg.Problems) {
		return
	}
	prob := p.c.Cfg.Problems[e.selected]
	e.titleLabel.SetText(fmt.Sprintf("%s（满分 %s 分）", prob.Name, fmtScore(fullScoreOf(p.c, prob.Name))))

	// 基本属性
	titleIn := widget.NewEntry()
	titleIn.SetText(prob.Title)
	titleIn.SetPlaceHolder(prob.Name)
	titleIn.OnChanged = func(v string) {
		p.updateProblem(e.selected, func(pr *contestProblem) { pr.Title = strings.TrimSpace(v) })
	}

	timeIn := widget.NewEntry()
	timeIn.SetText(fmt.Sprintf("%.2f", float64(prob.TimeMS)/1000))
	timeIn.Validator = positiveNumber
	timeIn.OnChanged = func(v string) {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f > 0 {
			p.updateProblem(e.selected, func(pr *contestProblem) { pr.TimeMS = int64(f * 1000) })
		}
	}

	memIn := widget.NewEntry()
	memIn.SetText(fmtScore(prob.MemMB))
	memIn.Validator = positiveNumber
	memIn.OnChanged = func(v string) {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f > 0 {
			p.updateProblem(e.selected, func(pr *contestProblem) { pr.MemMB = f })
		}
	}

	// 文件读写题：程序从文件读入、向文件写出（freopen）。
	ioChk := widget.NewCheck("文件读写题", nil)
	ioChk.SetChecked(prob.IOFile)
	ioChk.OnChanged = func(v bool) {
		p.updateProblem(e.selected, func(pr *contestProblem) { pr.IOFile = v })
	}
	ioIn := widget.NewEntry()
	ioIn.SetText(prob.IOIn)
	ioIn.SetPlaceHolder(prob.Name + ".in")
	ioIn.OnChanged = func(v string) {
		p.updateProblem(e.selected, func(pr *contestProblem) { pr.IOIn = strings.TrimSpace(v) })
	}
	ioOut := widget.NewEntry()
	ioOut.SetText(prob.IOOut)
	ioOut.SetPlaceHolder(prob.Name + ".out")
	ioOut.OnChanged = func(v string) {
		p.updateProblem(e.selected, func(pr *contestProblem) { pr.IOOut = strings.TrimSpace(v) })
	}

	e.buildSubtasks(prob)
	e.buildTests(prob)
	e.refreshSummary()

	basic := container.NewVBox(
		section("基本信息"),
		widget.NewForm(
			widget.NewFormItem("显示标题", titleIn),
			widget.NewFormItem("时限（秒）", timeIn),
			widget.NewFormItem("内存（MB）", memIn),
			widget.NewFormItem("输入输出", container.NewVBox(ioChk, ioIn, ioOut)),
		),
		e.scoreSum,
		widget.NewSeparator(),
		section("子任务"),
		caption("子任务决定一道题的分值构成。没有子任务时按测试点分值直接累加。"),
		e.subtaskBox,
		widget.NewSeparator(),
		section("测试点"),
		caption("测试点来自 data/<题目>/ 下的 .in 与 .ans/.out，分值可逐点调整。"),
		e.testBox,
	)
	e.detail.Objects = []fyne.CanvasObject{basic}
	e.detail.Refresh()
}

// buildSubtasks 生成子任务编辑区。
func (e *contestEditor) buildSubtasks(prob contestProblem) {
	e.subtaskBox.Objects = nil
	e.subtaskRows = nil

	if len(prob.Subtasks) == 0 {
		hint := caption("当前没有子任务，每个测试点独立计分。点下方按钮可以让每个测试点变成一个捆绑子任务。")
		add := widget.NewButtonWithIcon("按测试点生成子任务", theme.ContentAddIcon(), e.page.autoSubtasks)
		e.subtaskBox.Add(container.NewVBox(hint, container.NewHBox(add)))
		e.subtaskBox.Refresh()
		return
	}

	head := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(150, 28), caption("子任务名")),
		container.NewHBox(
			container.NewGridWrap(fyne.NewSize(70, 28), caption("分值")),
			container.NewGridWrap(fyne.NewSize(110, 28), caption("判定方式")),
			container.NewGridWrap(fyne.NewSize(52, 28), caption("操作")),
		),
		container.NewGridWrap(fyne.NewSize(200, 28), caption("包含测试点（逗号分隔）")),
	)
	e.subtaskBox.Add(head)

	for i, st := range prob.Subtasks {
		idx := i
		row := &subtaskRow{}

		row.name = widget.NewEntry()
		row.name.SetText(st.Name)
		row.name.OnChanged = func(v string) {
			e.page.updateProblem(e.selected, func(pr *contestProblem) {
				if idx < len(pr.Subtasks) {
					pr.Subtasks[idx].Name = v
				}
			})
		}

		row.score = widget.NewEntry()
		row.score.SetText(fmtScore(st.Score))
		row.score.Validator = positiveNumber
		row.score.OnChanged = func(v string) {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f < 0 {
				return
			}
			e.page.updateProblem(e.selected, func(pr *contestProblem) {
				if idx < len(pr.Subtasks) {
					pr.Subtasks[idx].Score = f
				}
			})
			e.refreshSummary()
		}

		row.mode = widget.NewSelect([]string{"捆绑（全部通过）", "任意通过", "按比例给分"}, nil)
		row.mode.SetSelected(modeLabelOf(st.Mode))
		row.mode.OnChanged = func(v string) {
			e.page.updateProblem(e.selected, func(pr *contestProblem) {
				if idx < len(pr.Subtasks) {
					pr.Subtasks[idx].Mode = modeOfLabel(v)
				}
			})
			e.refreshSummary()
		}

		row.tests = widget.NewEntry()
		row.tests.SetText(strings.Join(st.Tests, ", "))
		row.tests.SetPlaceHolder("1, 2, 3")
		row.tests.OnChanged = func(v string) {
			ids := splitIDs(v)
			e.page.updateProblem(e.selected, func(pr *contestProblem) {
				if idx < len(pr.Subtasks) {
					pr.Subtasks[idx].Tests = ids
				}
			})
			e.refreshSummary()
			e.refreshTestHints()
		}

		del := deleteIconButton("删除这个子任务", func() {
			e.page.removeSubtask(e.selected, idx)
			e.reload()
		})

		line := container.NewBorder(nil, nil,
			container.NewGridWrap(fyne.NewSize(150, 34), row.name),
			container.NewHBox(
				container.NewGridWrap(fyne.NewSize(70, 34), row.score),
				container.NewGridWrap(fyne.NewSize(110, 34), row.mode),
				del,
			),
			container.NewGridWrap(fyne.NewSize(200, 34), row.tests),
		)
		row.obj = line
		e.subtaskRows = append(e.subtaskRows, row)
		e.subtaskBox.Add(line)
	}

	addBtn := widget.NewButtonWithIcon("添加子任务", theme.ContentAddIcon(), func() {
		e.page.addSubtask(e.selected)
		e.reload()
	})
	e.subtaskBox.Add(container.NewVBox(widget.NewSeparator(), container.NewHBox(addBtn)))
	e.subtaskBox.Refresh()
}

// buildTests 生成测试点编辑区。
func (e *contestEditor) buildTests(prob contestProblem) {
	e.testBox.Objects = nil
	e.testRows = nil

	if len(prob.Tests) == 0 {
		e.testBox.Add(caption("这道题在 data/ 下没有成对的 .in 与 .ans/.out。"))
		e.testBox.Refresh()
		return
	}

	for i, t := range prob.Tests {
		idx := i
		row := &testRow{id: t.ID}

		row.score = widget.NewEntry()
		row.score.SetText(fmtScore(t.Score))
		row.score.Validator = positiveNumber
		row.score.OnChanged = func(v string) {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f < 0 {
				return
			}
			e.page.updateProblem(e.selected, func(pr *contestProblem) {
				if idx < len(pr.Tests) {
					pr.Tests[idx].Score = f
				}
			})
			e.refreshSummary()
		}

		row.subtask = caption("")
		line := container.NewBorder(nil, nil,
			container.NewGridWrap(fyne.NewSize(120, 34),
				widget.NewLabelWithStyle(t.ID, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})),
			container.NewGridWrap(fyne.NewSize(90, 34), row.score),
			container.NewGridWrap(fyne.NewSize(60, 34), caption("分")),
			container.NewGridWrap(fyne.NewSize(160, 34), row.subtask),
		)
		row.obj = line
		e.testRows = append(e.testRows, row)
		e.testBox.Add(line)
	}
	e.testBox.Refresh()
	e.refreshTestHints()
}

// refreshTestHints 在每个测试点后标注它属于哪个子任务。
func (e *contestEditor) refreshTestHints() {
	p := e.page
	if e.selected < 0 || e.selected >= len(p.c.Cfg.Problems) {
		return
	}
	prob := p.c.Cfg.Problems[e.selected]
	owner := map[string]string{}
	for _, st := range prob.Subtasks {
		for _, id := range st.Tests {
			if owner[id] == "" {
				owner[id] = st.Name
			}
		}
	}
	for _, row := range e.testRows {
		if name, ok := owner[row.id]; ok {
			row.subtask.SetText("属于 " + name)
		} else if len(prob.Subtasks) > 0 {
			row.subtask.SetText("未分配给子任务")
		} else {
			row.subtask.SetText("独立计分")
		}
	}
}

// refreshSummary 更新满分与配置告警。
func (e *contestEditor) refreshSummary() {
	p := e.page
	if e.selected < 0 || e.selected >= len(p.c.Cfg.Problems) {
		return
	}
	prob := p.c.Cfg.Problems[e.selected]
	full := fullScoreOf(p.c, prob.Name)
	e.scoreSum.SetText(fmt.Sprintf("本题满分 %s 分", fmtScore(full)))

	var warns []string
	if len(prob.Subtasks) == 0 {
		var sum float64
		for _, t := range prob.Tests {
			sum += t.Score
		}
		if sum <= 0 {
			warns = append(warns, "所有测试点分值都是 0，做对了也拿不到分。")
		}
	} else {
		testIDs := map[string]bool{}
		for _, t := range prob.Tests {
			testIDs[t.ID] = true
		}
		for _, st := range prob.Subtasks {
			if len(st.Tests) == 0 {
				warns = append(warns, fmt.Sprintf("子任务「%s」没有包含任何测试点。", st.Name))
			}
			for _, id := range st.Tests {
				if !testIDs[id] {
					warns = append(warns, fmt.Sprintf("子任务「%s」引用了不存在的测试点 %s。", st.Name, id))
				}
			}
		}
	}
	if len(warns) == 0 {
		e.warnLabel.Hide()
		return
	}
	e.warnLabel.SetText(strings.Join(warns, "\n"))
	e.warnLabel.Show()
}

// modeLabelOf / modeOfLabel 在内部取值与界面文案之间转换。
func modeLabelOf(mode string) string {
	switch mode {
	case subtaskAny:
		return "任意通过"
	case subtaskSum:
		return "按比例给分"
	default:
		return "捆绑（全部通过）"
	}
}

func modeOfLabel(label string) string {
	switch label {
	case "任意通过":
		return subtaskAny
	case "按比例给分":
		return subtaskSum
	default:
		return subtaskAll
	}
}

// splitIDs 把 "1, 2,3" 这样的输入拆成测试点列表。
func splitIDs(text string) []string {
	var ids []string
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '，' || r == ' ' || r == '\t'
	}) {
		if p := strings.TrimSpace(part); p != "" {
			ids = append(ids, p)
		}
	}
	return ids
}

// positiveNumber 是数值输入框的校验器。
func positiveNumber(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil // 输入过程中允许为空
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("请输入数字")
	}
	if f < 0 {
		return fmt.Errorf("不能为负数")
	}
	return nil
}

// ---- 以下是 contestPage 上的配置增删改（统一写回 contest.json） ----

// updateProblem 修改指定题目的配置并延迟落盘。
func (p *contestPage) updateProblem(idx int, mutate func(*contestProblem)) {
	if idx < 0 || idx >= len(p.c.Cfg.Problems) {
		return
	}
	mutate(&p.c.Cfg.Problems[idx])
	p.scheduleSave()
	// 界面上跟着更新的部分：满分与告警。
	if p.editor != nil {
		p.editor.refreshSummary()
		p.editor.refreshTestHints()
		p.refreshMeta()
	}
}

// addSubtask 给某道题追加一个空子任务。
func (p *contestPage) addSubtask(probIdx int) {
	if probIdx < 0 || probIdx >= len(p.c.Cfg.Problems) {
		return
	}
	prob := &p.c.Cfg.Problems[probIdx]
	prob.Subtasks = append(prob.Subtasks, contestSubtask{
		Name:  fmt.Sprintf("Sub%d", len(prob.Subtasks)+1),
		Score: 10,
		Mode:  subtaskAll,
		Tests: nil,
	})
	p.persist()
}

// removeSubtask 删除一个子任务。
func (p *contestPage) removeSubtask(probIdx, subIdx int) {
	if probIdx < 0 || probIdx >= len(p.c.Cfg.Problems) {
		return
	}
	prob := &p.c.Cfg.Problems[probIdx]
	if subIdx < 0 || subIdx >= len(prob.Subtasks) {
		return
	}
	prob.Subtasks = append(prob.Subtasks[:subIdx], prob.Subtasks[subIdx+1:]...)
	p.persist()
}

// scheduleSave 在下一次事件循环里合并写盘：连续打字不会产生几十次写文件。
func (p *contestPage) scheduleSave() {
	if p.savePending.CompareAndSwap(false, true) {
		time.AfterFunc(300*time.Millisecond, func() {
			p.savePending.Store(false)
			p.persist()
		})
	}
	if p.g != nil {
		p.g.setStatus("配置修改中，稍后自动保存 ...")
	}
}

// flushSave 立即执行尚未落盘的修改（切页、开始评测、单元测试用）。
func (p *contestPage) flushSave() {
	if p.savePending.Swap(false) {
		p.persist()
	}
}

// persist 把当前配置写回 contest.json。
func (p *contestPage) persist() {
	if err := p.c.save(); err != nil {
		if p.g != nil {
			p.g.showError(fmt.Errorf("保存配置失败：%v", err))
		}
		return
	}
	p.warns = validateContest(p.c)
	if p.g != nil {
		p.g.setStatus("比赛配置已保存")
	}
}

// refreshMeta 刷新标题行里的题目数、满分等汇总信息。
func (p *contestPage) refreshMeta() {
	if p.meta == nil {
		return
	}
	p.meta.SetText(fmt.Sprintf("%d 题 · %d 位选手 · %d 份提交 · 满分 %s · 并发 %d · %s",
		len(p.c.Cfg.Problems), len(p.c.Cfg.Players), len(p.c.Cfg.Sources),
		fmtScore(totalFullScore(p.c)), contestParallelism(), p.c.Cfg.ImportedAt))
}

// simpleRow 是配置编辑器左侧的题目列表行。
type simpleRow struct {
	widget.BaseWidget
	name *widget.Label
	sub  *widget.Label
}

func newSimpleRow() *simpleRow {
	r := &simpleRow{name: strong(""), sub: caption("")}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.sub.Truncation = fyne.TextTruncateEllipsis
	r.ExtendBaseWidget(r)
	return r
}

func (r *simpleRow) update(name, sub string, selected bool) {
	r.name.SetText(name)
	r.sub.SetText(sub)
	if selected {
		r.name.Importance = widget.HighImportance
	} else {
		r.name.Importance = widget.MediumImportance
	}
}

func (r *simpleRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(container.NewVBox(r.name, r.sub)))
}

// sortedIDs 返回排序后的测试点编号（供自动生成子任务时使用）。
func sortedIDs(tests []testPoint) []string {
	ids := make([]string, 0, len(tests))
	for _, t := range tests {
		ids = append(ids, t.ID)
	}
	sort.Strings(ids)
	return ids
}

var _ = layout.NewSpacer
