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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildContestList 是模拟赛入口页：比赛列表 + 导入按钮。
func (g *guiApp) buildContestList() fyne.CanvasObject {
	list := contestEntries()

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			container.NewVBox(
				widget.NewLabelWithStyle(tr("模拟赛"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				caption(tr("整场比赛一起导入、一起评测，自动出榜单，可导出 HTML。")),
			),
			// 用 Center 包一层：Border 的右槽会把按钮拉到整块高度，
			// 两个按钮并排时会显得又高又胖。
			container.NewCenter(g.importButtons()),
			nil,
		)),
		widget.NewSeparator(),
	)

	if len(list) == 0 {
		empty := container.NewCenter(container.NewVBox(
			widget.NewLabelWithStyle(tr("还没有导入任何模拟赛"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			caption(tr("上传目录结构：")+"\n  <目录>/data/<题目名>/<测试点>.in\n  <目录>/data/<题目名>/<测试点>.ans（或 .out）\n  <目录>/source/<选手名>/<题目名>.cpp"),
			caption(tr("也可以直接导入 .zip 数据包，连时限与题面一起还原。")),
			// 必须另建一套按钮：Fyne 里同一个控件不能挂在两处，否则
			// 标题栏和这里会抢着布局，按钮直接被挤到窗口外面。
			container.NewCenter(g.importButtons()),
		))
		return container.NewBorder(head, nil, nil, nil, empty)
	}

	// 筛选行 + 表格 + 底部计数：搜索或换排序时整块重建。
	// 选中状态随表格一起销毁，重建后取消选中是预期行为。
	var center, footBar *fyne.Container
	var search *widget.Entry
	var sortSel *widget.Select
	rebuild := func() {
		filtered := filterSortContests(list, search.Text, sortSel.SelectedIndex())
		table, actions := g.contestTable(filtered)
		center.Objects = []fyne.CanvasObject{table}
		center.Refresh()
		footBar.Objects = []fyne.CanvasObject{container.NewPadded(container.NewBorder(nil, nil,
			caption(fmt.Sprintf(tr("共 %d 场比赛 · 数据存放在 %s"), len(filtered), contestsDir())),
			actions, nil))}
		footBar.Refresh()
	}
	center = container.NewStack()
	footBar = container.NewStack()
	bar, s, sel := contestFilterBar(rebuild)
	search, sortSel = s, sel

	rebuild()
	head.Add(bar)
	return container.NewBorder(head, footBar, nil, nil, center)
}

// importButtons 新建一套"导入"按钮（目录导入 + zip 导入）。
// 每次调用都新建：同一个控件不能同时挂在标题栏和空状态里。
func (g *guiApp) importButtons() *fyne.Container {
	importBtn := widget.NewButtonWithIcon(tr("导入模拟赛"), theme.ContentAddIcon(), g.importContestDialog)
	importBtn.Importance = widget.HighImportance
	zipInBtn := widget.NewButtonWithIcon(tr("导入 ZIP"), theme.DownloadIcon(), g.importContestZipDialog)
	return container.NewHBox(zipInBtn, importBtn)
}

// contestFilterBar 生成比赛列表的筛选行：名称搜索框 + 排序下拉。
// 选择变化通过 onReload 通知调用方重建列表。
func contestFilterBar(onReload func()) (fyne.CanvasObject, *widget.Entry, *widget.Select) {
	search := widget.NewEntry()
	search.SetPlaceHolder(tr("按比赛名筛选…"))
	sortSel := widget.NewSelect([]string{
		tr("导入时间（新→旧）"),
		tr("导入时间（旧→新）"),
		tr("名称（A→Z）"),
		tr("名称（Z→A）"),
	}, nil)
	sortSel.SetSelected(tr("导入时间（新→旧）"))
	sortSel.OnChanged = func(string) { onReload() }
	// 搜索框占中间撑满，排序下拉固定在右侧（宽度要给够，否则
	// 「导入时间（新→旧）」会被截成「导入时间（…」）。
	bar := container.NewBorder(nil, nil, caption(tr("筛选")),
		container.NewGridWrap(fyne.NewSize(190, 36), sortSel), search)
	return container.NewPadded(bar), search, sortSel
}

// filterSortContests 按名称子串（不区分大小写）过滤，再按指定方式排序。
// mode: 0=导入时间新→旧, 1=旧→新, 2=名称 A→Z, 3=名称 Z→A。
func filterSortContests(list []contest, query string, mode int) []contest {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]contest, 0, len(list))
	for _, c := range list {
		if q == "" || strings.Contains(strings.ToLower(c.Cfg.Name), q) {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		switch mode {
		case 1:
			if out[i].Cfg.ImportedAt != out[j].Cfg.ImportedAt {
				return out[i].Cfg.ImportedAt < out[j].Cfg.ImportedAt
			}
		case 2:
			return out[i].Cfg.Name < out[j].Cfg.Name
		case 3:
			return out[i].Cfg.Name > out[j].Cfg.Name
		default:
			if out[i].Cfg.ImportedAt != out[j].Cfg.ImportedAt {
				return out[i].Cfg.ImportedAt > out[j].Cfg.ImportedAt
			}
		}
		return out[i].Cfg.Name < out[j].Cfg.Name
	})
	return out
}

// contestTable 用表格展示所有比赛：选中一行后，底部按钮条提供打开、
// 打开目录与删除操作。返回表格本体和按钮条。
func (g *guiApp) contestTable(list []contest) (fyne.CanvasObject, fyne.CanvasObject) {
	type row struct {
		c                                contest
		name, problems, players, sources string
		status, imported                 string
	}
	rows := make([]row, len(list))
	for i, c := range list {
		status := tr("未评测")
		if _, ok := loadContestResult(c); ok {
			status = tr("已评测")
		}
		rows[i] = row{
			c:        c,
			name:     c.Cfg.Name,
			problems: fmt.Sprintf("%d", len(c.Cfg.Problems)),
			players:  fmt.Sprintf("%d", len(c.Cfg.Players)),
			sources:  fmt.Sprintf("%d", len(c.Cfg.Sources)),
			status:   status,
			imported: c.Cfg.ImportedAt,
		}
	}

	// 列宽全部固定：表头用同样宽度的格子拼出来，才能和表体对齐。
	cols := []struct {
		title string
		width float32
	}{
		{tr("比赛"), 240},
		{tr("题数"), 56},
		{tr("选手"), 56},
		{tr("提交数"), 64},
		{tr("状态"), 84},
		{tr("导入时间"), 180},
	}

	headers := container.NewHBox()
	for _, col := range cols {
		headers.Add(container.NewGridWrap(fyne.NewSize(col.width, 22),
			widget.NewLabelWithStyle(col.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})))
	}

	table := widget.NewTable(
		func() (int, int) { return len(rows), len(cols) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			r := rows[id.Row]
			switch id.Col {
			case 0:
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText(r.name)
			case 1:
				lbl.TextStyle = fyne.TextStyle{}
				lbl.SetText(r.problems)
			case 2:
				lbl.SetText(r.players)
			case 3:
				lbl.SetText(r.sources)
			case 4:
				lbl.SetText(r.status)
			case 5:
				lbl.SetText(r.imported)
			}
		},
	)
	for i, col := range cols {
		table.SetColumnWidth(i, col.width)
	}

	openBtn := widget.NewButtonWithIcon(tr("打开"), theme.NavigateNextIcon(), nil)
	openBtn.Importance = widget.HighImportance
	dirBtn := widget.NewButtonWithIcon(tr("打开目录"), theme.FolderOpenIcon(), nil)
	zipBtn := widget.NewButtonWithIcon(tr("导出 ZIP"), theme.DownloadIcon(), nil)
	dupBtn := widget.NewButtonWithIcon(tr("复制"), theme.ContentCopyIcon(), nil)
	delBtn := deleteIconButton(tr("删除这场模拟赛"), nil)
	actions := container.NewHBox(openBtn, dirBtn, zipBtn, dupBtn, delBtn)

	var selected int = -1
	refreshActions := func() {
		if selected < 0 || selected >= len(rows) {
			openBtn.Disable()
			dirBtn.Disable()
			zipBtn.Disable()
			dupBtn.Disable()
			delBtn.Disable()
			return
		}
		openBtn.Enable()
		dirBtn.Enable()
		zipBtn.Enable()
		dupBtn.Enable()
		delBtn.Enable()
	}
	refreshActions()

	openBtn.OnTapped = func() {
		if selected >= 0 && selected < len(rows) {
			g.showContest(rows[selected].c)
		}
	}
	zipBtn.OnTapped = func() {
		if selected >= 0 && selected < len(rows) {
			g.exportContestZip(rows[selected].c)
		}
	}
	dupBtn.OnTapped = func() {
		if selected >= 0 && selected < len(rows) {
			g.duplicateContestDialog(rows[selected].c)
		}
	}
	dirBtn.OnTapped = func() {
		if selected < 0 || selected >= len(rows) {
			return
		}
		if err := openPath(rows[selected].c.Dir); err != nil {
			g.showError(err)
		}
	}
	delBtn.OnTapped = func() {
		if selected < 0 || selected >= len(rows) {
			return
		}
		c := rows[selected].c
		confirm := dialog.NewConfirm(tr("删除模拟赛"),
			tr("将删除整场比赛及其数据：")+"\n"+c.Dir, func(ok bool) {
				if !ok {
					return
				}
				if err := os.RemoveAll(c.Dir); err != nil {
					g.showError(err)
					return
				}
				g.showContestList()
				g.setStatus(tr("已删除模拟赛 ") + c.Cfg.Name)
			}, g.win)
		confirm.SetConfirmText(tr("删除"))
		confirm.Show()
	}
	table.OnSelected = func(id widget.TableCellID) {
		selected = id.Row
		refreshActions()
	}
	table.OnUnselected = func(widget.TableCellID) {
		selected = -1
		refreshActions()
	}

	return container.NewBorder(headers, nil, nil, nil, table), actions
}

// showContest 打开某场比赛的工作台。
func (g *guiApp) showContest(c contest) {
	g.generation.Add(1)
	if g.page != nil {
		g.page.stop()
		g.page = nil
	}
	g.selected = ""
	g.mode = modeContest
	if g.list != nil {
		g.list.UnselectAll()
		g.list.Refresh()
	}

	cp := newContestPage(g, c)
	g.contest = cp
	g.detail.Objects = []fyne.CanvasObject{cp.content}
	g.detail.Refresh()
	g.setStatus(tr("已打开模拟赛 ") + c.Cfg.Name)
}

// importContestDialog 选择上传目录并导入成一整场比赛。
func (g *guiApp) importContestDialog() {
	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder(tr("上传目录（内含 data/ 与 source/）"))
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(tr("比赛名称（默认取目录名）"))

	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		g.pickFolder(dirEntry, func(dir string) {
			nameEntry.SetText(filepath.Base(dir))
		})
	})
	dirRow := container.NewBorder(nil, nil, nil, browse, dirEntry)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(tr("上传目录"), dirRow),
			widget.NewFormItem(tr("比赛名称"), nameEntry),
		),
		caption(tr("目录结构：")+"\n  <目录>/data/<题目名>/<测试点>.in 与 .ans（或 .out）\n  <目录>/source/<选手名>/<题目名>.cpp"),
		caption(tr("导入后数据会复制到配置目录，原目录可以删掉。")),
	)

	dialog.ShowCustomConfirm(tr("导入模拟赛"), tr("导入"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		dir := strings.TrimSpace(dirEntry.Text)
		name := strings.TrimSpace(nameEntry.Text)
		if dir == "" {
			g.showError(fmt.Errorf("%s", tr("请选择上传目录")))
			return
		}
		g.runImport(tr("正在导入模拟赛 ..."), func() (contest, error) {
			return importContest(dir, name)
		})
	}, g.win)
}

// importContestZipDialog 选择 .zip 数据包导入一整场比赛。
func (g *guiApp) importContestZipDialog() {
	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder(tr("数据包（.zip）路径"))
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(tr("比赛名称（默认用包里的名字）"))

	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		picker := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			pathEntry.SetText(uc.URI().Path())
			_ = uc.Close()
		}, g.win)
		picker.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
		picker.Show()
	})
	pathRow := container.NewBorder(nil, nil, nil, browse, pathEntry)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(tr("数据包"), pathRow),
			widget.NewFormItem(tr("比赛名称"), nameEntry),
		),
		caption(tr("数据包里带 contest.json 时，时限、子任务、题面与提交清单都会一起还原。")),
		caption(tr("评测结果不会打包，导入后需要重新评测。")),
	)

	dialog.ShowCustomConfirm(tr("导入 ZIP 数据包"), tr("导入"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		zipPath := strings.TrimSpace(pathEntry.Text)
		name := strings.TrimSpace(nameEntry.Text)
		if zipPath == "" {
			g.showError(fmt.Errorf("%s", tr("请选择数据包")))
			return
		}
		g.runImport(tr("正在解压数据包 ..."), func() (contest, error) {
			return importContestArchive(zipPath, name)
		})
	}, g.win)
}

// exportContestZip 弹出保存框，把整场比赛打包成 zip 数据包。
func (g *guiApp) exportContestZip(c contest) {
	def := sanitizeFileName(c.Cfg.Name) + ".zip"
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		out := w.URI().Path()
		// 先关掉对话框建的空文件，压缩包由 exportContestArchive 自己写。
		_ = w.Close()
		g.runExport(tr("正在打包模拟赛 ..."), func() (string, error) {
			return exportContestArchive(c, out)
		})
	}, g.win)
	save.SetFileName(def)
	save.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
	save.Show()
}

// runImport 在后台执行导入，统一处理忙碌提示、报错与成功后的跳转。
func (g *guiApp) runImport(busyMsg string, fn func() (contest, error)) {
	g.busy(true, busyMsg)
	go func() {
		c, err := fn()
		ui(func() {
			if err != nil {
				g.busy(false, tr("导入失败"))
				g.showError(err)
				return
			}
			g.busy(false, tr("模拟赛已导入"))
			g.showContest(c)
			g.setStatus(fmt.Sprintf(tr("已导入 %s：%d 题 · %d 位选手 · %d 份提交"),
				c.Cfg.Name, len(c.Cfg.Problems), len(c.Cfg.Players), len(c.Cfg.Sources)))
		})
	}()
}

// runExport 在后台执行打包导出，成功后把路径报到状态栏。
func (g *guiApp) runExport(busyMsg string, fn func() (string, error)) {
	g.busy(true, busyMsg)
	go func() {
		path, err := fn()
		ui(func() {
			if err != nil {
				g.busy(false, tr("导出失败"))
				g.showError(err)
				return
			}
			g.busy(false, tr("数据包已导出"))
			g.setStatus(tr("数据包已导出：") + path)
		})
	}()
}

// duplicateContestDialog 让用户填新名字，然后把整场比赛复制一份。
// 默认给出「<原名> 副本」，直接回车即可——这是最常用的用法。
func (g *guiApp) duplicateContestDialog(c contest) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(c.Cfg.Name + " " + tr("副本"))

	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(tr("新名称"), nameEntry)),
		caption(tr("题目配置、测试数据与选手源码都会复制一份。")),
		caption(tr("评测结果不复制：新比赛要重新评测。")),
	)

	dialog.ShowCustomConfirm(tr("复制模拟赛"), tr("复制"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		g.busy(true, tr("正在复制模拟赛 ..."))
		go func() {
			dup, err := duplicateContest(c, name)
			ui(func() {
				if err != nil {
					g.busy(false, tr("复制失败"))
					g.showError(err)
					return
				}
				g.busy(false, tr("模拟赛已复制"))
				g.showContestList()
				g.setStatus(tr("已复制为 ") + dup.Cfg.Name)
			})
		}()
	}, g.win)
}
