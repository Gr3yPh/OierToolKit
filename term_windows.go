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
	"strings"
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

// termDiag 打印控制台探测细节供 otk doctor 排查显示问题。VT 试开后会
// 还原；代码页若能成功切到 65001 则保持不变（这本就是 otk 的期望状态）。
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
		// 现场试开 VT 再还原，拿到精确错误码；build>=14393 的系统
		// 上失败基本可断定控制台处于旧版模式。
		vt, _, vtErr := procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
		if vt != 0 {
			fmt.Println("VT 现场试开: 成功")
		} else {
			fmt.Printf("VT 现场试开失败: 错误码 %d (%v)\n", uint32(vtErr.(syscall.Errno)), vtErr)
		}
		procSetConsoleMode.Call(h, uintptr(mode)) // 还原，保持只读语义
	}

	cp0, _, _ := procGetConsoleOutputCP.Call()
	fmt.Printf("输出代码页: %d\n", cp0)
	if r2, _, cpErr := procSetConsoleOutputCP.Call(cpUTF8); r2 == 0 {
		fmt.Printf("SetConsoleOutputCP(65001) 失败: 错误码 %d (%v)\n", uint32(cpErr.(syscall.Errno)), cpErr)
	} else {
		cp1, _, _ := procGetConsoleOutputCP.Call()
		fmt.Printf("SetConsoleOutputCP(65001) 成功: 代码页 %d -> %d\n", cp0, cp1)
	}

	// 旧版控制台由 HKCU\Console\ForceV2=0 强制开启，会使 VT 与部分
	// 现代 API 失效；全局键与 cmd 专用子键都查一遍。
	if v, ok := regReadDWORD(`Console`, "ForceV2"); ok {
		fmt.Printf(`HKCU\Console\ForceV2 = %d`, v)
		if v == 0 {
			fmt.Println("（0 = 强制旧版控制台，VT 不可用）")
		} else {
			fmt.Println()
		}
	} else {
		fmt.Println(`HKCU\Console\ForceV2 未设置`)
	}
	sub := "Console\\" + strings.ReplaceAll(os.Getenv("ComSpec"), `\`, `_`)
	if v, ok := regReadDWORD(sub, "ForceV2"); ok {
		fmt.Printf(`HKCU\Console\<cmd>\ForceV2 = %d`, v)
		if v == 0 {
			fmt.Println("（0 = 强制旧版控制台，VT 不可用）")
		} else {
			fmt.Println()
		}
	} else {
		fmt.Printf("HKCU\\Console\\<cmd>\\ForceV2 未设置\n")
	}

	for _, k := range []string{"WT_SESSION", "TERM", "ANSICON", "ConEmuANSI", "ComSpec"} {
		fmt.Printf("%s=%q\n", k, os.Getenv(k))
	}
}

const keyRead = 0x20019 // KEY_READ

// regReadDWORD 读取 HKCU 下某个注册表项的 DWORD 值。
func regReadDWORD(subkey, name string) (uint32, bool) {
	sub, err := syscall.UTF16PtrFromString(subkey)
	if err != nil {
		return 0, false
	}
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, sub, 0, keyRead, &h); err != nil {
		return 0, false
	}
	defer syscall.RegCloseKey(h)
	nm, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false
	}
	var typ uint32
	var buf [4]byte
	n := uint32(len(buf))
	if err := syscall.RegQueryValueEx(h, nm, nil, &typ, &buf[0], &n); err != nil || typ != 4 /*REG_DWORD*/ || n != 4 {
		return 0, false
	}
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24, true
}
