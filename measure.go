//    OierToolKit - A lightweight tool designed for practicing and evaluating
//  in informatics competitions.
//    Copyright (C) 2026  魇珩Gr3yPh4ntom
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

// runResult 是一次受测运行的结果。
type runResult struct {
	wallMS   int64   // 墙钟运行时间（毫秒），由 Go 标准库直接测量
	memMB    float64 // 峰值常驻内存（MB），来自操作系统原生接口
	memOK    bool    // 内存测量是否可用
	timedOut bool    // 是否因超时被强制终止
	runErr   error   // 进程退出错误（timedOut 时为被杀死的错误）
	stdout   string
	stderr   string
}

// memProbeFunc 在目标进程结束（Wait 返回）后读取其峰值内存（MB）。
type memProbeFunc func() (float64, bool)

// runMeasured 在给定时间限制下运行程序，测量墙钟时间与峰值内存。
// 不依赖任何外部工具：计时用 Go 标准库，内存读取见各平台的 startMemProbe。
func runMeasured(exePath, inPath, workDir string, timeLimitSec float64) runResult {
	cmd := exec.Command(exePath)
	cmd.Dir = workDir
	if inPath != "" {
		f, err := os.Open(inPath)
		if err != nil {
			return runResult{runErr: err}
		}
		defer f.Close()
		cmd.Stdin = f
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return runResult{runErr: err}
	}
	// Windows 需要在进程存活期间打开句柄，Unix 实现则惰性读取 ProcessState。
	probe := startMemProbe(cmd)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	start := time.Now()
	var waitErr error
	timedOut := false
	select {
	case waitErr = <-done:
	case <-time.After(time.Duration(timeLimitSec * float64(time.Second))):
		timedOut = true
		_ = cmd.Process.Kill()
		waitErr = <-done // 回收子进程，避免僵尸
	}
	wallMS := time.Since(start).Milliseconds()

	memMB, memOK := probe()
	return runResult{
		wallMS:   wallMS,
		memMB:    memMB,
		memOK:    memOK,
		timedOut: timedOut,
		runErr:   waitErr,
		stdout:   outBuf.String(),
		stderr:   errBuf.String(),
	}
}
