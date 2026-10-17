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
	"os"
	"syscall"
	"unsafe"
)

const (
	enableVirtualTerminalProcessing = 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	cpUTF8                          = 65001
)

var (
	modkernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode     = modkernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = modkernel32.NewProc("SetConsoleMode")
	procGetConsoleOutputCP = modkernel32.NewProc("GetConsoleOutputCP")
	procSetConsoleOutputCP = modkernel32.NewProc("SetConsoleOutputCP")
)

// enableANSI 让 Windows 控制台（含 conhost）能正确显示 ANSI 颜色与 UTF-8
// 中文，返回控制台是否支持 ANSI。老式控制台（Win7/8 或关闭了 VT 的宿主）
// 上返回 false，调用方应整体关闭颜色，避免把转义码显示成乱码。
func enableANSI() bool {
	h := uintptr(syscall.Handle(os.Stdout.Fd()))
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		// stdout 是重定向或管道，不碰全局代码页。
		return false
	}
	// 输出代码页切到 UTF-8，避免 conhost 按 GBK 解释中文。
	if cp, _, _ := procGetConsoleOutputCP.Call(); cp != cpUTF8 {
		procSetConsoleOutputCP.Call(cpUTF8)
	}
	// conhost 默认不解释 ANSI 转义序列，必须由应用显式开启。
	r, _, _ = procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
