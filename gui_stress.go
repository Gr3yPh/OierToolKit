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
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// stressPage 是图形化的对拍：gen.cpp 造数据、brute.cpp 出标答。
type stressPage struct {
	pg      *projectPage
	content fyne.CanvasObject

	rounds    *widget.Entry
	keepChk   *widget.Check
	startBtn  *widget.Button
	stopBtn   *widget.Button
	bar       *widget.ProgressBar
	status    *widget.Label
	detail    *widget.Label
	dataBtn   *widget.Button
	saveBtn   *widget.Button
	manageBtn *widget.Button

	lastRes *stressResult

	// failBar 是跑完后铺开的反例轮次按钮条：多反例时不用开管理器就能
	// 逐个查看，「存为样例」跟着当前选中的轮次走。
	failBox    *fyne.Container
	failScroll *container.Scroll
	failBtns   map[string]*widget.Button
	curRound   string

	running atomic.Bool
	cancel  atomic.Bool
}

// reload 在切换题目后刷新提示文案。
func (s *stressPage) reload() {
	if s.running.Load() {
		return
	}
	p := s.pg.p
	s.status.SetText(fmt.Sprintf(tr("准备就绪，待测：%s.cpp。项目目录里需要有 gen.cpp 与 brute.cpp。"), p.codeName()))
	s.detail.SetText(tr("对拍结果会显示在这里。"))
}

func newStressPage(pg *projectPage) *stressPage {
	s := &stressPage{pg: pg}

	s.rounds = widget.NewEntry()
	s.rounds.SetText("100")
	s.rounds.Validator = func(text string) error {
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || n <= 0 {
			return fmt.Errorf("%s", tr("轮数必须是正整数"))
		}
		return nil
	}

	s.status = caption(tr("准备就绪。项目目录里需要有 gen.cpp 与 brute.cpp。"))
	s.detail = hint(tr("对拍结果会显示在这里。"))
	s.bar = widget.NewProgressBar()
	s.bar.Hide()

	s.startBtn = widget.NewButtonWithIcon(tr("开始对拍"), theme.MediaPlayIcon(), s.run)
	s.startBtn.Importance = widget.HighImportance
	s.stopBtn = widget.NewButtonWithIcon(tr("停止"), theme.MediaStopIcon(), s.stop)
	s.stopBtn.Disable()

	s.dataBtn = widget.NewButtonWithIcon(tr("打开 .stress 目录"), theme.FolderOpenIcon(), func() {
		if err := openPath(filepath.Join(s.pg.p.Dir, ".stress")); err != nil {
			s.pg.g.showError(err)
		}
	})
	s.dataBtn.Disable()

	s.saveBtn = widget.NewButtonWithIcon(tr("存为样例"), theme.ContentAddIcon(), s.saveCounterexample)
	s.saveBtn.Disable()

	s.manageBtn = widget.NewButtonWithIcon(tr("管理反例"), theme.StorageIcon(), s.manageDialog)

	s.keepChk = widget.NewCheck(tr("发现反例后继续收集"), nil)

	form := container.NewBorder(nil, nil,
		container.NewHBox(caption(tr("轮数")), container.NewGridWrap(fyne.NewSize(110, 36), s.rounds), s.keepChk),
		container.NewHBox(s.startBtn, s.stopBtn),
		nil,
	)

	head := container.NewVBox(
		container.NewPadded(form),
		container.NewPadded(s.bar),
		container.NewPadded(s.status),
		widget.NewSeparator(),
	)

	bottom := container.NewPadded(container.NewBorder(nil, nil, nil,
		container.NewHBox(s.saveBtn, s.manageBtn, s.dataBtn),
		caption(tr("反例数据保存为 <轮次>.in / .ans / .out，可一键存为样例后在评测页复测"))))

	s.failBtns = map[string]*widget.Button{}
	s.failBox = container.NewHBox()
	s.failScroll = container.NewHScroll(s.failBox)
	s.failScroll.Hide()

	// 结果区上方留给反例轮次条，隐藏时不占空间。
	middle := container.NewBorder(s.failScroll, nil, nil, nil, container.NewVScroll(s.detail))
	s.content = container.NewBorder(head, bottom, nil, nil, middle)
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
		s.pg.g.showError(fmt.Errorf("%s", tr("轮数必须是正整数")))
		return
	}

	s.running.Store(true)
	s.cancel.Store(false)
	s.detail.SetText("")
	s.dataBtn.Disable()
	s.saveBtn.Disable()
	// 清掉上一轮的反例按钮条，避免旧轮次混进新结果。
	s.curRound = ""
	s.failBox.Objects = nil
	s.failBtns = map[string]*widget.Button{}
	s.failScroll.Hide()
	s.bar.SetValue(0)
	s.bar.Show()
	s.startBtn.Disable()
	s.stopBtn.Enable()
	s.pg.g.busy(true, tr("对拍准备中 ..."))

	p := s.pg.p
	keepGoing := s.keepChk.Checked
	// 记下启动时的 generation：中途切走项目后，结束回调不能再去动
	// 新页面的忙碌状态和控件（与 judgePage.run 一致）。
	gen := s.pg.g.generation.Load()
	go func() {
		res, err := runStress(p, count, keepGoing, func(ev stressEvent) {
			ui(func() {
				if s.pg.g.generation.Load() != gen {
					return
				}
				switch ev.Stage {
				case stressCompile:
					s.status.SetText(fmt.Sprintf(tr("已编译 %s（%d/3）"), ev.Name, ev.Done+1))
				case stressStart:
					s.status.SetText(fmt.Sprintf(tr("对拍 %d 轮 ..."), ev.Total))
				case stressRound:
					s.bar.SetValue(float64(ev.Round) / float64(ev.Total))
					s.status.SetText(fmt.Sprintf(tr("第 %d/%d 轮"), ev.Round, ev.Total))
				}
			})
		}, func() bool { return s.cancel.Load() })

		ui(func() {
			if s.pg.g.generation.Load() != gen {
				return
			}
			s.running.Store(false)
			s.lastRes = &res
			s.startBtn.Enable()
			s.stopBtn.Disable()
			s.pg.g.busy(false, tr("对拍结束"))

			if err != nil {
				s.bar.Hide()
				if cf, ok := err.(compileFailure); ok {
					s.status.SetText("[CE] " + tr("编译失败"))
					s.detail.SetText(cf.Log)
					return
				}
				s.status.SetText(tr("对拍失败"))
				s.detail.SetText(err.Error())
				return
			}

			switch {
			case res.Stopped:
				s.status.SetText(fmt.Sprintf(tr("已中断（完成 %d 轮）"), res.Passed))
				s.detail.SetText(tr("对拍被手动停止。"))

			case res.FailRound > 0:
				extra := ""
				if res.Counterexamples > 1 {
					extra = fmt.Sprintf(tr("，共收集 %d 组反例"), res.Counterexamples)
				}
				s.status.SetText(fmt.Sprintf(tr("第 %d 轮发现反例（已保存到 .stress%s）"), res.FailRound, extra))
				s.detail.SetText(fmt.Sprintf(tr("输入数据：\n%s\n期望输出：\n%s\n实际输出：\n%s"),
					clip(res.Input), res.Expected, res.Actual))
				s.dataBtn.Enable()
				s.saveBtn.Enable()
				// 铺反例轮次条：多反例时直接在主页切换查看。
				s.fillFailRounds(res)

			default:
				s.bar.SetValue(1)
				s.status.SetText(fmt.Sprintf(tr("全部通过（%d/%d）"), res.Total, res.Total))
				summary := tr("没有发现反例。")
				if t := timingSummary(res); t != "" {
					summary += "\n\n" + t
				}
				s.detail.SetText(summary)
			}
		})
	}()
}

// saveCounterexample 把当前查看的反例轮次写入样例库，方便在评测页直接复测。
// 默认是首个反例；在轮次条上点过其他轮后，存的就是那一轮。
func (s *stressPage) saveCounterexample() {
	if s.lastRes == nil || s.lastRes.FailRound == 0 {
		return
	}
	round := s.curRound
	if round == "" {
		round = strconv.Itoa(s.lastRes.FailRound)
	}

	var input, expected string
	inB, errIn := os.ReadFile(filepath.Join(s.pg.p.Dir, ".stress", round+".in"))
	ansB, errAns := os.ReadFile(filepath.Join(s.pg.p.Dir, ".stress", round+".ans"))
	switch {
	case errIn == nil && errAns == nil:
		input, expected = string(inB), string(ansB)
	case round == strconv.Itoa(s.lastRes.FailRound):
		// 文件不在但内存里还有（比如首轮反例），退回内存数据。
		input, expected = s.lastRes.Input, s.lastRes.Expected
	default:
		s.pg.g.showError(fmt.Errorf("%s", tr("反例文件不存在，可能已被删除")))
		return
	}

	id := nextSampleID(readIndex(s.pg.p))
	if err := writeSample(s.pg.p, id, input, expected); err != nil {
		s.pg.g.showError(err)
		return
	}
	s.pg.g.setStatus(fmt.Sprintf(tr("已把第 %s 轮反例存为样例 #%s（含期望输出）"), round, id))
}

// fillFailRounds 跑完后把 .stress 里所有反例轮次铺成按钮条，
// 当前查看的轮次高亮，点击即在结果区切换。
func (s *stressPage) fillFailRounds(res stressResult) {
	rounds := stressRounds(res.StressDir)
	if len(rounds) == 0 {
		return
	}
	s.curRound = strconv.Itoa(res.FailRound)
	for _, r := range rounds {
		round := r
		btn := widget.NewButton(fmt.Sprintf(tr("第 %s 轮"), round), func() {
			s.curRound = round
			s.showRound(round)
			s.refreshFailButtons()
		})
		s.failBtns[round] = btn
		s.failBox.Add(btn)
	}
	s.refreshFailButtons()
	s.failScroll.Show()
}

// refreshFailButtons 按 curRound 刷新轮次条的高亮。
func (s *stressPage) refreshFailButtons() {
	for round, btn := range s.failBtns {
		if round == s.curRound {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
}

// showRound 把某一轮反例的输入/期望/实际输出显示到结果区。
func (s *stressPage) showRound(round string) {
	dir := filepath.Join(s.pg.p.Dir, ".stress")
	var b strings.Builder
	if data, err := os.ReadFile(filepath.Join(dir, round+".in")); err == nil {
		fmt.Fprintf(&b, tr("输入数据：\n%s\n"), clip(string(data)))
	}
	if data, err := os.ReadFile(filepath.Join(dir, round+".ans")); err == nil {
		fmt.Fprintf(&b, tr("期望输出：\n%s\n"), string(data))
	}
	if data, err := os.ReadFile(filepath.Join(dir, round+".out")); err == nil {
		fmt.Fprintf(&b, tr("实际输出：\n%s\n"), string(data))
	}
	s.detail.SetText(b.String())
}

// stressRounds 列出 .stress 目录里已有的反例轮次（升序）。
func stressRounds(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var rounds []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".in") {
			continue
		}
		round := strings.TrimSuffix(name, ".in")
		if round != "" && !seen[round] {
			seen[round] = true
			rounds = append(rounds, round)
		}
	}
	// 轮次名通常是纯数字，按数值排才是自然的先后顺序（"2" 要在 "10" 前）；
	// 出现非数字名字时退回字符串比较，保证排序稳定。
	sort.Slice(rounds, func(i, j int) bool {
		a, aerr := strconv.Atoi(rounds[i])
		b, berr := strconv.Atoi(rounds[j])
		if aerr == nil && berr == nil {
			return a < b
		}
		return rounds[i] < rounds[j]
	})
	return rounds
}

// manageDialog 打开反例管理器：左侧反例列表，右侧内容与操作。
func (s *stressPage) manageDialog() {
	dir := filepath.Join(s.pg.p.Dir, ".stress")
	var d dialog.Dialog

	detail := widget.NewLabel("")
	detail.TextStyle = fyne.TextStyle{Monospace: true}
	detail.Wrapping = fyne.TextWrapWord

	saveBtn := widget.NewButton(tr("存为样例"), nil)
	delBtn := widget.NewButton(tr("删除本轮"), nil)
	actions := container.NewHBox(saveBtn, delBtn)

	list := widget.NewList(
		func() int { return len(stressRounds(dir)) },
		func() fyne.CanvasObject { return widget.NewLabel("#0000") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			rounds := stressRounds(dir)
			if id < 0 || id >= len(rounds) {
				return
			}
			obj.(*widget.Label).SetText(fmt.Sprintf(tr("第 %s 轮"), rounds[id]))
		},
	)
	load := func(round string) {
		var b strings.Builder
		if data, err := os.ReadFile(filepath.Join(dir, round+".in")); err == nil {
			fmt.Fprintf(&b, tr("输入：\n%s\n"), clip(string(data)))
		}
		if data, err := os.ReadFile(filepath.Join(dir, round+".ans")); err == nil {
			fmt.Fprintf(&b, tr("期望输出：\n%s\n"), clip(string(data)))
		}
		if data, err := os.ReadFile(filepath.Join(dir, round+".out")); err == nil {
			fmt.Fprintf(&b, tr("实际输出：\n%s\n"), clip(string(data)))
		}
		detail.SetText(b.String())
	}

	current := ""
	list.OnSelected = func(id widget.ListItemID) {
		rounds := stressRounds(dir)
		if id < 0 || id >= len(rounds) {
			return
		}
		current = rounds[id]
		load(current)
		saveBtn.Enable()
		delBtn.Enable()
	}

	saveBtn.OnTapped = func() {
		if current == "" || !fileExists(filepath.Join(dir, current+".in")) {
			return
		}
		round := current
		inData, _ := os.ReadFile(filepath.Join(dir, round+".in"))
		ansData, _ := os.ReadFile(filepath.Join(dir, round+".ans"))
		id := nextSampleID(readIndex(s.pg.p))
		if err := writeSample(s.pg.p, id, string(inData), string(ansData)); err != nil {
			s.pg.g.showError(err)
			return
		}
		s.pg.g.setStatus(fmt.Sprintf(tr("已把第 %s 轮反例存为样例 #%s"), round, id))
	}
	delBtn.OnTapped = func() {
		if current == "" {
			return
		}
		round := current
		dialog.ShowConfirm(tr("删除反例"), fmt.Sprintf(tr("将删除第 %s 轮的 .in/.ans/.out 文件，确定吗？"), round), func(ok bool) {
			if !ok {
				return
			}
			for _, ext := range []string{".in", ".ans", ".out"} {
				_ = os.Remove(filepath.Join(dir, round+ext))
			}
			current = ""
			detail.SetText("")
			saveBtn.Disable()
			delBtn.Disable()
			list.Refresh()
			s.pg.g.setStatus(fmt.Sprintf(tr("已删除第 %s 轮反例"), round))
		}, s.pg.g.win)
	}

	if len(stressRounds(dir)) == 0 {
		body := container.NewVBox(caption(tr(".stress 目录里还没有反例。\n跑一次对拍，发现反例后会自动保存到这里。")))
		d = dialog.NewCustom(tr("管理反例"), tr("关闭"), container.NewPadded(body), s.pg.g.win)
	} else {
		body := container.NewHSplit(
			container.NewVScroll(list),
			container.NewVScroll(container.NewVBox(detail, widget.NewSeparator(), actions)),
		)
		body.SetOffset(0.22)
		d = dialog.NewCustom(tr("管理反例（.stress）"), tr("关闭"), body, s.pg.g.win)
	}
	d.Resize(fyne.NewSize(680, 460))
	d.Show()
}

// stop 请求中断对拍。
func (s *stressPage) stop() {
	s.cancel.Store(true)
	s.status.SetText(tr("正在停止 ..."))
}

// clip 限制超长输入在详情里的展示体量。
func clip(text string) string {
	const limit = 4000
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n" + tr("...（已截断）")
}
