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
	"sync/atomic"
	"time"

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
	ioFile  *widget.Check
	ioIn    *widget.Entry
	ioOut   *widget.Entry
	note    *widget.Label

	loading     bool
	savePending atomic.Bool // 自动保存去抖：连续修改只落一次盘
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

	// 文件读写题：勾选后程序从文件读入、向文件写出。
	o.ioFile = widget.NewCheck("文件读写题（freopen）", func(bool) { o.markDirty() })
	o.ioIn = widget.NewEntry()
	o.ioIn.SetPlaceHolder("缺省 " + o.pg.p.codeName() + ".in")
	o.ioOut = widget.NewEntry()
	o.ioOut.SetPlaceHolder("缺省 " + o.pg.p.codeName() + ".out")
	o.ioIn.OnChanged = func(string) { o.markDirty() }
	o.ioOut.OnChanged = func(string) { o.markDirty() }

	o.note = caption("配置修改后自动保存，无需手动操作。")
	o.note.Alignment = fyne.TextAlignLeading

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
		widget.NewFormItem("输入输出", o.ioFile),
		widget.NewFormItem("输入文件名", o.ioIn),
		widget.NewFormItem("输出文件名", o.ioOut),
	)

	body := container.NewVBox(
		info,
		widget.NewSeparator(),
		section("评测配置"),
		form,
		container.NewHBox(o.note),
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
	o.ioFile.SetChecked(propsOr(props, "io_mode", "") == "file")
	o.ioIn.SetText(props["io_in"])
	o.ioOut.SetText(props["io_out"])
	o.ioIn.SetPlaceHolder("缺省 " + p.codeName() + ".in")
	o.ioOut.SetPlaceHolder("缺省 " + p.codeName() + ".out")

	if base := props["remote_base"]; base != "" {
		o.remote.SetText(fmt.Sprintf("题目 %s @ %s", props["remote_id"], base))
	} else {
		o.remote.SetText("未配置远端。可用“从网站添加题目”自动写入，或手动填写地址与题号。")
	}
	o.note.SetText("配置修改后自动保存。")
}

// markDirty 由各控件回调调用：去抖后自动落盘（与模拟赛配置一致）。
func (o *overviewPage) markDirty() {
	if o.loading {
		return
	}
	if o.savePending.CompareAndSwap(false, true) {
		time.AfterFunc(400*time.Millisecond, func() {
			o.savePending.Store(false)
			o.autoSave()
		})
	}
	o.note.SetText("配置修改中，稍后自动保存 …")
}

// autoSave 静默落盘：与 save 相同的字段，但单个字段无效时跳过该项
// （输入到一半的数字等），不弹错误框、不重载控件，不打断输入。
func (o *overviewPage) autoSave() {
	if o.loading {
		return
	}
	p := o.pg.p
	write := func(item, value string) {
		_, _ = setProjectConfig(p, item, value)
	}

	if t := strings.TrimSpace(o.timeIn.Text); t != "" {
		if _, err := strconv.ParseFloat(t, 64); err == nil {
			write("time", t)
		}
	}
	if m := strings.TrimSpace(o.memIn.Text); m != "" {
		if _, err := strconv.ParseFloat(m, 64); err == nil {
			write("memory", m)
		}
	}
	if o.langSel.Selected != "" {
		write("version", o.langSel.Selected)
	}
	if o.o2.Checked {
		write("o2", "on")
	} else {
		write("o2", "off")
	}
	threadText := strings.TrimSpace(o.threads.Text)
	if threadText == "" {
		threadText = "0"
	}
	if n, err := strconv.Atoi(threadText); err == nil && n >= 0 {
		write("threads", threadText)
	}

	// 文件读写配置：文件名留空表示用 <源码名>.in/.out 缺省。
	if o.ioFile.Checked {
		write("io", "file")
	} else {
		write("io", "std")
	}
	write("io_in", strings.TrimSpace(o.ioIn.Text))
	write("io_out", strings.TrimSpace(o.ioOut.Text))

	o.note.SetText("配置已自动保存")
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
