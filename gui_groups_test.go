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

import "testing"

func TestProjectTreeModel(t *testing.T) {
	model := newProjectTreeModel([]projectTreeEntry{
		{Dir: "/p/a", Name: "A", Group: "基础"},
		{Dir: "/p/b", Name: "B", Group: "基础"},
		{Dir: "/p/c", Name: "C"},
	})

	rootChildren := model.children(projectTreeRootID)
	if len(rootChildren) != 2 {
		t.Fatalf("root children = %#v", rootChildren)
	}
	group := projectTreeGroupID("基础")
	if !containsString(rootChildren, group) || !containsString(rootChildren, projectTreeUngroupedID) {
		t.Fatalf("root children missing group/ungrouped: %#v", rootChildren)
	}
	if got := model.children(group); len(got) != 2 {
		t.Fatalf("group children = %#v", got)
	}
	if got := model.children(projectTreeUngroupedID); len(got) != 1 {
		t.Fatalf("ungrouped children = %#v", got)
	}
	if dir, ok := model.dirForID(projectTreeProjectID("/p/a")); !ok || dir != "/p/a" {
		t.Fatalf("dirForID = %q, %v", dir, ok)
	}
}
