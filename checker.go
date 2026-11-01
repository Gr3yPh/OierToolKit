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
	"strings"
	"time"
)

// 特殊判题（SPJ）：项目目录里放一个 checker.cpp 即启用。
// 约定与 testlib 一致，按位置传三个参数：
//
//	checker <输入文件> <选手输出> <期望输出>
//
// 退出码 0 判 AC，非 0 判 WA，stderr 作为反馈展示给选手。

// checkerSourceName 是特殊判题源码的固定文件名。
const checkerSourceName = "checker.cpp"

// checkerExePath 返回 checker 的编译产物路径（放在 .otk 里，不污染项目目录）。
func checkerExePath(p project) string {
	return filepath.Join(p.metaDir(), "checker"+exeSuffix())
}

// spjEnabled 返回当前项目是否应使用特殊判题：
// checker.cpp 存在且配置没有显式 spj=off 时启用。
func spjEnabled(p project, props map[string]string) bool {
	if strings.ToLower(strings.TrimSpace(props["spj"])) == "off" {
		return false
	}
	return fileExists(filepath.Join(p.Dir, checkerSourceName))
}

// prepareChecker 确保 checker 已编译为最新，返回 (exe 路径, 编译日志, 是否可用)。
// 未启用 SPJ 时返回空路径且 ok=true。产物比源码新时直接复用，否则重新编译。
func prepareChecker(p project, props map[string]string) (string, string, bool) {
	if !spjEnabled(p, props) {
		return "", "", true
	}
	src := filepath.Join(p.Dir, checkerSourceName)
	exe := checkerExePath(p)

	if fiSrc, err := os.Stat(src); err == nil {
		if fiExe, err := os.Stat(exe); err == nil && !fiExe.ModTime().Before(fiSrc.ModTime()) {
			return exe, "", true
		}
	}
	// 编译产物放进 .otk，目录不存在时先建好。
	if err := os.MkdirAll(p.metaDir(), 0755); err != nil {
		return "", tr("无法创建 .otk 目录: ") + err.Error(), false
	}
	log, ok := compileProjectTo(src, exe, map[string]string{"version": propsOr(props, "version", "c++17")})
	if !ok {
		return "", log, false
	}
	return exe, "", true
}

// runChecker 调用 checker 判一个测试点。返回是否通过（exit 0）与反馈文本。
// checker 本身 15 秒超时：正常判题器毫秒级返回，超时说明写挂了。
func runChecker(exe, inPath, actualPath, expectedPath string) (bool, string) {
	cmd := exec.Command(exe, inPath, actualPath, expectedPath)
	hideProcWindow(cmd)
	var buf syncBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return false, tr("checker 启动失败: ") + err.Error()
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		out := strings.TrimSpace(buf.String())
		switch {
		case err == nil:
			return true, out
		case out != "":
			return false, out
		default:
			return false, fmt.Sprintf(tr("checker 退出码 %d"), exitCodeOf(err))
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return false, tr("checker 运行超时（15s），请检查 checker.cpp")
	}
}

// exitCodeOf 从 exec 错误里取退出码，取不到时返回 -1。
func exitCodeOf(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// spjRunOnOutput 把选手输出（内存中的字符串）落成临时文件后调用 checker，
// 返回是否通过与反馈文本。文件读写与标准输入输出两种模式共用。
func spjRunOnOutput(checker, inPath, stdout, expectedPath string) (bool, string) {
	tmp, err := os.CreateTemp(filepath.Dir(expectedPath), ".otk-chk-*.out")
	if err != nil {
		return false, tr("无法写出选手输出供 checker 使用: ") + err.Error()
	}
	actualPath := tmp.Name()
	if _, err := tmp.WriteString(stdout); err != nil {
		tmp.Close()
		_ = os.Remove(actualPath)
		return false, tr("无法写出选手输出供 checker 使用: ") + err.Error()
	}
	tmp.Close()
	defer os.Remove(actualPath)
	return runChecker(checker, inPath, actualPath, expectedPath)
}

// judgeWithChecker 在普通判定之后追加特殊判题。只在 stdout/stdin 类判定
// （default 分支）之后调用，TLE/MLE/RE 不再问 checker。
func judgeWithChecker(job judgeJob, pt *pointResult, task pointJob, stdout string) {
	ok, msg := spjRunOnOutput(job.Checker, task.in, stdout, task.out)
	if ok {
		pt.Verdict = verdictAC
		pt.Detail = msg
		return
	}
	pt.Verdict = verdictWA
	pt.Detail = msg
}
