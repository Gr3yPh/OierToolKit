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
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// 项目分组：分组名保存在全局配置 gui_project_groups（| 分隔），项目自身
// 的 .otk/config 里保存 group=<分组名>。这样既能给空分组预留名字，也保证
// 项目目录移动后分组跟着项目走。
const (
	prefProjectGroups = "gui_project_groups"
	newGroupChoice    = "＋ 新建分组…"
)

func noGroupChoiceText() string { return tr("不分组") }

// projectGroups 返回已登记的分组名（去掉空项与重复项）。
func projectGroups() []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, raw := range strings.Split(guiPref(prefProjectGroups, ""), "|") {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// addProjectGroup 登记一个分组名；重复添加视为成功。
func addProjectGroup(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%s", tr("分组名不能为空"))
	}
	if strings.Contains(name, "|") {
		return "", fmt.Errorf("%s", tr("分组名不能包含竖线 |"))
	}
	for _, have := range projectGroups() {
		if have == name {
			return name, nil
		}
	}
	all := append(projectGroups(), name)
	if err := setGuiPref(prefProjectGroups, strings.Join(all, "|")); err != nil {
		return "", err
	}
	return name, nil
}

// projectGroupOfDir 读取项目的分组；目录缺失或未配置时返回空。
func projectGroupOfDir(dir string) string {
	return strings.TrimSpace(readIni(projectConfigPathOf(dir))["group"])
}

// projectGroupOf 读取项目的分组。
func projectGroupOf(p project) string {
	return strings.TrimSpace(p.cfg()["group"])
}

// groupSelectOptions 生成分组下拉框选项：不分组 + 已登记分组 + 新建入口。
func groupSelectOptions(current string) []string {
	options := []string{noGroupChoiceText()}
	for _, name := range projectGroups() {
		options = append(options, name)
	}
	if current != "" && !containsString(options, current) {
		options = append(options, current)
	}
	options = append(options, newGroupChoice)
	return options
}

// newGroupSelect 创建分组选择器。选择「新建分组…」时弹出输入框，
// 创建成功后刷新选项并回调 onChange。
func (g *guiApp) newGroupSelect(current string, onChange func(string)) *widget.Select {
	var sel *widget.Select
	lastValue := current
	if lastValue == "" {
		lastValue = noGroupChoiceText()
	}
	sel = widget.NewSelect(groupSelectOptions(current), func(value string) {
		if value == newGroupChoice {
			// 先回到上一个合法值；取消新建时不把“新建分组…”留在下拉框里。
			sel.SetSelected(lastValue)
			g.newGroupDialog(func(name string) {
				sel.SetOptions(groupSelectOptions(name))
				sel.SetSelected(name)
				if onChange != nil {
					onChange(name)
				}
			})
			return
		}
		lastValue = value
		if value == noGroupChoiceText() {
			value = ""
		}
		if onChange != nil {
			onChange(value)
		}
	})
	if current == "" {
		sel.SetSelected(noGroupChoiceText())
	} else {
		sel.SetSelected(current)
	}
	return sel
}

// newGroupDialog 弹出新建分组输入框。
func (g *guiApp) newGroupDialog(after func(string)) {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(tr("分组名，例如：基础 / 动态规划"))
	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(tr("分组名"), entry)),
	)
	g.showFormConfirm(tr("新建分组"), tr("创建"), tr("取消"), content, func(ok bool) {
		if !ok {
			return
		}
		name, err := addProjectGroup(entry.Text)
		if err != nil {
			g.showError(err)
			return
		}
		g.reloadProjects()
		g.setStatus(fmt.Sprintf(tr("已新建分组 %s"), name))
		if after != nil {
			after(name)
		}
	}, g.win)
}

// projectTreeEntry 是树里的一条项目记录。
type projectTreeEntry struct {
	Dir   string
	Name  string
	Group string
}

// projectTreeModel 把项目列表整理成「分组 → 项目」两级树。
type projectTreeModel struct {
	entries []projectTreeEntry
	groups  []string
}

const (
	projectTreeRootID        = "__otk_root__"
	projectTreeGroupPrefix   = "__otk_group__:"
	projectTreeUngroupedID   = "__otk_ungrouped__"
	projectTreeProjectPrefix = "__otk_project__:"
)

func newProjectTreeModelFromDirs(dirs []string) *projectTreeModel {
	entries := make([]projectTreeEntry, 0, len(dirs))
	for _, dir := range dirs {
		entries = append(entries, projectTreeEntry{
			Dir:   dir,
			Name:  projectNameOf(dir),
			Group: projectGroupOfDir(dir),
		})
	}
	return newProjectTreeModel(entries)
}

func newProjectTreeModelFromMetas(metas []projectMeta) *projectTreeModel {
	entries := make([]projectTreeEntry, 0, len(metas))
	for _, m := range metas {
		entries = append(entries, projectTreeEntry{
			Dir:   m.Dir,
			Name:  m.Name,
			Group: m.Group,
		})
	}
	return newProjectTreeModel(entries)
}

func newProjectTreeModel(entries []projectTreeEntry) *projectTreeModel {
	groupSet := map[string]bool{}
	for _, name := range projectGroups() {
		groupSet[name] = true
	}
	for _, e := range entries {
		if e.Group != "" {
			groupSet[e.Group] = true
		}
	}
	groups := make([]string, 0, len(groupSet))
	for name := range groupSet {
		groups = append(groups, name)
	}
	sort.Strings(groups)
	return &projectTreeModel{entries: entries, groups: groups}
}

func projectTreeGroupID(name string) string {
	return projectTreeGroupPrefix + name
}

func projectTreeProjectID(dir string) string {
	return projectTreeProjectPrefix + dir
}

func (m *projectTreeModel) isBranch(uid string) bool {
	return uid == projectTreeRootID || uid == projectTreeUngroupedID ||
		strings.HasPrefix(uid, projectTreeGroupPrefix)
}

func (m *projectTreeModel) groupLabel(uid string) string {
	switch uid {
	case projectTreeRootID:
		return tr("全部项目")
	case projectTreeUngroupedID:
		return tr("未分组")
	}
	return strings.TrimPrefix(uid, projectTreeGroupPrefix)
}

func (m *projectTreeModel) dirForID(uid string) (string, bool) {
	if !strings.HasPrefix(uid, projectTreeProjectPrefix) {
		return "", false
	}
	return strings.TrimPrefix(uid, projectTreeProjectPrefix), true
}

func (m *projectTreeModel) nameForID(uid string) string {
	dir, ok := m.dirForID(uid)
	if !ok {
		return ""
	}
	for _, e := range m.entries {
		if e.Dir == dir {
			return e.Name
		}
	}
	return projectNameOf(dir)
}

func (m *projectTreeModel) children(uid string) []string {
	projectsIn := func(group string) []string {
		out := make([]string, 0)
		for _, e := range m.entries {
			if e.Group == group {
				out = append(out, projectTreeProjectID(e.Dir))
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			a, _ := m.dirForID(out[i])
			b, _ := m.dirForID(out[j])
			return strings.ToLower(m.nameForID(out[i])+a) < strings.ToLower(m.nameForID(out[j])+b)
		})
		return out
	}

	switch {
	case uid == projectTreeRootID:
		out := make([]string, 0, len(m.groups)+1)
		for _, name := range m.groups {
			out = append(out, projectTreeGroupID(name))
		}
		hasUngrouped := false
		for _, e := range m.entries {
			if e.Group == "" {
				hasUngrouped = true
				break
			}
		}
		if hasUngrouped {
			out = append(out, projectTreeUngroupedID)
		}
		return out
	case uid == projectTreeUngroupedID:
		return projectsIn("")
	case strings.HasPrefix(uid, projectTreeGroupPrefix):
		name := strings.TrimPrefix(uid, projectTreeGroupPrefix)
		if name == "__ungrouped__" {
			return projectsIn("")
		}
		return projectsIn(name)
	}
	return nil
}

// newProjectSidebarTree 构建侧栏的项目树。
func (g *guiApp) newProjectSidebarTree() *widget.Tree {
	tree := widget.NewTree(
		func(uid string) []string {
			if g.treeModel == nil {
				return nil
			}
			return g.treeModel.children(uid)
		},
		func(uid string) bool {
			return g.treeModel != nil && g.treeModel.isBranch(uid)
		},
		func(branch bool) fyne.CanvasObject {
			if branch {
				label := widget.NewLabel("")
				label.Truncation = fyne.TextTruncateEllipsis
				return label
			}
			return newProjectRow(nil)
		},
		func(uid string, branch bool, obj fyne.CanvasObject) {
			if g.treeModel == nil {
				return
			}
			if branch {
				label := obj.(*widget.Label)
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Truncation = fyne.TextTruncateEllipsis
				label.SetText(g.treeModel.groupLabel(uid))
				return
			}
			dir, ok := g.treeModel.dirForID(uid)
			if !ok {
				return
			}
			row := obj.(*projectRow)
			row.update(g.treeModel.nameForID(uid), dir == g.selected, !dirExists(dir))
			row.remove.OnTapped = func() {
				g.removeProjectDialog(project{Name: g.treeModel.nameForID(uid), Dir: dir})
			}
		},
	)
	tree.Root = projectTreeRootID
	tree.OnSelected = func(uid string) {
		if g.treeModel == nil {
			return
		}
		if dir, ok := g.treeModel.dirForID(uid); ok {
			g.showProject(dir)
		}
	}
	return tree
}

// newProjectListTree 构建项目库页的树；选中结果写回 selectedDir。
func (g *guiApp) newProjectListTree(metas []projectMeta, selectedDir *string, sync func()) *widget.Tree {
	model := newProjectTreeModelFromMetas(metas)
	tree := widget.NewTree(
		func(uid string) []string { return model.children(uid) },
		func(uid string) bool { return model.isBranch(uid) },
		func(branch bool) fyne.CanvasObject {
			if branch {
				label := widget.NewLabel("")
				label.Truncation = fyne.TextTruncateEllipsis
				return label
			}
			return widget.NewLabel("")
		},
		func(uid string, branch bool, obj fyne.CanvasObject) {
			if branch {
				label := obj.(*widget.Label)
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Truncation = fyne.TextTruncateEllipsis
				label.SetText(model.groupLabel(uid))
				return
			}
			dir, ok := model.dirForID(uid)
			if !ok {
				return
			}
			label := obj.(*widget.Label)
			label.Truncation = fyne.TextTruncateEllipsis
			label.Wrapping = fyne.TextWrapOff
			label.TextStyle = fyne.TextStyle{Bold: true}
			label.Importance = widget.MediumImportance
			for _, m := range metas {
				if m.Dir == dir {
					if m.Missing {
						label.Importance = widget.DangerImportance
					}
					break
				}
			}
			label.SetText(model.nameForID(uid))
		},
	)
	tree.Root = projectTreeRootID
	tree.OnSelected = func(uid string) {
		if dir, ok := model.dirForID(uid); ok {
			*selectedDir = dir
			if sync != nil {
				sync()
			}
		}
	}
	tree.OnUnselected = func(uid string) {
		if dir, ok := model.dirForID(uid); ok && *selectedDir == dir {
			*selectedDir = ""
			if sync != nil {
				sync()
			}
		}
	}
	tree.OpenAllBranches()
	return tree
}
