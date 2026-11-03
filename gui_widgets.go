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
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// verdictBadge 是固定在列表里显示 AC/WA/TLE 等判定的小色块。
type verdictBadge struct {
	widget.BaseWidget

	bg   *canvas.Rectangle
	text *canvas.Text
}

func newVerdictBadge() *verdictBadge {
	b := &verdictBadge{
		bg:   canvas.NewRectangle(colorIdle),
		text: canvas.NewText("--", color.White),
	}
	b.bg.CornerRadius = 6
	b.text.TextSize = 12
	b.text.TextStyle = fyne.TextStyle{Bold: true}
	b.text.Alignment = fyne.TextAlignCenter
	b.ExtendBaseWidget(b)
	return b
}

// set 更新判定文本与配色。深色主题下用提亮版色值，压在暗背景上不至于发闷。
func (b *verdictBadge) set(verdict string) {
	if verdict == "" {
		verdict = "--"
	}
	dark := fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark
	b.text.Text = verdict
	b.bg.FillColor = verdictToneFor(verdict, dark)
	b.text.Refresh()
	b.bg.Refresh()
}

// createRenderer 把四块内容铺进一行，并让它们垂直居中对齐。
// Fyne 默认的单行 HBox 会把矮控件顶到上沿，所以这里自己算 y 偏移。
func (b *verdictBadge) CreateRenderer() fyne.WidgetRenderer {
	r := &badgeRenderer{badge: b, objects: []fyne.CanvasObject{b.bg, b.text}}
	return r
}

type badgeRenderer struct {
	badge   *verdictBadge
	objects []fyne.CanvasObject
}

func (r *badgeRenderer) Layout(size fyne.Size) {
	r.badge.bg.Resize(size)
	// canvas.Text 的绘制基线落在自己的下沿，所以先把它收缩到文字实际
	// 高度，再整体垂直居中；直接 Resize 成整个格子会把文字压到底部。
	h := r.badge.text.MinSize().Height
	r.badge.text.Resize(fyne.NewSize(size.Width, h))
	r.badge.text.Move(fyne.NewPos(0, (size.Height-h)/2))
}

func (r *badgeRenderer) MinSize() fyne.Size {
	return fyne.NewSize(54, 26)
}

func (r *badgeRenderer) Refresh() {
	canvas.Refresh(r.badge)
}

func (r *badgeRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *badgeRenderer) Destroy() {}

// caption 返回弱化的说明文字。
// 用 LowImportance 而不是自定义灰色：这样它会跟着主题走，深浅两套都清楚，
// 而且主题里已把 ColorNameDisabled 重新定义成"次要文字色"（见 gui_theme.go）。
func caption(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return l
}

// strong 返回加粗的主文字。
func strong(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

// hint 返回可换行、可选中复制的长文本（日志、对比、错误详情）。
func hint(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	l.Selectable = true
	return l
}

// section 返回一个小标题。
// 以前每个标题下面都跟一条整宽分隔线，一页四五个 section 就变成
// 四五道横线，页面被切得很碎。现在只留 13px 粗体 + 一点下边距，
// 层级靠字重和留白，不靠线。
func section(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	l.SizeName = sizeNameSection
	return container.NewVBox(l, spacerH(4))
}

// spacerH 返回固定高度的空白，用于控制纵向节奏。
func spacerH(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

// inset 给单个对象加内边距。container.NewPadded 四边固定 5px，
// 顶栏/状态栏需要更精确的横向留白，所以单独做一个。
func inset(left, top, right, bottom float32, obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(insetLayout{left: left, top: top, right: right, bottom: bottom}, obj)
}

type insetLayout struct {
	left, top, right, bottom float32
}

func (l insetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := size.Width - l.left - l.right
	h := size.Height - l.top - l.bottom
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	for _, obj := range objects {
		obj.Move(fyne.NewPos(l.left, l.top))
		obj.Resize(fyne.NewSize(w, h))
	}
}

func (l insetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, obj := range objects {
		if !obj.Visible() {
			continue
		}
		s := obj.MinSize()
		if s.Width > w {
			w = s.Width
		}
		if s.Height > h {
			h = s.Height
		}
	}
	return fyne.NewSize(w+l.left+l.right, h+l.top+l.bottom)
}

// vCenter 只在纵向上居中：宽度仍拉满父容器，便于 Label 截断长文本。
// container.NewCenter 会把子对象缩到 MinSize，长路径会顶出去。
func vCenter(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(vCenterLayout{}, obj)
}

type vCenterLayout struct{}

func (vCenterLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		h := obj.MinSize().Height
		if h > size.Height {
			h = size.Height
		}
		obj.Resize(fyne.NewSize(size.Width, h))
		obj.Move(fyne.NewPos(0, (size.Height-h)/2))
	}
}

func (vCenterLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, obj := range objects {
		s := obj.MinSize()
		w = fyne.Max(w, s.Width)
		h = fyne.Max(h, s.Height)
	}
	return fyne.NewSize(w, h)
}

// fixedHeight 把内容钉在固定高度（宽度仍跟随父容器），
// 用于 48px 顶栏和 28px 状态栏。
func fixedHeight(h float32, content fyne.CanvasObject) fyne.CanvasObject {
	w := &fixedHeightWidget{content: content, height: h}
	w.ExtendBaseWidget(w)
	return w
}

type fixedHeightWidget struct {
	widget.BaseWidget
	content fyne.CanvasObject
	height  float32
}

func (w *fixedHeightWidget) CreateRenderer() fyne.WidgetRenderer {
	return &fixedHeightRenderer{w: w}
}

type fixedHeightRenderer struct {
	w *fixedHeightWidget
}

func (r *fixedHeightRenderer) Layout(size fyne.Size) {
	r.w.content.Move(fyne.NewPos(0, (size.Height-r.w.height)/2))
	r.w.content.Resize(fyne.NewSize(size.Width, r.w.height))
}

func (r *fixedHeightRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.w.content.MinSize().Width, r.w.height)
}

func (r *fixedHeightRenderer) Refresh() {
	canvas.Refresh(r.w.content)
}

func (r *fixedHeightRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.w.content}
}

func (r *fixedHeightRenderer) Destroy() {}

// maxWidth 让内容在宽窗口里保持最大宽度并居中，避免表单和文字被拉满。
func maxWidth(max float32, obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(maxWidthLayout{max: max}, obj)
}

type maxWidthLayout struct {
	max float32
}

func (l maxWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := size.Width
	if w > l.max {
		w = l.max
	}
	x := (size.Width - w) / 2
	for _, obj := range objects {
		obj.Move(fyne.NewPos(x, 0))
		obj.Resize(fyne.NewSize(w, obj.MinSize().Height))
	}
}

func (l maxWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, obj := range objects {
		if !obj.Visible() {
			continue
		}
		s := obj.MinSize()
		if s.Width > w {
			w = s.Width
		}
		if s.Height > h {
			h = s.Height
		}
	}
	if w > l.max {
		w = l.max
	}
	return fyne.NewSize(w, h)
}

// infoRow 是一行只读信息：左边固定宽度标签，右边值。
func infoRow(key string, value fyne.CanvasObject) fyne.CanvasObject {
	k := caption(key)
	left := container.NewGridWrap(fyne.NewSize(72, k.MinSize().Height), k)
	return container.NewBorder(nil, nil, left, nil, value)
}

// field 是表单里的一格：小标签在上，控件在下。
func field(label string, control fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(caption(label), control)
}

// monoLabel 返回等宽、可选中、超长省略的标签（路径、命令、代码片段）。
func monoLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Monospace: true}
	l.Truncation = fyne.TextTruncateEllipsis
	l.Selectable = true
	return l
}

// prettyPath 把主目录缩成 ~，侧栏里不再出现一长串绝对路径。
func prettyPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

// navButton 是侧栏的扁平导航项：透明底、左对齐，悬停才显出底色。
func navButton(label string, icon fyne.Resource, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, icon, tapped)
	b.Alignment = widget.ButtonAlignLeading
	b.Importance = widget.LowImportance
	return b
}

// sidebarSplit 用固定宽度的侧栏替代按比例分栏：窗口最大化后
// 侧栏不会跟着涨到 480px，列表始终有稳定的阅读宽度。
func sidebarSplit(side, main fyne.CanvasObject) fyne.CanvasObject {
	w := &sidebarSplitWidget{side: side, main: main}
	w.ExtendBaseWidget(w)
	return w
}

type sidebarSplitWidget struct {
	widget.BaseWidget
	side fyne.CanvasObject
	main fyne.CanvasObject
}

func (w *sidebarSplitWidget) CreateRenderer() fyne.WidgetRenderer {
	return &sidebarSplitRenderer{w: w}
}

type sidebarSplitRenderer struct {
	w *sidebarSplitWidget
}

func (r *sidebarSplitRenderer) Layout(size fyne.Size) {
	w := float32(280)
	if size.Width < 900 {
		w = 240
	}
	if w > size.Width*0.4 {
		w = size.Width * 0.4
	}
	r.w.side.Move(fyne.NewPos(0, 0))
	r.w.side.Resize(fyne.NewSize(w, size.Height))
	r.w.main.Move(fyne.NewPos(w, 0))
	r.w.main.Resize(fyne.NewSize(size.Width-w, size.Height))
}

func (r *sidebarSplitRenderer) MinSize() fyne.Size {
	s := r.w.side.MinSize()
	m := r.w.main.MinSize()
	h := s.Height
	if m.Height > h {
		h = m.Height
	}
	return fyne.NewSize(s.Width+m.Width, h)
}

func (r *sidebarSplitRenderer) Refresh() {
	canvas.Refresh(r.w.side)
	canvas.Refresh(r.w.main)
}

func (r *sidebarSplitRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.w.side, r.w.main}
}

func (r *sidebarSplitRenderer) Destroy() {}

// scrollHint 把长文本放进纵向滚动区域。
func scrollHint(text string) fyne.CanvasObject {
	return container.NewVScroll(hint(text))
}

// toolbar 把若干控件排成一行，尾部用空白撑开。
func toolbar(items ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(items...)
}

// wrappedCaption 生成一段可换行的说明文字。长文本（路径、文件名）在窄容器
// 里不会换行就会把布局顶宽，这里统一用 Word 换行。
func wrappedCaption(text string) *widget.Label {
	l := caption(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// deleteButton 生成一个删除按钮：图标用系统自带的红色垃圾桶，按钮底色
// 保持中性。
//
// 不用 widget.DangerImportance：那是把整个按钮刷成一大块纯红，在列表里
// 一排按钮中格外抢眼，看起来像"当前最该点的操作"，实际却是个破坏性动作。
// 红色留给图标和最终确认对话框就够了。
func deleteButton(label string, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon(label, theme.DeleteIcon(), tapped)
	if label != "" {
		b.Alignment = widget.ButtonAlignLeading
	}
	return b
}

// deleteIconButton 是只有垃圾桶图标的删除按钮，用于列表行尾。
func deleteIconButton(hint string, tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon("", theme.DeleteIcon(), tapped)
	b.Importance = widget.LowImportance // 低调一些，鼠标悬停才明显
	b.Resize(b.MinSize())
	_ = hint // Fyne 的 Button 没有 tooltip，保留参数便于调用处自我说明
	return b
}
