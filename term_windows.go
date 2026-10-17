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
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableVirtualTerminalProcessing = 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	cpUTF8                          = 65001
	fileTypeChar                    = 0x0002 // FILE_TYPE_CHAR
	fileTypePipe                    = 0x0003 // FILE_TYPE_PIPE
)

var (
	modkernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode     = modkernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = modkernel32.NewProc("SetConsoleMode")
	procGetConsoleOutputCP = modkernel32.NewProc("GetConsoleOutputCP")
	procSetConsoleOutputCP = modkernel32.NewProc("SetConsoleOutputCP")
	procGetFileType        = modkernel32.NewProc("GetFileType")
	modntdll               = syscall.NewLazyDLL("ntdll.dll")
	procRtlGetVersion      = modntdll.NewProc("RtlGetVersion")
)

// osVersionInfoEx 对应 ntdll 的 OSVERSIONINFOEXW。
type osVersionInfoEx struct {
	size             uint32
	major            uint32
	minor            uint32
	build            uint32
	platformID       uint32
	csd              [128]uint16
	servicePackMajor uint16
	servicePackMinor uint16
	suiteMask        uint16
	productType      byte
	reserved         byte
}

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
	// conhost 默认不解释 ANSI 转义序列，需要应用显式开启 VT；
	// 老式控制台（Win10 1607 之前或勾了旧版控制台）开不了，此时没有
	// 颜色，但代码页照切——UTF-8 输出配 GBK 代码页必然乱码。
	vt, _, _ := procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
	if cp, _, _ := procGetConsoleOutputCP.Call(); cp != cpUTF8 {
		procSetConsoleOutputCP.Call(cpUTF8)
	}
	return vt != 0
}

// termDiag 打印控制台探测细节，只读不改，供 otk doctor 排查显示问题。
func termDiag() {
	var vi osVersionInfoEx
	vi.size = uint32(unsafe.Sizeof(vi))
	if r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&vi))); r == 0 {
		fmt.Printf("Windows 版本: %d.%d build %d\n", vi.major, vi.minor, vi.build)
	}

	h := uintptr(syscall.Handle(os.Stdout.Fd()))
	ft, _, _ := procGetFileType.Call(h)
	var ftName string
	switch ft {
	case fileTypeChar:
		ftName = "字符设备"
	case fileTypePipe:
		ftName = "管道"
	case 0x0001:
		ftName = "磁盘文件"
	default:
		ftName = fmt.Sprintf("0x%x", ft)
	}
	fmt.Printf("stdout 句柄类型: %s\n", ftName)

	var mode uint32
	r, _, err := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		fmt.Printf("GetConsoleMode 失败: 错误码 %d (%v)\n", uint32(err.(syscall.Errno)), err)
	} else {
		fmt.Printf("GetConsoleMode 成功: mode=0x%x\n", mode)
	}

	cp, _, _ := procGetConsoleOutputCP.Call()
	fmt.Printf("输出代码页: %d\n", cp)

	for _, k := range []string{"WT_SESSION", "TERM", "ANSICON", "ConEmuANSI", "ComSpec"} {
		fmt.Printf("%s=%q\n", k, os.Getenv(k))
	}
}
