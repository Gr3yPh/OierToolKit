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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// samplesPage 用来查看、编辑、新增与删除样例文件对。
type samplesPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	ids      []string
	list     *widget.List
	selected string

	inBox   *widget.Entry
	outBox  *widget.Entry
	title   *widget.Label
	note    *widget.Label
	saveBtn *widget.Button

	loading     bool
	tooLarge    bool
	lastLargeID string
}

func newSamplesPage(pg *projectPage) *samplesPage {
	s := &samplesPage{pg: pg}

	s.list = widget.NewList(
		func() int { return len(s.ids) },
		func() fyne.CanvasObject { return widget.NewLabel("#000") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(s.ids) {
				return
			}
			obj.(*widget.Label).SetText("#" + s.ids[id])
		},
	)
	s.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(s.ids) {
			return
		}
		s.selectSample(s.ids[id])
	}

	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), s.addDialog)
	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), s.deleteSelected)
	importBtn := widget.NewButtonWithIcon(tr("从剪贴板导入"), theme.ContentPasteIcon(), s.importDialog)
	syncBtn := widget.NewButtonWithIcon(tr("同步"), theme.ViewRefreshIcon(), func() {
		res := syncSamples(s.pg.p, true)
		s.reload()
		msg := tr("样例库已是最新")
		if len(res.added) > 0 || len(res.removed) > 0 {
			msg = fmt.Sprintf(tr("同步完成：新增 %d，移除 %d"), len(res.added), len(res.removed))
		}
		s.pg.g.setStatus(msg)
	})

	sideTop := container.NewPadded(section(tr("样例")))
	sideBottom := container.NewVBox(widget.NewSeparator(), container.NewPadded(container.NewHBox(addBtn, delBtn, syncBtn, importBtn)))
	side := container.NewBorder(sideTop, sideBottom, nil, nil, s.list)

	s.title = strong(tr("未选择样例"))
	s.note = caption("")
	s.inBox = widget.NewMultiLineEntry()
	s.inBox.SetPlaceHolder(tr("输入数据（.in）"))
	s.inBox.OnChanged = func(string) { s.note.SetText(tr("有未保存的修改")) }
	s.outBox = widget.NewMultiLineEntry()
	s.outBox.SetPlaceHolder(tr("期望输出（.out / .ans）"))
	s.outBox.OnChanged = func(string) { s.note.SetText(tr("有未保存的修改")) }

	s.saveBtn = widget.NewButtonWithIcon(tr("保存样例"), theme.DocumentSaveIcon(), s.save)
	saveBtn := s.saveBtn
	inPane := container.NewBorder(caption(tr("输入 .in")), nil, nil, nil, s.inBox)
	outPane := container.NewBorder(caption(tr("期望输出")), nil, nil, nil, s.outBox)

	right := container.NewBorder(
		container.NewVBox(container.NewPadded(container.NewBorder(nil, nil, s.title, container.NewHBox(saveBtn), s.note)), widget.NewSeparator()),
		nil, nil, nil,
		container.NewVSplit(inPane, outPane),
	)

	split := container.NewHSplit(side, right)
	split.SetOffset(0.22)
	s.content = split

	s.reload()
	return s
}

// reload 重新读取样例索引并加载当前选中的样例。
func (s *samplesPage) reload() {
	s.loading = true
	defer func() { s.loading = false }()

	s.ids = readIndex(s.pg.p)
	s.list.Refresh()

	if len(s.ids) == 0 {
		s.selected = ""
		s.tooLarge = false
		s.lastLargeID = ""
		s.title.SetText(tr("还没有样例"))
		s.inBox.Enable()
		s.outBox.Enable()
		s.saveBtn.Enable()
		s.inBox.SetText("")
		s.outBox.SetText("")
		s.note.SetText(tr("在项目目录里放入成对的 .in 与 .out/.ans 后点“同步”"))
		return
	}

	idx := indexOf(s.ids, s.selected)
	if idx < 0 {
		idx = 0
	}
	s.selected = s.ids[idx]
	s.list.Select(idx)
	s.loadSample(s.selected)
}

// selectSample 切换当前编辑的样例，切换前自动保存上一组修改。
func (s *samplesPage) selectSample(id string) {
	if s.selected == id && s.title.Text != tr("未选择样例") {
		return
	}
	if !s.loading {
		s.save()
	}
	s.selected = id
	s.loadSample(id)
}

// loadSample 把样例内容填进编辑器。过大的样例不进 Entry：Fyne 的
// MultiLineEntry 处理几 MB 文本会长时间卡住 UI，直接提示用外部编辑器。
func (s *samplesPage) loadSample(id string) {
	s.title.SetText("#" + id)
	size := sampleDataSize(s.pg.p, id)
	if size > maxInAppSampleSize {
		s.loading = true
		s.inBox.SetText("")
		s.outBox.SetText("")
		s.loading = false
		s.tooLarge = true
		s.inBox.Disable()
		s.outBox.Disable()
		s.saveBtn.Disable()
		s.note.SetText(fmt.Sprintf(tr("样例过大（%s），请使用外部编辑器查看。"), humanSize(int(size))))
		if s.lastLargeID != id {
			s.lastLargeID = id
			s.pg.g.showInfo(tr("样例过大"),
				fmt.Sprintf(tr("样例过大（%s），请使用外部编辑器查看。"), humanSize(int(size))))
		}
		return
	}

	s.tooLarge = false
	s.lastLargeID = ""
	s.inBox.Enable()
	s.outBox.Enable()
	s.saveBtn.Enable()

	input, output, err := readSample(s.pg.p, id)
	s.loading = true
	s.inBox.SetText(input)
	s.outBox.SetText(output)
	s.loading = false

	switch {
	case err != nil:
		s.note.SetText(fmt.Sprintf(tr("读取不完整：%v"), err))
	default:
		s.note.SetText(fmt.Sprintf(tr("输入 %s · 输出 %s"), humanSize(len(input)), humanSize(len(output))))
	}
}

// maxInAppSampleSize 是软件内查看样例的上限；超过后不再塞进 Fyne Entry。
const maxInAppSampleSize int64 = 512 << 10

func sampleDataSize(p project, id string) int64 {
	var total int64
	for _, kind := range []string{"in", "out"} {
		if info, err := os.Stat(sampleFilePath(p, id, kind)); err == nil {
			total += info.Size()
		}
	}
	return total
}

// save 写回当前样例；没有选中样例时什么也不做。
func (s *samplesPage) save() {
	if s.selected == "" || s.loading || s.tooLarge {
		return
	}
	if err := writeSample(s.pg.p, s.selected, s.inBox.Text, s.outBox.Text); err != nil {
		s.pg.g.showError(err)
		return
	}
	s.note.SetText(fmt.Sprintf(tr("已保存 · 输入 %s · 输出 %s"),
		humanSize(len(s.inBox.Text)), humanSize(len(s.outBox.Text))))
}

// addDialog 新建一组空样例。
func (s *samplesPage) addDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(tr("编号，例如 3 或 sample3"))
	entry.SetText(nextSampleID(s.ids))

	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(tr("样例编号"), entry)),
	)

	s.pg.g.showFormConfirm(tr("新建样例"), tr("创建"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		id := strings.TrimSpace(entry.Text)
		if id == "" {
			return
		}
		if err := writeSample(s.pg.p, id, "", ""); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.selected = id
		s.reload()
		s.pg.g.setStatus(fmt.Sprintf(tr("已新建样例 #%s"), id))
	}, s.pg.g.win)
}

// importDialog 从剪贴板粘贴输入与期望输出，直接入库为样例。
func (s *samplesPage) importDialog() {
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder(tr("编号，例如 3 或 sample3"))
	idEntry.SetText(nextSampleID(s.ids))

	inEntry := widget.NewMultiLineEntry()
	inEntry.SetPlaceHolder(tr("点击右侧按钮粘贴输入数据"))
	outEntry := widget.NewMultiLineEntry()
	outEntry.SetPlaceHolder(tr("点击右侧按钮粘贴期望输出"))

	clipText := func() string {
		return strings.ReplaceAll(s.pg.g.win.Clipboard().Content(), "\r\n", "\n")
	}

	form := widget.NewForm(
		widget.NewFormItem(tr("样例编号"), idEntry),
		widget.NewFormItem(tr("输入 .in"), container.NewBorder(nil, nil, nil,
			widget.NewButtonWithIcon(tr("粘贴"), theme.ContentPasteIcon(), func() { inEntry.SetText(clipText()) }), inEntry)),
		widget.NewFormItem(tr("期望输出"), container.NewBorder(nil, nil, nil,
			widget.NewButtonWithIcon(tr("粘贴"), theme.ContentPasteIcon(), func() { outEntry.SetText(clipText()) }), outEntry)),
	)

	content := container.NewVBox(
		form,
	)

	s.pg.g.showFormConfirm(tr("从剪贴板导入样例"), tr("保存"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		id := strings.TrimSpace(idEntry.Text)
		if id == "" {
			s.pg.g.showError(fmt.Errorf("%s", tr("样例编号不能为空")))
			return
		}
		if err := writeSample(s.pg.p, id, inEntry.Text, outEntry.Text); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.selected = id
		s.reload()
		s.pg.g.setStatus(fmt.Sprintf(tr("已从剪贴板导入样例 #%s"), id))
	}, s.pg.g.win)
}

// nextSampleID 根据现有样例编号给出下一个可用的整数编号。
func nextSampleID(ids []string) string {
	suggest := 1
	for _, id := range ids {
		var n int
		if _, err := fmt.Sscanf(strings.TrimPrefix(id, "sample"), "%d", &n); err == nil && n >= suggest {
			suggest = n + 1
		}
	}
	return fmt.Sprintf("%d", suggest)
}

// deleteSelected 删除当前样例的全部文件。
func (s *samplesPage) deleteSelected() {
	if s.selected == "" {
		s.pg.g.setStatus(tr("先选择一个样例"))
		return
	}
	id := s.selected
	dialog.ShowConfirm(tr("删除样例"), fmt.Sprintf(tr("将删除 #%s 的 .in 与 .out/.ans 文件，确定吗？"), id), func(ok bool) {
		if !ok {
			return
		}
		if err := deleteSample(s.pg.p, id); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.selected = ""
		s.reload()
		s.pg.g.setStatus(fmt.Sprintf(tr("已删除样例 #%s"), id))
	}, s.pg.g.win)
}

// humanSize 把字节数转成便于阅读的单位。
func humanSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func indexOf(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}
