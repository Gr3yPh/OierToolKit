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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// sourcePage 是内置的源码编辑器，等价于 otk edit，但不用离开窗口。
type sourcePage struct {
	pg      *projectPage
	content fyne.CanvasObject

	path   *widget.Label
	editor *widget.Entry
	note   *widget.Label

	loaded string
}

func newSourcePage(pg *projectPage) *sourcePage {
	s := &sourcePage{pg: pg}

	s.path = hint("")
	s.note = caption("")
	s.editor = widget.NewMultiLineEntry()
	s.editor.Wrapping = fyne.TextWrapOff
	s.editor.OnChanged = s.onChanged

	saveBtn := widget.NewButtonWithIcon(tr("保存"), theme.DocumentSaveIcon(), s.save)
	reloadBtn := widget.NewButtonWithIcon(tr("重新加载"), theme.ViewRefreshIcon(), s.reload)
	extBtn := widget.NewButtonWithIcon(tr("外部编辑器"), theme.DocumentCreateIcon(), func() {
		if err := launchEditor(pg.p); err != nil {
			pg.g.showError(err)
		}
	})
	gdbBtn := widget.NewButtonWithIcon(tr("gdb 调试"), theme.ComputerIcon(), func() {
		if err := launchGdb(pg.p); err != nil {
			pg.g.showError(err)
		}
	})
	statsBtn := widget.NewButtonWithIcon(tr("统计"), theme.InfoIcon(), s.showStats)

	head := container.NewBorder(nil, nil, nil,
		container.NewHBox(saveBtn, reloadBtn, extBtn, gdbBtn, statsBtn),
		container.NewVBox(container.NewPadded(s.path), s.note),
	)

	s.content = container.NewBorder(
		container.NewVBox(head, widget.NewSeparator()),
		nil, nil, nil,
		s.editor,
	)
	s.reload()
	return s
}

func (s *sourcePage) onChanged(text string) {
	if text != s.loaded {
		s.note.SetText(tr("有未保存的修改"))
	} else {
		s.note.SetText("")
	}
}

// reload 从磁盘重新读取源码。
func (s *sourcePage) reload() {
	path := s.pg.p.sourcePath()
	s.path.SetText(path)

	data, err := os.ReadFile(path)
	if err != nil {
		s.loaded = ""
		s.editor.SetText("")
		s.note.SetText(fmt.Sprintf(tr("读取失败：%v"), err))
		return
	}
	s.loaded = string(data)
	s.editor.SetText(s.loaded)
	s.note.SetText("")
}

// showStats 展示当前源码的静态统计：体量、括号配对与全角字符检查。
func (s *sourcePage) showStats() {
	cs := codeStatsOf(s.editor.Text)
	body := widget.NewLabel(statsSummary(cs))
	body.TextStyle = fyne.TextStyle{Monospace: true}
	d := dialog.NewCustom(tr("代码统计"), tr("关闭"), container.NewVScroll(body), s.pg.g.win)
	d.Resize(fyne.NewSize(480, 320))
	d.Show()
}

// save 把编辑器内容写回源码文件。
func (s *sourcePage) save() {
	path := s.pg.p.sourcePath()
	if err := os.WriteFile(path, []byte(s.editor.Text), 0644); err != nil {
		s.pg.g.showError(fmt.Errorf(tr("保存失败: %v"), err))
		return
	}
	s.loaded = s.editor.Text
	s.note.SetText("")
	s.pg.g.setStatus(fmt.Sprintf(tr("已保存 %s"), path))
}
