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
	"os"
	"path/filepath"
	"strings"
)

// 同一个可执行文件承担两种角色：命令行与图形界面。
//
// 区分方式是看文件名而不是看参数：发行时把同一份二进制分别复制成
// otk（CLI）与 otk-gui（GUI），这样双击哪个图标就进哪个模式，互不干扰。
// 直接运行 otk 时也可以显式写 `otk gui` 进入界面。
var guiBinaryNames = map[string]bool{
	"otk-gui":          true,
	"otk-gui.exe":      true,
	"otkgui":           true,
	"otkgui.exe":       true,
	"oier-toolkit":     true,
	"oier-toolkit.exe": true,
}

// isGuiBinary 判断当前进程是否以 GUI 身份启动（按可执行文件名）。
func isGuiBinary() bool {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	return guiBinaryNames[name]
}
