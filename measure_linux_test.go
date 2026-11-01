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
	"os/exec"
	"testing"
)

// TestMemProbeIgnoresParentRSS 守护一个曾在 Linux 上把所有测试点判成 MLE 的回归：
//
// 内核把 fork 继承来的地址空间峰值记进 signal->maxrss，wait4 的 ru_maxrss 因此
// 至少等于父进程的 RSS；otk-gui 常驻数百 MB 时，每个被测程序都报同一个几百 MB，
// 全部超限。这里先把本进程撑到 256MB 再拉起一个几乎不占内存的子进程，
// 采样到的峰值必须远小于父进程的占用。
func TestMemProbeIgnoresParentRSS(t *testing.T) {
	const balloonMB = 256

	// 撑大本进程的 RSS（每页写一次，确保真正驻留而不是只占虚拟地址）。
	balloon := make([]byte, balloonMB<<20)
	for i := 0; i < len(balloon); i += 4096 {
		balloon[i] = 1
	}
	defer func() { _ = balloon }()

	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动子进程失败: %v", err)
	}
	probe := startMemProbe(cmd)
	_ = cmd.Wait()

	mb, ok := probe()
	if !ok {
		t.Fatal("没有采到子进程的内存峰值")
	}
	// 留足余量：正常应是 1~2MB 量级，只有继承了父进程才会落到 256MB 附近。
	if mb > 64 {
		t.Fatalf("子进程峰值内存 %.2fMB，明显继承了父进程的 %dMB", mb, balloonMB)
	}
}
