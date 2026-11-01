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
func section(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewVBox(l, widget.NewSeparator())
}

// kv 返回一行「标签：值」，用于概览页的只读信息。
func kv(key, value string) fyne.CanvasObject {
	k := caption(key)
	v := widget.NewLabel(value)
	v.Selectable = true
	v.Wrapping = fyne.TextWrapWord
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(88, k.MinSize().Height), k), nil, v)
}

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
