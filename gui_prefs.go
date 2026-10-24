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
	"strings"

	"fyne.io/fyne/v2/dialog"
)

// GUI 偏好项与命令行配置共用 ~/.otkprofile/config，只是键名加 gui_ 前缀，
// 免得和项目/远端配置混在一起。

const (
	prefFullscreen    = "gui_fullscreen"     // 启动时全屏
	prefShortcutAsked = "gui_shortcut_asked" // 是否已经问过"要不要创建桌面快捷方式"
)

// guiPref 读取一个 GUI 偏好，缺失时返回 def。
func guiPref(key, def string) string {
	return propsOr(readIni(globalConfigPath()), key, def)
}

// guiPrefBool 读取布尔偏好。"1"/"true"/"on"/"yes" 视为真。
func guiPrefBool(key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(guiPref(key, "")))
	if raw == "" {
		return def
	}
	switch raw {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	}
	return def
}

// setGuiPref 写入一个 GUI 偏好。
func setGuiPref(key, value string) error {
	props := readIni(globalConfigPath())
	props[key] = value
	writeIni(globalConfigPath(), props)
	return nil
}

// setGuiPrefBool 写入布尔偏好。
func setGuiPrefBool(key string, value bool) error {
	v := "0"
	if value {
		v = "1"
	}
	return setGuiPref(key, v)
}

// maybeAskShortcut 在首次启动 GUI 时询问是否创建桌面快捷方式。
// 只问一次：无论用户选是还是否，都会记下"已询问"，之后可在设置里手动创建。
func (g *guiApp) maybeAskShortcut() {
	if guiPrefBool(prefShortcutAsked, false) {
		return
	}
	// 已经从桌面快捷方式启动了（本身就以 otk-gui 运行），没必要再问。
	if isGuiBinary() && shortcutExists() {
		_ = setGuiPrefBool(prefShortcutAsked, true)
		return
	}

	msg := "要不要在桌面创建一个 OierToolKit 快捷方式？\n\n" +
		"创建后双击图标即可直接打开图形界面，不必再敲命令。\n" +
		"以后也可以在“设置”里随时创建或删除。"
	d := dialog.NewConfirm("创建桌面快捷方式", msg, func(ok bool) {
		// 先记下已询问，避免用户取消后每次启动都被追问。
		_ = setGuiPrefBool(prefShortcutAsked, true)
		if !ok {
			return
		}
		g.doCreateShortcut()
	}, g.win)
	d.SetConfirmText("创建")
	d.SetDismissText("暂不创建")
	d.Show()
}

// doCreateShortcut 创建桌面快捷方式并反馈结果。
func (g *guiApp) doCreateShortcut() {
	path, err := createDesktopShortcut()
	if err != nil {
		g.setStatus("创建快捷方式失败")
		g.showError(err)
		return
	}
	g.setStatus("桌面快捷方式已创建")
	dialog.ShowInformation("创建成功",
		"已在桌面创建快捷方式：\n"+path+"\n\n双击即可打开图形界面。", g.win)
}

// removeShortcut 删除桌面快捷方式。
func (g *guiApp) removeShortcut() {
	desktop, err := desktopDir()
	if err != nil {
		g.showError(err)
		return
	}
	path := shortcutPath(desktop)
	if _, err := os.Stat(path); err != nil {
		g.showError(fmt.Errorf("桌面上没有找到快捷方式"))
		return
	}
	if err := os.Remove(path); err != nil {
		g.showError(err)
		return
	}
	g.setStatus("桌面快捷方式已删除")
}

// fullscreenLabel 生成全屏偏好的文案。
func fullscreenLabel(on bool) string {
	if on {
		return "全屏启动"
	}
	return "窗口化启动（1180×760 居中）"
}
