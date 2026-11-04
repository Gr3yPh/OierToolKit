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
	"fyne.io/fyne/v2/widget"
)

// wheelForwarder 是盖在内容上的透明 Scrollable。Fyne 的滚轮事件只会交给
// 鼠标下最顶层的 Scrollable；题面里的代码块和表格内部各有一个 HScroll，
// 指针落在这些位置时滚轮会被内层吃掉，外层的 VScroll 收不到。
//
// 把这一层放在 Stack 顶部：它没有 Tappable/Hoverable/Draggable 等接口，
// 因此不会挡住链接、文本选择或滚动条拖动，只负责把滚轮事件转给外层 VScroll。
type wheelForwarder struct {
	widget.BaseWidget
	target *container.Scroll
}

func newWheelForwarder(target *container.Scroll) *wheelForwarder {
	f := &wheelForwarder{target: target}
	f.ExtendBaseWidget(f)
	return f
}

func (f *wheelForwarder) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (f *wheelForwarder) Scrolled(ev *fyne.ScrollEvent) {
	if f.target != nil {
		f.target.Scrolled(ev)
	}
}

// scrollWithWheel 把一个 VScroll 包成“整个可见区域都能滚”的控件。
func scrollWithWheel(scroll *container.Scroll) fyne.CanvasObject {
	return container.NewStack(scroll, newWheelForwarder(scroll))
}
