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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed logo.png
var guiIconPngData []byte

// shortcutPath 返回桌面快捷方式文件路径。
func shortcutPath(desktop string) string {
	if runtime.GOOS == "darwin" {
		// macOS 用 .command 脚本：双击即在图形环境里启动可执行文件。
		return filepath.Join(desktop, "OierToolKit.command")
	}
	return filepath.Join(desktop, "OierToolKit.desktop")
}

// writeShortcut 在桌面目录写一个启动 GUI 的快捷方式。
func writeShortcut(desktop, target string) (string, error) {
	path := shortcutPath(desktop)

	if runtime.GOOS == "darwin" {
		// macOS 的 .command 只是 shell 脚本，Finder 图标来自 .app 包结构，
		// 单文件做不了自定义图标；保持脚本最简。
		content := "#!/bin/sh\nexec \"" + target + "\" >/dev/null 2>&1 &\n"
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			return "", fmt.Errorf("写入快捷方式失败：%v", err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			return "", fmt.Errorf("设置快捷方式权限失败：%v", err)
		}
		return path, nil
	}

	// Linux：把内嵌 PNG 导出为图标文件，.desktop 里用绝对路径引用。
	iconPath, iconErr := ensureDesktopIconFile()
	iconLine := ""
	if iconErr == nil {
		iconLine = "Icon=" + iconPath
	}

	content := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Version=1.0",
		"Name=OierToolKit",
		"Comment=本地评测 · 题目管理 · 对拍",
		`Exec="` + target + `"`,
		iconLine,
		"Terminal=false",
		"Categories=Development;Education;",
		"StartupNotify=true",
		"",
	}, "\n")
	content = strings.ReplaceAll(content, "\n\n", "\n")

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("写入快捷方式失败：%v", err)
	}
	// 部分桌面环境要求 .desktop 文件带可执行位才算"可信"。
	if err := os.Chmod(path, 0755); err != nil {
		return "", fmt.Errorf("设置快捷方式权限失败：%v", err)
	}
	return path, nil
}

// ensureDesktopIconFile 把内嵌 logo 导出为 PNG 图标文件（与可执行文件同目录）。
// .desktop 的 Icon= 支持绝对路径，导出一份最稳，不依赖主题包。
func ensureDesktopIconFile() (string, error) {
	path := filepath.Join(filepath.Dir(selfExe()), "otk-icon.png")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, guiIconPngData, 0644); err != nil {
		return "", fmt.Errorf("无法写入图标文件：%v", err)
	}
	return path, nil
}
