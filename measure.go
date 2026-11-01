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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// fileIO 描述文件读写式题目的输入输出文件名（相对程序运行目录）。
// nil 或 In 为空表示标准输入输出。
type fileIO struct {
	In  string // 程序读取的输入文件名，如 "apple.in"
	Out string // 程序写出的答案文件名，如 "apple.out"
}

func (f *fileIO) enabled() bool { return f != nil && f.In != "" }

// runResult 是一次受测运行的结果。
type runResult struct {
	wallMS      int64   // 墙钟运行时间（毫秒），由 Go 标准库直接测量
	memMB       float64 // 峰值常驻内存（MB），来自操作系统原生接口
	memOK       bool    // 内存测量是否可用
	timedOut    bool    // 是否因超时被强制终止
	aborted     bool    // 是否因调用方请求停止而被强制终止
	memLimitHit bool    // 是否顶到内存硬上限（Windows Job Object）
	runErr      error   // 进程退出错误（timedOut/aborted 时为被杀死的错误）
	stdout      string
	stderr      string
}

// memProbeFunc 在目标进程结束（Wait 返回）后读取其峰值内存（MB）。
type memProbeFunc func() (float64, bool)

// runMeasured 在给定时间限制下运行程序，测量墙钟时间与峰值内存。
// 不依赖任何外部工具：计时用 Go 标准库，内存读取见各平台的 startMemProbe。
// memLimitMB > 0 时施加内存硬上限（Windows Job Object；其它平台忽略）。
func runMeasured(exePath, inPath, workDir string, timeLimitSec, memLimitMB float64) runResult {
	return runMeasuredStopped(exePath, inPath, workDir, timeLimitSec, memLimitMB, nil, nil)
}

// runMeasuredStopped 与 runMeasured 相同，但额外接受一个停止信号：
// 一旦 stopped 返回 true，立即杀死被测进程并返回 aborted=true 的结果。
// 用于评测中途取消，避免只能等当前测试点跑完才响应。
//
// fio 非 nil 时按文件读写方式运行：把 inPath 复制为运行目录下的 fio.In，
// 结束后把 fio.Out 的内容作为 stdout 返回（文件不存在视为空输出）。
// 程序在一个独享的临时目录里运行——并发评测多个测试点时互不踩踏，
// 也避免在项目目录里留下中间文件。
func runMeasuredStopped(exePath, inPath, workDir string, timeLimitSec, memLimitMB float64, stopped func() bool, fio *fileIO) runResult {
	runDir := workDir
	if fio.enabled() {
		tmp, err := os.MkdirTemp(workDir, ".otk-io-")
		if err != nil {
			return runResult{runErr: err}
		}
		defer os.RemoveAll(tmp)
		if err := copyFile(inPath, filepath.Join(tmp, fio.In)); err != nil {
			return runResult{runErr: err}
		}
		runDir = tmp
	}

	cmd := exec.Command(exePath)
	cmd.Dir = runDir
	hideProcWindow(cmd)
	if inPath != "" && !fio.enabled() {
		f, err := os.Open(inPath)
		if err != nil {
			return runResult{runErr: err}
		}
		defer f.Close()
		cmd.Stdin = f
	}
	var outBuf, errBuf syncBuffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return runResult{runErr: err}
	}
	// 内存硬限制（Windows Job Object）：超限的程序分配直接失败，
	// 不会把机器内存吃穿。assign 在 Start 之后尽快调用。
	limiter := newMemLimiter(memLimitMB)
	defer limiter.close()
	limiter.assign(cmd)
	// Windows 需要在进程存活期间打开句柄，Unix 实现则惰性读取 ProcessState。
	probe := startMemProbe(cmd)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// 停止信号轮询：进程一旦被取消就立刻杀掉，不必等满时限。
	var cancelWatch chan struct{}
	var cancelOnce sync.Once
	watchDone := make(chan struct{})
	if stopped != nil {
		cancelWatch = make(chan struct{})
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-watchDone:
					return
				case <-ticker.C:
					if stopped() {
						cancelOnce.Do(func() { close(cancelWatch) })
						return
					}
				}
			}
		}()
	}
	stopWatch := func() {
		if cancelWatch != nil {
			close(watchDone)
		}
	}
	defer stopWatch()

	start := time.Now()
	var waitErr error
	timedOut := false
	aborted := false

	limit := time.Duration(timeLimitSec * float64(time.Second))
	// 时限与取消信号二选一，谁先到就杀进程。
	timer := time.NewTimer(limit)
	defer timer.Stop()

	var cancelCh <-chan struct{}
	if cancelWatch != nil {
		cancelCh = cancelWatch
	}

	select {
	case waitErr = <-done:
	case <-timer.C:
		timedOut = true
		_ = cmd.Process.Kill()
		waitErr = <-done
	case <-cancelCh:
		aborted = true
		_ = cmd.Process.Kill()
		waitErr = <-done
	}
	wallMS := time.Since(start).Milliseconds()

	memMB, memOK := probe()
	// 有硬限制时优先用 Job 的提交内存峰值：比 working set 采样更准，
	// 也不会漏掉"分配成功但没碰页"的内存。
	if p, ok := limiter.peakMB(); ok {
		memMB, memOK = p, true
	}

	stdout := outBuf.String()
	stderr := errBuf.String()
	if fio.enabled() {
		// 答案在文件里：读 fio.Out 作为程序输出；没写出来视为空输出，
		// 会在比较阶段得到 WA（评测语义上这比报错更有用）。
		data, err := os.ReadFile(filepath.Join(runDir, fio.Out))
		if err == nil {
			stdout = string(data)
		} else if fio.Out != "" {
			stderr += tr("\n[otk] 程序未写出 ") + fio.Out
		}
	}

	return runResult{
		wallMS:      wallMS,
		memMB:       memMB,
		memOK:       memOK,
		timedOut:    timedOut,
		aborted:     aborted,
		memLimitHit: limiter.hit(),
		runErr:      waitErr,
		stdout:      stdout,
		stderr:      stderr,
	}
}

// syncBuffer 是带锁的字符串缓冲：Wait 返回后可能仍有子进程的
// 输出协程在写，读之前需要保证不并发。
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
