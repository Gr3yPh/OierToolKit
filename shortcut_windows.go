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
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/icon.ico
var guiIcoData []byte

// shortcutPath 返回桌面快捷方式文件路径（Windows 用 .lnk）。
func shortcutPath(desktop string) string {
	return filepath.Join(desktop, "OierToolKit.lnk")
}

// writeShortcut 在桌面创建指向 GUI 副本的 .lnk 文件。
//
// 不引入第三方库：直接调用 Windows Script Host 的 WScript.Shell 来创建
// 快捷方式，这是系统自带的稳妥做法（不需要管理员权限）。
// 图标指向同目录的 otk.ico——exe 自身也带图标，但 ICO 文件更直观：
// 用户改过 otk-gui.exe 的名字时快捷方式也不会丢图标。
func writeShortcut(desktop, target string) (string, error) {
	path := shortcutPath(desktop)
	// 图标优先用随程序发布的 otk.ico（首次创建时从内嵌资源导出）；
	// 导出失败就退回 exe 自带的图标。
	icon := target
	if p, err := ensureIcoFile(); err == nil {
		icon = p
	}

	// 路径里可能有空格和中文，全部用单引号包住并转义内层单引号。
	script := fmt.Sprintf(
		"$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s');"+
			"$s.TargetPath='%s';"+
			"$s.WorkingDirectory='%s';"+
			"$s.IconLocation='%s,0';"+
			"$s.WindowStyle=7;"+ // 7 = 最小化启动，配合程序内隐藏控制台，避免黑框闪一下
			"$s.Description='OierToolKit 本地评测与题目管理';"+
			"$s.Save()",
		psQuote(path), psQuote(target), psQuote(filepath.Dir(target)), psQuote(icon),
	)

	if err := runPowerShell(script); err != nil {
		return "", fmt.Errorf("创建快捷方式失败：%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("快捷方式没有生成：%v", err)
	}
	return path, nil
}

// icoFilePath 返回随程序发布的 ICO 文件路径（与可执行文件同目录）。
func icoFilePath() string {
	return filepath.Join(filepath.Dir(selfExe()), "otk.ico")
}

// ensureIcoFile 把内嵌的 ICO 写到 otk.ico（已存在则跳过）。
// .lnk 的 IconLocation 必须指向磁盘上真实存在的文件，内嵌资源帮不上忙，
// 所以首次创建快捷方式时顺手导出一份。
func ensureIcoFile() (string, error) {
	path := icoFilePath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, guiIcoData, 0644); err != nil {
		return "", fmt.Errorf("无法写入图标文件：%v", err)
	}
	return path, nil
}

// psQuote 把路径转义成 PowerShell 单引号字符串的内容。
func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// runPowerShell 执行一段 PowerShell 脚本，隐藏窗口。
func runPowerShell(script string) error {
	cmd := powershellCommand(
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", script,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
