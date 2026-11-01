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

//go:build !windows && !linux

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// startMemProbe 在 macOS/BSD 上通过 wait4 填充的 getrusage 结构读取子进程
// 峰值常驻内存 (Maxrss)，无需任何外部工具。
//
// Linux 不用这套：内核会在 exec 时把 fork 继承来的地址空间峰值记进
// signal->maxrss 且只增不减，导致被测程序的 ru_maxrss 至少等于父进程的
// RSS（详见 measure_linux.go 里改为采样 VmHWM 的原因）。
func startMemProbe(cmd *exec.Cmd) memProbeFunc {
	return func() (float64, bool) {
		if cmd == nil || cmd.ProcessState == nil {
			return 0, false
		}
		ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		if !ok || ru == nil {
			return 0, false
		}
		v := int64(ru.Maxrss)
		switch runtime.GOOS {
		case "darwin", "freebsd", "dragonfly":
			// 这几家的 ru_maxrss 单位是字节；Linux/NetBSD/OpenBSD 是 KB。
			v /= 1024
		}
		return float64(v) / 1024.0, true
	}
}
