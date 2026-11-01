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
	"sync/atomic"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// batchJudgeDialog 让用户勾选要批量评测的项目。
//
// 勾选框而不是在列表里多选：项目库那个列表是 widget.List，加多选要自己
// 管选中集合与点击语义，容易和「点一行打开项目」打架。这里列一遍项目名，
// 默认全部勾上，取消勾选成本比多选低得多。
func (g *guiApp) batchJudgeDialog() {
	if len(g.dirs) == 0 {
		g.showError(fmt.Errorf("%s", tr("项目库是空的，先新建或添加一个项目")))
		return
	}
	// 复制一份：对话框开着的时候项目库可能被刷新。
	dirs := append([]string(nil), g.dirs...)
	checks := make([]*widget.Check, len(dirs))
	box := container.NewVBox()
	for i, dir := range dirs {
		name := projectNameOf(dir)
		missing := !dirExists(dir)
		if missing {
			name += " " + tr("（目录已不存在）")
		}
		cb := widget.NewCheck(name, nil)
		cb.SetChecked(!missing)
		if missing {
			cb.Disable()
		}
		checks[i] = cb
		box.Add(cb)
	}

	allBtn := widget.NewButton(tr("全选"), func() {
		for i, cb := range checks {
			if dirExists(dirs[i]) {
				cb.SetChecked(true)
			}
		}
	})
	noneBtn := widget.NewButton(tr("全不选"), func() {
		for _, cb := range checks {
			cb.SetChecked(false)
		}
	})

	content := container.NewVBox(
		caption(tr("串行评测：每个项目用自己的时限、内存与线程数，结果写进各自的评测历史。")),
		container.NewHBox(allBtn, noneBtn),
		widget.NewSeparator(),
		box,
	)

	g.showWideDialog(tr("批量评测"), tr("开始评测"), tr("取消"), content, func() {
		var sel []project
		for i, cb := range checks {
			if cb.Checked && dirExists(dirs[i]) {
				sel = append(sel, project{Name: projectNameOf(dirs[i]), Dir: dirs[i]})
			}
		}
		if len(sel) == 0 {
			g.showError(fmt.Errorf("%s", tr("至少勾选一个项目")))
			return
		}
		// 这里不能直接开浮层：showSheet 的 OnConfirm 返回后会把当前浮层关掉，
		// 新开的进度面板会被一起收走。丢给主循环下一轮执行即可。
		go func() {
			ui(func() { g.runBatchJudge(sel) })
		}()
	})
}

// runBatchJudge 串行评测选中的项目，并显示进度面板。
func (g *guiApp) runBatchJudge(list []project) {
	total := len(list)
	var cancelled atomic.Bool

	bar := widget.NewProgressBar()
	titleLbl := widget.NewLabel(fmt.Sprintf(tr("共 %d 个项目，准备开始 ..."), total))
	rows := make([]*widget.Label, total)
	box := container.NewVBox()
	for i, p := range list {
		rows[i] = widget.NewLabel("· " + p.Name)
		box.Add(rows[i])
	}

	stopBtn := widget.NewButtonWithIcon(tr("停止"), theme.MediaStopIcon(), nil)
	stopBtn.OnTapped = func() {
		cancelled.Store(true)
		stopBtn.Disable()
		stopBtn.SetText(tr("正在停止 ..."))
	}

	content := container.NewBorder(
		container.NewVBox(
			container.NewPadded(titleLbl),
			container.NewPadded(bar),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewHBox(stopBtn, caption(tr("停止后不再开始新项目，正在跑的那个会提前收工。"))),
		),
		nil, nil,
		box,
	)

	g.showSheet(sheetOptions{
		Title:       tr("批量评测"),
		Confirm:     tr("关闭"),
		Dismiss:     tr("关闭"),
		Content:     content,
		HeightRatio: 0.7,
		HideDismiss: true,
		OnConfirm:   func() { cancelled.Store(true) },
		OnDismiss:   func() { cancelled.Store(true) },
	})

	go func() {
		passed, failed, stopped := 0, 0, 0
		done := 0
		for i, p := range list {
			if cancelled.Load() {
				stopped += total - i
				break
			}
			ui(func() {
				titleLbl.SetText(fmt.Sprintf(tr("正在评测 %d/%d：%s"), i+1, total, p.Name))
				bar.SetValue(float64(i) / float64(total))
			})

			res := judgeProjectHeadless(p, func() bool { return cancelled.Load() })
			switch {
			case res.OK():
				passed++
			case cancelled.Load():
				stopped++
			default:
				failed++
			}
			done++

			text := "✓ " + res.Name + "  " + res.score()
			if !res.OK() {
				text = "✗ " + res.Name + "  " + res.score()
			}
			idx := i
			ui(func() {
				rows[idx].SetText(text)
				bar.SetValue(float64(done) / float64(total))
			})
		}

		summary := fmt.Sprintf(tr("批量评测完成：%d 个跑完，%d 个没跑起来"), passed, failed)
		if stopped > 0 {
			summary += fmt.Sprintf(tr("，%d 个未开始或中断"), stopped)
		}
		ui(func() {
			bar.SetValue(1)
			titleLbl.SetText(summary)
			stopBtn.Disable()
			g.setStatus(summary)
		})
	}()
}
