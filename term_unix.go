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
	"fmt"
	"os"
)

// enableANSI 在非 Windows 平台总是可用：终端默认解释 ANSI 转义序列，
// stdout 为重定向时由调用方的字符设备检查拦下，不会误开颜色。
func enableANSI() bool { return true }

// otkExit 非 Windows 平台没有要还原的控制台状态。
func otkExit(code int) { os.Exit(code) }

// termDiag 在非 Windows 平台只打印 TERM 环境变量。
func termDiag() {
	fmt.Printf("TERM=%q\n", os.Getenv("TERM"))
}
