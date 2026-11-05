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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// checkForUpdates 检查新版本。启动自动检查失败静默，发现新版本会弹窗；
// manual=true 时也会给出“已是最新”或失败结果。
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

		ui(func() {
			g.setStatus(fmt.Sprintf(tr("发现新版本 %s（当前 %s），建议立即更新"), info.Version, otkVersion))
			g.showUpdatePrompt(info)
		})
	}()
}

// showUpdatePrompt 显示新版本提示。启动自动检查和手动检查共用这一套，
// 这样检测到更新时用户可以直接看到「立即更新」按钮。
func (g *guiApp) showUpdatePrompt(info updateInfo) {
	if g.win == nil {
		return
	}
	link := updatePageURL(info)
	label := widget.NewLabel(fmt.Sprintf(tr("发现新版本 %s（当前 %s），建议立即更新。"),
		info.Version, otkVersion))
	openLabel := tr("打开主页")
	if uiLangIsEnglish() {
		openLabel = tr("打开发布页")
	}
	var updateDialog *dialog.CustomDialog
	updateBtn := widget.NewButtonWithIcon(tr("立即更新"), theme.DownloadIcon(), func() {
		if updateDialog != nil {
			updateDialog.Hide()
		}
		g.startUpdate(info)
	})
	updateBtn.Importance = widget.HighImportance
	content := container.NewVBox(label, updateBtn,
		widget.NewHyperlink(openLabel, parseURL(link)))
	updateDialog = dialog.NewCustom(tr("发现新版本"), tr("关闭"), content, g.win)
	updateDialog.Show()
}

// startUpdate 一键下载当前平台的发行包，显示下载进度，替换并重启。
func (g *guiApp) startUpdate(info updateInfo) {
	if asset, ok := updateAssetName(); !ok || asset == "" {
		g.showError(fmt.Errorf("%s", tr("当前平台没有可用的更新包。")))
		return
	}
	if _, _, _, err := currentUpdateExecutable(); err != nil {
		g.showError(err)
		return
	}

	status := widget.NewLabel(tr("准备下载更新 ..."))
	status.Wrapping = fyne.TextWrapWord
	bar := widget.NewProgressBar()
	bar.Min = 0
	bar.Max = 1
	content := container.NewVBox(status, bar)
	progressDialog := dialog.NewCustomWithoutButtons(tr("更新 OierToolKit"), content, g.win)
	progressDialog.Show()

	go func() {
		var last time.Time
		pkg, err := downloadAndExtractUpdate(info, func(done, total int64) {
			if time.Since(last) < 100*time.Millisecond {
				return
			}
			last = time.Now()
			ui(func() {
				if total > 0 {
					bar.SetValue(float64(done) / float64(total))
					status.SetText(fmt.Sprintf(tr("正在下载更新 ... %s / %s"),
						humanSize(int(done)), humanSize(int(total))))
					return
				}
				status.SetText(fmt.Sprintf(tr("正在下载更新 ... %s"), humanSize(int(done))))
			})
		})
		if err != nil {
			ui(func() {
				progressDialog.Hide()
				g.showError(fmt.Errorf(tr("下载更新失败：%v"), err))
			})
			return
		}

		ui(func() {
			status.SetText(tr("正在安装更新 ..."))
			bar.SetValue(1)
		})
		if err := installUpdatePackage(pkg); err != nil {
			pkg.cleanup()
			ui(func() {
				progressDialog.Hide()
				g.showError(fmt.Errorf(tr("安装更新失败：%v"), err))
			})
			return
		}
		ui(func() {
			progressDialog.Hide()
			if g.win != nil {
				g.win.Close()
			}
			if g.app != nil {
				g.app.Quit()
			}
		})
	}()
}
