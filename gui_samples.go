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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// samplesPage 用来查看、编辑、新增与删除样例文件对。
type samplesPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	ids      []string
	list     *widget.List
	selected string

	inBox  *widget.Entry
	outBox *widget.Entry
	title  *widget.Label
	note   *widget.Label

	loading bool
}

func newSamplesPage(pg *projectPage) *samplesPage {
	s := &samplesPage{pg: pg}

	s.list = widget.NewList(
		func() int { return len(s.ids) },
		func() fyne.CanvasObject { return widget.NewLabel("#000") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(s.ids) {
				return
			}
			obj.(*widget.Label).SetText("#" + s.ids[id])
		},
	)
	s.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(s.ids) {
			return
		}
		s.selectSample(s.ids[id])
	}

	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), s.addDialog)
	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), s.deleteSelected)
	syncBtn := widget.NewButtonWithIcon("同步", theme.ViewRefreshIcon(), func() {
		res := syncSamples(s.pg.p, true)
		s.reload()
		msg := "样例库已是最新"
		if len(res.added) > 0 || len(res.removed) > 0 {
			msg = fmt.Sprintf("同步完成：新增 %d，移除 %d", len(res.added), len(res.removed))
		}
		s.pg.g.setStatus(msg)
	})

	sideTop := container.NewPadded(section("样例"))
	sideBottom := container.NewVBox(widget.NewSeparator(), container.NewPadded(container.NewHBox(addBtn, delBtn, syncBtn)))
	side := container.NewBorder(sideTop, sideBottom, nil, nil, s.list)

	s.title = strong("未选择样例")
	s.note = caption("切换样例或刷新时自动保存修改")
	s.inBox = widget.NewMultiLineEntry()
	s.inBox.SetPlaceHolder("输入数据（.in）")
	s.inBox.OnChanged = func(string) { s.note.SetText("有未保存的修改") }
	s.outBox = widget.NewMultiLineEntry()
	s.outBox.SetPlaceHolder("期望输出（.out / .ans）")
	s.outBox.OnChanged = func(string) { s.note.SetText("有未保存的修改") }

	saveBtn := widget.NewButtonWithIcon("保存样例", theme.DocumentSaveIcon(), s.save)
	inPane := container.NewBorder(caption("输入 .in"), nil, nil, nil, s.inBox)
	outPane := container.NewBorder(caption("期望输出"), nil, nil, nil, s.outBox)

	right := container.NewBorder(
		container.NewVBox(container.NewPadded(container.NewBorder(nil, nil, s.title, container.NewHBox(saveBtn), s.note)), widget.NewSeparator()),
		nil, nil, nil,
		container.NewVSplit(inPane, outPane),
	)

	split := container.NewHSplit(side, right)
	split.SetOffset(0.22)
	s.content = split

	s.reload()
	return s
}

// reload 重新读取样例索引并加载当前选中的样例。
func (s *samplesPage) reload() {
	s.loading = true
	defer func() { s.loading = false }()

	s.ids = readIndex(s.pg.p)
	s.list.Refresh()

	if len(s.ids) == 0 {
		s.selected = ""
		s.title.SetText("还没有样例")
		s.inBox.SetText("")
		s.outBox.SetText("")
		s.note.SetText("在项目目录里放入成对的 .in 与 .out/.ans 后点“同步”")
		return
	}

	idx := indexOf(s.ids, s.selected)
	if idx < 0 {
		idx = 0
	}
	s.selected = s.ids[idx]
	s.list.Select(idx)
	s.loadSample(s.selected)
}

// selectSample 切换当前编辑的样例，切换前自动保存上一组修改。
func (s *samplesPage) selectSample(id string) {
	if s.selected == id && s.title.Text != "未选择样例" {
		return
	}
	if !s.loading {
		s.save()
	}
	s.selected = id
	s.loadSample(id)
}

// loadSample 把样例内容填进编辑器。
func (s *samplesPage) loadSample(id string) {
	input, output, err := readSample(s.pg.p, id)
	s.loading = true
	s.inBox.SetText(input)
	s.outBox.SetText(output)
	s.loading = false

	s.title.SetText("#" + id)
	switch {
	case err != nil:
		s.note.SetText(fmt.Sprintf("读取不完整：%v", err))
	default:
		s.note.SetText(fmt.Sprintf("输入 %s · 输出 %s", humanSize(len(input)), humanSize(len(output))))
	}
}

// save 写回当前样例；没有选中样例时什么也不做。
func (s *samplesPage) save() {
	if s.selected == "" || s.loading {
		return
	}
	if err := writeSample(s.pg.p, s.selected, s.inBox.Text, s.outBox.Text); err != nil {
		s.pg.g.showError(err)
		return
	}
	s.note.SetText(fmt.Sprintf("已保存 · 输入 %s · 输出 %s",
		humanSize(len(s.inBox.Text)), humanSize(len(s.outBox.Text))))
}

// addDialog 新建一组空样例。
func (s *samplesPage) addDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("编号，例如 3 或 sample3")
	suggest := 1
	for _, id := range s.ids {
		var n int
		if _, err := fmt.Sscanf(strings.TrimPrefix(id, "sample"), "%d", &n); err == nil && n >= suggest {
			suggest = n + 1
		}
	}
	entry.SetText(fmt.Sprintf("%d", suggest))

	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem("样例编号", entry)),
		caption("会创建 <编号>.in 与 <编号>.out 两个文件。"),
	)

	dialog.ShowCustomConfirm("新建样例", "创建", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		id := strings.TrimSpace(entry.Text)
		if id == "" {
			return
		}
		if err := writeSample(s.pg.p, id, "", ""); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.selected = id
		s.reload()
		s.pg.g.setStatus("已新建样例 #" + id)
	}, s.pg.g.win)
}

// deleteSelected 删除当前样例的全部文件。
func (s *samplesPage) deleteSelected() {
	if s.selected == "" {
		s.pg.g.setStatus("先选择一个样例")
		return
	}
	id := s.selected
	dialog.ShowConfirm("删除样例", "将删除 #"+id+" 的 .in 与 .out/.ans 文件，确定吗？", func(ok bool) {
		if !ok {
			return
		}
		if err := deleteSample(s.pg.p, id); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.selected = ""
		s.reload()
		s.pg.g.setStatus("已删除样例 #" + id)
	}, s.pg.g.win)
}

// humanSize 把字节数转成便于阅读的单位。
func humanSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}
