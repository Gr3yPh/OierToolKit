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
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Fyne 自带的 ShowCustomConfirm 宽度只有约 400px，放设置这种多列表单太挤。
// 这里用一个自绘的宽面板代替，宽度跟着主窗口走（比例 + 上下限），含标题、
// 可滚动的表单区和底部按钮；Esc 关闭，行为与对话框一致。
//
// 关键：面板不是塞进 canvas.Overlays()，而是套在窗口内容的最外层。
// 原因有两个——
//  1. Overlay 一旦入栈就会顶替 canvas 的整个焦点管理器，键盘事件不再交给
//     下层的输入框，回车/退格/空格全部失灵；
//  2. 命中检测只走栈顶那一个 overlay，被判定挡住的内容既点不到也看不见。
//
// 套在内容层则完全复用主内容的焦点管理与命中检测，行为与普通页面一致。
//
// 也不用 dialog.Dialog.Resize：默认对话框内容用 MaxContainer 居中，
// 只把外框改宽并不会让表单变宽，反而两侧多出大片空白。

// sheetOptions 描述一个浮层面板。
type sheetOptions struct {
	Title   string
	Confirm string
	Dismiss string
	Content fyne.CanvasObject
	// WidthRatio 占主窗口宽度的比例（0 表示 0.62），下限 600px、上限 1000px。
	WidthRatio float64
	// HeightRatio 占主窗口高度的比例（0 表示 0.82），下限 560px、上限 900px。
	HeightRatio float64
	OnConfirm   func()
	OnDismiss   func()
}

// showWideDialog 用宽面板替换 Fyne 默认的窄对话框（约 400px），
// 适合设置这类多列表单。行为与 ShowCustomConfirm 一致：有确认/取消，
// 点取消或按 Esc 视为放弃。
func (g *guiApp) showWideDialog(title, confirm, dismiss string, content fyne.CanvasObject, onConfirm func()) {
	g.showSheet(sheetOptions{
		Title:     title,
		Confirm:   confirm,
		Dismiss:   dismiss,
		Content:   content,
		OnConfirm: onConfirm,
	})
}

// showSheet 在窗口内容之上叠一层浮层面板，返回时立即生效（无需排队）。
func (g *guiApp) showSheet(opt sheetOptions) {
	if g.win == nil || g.win.Canvas() == nil {
		return
	}
	base := g.win.Content()
	if base == nil {
		return
	}
	// 已经有一个浮层在显示时先撤掉，避免叠成两层。
	if g.sheet != nil {
		g.closeSheet()
	}

	ratio := opt.WidthRatio
	if ratio <= 0 {
		ratio = 0.62
	}
	hRatio := opt.HeightRatio
	if hRatio <= 0 {
		hRatio = 0.82
	}
	confirmText := opt.Confirm
	if confirmText == "" {
		confirmText = "保存"
	}
	dismissText := opt.Dismiss
	if dismissText == "" {
		dismissText = "取消"
	}

	title := widget.NewLabelWithStyle(opt.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText

	head := container.NewVBox(
		container.NewPadded(title),
		widget.NewSeparator(),
	)

	body := container.NewVScroll(container.NewPadded(opt.Content))

	cancel := func() {
		g.closeSheet()
		if opt.OnDismiss != nil {
			opt.OnDismiss()
		}
	}
	doConfirm := func() {
		// 先交给调用方：校验失败时它自己会再弹一次浮层。
		if opt.OnConfirm != nil {
			opt.OnConfirm()
		}
		g.closeSheet()
	}

	dismissBtn := widget.NewButton(dismissText, cancel)
	confirmBtn := widget.NewButtonWithIcon(confirmText, theme.ConfirmIcon(), doConfirm)

	foot := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(container.NewBorder(nil, nil,
			caption("Esc 取消"),
			container.NewHBox(dismissBtn, confirmBtn),
		)),
	)

	panel := container.NewStack(
		canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground)),
		container.NewBorder(head, foot, nil, nil, body),
	)
	holder := container.New(&sheetLayout{
		widthRatio:  ratio,
		minWidth:    600,
		maxWidth:    1000,
		heightRatio: hRatio,
		minHeight:   560,
		maxHeight:   900,
	}, panel)

	// 遮罩：半透明黑，挡住下层点击，也让面板更聚焦。
	scrim := canvas.NewRectangle(color.NRGBA{A: 170})

	g.sheet = container.NewStack(base, scrim, container.NewCenter(holder))
	g.sheetBase = base
	g.win.SetContent(g.sheet)

	// Esc 关闭，其余按键转交给原有处理器（F11 等全局快捷键仍然有效）。
	prev := g.win.Canvas().OnTypedKey()
	g.sheetRestore = func() {
		if g.win != nil && g.win.Canvas() != nil {
			g.win.Canvas().SetOnTypedKey(prev)
		}
	}
	g.win.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			cancel()
			return
		}
		if prev != nil {
			prev(ev)
		}
	})
}

// closeSheet 撤掉浮层面板，把窗口内容还原成原页面。
func (g *guiApp) closeSheet() {
	if g.sheet == nil || g.win == nil || g.win.Canvas() == nil {
		return
	}
	base := g.sheetBase
	g.sheet = nil
	g.sheetBase = nil
	g.win.SetContent(base)
	if g.sheetRestore != nil {
		g.sheetRestore()
		g.sheetRestore = nil
	}
}

// sheetLayout 让面板在主窗口的比例范围内自适应，并限制上下限。
type sheetLayout struct {
	widthRatio  float64
	minWidth    float32
	maxWidth    float32
	heightRatio float64
	minHeight   float32
	maxHeight   float32
}

// MinSize 报告面板的最小尺寸，保证窗口不会被压得放不下按钮。
func (l *sheetLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.minWidth, l.minHeight)
}

// Layout 把面板居中摆放，宽高各自按比例算并夹在上下限之间。
func (l *sheetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := float32(float64(size.Width) * l.widthRatio)
	if w < l.minWidth {
		w = l.minWidth
	}
	if w > l.maxWidth {
		w = l.maxWidth
	}
	if w > size.Width {
		w = size.Width
	}

	h := float32(float64(size.Height) * l.heightRatio)
	if h < l.minHeight {
		h = l.minHeight
	}
	if h > l.maxHeight {
		h = l.maxHeight
	}
	// 绝不超过可用高度，否则底部按钮会被挤出屏幕。
	if h > size.Height {
		h = size.Height
	}

	for _, o := range objs {
		o.Move(fyne.NewPos((size.Width-w)/2, (size.Height-h)/2))
		o.Resize(fyne.NewSize(w, h))
	}
}

var _ fyne.Layout = (*sheetLayout)(nil)
