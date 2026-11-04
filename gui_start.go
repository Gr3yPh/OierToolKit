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
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *guiApp) buildStartPage() fyne.CanvasObject {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo.png", guiLogoData))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(64, 64))

	title := widget.NewLabelWithStyle(tr("开始"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	version := caption("OierToolKit " + otkVersion)

	actions := container.NewGridWithColumns(2,
		startAction(tr("新建项目"), theme.ContentAddIcon(), g.newProjectDialog),
		startAction(tr("从网站添加题目"), theme.DownloadIcon(), g.cloneDialog),
		startAction(tr("项目库"), theme.ListIcon(), g.showProjectList),
		startAction(tr("模拟赛"), theme.GridIcon(), g.showContestList),
	)

	body := container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(title),
		container.NewCenter(version),
		spacerH(18),
		actions,
	)

	if len(g.dirs) > 0 {
		body.Add(section(tr("最近项目")))
		limit := len(g.dirs)
		if limit > 6 {
			limit = 6
		}
		for _, dir := range g.dirs[:limit] {
			dir := dir
			b := widget.NewButton(projectNameOf(dir), func() { g.openProjectByDir(dir) })
			b.Alignment = widget.ButtonAlignLeading
			b.Importance = widget.LowImportance
			body.Add(b)
		}
	}

	return container.NewVScroll(maxWidth(820, inset(24, 24, 24, 24, body)))
}

func startAction(label string, icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, icon, tapped)
	b.Alignment = widget.ButtonAlignLeading
	b.Importance = widget.LowImportance
	return b
}
