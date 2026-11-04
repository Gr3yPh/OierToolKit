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

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// checkForUpdates 检查新版本。manual=false 时只更新状态栏，失败静默；
// manual=true 时给出明确结果，方便用户主动点击后知道发生了什么。
func (g *guiApp) checkForUpdates(manual bool) {
	if manual {
		g.setStatus(tr("正在检查更新 ..."))
	}
	go func() {
		info := fetchLatestUpdate()
		if info.Version == "" {
			if manual {
				ui(func() {
					g.showError(fmt.Errorf("%s", tr("检查更新失败，请稍后再试")))
				})
			}
			return
		}
		if !isNewerVersion(info.Version, otkVersion) {
			if manual {
				ui(func() {
					dialog.ShowInformation(tr("检查更新"),
						fmt.Sprintf(tr("当前已是最新版本 %s。"), otkVersion), g.win)
				})
			}
			return
		}

		link := updatePageURL(info)
		ui(func() {
			if manual {
				label := widget.NewLabel(fmt.Sprintf(tr("发现新版本 %s（当前 %s）。"), info.Version, otkVersion))
				openLabel := tr("打开主页")
				if uiLangIsEnglish() {
					openLabel = tr("打开发布页")
				}
				content := container.NewVBox(label,
					widget.NewHyperlink(openLabel, parseURL(link)))
				dialog.ShowCustom(tr("检查更新"), tr("关闭"), content, g.win)
				return
			}
			g.setStatus(fmt.Sprintf(tr("发现新版本 %s（当前 %s），%s 可下载"),
				info.Version, otkVersion, link))
		})
	}()
}
