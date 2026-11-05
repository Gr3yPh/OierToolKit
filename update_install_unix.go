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

//go:build !windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// finishUpdateInstall 在 Unix 上直接原子替换（运行中的可执行文件也可以
// 先 unlink/rename），然后启动新进程。调用方随后关闭旧窗口。
func finishUpdateInstall(dir, currentBase, tempDir string, sources map[string]string) error {
	if err := replaceUpdateFiles(dir, sources); err != nil {
		if !errors.Is(err, os.ErrPermission) {
			return err
		}
		// 程序通常装在 /usr/local/bin；普通用户没有写权限时走系统认证，
		// Linux 用 pkexec（PolicyKit 弹窗），macOS 用 osascript 的管理员授权。
		if err := replaceUpdateFilesElevated(dir, tempDir, sources); err != nil {
			return err
		}
	}

	exe := filepath.Join(dir, currentBase)
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func atomicReplaceFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".otk-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

func replaceUpdateFiles(dir string, sources map[string]string) error {
	names := sortedUpdateNames(sources)
	for _, name := range names {
		if err := atomicReplaceFile(sources[name], filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func sortedUpdateNames(sources map[string]string) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		if isUpdateBinaryName(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func replaceUpdateFilesElevated(dir, tempDir string, sources map[string]string) error {
	scriptPath := filepath.Join(tempDir, "elevated-update.sh")
	var b strings.Builder
	b.WriteString("#!/bin/sh\nset -e\n")
	for _, name := range sortedUpdateNames(sources) {
		src := sources[name]
		dst := filepath.Join(dir, name)
		fmt.Fprintf(&b, "cp -f %s %s\n", shellQuote(src), shellQuote(dst))
		fmt.Fprintf(&b, "chmod 755 %s\n", shellQuote(dst))
	}
	if err := os.WriteFile(scriptPath, []byte(b.String()), 0755); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		pkexec, err := exec.LookPath("pkexec")
		if err != nil {
			return fmt.Errorf("%s", tr("安装目录需要管理员权限，但系统中没有找到 pkexec；请手动更新。"))
		}
		cmd := exec.Command(pkexec, "/bin/sh", scriptPath)
		return cmd.Run()
	case "darwin":
		command := "/bin/sh " + shellQuote(scriptPath)
		apple := "do shell script " + appleScriptQuote(command) + " with administrator privileges"
		cmd := exec.Command("osascript", "-e", apple)
		return cmd.Run()
	default:
		return fmt.Errorf("%s", tr("当前平台无法请求管理员权限，请手动更新。"))
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
