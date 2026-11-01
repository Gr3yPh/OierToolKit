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
	"os/exec"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// Linux 上用 RLIMIT_AS 给被测程序设内存硬上限，对应 Windows 那边的 Job Object：
//   - 超限时 mmap/brk 直接失败（C++ 里表现为 std::bad_alloc），
//     不会像"事后测量"那样先把机器内存吃穿再判 MLE；
//   - 限制来自内核，程序 fork 出的子进程一并继承，逃不掉。
//
// 为什么是 AS 而不是 DATA：实测（scripts 里的探测）普通 C++ 程序基线的
// VmSize 只有 2.7MB，栈与动态库带来的偏差可忽略；而 RLIMIT_DATA 在各内核
// 版本上对匿名映射的统计口径并不一致。AS 是 Linux 评测机的通行做法。
//
// 为什么不用 cgo 包 setrlimit：对**已启动**的子进程直接调用 prlimit64 就够了，
// 不需要在子进程里跑任何代码，也就不需要 cgo。

// 硬上限比题目限额留一段余量，判定线再落在余量中间。
//
// 为什么不能把 RLIMIT_AS 直接设成题目限额：命中判定只能靠采样 /proc 的
// VmPeak，而进程退出瞬间 /proc 就没了，最后一拍必然落后几毫秒——一个每毫秒
// 分配 2~3MB 的程序，峰值会停在"限额 - 好几 MB"，于是"刚好够到上限"这种
// 精确判定根本不成立（实测：限额 125MB 的程序真的撞到上限，采样却只看到
// 120.75MB，判成了 WA）。
//
// 所以上限放到"限额 + 余量"，判定线放在"限额 + 余量/4"：撞到上限的程序
// 峰值一定落在判定线之上，而仅仅接近限额的程序不会被误伤。多出的几 MB
// 不影响判定正确性（真正的口径仍是测到的驻留内存），也依然守住机器内存。
const (
	memLimitSlackKB    = 8 * 1024
	memLimitHitGuardKB = 2 * 1024
)

// memLimiter 控制一个被测进程的内存上限；nil 表示不限制。
type memLimiter struct {
	limitKB  int64 // 题目内存限额（判定基准）
	capKB    int64 // 实际下发的 RLIMIT_AS
	pid      int
	assigned bool // prlimit64 是否成功：失败就退化成"事后测量"，不能误判

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}

	mu     sync.Mutex
	peakKB int64 // 采样到的 VmPeak 峰值
}

// newMemLimiter 准备一个限制器；memMB <= 0 返回 nil（不限制）。
// 真正的 prlimit64 在 assign 里做，因为要等进程启动后才有 PID。
func newMemLimiter(memMB float64) *memLimiter {
	if memMB <= 0 {
		return nil
	}
	limitKB := int64(memMB * 1024)
	return &memLimiter{
		limitKB: limitKB,
		capKB:   limitKB + memLimitSlackKB,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// assign 对已启动的子进程下调 RLIMIT_AS。
//
// 这里存在一个微秒级的窗口：进程已经 exec 完、但限制还没落地时，
// 它理论上可以先分配一大块。Windows 的 Job Object 同样是 Start 之后才挂，
// 行为一致；而 .bss 里的大数组在真正触碰前不占页，实际影响可忽略。
func (m *memLimiter) assign(cmd *exec.Cmd) {
	if m == nil {
		return
	}
	if cmd.Process == nil {
		close(m.done) // 没有进程就没采样协程，让 close 不至于等死
		return
	}
	m.pid = cmd.Process.Pid
	lim := unix.Rlimit{Cur: uint64(m.capKB) * 1024, Max: uint64(m.capKB) * 1024}
	if err := unix.Prlimit(m.pid, unix.RLIMIT_AS, &lim, nil); err != nil {
		// 设不上（比如超过系统硬上限、权限不足）就不假装有限制。
		close(m.done)
		return
	}
	m.assigned = true
	go m.sample()
}

// sample 在进程存活期间记录 VmPeak 的峰值。
//
// VmPeak 是内核维护的单调峰值，晚读不会漏掉早先的尖峰；密集采样只是为了
// "在进程消失前至少读到一次"。进程一退出 /proc/<pid> 就没了，所以采样
// 必须在运行期间持续进行，不能等 Wait 返回后再读。
func (m *memLimiter) sample() {
	defer close(m.done)
	path := fmt.Sprintf("/proc/%d/status", m.pid)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			if kb, ok := readVmPeak(path); ok {
				m.mu.Lock()
				if kb > m.peakKB {
					m.peakKB = kb
				}
				m.mu.Unlock()
			}
		}
	}
}

// peakMB 返回 false：Linux 侧的上报内存仍用 VmHWM（真实驻留）那一份，
// 不能拿虚拟地址空间充数，否则程序看到的数字会比 Windows 大一截。
func (m *memLimiter) peakMB() (float64, bool) { return 0, false }

// hit 判断是否顶到了硬上限。做出失败的那次 mmap 本身不会计入 VmPeak
// （申请失败就回滚了），采样还会落后于进程退出，所以判定线取在
// "限额 + 余量/4"：真正撞上上限的程序峰值必然越过这条线，
// 而只是接近限额、正常跑完的程序不会被误伤。
func (m *memLimiter) hit() bool {
	if m == nil || !m.assigned {
		return false
	}
	m.mu.Lock()
	peak := m.peakKB
	m.mu.Unlock()
	if peak <= 0 {
		return false
	}
	return peak >= m.limitKB+memLimitHitGuardKB
}

// close 停止采样。进程此时已退出，/proc 早已消失，停不停都读不到新值，
// 这里只是收拾协程。
func (m *memLimiter) close() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() { close(m.stop) })
	<-m.done
}
