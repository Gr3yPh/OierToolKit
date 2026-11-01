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
	"testing"
	"time"
)

// TestMemLimiterAssignsRLimit 确认 assign 真的把 RLIMIT_AS 下到了子进程上。
// 直接查 /proc/<pid>/limits 的 "Max address space" 行，比跑一个吃到上限的
// 程序更快也更稳。
func TestMemLimiterAssignsRLimit(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skipf("起不了子进程: %v", err)
	}
	m := newMemLimiter(64)
	m.assign(cmd)
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		m.close()
	}()

	if !m.assigned {
		t.Skip("prlimit64 未生效（可能权限受限），退化为事后测量")
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/limits", cmd.Process.Pid))
	if err != nil {
		t.Fatalf("读 /proc/%d/limits 失败: %v", cmd.Process.Pid, err)
	}
	// 目标行形如：Max address space   <soft>   <hard>   bytes
	var got string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Max address space") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				got = fields[3]
			}
		}
	}
	want := strconv.FormatInt(m.capKB*1024, 10)
	if got != want {
		t.Fatalf("子进程 RLIMIT_AS = %q，期望 %q", got, want)
	}
}

// TestMemLimiterIdle 检查没有内存压力的短命进程不会被误判成超限。
func TestMemLimiterIdle(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Skipf("起不了子进程: %v", err)
	}
	m := newMemLimiter(64)
	m.assign(cmd)
	_ = cmd.Wait()
	time.Sleep(2 * time.Millisecond) // 让采样协程至少跑过一拍
	defer m.close()

	if m.hit() {
		t.Fatal("一个只退出的进程被判成了顶到内存上限")
	}
	if _, ok := m.peakMB(); ok {
		t.Fatal("Linux 侧不应拿虚拟地址空间冒充上报内存")
	}
}
