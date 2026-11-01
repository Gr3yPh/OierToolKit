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

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// saveAsTemplateDialog 把当前项目存成模板。
//
// 带走的是可复用的部分：源码骨架、时限/内存/语言标准/编译参数、gen.cpp 与
// brute.cpp、以及（可选的）样例。项目名、远端地址、题号这些只属于这一道题
// 的字段不会进模板。
func (g *guiApp) saveAsTemplateDialog(p project) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(p.Name)
	withSamples := widget.NewCheck(tr("带上样例（每组不超过 64KB，最多 10 组）"), nil)
	withSamples.SetChecked(true)

	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(tr("模板名称"), nameEntry)),
		withSamples,
		caption(tr("源码、时限内存、语言标准、编译参数与 gen.cpp / brute.cpp 都会带上。")),
		caption(tr("同名模板会被覆盖；内置模板不会被顶掉。")),
	)

	g.showWideDialog(tr("存为模板"), tr("保存"), tr("取消"), content, func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			g.showError(fmt.Errorf("%s", tr("模板名称不能为空")))
			return
		}
		t, err := templateFromProject(p, name, withSamples.Checked)
		if err != nil {
			g.showError(err)
			return
		}
		if err := saveCustomTemplate(t); err != nil {
			g.showError(err)
			return
		}
		g.setStatus(fmt.Sprintf(tr("已存为模板：%s"), name))
	})
}
