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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// 顶层工作区标签。和项目内部的 AppTabs 分开：这里一个标签可以是一个项目、
// 一场比赛，也可以是 Start Here / 项目库 / 模拟赛列表。
const (
	tabStart       = "start"
	tabProject     = "project"
	tabProjectList = "projects"
	tabContestList = "contests"
	tabContest     = "contest"
)

type workspaceTab struct {
	kind    string
	id      string // 项目目录或比赛目录；静态页为空
	title   string
	item    *container.TabItem
	content fyne.CanvasObject
	page    *projectPage
	contest *contestPage
}

func (g *guiApp) newWorkspace() *container.DocTabs {
	tabs := container.NewDocTabs()
	tabs.SetTabLocation(container.TabLocationTop)
	tabs.CloseIntercept = g.closeWorkspaceTab
	tabs.OnSelected = func(item *container.TabItem) {
		if t := g.tabs[item]; t != nil {
			g.activateWorkspaceTab(t)
		}
	}
	return tabs
}

func (g *guiApp) findWorkspaceTab(kind, id string) *workspaceTab {
	for _, t := range g.tabs {
		if t.kind == kind && t.id == id {
			return t
		}
	}
	return nil
}

func (g *guiApp) addWorkspaceTab(kind, id, title string, icon fyne.Resource, content fyne.CanvasObject, page *projectPage, contest *contestPage) *workspaceTab {
	t := &workspaceTab{kind: kind, id: id, title: title, content: content, page: page, contest: contest}
	t.item = container.NewTabItemWithIcon(title, icon, content)
	g.tabs[t.item] = t
	g.workspace.Append(t.item)
	g.workspace.Select(t.item)
	return t
}

func (g *guiApp) showStartHere() {
	if t := g.findWorkspaceTab(tabStart, ""); t != nil {
		// Start Here 上的最近项目会变，重新生成内容即可。
		t.content = g.buildStartPage()
		t.item.Content = t.content
		g.workspace.Refresh()
		g.workspace.Select(t.item)
		return
	}
	g.addWorkspaceTab(tabStart, "", tr("开始"), theme.HomeIcon(), g.buildStartPage(), nil, nil)
}

func (g *guiApp) showProject(dir string) {
	if t := g.findWorkspaceTab(tabProject, dir); t != nil {
		g.workspace.Select(t.item)
		return
	}

	if !dirExists(dir) {
		g.addWorkspaceTab(tabProject, dir, filepath.Base(dir), theme.WarningIcon(), g.missingView(dir), nil, nil)
		return
	}

	pg := newProjectPage(g, dir)
	g.addWorkspaceTab(tabProject, dir, pg.p.Name, theme.DocumentIcon(), pg.content, pg, nil)
}

func (g *guiApp) showProjectList() {
	if t := g.findWorkspaceTab(tabProjectList, ""); t != nil {
		t.content = g.buildProjectList()
		t.item.Content = t.content
		g.workspace.Refresh()
		g.workspace.Select(t.item)
		return
	}
	g.addWorkspaceTab(tabProjectList, "", tr("项目库"), theme.ListIcon(), g.buildProjectList(), nil, nil)
}

func (g *guiApp) showContestList() {
	if t := g.findWorkspaceTab(tabContestList, ""); t != nil {
		t.content = g.buildContestList()
		t.item.Content = t.content
		g.workspace.Refresh()
		g.workspace.Select(t.item)
		return
	}
	g.addWorkspaceTab(tabContestList, "", tr("模拟赛"), theme.GridIcon(), g.buildContestList(), nil, nil)
}

func (g *guiApp) showContest(c contest) {
	if t := g.findWorkspaceTab(tabContest, c.Dir); t != nil {
		g.workspace.Select(t.item)
		return
	}
	cp := newContestPage(g, c)
	g.addWorkspaceTab(tabContest, c.Dir, c.Cfg.Name, theme.MediaPlayIcon(), cp.content, nil, cp)
}

func (g *guiApp) activateWorkspaceTab(t *workspaceTab) {
	g.page = t.page
	g.contest = t.contest
	g.selected = ""
	switch t.kind {
	case tabProject:
		g.mode = modeProject
		g.selected = t.id
		g.setStatus(fmt.Sprintf(tr("已打开 %s"), t.title))
	case tabProjectList:
		g.mode = modeProjectList
		g.setStatus(tr("项目库"))
	case tabContest:
		g.mode = modeContest
		g.setStatus(fmt.Sprintf(tr("已打开模拟赛 %s"), t.title))
	case tabContestList:
		g.mode = modeContest
		g.setStatus(tr("模拟赛"))
	default:
		g.mode = modeStart
		g.setStatus(tr("就绪"))
	}
	g.setNav(g.mode)
	if g.list != nil {
		g.list.Refresh()
	}
}

func (g *guiApp) closeWorkspaceTab(item *container.TabItem) {
	t := g.tabs[item]
	if t == nil {
		return
	}
	current := g.workspace.Selected() == item
	index := -1
	for i, it := range g.workspace.Items {
		if it == item {
			index = i
			break
		}
	}
	delete(g.tabs, item)
	if t.page != nil {
		t.page.close()
	}
	if t.contest != nil {
		t.contest.close()
	}
	g.workspace.Remove(item)

	if !current {
		return
	}
	if len(g.workspace.Items) == 0 {
		g.showStartHere()
		return
	}
	if index < 0 || index >= len(g.workspace.Items) {
		index = len(g.workspace.Items) - 1
	}
	g.workspace.SelectIndex(index)
	if sel := g.workspace.Selected(); sel != nil {
		if next := g.tabs[sel]; next != nil {
			g.activateWorkspaceTab(next)
		}
	}
}

func (g *guiApp) closeProjectTab(dir string) {
	if t := g.findWorkspaceTab(tabProject, dir); t != nil {
		g.closeWorkspaceTab(t.item)
	}
}

func (g *guiApp) closeContestTab(dir string) {
	if t := g.findWorkspaceTab(tabContest, dir); t != nil {
		g.closeWorkspaceTab(t.item)
	}
}

// openProjectByDir 用于 Start Here 的最近项目按钮。
func (g *guiApp) openProjectByDir(dir string) {
	if !dirExists(dir) {
		g.setStatus(fmt.Sprintf(tr("目录不存在：%s"), prettyPath(dir)))
		return
	}
	g.showProject(dir)
}
