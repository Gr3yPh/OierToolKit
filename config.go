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
	"runtime"
	"strconv"
	"strings"
	"time"
)

func cmdConfig(args []string) {
	// 子命令先判：export / import 与"设某项配置"不冲突，
	// 因为配置项里没有叫这两个名字的。
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "export", "save":
			cmdConfigExport(args[1:])
			return
		case "import", "load":
			cmdConfigImport(args[1:])
			return
		}
	}
	switch {
	case len(args) == 0:
		showConfig()
	case len(args) == 2:
		setConfig(args[0], args[1])
	case len(args) >= 2 && strings.ToLower(args[0]) == "remote":
		cmdRemoteConfig(args[1:])
	default:
		fatal(tr("用法: otk config [<项> <值>]"))
	}
}

// cmdConfigExport 把全局配置打包成 zip。
//
//	otk config export [文件.zip] [-r] [-t]
//
// -r 连项目库一起导出，-t 连 OJ 令牌一起导出（默认不带，凭据不外流）。
func cmdConfigExport(args []string) {
	opts := configExportOptions{}
	path := ""
	for _, a := range args {
		switch a {
		case "-r", "--registry":
			opts.IncludeRegistry = true
		case "-t", "--tokens":
			opts.IncludeTokens = true
		case "-h", "--help":
			fmt.Println(tr("用法: otk config export [文件.zip] [-r] [-t]"))
			return
		default:
			if strings.HasPrefix(a, "-") || path != "" {
				fatal(tr("用法: otk config export [文件.zip] [-r] [-t]"))
			}
			path = a
		}
	}
	if path == "" {
		path = "otk-config-" + time.Now().Format("20060102") + ".zip"
	}
	info, err := exportConfigArchive(path, opts)
	if err != nil {
		fatal("%v", err)
	}
	extra := ""
	if info.WithRegistry {
		extra += tr("，含项目库")
	}
	if info.WithTokens {
		extra += tr("，含令牌")
	}
	fmt.Printf(tr("配置已导出: %s（%d 项%s）\n"), path, info.Keys, extra)
}

// cmdConfigImport 从 zip 导入配置。
//
//	otk config import <文件.zip> [-f] [-t] [-r]
//
// 默认只补本地没有的项；-f 覆盖同名项，-t 导入令牌，-r 导入项目库。
func cmdConfigImport(args []string) {
	opts := configImportOptions{}
	path := ""
	for _, a := range args {
		switch a {
		case "-f", "--overwrite":
			opts.Overwrite = true
		case "-t", "--tokens":
			opts.IncludeTokens = true
		case "-r", "--registry":
			opts.IncludeRegistry = true
		case "-h", "--help":
			fmt.Println(tr("用法: otk config import <文件.zip> [-f] [-t] [-r]"))
			return
		default:
			if strings.HasPrefix(a, "-") || path != "" {
				fatal(tr("用法: otk config import <文件.zip> [-f] [-t] [-r]"))
			}
			path = a
		}
	}
	if path == "" {
		fatal(tr("用法: otk config import <文件.zip> [-f] [-t] [-r]"))
	}
	res, err := importConfigArchive(path, opts)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf(tr("从 %s 导入配置: %s\n"), res.From, configArchiveSummary(res))
	if len(res.Added) > 0 {
		fmt.Printf(tr("新增: %s\n"), strings.Join(res.Added, " "))
	}
	if len(res.Updated) > 0 {
		fmt.Printf(tr("覆盖: %s\n"), strings.Join(res.Updated, " "))
	}
	if len(res.Skipped) > 0 {
		fmt.Printf(tr("保留本地: %s\n"), strings.Join(res.Skipped, " "))
	}
	fmt.Println(tr("部分改动（主题、语言）重启后生效。"))
}

// cmdRemoteConfig 管理命名远端列表：
//
//	otk config remote ls                    列出全部远端（令牌打码）
//	otk config remote add <名> <地址>        新增远端
//	otk config remote token <名> <令牌>      设置远端令牌
//	otk config remote rm <名>               删除远端
func cmdRemoteConfig(args []string) {
	switch {
	case len(args) == 0 || args[0] == "ls" || args[0] == "list":
		remotes := listRemotes()
		if len(remotes) == 0 {
			fmt.Println(tr("（还没有配置远端，otk config remote add <名> <地址> 添加）"))
			return
		}
		width := 0
		for _, r := range remotes {
			if len(r.Name) > width {
				width = len(r.Name)
			}
		}
		for _, r := range remotes {
			token := tr("无令牌")
			if r.Token != "" {
				token = tr("有令牌")
			}
			fmt.Printf("%-*s  %s  %s\n", width, r.Name, r.Base, token)
		}
	case len(args) == 3 && (args[0] == "add" || args[0] == "set"):
		if err := saveRemote(args[1], args[2], ""); err != nil {
			fatal("%v", err)
		}
		fmt.Printf(tr("远端 %s = %s")+"\n", args[1], strings.TrimRight(args[2], "/"))
	case len(args) == 3 && args[0] == "token":
		name := strings.TrimSpace(args[1])
		if err := saveRemoteToken(name, args[2]); err != nil {
			fatal("%v", err)
		}
		fmt.Printf(tr("远端 %s 令牌已更新")+"\n", name)
	case len(args) == 2 && (args[0] == "rm" || args[0] == "remove"):
		if err := removeRemote(args[1]); err != nil {
			fatal("%v", err)
		}
		fmt.Printf(tr("已删除远端 %s")+"\n", args[1])
	default:
		fatal(tr("用法: otk config remote ls | add <名> <地址> | token <名> <令牌> | rm <名>"))
	}
}

// saveRemoteToken 只更新某条远端的令牌（地址保持不变）。
func saveRemoteToken(name, token string) error {
	props := readIni(globalConfigPath())
	key := "token." + name
	if name == legacyRemoteName {
		if props["remote"] == "" {
			return fmt.Errorf(tr("没有名为 '%s' 的远端"), name)
		}
		key = "remote_token"
	} else if _, ok := props["remote."+name]; !ok {
		return fmt.Errorf(tr("没有名为 '%s' 的远端"), name)
	}
	props[key] = strings.TrimSpace(token)
	writeIni(globalConfigPath(), props)
	return nil
}

func showConfig() {
	globalProps := readIni(globalConfigPath())
	fmt.Printf("editor=%s\n", propsOr(globalProps, "editor", defaultEditorCommand()))
	remotes := listRemotes()
	for _, r := range remotes {
		fmt.Printf("remote[%s]=%s\n", r.Name, r.Base)
	}

	p, ok := findProject()
	if !ok {
		return
	}
	props := p.cfg()

	o2 := propsOr(props, "o2", "1")
	if o2 == "1" {
		o2 = "on"
	} else {
		o2 = "off"
	}
	fmt.Printf("time=%ss memory=%sMB compiler=%s version=%s o2=%s\n",
		propsOr(props, "time_limit", "1.00"),
		propsOr(props, "memory_limit", "125"),
		compilerOf(props),
		propsOr(props, "version", "c++17"),
		o2)
	if propsOr(props, "io_mode", "") == "file" {
		fmt.Printf("io=file in=%s out=%s\n",
			propsOr(props, "io_in", tr("（<源码名>.in）")),
			propsOr(props, "io_out", tr("（<源码名>.out）")))
	}
	if c := props["cflags"]; c != "" {
		fmt.Printf("cflags=%s\n", c)
	}
	if spjEnabled(p, props) {
		fmt.Println("spj=on（checker.cpp）")
	} else if strings.ToLower(strings.TrimSpace(props["spj"])) == "off" {
		fmt.Println("spj=off")
	}
	if tag := props["tag"]; tag != "" {
		fmt.Printf("tag=%s\n", tag)
	}
	if b := props["remote_base"]; b != "" {
		fmt.Printf("remote=%s @ %s\n", props["remote_id"], b)
	}
}

func setConfig(item, value string) {
	if isGlobalConfigItem(item) {
		msg, err := setGlobalConfig(item, value)
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(msg)
		return
	}

	p := requireProject()
	msg, err := setProjectConfig(p, item, value)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println(msg)
}

// isGlobalConfigItem 判断配置项是否属于全局配置（不依赖当前项目）。
func isGlobalConfigItem(item string) bool {
	switch strings.ToLower(item) {
	case "editor", "remote", "remote_token":
		return true
	}
	return false
}

// setGlobalConfig 写入全局配置项，返回给用户看的结果文案。
func setGlobalConfig(item, value string) (string, error) {
	props := readIni(globalConfigPath())
	switch strings.ToLower(item) {
	case "editor":
		props["editor"] = value
	case "remote":
		props["remote"] = strings.TrimRight(value, "/")
	case "remote_token":
		props["remote_token"] = value
	default:
		return "", fmt.Errorf(tr("未知配置项 '%s'"), item)
	}
	writeIni(globalConfigPath(), props)

	switch strings.ToLower(item) {
	case "editor":
		return "editor=" + props["editor"], nil
	case "remote":
		return "remote=" + props["remote"], nil
	default:
		return "remote_token=****", nil
	}
}

// setProjectConfig 写入项目配置项，返回给用户看的结果文案。
func setProjectConfig(p project, item, value string) (string, error) {
	props := p.cfg()
	msg := ""

	switch strings.ToLower(item) {
	case "time":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "", fmt.Errorf("%s", tr("time 必须是数字 (如 1.00)"))
		}
		props["time_limit"] = value
		msg = fmt.Sprintf("time=%ss", value)

	case "memory", "mem":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "", fmt.Errorf("%s", tr("memory 必须是数字 (如 125)"))
		}
		props["memory_limit"] = value
		msg = fmt.Sprintf("memory=%sMB", value)

	case "version", "ver":
		props["version"] = value
		msg = fmt.Sprintf("version=%s", value)

	case "compiler", "cc":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "g++", "cpp", "c++":
			props["compiler"] = "g++"
			msg = "compiler=g++"
		case "gcc", "c":
			props["compiler"] = "gcc"
			msg = tr("compiler=gcc（源码按 C 编译）")
		default:
			return "", fmt.Errorf("%s", tr("compiler 只支持 g++ 或 gcc"))
		}

	case "o2":
		switch strings.ToLower(value) {
		case "on", "true", "1":
			props["o2"] = "1"
			msg = "o2=on"
		case "off", "false", "0":
			props["o2"] = "0"
			msg = "o2=off"
		default:
			return "", fmt.Errorf("%s", tr("o2 必须是 on/off"))
		}

	case "threads", "thread":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return "", fmt.Errorf("%s", tr("threads 必须是非负整数（0 = 按 CPU 核心数自动，上限 8）"))
		}
		props["threads"] = strconv.Itoa(n)
		switch {
		case n == 0:
			msg = tr("threads=0（自动并发）")
		case n == 1:
			msg = tr("threads=1（串行）")
		default:
			msg = fmt.Sprintf("threads=%d", n)
		}

	case "io":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "file":
			props["io_mode"] = "file"
			msg = tr("io=file（文件读写，freopen）")
		case "std", "stdio", "stdin", "stdout", "":
			props["io_mode"] = ""
			msg = tr("io=stdio（标准输入输出）")
		default:
			return "", fmt.Errorf("%s", tr("io 必须是 std/file"))
		}

	case "io_in":
		props["io_in"] = strings.TrimSpace(value)
		if props["io_in"] == "" {
			msg = tr("io_in=（缺省用 <源码名>.in）")
		} else {
			msg = "io_in=" + props["io_in"]
		}

	case "io_out":
		props["io_out"] = strings.TrimSpace(value)
		if props["io_out"] == "" {
			msg = tr("io_out=（缺省用 <源码名>.out）")
		} else {
			msg = "io_out=" + props["io_out"]
		}

	case "cflags":
		// 额外编译参数（如 -Wall）：引号语义与命令行一致。
		value = strings.TrimSpace(value)
		if value != "" && len(splitQuoted(value)) == 0 {
			return "", fmt.Errorf("%s", tr("cflags 格式无效"))
		}
		props["cflags"] = value
		if value == "" {
			msg = tr("cflags=（无额外编译参数）")
		} else {
			msg = "cflags=" + value
		}

	case "spj":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "on", "true", "1":
			props["spj"] = "1"
			msg = tr("spj=on（存在 checker.cpp 时启用特殊判题）")
		case "off", "false", "0":
			props["spj"] = "off"
			msg = "spj=off"
		default:
			return "", fmt.Errorf("%s", tr("spj 必须是 on/off"))
		}

	case "remote_base":
		props["remote_base"] = strings.TrimRight(value, "/")
		msg = fmt.Sprintf("remote_base=%s", props["remote_base"])

	case "remote_id":
		props["remote_id"] = value
		msg = fmt.Sprintf("remote_id=%s", value)

	default:
		return "", fmt.Errorf(tr("未知配置项 '%s'（项目: time/memory/version/compiler/o2/threads/io/io_in/io_out/cflags/spj/remote_base/remote_id；全局: editor/remote/remote_token；远端管理: otk config remote ls|add|token|rm）"), item)
	}

	writeIni(p.configPath(), props)
	return msg, nil
}

func globalConfigPath() string { return filepath.Join(baseDir, "config") }

func defaultEditorCommand() string {
	if runtime.GOOS == "windows" {
		return "notepad %s"
	}
	return "vim %s"
}
