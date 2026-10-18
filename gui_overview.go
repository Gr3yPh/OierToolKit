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
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// overviewPage 显示项目信息与评测配置，并提供常用操作入口。
type overviewPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	name    *widget.Label
	dir     *widget.Label
	source  *widget.Label
	count   *widget.Label
	remote  *widget.Label
	timeIn  *widget.Entry
	memIn   *widget.Entry
	langSel *widget.Select
	o2      *widget.Check
	threads *widget.Entry
	note    *widget.Label

	loading bool
}

var languageOptions = []string{"c++11", "c++14", "c++17", "c++20", "c++23"}

func newOverviewPage(pg *projectPage) *overviewPage {
	o := &overviewPage{pg: pg}

	o.name = strong("")
	o.dir = hint("")
	o.source = hint("")
	o.count = hint("")
	o.remote = hint("")

	o.timeIn = widget.NewEntry()
	o.timeIn.SetPlaceHolder("1.00")
	o.memIn = widget.NewEntry()
	o.memIn.SetPlaceHolder("125")

	o.langSel = widget.NewSelect(languageOptions, func(string) { o.markDirty() })
	o.o2 = widget.NewCheck("开启 -O2 优化", func(bool) { o.markDirty() })
	o.timeIn.OnChanged = func(string) { o.markDirty() }
	o.memIn.OnChanged = func(string) { o.markDirty() }

	// 默认并发：0 表示按 CPU 核心数自动选择（上限 8）。
	o.threads = widget.NewEntry()
	o.threads.SetPlaceHolder("0")
	o.threads.OnChanged = func(string) { o.markDirty() }

	o.note = caption("修改后点“保存配置”生效。")
	o.note.Alignment = fyne.TextAlignLeading

	saveBtn := widget.NewButtonWithIcon("保存配置", theme.DocumentSaveIcon(), o.save)
	refreshBtn := widget.NewButtonWithIcon("同步样例", theme.ViewRefreshIcon(), o.refreshSamples)
	openBtn := widget.NewButtonWithIcon("打开目录", theme.FolderOpenIcon(), func() {
		if err := openPath(pg.p.Dir); err != nil {
			pg.g.showError(err)
		}
	})
	editBtn := widget.NewButtonWithIcon("外部编辑器", theme.DocumentCreateIcon(), func() {
		if err := launchEditor(pg.p); err != nil {
			pg.g.showError(err)
		}
	})
	termBtn := widget.NewButtonWithIcon("在终端打开", theme.ComputerIcon(), func() {
		if err := openTerminal(pg.p.Dir, ""); err != nil {
			pg.g.showError(err)
		}
	})
	removeBtn := deleteButton("移除项目…", func() {
		pg.g.removeProjectDialog(pg.p)
	})

	info := container.NewVBox(
		section("项目"),
		caption("名称"), o.name,
		caption("目录"), o.dir,
		caption("源码"), o.source,
		caption("样例"), o.count,
	)

	form := widget.NewForm(
		widget.NewFormItem("时限（秒）", o.timeIn),
		widget.NewFormItem("内存（MB）", o.memIn),
		widget.NewFormItem("语言标准", o.langSel),
		widget.NewFormItem("优化", o.o2),
		widget.NewFormItem("并发线程", o.threads),
	)

	body := container.NewVBox(
		info,
		widget.NewSeparator(),
		section("评测配置"),
		form,
		container.NewHBox(saveBtn, o.note),
		widget.NewSeparator(),
		section("远端"),
		o.remote,
		container.NewHBox(
			widget.NewButton("配置远端", func() { pg.g.remoteConfigDialog(pg.p) }),
			caption("从网站添加的题目会自动填好这两项"),
		),
		widget.NewSeparator(),
		section("操作"),
		container.NewHBox(refreshBtn, openBtn, editBtn, termBtn),
		container.NewHBox(removeBtn, layout.NewSpacer()),
	)

	o.content = container.NewVScroll(container.NewPadded(body))
	o.reload()
	return o
}

// markDirty 提示配置尚未保存。
func (o *overviewPage) markDirty() {
	if o.loading {
		return
	}
	o.note.SetText("配置已修改，记得保存。")
}

// reload 从项目配置重新填充控件。
func (o *overviewPage) reload() {
	p := o.pg.p
	props := p.cfg()

	o.loading = true
	defer func() { o.loading = false }()

	o.name.SetText(p.Name)
	o.dir.SetText(p.Dir)
	o.source.SetText(p.sourcePath())
	o.count.SetText(fmt.Sprintf("%d 组（%s.samples 索引）", len(readIndex(p)), p.codeName()))
	o.timeIn.SetText(propsOr(props, "time_limit", "1.00"))
	o.memIn.SetText(propsOr(props, "memory_limit", "125"))

	version := propsOr(props, "version", "c++17")
	if !containsString(languageOptions, version) {
		o.langSel.SetOptions(append(languageOptions, version))
	} else {
		o.langSel.SetOptions(languageOptions)
	}
	o.langSel.SetSelected(version)
	o.o2.SetChecked(propsOr(props, "o2", "1") == "1")
	o.threads.SetText(propsOr(props, "threads", "0"))

	if base := props["remote_base"]; base != "" {
		o.remote.SetText(fmt.Sprintf("题目 %s @ %s", props["remote_id"], base))
	} else {
		o.remote.SetText("未配置远端。可用“从网站添加题目”自动写入，或手动填写地址与题号。")
	}
	o.note.SetText("修改后点“保存配置”生效。")
}

// save 校验并写入项目配置。
func (o *overviewPage) save() {
	p := o.pg.p
	timeText := strings.TrimSpace(o.timeIn.Text)
	memText := strings.TrimSpace(o.memIn.Text)

	if _, err := strconv.ParseFloat(timeText, 64); err != nil {
		o.pg.g.showError(fmt.Errorf("时限必须是数字，例如 1.00"))
		return
	}
	if _, err := strconv.ParseFloat(memText, 64); err != nil {
		o.pg.g.showError(fmt.Errorf("内存必须是数字，例如 125"))
		return
	}

	o2 := "off"
	if o.o2.Checked {
		o2 = "on"
	}
	threadText := strings.TrimSpace(o.threads.Text)
	if threadText == "" {
		threadText = "0"
	}
	if n, err := strconv.Atoi(threadText); err != nil || n < 0 {
		o.pg.g.showError(fmt.Errorf("并发线程必须是非负整数（0 表示自动）"))
		return
	}

	settings := [][2]string{
		{"time", timeText},
		{"memory", memText},
		{"version", o.langSel.Selected},
		{"o2", o2},
	}
	for _, s := range settings {
		if _, err := setProjectConfig(p, s[0], s[1]); err != nil {
			o.pg.g.showError(err)
			return
		}
	}
	// threads 走单独入口：0 表示自动，需要绕过"正整数"校验。
	if _, err := setProjectConfig(p, "threads", threadText); err != nil {
		o.pg.g.showError(err)
		return
	}

	o.reload()
	o.pg.g.setStatus("配置已保存")
}

// refreshSamples 扫描项目目录，把成对的 .in/.out(.ans) 同步进样例库。
func (o *overviewPage) refreshSamples() {
	res := syncSamples(o.pg.p, true)
	o.reload()
	o.pg.samples.reload()

	msg := "样例库已是最新"
	if len(res.added) > 0 || len(res.removed) > 0 {
		msg = fmt.Sprintf("同步完成：新增 %d 组，移除 %d 组", len(res.added), len(res.removed))
	}
	if len(res.missingOut) > 0 || len(res.missingIn) > 0 {
		msg += fmt.Sprintf("，%d 个文件未成对", len(res.missingOut)+len(res.missingIn))
	}
	o.pg.g.setStatus(msg)
}

// containsString 判断切片中是否包含某值。
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
