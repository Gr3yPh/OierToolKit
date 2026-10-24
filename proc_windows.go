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
	"os/exec"
	"syscall"
)

// hideProcWindow 让子进程不创建控制台窗口。
//
// GUI 版本身是 windows 子系统、没有控制台，它启动的 g++、被测程序等
// 控制台程序默认会各自分配一个新控制台——表现为评测/编译时黑框一闪而过。
// CREATE_NO_WINDOW (0x08000000) 阻止这次分配；标准输入输出仍通过管道
// 正常收发。该标志只影响控制台分配，子进程如果是 GUI 程序（如编辑器），
// 自己的窗口照常显示。
func hideProcWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
