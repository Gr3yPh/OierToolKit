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
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// wizardLive 在交互模式下开启打字机效果，管道输出保持瞬时。
var wizardLive bool

// cmdWizard 是交互式新手向导：以对话方式引导用户亲手输入命令，
// 走完 init → refresh → judge 流程，结束后自动清理。
func cmdWizard() {
	if _, err := exec.LookPath("g++"); err != nil {
		fatal("未找到 g++，wizard 需要编译 C++ 代码")
	}

	wizardLive = wizardInteractive()

	// 父目录放临时垃圾，demo 子目录才是项目根（目录名即代码文件名）。
	root, err := os.MkdirTemp("", "otk-wizard-")
	if err != nil {
		fatal("无法创建临时目录: %v", err)
	}
	dir := filepath.Join(root, "demo")
	if err := os.Mkdir(dir, 0755); err != nil {
		fatal("无法创建演示目录: %v", err)
	}
	cleanup := func() {
		registryRemove(dir)
		_ = os.RemoveAll(root)
	}
	defer cleanup()

	// Ctrl+C 中断时同样清理演示现场。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		fmt.Println()
		cleanup()
		fmt.Println("演示现场已清理。")
		otkExit(130)
	}()

	oldWd, err := os.Getwd()
	if err != nil {
		fatal("无法获取当前目录: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		fatal("无法进入临时目录: %v", err)
	}
	defer os.Chdir(oldWd)

	if !wizardRun() {
		fmt.Println()
		wizardSay("已退出向导。")
		return
	}
	fmt.Println()
	wizardSay("otk cd <名称> 可跳到项目库中的项目，otk help 查看全部命令。")
	wizardSay("如果觉得好用，一定要返回来给个 star！ https://github.com/Gr3yPh/OierToolKit")
}

func wizardRun() bool {
	wizardSay("你好，我是 otk 向导。带你花两分钟，亲手走一遍完整流程。")

	wizardStep("第 1 步 · 建项目")
	wizardSay("做题先要有个放代码和样例的目录，题目目录我已经替你建好了。")
	wizardSay("接下来每一步的命令都要你自己亲手输。先初始化：")
	if _, ok := wizardInput("otk init demo"); !ok {
		return false
	}
	cmdInit([]string{"demo"})
	wizardSay("很好，项目已经登记进项目库了。")

	wizardStep("第 2 步 · 样例与代码")
	wizardSay("样例就是目录里的文件对：.in 配 .out 或 .ans。")
	wizardSay("我替你放好一组，还写了一个 A+B 的解：")
	wizardSay("  sample1.in   1 2")
	wizardSay("  sample1.out  3")
	p := project{Name: "demo", Dir: wizardWd()}
	wizardWriteFile(filepath.Join(p.Dir, "sample1.in"), "1 2\n")
	wizardWriteFile(filepath.Join(p.Dir, "sample1.out"), "3\n")
	wizardWriteFile(p.sourcePath(), wizardSolution)
	wizardSay("确认放好了，按回车继续：")
	if _, ok := wizardInput(""); !ok {
		return false
	}

	wizardStep("第 3 步 · 样例入库")
	wizardSay("文件放好了，还要登记进样例库，评测才知道有哪些数据：")
	if _, ok := wizardInput("otk refresh"); !ok {
		return false
	}
	cmdRefresh()
	wizardSay("入库完成，样例库里现在有一组数据。")

	wizardStep("第 4 步 · 看状态")
	wizardSay("评测之前，先检查项目状态：")
	if _, ok := wizardInput("otk st"); !ok {
		return false
	}
	cmdStatus()
	wizardSay("时限、内存、编译选项、样例数，都在这一行里。")

	wizardStep("第 5 步 · 编译评测")
	wizardSay("重头戏来了：编译并评测。这是你以后每天都要敲的命令：")
	if _, ok := wizardInput("otk j"); !ok {
		return false
	}
	cmdJudge(nil)

	wizardSay("恭喜你 AC 了这道题！")
	wizardSay("回顾一下：init 建项目、refresh 样例入库、j 评测。")
	wizardSay("还有很多有用的功能没有在这里展示，可以运行 otk help 查看完整的帮助文件 。")
	wizardSay("向导流程完毕，赶紧给我做题去")
	return true
}

// wizardSay 以向导口吻输出一句台词，交互模式下逐字打出。
func wizardSay(text string) {
	if !wizardLive {
		fmt.Println(text)
		return
	}
	fmt.Print(cyan + "otk ›" + reset + " ")
	for _, r := range text {
		fmt.Print(string(r))
		time.Sleep(runeDelay(r))
	}
	fmt.Println()
	time.Sleep(220 * time.Millisecond)
}

// runeDelay 返回逐字打印时单个字符后的停顿：汉字慢、标点再顿一下。
func runeDelay(r rune) time.Duration {
	switch {
	case r == '。' || r == '！' || r == '？' || r == '：':
		return 160 * time.Millisecond
	case r == '，' || r == '、' || r == '；':
		return 110 * time.Millisecond
	case unicode.Is(unicode.Han, r):
		return 25 * time.Millisecond
	default:
		return 10 * time.Millisecond
	}
}

// wizardStep 打印步骤标题，前后留出呼吸感。
func wizardStep(title string) {
	if wizardLive {
		time.Sleep(400 * time.Millisecond)
	}
	fmt.Println()
	fmt.Println(yellow + "▌ " + title + reset)
}

// wizardInput 展示期望命令后等待用户亲手敲入，提示符明确标出输入位置。
// 返回 false 表示用户退出（q / EOF）。expect 为空表示按回车继续。
func wizardInput(expect string) (string, bool) {
	if !wizardLive {
		if expect != "" {
			fmt.Println("> " + expect)
		}
		return expect, true
	}
	wizardHint(expect)
	prompt := green + "请输入 > " + reset
	if expect == "" {
		prompt = green + "按回车继续 > " + reset
	}
	for {
		fmt.Print(prompt)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", false
		}
		input := strings.TrimSpace(line)
		if input == "q" || input == "quit" {
			return "", false
		}
		if expect == "" || input == "" || wizardMatch(input, expect) {
			time.Sleep(300 * time.Millisecond)
			return input, true
		}
		fmt.Printf("  嗯？再输入一次 '%s'（或 q 退出）\n", expect)
	}
}

// wizardHint 逐字打出期望输入的命令，像向导在演示；下一行的提示符才是输入位置。
func wizardHint(expect string) {
	if expect == "" {
		return
	}
	fmt.Print("  ")
	for _, r := range expect {
		fmt.Print(string(r))
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Println()
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
	// 交互与否只看 stdin/stdout 是否为终端；ANSI 可用性只影响颜色，
	// 不能让它决定向导是否等待输入。
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
