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
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 配置导出：先选选项，再选保存位置。
func (g *guiApp) exportConfigDialog() {
	withRegistry := widget.NewCheck(tr("连项目库一起导出（每行一个项目路径）"), nil)
	withTokens := widget.NewCheck(tr("连 OJ 令牌一起导出"), nil)

	content := container.NewVBox(
		caption(tr("导出 ~/.otkprofile 下的配置：编辑器、远端地址、界面偏好都在里面。")),
		withRegistry,
		withTokens,
		caption(tr("令牌是登录凭据，默认不带。带了又放到网盘上，等于把账号交出去。")),
	)

	dialog.ShowCustomConfirm(tr("导出配置"), tr("下一步"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			path := w.URI().Path()
			_ = w.Close()
			g.busy(true, tr("正在导出配置 ..."))
			go func() {
				info, err := exportConfigArchive(path, configExportOptions{
					IncludeRegistry: withRegistry.Checked,
					IncludeTokens:   withTokens.Checked,
				})
				ui(func() {
					if err != nil {
						g.busy(false, tr("导出失败"))
						g.showError(err)
						return
					}
					g.busy(false, tr("配置已导出"))
					g.setStatus(fmt.Sprintf(tr("配置已导出：%s（%d 项）"), path, info.Keys))
				})
			}()
		}, g.win)
		save.SetFileName("otk-config-" + time.Now().Format("20060102") + ".zip")
		save.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
		save.Show()
	}, g.win)
}

// 配置导入：先选文件，再确认合并方式。
func (g *guiApp) importConfigDialog() {
	open := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := r.URI().Path()
		_ = r.Close()
		g.confirmImportConfig(path)
	}, g.win)
	open.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
	open.Show()
}

// confirmImportConfig 让用户选合并方式，然后后台导入。
func (g *guiApp) confirmImportConfig(path string) {
	overwrite := widget.NewCheck(tr("覆盖本地同名项"), nil)
	withTokens := widget.NewCheck(tr("导入包里的 OJ 令牌"), nil)
	withRegistry := widget.NewCheck(tr("同时导入项目库"), nil)
	withRegistry.SetChecked(true)

	content := container.NewVBox(
		wrappedCaption(tr("来源：")+path),
		caption(tr("默认只补本地没有的项，已有的设置保持不动。")),
		overwrite,
		withTokens,
		withRegistry,
		caption(tr("项目库是绝对路径，换机器后多半指向不存在的目录，会在列表里标成「目录缺失」。")),
	)

	dialog.ShowCustomConfirm(tr("导入配置"), tr("导入"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		opts := configImportOptions{
			Overwrite:       overwrite.Checked,
			IncludeTokens:   withTokens.Checked,
			IncludeRegistry: withRegistry.Checked,
		}
		g.busy(true, tr("正在导入配置 ..."))
		go func() {
			res, err := importConfigArchive(path, opts)
			ui(func() {
				if err != nil {
					g.busy(false, tr("导入失败"))
					g.showError(err)
					return
				}
				g.busy(false, tr("配置已导入"))
				g.setStatus(fmt.Sprintf(tr("配置已导入：%s"), configArchiveSummary(res)))
				g.reloadProjects()
				dialog.ShowInformation(tr("配置已导入"),
					fmt.Sprintf(tr("来源：%s\n%s\n\n部分改动（主题、语言）重启后生效。"),
						res.From, configArchiveSummary(res)), g.win)
			})
		}()
	}, g.win)
}

// configBackupButtons 是设置页「备份与迁移」那一行。
func (g *guiApp) configBackupButtons() *fyne.Container {
	return container.NewHBox(
		widget.NewButtonWithIcon(tr("导出配置"), theme.DownloadIcon(), g.exportConfigDialog),
		widget.NewButtonWithIcon(tr("导入配置"), theme.UploadIcon(), g.importConfigDialog),
	)
}
