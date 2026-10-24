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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildContestList 是模拟赛入口页：比赛列表 + 导入按钮。
func (g *guiApp) buildContestList() fyne.CanvasObject {
	list := contestEntries()

	importBtn := widget.NewButtonWithIcon("导入模拟赛", theme.ContentAddIcon(), g.importContestDialog)
	importBtn.Importance = widget.HighImportance

	head := container.NewVBox(
		container.NewPadded(container.NewBorder(nil, nil,
			container.NewVBox(
				widget.NewLabelWithStyle("模拟赛", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				caption("整场比赛一起导入、一起评测，自动出榜单，可导出 HTML。"),
			),
			importBtn,
			nil,
		)),
		widget.NewSeparator(),
	)

	if len(list) == 0 {
		empty := container.NewCenter(container.NewVBox(
			widget.NewLabelWithStyle("还没有导入任何模拟赛", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			caption("上传目录结构：\n  <目录>/data/<题目名>/<测试点>.in\n  <目录>/data/<题目名>/<测试点>.ans（或 .out）\n  <目录>/source/<选手名>/<题目名>.cpp"),
			container.NewCenter(importBtn),
		))
		return container.NewBorder(head, nil, nil, nil, empty)
	}

	box := container.NewVBox()
	for _, c := range list {
		box.Add(g.contestListRow(c))
	}

	foot := container.NewPadded(caption(fmt.Sprintf("共 %d 场比赛 · 数据存放在 %s", len(list), contestsDir())))
	return container.NewBorder(head, foot, nil, nil, container.NewVScroll(container.NewPadded(box)))
}

// contestListRow 是比赛列表中的一行卡片。
func (g *guiApp) contestListRow(c contest) fyne.CanvasObject {
	open := widget.NewButtonWithIcon("打开", theme.NavigateNextIcon(), func() {
		g.showContest(c)
	})
	open.Importance = widget.HighImportance

	name := widget.NewLabelWithStyle(c.Cfg.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	judged := "未评测"
	if _, ok := loadContestResult(c); ok {
		judged = "已评测"
	}
	info := caption(fmt.Sprintf("%d 道题 · %d 位选手 · %d 份提交 · 满分 %s 分 · %s · %s",
		len(c.Cfg.Problems), len(c.Cfg.Players), len(c.Cfg.Sources),
		fmtScore(totalFullScore(c)), judged, c.Cfg.ImportedAt))

	remove := deleteIconButton("删除这场模拟赛", func() {
		confirm := dialog.NewConfirm("删除模拟赛",
			"将删除整场比赛及其数据：\n"+c.Dir, func(ok bool) {
				if !ok {
					return
				}
				if err := os.RemoveAll(c.Dir); err != nil {
					g.showError(err)
					return
				}
				g.showContestList()
				g.setStatus("已删除模拟赛 " + c.Cfg.Name)
			}, g.win)
		confirm.SetConfirmText("删除")
		confirm.Show()
	})

	dirBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		if err := openPath(c.Dir); err != nil {
			g.showError(err)
		}
	})

	right := container.NewHBox(open, dirBtn, remove)
	card := container.NewBorder(nil, nil, nil, right, container.NewVBox(name, info))
	return container.NewVBox(container.NewPadded(card), widget.NewSeparator())
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
	g.setStatus("已打开模拟赛 " + c.Cfg.Name)
}

// importContestDialog 选择上传目录并导入成一整场比赛。
func (g *guiApp) importContestDialog() {
	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder("上传目录（内含 data/ 与 source/）")
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("比赛名称（默认取目录名）")

	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		g.pickFolder(dirEntry, func(dir string) {
			nameEntry.SetText(filepath.Base(dir))
		})
	})
	dirRow := container.NewBorder(nil, nil, nil, browse, dirEntry)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("上传目录", dirRow),
			widget.NewFormItem("比赛名称", nameEntry),
		),
		caption("目录结构：\n  <目录>/data/<题目名>/<测试点>.in 与 .ans（或 .out）\n  <目录>/source/<选手名>/<题目名>.cpp"),
		caption("导入后数据会复制到配置目录，原目录可以删掉。"),
	)

	dialog.ShowCustomConfirm("导入模拟赛", "导入", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		dir := strings.TrimSpace(dirEntry.Text)
		name := strings.TrimSpace(nameEntry.Text)
		if dir == "" {
			g.showError(fmt.Errorf("请选择上传目录"))
			return
		}
		g.busy(true, "正在导入模拟赛 ...")
		go func() {
			c, err := importContest(dir, name)
			ui(func() {
				if err != nil {
					g.busy(false, "导入失败")
					g.showError(err)
					return
				}
				g.busy(false, "模拟赛已导入")
				g.showContest(c)
				g.setStatus(fmt.Sprintf("已导入 %s：%d 题 · %d 位选手 · %d 份提交",
					c.Cfg.Name, len(c.Cfg.Problems), len(c.Cfg.Players), len(c.Cfg.Sources)))
			})
		}()
	}, g.win)
}
