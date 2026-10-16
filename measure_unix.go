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
	"os/exec"
	"runtime"
	"syscall"
)

// startMemProbe 在 Unix (Linux/macOS/BSD) 上通过 wait4 填充的 getrusage
// 结构读取子进程峰值常驻内存 (Maxrss)，无需任何外部工具。
func startMemProbe(cmd *exec.Cmd) memProbeFunc {
	return func() (float64, bool) {
		if cmd == nil || cmd.ProcessState == nil {
			return 0, false
		}
		ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		if !ok || ru == nil {
			return 0, false
		}
		kb := int64(ru.Maxrss)
		if runtime.GOOS == "darwin" {
			// macOS 的 Maxrss 单位是字节，Linux 等是 KB。
			kb /= 1024
		}
		return float64(kb) / 1024.0, true
	}
}
