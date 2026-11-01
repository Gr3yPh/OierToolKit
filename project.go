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
)

// project 代表一个项目。目录中含 .otk/ 即为项目根，位置任意，
// 统一登记在项目库（baseDir/registry）中管理。
type project struct {
	Name string
	Dir  string
}

// findProject 仿照 git 的仓库发现机制：从当前目录逐级向上，
// 找到第一个含 .otk/ 标记的目录。
func findProject() (project, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return project{}, false
	}
	dir := filepath.Clean(cwd)
	for {
		if isProjectRoot(dir) {
			return project{
				Name: projectNameOf(dir),
				Dir:  dir,
			}, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return project{}, false
		}
		dir = parent
	}
}

// isProjectRoot 判断目录是否为项目根：新布局看 .otk/config 文件，
// 同时兼容旧布局的 <目录名>.ini。
func isProjectRoot(dir string) bool {
	if fileExists(filepath.Join(dir, ".otk", "config")) {
		return true
	}
	_, err := os.Stat(filepath.Join(dir, filepath.Base(dir)+".ini"))
	return err == nil
}

func projectNameOf(dir string) string {
	name := readIni(projectConfigPathOf(dir))["name"]
	if name == "" {
		name = filepath.Base(dir)
	}
	return name
}

func projectConfigPathOf(dir string) string {
	newPath := filepath.Join(dir, ".otk", "config")
	if fileExists(newPath) {
		return newPath
	}
	return filepath.Join(dir, filepath.Base(dir)+".ini")
}

// requireProject 返回当前项目，检测不到则报错退出。
func requireProject() project {
	p, ok := findProject()
	if !ok {
		cwd, _ := os.Getwd()
		fatal(tr("当前目录不属于任何项目: %s\n用 'otk init' 在此创建项目。"), cwd)
	}
	return p
}

func (p project) metaDir() string     { return filepath.Join(p.Dir, ".otk") }
func (p project) samplesPath() string { return filepath.Join(p.metaDir(), "samples") }

// codeName 是代码文件的基名：固定跟随目录名，与项目标签（Name）解耦。
// 标签可以是任意文字（如题面标题），目录名才是文件系统友好的身份。
func (p project) codeName() string { return filepath.Base(p.Dir) }

func (p project) sourcePath() string     { return filepath.Join(p.Dir, p.codeName()+".cpp") }
func (p project) exePath() string        { return exePathIn(p.Dir, p.codeName()) }
func (p project) cfg() map[string]string { return readIni(p.configPath()) }

// configPath 优先读新布局，旧布局项目尚未迁移时读同名 .ini。
func (p project) configPath() string {
	return projectConfigPathOf(p.Dir)
}

// cmdInit 在当前目录初始化项目，并登记到项目库。
func cmdInit(args []string) {
	var name, tplKey string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-t" || a == "--template":
			if i+1 >= len(args) {
				fatal(tr("用法: otk init [-t 模板] [名称]"))
			}
			i++
			tplKey = args[i]
		case strings.HasPrefix(a, "--template="):
			tplKey = strings.TrimPrefix(a, "--template=")
		case strings.HasPrefix(a, "-t="):
			tplKey = strings.TrimPrefix(a, "-t=")
		case strings.HasPrefix(a, "-"):
			fatal(tr("用法: otk init [-t 模板] [名称]"))
		default:
			if name != "" {
				fatal(tr("用法: otk init [-t 模板] [名称]"))
			}
			name = a
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatal(tr("无法获取当前目录: %v"), err)
	}
	if name == "" {
		name = filepath.Base(cwd)
	}

	var tmpl *projectTemplate
	if tplKey != "" {
		t, ok := findTemplate(tplKey)
		if !ok {
			fatal(tr("没有这个模板：%s（可用：%s）"), tplKey, templateNames())
		}
		tmpl = &t
	}

	p, res, created, err := initProjectAtWith(cwd, name, tmpl)
	if err != nil {
		fatal("%v", err)
	}

	if created {
		fmt.Printf(tr("已初始化项目 %s%s%s (%s)\n"), cyan, p.Name, reset, p.Dir)
	} else {
		fmt.Printf(tr("项目 %s%s%s 已初始化\n"), cyan, p.Name, reset)
	}
	if tmpl != nil {
		fmt.Printf(tr("模板：%s（已有文件不会被覆盖）\n"), templateLabel(*tmpl))
	}
	if len(res.added) > 0 {
		ids := make([]string, 0, len(res.added))
		for _, id := range res.added {
			ids = append(ids, "#"+id)
		}
		fmt.Printf(tr("已登记样例: %s\n"), strings.Join(ids, " "))
	}
	printPairWarnings(p, res)
}

// initProjectAt 在 dir 里初始化项目（创建 .otk/config、源码文件、登记项目
// 库并同步样例）。created 表示配置是否为本次新建。不打印、不退出。
func initProjectAt(dir, name string) (project, refreshResult, bool, error) {
	return initProjectAtWith(dir, name, nil)
}

// initProjectAtWith 与 initProjectAt 相同，但按模板写源码骨架、配置预设值、
// 附加文件与初始样例。tmpl 为 nil 时就是原来的空白项目。
//
// 模板只在**文件不存在**时生效：对一个已有项目再"用模板初始化"，
// 已有的源码、配置、样例都不会被改动。
func initProjectAtWith(dir, name string, tmpl *projectTemplate) (project, refreshResult, bool, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return project{}, refreshResult{}, false, fmt.Errorf(tr("非法的项目名称: %s"), name)
	}

	base := filepath.Base(dir)
	created := false
	cfgPath := filepath.Join(dir, ".otk", "config")
	oldLabel := readIni(cfgPath)["name"] // 现有项目的旧标签，用于源码文件名迁移
	if !fileExists(cfgPath) {
		// 旧版布局迁移：<名称>.ini → .otk/config
		props := map[string]string{}
		legacy := filepath.Join(dir, name+".ini")
		migrated := false
		if fileExists(legacy) {
			props = readIni(legacy)
			oldLabel = props["name"]
			migrated = true
		}
		// 老项目迁移时以它自己的配置为准，模板只负责全新的目录。
		if tmpl != nil && !migrated {
			for k, v := range tmpl.Props {
				props[k] = tmpl.render(v, base)
			}
		}
		props["name"] = name
		setDefault(props, "time_limit", "1.00")
		setDefault(props, "memory_limit", "125")
		setDefault(props, "version", "c++17")
		setDefault(props, "o2", "1")
		writeIni(cfgPath, props)
		if fileExists(legacy) {
			_ = os.Remove(legacy)
		}
		created = true
	}

	src := filepath.Join(dir, base+".cpp")
	// 旧版本以项目名作源码文件名，旧标签与目录名不同时自动改名迁移。
	if !fileExists(src) && oldLabel != "" && oldLabel != base && !strings.ContainsAny(oldLabel, `/\`) {
		if fileExists(filepath.Join(dir, oldLabel+".cpp")) {
			_ = os.Rename(filepath.Join(dir, oldLabel+".cpp"), src)
		}
	}
	if !fileExists(src) {
		skeleton := defaultSourceSkeleton
		if tmpl != nil && strings.TrimSpace(tmpl.Source) != "" {
			skeleton = tmpl.render(tmpl.Source, base)
		}
		if err := os.WriteFile(src, []byte(skeleton), 0644); err != nil {
			return project{}, refreshResult{}, created, fmt.Errorf(tr("无法创建源码文件: %v"), err)
		}
	}
	if tmpl != nil {
		if err := applyTemplateExtras(dir, *tmpl); err != nil {
			return project{}, refreshResult{}, created, err
		}
	}

	registryAdd(dir)

	p := project{Name: projectNameOf(dir), Dir: dir}
	if p.Name == "" {
		p.Name = name
	}
	res := syncSamples(p, true)
	return p, res, created, nil
}

// cmdStatus 输出当前项目状态。
func cmdStatus() {
	p := requireProject()
	props := p.cfg()

	o2 := "off"
	if propsOr(props, "o2", "1") == "1" {
		o2 = "O2"
	}
	fmt.Printf("%s%s%s  %s\n", cyan, p.Name, reset, p.Dir)
	fmt.Printf(tr("  %s.cpp · %ss / %sMB · %s %s · 样例 %d\n"),
		p.codeName(),
		propsOr(props, "time_limit", "1.00"),
		propsOr(props, "memory_limit", "125"),
		propsOr(props, "version", "c++17"),
		o2,
		len(readIndex(p)))
}

func setDefault(props map[string]string, key, value string) {
	if props[key] == "" {
		props[key] = value
	}
}

func readIni(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	return parseIni(b)
}

// parseIni 解析 ini 文本。注释行（# 与 ;）跳过，只认 key=value。
func parseIni(b []byte) map[string]string {
	props := make(map[string]string)
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		if parts := strings.SplitN(l, "=", 2); len(parts) == 2 {
			props[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return props
}

// iniText 把配置渲染成 ini 文本：键名排序，输出稳定，方便 diff。
func iniText(props map[string]string) string {
	lines := []string{"# Updated by otk (Go version)"}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, props[k]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeIni(path string, props map[string]string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(iniText(props)), 0644)
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func propsOr(props map[string]string, key, def string) string {
	if v := props[key]; v != "" {
		return v
	}
	return def
}

func floatProp(props map[string]string, key string, def float64) float64 {
	if v, err := strconv.ParseFloat(props[key], 64); err == nil {
		return v
	}
	return def
}
