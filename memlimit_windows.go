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
	limitBytes uintptr // 题目内存限额（判定基准）
	capBytes   uintptr // 实际下发给 Job 的上限
}

// Job 上限比题目限额留一段余量，判定线落在余量里。与 Linux 侧同一套口径
// （见 memlimit_linux.go 的说明）。
//
// 为什么不把 Job 上限设成限额本身：装载器、CRT 堆本身要占几 MB，Job 记的是
// 提交内存的绝对值，所以"限额 125MB"的程序在 124MB 就会分配失败；而失败的
// 那次申请不计入 PeakProcessMemoryUsed，于是峰值仍停在一两 MB——既判不出
// MLE，还因为 mingw 的 abort 不刷 stderr 而只剩一个光秃秃的 RE。
// 实测：124MB 的程序判 RE（峰值 1.94MB），122MB 的程序反倒被判 MLE。
const (
	jobMemSlackBytes = 8 << 20 // 上限放宽量
	jobMemGuardBytes = 2 << 20 // 判定线：峰值达到 限额+2MB 才算顶到上限
)

// newMemLimiter 创建 Job Object；memMB <= 0 返回 nil（不限制）。
func newMemLimiter(memMB float64) *memLimiter {
	if memMB <= 0 {
		return nil
	}
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return nil
	}
	limitBytes := uintptr(memMB * 1024 * 1024)
	m := &memLimiter{
		handle:     syscall.Handle(h),
		limitBytes: limitBytes,
		capBytes:   limitBytes + jobMemSlackBytes,
	}
	info := jobObjectExtendedLimitInfo{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitProcessMemory | jobObjectLimitKillOnJobClose
	info.ProcessMemoryLimit = m.capBytes
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

// hit 判断硬限制是否生效过。
//
// 上限比限额宽 8MB，所以"峰值够到限额 + 2MB"才算真的顶到上限：这样既不会
// 把只是接近限额的程序误判成 MLE，也能认出"分配失败但峰值没涨"之前的那
// 一小段（真正撞墙的程序峰值一定会越过余量）。
//
// 一次性申请超过上限的程序仍然认不出来（那次申请失败、不计入峰值），
// 这类还是 RE——与 Linux 侧同一个已知边界。
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
	return info.PeakProcessMemoryUsed >= m.limitBytes+jobMemGuardBytes
}

// close 关闭 Job 句柄。KILL_ON_JOB_CLOSE 会把还活着的子进程树杀掉，
// 因此正常路径（进程已退出）与超时路径都安全。
func (m *memLimiter) close() {
	if m != nil && m.handle != 0 {
		jobProcCloseHandle.Call(uintptr(m.handle))
		m.handle = 0
	}
}
