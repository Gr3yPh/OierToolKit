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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 选手与提交页：查看谁交了哪道题、补交漏掉的源码、删掉交错的提交、
// 以及整队改名（改名会同步移动 source/ 下的目录）。
type contestRoster struct {
	page    *contestPage
	content fyne.CanvasObject

	list    *widget.List
	entries []rosterEntry
	summary *widget.Label
	preview *widget.Label

	selected int
}

// rosterEntry 是一行「选手 / 题目 / 源码路径」。
type rosterEntry struct {
	Player  string
	Problem string
	Path    string
	Missing bool
}

func newContestRoster(p *contestPage) *contestRoster {
	r := &contestRoster{page: p, selected: -1}
	r.summary = caption("")
	r.preview = hint("选择一行查看源码路径与文件是否存在。")

	r.list = widget.NewList(
		func() int { return len(r.entries) },
		func() fyne.CanvasObject { return newRosterRow() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < 0 || id >= len(r.entries) {
				return
			}
			e := r.entries[id]
			o.(*rosterRow).update(e.Player, e.Problem, e.Missing, id == r.selected)
		},
	)
	r.list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(r.entries) {
			return
		}
		r.selected = id
		e := r.entries[id]
		state := "文件存在"
		if e.Missing {
			state = "源码缺失"
		}
		full := filepath.Join(p.c.Dir, filepath.FromSlash(e.Path))
		r.preview.SetText(fmt.Sprintf("%s / %s\n%s\n%s", e.Player, e.Problem, full, state))
	}

	addBtn := widget.NewButtonWithIcon("补交源码", theme.ContentAddIcon(), r.addDialog)
	renameBtn := widget.NewButtonWithIcon("选手改名", theme.AccountIcon(), r.renameDialog)
	removeBtn := deleteButton("删除提交", r.removeSelected)
	openDirBtn := widget.NewButtonWithIcon("打开 source 目录", theme.FolderOpenIcon(), func() {
		if err := openPath(p.c.sourceDir()); err != nil {
			p.g.showError(err)
		}
	})

	head := container.NewVBox(
		container.NewPadded(container.NewVBox(
			caption("比赛里的选手与提交决定了评测范围。src 目录结构为 source/<选手>/<题目>.cpp。"),
			container.NewHBox(addBtn, renameBtn, removeBtn, openDirBtn),
			r.summary,
			widget.NewSeparator(),
		)),
	)

	body := container.NewHSplit(r.list, container.NewVScroll(container.NewPadded(r.preview)))
	body.SetOffset(0.5)

	r.content = container.NewBorder(head, nil, nil, nil, body)
	r.reload()
	return r
}

// reload 重新从配置生成列表。
func (r *contestRoster) reload() {
	c := r.page.c
	entries := make([]rosterEntry, 0, len(c.Cfg.Sources))
	players := map[string]int{}
	for _, s := range c.Cfg.Sources {
		full := filepath.Join(c.Dir, filepath.FromSlash(s.Path))
		entries = append(entries, rosterEntry{
			Player:  s.Player,
			Problem: s.Problem,
			Path:    s.Path,
			Missing: !fileExists(full),
		})
		players[s.Player]++
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Player != entries[j].Player {
			return entries[i].Player < entries[j].Player
		}
		return entries[i].Problem < entries[j].Problem
	})
	r.entries = entries

	missing := 0
	for _, e := range entries {
		if e.Missing {
			missing++
		}
	}
	text := fmt.Sprintf("%d 位选手 · %d 份提交 · %d 道题", len(players), len(entries), len(c.Cfg.Problems))
	if missing > 0 {
		text += fmt.Sprintf(" · %d 份源码缺失", missing)
	}
	r.summary.SetText(text)

	if r.list != nil {
		r.list.UnselectAll()
		r.list.Refresh()
	}
	r.selected = -1
}

// addDialog 可以手动补一份提交：指定选手、题目与源码文件。
func (r *contestRoster) addDialog() {
	c := r.page.c
	var probNames []string
	for _, p := range c.Cfg.Problems {
		probNames = append(probNames, p.Name)
	}
	if len(probNames) == 0 {
		r.page.g.showError(fmt.Errorf("这场比赛没有题目，先在“题目配置”里重新扫描测试点"))
		return
	}

	playerIn := widget.NewSelectEntry(nil)
	var known []string
	for _, p := range c.Cfg.Players {
		known = append(known, p.Name)
	}
	playerIn.SetOptions(known)
	playerIn.SetPlaceHolder("选手名")

	probSel := widget.NewSelect(probNames, nil)
	probSel.SetSelected(probNames[0])

	pathIn := widget.NewEntry()
	pathIn.SetPlaceHolder("选择 .cpp 源码文件")
	browse := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
		picker := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			pathIn.SetText(uc.URI().Path())
			_ = uc.Close()
		}, r.page.g.win)
		picker.Show()
	})
	pathRow := container.NewBorder(nil, nil, nil, browse, pathIn)

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("选手", playerIn),
			widget.NewFormItem("题目", probSel),
			widget.NewFormItem("源码", pathRow),
		),
		caption("源码会被复制到 source/<选手>/<题目>.cpp，原有同名文件会被覆盖。"),
	)

	dialog.ShowCustomConfirm("补交源码", "添加", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		player := strings.TrimSpace(playerIn.Text)
		prob := probSel.Selected
		src := strings.TrimSpace(pathIn.Text)
		if player == "" || prob == "" || src == "" {
			r.page.g.showError(fmt.Errorf("选手、题目与源码都不能为空"))
			return
		}
		if err := r.page.addSubmission(player, prob, src); err != nil {
			r.page.g.showError(err)
			return
		}
		r.reload()
		r.page.g.setStatus(fmt.Sprintf("已添加 %s / %s", player, prob))
	}, r.page.g.win)
}

// renameDialog 修改选手名，并同步移动 source/ 下的目录。
func (r *contestRoster) renameDialog() {
	var names []string
	seen := map[string]bool{}
	for _, p := range r.page.c.Cfg.Players {
		if !seen[p.Name] {
			seen[p.Name] = true
			names = append(names, p.Name)
		}
	}
	if len(names) == 0 {
		r.page.g.showError(fmt.Errorf("还没有任何选手"))
		return
	}
	sort.Strings(names)

	from := widget.NewSelect(names, nil)
	from.SetSelected(names[0])
	to := widget.NewEntry()
	to.SetPlaceHolder("新的选手名")

	content := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("原选手", from),
			widget.NewFormItem("新名字", to),
		),
		caption("会把 source/<原选手>/ 整个目录改名为新名字，并更新配置。"),
	)

	dialog.ShowCustomConfirm("选手改名", "改名", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		newName := strings.TrimSpace(to.Text)
		if newName == "" {
			r.page.g.showError(fmt.Errorf("新名字不能为空"))
			return
		}
		if err := r.page.renamePlayer(from.Selected, newName); err != nil {
			r.page.g.showError(err)
			return
		}
		r.reload()
		r.page.g.setStatus(fmt.Sprintf("%s → %s", from.Selected, newName))
	}, r.page.g.win)
}

// removeSelected 从比赛中删除一行提交（只删配置与源码文件）。
func (r *contestRoster) removeSelected() {
	if r.selected < 0 || r.selected >= len(r.entries) {
		r.page.g.showError(fmt.Errorf("先在上面选中一行"))
		return
	}
	e := r.entries[r.selected]
	dialog.ShowConfirm("删除提交",
		fmt.Sprintf("将从比赛中移除 %s / %s（连同源码文件）。", e.Player, e.Problem),
		func(ok bool) {
			if !ok {
				return
			}
			if err := r.page.removeSubmission(e.Player, e.Problem); err != nil {
				r.page.g.showError(err)
				return
			}
			r.reload()
			r.page.g.setStatus("已删除 " + e.Player + " / " + e.Problem)
		}, r.page.g.win)
}

// ---- contestPage 上的提交管理 ----

// addSubmission 复制一份源码进比赛目录并登记到配置。
func (p *contestPage) addSubmission(player, problem, srcPath string) error {
	prob, ok := p.c.problemOf(problem)
	if !ok {
		return fmt.Errorf("题目 %s 不存在", problem)
	}
	_ = prob
	if !fileExists(srcPath) {
		return fmt.Errorf("源码文件不存在：%s", srcPath)
	}

	rel := filepath.ToSlash(filepath.Join("source", sanitizeFileName(player), sanitizeFileName(problem)+".cpp"))
	dest := filepath.Join(p.c.Dir, filepath.FromSlash(rel))
	if err := copyFile(srcPath, dest); err != nil {
		return fmt.Errorf("复制源码失败：%v", err)
	}

	// 同一个人同一道题只保留一条记录。
	replaced := false
	for i := range p.c.Cfg.Sources {
		if p.c.Cfg.Sources[i].Player == player && p.c.Cfg.Sources[i].Problem == problem {
			p.c.Cfg.Sources[i].Path = rel
			replaced = true
			break
		}
	}
	if !replaced {
		p.c.Cfg.Sources = append(p.c.Cfg.Sources, contestSource{Player: player, Problem: problem, Path: rel})
	}
	p.ensurePlayer(player)
	p.persist()
	if p.editor != nil {
		p.editor.reload()
	}
	return nil
}

// removeSubmission 删除一份提交的配置与源码文件。
func (p *contestPage) removeSubmission(player, problem string) error {
	kept := p.c.Cfg.Sources[:0]
	var removedPath string
	for _, s := range p.c.Cfg.Sources {
		if s.Player == player && s.Problem == problem {
			removedPath = s.Path
			continue
		}
		kept = append(kept, s)
	}
	p.c.Cfg.Sources = kept
	if removedPath != "" {
		_ = os.Remove(filepath.Join(p.c.Dir, filepath.FromSlash(removedPath)))
	}
	// 选手没有其它提交了就把选手记录也去掉。
	stillHas := false
	for _, s := range p.c.Cfg.Sources {
		if s.Player == player {
			stillHas = true
			break
		}
	}
	if !stillHas {
		var players []contestPlayer
		for _, pl := range p.c.Cfg.Players {
			if pl.Name != player {
				players = append(players, pl)
			}
		}
		p.c.Cfg.Players = players
	}
	p.persist()
	if p.editor != nil {
		p.editor.reload()
	}
	return nil
}

// renamePlayer 改名并移动源码目录。
func (p *contestPage) renamePlayer(from, to string) error {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" || from == to {
		return nil
	}
	for _, pl := range p.c.Cfg.Players {
		if pl.Name == to {
			return fmt.Errorf("已经存在选手 %s", to)
		}
	}

	oldDir := filepath.Join(p.c.sourceDir(), sanitizeFileName(from))
	newDir := filepath.Join(p.c.sourceDir(), sanitizeFileName(to))
	if dirExists(oldDir) {
		if dirExists(newDir) {
			return fmt.Errorf("目录 %s 已存在，改名前请先清理", newDir)
		}
		if err := os.Rename(oldDir, newDir); err != nil {
			return fmt.Errorf("移动目录失败：%v", err)
		}
	}

	for i := range p.c.Cfg.Players {
		if p.c.Cfg.Players[i].Name == from {
			p.c.Cfg.Players[i].Name = to
		}
	}
	for i := range p.c.Cfg.Sources {
		if p.c.Cfg.Sources[i].Player == from {
			p.c.Cfg.Sources[i].Player = to
			p.c.Cfg.Sources[i].Path = filepath.ToSlash(filepath.Join("source", sanitizeFileName(to), sanitizeFileName(p.c.Cfg.Sources[i].Problem)+".cpp"))
		}
	}
	p.persist()
	if p.editor != nil {
		p.editor.reload()
	}
	return nil
}

// ensurePlayer 确保选手在选手表里。
func (p *contestPage) ensurePlayer(name string) {
	for _, pl := range p.c.Cfg.Players {
		if pl.Name == name {
			return
		}
	}
	p.c.Cfg.Players = append(p.c.Cfg.Players, contestPlayer{Name: name})
}

// rosterRow 是选手列表的一行。
type rosterRow struct {
	widget.BaseWidget
	name *widget.Label
	info *widget.Label
}

func newRosterRow() *rosterRow {
	r := &rosterRow{name: strong(""), info: caption("")}
	r.name.Truncation = fyne.TextTruncateEllipsis
	r.info.Truncation = fyne.TextTruncateEllipsis
	r.ExtendBaseWidget(r)
	return r
}

func (r *rosterRow) update(player, problem string, missing, selected bool) {
	r.name.SetText(player)
	r.info.SetText(problem + ".cpp")
	if missing {
		r.info.SetText(problem + ".cpp · 源码缺失")
		r.name.Importance = widget.DangerImportance
	} else if selected {
		r.name.Importance = widget.HighImportance
	} else {
		r.name.Importance = widget.MediumImportance
	}
}

func (r *rosterRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(container.NewVBox(r.name, r.info)))
}
