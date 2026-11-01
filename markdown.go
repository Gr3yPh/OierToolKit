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
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 题面展示页：直接用 Fyne 内置的 Markdown 渲染，支持标题、粗体、斜体、
// 行内代码、代码块、列表、链接与表格（渲染范围以 Fyne 实现为准）。

// statementFileNames 是题面文件的候选名，按优先级排列。
var statementFileNames = []string{"problem.md", "statement.md", tr("题面.md"), "README.md"}

// findStatement 在项目目录里找题面文件，返回路径或空串。
func findStatement(p project) string {
	for _, name := range statementFileNames {
		path := filepath.Join(p.Dir, name)
		if fileExists(path) {
			return path
		}
	}
	return ""
}

// statementPage 是项目题面页：默认渲染预览，可原地编辑并保存成
// problem.md（题面文件已存在时沿用原文件名，避免多份题面）。
type statementPage struct {
	pg      *projectPage
	content fyne.CanvasObject
	rich    *widget.RichText
	title   *widget.Label

	// 编辑层
	editor    *widget.Entry
	preview   fyne.CanvasObject
	paneStack *fyne.Container
	editBtn   *widget.Button
	editing   bool
	text      string // 当前题面源码（未落盘的最新编辑结果）
}

func newStatementPage(pg *projectPage) *statementPage {
	s := &statementPage{pg: pg}
	s.title = hint("")

	s.rich = widget.NewRichText()
	s.rich.Wrapping = fyne.TextWrapWord
	s.editor = widget.NewMultiLineEntry()
	s.editor.Wrapping = fyne.TextWrapOff
	s.editor.SetPlaceHolder(tr("## 题面\n\n在这里写 Markdown，Ctrl+S 保存。"))
	s.preview = container.NewVScroll(container.NewPadded(s.rich))
	s.paneStack = container.NewStack(s.preview)

	reloadBtn := widget.NewButtonWithIcon(tr("重新加载"), theme.ViewRefreshIcon(), s.reload)
	s.editBtn = widget.NewButtonWithIcon(tr("编辑题面"), theme.DocumentCreateIcon(), s.toggleEdit)
	saveBtn := widget.NewButtonWithIcon(tr("保存"), theme.DocumentSaveIcon(), s.save)
	head := container.NewBorder(nil, nil, s.editBtn,
		container.NewHBox(saveBtn, reloadBtn), container.NewPadded(s.title))
	s.content = container.NewBorder(
		container.NewVBox(head, widget.NewSeparator()),
		nil, nil, nil,
		s.paneStack,
	)
	s.reload()
	return s
}

// toggleEdit 在预览与源码编辑之间切换。
func (s *statementPage) toggleEdit() {
	s.editing = !s.editing
	if s.editing {
		s.editor.SetText(s.text)
		s.paneStack.Objects = []fyne.CanvasObject{container.NewVScroll(s.editor)}
		s.editBtn.SetText(tr("退出编辑"))
	} else {
		s.rich.ParseMarkdown(stmtPreviewText(s.text))
		s.paneStack.Objects = []fyne.CanvasObject{s.preview}
		s.editBtn.SetText(tr("编辑题面"))
	}
	s.paneStack.Refresh()
}

// save 把编辑内容写回题面文件（没有则新建 problem.md）。
func (s *statementPage) save() {
	if s.editing {
		s.text = s.editor.Text
	}
	path := findStatement(s.pg.p)
	if path == "" {
		path = filepath.Join(s.pg.p.Dir, "problem.md")
	}
	if err := os.WriteFile(path, []byte(s.text), 0644); err != nil {
		s.pg.g.showError(err)
		return
	}
	s.title.SetText(path)
	s.rich.ParseMarkdown(stmtPreviewText(s.text))
	s.pg.g.setStatus(tr("题面已保存：") + path)
}

// reload 读取题面文件并渲染；没有题面时给放置提示。
func (s *statementPage) reload() {
	path := findStatement(s.pg.p)
	if path == "" {
		s.title.SetText(tr("没有找到题面文件（点“编辑题面”可直接新建 problem.md）"))
		s.text = ""
		s.rich.ParseMarkdown(stmtPreviewText(""))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.title.SetText(tr("读取失败：") + err.Error())
		s.rich.Segments = nil
		s.rich.Refresh()
		return
	}
	s.title.SetText(path)
	s.text = string(data)
	s.rich.ParseMarkdown(stmtPreviewText(s.text))
}
