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

// Windows 上用 Job Object 给被测程序设内存硬上限：
//   - 超限时程序的分配直接失败，不会把机器内存吃穿（事后测量做不到这点）；
//   - KILL_ON_JOB_CLOSE 让句柄关闭时连子进程树一起杀，程序 fork 出去的
//     进程也逃不掉；
//   - PeakProcessMemoryUsed 用于判定是否真的顶到了上限（判 MLE）。

var jobKernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procCreateJobObjectW          = jobKernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject   = jobKernel32.NewProc("SetInformationJobObject")
	procQueryInformationJobObject = jobKernel32.NewProc("QueryInformationJobObject")
	procAssignProcessToJobObject  = jobKernel32.NewProc("AssignProcessToJobObject")
	jobProcOpenProcess            = jobKernel32.NewProc("OpenProcess")
	jobProcCloseHandle            = jobKernel32.NewProc("CloseHandle")
)

const (
	jobObjectExtendedLimitInfoClass   = 9
	jobObjectLimitProcessMemory       = 0x00000100
	jobObjectLimitKillOnJobClose      = 0x00002000
	jobProcAccessSetQuotaAndTerminate = 0x0100 | 0x0001 // PROCESS_SET_QUOTA | PROCESS_TERMINATE
)

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInfo struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// memLimiter 控制一个被测进程的内存上限；nil 表示不限制。
type memLimiter struct {
	handle     syscall.Handle
	limitBytes uintptr
}

// newMemLimiter 创建 Job Object；memMB <= 0 返回 nil（不限制）。
func newMemLimiter(memMB float64) *memLimiter {
	if memMB <= 0 {
		return nil
	}
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return nil
	}
	m := &memLimiter{
		handle:     syscall.Handle(h),
		limitBytes: uintptr(memMB * 1024 * 1024),
	}
	info := jobObjectExtendedLimitInfo{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitProcessMemory | jobObjectLimitKillOnJobClose
	info.ProcessMemoryLimit = m.limitBytes
	r, _, _ := procSetInformationJobObject.Call(
		h, jobObjectExtendedLimitInfoClass,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		jobProcCloseHandle.Call(h)
		return nil
	}
	return m
}

// assign 把已启动的进程放进 Job。os.Process 不暴露句柄，
// 用 PID 重新 OpenProcess 拿一个（需要 SET_QUOTA | TERMINATE 权限）。
func (m *memLimiter) assign(cmd *exec.Cmd) {
	if m == nil || cmd.Process == nil {
		return
	}
	h, _, _ := jobProcOpenProcess.Call(jobProcAccessSetQuotaAndTerminate, 0, uintptr(cmd.Process.Pid))
	if h == 0 {
		return
	}
	procAssignProcessToJobObject.Call(uintptr(m.handle), h)
	jobProcCloseHandle.Call(h) // Job 持有自己的引用，这个临时句柄可以立刻关
}

// peakMB 返回进程（含子进程）的峰值提交内存（MB）。
// 提交内存比 working set 更贴近"程序申请了多少"，作 MLE 判定更准。
func (m *memLimiter) peakMB() (float64, bool) {
	if m == nil || m.handle == 0 {
		return 0, false
	}
	var info jobObjectExtendedLimitInfo
	var ret uint32
	r, _, _ := procQueryInformationJobObject.Call(
		uintptr(m.handle), jobObjectExtendedLimitInfoClass,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
		uintptr(unsafe.Pointer(&ret)))
	if r == 0 {
		return 0, false
	}
	return float64(info.PeakProcessMemoryUsed) / (1024 * 1024), true
}

// hit 判断硬限制是否生效过。注意：导致分配失败的那次申请并不会计入
// 峰值（提交失败就回滚了），所以峰值通常停在"限额 - 最后一块分配"，
// 这里留 1MB 容差兜住小额分配的场景。
func (m *memLimiter) hit() bool {
	if m == nil || m.handle == 0 {
		return false
	}
	var info jobObjectExtendedLimitInfo
	var ret uint32
	r, _, _ := procQueryInformationJobObject.Call(
		uintptr(m.handle), jobObjectExtendedLimitInfoClass,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
		uintptr(unsafe.Pointer(&ret)))
	if r == 0 {
		return false
	}
	const margin = 1 << 20
	return info.PeakProcessMemoryUsed+margin >= m.limitBytes
}

// close 关闭 Job 句柄。KILL_ON_JOB_CLOSE 会把还活着的子进程树杀掉，
// 因此正常路径（进程已退出）与超时路径都安全。
func (m *memLimiter) close() {
	if m != nil && m.handle != 0 {
		jobProcCloseHandle.Call(uintptr(m.handle))
		m.handle = 0
	}
}
