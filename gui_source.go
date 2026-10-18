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

	saveBtn := widget.NewButtonWithIcon("保存", theme.DocumentSaveIcon(), s.save)
	reloadBtn := widget.NewButtonWithIcon("重新加载", theme.ViewRefreshIcon(), s.reload)
	extBtn := widget.NewButtonWithIcon("外部编辑器", theme.DocumentCreateIcon(), func() {
		if err := launchEditor(pg.p); err != nil {
			pg.g.showError(err)
		}
	})
	gdbBtn := widget.NewButtonWithIcon("gdb 调试", theme.ComputerIcon(), func() {
		if err := launchGdb(pg.p); err != nil {
			pg.g.showError(err)
		}
	})

	head := container.NewBorder(nil, nil, nil,
		container.NewHBox(saveBtn, reloadBtn, extBtn, gdbBtn),
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
		s.note.SetText("有未保存的修改")
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
		s.note.SetText(fmt.Sprintf("读取失败：%v", err))
		return
	}
	s.loaded = string(data)
	s.editor.SetText(s.loaded)
	s.note.SetText("")
}

// save 把编辑器内容写回源码文件。
func (s *sourcePage) save() {
	path := s.pg.p.sourcePath()
	if err := os.WriteFile(path, []byte(s.editor.Text), 0644); err != nil {
		s.pg.g.showError(fmt.Errorf("保存失败: %v", err))
		return
	}
	s.loaded = s.editor.Text
	s.note.SetText("")
	s.pg.g.setStatus("已保存 " + path)
}
