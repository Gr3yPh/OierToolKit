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
	"fmt"
	"os"
	"runtime"
)

// cmdDoctor 打印终端环境诊断信息，用于排查 Windows 控制台下的显示问题。
// 输出全部是探测结果，不做任何修改。
func cmdDoctor(args []string) {
	requireNoArgs(args, "doctor")
	fmt.Printf("OierToolKit %s\n", otkVersion)
	fmt.Printf("go %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	for _, s := range []struct {
		name string
		f    *os.File
	}{{"stdin", os.Stdin}, {"stdout", os.Stdout}, {"stderr", os.Stderr}} {
		kind := "未知"
		if fi, err := s.f.Stat(); err == nil {
			switch {
			case fi.Mode()&os.ModeCharDevice != 0:
				kind = "字符设备"
			case fi.Mode()&os.ModeNamedPipe != 0:
				kind = "管道"
			case fi.Mode().IsRegular():
				kind = "普通文件"
			default:
				kind = fi.Mode().String()
			}
		} else {
			kind = fmt.Sprintf("stat 失败: %v", err)
		}
		fmt.Printf("%s: %s\n", s.name, kind)
	}

	termDiag()

	fmt.Printf("ansiOK=%v wizardInteractive=%v\n", ansiOK, wizardInteractive())
	if !ansiOK && runtime.GOOS == "windows" {
		fmt.Println("（颜色关闭时中文仍乱码说明问题不在 ANSI，见上面代码页）")
	}
}
