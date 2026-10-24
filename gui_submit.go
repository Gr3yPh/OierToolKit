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
	"strconv"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// submitPage 把源码提交到远端 OJ 并轮询判题结果。
type submitPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	info    *widget.Label
	status  *widget.Label
	badge   *verdictBadge
	verdict *widget.Label
	meta    *widget.Label
	list    *widget.List
	detail  *widget.Label

	points []remoteTestPoint

	waitBtn  *widget.Button
	quietBtn *widget.Button
	stopBtn  *widget.Button

	running atomic.Bool
	cancel  atomic.Bool
}

func newSubmitPage(pg *projectPage) *submitPage {
	s := &submitPage{pg: pg}

	s.info = hint("")
	s.status = caption("提交前请确认远端地址与题号。")
	s.badge = newVerdictBadge()
	s.badge.set("")
	s.verdict = strong("尚未提交")
	s.meta = caption("")
	s.detail = hint("选择测试点查看详情。")

	s.waitBtn = widget.NewButtonWithIcon("提交并等待结果", theme.UploadIcon(), func() { s.run(true) })
	s.waitBtn.Importance = widget.HighImportance
	s.quietBtn = widget.NewButtonWithIcon("仅提交", theme.MailSendIcon(), func() { s.run(false) })
	s.stopBtn = widget.NewButtonWithIcon("停止等待", theme.MediaStopIcon(), s.stop)
	s.stopBtn.Disable()

	s.list = widget.NewList(
		func() int { return len(s.points) },
		func() fyne.CanvasObject { return newPointRow() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < 0 || id >= len(s.points) {
				return
			}
			t := s.points[id]
			pt := pointResult{
				ID:      strconv.Itoa(t.Index),
				Verdict: strings.ToUpper(strings.TrimSpace(t.Verdict)),
				WallMS:  t.TimeMS,
				MemMB:   float64(t.MemoryKB) / 1024,
				MemOK:   t.MemoryKB > 0,
			}
			o.(*pointRow).update(pt, 0)
		},
	)
	s.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(s.points) {
			return
		}
		t := s.points[id]
		if strings.TrimSpace(t.Detail) == "" {
			s.detail.SetText(fmt.Sprintf("#%d %s\n\n用时 %dms，内存 %dKB。", t.Index,
				strings.ToUpper(strings.TrimSpace(t.Verdict)), t.TimeMS, t.MemoryKB))
			return
		}
		s.detail.SetText(fmt.Sprintf("#%d %s\n\n%s", t.Index,
			strings.ToUpper(strings.TrimSpace(t.Verdict)), t.Detail))
	}

	head := container.NewVBox(
		container.NewPadded(container.NewVBox(
			s.info,
			container.NewHBox(s.waitBtn, s.quietBtn, s.stopBtn, layout.NewSpacer(), s.status),
		)),
		widget.NewSeparator(),
	)

	summary := container.NewPadded(container.NewHBox(s.badge, s.verdict, s.meta))
	body := container.NewHSplit(s.list, container.NewVScroll(s.detail))
	body.SetOffset(0.34)

	s.content = container.NewBorder(head, nil, nil, nil,
		container.NewBorder(summary, nil, nil, nil, body))
	s.reload()
	return s
}

// reload 刷新远端信息展示（切换项目或保存配置后调用）。
func (s *submitPage) reload() {
	props := s.pg.p.cfg()
	base := strings.TrimRight(props["remote_base"], "/")
	id := props["remote_id"]
	switch {
	case base == "" && id == "":
		s.info.SetText("该项目未配置远端。可以点“配置远端”，或用“从网站添加题目”建立带远端信息的项目。")
	case base == "":
		s.info.SetText("已配置题号 " + id + "，但缺少远端地址。")
	default:
		s.info.SetText(fmt.Sprintf("题目 %s @ %s", id, base))
	}
	if s.pg.p.cfg()["remote_id"] != "" {
		s.info.SetText(s.info.Text + "\n远端题号：" + s.pg.p.cfg()["remote_id"])
	}
}

// run 提交源码。wait 为 true 时轮询到终态。
func (s *submitPage) run(wait bool) {
	if s.running.Load() {
		return
	}
	p := s.pg.p
	props := p.cfg()
	if strings.TrimRight(props["remote_base"], "/") == "" {
		s.pg.g.showError(fmt.Errorf("请先配置远端地址与题号"))
		return
	}

	s.running.Store(true)
	s.cancel.Store(false)
	s.points = nil
	s.list.Refresh()
	s.detail.SetText("")
	s.badge.set("")
	s.verdict.SetText("提交中 ...")
	s.meta.SetText("")
	s.waitBtn.Disable()
	s.quietBtn.Disable()
	s.stopBtn.Enable()
	s.pg.g.busy(true, "正在提交 ...")

	go func() {
		res, err := submitSolution(p, wait, func(v remoteVerdict) {
			verdict := strings.ToUpper(strings.TrimSpace(v.Verdict))
			if verdict == "" {
				verdict = "PENDING"
			}
			ui(func() {
				s.status.SetText("判题中：" + verdict)
			})
		}, func() bool { return s.cancel.Load() })

		ui(func() {
			s.running.Store(false)
			s.waitBtn.Enable()
			s.quietBtn.Enable()
			s.stopBtn.Disable()

			switch {
			case err != nil:
				s.verdict.SetText("提交失败")
				s.status.SetText(err.Error())
				s.pg.g.busy(false, "提交失败")
				s.pg.g.showError(err)

			case !wait:
				s.verdict.SetText("已提交")
				s.meta.SetText("提交号 " + res.ID)
				s.status.SetText("已提交，未等待结果")
				s.pg.g.busy(false, "已提交 "+res.ID)

			case res.Timeout:
				s.verdict.SetText("等待超时")
				s.meta.SetText("提交号 " + res.ID + " · 最后状态 " + res.Last)
				s.status.SetText("可到 OJ 页面查看结果")
				s.pg.g.busy(false, "等待超时")
				s.pg.g.showError(fmt.Errorf("仍然没有出现终态（最后状态 %s），提交号 %s", res.Last, res.ID))

			case s.cancel.Load():
				s.verdict.SetText("已停止等待")
				s.meta.SetText("提交号 " + res.ID)
				s.pg.g.busy(false, "已停止等待")

			default:
				verdict := strings.ToUpper(strings.TrimSpace(res.Verdict.Verdict))
				s.badge.set(verdict)
				s.verdict.SetText(verdict)
				s.meta.SetText(scoreText(res.Verdict))
				s.points = res.Verdict.Points
				s.list.Refresh()
				if d := strings.TrimSpace(res.Verdict.Detail); d != "" {
					s.detail.SetText(d)
				}
				s.status.SetText("提交号 " + res.ID)
				s.pg.g.busy(false, "远端判定："+verdict)
			}
		})
	}()
}

// stop 停止等待判题（提交已经发出，无法撤回）。
func (s *submitPage) stop() {
	s.cancel.Store(true)
	s.status.SetText("正在停止等待 ...")
}

// scoreText 组合得分与时空信息。
func scoreText(v remoteVerdict) string {
	var parts []string
	if v.MaxScore > 0 || v.Score > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d 分", v.Score, v.MaxScore))
	}
	if v.TimeMS > 0 || v.MemoryKB > 0 {
		parts = append(parts, fmt.Sprintf("%dms · %dKB", v.TimeMS, v.MemoryKB))
	}
	return strings.Join(parts, " · ")
}
