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
)

func cmdConfig(args []string) {
	switch len(args) {
	case 0:
		showConfig()
	case 2:
		setConfig(args[0], args[1])
	default:
		fatal("用法: otk config [<项> <值>]")
	}
}

func showConfig() {
	globalProps := readIni(globalConfigPath())
	fmt.Printf("editor=%s\n", propsOr(globalProps, "editor", defaultEditorCommand()))
	if r := propsOr(globalProps, "remote", ""); r != "" {
		fmt.Printf("remote=%s\n", r)
	}
	if propsOr(globalProps, "remote_token", "") != "" {
		fmt.Println("remote_token=****")
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
	fmt.Printf("time=%ss memory=%sMB version=%s o2=%s\n",
		propsOr(props, "time_limit", "1.00"),
		propsOr(props, "memory_limit", "125"),
		propsOr(props, "version", "c++17"),
		o2)
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
		return "", fmt.Errorf("未知配置项 '%s'", item)
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
			return "", fmt.Errorf("time 必须是数字 (如 1.00)")
		}
		props["time_limit"] = value
		msg = fmt.Sprintf("time=%ss", value)

	case "memory", "mem":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "", fmt.Errorf("memory 必须是数字 (如 125)")
		}
		props["memory_limit"] = value
		msg = fmt.Sprintf("memory=%sMB", value)

	case "version", "ver":
		props["version"] = value
		msg = fmt.Sprintf("version=%s", value)

	case "o2":
		switch strings.ToLower(value) {
		case "on", "true", "1":
			props["o2"] = "1"
			msg = "o2=on"
		case "off", "false", "0":
			props["o2"] = "0"
			msg = "o2=off"
		default:
			return "", fmt.Errorf("o2 必须是 on/off")
		}

	case "threads", "thread":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return "", fmt.Errorf("threads 必须是非负整数（0 = 按 CPU 核心数自动，上限 8）")
		}
		props["threads"] = strconv.Itoa(n)
		switch {
		case n == 0:
			msg = "threads=0（自动并发）"
		case n == 1:
			msg = "threads=1（串行）"
		default:
			msg = fmt.Sprintf("threads=%d", n)
		}

	case "remote_base":
		props["remote_base"] = strings.TrimRight(value, "/")
		msg = fmt.Sprintf("remote_base=%s", props["remote_base"])

	case "remote_id":
		props["remote_id"] = value
		msg = fmt.Sprintf("remote_id=%s", value)

	default:
		return "", fmt.Errorf("未知配置项 '%s'（项目: time/memory/version/o2/threads/remote_base/remote_id；全局: editor/remote/remote_token）", item)
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
