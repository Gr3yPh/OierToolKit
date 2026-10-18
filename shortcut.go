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
	"path/filepath"
	"runtime"
)

// 桌面快捷方式：把当前可执行文件做成一个带图标的入口。
//
// 关键点是不能指向 otk.exe 本身——那样打开的是命令行模式。做法是把同一份
// 二进制复制成 otk-gui(.exe) 放进入口指向的位置，再让快捷方式指向副本，
// 程序启动时按文件名识别出 GUI 身份（见 mode.go）。

// shortcutTarget 返回 GUI 副本的路径：与可执行文件同目录。
func shortcutTarget() string {
	return filepath.Join(filepath.Dir(selfExe()), guiBinaryFileName())
}

// guiBinaryFileName 返回当前平台下 GUI 副本的文件名（otk-gui / otk-gui.exe）。
func guiBinaryFileName() string {
	name := "otk-gui"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// selfExe 返回当前可执行文件的绝对路径。
func selfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// ensureGuiCopy 确保 GUI 副本存在且是最新的（按大小与修改时间粗略判断）。
func ensureGuiCopy() (string, error) {
	src := selfExe()
	dst := shortcutTarget()
	if filepath.Clean(src) == filepath.Clean(dst) {
		// 本身就以 otk-gui 运行，不用复制。
		return dst, nil
	}

	if ok := copyFileIfStale(src, dst); ok {
		return dst, nil
	}
	return dst, nil
}

// copyFileIfStale 在目标缺失或比源旧时复制一次，返回是否执行了复制。
func copyFileIfStale(src, dst string) bool {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		// 大小一致且不比源旧，认为是最新的（够用，避免每次启动都复制几十 MB）。
		if dstInfo.Size() == srcInfo.Size() && !dstInfo.ModTime().Before(srcInfo.ModTime()) {
			return false
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return false
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		os.Remove(tmp)
		return false
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return false
	}
	_ = os.Chmod(tmp, 0755)
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return false
	}
	return true
}

// createDesktopShortcut 在当前用户的桌面创建 GUI 快捷方式。
func createDesktopShortcut() (string, error) {
	if _, err := ensureGuiCopy(); err != nil {
		return "", err
	}
	desktop, err := desktopDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(desktop, 0755); err != nil {
		return "", fmt.Errorf("无法访问桌面目录：%v", err)
	}
	return writeShortcut(desktop, shortcutTarget())
}

// shortcutExists 判断桌面快捷方式是否已经存在。
func shortcutExists() bool {
	desktop, err := desktopDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(shortcutPath(desktop))
	return err == nil
}

// desktopDir 返回当前用户的桌面目录（跨平台）。
func desktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位用户主目录：%v", err)
	}
	if dir := os.Getenv("XDG_DESKTOP_DIR"); dir != "" && runtime.GOOS == "linux" {
		return dir, nil
	}
	return filepath.Join(home, "Desktop"), nil
}
