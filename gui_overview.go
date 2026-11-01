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
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
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
	compSel *widget.Select
	timeIn  *widget.Entry
	memIn   *widget.Entry
	langSel *widget.Select
	o2      *widget.Check
	cflags  *widget.Entry
	threads *widget.Entry
	ioFile  *widget.Check
	ioIn    *widget.Entry
	ioOut   *widget.Entry
	spjTip  *widget.Label
	note    *widget.Label

	loading     bool
	savePending atomic.Bool // 自动保存去抖：连续修改只落一次盘
}

var languageOptions = []string{"c++11", "c++14", "c++17", "c++20", "c++23"}
var cLanguageOptions = []string{"c89", "c99", "c11", "c17"}

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

	// 编译器：g++（C++）或 gcc（C，源码按 C 编译并链接 -lm）。
	o.compSel = widget.NewSelect([]string{"g++", "gcc"}, func(v string) {
		if !o.loading {
			o.langSel.SetOptions(o.langOptionsFor(v))
			if !containsString(o.langOptionsFor(v), o.langSel.Selected) {
				if v == "gcc" {
					o.langSel.SetSelected("c17")
				} else {
					o.langSel.SetSelected("c++17")
				}
			}
		}
		o.markDirty()
	})
	o.o2 = widget.NewCheck(tr("开启 -O2 优化"), func(bool) { o.markDirty() })
	o.timeIn.OnChanged = func(string) { o.markDirty() }
	o.memIn.OnChanged = func(string) { o.markDirty() }

	// 额外编译参数：每题可单独配置，例如 -Wall。
	o.cflags = widget.NewEntry()
	o.cflags.SetPlaceHolder(tr("如 -Wall，留空表示不加"))
	o.cflags.OnChanged = func(string) { o.markDirty() }

	// 默认并发：0 表示按 CPU 核心数自动选择（上限 8）。
	o.threads = widget.NewEntry()
	o.threads.SetPlaceHolder("0")
	o.threads.OnChanged = func(string) { o.markDirty() }

	// 文件读写题：勾选后程序从文件读入、向文件写出。
	o.ioFile = widget.NewCheck(tr("文件读写题（freopen）"), func(bool) { o.markDirty() })
	o.ioIn = widget.NewEntry()
	o.ioIn.SetPlaceHolder(fmt.Sprintf(tr("缺省 %s.in"), o.pg.p.codeName()))
	o.ioOut = widget.NewEntry()
	o.ioOut.SetPlaceHolder(fmt.Sprintf(tr("缺省 %s.out"), o.pg.p.codeName()))
	o.ioIn.OnChanged = func(string) { o.markDirty() }
	o.ioOut.OnChanged = func(string) { o.markDirty() }

	o.note = caption(tr("配置修改后自动保存，无需手动操作。"))
	o.note.Alignment = fyne.TextAlignLeading

	refreshBtn := widget.NewButtonWithIcon(tr("同步样例"), theme.ViewRefreshIcon(), o.refreshSamples)
	openBtn := widget.NewButtonWithIcon(tr("打开目录"), theme.FolderOpenIcon(), func() {
		if err := openPath(pg.p.Dir); err != nil {
			pg.g.showError(err)
		}
	})
	editBtn := widget.NewButtonWithIcon(tr("外部编辑器"), theme.DocumentCreateIcon(), func() {
		if err := launchEditor(pg.p); err != nil {
			pg.g.showError(err)
		}
	})
	termBtn := widget.NewButtonWithIcon(tr("在终端打开"), theme.ComputerIcon(), func() {
		if err := openTerminal(pg.p.Dir, ""); err != nil {
			pg.g.showError(err)
		}
	})
	removeBtn := deleteButton(tr("移除项目…"), func() {
		pg.g.removeProjectDialog(pg.p)
	})
	packBtn := widget.NewButtonWithIcon(tr("打包 ZIP"), theme.DownloadIcon(), func() {
		pg.g.exportProjectZip(pg.p)
	})
	tplBtn := widget.NewButtonWithIcon(tr("存为模板"), theme.ContentCopyIcon(), func() {
		pg.g.saveAsTemplateDialog(pg.p)
	})

	info := container.NewVBox(
		section(tr("项目")),
		caption(tr("名称")), o.name,
		caption(tr("目录")), o.dir,
		caption(tr("源码")), o.source,
		caption(tr("样例")), o.count,
	)

	form := widget.NewForm(
		widget.NewFormItem(tr("时限（秒）"), o.timeIn),
		widget.NewFormItem(tr("内存（MB）"), o.memIn),
		widget.NewFormItem(tr("编译器"), o.compSel),
		widget.NewFormItem(tr("语言标准"), o.langSel),
		widget.NewFormItem(tr("优化"), o.o2),
		widget.NewFormItem(tr("编译参数"), o.cflags),
		widget.NewFormItem(tr("并发线程"), o.threads),
		widget.NewFormItem(tr("输入输出"), o.ioFile),
		widget.NewFormItem(tr("输入文件名"), o.ioIn),
		widget.NewFormItem(tr("输出文件名"), o.ioOut),
	)

	o.spjTip = caption("")

	body := container.NewVBox(
		info,
		widget.NewSeparator(),
		section(tr("评测配置")),
		form,
		container.NewHBox(o.spjTip),
		container.NewHBox(o.note),
		widget.NewSeparator(),
		section(tr("远端")),
		o.remote,
		container.NewHBox(
			widget.NewButton(tr("配置远端"), func() { pg.g.remoteConfigDialog(pg.p) }),
			caption(tr("从网站添加的题目会自动填好这两项")),
		),
		widget.NewSeparator(),
		section(tr("操作")),
		container.NewHBox(refreshBtn, openBtn, editBtn, termBtn),
		container.NewHBox(packBtn, tplBtn),
		container.NewHBox(caption(tr("打包 ZIP：源码、样例与配置打成一个包，不含编译产物。存为模板：把这份骨架留作下次新建的起手式。"))),
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
	o.count.SetText(fmt.Sprintf(tr("%d 组（%s.samples 索引）"), len(readIndex(p)), p.codeName()))
	o.timeIn.SetText(propsOr(props, "time_limit", "1.00"))
	o.memIn.SetText(propsOr(props, "memory_limit", "125"))

	compiler := compilerOf(props)
	o.compSel.SetSelected(compiler)
	version := propsOr(props, "version", "c++17")
	options := o.langOptionsFor(compiler)
	if !containsString(options, version) {
		options = append(append([]string{}, options...), version)
	}
	o.langSel.SetOptions(options)
	o.langSel.SetSelected(version)
	o.o2.SetChecked(propsOr(props, "o2", "1") == "1")
	o.cflags.SetText(props["cflags"])
	o.threads.SetText(propsOr(props, "threads", "0"))
	o.ioFile.SetChecked(propsOr(props, "io_mode", "") == "file")
	o.ioIn.SetText(props["io_in"])
	o.ioOut.SetText(props["io_out"])
	o.ioIn.SetPlaceHolder(fmt.Sprintf(tr("缺省 %s.in"), p.codeName()))
	o.ioOut.SetPlaceHolder(fmt.Sprintf(tr("缺省 %s.out"), p.codeName()))

	if spjEnabled(p, props) {
		o.spjTip.SetText(fmt.Sprintf(tr("已启用特殊判题：%s（checker <输入> <选手输出> <期望输出>，退出码 0 为 AC）"), checkerSourceName))
	} else if fileExists(filepath.Join(p.Dir, checkerSourceName)) {
		o.spjTip.SetText(tr("特殊判题已在配置中关闭（otk config spj on 重新启用）"))
	} else {
		o.spjTip.SetText(fmt.Sprintf(tr("特殊判题：在项目目录放一个 %s 即自动启用"), checkerSourceName))
	}

	if base := props["remote_base"]; base != "" {
		o.remote.SetText(fmt.Sprintf(tr("题目 %s @ %s"), props["remote_id"], base))
	} else {
		o.remote.SetText(tr("未配置远端。可用“从网站添加题目”自动写入，或手动填写地址与题号。"))
	}
	o.note.SetText(tr("配置修改后自动保存。"))
}

// langOptionsFor 返回编译器对应的语言标准选项。
func (o *overviewPage) langOptionsFor(compiler string) []string {
	if compiler == "gcc" {
		return cLanguageOptions
	}
	return languageOptions
}

// markDirty 由各控件回调调用：去抖后自动落盘（与模拟赛配置一致）。
func (o *overviewPage) markDirty() {
	if o.loading {
		return
	}
	if o.savePending.CompareAndSwap(false, true) {
		// AfterFunc 的回调跑在独立 goroutine，autoSave 要读输入框、写 note
		// 标签，全部是 UI 操作，必须切回 UI 线程执行。
		time.AfterFunc(400*time.Millisecond, func() {
			ui(func() {
				o.savePending.Store(false)
				o.autoSave()
			})
		})
	}
	o.note.SetText(tr("配置修改中，稍后自动保存 …"))
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
	if o.compSel.Selected != "" {
		write("compiler", o.compSel.Selected)
	}
	if o.o2.Checked {
		write("o2", "on")
	} else {
		write("o2", "off")
	}
	write("cflags", strings.TrimSpace(o.cflags.Text))
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

	o.note.SetText(tr("配置已自动保存"))
}

// refreshSamples 扫描项目目录，把成对的 .in/.out(.ans) 同步进样例库。
func (o *overviewPage) refreshSamples() {
	res := syncSamples(o.pg.p, true)
	o.reload()
	o.pg.samples.reload()

	msg := tr("样例库已是最新")
	if len(res.added) > 0 || len(res.removed) > 0 {
		msg = fmt.Sprintf(tr("同步完成：新增 %d 组，移除 %d 组"), len(res.added), len(res.removed))
	}
	if len(res.missingOut) > 0 || len(res.missingIn) > 0 {
		msg += fmt.Sprintf(tr("，%d 个文件未成对"), len(res.missingOut)+len(res.missingIn))
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

// exportProjectZip 弹保存框把当前项目打包成 zip 数据包。
// 默认文件名用项目名，落在用户上次选择的位置（保存框自己记忆）。
func (g *guiApp) exportProjectZip(p project) {
	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		out := w.URI().Path()
		// 保存框会先把文件建出来，压缩包由 exportProjectArchive 自己写。
		_ = w.Close()
		g.runExport(tr("正在打包项目 ..."), func() (string, error) {
			return exportProjectArchive(p, out)
		})
	}, g.win)
	save.SetFileName(sanitizeFileName(p.Name) + ".zip")
	save.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
	save.Show()
}
