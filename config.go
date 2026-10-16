//    OierToolKit - A lightweight tool designed for practicing and evaluating
//  in informatics competitions.
//    Copyright (C) 2026  魇珩Gr3yPh4ntom
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
		fatal("用法: otk config 或 otk config <项> <值>（time/memory/version/o2/editor）")
	}
}

func showConfig() {
	globalProps := readIni(globalConfigPath())
	fmt.Printf("editor=%s\n", propsOr(globalProps, "editor", defaultEditorCommand()))

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
}

func setConfig(item, value string) {
	if strings.EqualFold(item, "editor") {
		props := readIni(globalConfigPath())
		props["editor"] = value
		writeIni(globalConfigPath(), props)
		fmt.Printf("editor=%s\n", value)
		return
	}

	p := requireProject()
	props := p.cfg()

	switch strings.ToLower(item) {
	case "time":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			fatal("time 必须是数字 (如 1.00)")
		}
		props["time_limit"] = value
		fmt.Printf("time=%ss\n", value)

	case "memory", "mem":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			fatal("memory 必须是数字 (如 125)")
		}
		props["memory_limit"] = value
		fmt.Printf("memory=%sMB\n", value)

	case "version", "ver":
		props["version"] = value
		fmt.Printf("version=%s\n", value)

	case "o2":
		switch strings.ToLower(value) {
		case "on", "true", "1":
			props["o2"] = "1"
			value = "on"
		case "off", "false", "0":
			props["o2"] = "0"
			value = "off"
		default:
			fatal("o2 必须是 on/off")
		}
		fmt.Printf("o2=%s\n", value)

	default:
		fatal("未知配置项 '%s'（可用: time/memory/version/o2）", item)
	}

	writeIni(p.configPath(), props)
}

func globalConfigPath() string { return filepath.Join(baseDir, "config") }

func defaultEditorCommand() string {
	if runtime.GOOS == "windows" {
		return "notepad %s"
	}
	return "vim %s"
}
