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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 项目库总览页：把所有登记过的项目摊成一张表，可按列排序、按名字筛选，
// 选中后可以直接打开、打包、存为模板或移除。
//
// 为什么单独做一页而不是把左侧列表换成表格：侧栏只占窗口 25%（约 300px），
// 五六列塞进去每列只剩几十像素，等于什么都看不清。侧栏继续当"快速切换"，
// 这里当"管理视图"。

// projectMeta 是项目库一行的元数据，全部来自项目目录里的文件。
type projectMeta struct {
	Dir       string
	Name      string
	Samples   int
	TimeLimit string
	MemLimit  string
	LastAt    time.Time
	LastScore string
	Missing   bool
}

// scanProjectMeta 读取一个项目的元数据。目录不存在时只保留名字与路径，
// 其余留空——项目库里的记录可能指向已被删掉的目录。
func scanProjectMeta(dir string) projectMeta {
	m := projectMeta{Dir: dir, Name: projectNameOf(dir), TimeLimit: "-", MemLimit: "-"}
	if !dirExists(dir) {
		m.Missing = true
		return m
	}
	p := project{Name: m.Name, Dir: dir}
	props := p.cfg()
	m.TimeLimit = propsOr(props, "time_limit", "1.00")
	m.MemLimit = propsOr(props, "memory_limit", "125")
	m.Samples = len(readIndex(p))
	if h := readHistory(p, 1); len(h) > 0 {
		last := h[len(h)-1]
		if t, err := time.Parse("2006-01-02 15:04:05", last.Time); err == nil {
			m.LastAt = t
		}
		m.LastScore = fmt.Sprintf("%d/%d", last.Passed, last.Total)
	}
	return m
}

// lastText 返回"最近评测"列的文本。
func (m projectMeta) lastText() string {
	if m.LastAt.IsZero() {
		return ""
	}
	return m.LastAt.Format("01-02 15:04") + " · " + m.LastScore
}

// statusText 返回"状态"列文本。
func (m projectMeta) statusText() string {
	switch {
	case m.Missing:
		return tr("目录缺失")
	case !m.LastAt.IsZero():
		return tr("已评测")
	}
	return tr("未评测")
}

// 排序列。
const (
	projSortName = iota
	projSortDir
	projSortSamples
	projSortLimit
	projSortRecent
	projSortStatus
)

// sortProjectMetas 按指定列排序；desc 为真时倒序。
// 用稳定排序：同值保持注册顺序，反复点表头也不会看出抖动。
func sortProjectMetas(list []projectMeta, col int, desc bool) {
	less := func(a, b projectMeta) bool {
		switch col {
		case projSortDir:
			return strings.ToLower(a.Dir) < strings.ToLower(b.Dir)
		case projSortSamples:
			return a.Samples < b.Samples
		case projSortLimit:
			return propFloat(a.TimeLimit) < propFloat(b.TimeLimit)
		case projSortRecent:
			return a.LastAt.Before(b.LastAt)
		case projSortStatus:
			return a.LastAt.Before(b.LastAt)
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}

	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		// 「最近评测」和「状态」有优先级语义，不跟着正序/倒序翻：
		// 没评测过的永远沉底，目录缺失的永远冒头。否则一按倒序，
		// 一堆没评过的项目就顶到第一屏，反而更难找。
		if col == projSortRecent || col == projSortStatus {
			if pa, pb := pinClass(a, col), pinClass(b, col); pa != pb {
				return pa < pb
			}
		}
		if desc {
			return less(b, a)
		}
		return less(a, b)
	})
}

// pinClass 返回某行在"固定端"排序里的分组：0 组永远在前，1 组在后。
func pinClass(m projectMeta, col int) int {
	if col == projSortRecent {
		if m.LastAt.IsZero() {
			return 1 // 没评测过 → 沉底
		}
		return 0
	}
	if m.Missing {
		return 0 // 目录缺失 → 冒头
	}
	return 1
}

// propFloat 把配置里的数值解析成 float；解析不了当 0。
func propFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// buildProjectList 构建项目库页。
func (g *guiApp) buildProjectList() fyne.CanvasObject {
	dirs := append([]string{}, g.dirs...)

	// 先放占位行，后台扫完元数据再替换：几百个项目逐个读 .otk 会明显卡 UI。
	metas := make([]projectMeta, len(dirs))
	for i, d := range dirs {
		metas[i] = projectMeta{Dir: d, Name: projectNameOf(d), TimeLimit: "-", MemLimit: "-"}
	}

	var search *widget.Entry
	search = widget.NewEntry()
	search.SetPlaceHolder(tr("按项目名或路径筛选…"))
	search.SetIcon(theme.SearchIcon())

	sortCol := projSortName
	sortDesc := false

	// 表头点击要重建自身，只能先声明再赋值（闭包里引用自己是未定义）。
	var rebuild func()
	var center, foot *fyne.Container
	rebuild = func() {
		shown := filterProjectMetas(metas, search.Text)
		sortProjectMetas(shown, sortCol, sortDesc)
		table, actions := g.projectTable(shown, sortCol, sortDesc, func(col int) {
			if col == sortCol {
				sortDesc = !sortDesc
			} else {
				sortCol, sortDesc = col, false
			}
			rebuild()
		})
		center.Objects = []fyne.CanvasObject{table}
		center.Refresh()
		foot.Objects = []fyne.CanvasObject{container.NewPadded(container.NewBorder(nil, nil,
			caption(fmt.Sprintf(tr("共 %d 个项目 · 记录在 %s"), len(shown), registryPath())),
			actions, nil))}
		foot.Refresh()
	}
	center = container.NewStack()
	foot = container.NewStack()

	search.OnChanged = func(string) { rebuild() }

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			container.NewVBox(
				widget.NewLabelWithStyle(tr("项目库"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				caption(tr("所有登记过的项目一张表：点表头排序，选中一行后可打开、打包或存为模板。")),
			),
			container.NewCenter(container.NewHBox(
				widget.NewButtonWithIcon(tr("批量评测"), theme.MediaPlayIcon(), g.batchJudgeDialog),
				g.newProjectButton(),
			)),
			nil,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewBorder(nil, nil, caption(tr("筛选")), nil, search)),
	)

	if len(dirs) == 0 {
		empty := container.NewCenter(container.NewVBox(
			widget.NewLabelWithStyle(tr("项目库还是空的"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			caption(tr("新建一个项目，或者从网站添加题目——建好的项目都会出现在这张表里。")),
			container.NewCenter(g.newProjectButton()),
		))
		return container.NewBorder(head, nil, nil, nil, empty)
	}

	rebuild()

	// 后台扫描元数据。扫完若用户已经离开本页，更新的是一个没人看的对象，
	// 无副作用（ui() 里只会刷新这些格子）。
	go func() {
		full := make([]projectMeta, len(dirs))
		for i, d := range dirs {
			full[i] = scanProjectMeta(d)
		}
		ui(func() {
			metas = full
			rebuild()
		})
	}()

	return container.NewBorder(head, foot, nil, nil, center)
}

// newProjectButton 每次新建一个"新建项目"按钮：同一个控件不能挂两处。
func (g *guiApp) newProjectButton() *widget.Button {
	b := widget.NewButtonWithIcon(tr("新建项目"), theme.ContentAddIcon(), g.newProjectDialog)
	b.Importance = widget.HighImportance
	return b
}

// missingDash 在目录缺失时用破折号代替具体数值。
func missingDash(missing bool, text string) string {
	if missing {
		return "—"
	}
	return text
}

// filterProjectMetas 按名称或路径子串（忽略大小写）过滤。
func filterProjectMetas(list []projectMeta, keyword string) []projectMeta {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	out := make([]projectMeta, 0, len(list))
	for _, m := range list {
		if keyword == "" ||
			strings.Contains(strings.ToLower(m.Name), keyword) ||
			strings.Contains(strings.ToLower(m.Dir), keyword) {
			out = append(out, m)
		}
	}
	return out
}

// projectTable 生成表格与底部操作条。onSort 由表头点击触发，参数是列号。
func (g *guiApp) projectTable(rows []projectMeta, sortCol int, sortDesc bool,
	onSort func(int)) (fyne.CanvasObject, fyne.CanvasObject) {

	cols := []struct {
		title string
		width float32
		sort  int
	}{
		{tr("项目"), 164, projSortName},
		{tr("目录"), 300, projSortDir},
		{tr("样例"), 56, projSortSamples},
		{tr("时限 / 内存"), 124, projSortLimit},
		{tr("最近评测"), 132, projSortRecent},
		{tr("状态"), 76, projSortStatus},
	}

	headers := container.NewHBox()
	for _, col := range cols {
		title := col.title
		if col.sort == sortCol {
			if sortDesc {
				title += " ▼"
			} else {
				title += " ▲"
			}
		}
		col := col
		// 外面套一层 GridWrap 固定宽度：tapHeaderCell 自己虽然报了 MinSize，
		// 但 HBox 里各格子的实际宽度只认自己的最小值，和表格列宽容易差几像素，
		// 一列一列错开后就完全对不上了。
		headers.Add(container.NewGridWrap(fyne.NewSize(col.width, 26),
			newTapHeaderCell(title, col.width, col.sort == sortCol, func() {
				onSort(col.sort)
			})))
	}

	table := widget.NewTable(
		func() (int, int) { return len(rows), len(cols) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			// 单元格不换行、超出就省略：目录很长，不截断会糊到隔壁列上。
			lbl.Wrapping = fyne.TextWrapOff
			lbl.Truncation = fyne.TextTruncateEllipsis
			return lbl
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			if id.Row < 0 || id.Row >= len(rows) {
				lbl.SetText("")
				return
			}
			m := rows[id.Row]
			lbl.TextStyle = fyne.TextStyle{}
			lbl.Importance = widget.MediumImportance
			switch id.Col {
			case 0:
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText(m.Name)
			case 1:
				lbl.SetText(m.Dir)
			case 2:
				// 目录都没了，样例数没有意义，别拿 0 冒充。
				lbl.SetText(missingDash(m.Missing, strconv.Itoa(m.Samples)))
			case 3:
				lbl.SetText(missingDash(m.Missing, m.TimeLimit+"s / "+m.MemLimit+"MB"))
			case 4:
				lbl.SetText(m.lastText())
			case 5:
				// 先改 Importance 再 SetText：SetText 才会触发刷新去取新颜色，
				// 反过来的话这一格要等下一次刷新才变色。
				if m.Missing {
					lbl.Importance = widget.DangerImportance
				}
				lbl.SetText(m.statusText())
			}
		},
	)
	for i, col := range cols {
		table.SetColumnWidth(i, col.width)
	}

	openBtn := widget.NewButtonWithIcon(tr("打开"), theme.NavigateNextIcon(), nil)
	openBtn.Importance = widget.HighImportance
	dirBtn := widget.NewButtonWithIcon(tr("打开目录"), theme.FolderOpenIcon(), nil)
	packBtn := widget.NewButtonWithIcon(tr("打包 ZIP"), theme.DownloadIcon(), nil)
	tplBtn := widget.NewButtonWithIcon(tr("存为模板"), theme.ContentCopyIcon(), nil)
	delBtn := deleteIconButton(tr("移除项目…"), nil)
	actions := container.NewHBox(openBtn, dirBtn, packBtn, tplBtn, delBtn)

	selected := -1
	syncActions := func() {
		on := selected >= 0 && selected < len(rows)
		for _, b := range []*widget.Button{openBtn, dirBtn, packBtn, tplBtn, delBtn} {
			if on {
				b.Enable()
			} else {
				b.Disable()
			}
		}
	}
	syncActions()

	current := func() (projectMeta, bool) {
		if selected >= 0 && selected < len(rows) {
			return rows[selected], true
		}
		return projectMeta{}, false
	}

	openBtn.OnTapped = func() {
		if m, ok := current(); ok {
			g.showProject(m.Dir)
		}
	}
	dirBtn.OnTapped = func() {
		if m, ok := current(); ok {
			if err := openPath(m.Dir); err != nil {
				g.showError(err)
			}
		}
	}
	packBtn.OnTapped = func() {
		if m, ok := current(); ok {
			g.exportProjectZip(project{Name: m.Name, Dir: m.Dir})
		}
	}
	tplBtn.OnTapped = func() {
		if m, ok := current(); ok {
			g.saveAsTemplateDialog(project{Name: m.Name, Dir: m.Dir})
		}
	}
	delBtn.OnTapped = func() {
		if m, ok := current(); ok {
			g.removeProjectDialog(project{Name: m.Name, Dir: m.Dir})
		}
	}

	table.OnSelected = func(id widget.TableCellID) {
		if id.Row < 0 || id.Row >= len(rows) {
			return
		}
		selected = id.Row
		syncActions()
	}
	table.OnUnselected = func(widget.TableCellID) {
		selected = -1
		syncActions()
	}

	return container.NewBorder(headers, nil, nil, nil, table), actions
}
