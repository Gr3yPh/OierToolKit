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
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// parseURL 把字符串转成超链接控件需要的 *url.URL（非法地址返回 nil）。
func parseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

// detached 启动一个与当前进程脱钩的 GUI/终端程序，不阻塞界面。
func detached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// openPath 用系统默认方式打开目录（资源管理器 / Finder / xdg-open）。
func openPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := detached(cmd); err != nil {
		return fmt.Errorf("无法打开 %s: %v", path, err)
	}
	return nil
}

// launchEditor 用全局配置的编辑器打开项目源码。
func launchEditor(p project) error {
	editor := propsOr(readIni(globalConfigPath()), "editor", defaultEditorCommand())
	parts := splitQuoted(fmt.Sprintf(editor, p.sourcePath()))
	if len(parts) == 0 {
		return fmt.Errorf("无效的编辑器命令，请在设置里修改")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = p.Dir
	hideProcWindow(cmd) // 防止控制台类编辑器闪黑框；GUI 编辑器窗口不受影响
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开编辑器 %s 失败: %v（可在设置里改成自己的编辑器）", parts[0], err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// launchGdb 在终端里用 gdb 调试已编译出的可执行文件——gdb 需要真实终端。
func launchGdb(p project) error {
	if !fileExists(p.exePath()) {
		return fmt.Errorf("未找到可执行文件，先运行一次评测或编译")
	}
	return openTerminal(p.Dir, "gdb --args "+quoteArg(p.exePath()))
}

// launchWizard 在项目目录里开一个终端跑 otk wizard。
func launchWizard(dir string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = "otk"
	}
	return openTerminal(dir, quoteArg(exe)+" wizard")
}

// openTerminal 在 dir 下打开系统终端并执行 command。
func openTerminal(dir, command string) error {
	switch runtime.GOOS {
	case "windows":
		inner := "cd /d " + quoteArg(dir) + " && " + command
		cmd := exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", inner)
		if err := detached(cmd); err != nil {
			return fmt.Errorf("无法启动终端: %v", err)
		}
		return nil

	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal" to do script "cd %s && %s"`, dir, command)
		cmd := exec.Command("osascript", "-e", script)
		if err := detached(cmd); err != nil {
			return fmt.Errorf("无法启动终端: %v", err)
		}
		return nil

	default:
		inner := fmt.Sprintf("cd %s && %s; exec $SHELL", quoteArg(dir), command)
		candidates := [][]string{
			{"x-terminal-emulator", "-e", "sh", "-c", inner},
			{"gnome-terminal", "--", "sh", "-c", inner},
			{"konsole", "-e", "sh", "-c", inner},
			{"xfce4-terminal", "-e", inner},
			{"xterm", "-e", "sh", "-c", inner},
		}
		for _, c := range candidates {
			if _, err := exec.LookPath(c[0]); err != nil {
				continue
			}
			cmd := exec.Command(c[0], c[1:]...)
			cmd.Dir = dir
			if err := detached(cmd); err != nil {
				continue
			}
			return nil
		}
		return fmt.Errorf("没有找到可用的终端程序（试过 x-terminal-emulator / gnome-terminal / konsole / xterm）")
	}
}

// quoteArg 给含空格的路径加双引号（cmd/sh 都认这种最简形式）。
func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t") && !strings.HasPrefix(s, `"`) {
		return `"` + s + `"`
	}
	return s
}
