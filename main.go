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
var otkVersion = "v2.13.6"

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
	"init":       "init",
	"status":     "status",
	"st":         "status",
	"ls":         "ls",
	"groups":     "groups",
	"search":     "search",
	"se":         "search",
	"rm":         "rm",
	"cd":         "cd",
	"refresh":    "refresh",
	"rf":         "refresh",
	"clone":      "clone",
	"submit":     "submit",
	"sub":        "submit",
	"samples":    "samples",
	"config":     "config",
	"cfg":        "config",
	"judge":      "judge",
	"j":          "judge",
	"history":    "history",
	"his":        "history",
	"run":        "run",
	"r":          "run",
	"debug":      "debug",
	"d":          "debug",
	"edit":       "edit",
	"e":          "edit",
	"stress":     "stress",
	"ss":         "stress",
	"pack":       "pack",
	"pk":         "pack",
	"contest":    "contest",
	"ct":         "contest",
	"gui":        "gui",
	"show":       "show",
	"wizard":     "wizard",
	"doctor":     "doctor",
	"completion": "completion",
	"help":       "help",
	"h":          "help",
	"version":    "version",
	"v":          "version",
}

func main() {
	initBaseDir()
	initColors()

	args := os.Args[1:]
	// 以 otk-gui 之名启动时直接进图形界面，忽略命令行参数：
	// 双击快捷键/桌面图标时可能带上文件路径之类的杂项。
	// 这一份二进制编成了 windows 子系统，系统不会给它分配控制台。
	if isGuiBinary() {
		cmdGui()
		return
	}
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
		cmdLs(args[1:]...)
	case "groups":
		cmdGroups(args[1:]...)
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
	case "clone":
		cmdClone(args[1:])
	case "submit":
		cmdSubmit(args[1:])
	case "config":
		cmdConfig(args[1:])
	case "judge":
		cmdJudge(args[1:])
	case "history":
		cmdHistory(args[1:])
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
	case "pack":
		cmdPack(args[1:])
	case "contest":
		cmdContest(args[1:])
	case "gui":
		// 显式敲 otk gui：没控制台也不影响使用，界面正常弹出。
		// 若 stdout 重定向到文件，Fyne 的日志仍会写进去。
		requireNoArgs(args[1:], "gui")
		cmdGui()
	case "show":
		cmdShow(args[1:])
	case "wizard":
		requireNoArgs(args[1:], "wizard")
		cmdWizard()
	case "doctor":
		cmdDoctor(args[1:])
	case "completion":
		cmdCompletion(args[1:])
	case "help":
		printUsage(os.Stdout)
	case "version":
		fmt.Printf("OierToolKit %s\n", otkVersion)
	default:
		fmt.Fprintf(os.Stderr, "%s"+tr("otk: 未知命令 '%s'")+"%s\n", red, args[0], reset)
		fmt.Fprintln(os.Stderr, tr("运行 'otk help' 查看帮助。"))
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
		fatal(tr("otk %s 不接受参数"), cmd)
	}
}

func initBaseDir() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, tr("无法获取用户主目录"))
		otkExit(1)
	}
	baseDir = filepath.Join(home, ".otkprofile")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, tr("无法创建目录 %s: %v")+"\n", baseDir, err)
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
		fatal(tr("用法: otk show w（担保）或 otk show c（许可证）"))
	}
	switch args[0] {
	case "w":
		fmt.Println(tr("OierToolKit 版权所有 (C) 2026 魇珩Gr3yPh4ntom"))
		fmt.Println(tr("本程序不提供任何担保，甚至不保证适用于特定目的。"))
		fmt.Println(tr("详情请参阅 GNU 通用公共许可证。"))
	case "c":
		fmt.Println(tr("这是一个自由软件，欢迎您在 GPLv3 协议下重新分发它。"))
		fmt.Println(tr("在特定条件下，您可以修改并共享它。"))
		fmt.Println(tr("完整许可证全文请查看项目中的 LICENSE 文件：https://github.com/Gr3yPh/OierToolKit/blob/main/LICENSE。"))
	default:
		fatal(tr("未知的 show 参数 '%s'，请输入 'otk show w' 或 'otk show c'"), args[0])
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
			fatal(tr("当前目录不属于任何项目"))
		}
		dir = p.Dir
	case 1:
		dir = resolveRegistry(args[0])
	default:
		fatal(tr("用法: otk cd [名称]"))
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
		fatal(tr("无法启动 shell (%s): %v"), shell, err)
	}
	_ = cmd.Wait() // 用户主动 exit 的退出码无需处理
}

func printUsage(w io.Writer) {
	if uiLangIsEnglish() {
		fmt.Fprintf(w, `Usage: otk <command> [arguments]

Projects:
  otk init [-t template] [name]          Initialize a project here (-t: blank/io/multi/fileio/stress)
  otk status | st                        Show the current project status
  otk ls [group|-]                       List projects, optionally filtered by group or ungrouped (-)
  otk groups ls | add <name> | rm <name> List, add or remove project groups
  otk se <regex>                         Search projects by name, path or label
  otk rm [--soft|--hard] [name]         Remove from the registry, or delete project data
  otk cd [name]                          Open a project in a new shell
  otk pack | pk [directory] [output.zip] Pack source, samples and project config

Samples:
  otk refresh | rf                       Sync sample files
  otk samples                            List samples

Config:
  otk config                             Show the current configuration
  otk config <key> <value>               Change a setting
  otk config name <name>                 Set project name (project folder: empty value resets it)
  otk config group <name|->              Set project group (project folder: - removes the group)
  otk config lang zh|en|auto             Change the CLI and GUI language
  otk config export [zip] [-r] [-t]      Export settings (-r: registry, -t: tokens)
  otk config import <zip> [-f] [-r] [-t] Import settings (-f: overwrite, -r: registry, -t: tokens)

Run:
  otk judge | j [-v] [sample...]         Compile and judge (-v: show RE/WA details)
  otk history | his [-e json|csv] [N]    Show or export evaluation history
  otk run | r                            Compile and run
  otk debug | d                          Debug with gdb
  otk edit | e                           Edit source
  otk stress | ss [N]                    Stress test counterexamples

Remote OJ:
  otk clone [-r remote] [address] <id>   Clone a problem
  otk submit | sub [-q]                  Submit source and wait for the verdict (-q: submit only)

Contests:
  otk contest | ct add <directory> [name] Import a contest (data/ + source/)
  otk contest ls                          List contests
  otk contest judge [name]                Judge a contest and generate standings
  otk contest rank [name] [-w file]       Show or export standings
  otk contest export <name> [output.zip]  Export a contest package
  otk contest dup <name> [new-name]       Duplicate a contest

Misc:
  otk gui                                 Open the graphical interface
  otk wizard                              Interactive setup and workflow guide
  otk doctor                              Diagnose terminal and runtime issues
  otk completion <shell>                  Print completion script (bash/zsh/powershell)
  otk show w|c                            Warranty / license
  otk version                             Show the version
  otk help                                Show this help

OierToolKit %s — 魇珩Gr3yPh4ntom
Copyright (C) 2026 魇珩Gr3yPh4ntom. GPLv3+, no warranty.
https://github.com/Gr3yPh/OierToolKit
Homepage: https://otk.gr3yph4ntom.cn
`, otkVersion)
		return
	}

	fmt.Fprintf(w, `用法: otk <命令> [参数]

项目:
  otk init [-t 模板] [名称]             在当前目录初始化项目（-t 可选 blank/io/multi/fileio/stress）
  otk status | st                        显示当前项目状态
  otk ls [分组名|-]                      按分组列出项目；- 或“未分组”筛选未分组项目
  otk groups ls | add <分组名> | rm <分组名>  列出、新建或删除分组
  otk se <正则>                          按名称、路径或标签搜索项目
  otk rm [--soft|--hard] [名称]          移除项目库记录或删除项目数据
  otk cd [名称]                          在新 shell 中打开项目目录
  otk pack | pk [目录] [输出.zip]        打包源码、样例和项目配置

样例:
  otk refresh | rf                       同步样例库
  otk samples                            列出样例

配置:
  otk config                             查看配置
  otk config <项> <值>                   修改配置
  otk config name <名称>                 修改当前项目名称（空名称恢复目录名）
  otk config group <分组名|->            修改当前项目分组（- 清除分组）
  otk config lang zh|en|auto             切换中英文界面和命令行输出
  otk config export [zip] [-r] [-t]      导出配置（-r 含项目库，-t 含令牌）
  otk config import <zip> [-f] [-r] [-t] 导入配置（-f 覆盖同名项，-r 含项目库，-t 含令牌）

运行:
  otk judge | j [-v] [样例...]           编译并评测（-v 显示 RE/WA 详情）
  otk history | his [-e json|csv] [次数] 查看或导出评测历史
  otk run | r                            编译并运行
  otk debug | d                          使用 gdb 调试
  otk edit | e                           编辑源码
  otk stress | ss [N]                    运行对拍并保存反例

远程 OJ:
  otk clone [-r 远端名] [地址] <题号>    从网站添加题目（协议见 PROTOCOL.md）
  otk submit | sub [-q]                  提交源码并等待 verdict（-q 只提交）

模拟赛:
  otk contest | ct add <目录> [名称]     导入比赛（data/ + source/）
  otk contest ls                         列出已导入的比赛
  otk contest judge [名称]               评测整场比赛并生成榜单
  otk contest rank [名称] [-w 文件]      查看或导出 HTML 榜单
  otk contest export <名称> [输出.zip]   导出比赛数据包
  otk contest dup <名称> [新名称]        复制一场比赛

杂项:
  otk gui                                打开图形界面
  otk wizard                             新手向导，一步步体验工作流
  otk doctor                             诊断终端和运行环境问题
  otk completion <shell>                 输出补全脚本（bash / zsh / powershell）
  otk show w|c                           显示担保或许可证
  otk version                            显示版本
  otk help                               显示帮助

OierToolKit %s — 魇珩Gr3yPh4ntom
Copyright (C) 2026 魇珩Gr3yPh4ntom. 按 GPLv3+ 发布，不提供任何担保。
https://github.com/Gr3yPh/OierToolKit
主页: https://otk.gr3yph4ntom.cn
`, otkVersion)
}
