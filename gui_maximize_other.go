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

//go:build !windows

package main

import (
	"fyne.io/fyne/v2"
)

// 非 Windows 平台没有 WndProc 可发消息，Fyne 也没有暴露"最大化"这一步，
// 这里返回 false，由调用方用"Resize 到屏幕尺寸 + 居中"兜底。
// Linux 下多数窗口管理器会把这种等于屏幕尺寸的窗口当作最大化处理。

func nativeMaximize(fyne.Window, bool) bool { return false }

func nativeIsMaximized(fyne.Window) bool { return false }

func workAreaSize(fyne.Window) (fyne.Size, bool) { return fyne.Size{}, false }
