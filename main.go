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

// OierToolKit 提供 git 风格的子命令界面，例如：
//
//	otk init
//	otk refresh
//	otk judge
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// otkVersion 用 var 而非 const，便于 release workflow 用 -ldflags "-X" 覆盖。
var otkVersion = "v2.2.2"

var (
	baseDir string

	// ansiOK 表示控制台能正确解释 ANSI 转义序列。Windows 旧式控制台
	// (conhost) 需显式开启 VT 处理，失败时关闭颜色并让向导降级。
	ansiOK bool

	// 颜色仅在 stdout 为终端时启用，保证重定向输出纯净。
	reset  = ""
	red    = ""
	green  = ""
	yellow = ""
	cyan   = ""
)

// cmdAlias 提供短别名，减少输入（如 otk j = otk judge）。
var cmdAlias = map[string]string{
	"init":    "init",
	"status":  "status",
	"st":      "status",
	"ls":      "ls",
	"search":  "search",
	"se":      "search",
	"rm":      "rm",
	"cd":      "cd",
	"refresh": "refresh",
	"rf":      "refresh",
	"fetch":   "fetch",
	"f":       "fetch",
	"submit":  "submit",
	"sub":     "submit",
	"samples": "samples",
	"config":  "config",
	"cfg":     "config",
	"judge":   "judge",
	"j":       "judge",
	"run":     "run",
	"r":       "run",
	"debug":   "debug",
	"d":       "debug",
	"edit":    "edit",
	"e":       "edit",
	"stress":  "stress",
	"ss":      "stress",
	"show":    "show",
	"wizard":  "wizard",
	"doctor":  "doctor",
	"help":    "help",
	"h":       "help",
	"version": "version",
	"v":       "version",
}

func main() {
	initBaseDir()
	initColors()

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage(os.Stderr)
		otkExit(1)
	}

	switch resolveCmd(args[0]) {
	case "init":
		cmdInit(args[1:])
	case "status":
		requireNoArgs(args[1:], "status")
		cmdStatus()
	case "ls":
		requireNoArgs(args[1:], "ls")
		cmdLs()
	case "search":
		cmdSearch(args[1:])
	case "rm":
		cmdRm(args[1:])
	case "cd":
		cmdCd(args[1:])
	case "refresh":
		requireNoArgs(args[1:], "refresh")
		cmdRefresh()
	case "samples":
		requireNoArgs(args[1:], "samples")
		cmdSamples()
	case "fetch":
		cmdFetch(args[1:])
	case "submit":
		cmdSubmit(args[1:])
	case "config":
		cmdConfig(args[1:])
	case "judge":
		cmdJudge(args[1:])
	case "run":
		requireNoArgs(args[1:], "run")
		cmdRun()
	case "debug":
		requireNoArgs(args[1:], "debug")
		cmdDebug()
	case "edit":
		requireNoArgs(args[1:], "edit")
		cmdEdit()
	case "stress":
		cmdStress(args[1:])
	case "show":
		cmdShow(args[1:])
	case "wizard":
		requireNoArgs(args[1:], "wizard")
		cmdWizard()
	case "doctor":
		cmdDoctor(args[1:])
	case "help":
		printUsage(os.Stdout)
	case "version":
		fmt.Printf("OierToolKit %s\n", otkVersion)
	default:
		fmt.Fprintf(os.Stderr, "%sotk: 未知命令 '%s'%s\n", red, args[0], reset)
		fmt.Fprintln(os.Stderr, "运行 'otk help' 查看帮助。")
		otkExit(1)
	}
}

func resolveCmd(name string) string {
	if cmd, ok := cmdAlias[name]; ok {
		return cmd
	}
	return name
}

// fatal 打印错误到 stderr 并以状态码 1 退出。
func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, red+format+reset+"\n", a...)
	otkExit(1)
}

func requireNoArgs(args []string, cmd string) {
	if len(args) > 0 {
		fatal("otk %s 不接受参数", cmd)
	}
}

func initBaseDir() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法获取用户主目录")
		otkExit(1)
	}
	baseDir = filepath.Join(home, ".otkprofile")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "无法创建目录 %s: %v\n", baseDir, err)
		otkExit(1)
	}
}

func initColors() {
	fi, err := os.Stdout.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return
	}
	ansiOK = enableANSI()
	if !ansiOK {
		// 老式控制台开不了 VT：不打颜色码，宁可没颜色也不要乱码。
		return
	}
	reset = "\u001B[0m"
	red = "\u001B[31m"
	green = "\u001B[32m"
	yellow = "\u001B[33m"
	cyan = "\u001B[36m"
}

func cmdShow(args []string) {
	if len(args) != 1 {
		fatal("用法: otk show w（担保）或 otk show c（许可证）")
	}
	switch args[0] {
	case "w":
		fmt.Println("OierToolKit 版权所有 (C) 2026 魇珩Gr3yPh4ntom")
		fmt.Println("本程序不提供任何担保，甚至不保证适用于特定目的。")
		fmt.Println("详情请参阅 GNU 通用公共许可证。")
	case "c":
		fmt.Println("这是一个自由软件，欢迎您在 GPLv3 协议下重新分发它。")
		fmt.Println("在特定条件下，您可以修改并共享它。")
		fmt.Println("完整许可证全文请查看项目中的 LICENSE 文件：https://github.com/Gr3yPh/OierToolKit/blob/main/LICENSE。")
	default:
		fatal("未知的 show 参数 '%s'，请输入 'otk show w' 或 'otk show c'", args[0])
	}
}

// cmdCd 在项目目录中启动一个新 shell，exit 后回到原目录。
// 子进程无法改变父 shell 的工作目录，这是免配置直接跳转的实现方式。
func cmdCd(args []string) {
	var dir string
	switch len(args) {
	case 0:
		p, ok := findProject()
		if !ok {
			fatal("当前目录不属于任何项目")
		}
		dir = p.Dir
	case 1:
		dir = resolveRegistry(args[0])
	default:
		fatal("用法: otk cd [名称]")
	}

	shell := os.Getenv("SHELL")
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
	} else if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatal("无法启动 shell (%s): %v", shell, err)
	}
	_ = cmd.Wait() // 用户主动 exit 的退出码无需处理
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `用法: otk <命令> [参数...]

项目:
  otk init [名称]        在当前目录初始化项目
  otk status | st        当前项目状态
  otk ls                 列出全部项目
  otk se <正则>          搜索项目
  otk rm [--soft|--hard] [名称]  移除项目
  otk cd [名称]          在新 shell 中打开项目目录

样例:
  otk refresh | rf       同步样例库
  otk samples            列出样例

配置:
  otk config             查看配置
  otk config <项> <值>   修改配置

运行:
  otk judge | j [-v]     编译并评测（-v 显示报错与 WA 详情）
  otk run | r            编译并运行
  otk debug | d          gdb 调试
  otk edit | e           编辑源码
  otk stress | ss [N]    对拍

远程 OJ:
  otk fetch [url] <id>   拉取题目与样例入库（协议见 PROTOCOL.md）
  otk submit | sub [-q]  提交源码评测并等待结果（-q 只提交）

杂项:
  otk wizard             新手向导，完整体验一遍工作流
  otk doctor             诊断终端环境（显示异常时运行并反馈输出）
  otk show w|c           担保 / 许可证
  otk version            版本
  otk help               帮助

OierToolKit `+otkVersion+` — 魇珩Gr3yPh4ntom
Copyright (C) 2026 魇珩Gr3yPh4ntom. 按 GPLv3+ 发布，不提供任何担保。
https://github.com/Gr3yPh/OierToolKit
主页: https://otk.gr3yph4ntom.cn
`)
}
