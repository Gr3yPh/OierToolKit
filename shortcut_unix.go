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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

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
			return "", fmt.Errorf(tr("写入快捷方式失败：%v"), err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			return "", fmt.Errorf(tr("设置快捷方式权限失败：%v"), err)
		}
		return path, nil
	}

	// Linux：把内嵌 PNG 导出为图标文件，.desktop 里用绝对路径引用。
	iconPath, _ := ensureDesktopIconFile()
	if err := writeDesktopEntry(path, target, iconPath, 0755); err != nil {
		return "", err
	}
	return path, nil
}

// desktopEntryContent 生成 .desktop 内容。桌面快捷方式和应用菜单共用，
// 只是路径和权限不同。
func desktopEntryContent(target, iconPath string) string {
	lines := []string{
		"[Desktop Entry]",
		"Type=Application",
		"Version=1.0",
		"Name=OierToolKit",
		tr("Comment=本地评测 · 题目管理 · 对拍"),
		"Exec=" + desktopExecQuote(target),
	}
	if iconPath != "" {
		lines = append(lines, "Icon="+iconPath)
	}
	lines = append(lines,
		"Terminal=false",
		"Categories=Development;Education;",
		"StartupNotify=true",
		// X11 下 WM_CLASS 来自可执行文件名；Wayland 下则按 app_id
		// （也就是 appDesktopID.desktop 的文件名）匹配，不依赖这一行。
		"StartupWMClass=otk-gui",
		"",
	)
	return strings.Join(lines, "\n")
}

func desktopExecQuote(path string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", "\\$")
	return `"` + r.Replace(path) + `"`
}

func writeDesktopEntry(path, target, iconPath string, mode os.FileMode) error {
	if err := os.WriteFile(path, []byte(desktopEntryContent(target, iconPath)), mode); err != nil {
		return fmt.Errorf(tr("写入快捷方式失败：%v"), err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf(tr("设置快捷方式权限失败：%v"), err)
	}
	return nil
}

// applicationsDir 返回当前用户的应用菜单目录。
func applicationsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf(tr("无法定位用户主目录：%v"), err)
	}
	return filepath.Join(home, ".local", "share", "applications"), nil
}

// menuShortcutPath 返回应用菜单入口。Linux 下文件名必须等于 app_id，
// 否则 Wayland 合成器无法把窗口和图标关联起来。
func menuShortcutPath() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf(tr("无法定位用户主目录：%v"), err)
		}
		return filepath.Join(home, "Applications", "OierToolKit.app"), nil
	}
	dir, err := applicationsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDesktopID+".desktop"), nil
}

func menuShortcutExists() bool {
	path, err := menuShortcutPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// createMenuShortcut 注册应用菜单入口。Linux 下这一步同时解决 Wayland
// 任务栏图标：合成器只认 app_id 对应的 .desktop 文件，SetIcon 会被忽略。
func createMenuShortcut() (string, error) {
	if runtime.GOOS == "darwin" {
		return createDarwinAppBundle()
	}

	target, err := ensureGuiCopy()
	if err != nil {
		return "", err
	}
	path, err := menuShortcutPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf(tr("无法访问应用菜单目录：%v"), err)
	}
	iconPath, _ := ensureDesktopIconFile()
	if err := writeDesktopEntry(path, target, iconPath, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func removeMenuShortcut() (string, error) {
	path, err := menuShortcutPath()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		if err := os.RemoveAll(path); err != nil {
			return "", err
		}
		return path, nil
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

// autoInstallMenuShortcut 在启动时补一次应用菜单入口。
//
// 只对稳定路径生效：go run 的临时二进制不能写进菜单，否则一退出就坏。
// 安装版启动后会自动注册，Wayland 任务栏从下一次启动起就能拿到图标；
// 用户在设置里主动移除过的话不再自动加回来。
func autoInstallMenuShortcut() {
	if runtime.GOOS != "linux" || isTempExecutable() || !guiPrefBool(prefMenuShortcut, true) {
		return
	}
	target := shortcutTarget()
	if _, err := os.Stat(target); err != nil {
		return
	}
	path, err := menuShortcutPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	iconPath, _ := ensureDesktopIconFile()
	_ = writeDesktopEntry(path, target, iconPath, 0644)
}

// ensureDesktopIconFile 把内嵌图标导出到用户图标目录。
// 不写可执行文件所在目录：/usr/local/bin 通常不可写，会导致快捷方式没有图标。
func ensureDesktopIconFile() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		dir := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps")
		if err := os.MkdirAll(dir, 0755); err == nil {
			path := filepath.Join(dir, appDesktopID+".png")
			if err := os.WriteFile(path, guiAppIconData, 0644); err == nil {
				return path, nil
			}
		}
	}

	// 极端情况下退回可执行文件同目录。
	path := filepath.Join(filepath.Dir(selfExe()), "otk-icon.png")
	if err := os.WriteFile(path, guiAppIconData, 0644); err != nil {
		return "", fmt.Errorf(tr("无法写入图标文件：%v"), err)
	}
	return path, nil
}

// createDarwinAppBundle 在 ~/Applications 生成一个最小 .app 包，
// 让 Finder/Launchpad 能以应用形式启动。图标使用 PNG，部分 macOS
// 版本会忽略它，但至少入口和名称是正确的。
func createDarwinAppBundle() (string, error) {
	target, err := ensureGuiCopy()
	if err != nil {
		return "", err
	}
	appDir, err := menuShortcutPath()
	if err != nil {
		return "", err
	}
	contents := filepath.Join(appDir, "Contents")
	macosDir := filepath.Join(contents, "MacOS")
	resources := filepath.Join(contents, "Resources")
	if err := os.MkdirAll(macosDir, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(resources, 0755); err != nil {
		return "", err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>OierToolKit</string>
  <key>CFBundleDisplayName</key><string>OierToolKit</string>
  <key>CFBundleIdentifier</key><string>` + appDesktopID + `</string>
  <key>CFBundleExecutable</key><string>OierToolKit</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>1.0</string>
  <key>CFBundleShortVersionString</key><string>` + otkVersion + `</string>
  <key>CFBundleIconFile</key><string>icon.png</string>
  <key>LSMinimumSystemVersion</key><string>10.13</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0644); err != nil {
		return "", err
	}
	script := "#!/bin/sh\nexec \"" + target + "\" >/dev/null 2>&1 &\n"
	exe := filepath.Join(macosDir, "OierToolKit")
	if err := os.WriteFile(exe, []byte(script), 0755); err != nil {
		return "", err
	}
	_ = os.WriteFile(filepath.Join(resources, "icon.png"), guiAppIconData, 0644)
	return appDir, nil
}
