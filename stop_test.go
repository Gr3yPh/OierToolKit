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
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// mustCompile 编译一段临时 C++ 程序，环境没 g++ 或编译失败就跳过。
func mustCompile(t *testing.T, src, exe string) {
	t.Helper()
	log, ok, _ := compileProjectStopped(src, exe, map[string]string{"version": "c++17", "o2": "0"}, nil)
	if !ok {
		t.Skipf("编译失败，跳过：%v\n%s", src, log)
	}
}

// TestRunMeasuredStoppedKillsProcess 验证停止信号能在时限之内杀掉死循环进程。
func TestRunMeasuredStoppedKillsProcess(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "spin.cpp")
	writeText(t, src, "int main(){volatile long long i=0;for(;;)i++;}")

	exe := exePathIn(dir, "spin")
	mustCompile(t, src, exe)

	var stop atomic.Bool
	go func() {
		time.Sleep(200 * time.Millisecond)
		stop.Store(true)
	}()

	start := time.Now()
	res := runMeasuredStopped(exe, "", dir, 30, 0, stop.Load, nil) // 时限 30s，但我们 200ms 就停
	elapsed := time.Since(start)

	if !res.aborted {
		t.Fatalf("期望 aborted=true，实际 %+v", res)
	}
	if res.timedOut {
		t.Fatalf("不应标记为 TLE")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("停止响应太慢：%v", elapsed)
	}
}

// TestRunMeasuredStoppedNormalRun 验证不打断时行为与普通运行一致。
func TestRunMeasuredStoppedNormalRun(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "echo.cpp")
	writeText(t, src, `#include <cstdio>
int main(){int a,b;scanf("%d %d",&a,&b);printf("%d\n",a+b);}`)

	exe := exePathIn(dir, "echo")
	mustCompile(t, src, exe)

	in := filepath.Join(dir, "1.in")
	writeText(t, in, "1 2\n")

	res := runMeasuredStopped(exe, in, dir, 5, 0, func() bool { return false }, nil)
	if res.aborted {
		t.Fatalf("不应被中断")
	}
	if normalizeOutput(res.stdout) != "3" {
		t.Fatalf("输出不对：%q", res.stdout)
	}
}

// TestJudgeContestStop 验证整场评测能被中断：B 选手死循环，不打断就永不返回。
func TestJudgeContestStop(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()
	writeText(t, filepath.Join(dir, "data/p1/1.in"), "1 2\n")
	writeText(t, filepath.Join(dir, "data/p1/1.ans"), "3\n")
	writeText(t, filepath.Join(dir, "source/A/p1.cpp"), `#include <cstdio>
int main(){int a,b;scanf("%d %d",&a,&b);printf("%d\n",a+b);}`)
	writeText(t, filepath.Join(dir, "source/B/p1.cpp"), `int main(){volatile long long i=0;for(;;)i++;}`)

	c, err := importContest(dir, fmt.Sprintf("停止测试-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("importContest: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(c.Dir) })

	var stop atomic.Bool
	go func() {
		time.Sleep(1500 * time.Millisecond)
		stop.Store(true)
	}()

	start := time.Now()
	_, _ = judgeContest(c, nil, stop.Load)
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("中断没生效，耗时 %v", elapsed)
	}
}

// writeText 写文件并在必要时创建父目录。
func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
