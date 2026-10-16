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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 项目库：baseDir/registry，每行一个项目绝对路径。
// 项目可以建在任意位置，这里把它们统一登记起来。
func registryPath() string { return filepath.Join(baseDir, "registry") }

func registryRead() []string {
	var paths []string
	b, err := os.ReadFile(registryPath())
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			paths = append(paths, l)
		}
	}
	return paths
}

func registryWrite(paths []string) {
	if len(paths) == 0 {
		_ = os.Remove(registryPath())
		return
	}
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.WriteFile(registryPath(), []byte(strings.Join(paths, "\n")+"\n"), 0644)
}

// registryAdd 登记项目目录，返回是否为新条目。
func registryAdd(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fatal("无法解析路径: %v", err)
	}
	paths := registryRead()
	for _, p := range paths {
		if p == abs {
			return false
		}
	}
	registryWrite(append(paths, abs))
	return true
}

// registryRemove 从项目库移除目录（不删除文件），返回是否移除了条目。
func registryRemove(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	paths := registryRead()
	var kept []string
	found := false
	for _, p := range paths {
		if p == abs {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if found {
		registryWrite(kept)
	}
	return found
}

// registryFind 按目录名在项目库中查找。
func registryFind(name string) []string {
	var hits []string
	for _, dir := range registryRead() {
		if filepath.Base(dir) == name {
			hits = append(hits, dir)
		}
	}
	return hits
}

// resolveRegistry 根据名称取唯一项目目录，不唯一或不存在则报错退出。
func resolveRegistry(name string) string {
	hits := registryFind(name)
	switch len(hits) {
	case 0:
		fatal("项目库中没有 '%s'", name)
	case 1:
		if _, err := os.Stat(hits[0]); err != nil {
			fatal("目录不存在: %s", hits[0])
		}
		return hits[0]
	default:
		fatal("'%s' 匹配多个项目:\n  %s", name, strings.Join(hits, "\n  "))
	}
	return ""
}

// cmdLs 列出项目库中的全部项目。
func cmdLs() {
	paths := registryRead()
	if len(paths) == 0 {
		fmt.Println("（项目库为空，用 'otk init' 创建项目）")
		return
	}
	width := 0
	for _, dir := range paths {
		if n := len(filepath.Base(dir)); n > width {
			width = n
		}
	}
	alive := 0
	for _, dir := range paths {
		name := filepath.Base(dir)
		pad := strings.Repeat(" ", width-len(name))
		if _, err := os.Stat(dir); err != nil {
			fmt.Printf("  %s%s%s%s  %s缺失%s\n", yellow, name, reset, pad, yellow, reset)
			continue
		}
		fmt.Printf("  %s%s%s%s  %s\n", cyan, name, reset, pad, dir)
		alive++
	}
	fmt.Printf("共 %d 个项目\n", alive)
}

// cmdSearch 按正则搜索项目（匹配名称、路径与标签）。
func cmdSearch(args []string) {
	if len(args) != 1 {
		fatal("用法: otk se <正则>")
	}
	re, err := regexp.Compile(args[0])
	if err != nil {
		fatal("正则表达式无效: %v", err)
	}
	count := 0
	for _, dir := range registryRead() {
		name := filepath.Base(dir)
		tag := ""
		if fileExists(filepath.Join(dir, ".otk", "config")) {
			tag = readIni(filepath.Join(dir, ".otk", "config"))["tag"]
		}
		if !re.MatchString(name) && !re.MatchString(dir) && (tag == "" || !re.MatchString(tag)) {
			continue
		}
		if tag != "" {
			fmt.Printf("  %s%s%s  %s\n", cyan, name, reset, tag)
		} else {
			fmt.Printf("  %s%s%s\n", cyan, name, reset)
		}
		count++
	}
	if count == 0 {
		fmt.Println("无匹配项目")
	}
}

// cmdRm 从项目库移除项目，可选择清除 OTK 数据或整个项目目录。
func cmdRm(args []string) {
	mode := "soft"
	selectedMode := ""
	name := ""
	for _, arg := range args {
		switch arg {
		case "--soft", "--hard":
			if selectedMode != "" && selectedMode != arg {
				fatal("--soft 和 --hard 不能同时使用")
			}
			selectedMode = arg
			mode = strings.TrimPrefix(arg, "--")
		default:
			if strings.HasPrefix(arg, "-") {
				fatal("未知选项: %s", arg)
			}
			if name != "" {
				fatal("用法: otk rm [--soft|--hard] [名称]")
			}
			name = arg
		}
	}

	var dir string
	if name == "" {
		p, ok := findProject()
		if !ok {
			fatal("当前目录不属于任何项目")
		}
		dir = p.Dir
	} else {
		dir = resolveRegistry(name)
	}
	if !registryContains(dir) {
		fatal("项目不在项目库中: %s", dir)
	}

	switch mode {
	case "soft":
		p := project{Name: projectNameOf(dir), Dir: dir}
		if err := removeOTKData(p); err != nil {
			fatal("无法移除项目 OTK 数据: %v", err)
		}
	case "hard":
		if err := os.RemoveAll(dir); err != nil {
			fatal("无法删除项目目录 %s: %v", dir, err)
		}
	}

	if !registryRemove(dir) {
		fatal("项目不在项目库中: %s", dir)
	}
	if mode == "hard" {
		fmt.Printf("已删除项目 %s 及其项目库记录\n", filepath.Base(dir))
	} else {
		fmt.Printf("已移除项目 %s 的 OTK 数据及项目库记录（源码和样例保留）\n", filepath.Base(dir))
	}
}

func registryContains(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for _, registered := range registryRead() {
		if filepath.Clean(registered) == filepath.Clean(abs) {
			return true
		}
	}
	return false
}

func removeOTKData(p project) error {
	for _, path := range []string{
		p.metaDir(),
		filepath.Join(p.Dir, ".stress"),
	} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	legacyConfig := filepath.Join(p.Dir, filepath.Base(p.Dir)+".ini")
	if err := removeFileIfExists(legacyConfig); err != nil {
		return err
	}

	if p.Name != "" && filepath.Base(p.Name) == p.Name && !strings.ContainsAny(p.Name, `/\\`) {
		for _, name := range []string{p.Name, "gen", "brute"} {
			if err := removeFileIfExists(exePathIn(p.Dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeFileIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
