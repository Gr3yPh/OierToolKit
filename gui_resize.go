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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// resizeRefresh 在窗口尺寸稳定后强制刷新一次内容。
//
// Fyne 在某些 HiDPI/分数缩放环境里会复用旧的字形纹理，窗口 resize 后
// 文字可能被 GPU 轻微拉伸而变糊。这里不改变布局，只在 resize 停止后
// 让整棵画布重新刷新，迫使文本纹理按当前像素尺寸重画。
type resizeRefresh struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	last     fyne.Size
	refresh  func()
	settle   *time.Timer
	settleMs int
}

func newResizeRefresh(content fyne.CanvasObject, refresh func()) *resizeRefresh {
	r := &resizeRefresh{content: content, refresh: refresh, settleMs: 120}
	r.ExtendBaseWidget(r)
	return r
}

func (r *resizeRefresh) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

func (r *resizeRefresh) Resize(size fyne.Size) {
	r.BaseWidget.Resize(size)
	if r.last == size {
		return
	}
	r.last = size
	if r.settle != nil {
		r.settle.Stop()
	}
	r.settle = time.AfterFunc(time.Duration(r.settleMs)*time.Millisecond, func() {
		ui(func() {
			if r.refresh != nil {
				r.refresh()
			}
		})
	})
}

// refreshRoot 让 root 下的文本重新走一遍绘制/纹理生成。
func refreshCanvasRoot(root fyne.CanvasObject) {
	if root != nil {
		canvas.Refresh(root)
	}
}
