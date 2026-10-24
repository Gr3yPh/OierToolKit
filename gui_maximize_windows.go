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

//go:build windows

package main

import (
	"sync"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// Windows 上真正意义上的"最大化"必须交给窗口管理器做：
//
//   - w.Resize(屏幕尺寸) 只是把客户区撑到那么高再居中——会盖住任务栏，
//     而且不是最大化状态（任务栏按钮不会变成"还原"，Aero Snap 也不认）。
//   - w.SetFullScreen(true) 是独占全屏，标题栏和右上角三个按钮直接消失。
//
// 所以这里拿到底层 HWND，发 SW_MAXIMIZE / SW_RESTORE，效果与点标题栏上的
// 最大化按钮完全一致：铺满工作区、标题栏保留、任务栏可见、再点能还原。
// HWND 走 Fyne 公开的 driver.NativeWindow 接口取，不碰 internal 包。

var (
	user32Once sync.Once

	procShowWindowAsync     *syscall.LazyProc
	procIsZoomed            *syscall.LazyProc
	procSetForegroundWindow *syscall.LazyProc
	procMonitorFromWindow   *syscall.LazyProc
	procGetMonitorInfoW     *syscall.LazyProc
	procSystemParameters    *syscall.LazyProc
)

const (
	swMaximize = 3 // SW_MAXIMIZE
	swRestore  = 9 // SW_RESTORE

	spiGetWorkArea = 0x0030 // SPI_GETWORKAREA

	monitorDefaultToNearest = 0x00000002 // MONITOR_DEFAULTTONEAREST
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

func loadUser32() {
	user32 := syscall.NewLazyDLL("user32.dll")
	procShowWindowAsync = user32.NewProc("ShowWindowAsync")
	procIsZoomed = user32.NewProc("IsZoomed")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW = user32.NewProc("GetMonitorInfoW")
	procSystemParameters = user32.NewProc("SystemParametersInfoW")
}

// hwndOf 取出 Fyne 窗口背后的原生窗口句柄；取不到时返回 0。
func hwndOf(w fyne.Window) uintptr {
	if w == nil {
		return 0
	}
	native, ok := w.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var hwnd uintptr
	native.RunNative(func(ctx any) {
		if c, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = c.HWND
		}
	})
	return hwnd
}

// nativeMaximize 切换窗口的最大化状态。
// 返回 true 表示已交给窗口管理器处理，调用方不必再自己 Resize。
func nativeMaximize(w fyne.Window, on bool) bool {
	hwnd := hwndOf(w)
	if hwnd == 0 {
		return false
	}
	user32Once.Do(loadUser32)

	if on {
		// 对已经最大化的窗口调用 SW_MAXIMIZE 是幂等的，不必先判断。
		procShowWindowAsync.Call(hwnd, swMaximize)
		procSetForegroundWindow.Call(hwnd)
		return true
	}
	if isZoomed(hwnd) {
		procShowWindowAsync.Call(hwnd, swRestore)
	}
	return true
}

// nativeIsMaximized 报告窗口当前是否处于最大化状态。
func nativeIsMaximized(w fyne.Window) bool {
	hwnd := hwndOf(w)
	if hwnd == 0 {
		return false
	}
	user32Once.Do(loadUser32)
	return isZoomed(hwnd)
}

func isZoomed(hwnd uintptr) bool {
	r, _, _ := procIsZoomed.Call(hwnd)
	return r != 0
}

// workAreaSize 返回窗口所在显示器的工作区尺寸（已扣掉任务栏）。
// 只在取不到 HWND 的极端情况下用来兜底。
func workAreaSize(w fyne.Window) (fyne.Size, bool) {
	hwnd := hwndOf(w)
	if hwnd == 0 {
		return fyne.Size{}, false
	}
	user32Once.Do(loadUser32)

	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if mon != 0 {
		mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
			return fyne.NewSize(
				float32(mi.Work.Right-mi.Work.Left),
				float32(mi.Work.Bottom-mi.Work.Top),
			), true
		}
	}

	var area winRect
	if ok, _, _ := procSystemParameters.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0); ok != 0 {
		return fyne.NewSize(
			float32(area.Right-area.Left),
			float32(area.Bottom-area.Top),
		), true
	}
	return fyne.Size{}, false
}
