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
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// stressPage 是图形化的对拍：gen.cpp 造数据、brute.cpp 出标答。
type stressPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	rounds   *widget.Entry
	startBtn *widget.Button
	stopBtn  *widget.Button
	bar      *widget.ProgressBar
	status   *widget.Label
	detail   *widget.Label
	dataBtn  *widget.Button

	running atomic.Bool
	cancel  atomic.Bool
}

// reload 在切换题目后刷新提示文案。
func (s *stressPage) reload() {
	if s.running.Load() {
		return
	}
	p := s.pg.p
	s.status.SetText("准备就绪，待测：" + p.codeName() + ".cpp。项目目录里需要有 gen.cpp 与 brute.cpp。")
	s.detail.SetText("对拍结果会显示在这里。")
}

func newStressPage(pg *projectPage) *stressPage {
	s := &stressPage{pg: pg}

	s.rounds = widget.NewEntry()
	s.rounds.SetText("100")
	s.rounds.Validator = func(text string) error {
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || n <= 0 {
			return fmt.Errorf("轮数必须是正整数")
		}
		return nil
	}

	s.status = caption("准备就绪。项目目录里需要有 gen.cpp 与 brute.cpp。")
	s.detail = hint("对拍结果会显示在这里。")
	s.bar = widget.NewProgressBar()
	s.bar.Hide()

	s.startBtn = widget.NewButtonWithIcon("开始对拍", theme.MediaPlayIcon(), s.run)
	s.startBtn.Importance = widget.HighImportance
	s.stopBtn = widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), s.stop)
	s.stopBtn.Disable()

	s.dataBtn = widget.NewButtonWithIcon("打开 .stress 目录", theme.FolderOpenIcon(), func() {
		if err := openPath(filepath.Join(s.pg.p.Dir, ".stress")); err != nil {
			s.pg.g.showError(err)
		}
	})
	s.dataBtn.Disable()

	form := container.NewBorder(nil, nil,
		container.NewHBox(caption("轮数"), container.NewGridWrap(fyne.NewSize(110, 36), s.rounds)),
		container.NewHBox(s.startBtn, s.stopBtn),
		nil,
	)

	head := container.NewVBox(
		container.NewPadded(form),
		container.NewPadded(s.bar),
		container.NewPadded(s.status),
		widget.NewSeparator(),
	)

	bottom := container.NewPadded(container.NewBorder(nil, nil, nil, s.dataBtn, caption("反例数据保存为 <轮次>.in / .ans / .out")))
	s.content = container.NewBorder(head, bottom, nil, nil, container.NewVScroll(s.detail))
	s.reload()
	return s
}

// run 在后台跑对拍，按轮次更新进度。
func (s *stressPage) run() {
	if s.running.Load() {
		return
	}
	count, err := strconv.Atoi(strings.TrimSpace(s.rounds.Text))
	if err != nil || count <= 0 {
		s.pg.g.showError(fmt.Errorf("轮数必须是正整数"))
		return
	}

	s.running.Store(true)
	s.cancel.Store(false)
	s.detail.SetText("")
	s.dataBtn.Disable()
	s.bar.SetValue(0)
	s.bar.Show()
	s.startBtn.Disable()
	s.stopBtn.Enable()
	s.pg.g.busy(true, "对拍准备中 ...")

	p := s.pg.p
	go func() {
		res, err := runStress(p, count, func(ev stressEvent) {
			ui(func() {
				switch ev.Stage {
				case stressCompile:
					s.status.SetText(fmt.Sprintf("已编译 %s（%d/3）", ev.Name, ev.Done+1))
				case stressStart:
					s.status.SetText(fmt.Sprintf("对拍 %d 轮 ...", ev.Total))
				case stressRound:
					s.bar.SetValue(float64(ev.Round) / float64(ev.Total))
					s.status.SetText(fmt.Sprintf("第 %d/%d 轮", ev.Round, ev.Total))
				}
			})
		}, func() bool { return s.cancel.Load() })

		ui(func() {
			s.running.Store(false)
			s.startBtn.Enable()
			s.stopBtn.Disable()
			s.pg.g.busy(false, "对拍结束")

			if err != nil {
				s.bar.Hide()
				if cf, ok := err.(compileFailure); ok {
					s.status.SetText("[CE] 编译失败")
					s.detail.SetText(cf.Log)
					return
				}
				s.status.SetText("对拍失败")
				s.detail.SetText(err.Error())
				return
			}

			switch {
			case res.Stopped:
				s.status.SetText(fmt.Sprintf("已中断（完成 %d 轮）", res.Passed))
				s.detail.SetText("对拍被手动停止。")

			case res.FailRound > 0:
				s.status.SetText(fmt.Sprintf("第 %d 轮发现反例（已保存到 .stress）", res.FailRound))
				s.detail.SetText(fmt.Sprintf("输入数据：\n%s\n期望输出：\n%s\n实际输出：\n%s",
					clip(res.Input), res.Expected, res.Actual))
				s.dataBtn.Enable()

			default:
				s.bar.SetValue(1)
				s.status.SetText(fmt.Sprintf("全部通过（%d/%d）", res.Total, res.Total))
				s.detail.SetText("没有发现反例。")
			}
		})
	}()
}

// stop 请求中断对拍。
func (s *stressPage) stop() {
	s.cancel.Store(true)
	s.status.SetText("正在停止 ...")
}

// clip 限制超长输入在详情里的展示体量。
func clip(text string) string {
	const limit = 4000
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n...（已截断）"
}
