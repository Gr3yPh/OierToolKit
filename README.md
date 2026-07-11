# OierToolKit (otk)

一个为 Oier 量身定制的轻量级跨平台本地评测姬与题目管理工具。
~~叫做OierOperationSimplifier更合理吧。。~~

![Demo Image](demo.png)

## 🌟 特性
* **清爽控制界面**：内嵌交互式 Shell，赏心悦目的彩色输出。
* **时空精准监控**：借由 GNU time 抓取高精度运行时间（ms）与最大常驻内存（MB）。
* **智能 Diff 引擎**：WA（Wrong Answer）时自动对比标准输出与你的输出差异。
* **命令穿透**：通过 `cmd` 快捷执行 Linux 系统命令。
* 总之就是很流畅啦～

## 🚀 安装
1. clone本项目并确保系统安装了GNU工具集并包含 `g++` 与 `time` 命令。
2. 运行 `go build -o otk` 并执行可执行文件
3. 也可以从release页面下载预编译二进制

## ⏩ 快速开始
```bash
###################func executeSystemCommand(sysCmd string) { //
	if sysCmd == "" {
		fmt.Println(YELLOW + "用法: cmd [系统命令]" + RESET)
		return
	}
	
	var cmd *exec.Cmd
	if runningWindows {
		// Windows: 使用 cmd.exe /c
		cmd = exec.Command("cmd.exe", "/c", sysCmd)
	} else {
		// Linux/Mac: 使用 bash -c
		cmd = exec.Command("bash", "-c", sysCmd)
	}
	
	cmd.Dir = currentDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s执行命令失败: %v%s\n", RED, err, RESET)
	}
}############################################################
#            ____       ________    ________  ______   _______                #
#        ____\_  \__   /        \  /        \|\     \  \      \               #
#       /     /     \ |\         \/         /|\     \  |     /|               #
#      /     /\      || \            /\____/ | \|     |/     //               #
#     |     |  |     ||  \______/\   \     | |  |     |_____//                #
#     |     |  |     | \ |      | \   \____|/   |     |\     \                #
#     |     | /     /|  \|______|  \   \       /     /|\|     |               #
#     |\     \_____/ |           \  \___\     /_____/ |/_____/|               #
#     | \_____\   | /             \ |   |    |     | / |    | |               #
#      \ |    |___|/               \|___|    |_____|/  |____|/                #
#       \|____|                                                 OierToolKit   #
#                                            v1.0 Go Edition by Gr3yPh4ntom   #
###############################################################################
本程序不提供任何担保；详情请输入“show w”。
这是自由软件，欢迎您重新分发。
在特定条件下；输入“show c”查看详情。
输入 'h' 查看帮助。输入 'cmd [命令]' 执行外部系统命令。

[otk @ ~]$ c test
成功创建项目: test
已切换至: test
[otk @ test]$ ne
请输入样例输入 (输入完成后换行，并输入EOF提交):
 > 2 8
 > 请输入样例输出 (输入完成后换行，并输入EOF提交):
 > 10
 > 成功添加样例 #1
[otk @ test]$ ne
请输入样例输入 (输入完成后换行，并输入EOF提交):
 > 26 1
 > 请输入样例输出 (输入完成后换行，并输入EOF提交):
 > 27
 > 成功添加样例 #2
[otk @ test]$ r
正在编译 test.cpp... 编译成功
开始评测 (限制: 1.00s / 125MB):
  样例 #1 : AC (10ms, 4.06MB)
  样例 #2 : AC (0ms, 4.21MB)
[otk @ test]$
```

**(C)opyright 2026 魇珩Gr3yPh4ntom. All rights reserved.**

本工具依据 **GNU General Public License v3.0 (GPLv3)** 开源协议免费分发与修改，详情参见仓库下LICENSE文件。
