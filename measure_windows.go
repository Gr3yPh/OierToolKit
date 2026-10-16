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
	"unsafe"
)

const (
	processQueryInformation = 0x0400 // PROCESS_QUERY_INFORMATION
	processVMRead           = 0x0010 // PROCESS_VM_READ
)

var (
	modpsapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
)

// processMemoryCountersEx 对应 Win32 的 PROCESS_MEMORY_COUNTERS_EX，
// SIZE_T 字段以 uintptr 表示，自动适配 32/64 位。
type processMemoryCountersEx struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
	privateUsage               uintptr
}

func getProcessMemoryInfo(h syscall.Handle) (peakBytes uintptr, ok bool) {
	var counters processMemoryCountersEx
	counters.cb = uint32(unsafe.Sizeof(counters))
	r1, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.cb),
	)
	if r1 == 0 {
		return 0, false
	}
	return counters.peakWorkingSetSize, true
}

// startMemProbe 在 Windows 上通过 GetProcessMemoryInfo 读取进程峰值工作集
// 大小 (PeakWorkingSetSize)，无需任何外部工具。
//
// 进程退出后按 PID 打开句柄会失败，因此必须在进程存活期间（Start 之后、
// Wait 之前）先持有句柄；终止后的进程对象在句柄关闭前始终可查询。
func startMemProbe(cmd *exec.Cmd) memProbeFunc {
	if cmd == nil || cmd.Process == nil {
		return func() (float64, bool) { return 0, false }
	}
	h, err := syscall.OpenProcess(processQueryInformation|processVMRead, false, uint32(cmd.Process.Pid))
	if err != nil {
		return func() (float64, bool) { return 0, false }
	}
	return func() (float64, bool) {
		defer syscall.CloseHandle(h)
		peak, ok := getProcessMemoryInfo(h)
		if !ok {
			return 0, false
		}
		return float64(peak) / (1024.0 * 1024.0), true
	}
}
