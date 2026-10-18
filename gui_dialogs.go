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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// newProjectDialog 在选定目录里初始化一个新项目。
func (g *guiApp) newProjectDialog() {
	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder("项目目录，例如 D:\\OI\\aplusb")
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("项目名称（默认取目录名）")

	if cwd, err := os.Getwd(); err == nil {
		dirEntry.SetText(cwd)
		nameEntry.SetText(filepath.Base(cwd))
	}

	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		g.pickFolder(dirEntry, func(dir string) {
			nameEntry.SetText(filepath.Base(dir))
		})
	})
	dirRow := container.NewBorder(nil, nil, nil, browse, dirEntry)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("目录", dirRow),
			widget.NewFormItem("名称", nameEntry),
		),
		caption("目录不存在会自动创建；已有 .cpp 与样例文件时不会覆盖。"),
	)

	g.showWideDialog("新建项目", "创建", "取消", content, func() {
		dir := strings.TrimSpace(dirEntry.Text)
		if dir == "" {
			g.showError(fmt.Errorf("请填写项目目录"))
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			name = filepath.Base(dir)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			g.showError(fmt.Errorf("无法创建目录: %v", err))
			return
		}
		p, res, created, err := initProjectAt(dir, name)
		if err != nil {
			g.showError(err)
			return
		}

		g.reloadProjects()
		g.showProject(p.Dir)

		msg := "项目 " + p.Name + " 已初始化"
		if created {
			msg = "已创建项目 " + p.Name
		}
		if len(res.added) > 0 {
			msg += fmt.Sprintf("，登记样例 %d 组", len(res.added))
		}
		g.setStatus(msg)
	})
}

// cloneDialog 从支持 otk-remote 协议的网站添加题目。
func (g *guiApp) cloneDialog() {
	global := readIni(globalConfigPath())
	baseEntry := widget.NewEntry()
	baseEntry.SetPlaceHolder("https://xxxoj.ac")
	baseEntry.SetText(propsOr(global, "remote", ""))

	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("题号，例如 1001")

	dirEntry := widget.NewEntry()
	if cwd, err := os.Getwd(); err == nil {
		dirEntry.SetText(cwd)
	}
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		g.pickFolder(dirEntry, nil)
	})
	dirRow := container.NewBorder(nil, nil, nil, browse, dirEntry)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("网站地址", baseEntry),
			widget.NewFormItem("题号", idEntry),
			widget.NewFormItem("保存到", dirRow),
		),
		caption("网站需要支持 otk-remote 协议（见 PROTOCOL.md）。\n会在保存目录下新建 <域名首段>-<题号> 目录，并写入题面样例、时限与远端信息。"),
	)

	g.showWideDialog("从网站添加题目", "添加", "取消", content, func() {
		base := strings.TrimRight(strings.TrimSpace(baseEntry.Text), "/")
		id := strings.TrimSpace(idEntry.Text)
		parent := strings.TrimSpace(dirEntry.Text)
		if base == "" || id == "" || parent == "" {
			g.showError(fmt.Errorf("远端地址、题号与保存目录都不能为空"))
			return
		}

		g.busy(true, "正在从 "+base+" 拉取题目 "+id+" ...")
		gen := g.generation.Load()
		go func() {
			p, prob, _, err := cloneProblem(parent, base, id)
			ui(func() {
				if gen != g.generation.Load() && g.page != nil {
					// 用户已切走，只更新项目库不打扰当前页面
					g.reloadProjects()
					return
				}
				g.busy(false, "题目已添加")
				if err != nil {
					g.showError(err)
					return
				}
				g.reloadProjects()
				g.showProject(p.Dir)
				g.setStatus(fmt.Sprintf("已添加题目 %s（%d 组样例）", prob.ID, len(prob.Samples)))
			})
		}()
	})
}

// settingsDialog 编辑全局配置（外观、编辑器、远端地址、令牌、桌面快捷方式）。
func (g *guiApp) settingsDialog() {
	global := readIni(globalConfigPath())

	editorEntry := widget.NewEntry()
	editorEntry.SetText(propsOr(global, "editor", defaultEditorCommand()))
	remoteEntry := widget.NewEntry()
	remoteEntry.SetText(propsOr(global, "remote", ""))
	remoteEntry.SetPlaceHolder("https://xxxoj.ac")
	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetPlaceHolder("留空表示保持原有令牌不变")

	fullscreen := widget.NewCheck("全屏（保留标题栏与右上角窗口按钮）",
		func(bool) {})
	fullscreen.SetChecked(guiPrefBool(prefFullscreen, true))

	shortcutState := caption("")
	refreshShortcutState := func() {
		if shortcutExists() {
			shortcutState.SetText("桌面快捷方式：已创建")
		} else {
			shortcutState.SetText("桌面快捷方式：未创建")
		}
	}
	refreshShortcutState()

	makeShortcut := widget.NewButton("创建桌面快捷方式", func() {
		// 不关对话框：创建完直接刷新状态，用户能立刻看到结果。
		if _, err := createDesktopShortcut(); err != nil {
			g.showError(err)
			return
		}
		refreshShortcutState()
		g.setStatus("桌面快捷方式已创建")
	})
	delShortcut := widget.NewButton("删除", func() {
		desktop, err := desktopDir()
		if err != nil {
			g.showError(err)
			return
		}
		path := shortcutPath(desktop)
		if _, err := os.Stat(path); err != nil {
			g.showError(fmt.Errorf("桌面上没有找到快捷方式"))
			return
		}
		if err := os.Remove(path); err != nil {
			g.showError(err)
			return
		}
		refreshShortcutState()
		g.setStatus("桌面快捷方式已删除")
	})

	content := container.NewVBox(
		section("外观"),
		widget.NewForm(
			widget.NewFormItem("窗口模式", fullscreen),
		),
		caption("全屏会保留系统标题栏和右上角的最小化、最大化、关闭按钮；"+
			"也可以用 F11 随时切换，Esc 退出全屏。"),
		widget.NewSeparator(),
		section("桌面快捷方式"),
		container.NewVBox(
			shortcutState,
			container.NewHBox(makeShortcut, delShortcut),
			wrappedCaption("快捷方式指向同目录下的 "+guiBinaryFileName()+
				"，与命令行版本 "+filepath.Base(selfExe())+" 互不影响。"),
		),
		widget.NewSeparator(),
		section("编辑器与远端"),
		widget.NewForm(
			widget.NewFormItem("编辑器命令", editorEntry),
			widget.NewFormItem("远端地址", remoteEntry),
			widget.NewFormItem("远端令牌", tokenEntry),
		),
		caption("编辑器命令中的 %s 会替换为源码路径，例如 code -g %s。"),
		caption("配置文件："+globalConfigPath()),
		container.NewHBox(
			widget.NewButton("打开配置目录", func() {
				if err := openPath(baseDir); err != nil {
					g.showError(err)
				}
			}),
			widget.NewButton("环境诊断", g.diagnosticsDialog),
		),
	)

	g.showSheet(sheetOptions{
		Title:       "设置",
		Confirm:     "保存",
		Dismiss:     "取消",
		Content:     content,
		HeightRatio: 0.9,
		OnConfirm: func() {
			if err := setGuiPrefBool(prefFullscreen, fullscreen.Checked); err != nil {
				g.showError(err)
				return
			}
			if _, err := setGlobalConfig("editor", strings.TrimSpace(editorEntry.Text)); err != nil {
				g.showError(err)
				return
			}
			if _, err := setGlobalConfig("remote", strings.TrimSpace(remoteEntry.Text)); err != nil {
				g.showError(err)
				return
			}
			if token := strings.TrimSpace(tokenEntry.Text); token != "" {
				if _, err := setGlobalConfig("remote_token", token); err != nil {
					g.showError(err)
					return
				}
			}
			// 全屏偏好立即生效，不必重启。
			g.setMaximized(fullscreen.Checked)
			g.setStatus("全局设置已保存")
		},
	})
}

// remoteConfigDialog 编辑当前项目的远端地址与题号。
func (g *guiApp) remoteConfigDialog(p project) {
	props := p.cfg()

	baseEntry := widget.NewEntry()
	baseEntry.SetText(props["remote_base"])
	baseEntry.SetPlaceHolder("https://xxxoj.ac")
	idEntry := widget.NewEntry()
	idEntry.SetText(props["remote_id"])
	idEntry.SetPlaceHolder("题号")

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("远端地址", baseEntry),
			widget.NewFormItem("题号", idEntry),
		),
		caption("地址需带协议头。保存后“提交”页会立即使用新配置。"),
	)

	dialog.ShowCustomConfirm("配置远端", "保存", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		if _, err := setProjectConfig(p, "remote_base", strings.TrimSpace(baseEntry.Text)); err != nil {
			g.showError(err)
			return
		}
		if _, err := setProjectConfig(p, "remote_id", strings.TrimSpace(idEntry.Text)); err != nil {
			g.showError(err)
			return
		}
		if g.page != nil {
			g.page.refresh()
		}
		g.setStatus("远端配置已保存")
	}, g.win)
}

// aboutDialog 显示版本、许可与项目地址。
func (g *guiApp) aboutDialog() {
	title := widget.NewLabelWithStyle("OierToolKit "+otkVersion, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText

	desc := caption("为 OIer 打造的轻量级本地评测与题目管理工具\n命令行与图形界面共用同一套引擎")
	desc.Alignment = fyne.TextAlignCenter

	github := widget.NewHyperlink("项目地址", parseURL("https://github.com/Gr3yPh/OierToolKit"))
	home := widget.NewHyperlink("主页", parseURL("https://otk.gr3yph4ntom.cn"))
	links := container.NewHBox(layout.NewSpacer(), github, home, layout.NewSpacer())

	license := widget.NewLabel("Copyright (C) 2026 魇珩Gr3yPh4ntom\n按 GNU GPLv3 或更新版本发布，不提供任何担保。")
	license.Alignment = fyne.TextAlignCenter
	license.Wrapping = fyne.TextWrapWord
	license.Importance = widget.LowImportance

	content := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(desc),
		widget.NewSeparator(),
		container.NewCenter(links),
		container.NewCenter(license),
		caption("界面字体：Noto Sans SC 子集（SIL Open Font License 1.1）"),
	)
	dialog.ShowCustom("关于", "关闭", content, g.win)
}

// diagnosticsDialog 展示运行环境摘要，方便排查“为什么编译不了”这类问题。
func (g *guiApp) diagnosticsDialog() {
	text := diagnosticsText()
	label := hint(text)

	content := container.NewBorder(
		nil,
		container.NewHBox(
			widget.NewButton("复制", func() {
				g.win.Clipboard().SetContent(text)
				g.setStatus("诊断信息已复制")
			}),
			caption("把这段内容贴到 issue 里通常就够了"),
		),
		nil, nil,
		container.NewVScroll(label),
	)
	dialog.ShowCustom("环境诊断", "关闭", content, g.win)
}

// diagnosticsText 汇总版本、平台与工具链信息。
func diagnosticsText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "OierToolKit %s\n", otkVersion)
	fmt.Fprintf(&b, "go %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "配置目录: %s\n\n", baseDir)

	for _, tool := range []string{"g++", "gdb"} {
		if path, err := exec.LookPath(tool); err == nil {
			fmt.Fprintf(&b, "%s: %s\n", tool, path)
		} else {
			fmt.Fprintf(&b, "%s: 未找到\n", tool)
		}
	}

	paths := registryRead()
	alive := 0
	for _, dir := range paths {
		if dirExists(dir) {
			alive++
		}
	}
	fmt.Fprintf(&b, "\n项目库: %d 个条目，%d 个目录可用\n", len(paths), alive)
	fmt.Fprintf(&b, "编辑器: %s\n", propsOr(readIni(globalConfigPath()), "editor", defaultEditorCommand()))
	return b.String()
}

// pickFolder 弹出目录选择框，选中后写入 entry。
func (g *guiApp) pickFolder(entry *widget.Entry, after func(string)) {
	picker := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
		if err != nil || list == nil {
			return
		}
		entry.SetText(list.Path())
		if after != nil {
			after(list.Path())
		}
	}, g.win)
	picker.Show()
}

// removeProjectDialog 让用户选择软删除或硬删除当前项目。
func (g *guiApp) removeProjectDialog(p project) {
	msg := widget.NewLabel(fmt.Sprintf("软删除只清理 .otk/、.stress/ 与编译产物，保留源码和样例；\n硬删除会连同目录 %s 一起删掉。", p.Dir))
	msg.Wrapping = fyne.TextWrapWord

	var d dialog.Dialog
	soft := widget.NewButton("软删除", func() {
		d.Hide()
		if err := removeOTKData(p); err != nil {
			g.showError(fmt.Errorf("无法移除 OTK 数据: %v", err))
			return
		}
		registryRemove(p.Dir)
		g.afterRemove(p.Dir, "已移除项目 "+p.Name+" 的 OTK 数据（源码与样例保留）")
	})
	hard := deleteButton("硬删除整个目录", func() {
		d.Hide()
		confirm := dialog.NewConfirm("确认硬删除", "将删除整个目录：\n"+p.Dir, func(ok bool) {
			if !ok {
				return
			}
			if err := os.RemoveAll(p.Dir); err != nil {
				g.showError(fmt.Errorf("无法删除目录: %v", err))
				return
			}
			registryRemove(p.Dir)
			g.afterRemove(p.Dir, "已删除项目目录 "+filepath.Base(p.Dir))
		}, g.win)
		confirm.SetConfirmText("删除")
		confirm.Show()
	})

	content := container.NewVBox(msg, container.NewHBox(soft, hard))
	d = dialog.NewCustom("移除项目", "取消", content, g.win)
	d.Show()
}

// afterRemove 在项目被移除后刷新列表与详情。
func (g *guiApp) afterRemove(dir, msg string) {
	g.reloadProjects()
	if g.selected == dir {
		g.page = nil
		g.selected = ""
		g.detail.Objects = []fyne.CanvasObject{g.emptyView()}
		g.detail.Refresh()
	}
	g.setStatus(msg)
}
