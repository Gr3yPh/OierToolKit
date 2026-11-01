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

//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// startMemProbe 在 Linux 上采样 /proc/<pid>/status 的 VmHWM（峰值常驻内存）。
//
// 为什么不用 wait4 的 ru_maxrss：内核在 exec_mmap() 里会
// setmax_mm_hiwater_rss(&tsk->signal->maxrss, old_mm)，把 fork 继承来的那块
// 地址空间的峰值 RSS 记进 signal->maxrss；getrusage()/wait4 取的是
// max(当前 mm 峰值, signal->maxrss)，而 signal->maxrss 只增不减。Go 在 Linux
// 用 clone(CLONE_VFORK|CLONE_VM) 起子进程，于是被测程序的 ru_maxrss 至少等于
// 拉起它的父进程的 RSS——otk-gui 常驻 500MB 时，每个测试点都报 ~514MB，
// 全部超过内存限制被判成 MLE（Windows 走 CreateProcess + Job，没有这套机制）。
//
// VmHWM 来自被测程序自己的 mm，exec 后是全新的地址空间，不受父进程影响。
func startMemProbe(cmd *exec.Cmd) memProbeFunc {
	if cmd == nil || cmd.Process == nil {
		return func() (float64, bool) { return 0, false }
	}
	path := fmt.Sprintf("/proc/%d/status", cmd.Process.Pid)

	stop := make(chan struct{})
	done := make(chan struct{})

	var (
		mu     sync.Mutex
		peakKB int64
		seen   bool
	)

	go func() {
		defer close(done)
		sample := func() {
			kb, ok := readVmHWM(path)
			if !ok {
				return
			}
			mu.Lock()
			if kb > peakKB {
				peakKB = kb
			}
			seen = true
			mu.Unlock()
		}

		// 先立刻抓一次：cout 1 这种几毫秒就结束的程序只有这一次机会。
		sample()

		// VmHWM 本身就是内核维护的峰值，晚读也不会漏掉尖峰；密度只影响
		// "能不能在进程消失前读到"，所以前 100ms 密采样，之后放稀。
		timer := time.NewTimer(time.Millisecond)
		defer timer.Stop()
		ticks := 0
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				sample()
				ticks++
				next := time.Millisecond
				if ticks >= 100 {
					next = 5 * time.Millisecond
				}
				timer.Reset(next)
			}
		}
	}()

	return func() (float64, bool) {
		close(stop)
		<-done
		mu.Lock()
		defer mu.Unlock()
		if !seen {
			return 0, false
		}
		return float64(peakKB) / 1024.0, true
	}
}

// readVmHWM 读取 /proc/<pid>/status 里的 VmHWM 行（峰值常驻内存，单位 kB）。
// 进程已退出时文件不存在，返回 false。
func readVmHWM(path string) (int64, bool) { return readStatusKB(path, "VmHWM:") }

// readVmPeak 读取 /proc/<pid>/status 里的 VmPeak 行（峰值虚拟地址空间，单位 kB）。
// 内存硬限制用 RLIMIT_AS，卡的就是虚拟地址空间，所以判定"顶到上限"要读这一项。
func readVmPeak(path string) (int64, bool) { return readStatusKB(path, "VmPeak:") }

func readStatusKB(path, field string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, field) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb, true
	}
	return 0, false
}
