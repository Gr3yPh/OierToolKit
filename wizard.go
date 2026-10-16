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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cmdWizard 是交互式新手向导：在临时目录建演示项目，引导用户
// 亲手输入命令走完 init → refresh → judge 流程，结束后自动清理。
func cmdWizard() {
	if _, err := exec.LookPath("g++"); err != nil {
		fatal("未找到 g++，wizard 需要编译 C++ 代码")
	}

	dir, err := os.MkdirTemp("", "otk-wizard-")
	if err != nil {
		fatal("无法创建临时目录: %v", err)
	}
	defer func() {
		registryRemove(dir)
		_ = os.RemoveAll(dir)
	}()

	interactive := wizardInteractive()

	fmt.Println("otk wizard — 新手向导")
	fmt.Println("在临时目录中走一遍完整流程，结束后自动清理。")
	if interactive {
		fmt.Println("按提示输入命令，q 退出。")
	}
	fmt.Println()

	oldWd, err := os.Getwd()
	if err != nil {
		fatal("无法获取当前目录: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		fatal("无法进入临时目录: %v", err)
	}
	defer os.Chdir(oldWd)

	if !wizardRun(interactive) {
		fmt.Println("已退出。")
		return
	}
	fmt.Println()
	fmt.Println("otk cd <名称> 可跳到项目库中的项目，otk help 查看全部命令。")
}

func wizardRun(interactive bool) bool {
	if !wizardAsk(interactive,
		"第 1 步 · 建项目\n建好题目目录，进去后初始化（这里叫 demo）：",
		"otk init demo",
		func() { cmdInit([]string{"demo"}) }) {
		return false
	}

	// 替用户放好样例与代码，相当于真实做题时的复制粘贴。
	p := project{Name: "demo", Dir: wizardWd()}
	wizardWriteFile(filepath.Join(p.Dir, "sample1.in"), "1 2\n")
	wizardWriteFile(filepath.Join(p.Dir, "sample1.out"), "3\n")
	wizardWriteFile(p.sourcePath(), wizardSolution)

	if !wizardAsk(interactive,
		"第 2 步 · 样例与代码\n样例就是目录里成对的 .in/.out 文件，已替你放好：\n  sample1.in   1 2\n  sample1.out  3\ndemo.cpp 也写好了一个 A+B 的解。",
		"", nil) {
		return false
	}

	if !wizardAsk(interactive,
		"第 3 步 · 样例入库\n把文件对登记进样例库：",
		"otk refresh", func() { cmdRefresh() }) {
		return false
	}
	if !wizardAsk(interactive,
		"第 4 步 · 看状态\n随时查看项目概况：",
		"otk st", func() { cmdStatus() }) {
		return false
	}
	if !wizardAsk(interactive,
		"第 5 步 · 编译评测\n编译并对全部样例评测：",
		"otk j", func() { cmdJudge() }) {
		return false
	}

	fmt.Println()
	fmt.Println("全部 AC，流程走完了。")
	fmt.Println("真实做题就是这几步：init 建项目、放样例文件对、refresh 入库、写代码、j 评测。")
	return true
}

// wizardAsk 展示步骤说明，等待用户输入期望命令后执行 run。
// expect 为空表示该步无需输入，回车继续即可。返回 false 表示用户退出。
// 非交互模式（stdin 非终端）不等待输入，直接执行，便于管道查看。
func wizardAsk(interactive bool, desc, expect string, run func()) bool {
	fmt.Println(desc)
	if expect == "" {
		if interactive {
			fmt.Print("> 按回车继续 ")
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "q" {
				return false
			}
		}
		fmt.Println()
		return true
	}

	for {
		if interactive {
			fmt.Printf("> %s%s%s  ", cyan, expect, reset)
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return false
			}
			input := strings.TrimSpace(line)
			if input == "q" || input == "quit" {
				return false
			}
			if !wizardMatch(input, expect) {
				fmt.Printf("  请输入 '%s'（或 q 退出）\n", expect)
				continue
			}
		} else {
			fmt.Println("> " + expect)
		}
		run()
		fmt.Println()
		return true
	}
}

// wizardMatch 宽松匹配用户输入与期望命令：otk 前缀可省，别名等价。
func wizardMatch(input, expect string) bool {
	in := strings.Fields(input)
	exp := strings.Fields(expect)
	if len(in) > 0 && in[0] == "otk" {
		in = in[1:]
	}
	if len(exp) > 0 && exp[0] == "otk" {
		exp = exp[1:]
	}
	if len(in) == 0 || len(in) != len(exp) {
		return false
	}
	if resolveCmd(in[0]) != resolveCmd(exp[0]) {
		return false
	}
	return strings.Join(in[1:], " ") == strings.Join(exp[1:], " ")
}

func wizardInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	fi, err = os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func wizardWd() string {
	cwd, _ := os.Getwd()
	return cwd
}

func wizardWriteFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fatal("无法写入 %s: %v", path, err)
	}
}

// wizardSolution 是演示用的 A+B 题解。
const wizardSolution = `#include<iostream>
using namespace std;

int main(){
    long long a, b;
    cin >> a >> b;
    cout << a + b << endl;
    return 0;
}
`
