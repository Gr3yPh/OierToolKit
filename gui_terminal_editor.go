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
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
)

// editorActionLabel 根据设置决定“编辑器”按钮该指向外部程序还是内置终端。
func editorActionLabel() string {
	if guiPrefBool(prefEditorTerminal, false) {
		return tr("终端编辑器")
	}
	return tr("外部编辑器")
}

func (pg *projectPage) ensureEditorTab() *container.TabItem {
	if pg.editorItem != nil {
		return pg.editorItem
	}
	pg.editor = newTerminalEditorPage(pg)
	pg.editorItem = container.NewTabItemWithIcon(tr("终端编辑器"), theme.ComputerIcon(), pg.editor.content)
	pg.tabs.Append(pg.editorItem)
	pg.tabs.Refresh()
	return pg.editorItem
}

// closeEditorTab 关闭内置终端并回到源码页。
func (pg *projectPage) closeEditorTab() {
	e, item := pg.editor, pg.editorItem
	if e == nil {
		return
	}
	pg.editor = nil
	pg.editorItem = nil
	if item != nil {
		pg.tabs.Remove(item)
	}
	e.close()
	if pg.sourceItem != nil {
		pg.tabs.Select(pg.sourceItem)
		pg.source.reloadIfClean()
	}
}

// openEditor 按设置打开外部编辑器，或在项目内启动一个终端编辑器标签。
func (pg *projectPage) openEditor() error {
	if guiPrefBool(prefEditorTerminal, false) {
		item := pg.ensureEditorTab()
		pg.tabs.Select(item)
		pg.editor.start()
		return nil
	}
	return launchEditor(pg.p)
}

// terminalEditorPage 是项目内的懒加载终端。启用内置终端后，点“编辑器”
// 才会创建它；没打开过的项目不会白占一个 shell/PTY。
type terminalEditorPage struct {
	pg      *projectPage
	term    *terminal.Terminal
	content fyne.CanvasObject
	started bool
}

func newTerminalEditorPage(pg *projectPage) *terminalEditorPage {
	t := terminal.New()
	t.SetStartDir(pg.p.Dir)
	closeBtn := widget.NewButtonWithIcon(tr("关闭终端"), theme.CancelIcon(), pg.closeEditorTab)
	head := container.NewHBox(layout.NewSpacer(), closeBtn)
	return &terminalEditorPage{pg: pg, term: t, content: container.NewBorder(head, nil, nil, nil, t)}
}

func (e *terminalEditorPage) start() {
	if e.started {
		return
	}
	e.started = true

	ready := make(chan terminal.Config, 1)
	e.term.AddListener(ready)

	go func() {
		err := e.term.RunLocalShell()
		ui(func() {
			e.started = false
			if e.pg.closed.Load() || e.pg.editor != e {
				return
			}
			if err != nil {
				e.pg.g.setStatus(fmt.Sprintf(tr("终端编辑器退出：%v"), err))
			}
			// 编辑器进程结束（例如 :q 退出 nvim）后自动收起终端标签，
			// 回到源码页；否则用户会看到一个“退不出来”的空终端。
			e.pg.closeEditorTab()
		})
	}()

	go func() {
		select {
		case <-ready:
		case <-time.After(8 * time.Second):
		}
		editor := propsOr(readIni(globalConfigPath()), "editor", defaultEditorCommand())
		command := fmt.Sprintf(editor, quoteArg(e.pg.p.sourcePath()))
		// 终端编辑器退出后自动收起标签，避免留下一个看起来还在运行的
		// shell。Unix 用 exec 把编辑器替换成 shell；PowerShell 则在命令
		// 后面追加 exit。
		parts := splitQuoted(command)
		terminalEditor := len(parts) > 0 && terminalEditors[editorName(parts[0])]
		switch {
		case terminalEditor && runtime.GOOS == "windows":
			command += "; exit"
		case terminalEditor:
			command = "exec " + command
		}
		if _, err := e.term.Write([]byte(command + "\n")); err != nil {
			ui(func() {
				if !e.pg.closed.Load() {
					e.pg.g.showError(fmt.Errorf(tr("启动终端编辑器失败: %v"), err))
				}
			})
			return
		}
		ui(func() {
			if !e.pg.closed.Load() && e.pg.g.win != nil {
				e.pg.g.win.Canvas().Focus(e.term)
			}
		})
	}()
}

func (e *terminalEditorPage) close() {
	if e.term != nil {
		e.term.Close()
	}
}
