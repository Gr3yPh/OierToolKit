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
	"strings"
)

// appDesktopID 同时是 Fyne 的 UniqueID、Wayland app_id 和 Linux 应用菜单
// 文件名。Wayland 下窗口/任务栏图标就是靠这个 ID 去匹配 .desktop 的，
// 所以这三处必须保持一致。
const appDesktopID = "cn.gr3yph4ntom.otk"

// 桌面快捷方式：把当前可执行文件做成一个带图标的入口。
//
// 关键点是不能指向 otk.exe 本身——那样打开的是命令行模式。做法是把同一份
// 二进制复制成 otk-gui(.exe) 放进入口指向的位置，再让快捷方式指向副本，
// 程序启动时按文件名识别出 GUI 身份（见 mode.go）。

// shortcutTarget 返回 GUI 副本的路径。
//
// 以 otk-gui 运行时就用同目录的自身；以 otk gui 运行时优先复用同目录
// 的 otk-gui。若当前进程来自 go-build 临时目录或只读目录，入口不能指向
// 那里，否则程序一退出快捷方式就失效，此时落到 ~/.local/bin/otk-gui。
func shortcutTarget() string {
	dir := filepath.Dir(selfExe())
	if isGuiBinary() {
		return filepath.Join(dir, guiBinaryFileName())
	}
	// otk gui 启动时，安装脚本通常已经把 otk-gui 放在同一个 bin 目录，
	// 直接复用它，避免再往 ~/.local/bin 复制一份。
	sibling := filepath.Join(dir, guiBinaryFileName())
	if st, err := os.Stat(sibling); err == nil && !st.IsDir() {
		return sibling
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".local", "bin", guiBinaryFileName())
	}
	return sibling
}

// isTempExecutable 判断当前进程是否来自 go run 的临时构建目录。
func isTempExecutable() bool {
	exe := filepath.Clean(selfExe())
	if strings.Contains(exe, string(filepath.Separator)+"go-build") {
		return true
	}
	tmp := filepath.Clean(os.TempDir())
	return tmp != "" && strings.HasPrefix(exe, tmp+string(filepath.Separator))
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

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", fmt.Errorf(tr("无法创建 GUI 副本目录：%v"), err)
	}
	_ = copyFileIfStale(src, dst)
	if _, err := os.Stat(dst); err != nil {
		return "", fmt.Errorf(tr("无法创建 GUI 副本：%v"), err)
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
		return "", fmt.Errorf(tr("无法访问桌面目录：%v"), err)
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
		return "", fmt.Errorf(tr("无法定位用户主目录：%v"), err)
	}
	if runtime.GOOS == "linux" {
		if dir := linuxDesktopDir(home); dir != "" {
			return dir, nil
		}
	}
	return filepath.Join(home, "Desktop"), nil
}

// linuxDesktopDir 解析 XDG 桌面目录。
//
// 很多中文桌面只把 XDG_DESKTOP_DIR 写进 ~/.config/user-dirs.dirs，
// 并不会导出到进程环境里；只读环境变量就会误判成 ~/Desktop，
// 于是快捷方式“创建成功”却不出现在真正的桌面上。
func linuxDesktopDir(home string) string {
	if dir := expandXDGPath(os.Getenv("XDG_DESKTOP_DIR"), home); dir != "" {
		return dir
	}
	if data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "XDG_DESKTOP_DIR=") {
				continue
			}
			if dir := expandXDGPath(strings.TrimPrefix(line, "XDG_DESKTOP_DIR="), home); dir != "" {
				return dir
			}
		}
	}
	// 没有配置时优先用真实存在的目录，兼容中文桌面。
	for _, name := range []string{"Desktop", "桌面"} {
		dir := filepath.Join(home, name)
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	return ""
}

func expandXDGPath(value, home string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "${HOME}", home)
	value = strings.ReplaceAll(value, "$HOME", home)
	if strings.HasPrefix(value, "~/") {
		value = filepath.Join(home, value[2:])
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(home, value)
	}
	return filepath.Clean(value)
}
