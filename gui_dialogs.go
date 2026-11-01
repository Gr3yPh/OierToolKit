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
	"strconv"
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
	tpls := allTemplates()
	builtin := map[string]bool{}
	for _, t := range builtinTemplates() {
		builtin[t.ID] = true
	}

	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder(tr("项目目录，例如 D:\\OI\\aplusb"))
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(tr("项目名称（默认取目录名）"))

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

	tplLabels := make([]string, len(tpls))
	for i, t := range tpls {
		tplLabels[i] = templateLabel(t)
	}
	descLbl := caption("")
	delBtn := widget.NewButtonWithIcon(tr("删除模板"), theme.DeleteIcon(), nil)
	delBtn.Hide()

	var tplSel *widget.Select
	syncTemplate := func() {
		i := tplSel.SelectedIndex()
		if i < 0 || i >= len(tpls) {
			descLbl.SetText("")
			delBtn.Hide()
			return
		}
		t := tpls[i]
		descLbl.SetText(strings.TrimSuffix(templateDesc(t), "。") + " · " + templatePropsSummary(t))
		// 内置模板没什么可删的，直接把按钮收起来，别摆一个永远是灰的按钮。
		if builtin[t.ID] {
			delBtn.Hide()
		} else {
			delBtn.Show()
		}
	}
	tplSel = widget.NewSelect(tplLabels, func(string) { syncTemplate() })
	tplSel.SetSelectedIndex(0)
	tplRow := container.NewBorder(nil, nil, nil, delBtn, tplSel)

	delBtn.OnTapped = func() {
		i := tplSel.SelectedIndex()
		if i < 0 || i >= len(tpls) {
			return
		}
		t := tpls[i]
		dialog.ShowConfirm(tr("删除模板"),
			fmt.Sprintf(tr("删除模板「%s」？项目本身不受影响。"), templateLabel(t)),
			func(ok bool) {
				if !ok {
					return
				}
				if err := deleteCustomTemplate(t.ID); err != nil {
					g.showError(err)
					return
				}
				// 下拉框已经建好了，只能自己换掉选项列表再刷新。
				tpls = allTemplates()
				tplLabels = tplLabels[:0]
				for _, tt := range tpls {
					tplLabels = append(tplLabels, templateLabel(tt))
				}
				tplSel.Options = tplLabels
				tplSel.ClearSelected()
				tplSel.Refresh()
				if len(tpls) > 0 {
					tplSel.SetSelectedIndex(0)
				} else {
					syncTemplate()
				}
			}, g.win)
	}

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(tr("模板"), tplRow),
			widget.NewFormItem(tr("目录"), dirRow),
			widget.NewFormItem(tr("名称"), nameEntry),
		),
		descLbl,
		caption(tr("目录不存在会自动创建；已存在的源码、样例与配置都不会被覆盖。")),
	)

	g.showWideDialog(tr("新建项目"), tr("创建"), tr("取消"), content, func() {
		dir := strings.TrimSpace(dirEntry.Text)
		if dir == "" {
			g.showError(fmt.Errorf("%s", tr("请填写项目目录")))
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			name = filepath.Base(dir)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			g.showError(fmt.Errorf(tr("无法创建目录: %v"), err))
			return
		}
		var tmpl *projectTemplate
		if i := tplSel.SelectedIndex(); i >= 0 && i < len(tpls) {
			t := tpls[i]
			tmpl = &t
		}
		p, res, created, err := initProjectAtWith(dir, name, tmpl)
		if err != nil {
			g.showError(err)
			return
		}

		g.reloadProjects()
		g.showProject(p.Dir)

		msg := fmt.Sprintf(tr("项目 %s 已初始化"), p.Name)
		if created {
			msg = fmt.Sprintf(tr("已创建项目 %s"), p.Name)
		}
		if len(res.added) > 0 {
			msg += fmt.Sprintf(tr("，登记样例 %d 组"), len(res.added))
		}
		g.setStatus(msg)
	})
}

// cloneDialog 从支持 otk-remote 协议的网站添加题目。
func (g *guiApp) cloneDialog() {
	baseEntry := widget.NewEntry()
	baseEntry.SetPlaceHolder("https://xxxoj.ac")
	baseEntry.SetText(defaultRemoteBase())

	// 已配置的命名远端直接下拉选择，地址与令牌自动带上。
	var remoteNames []string
	for _, r := range listRemotes() {
		remoteNames = append(remoteNames, r.Name)
	}
	remoteSel := widget.NewSelect(remoteNames, func(name string) {
		for _, r := range listRemotes() {
			if r.Name == name {
				baseEntry.SetText(r.Base)
			}
		}
	})
	if baseEntry.Text == "" && len(remoteNames) == 0 {
		remoteSel.Hide()
	}

	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder(tr("题号，例如 1001"))

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
			widget.NewFormItem(tr("已配置远端"), remoteSel),
			widget.NewFormItem(tr("网站地址"), baseEntry),
			widget.NewFormItem(tr("题号"), idEntry),
			widget.NewFormItem(tr("保存到"), dirRow),
		),
		caption(tr("网站需要支持 otk-remote 协议（见 PROTOCOL.md）。\n会在保存目录下新建 <域名首段>-<题号> 目录，并写入题面样例、时限与远端信息。\n令牌按地址自动匹配（设置里可配置多个远端，各自有独立令牌）。")),
	)

	g.showWideDialog(tr("从网站添加题目"), tr("添加"), tr("取消"), content, func() {
		base := strings.TrimRight(strings.TrimSpace(baseEntry.Text), "/")
		id := strings.TrimSpace(idEntry.Text)
		parent := strings.TrimSpace(dirEntry.Text)
		if base == "" || id == "" || parent == "" {
			g.showError(fmt.Errorf("%s", tr("远端地址、题号与保存目录都不能为空")))
			return
		}

		g.busy(true, fmt.Sprintf(tr("正在从 %s 拉取题目 %s ..."), base, id))
		gen := g.generation.Load()
		go func() {
			p, prob, _, err := cloneProblem(parent, base, id)
			ui(func() {
				if gen != g.generation.Load() && g.page != nil {
					// 用户已切走，只更新项目库不打扰当前页面
					g.reloadProjects()
					return
				}
				g.busy(false, tr("题目已添加"))
				if err != nil {
					g.showError(err)
					return
				}
				g.reloadProjects()
				g.showProject(p.Dir)
				g.setStatus(fmt.Sprintf(tr("已添加题目 %s（%d 组样例）"), prob.ID, len(prob.Samples)))
			})
		}()
	})
}

// settingsDialog 编辑全局配置（外观、编辑器、远端列表、桌面快捷方式）。
// 远端支持配置多条（名称 + 地址 + 令牌），各自独立。
func (g *guiApp) settingsDialog() {
	global := readIni(globalConfigPath())

	editorEntry := widget.NewEntry()
	editorEntry.SetText(propsOr(global, "editor", defaultEditorCommand()))

	// 远端编辑行：新增行名字可改，已有行名字固定（避免改名带来的歧义）。
	type remoteRow struct {
		origName string
		nameE    *widget.Entry
		baseE    *widget.Entry
		tokE     *widget.Entry
		removed  bool
		row      *fyne.Container
	}
	var rows []*remoteRow
	rowsBox := container.NewVBox()

	newRow := func(origName, base, token string) *remoteRow {
		r := &remoteRow{origName: origName}
		r.nameE = widget.NewEntry()
		r.nameE.SetPlaceHolder(tr("名称"))
		if origName != "" {
			r.nameE.SetText(origName)
			r.nameE.Disable()
		}
		r.baseE = widget.NewEntry()
		r.baseE.SetPlaceHolder("https://xxxoj.ac")
		r.baseE.SetText(base)
		r.tokE = widget.NewPasswordEntry()
		r.tokE.SetPlaceHolder(tr("令牌（留空保持不变）"))
		if token != "" {
			r.tokE.SetPlaceHolder(tr("令牌已配置（留空保持不变）"))
		}
		del := deleteIconButton(tr("删除这条远端"), func() {})
		r.row = container.NewStack(container.NewVBox(
			// 名称固定宽，地址占满剩余宽度：远端行不会被压成一条缝。
			container.NewBorder(nil, nil,
				container.NewGridWrap(fyne.NewSize(150, 34), r.nameE), del, r.baseE),
			r.tokE,
		))
		del.OnTapped = func() { r.removed = true; r.row.Hide() }
		rows = append(rows, r)
		rowsBox.Add(r.row)
		return r
	}

	for _, remote := range listRemotes() {
		newRow(remote.Name, remote.Base, remote.Token)
	}
	addRemoteBtn := widget.NewButtonWithIcon(tr("添加远端"), theme.ContentAddIcon(), func() {
		newRow("", "", "")
	})

	fullscreen := widget.NewCheck(tr("全屏（保留标题栏与右上角窗口按钮）"),
		func(bool) {})
	fullscreen.SetChecked(guiPrefBool(prefFullscreen, true))

	shortcutState := caption("")
	refreshShortcutState := func() {
		if shortcutExists() {
			shortcutState.SetText(tr("桌面快捷方式：已创建"))
		} else {
			shortcutState.SetText(tr("桌面快捷方式：未创建"))
		}
	}
	refreshShortcutState()

	makeShortcut := widget.NewButton(tr("创建桌面快捷方式"), func() {
		// 不关对话框：创建完直接刷新状态，用户能立刻看到结果。
		if _, err := createDesktopShortcut(); err != nil {
			g.showError(err)
			return
		}
		refreshShortcutState()
		g.setStatus(tr("桌面快捷方式已创建"))
	})
	delShortcut := widget.NewButton(tr("删除"), func() {
		desktop, err := desktopDir()
		if err != nil {
			g.showError(err)
			return
		}
		path := shortcutPath(desktop)
		if _, err := os.Stat(path); err != nil {
			g.showError(fmt.Errorf("%s", tr("桌面上没有找到快捷方式")))
			return
		}
		if err := os.Remove(path); err != nil {
			g.showError(err)
			return
		}
		refreshShortcutState()
		g.setStatus(tr("桌面快捷方式已删除"))
	})

	langSel := widget.NewSelect(languageChoices, nil)
	langSel.SetSelected(languageChoiceFromValue(guiPref(prefLang, "auto")))
	langNote := caption(tr("语言改动在重启应用后生效。"))

	// 主题：跟随系统 / 浅色 / 深色。按序号的写法与并发度一致，
	// 切换界面语言后选项文案会变，不能拿显示文本当键。
	themeSel := widget.NewSelect(themeLabels(), nil)
	themeSel.SetSelectedIndex(themeIndexFromValue(guiPref(prefTheme, "auto")))
	themeNote := caption(tr("主题改动在重启应用后生效；") +
		tr("“跟随系统”会跟着 Windows 的浅色/深色设置走。"))

	// 评测并发度：首项是「自动」，其余是固定值。用序号而不是文案做判断，
	// 避免切换语言后选项文本变化导致解析错乱。
	parallelChoices := []string{tr("自动（推荐）"), "1", "2", "4", "8", "12", "16", "24", "32"}
	parSel := widget.NewSelect(parallelChoices, nil)
	if cur := guiPrefInt(prefJudgeParallel, 0); cur > 0 {
		parSel.SetSelected(strconv.Itoa(cur))
	} else {
		parSel.SetSelectedIndex(0)
	}
	if parSel.Selected == "" {
		// 配置里写了个选项之外的值（比如手改过 ini）：保持原值，
		// 只在用户主动改选项时才覆盖。
		parSel.PlaceHolder = strconv.Itoa(guiPrefInt(prefJudgeParallel, 0))
	}

	content := container.NewVBox(
		section(tr("外观")),
		widget.NewForm(
			widget.NewFormItem(tr("窗口模式"), fullscreen),
			widget.NewFormItem(tr("界面语言"), langSel),
			widget.NewFormItem(tr("主题"), themeSel),
		),
		langNote,
		themeNote,
		caption(tr("全屏会保留系统标题栏和右上角的最小化、最大化、关闭按钮；")+
			tr("也可以用 F11 随时切换，Esc 退出全屏。")),
		widget.NewSeparator(),
		section(tr("评测")),
		widget.NewForm(
			widget.NewFormItem(tr("并发度"), parSel),
		),
		caption(tr("同时运行的评测任务数。自动 = CPU 核数减一（最多 8）；")+
			tr("机器卡顿时可以调小，大比赛想跑满可以调大。")),
		widget.NewSeparator(),
		section(tr("桌面快捷方式")),
		container.NewVBox(
			shortcutState,
			container.NewHBox(makeShortcut, delShortcut),
			wrappedCaption(tr("快捷方式指向同目录下的 ")+guiBinaryFileName()+
				tr("，与命令行版本 ")+filepath.Base(selfExe())+tr(" 互不影响。")),
		),
		widget.NewSeparator(),
		section(tr("编辑器")),
		widget.NewForm(
			widget.NewFormItem(tr("编辑器命令"), editorEntry),
		),
		caption(tr("编辑器命令中的 %s 会替换为源码路径，例如 code -g %s。")),
		widget.NewSeparator(),
		section(tr("远端列表")),
		rowsBox,
		container.NewHBox(addRemoteBtn),
		caption(tr("每条远端有独立的地址与令牌；克隆与提交时按地址自动匹配令牌。\n“默认”远端兼容旧版 otk config remote <地址> 的配置。")),
		widget.NewSeparator(),
		section(tr("备份与迁移")),
		g.configBackupButtons(),
		caption(tr("导出的 zip 只有几十 KB：换机器时把它拷过去再导入，编辑器和偏好就都回来了。")),
		caption(tr("导入默认只补空缺项，不动本机已有的设置；令牌与项目库需要单独勾选。")),
		widget.NewSeparator(),
		section(tr("其他")),
		container.NewVBox(
			container.NewHBox(
				widget.NewButton(tr("打开配置目录"), func() {
					if err := openPath(baseDir); err != nil {
						g.showError(err)
					}
				}),
				widget.NewButton(tr("环境诊断"), g.diagnosticsDialog),
			),
		),
	)

	g.showSheet(sheetOptions{
		Title:       tr("设置"),
		Confirm:     tr("保存"),
		Dismiss:     tr("取消"),
		Content:     content,
		HeightRatio: 0.9,
		OnConfirm: func() {
			if err := setGuiPrefBool(prefFullscreen, fullscreen.Checked); err != nil {
				g.showError(err)
				return
			}
			if langSel.Selected != "" {
				if err := setGuiPref(prefLang, languageValueFromChoice(langSel.Selected)); err != nil {
					g.showError(err)
					return
				}
			}
			if idx := themeSel.SelectedIndex(); idx >= 0 && idx < len(themeValues) {
				if err := setGuiPref(prefTheme, themeValues[idx]); err != nil {
					g.showError(err)
					return
				}
			}
			// 并发度：选「自动」写 0，其余直接写下拉显示的数字；
			// Selected 为空说明用户没动过那个 ini 里的非常规值，保持原样。
			if parSel.SelectedIndex() == 0 {
				if err := setGuiPref(prefJudgeParallel, "0"); err != nil {
					g.showError(err)
					return
				}
			} else if parSel.Selected != "" {
				if err := setGuiPref(prefJudgeParallel, parSel.Selected); err != nil {
					g.showError(err)
					return
				}
			}
			if _, err := setGlobalConfig("editor", strings.TrimSpace(editorEntry.Text)); err != nil {
				g.showError(err)
				return
			}
			// 远端行：先落删除，再保存每行的最新值（令牌留空保持不变）。
			for _, r := range rows {
				if r.removed {
					if r.origName != "" {
						if err := removeRemote(r.origName); err != nil {
							g.showError(err)
							return
						}
					}
					continue
				}
				name := strings.TrimSpace(r.nameE.Text)
				base := strings.TrimSpace(r.baseE.Text)
				token := strings.TrimSpace(r.tokE.Text)
				if name == "" && base == "" {
					continue
				}
				if name == "" || base == "" {
					g.showError(fmt.Errorf(tr("远端“%s”的名称与地址都要填写"), name))
					return
				}
				if err := saveRemote(name, base, token); err != nil {
					g.showError(err)
					return
				}
				// 新增行填了与已有远端相同的名字，或者行内改了地址，
				// 都以这里保存的为准；改名暂不支持，保持简单。
				if r.origName != "" && r.origName != name {
					_ = removeRemote(r.origName)
				}
			}
			// 全屏偏好立即生效，不必重启。
			g.setMaximized(fullscreen.Checked)
			g.setStatus(tr("全局设置已保存"))
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
	idEntry.SetPlaceHolder(tr("题号"))

	var remoteNames []string
	for _, r := range listRemotes() {
		remoteNames = append(remoteNames, r.Name)
	}
	remoteSel := widget.NewSelect(remoteNames, func(name string) {
		for _, r := range listRemotes() {
			if r.Name == name {
				baseEntry.SetText(r.Base)
			}
		}
	})
	if len(remoteNames) == 0 {
		remoteSel.Hide()
	}

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(tr("选择远端"), remoteSel),
			widget.NewFormItem(tr("远端地址"), baseEntry),
			widget.NewFormItem(tr("题号"), idEntry),
		),
		caption(tr("地址需带协议头。保存后“提交”页会立即使用新配置；\n令牌按地址自动匹配，可在设置里为每个远端单独配置。")),
	)

	dialog.ShowCustomConfirm(tr("配置远端"), tr("保存"), tr("取消"), content, func(ok bool) {
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
		g.setStatus(tr("远端配置已保存"))
	}, g.win)
}

// aboutDialog 显示版本、许可与项目地址。
func (g *guiApp) aboutDialog() {
	title := widget.NewLabelWithStyle("OierToolKit "+otkVersion, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText

	desc := caption(tr("为 OIer 打造的轻量级本地评测与题目管理工具\n命令行与图形界面共用同一套引擎"))
	desc.Alignment = fyne.TextAlignCenter

	github := widget.NewHyperlink(tr("项目地址"), parseURL("https://github.com/Gr3yPh/OierToolKit"))
	home := widget.NewHyperlink(tr("主页"), parseURL("https://otk.gr3yph4ntom.cn"))
	links := container.NewHBox(layout.NewSpacer(), github, home, layout.NewSpacer())

	license := widget.NewLabel(tr("Copyright (C) 2026 魇珩Gr3yPh4ntom\n") + tr("按 GNU GPLv3 或更新版本发布，不提供任何担保。"))
	license.Alignment = fyne.TextAlignCenter
	license.Wrapping = fyne.TextWrapWord
	license.Importance = widget.LowImportance

	content := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(desc),
		widget.NewSeparator(),
		container.NewCenter(links),
		container.NewCenter(license),
		caption(tr("界面字体：Noto Sans SC 子集（SIL Open Font License 1.1）")),
	)
	dialog.ShowCustom(tr("关于"), tr("关闭"), content, g.win)
}

// diagnosticsDialog 展示运行环境摘要，方便排查“为什么编译不了”这类问题。
func (g *guiApp) diagnosticsDialog() {
	text := diagnosticsText()
	label := hint(text)

	content := container.NewBorder(
		nil,
		container.NewHBox(
			widget.NewButton(tr("复制"), func() {
				g.win.Clipboard().SetContent(text)
				g.setStatus(tr("诊断信息已复制"))
			}),
			caption(tr("把这段内容贴到 issue 里通常就够了")),
		),
		nil, nil,
		container.NewVScroll(label),
	)
	dialog.ShowCustom(tr("环境诊断"), tr("关闭"), content, g.win)
}

// diagnosticsText 汇总版本、平台与工具链信息。
func diagnosticsText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "OierToolKit %s\n", otkVersion)
	fmt.Fprintf(&b, "go %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, tr("配置目录: %s\n\n"), baseDir)

	for _, tool := range []string{"g++", "gdb"} {
		if path, err := exec.LookPath(tool); err == nil {
			fmt.Fprintf(&b, "%s: %s\n", tool, path)
		} else {
			fmt.Fprintf(&b, tr("%s: 未找到\n"), tool)
		}
	}

	paths := registryRead()
	alive := 0
	for _, dir := range paths {
		if dirExists(dir) {
			alive++
		}
	}
	fmt.Fprintf(&b, tr("\n项目库: %d 个条目，%d 个目录可用\n"), len(paths), alive)
	fmt.Fprintf(&b, tr("编辑器: %s\n"), propsOr(readIni(globalConfigPath()), "editor", defaultEditorCommand()))
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
	msg := widget.NewLabel(fmt.Sprintf(tr("软删除只清理 .otk/、.stress/ 与编译产物，保留源码和样例；\n硬删除会连同目录 %s 一起删掉。"), p.Dir))
	msg.Wrapping = fyne.TextWrapWord

	var d dialog.Dialog
	soft := widget.NewButton(tr("软删除"), func() {
		d.Hide()
		if err := removeOTKData(p); err != nil {
			g.showError(fmt.Errorf(tr("无法移除 OTK 数据: %v"), err))
			return
		}
		registryRemove(p.Dir)
		g.afterRemove(p.Dir, fmt.Sprintf(tr("已移除项目 %s 的 OTK 数据（源码与样例保留）"), p.Name))
	})
	hard := deleteButton(tr("硬删除整个目录"), func() {
		d.Hide()
		confirm := dialog.NewConfirm(tr("确认硬删除"), tr("将删除整个目录：")+"\n"+p.Dir, func(ok bool) {
			if !ok {
				return
			}
			if err := os.RemoveAll(p.Dir); err != nil {
				g.showError(fmt.Errorf(tr("无法删除目录: %v"), err))
				return
			}
			registryRemove(p.Dir)
			g.afterRemove(p.Dir, fmt.Sprintf(tr("已删除项目目录 %s"), filepath.Base(p.Dir)))
		}, g.win)
		confirm.SetConfirmText(tr("删除"))
		confirm.Show()
	})

	content := container.NewVBox(msg, container.NewHBox(soft, hard))
	d = dialog.NewCustom(tr("移除项目"), tr("取消"), content, g.win)
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
