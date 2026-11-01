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
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// GUI 偏好项与命令行配置共用 ~/.otkprofile/config，只是键名加 gui_ 前缀，
// 免得和项目/远端配置混在一起。

const (
	prefFullscreen    = "gui_fullscreen"     // 启动时全屏
	prefShortcutAsked = "gui_shortcut_asked" // 是否已经问过"要不要创建桌面快捷方式"
	prefJudgeParallel = "gui_judge_parallel" // 评测并发度：0=自动（CPU核数-1，上限8）
	prefTheme         = "gui_theme"          // 主题：auto=跟随系统 / light / dark

	// 窗口状态记忆：关闭时的最大化状态与窗口化时的尺寸/位置。
	prefWinMax = "gui_win_max"
	prefWinW   = "gui_win_w"
	prefWinH   = "gui_win_h"
	prefWinX   = "gui_win_x"
	prefWinY   = "gui_win_y"
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

// guiPrefInt 读取整数偏好，缺失或非法时返回 def。
func guiPrefInt(key string, def int) int {
	raw := strings.TrimSpace(guiPref(key, ""))
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	return def
}

// setGuiPrefInt 写入整数偏好。
func setGuiPrefInt(key string, value int) error {
	return setGuiPref(key, strconv.Itoa(value))
}

// saveWindowState 在窗口关闭前记住当前状态：
// 是否最大化；窗口化时的客户区尺寸与屏幕位置（位置仅 Windows 支持）。
func (g *guiApp) saveWindowState() {
	if g == nil || g.win == nil {
		return
	}
	maxed := nativeIsMaximized(g.win)
	_ = setGuiPrefBool(prefWinMax, maxed)
	if maxed {
		return
	}
	if c := g.win.Canvas(); c != nil {
		if sz := c.Size(); sz.Width > 100 && sz.Height > 100 {
			_ = setGuiPrefInt(prefWinW, int(sz.Width))
			_ = setGuiPrefInt(prefWinH, int(sz.Height))
		}
	}
	if x, y, ok := nativeWindowPos(g.win); ok {
		_ = setGuiPrefInt(prefWinX, x)
		_ = setGuiPrefInt(prefWinY, y)
	}
}

// restoreWindowState 在"窗口化启动"（关闭了最大化偏好）时恢复上次的
// 尺寸与位置。没有历史记录时维持 1180×760 居中的默认值。
func (g *guiApp) restoreWindowState() {
	if g == nil || g.win == nil {
		return
	}
	if w, h := guiPrefInt(prefWinW, 0), guiPrefInt(prefWinH, 0); w > 200 && h > 150 {
		g.win.Resize(fyne.NewSize(float32(w), float32(h)))
	}
	if x, y := guiPrefInt(prefWinX, 0), guiPrefInt(prefWinY, 0); x != 0 || y != 0 {
		nativeMoveWindow(g.win, x, y)
	}
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

	msg := tr("要不要在桌面创建一个 OierToolKit 快捷方式？") + "\n\n" +
		tr("创建后双击图标即可直接打开图形界面，不必再敲命令。") + "\n" +
		tr("以后也可以在“设置”里随时创建或删除。")
	d := dialog.NewConfirm(tr("创建桌面快捷方式"), msg, func(ok bool) {
		// 先记下已询问，避免用户取消后每次启动都被追问。
		_ = setGuiPrefBool(prefShortcutAsked, true)
		if !ok {
			return
		}
		g.doCreateShortcut()
	}, g.win)
	d.SetConfirmText(tr("创建"))
	d.SetDismissText(tr("暂不创建"))
	d.Show()
}

// doCreateShortcut 创建桌面快捷方式并反馈结果。
func (g *guiApp) doCreateShortcut() {
	path, err := createDesktopShortcut()
	if err != nil {
		g.setStatus(tr("创建快捷方式失败"))
		g.showError(err)
		return
	}
	g.setStatus(tr("桌面快捷方式已创建"))
	dialog.ShowInformation(tr("创建成功"),
		tr("已在桌面创建快捷方式：")+"\n"+path+"\n\n"+tr("双击即可打开图形界面。"), g.win)
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
		g.showError(fmt.Errorf("%s", tr("桌面上没有找到快捷方式")))
		return
	}
	if err := os.Remove(path); err != nil {
		g.showError(err)
		return
	}
	g.setStatus(tr("桌面快捷方式已删除"))
}

// fullscreenLabel 生成全屏偏好的文案。
func fullscreenLabel(on bool) string {
	if on {
		return tr("全屏启动")
	}
	return tr("窗口化启动（恢复上次窗口大小与位置）")
}
