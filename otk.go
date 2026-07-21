package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"runtime"
	"github.com/chzyer/readline"
)

const (
	RESET  = "\u001B[0m"
	RED    = "\u001B[31m"
	GREEN  = "\u001B[32m"
	YELLOW = "\u001B[33m"
	BLUE   = "\u001B[34m"
	CYAN   = "\u001B[36m"
)

var (
	baseDir        string
	currentDir     string
	currentProject string
	runningWindows bool
	otkVersion     = "v1.6.1"
	otkEditor      string
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(RED + "无法获取用户主目录" + RESET)
		return
	}
	baseDir = filepath.Join(home, ".otk")
	currentDir = baseDir

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("%s无法创建根目录 ~/.otk/: %v%s\n", RED, err, RESET)
		return
	}
	if runtime.GOOS=="windows" {
		runningWindows=true;
	}
	loadOtkRc()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "",
		HistoryFile:       filepath.Join(baseDir, ".otk_history"), 
		//AutoComplete:      completer, 
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		fmt.Println(RED + "无法初始化命令行: " + err.Error() + RESET)
		return
	}
	defer rl.Close()
	startUpMes := fmt.Sprintf(`
###############################################################################
#            ____       ________    ________  ______   _______                #
#        ____\_  \__   /        \  /        \|\     \  \      \               #
#       /     /     \ |\         \/         /|\     \  |     /|               #
#      /     /\      || \            /\____/ | \|     |/     //               #
#     |     |  |     ||  \______/\   \     | |  |     |_____//                #
#     |     |  |     | \ |      | \   \____|/   |     |\     \                #
#     |     | /     /|  \|______|  \   \       /     /|\|     |               #
#     |\     \_____/ |           \  \___\     /_____/ |/_____/|               #
#     | \_____\   | /             \ |   |    |     | / |    | |               #
#      \ |    |___|/               \|___|    |_____|/  |____/|                #
#       \|____|                                                 OierToolKit   #
#                                          %s Go Edition by Gr3yPh4ntom   #
###############################################################################
                    https://github.com/Gr3yPh/OierToolKit`, otkVersion)
	fmt.Println(CYAN + startUpMes + RESET)
	fmt.Println(`本程序不提供任何担保；详情请输入“show w”。
这是自由软件，欢迎您重新分发。
在特定条件下；输入“show c”查看详情。`)
	fmt.Println("输入 'h' 查看帮助。输入 'cmd [命令]' 执行外部系统命令。\n")

	reader := bufio.NewReader(os.Stdin)
	for {
		promptProject := "~"
		if currentProject != "" {
			promptProject = currentProject
		}
		//fmt.Printf("%s[otk @ %s]$ %s", BLUE, promptProject, RESET)
		rl.SetPrompt(fmt.Sprintf("%s[otk @ %s]$ %s", BLUE, promptProject, RESET))

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			} else {
				fmt.Println(GREEN + "\nGoodbye, Oier!" + RESET)
				break
			}
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "cmd ") {
			executeSystemCommand(strings.TrimSpace(line[4:]))
			continue
		}

		tokens := splitQuoted(line)
		cmd := strings.ToLower(tokens[0])

		switch cmd {
		case "p", "project":
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: p [n/d/s/l/se] ..." + RESET)
				continue
			}
			sub := strings.ToLower(tokens[1])
			switch sub {
			case "n", "new":
				if len(tokens) < 3 {
					fmt.Println(YELLOW + "用法: p n [PROJECT] [TAG]" + RESET)
				} else {
					tag := ""
					if len(tokens) >= 4 {
						tag = tokens[3]
					}
					createProject(tokens[2], tag)
				}
			case "d", "delete":
				if len(tokens) < 3 {
					fmt.Println(YELLOW + "用法: p d [PROJECT]" + RESET)
				} else {
					deleteProject(tokens[2])
				}
			case "s", "switch":
				if len(tokens) < 3 {
					fmt.Println(YELLOW + "用法: p s [PROJECT]" + RESET)
				} else {
					switchProject(tokens[2])
				}
			case "l", "list":
				listInfo(true)
			case "se", "search":
				if len(tokens) < 3 {
					fmt.Println(YELLOW + "用法: p se(arch) [REGEX]" + RESET)
				} else {
					searchProjects(tokens[2])
				}
			default:
				fmt.Printf("未知的 p 子命令: %s\n", sub)
			}

		case "e": // example management
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
				continue
			}
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: e [n/v/c/l] ..." + RESET)
				continue
			}
			sub := strings.ToLower(tokens[1])
			switch sub {
			case "n", "new":
				// e new  OR e new [INPUT FILE] [ANSWER FILE]
				if len(tokens) == 2 {
					createSample(reader)
				} else if len(tokens) >= 4 {
					inputF := tokens[2]
					ansF := tokens[3]
					createSampleFromFiles(inputF, ansF)
				} else {
					fmt.Println(YELLOW + "用法: e new 或 e new [INPUT FILE] [ANSWER FILE]" + RESET)
				}
			case "v", "view":
				if len(tokens) < 3 {
					fmt.Println(YELLOW + "用法: e v [EXAMPLE_ID]" + RESET)
				} else {
					viewSample(tokens[2])
				}
			case "c", "clear":
				clearSamples()
			case "l", "list":
				listSamples()
			default:
				fmt.Printf("未知的 e 子命令: %s\n", sub)
			}

		case "c", "config":
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
				continue
			}
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: c l(ist) - 列出当前配置" + RESET)
				fmt.Println(YELLOW + "      c s(et) [ITEM] [VALUE] - 设置配置项" + RESET)
				fmt.Println(YELLOW + "      配置项: time, memory, version, o2" + RESET)
				continue
			}
			sub := strings.ToLower(tokens[1])
			switch sub {
			case "l", "list":
				showConfig()
			case "s", "set":
				if len(tokens) < 4 {
					fmt.Println(YELLOW + "用法: c s(et) [ITEM] [VALUE]" + RESET)
					fmt.Println(YELLOW + "      配置项: time, memory, version, o2" + RESET)
				} else {
					setConfig(tokens[2], tokens[3])
				}
			default:
				fmt.Printf("未知的 c 子命令: %s\n", sub)
			}

		case "j", "judge":
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				runTest()
			}

		case "r", "run":
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				runOnly()
			}

		case "d", "debug":
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				debugCurrent()
			}

		case "cmd":
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: cmd [系统命令]" + RESET)
			} else {
				executeSystemCommand(strings.Join(tokens[1:], " "))
			}

		case "ed", "edit":
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				editCurrent()
			}

		case "h", "help":
			printHelp()

		case "q", "exit", "quit":
			fmt.Println(GREEN + "Goodbye, Oier!" + RESET)
			return

		case "show":
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: show w (免责声明) 或 show c (许可证)" + RESET)
			} else {
				subCmd := strings.ToLower(tokens[1])
				if subCmd == "w" {
					fmt.Println("OierToolKit 版权所有 (C) 2026 魇珩Gr3yPh4ntom")
					fmt.Println("本程序不提供任何担保，甚至不保证适用于特定目的。")
					fmt.Println("详情请参阅 GNU 通用公共许可证。")
				} else if subCmd == "c" {
					fmt.Println("这是一个自由软件，欢迎您在 GPLv3 协议下重新分发它。")
					fmt.Println("在特定条件下，您可以修改并共享它。")
					fmt.Println("完整许可证全文请查看根目录下的 LICENSE 文件。")
				} else {
					fmt.Printf("%s错误: 未知的 show 参数 '%s'，请输入 'show w' 或 'show c'%s\n", RED, tokens[1], RESET)
				}
			}

	case "cl", "clear":
		fmt.Print("\u001B[2J\u001B[H")

	case "st", "stress-test":
		count := 100
		if len(tokens) >= 2 {
			if c, err := strconv.Atoi(tokens[1]); err == nil && c > 0 {
				count = c
			}
		}
		stressTest(count)

		default:
			fmt.Printf("未知命令: %s。输入 'h' 查看帮助。\n", cmd)
		}
	}
}

func splitQuoted(s string) []string {
	var args []string
	var buf strings.Builder
	inSingle := false
	inDouble := false

	for _, r := range s {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == ' ' && !inSingle && !inDouble:
			if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

func printHelp() { //
	fmt.Println("可用命令列表:")
	fmt.Println("  项目管理:")
	fmt.Println("    p n(ew) [PROJECT] [TAG] - 创建项目 (可指定标签)")
	fmt.Println("    p d(elete) [PROJECT]  - 删除项目")
	fmt.Println("    p s(witch) [PROJECT]  - 切换项目 (p s ~ 返回根目录)")
	fmt.Println("    p l(ist)              - 列出项目 (带标签高亮)")
	fmt.Println("    p se(arch) [REGEX]    - 按名称或标签搜索项目")
	fmt.Println("  样例管理:")
	fmt.Println("    e n(ew)             - 交互式新建样例")
	fmt.Println("    e n(ew) [IN] [OUT]  - 从文件新建样例")
	fmt.Println("    e v(iew) [ID]       - 查看指定ID的样例文件")
	fmt.Println("    e c(lear)           - 删除所有样例")
	fmt.Println("    e l(ist)            - 列出样例")
	fmt.Println("  项目配置:")
	fmt.Println("    c l(ist)            - 列出当前项目的所有配置")
	fmt.Println("    c s(et) [ITEM] [VAL]- 设置配置项")
	fmt.Println("      配置项: time(时间限制), memory(内存限制), version(C++版本), o2(是否启用O2优化)")
	fmt.Println("  运行与调试:")
	fmt.Println("    j(udge)            - 编译并评测 (遍历样例进行评测)")
	fmt.Println("    r(un)              - 仅编译并运行程序 (交互模式)")
	fmt.Println("    d(ebug)            - 使用 gdb 调试当前可执行文件")
	fmt.Println("    cmd [SYS_CMD]      - 执行系统命令")
	fmt.Println("    ed(it)             - 用 vim 编辑当前源码文件")
	fmt.Println("    st(ress-test) [N]  - 对拍 N 轮 (默认100轮)，需要 gen.cpp brute.cpp")
	fmt.Println("  杂项:")
	fmt.Println("    cl, clear          - 清屏")
	fmt.Println("    h, help            - 帮助信息")
	fmt.Println("    show w             - 查看担保 (Warranty)")
	fmt.Println("    show c             - 查看版权/许可证信息 (Copying)")
	fmt.Println("    q, exit            - 退出")
}

func loadOtkRc() {
	home, err := os.UserHomeDir()
	rcPath := filepath.Join(home,".otkrc")
	bytes, err := os.ReadFile(rcPath)
	if err != nil {
		return // no config file, use defaults
	}
	lines := strings.Split(string(bytes), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "otk.editor":
			otkEditor = value
		}
	}
}

func executeSystemCommand(sysCmd string) { //
	if sysCmd == "" {
		fmt.Println(YELLOW + "用法: cmd [系统命令]" + RESET)
		return
	}
	
	var cmd *exec.Cmd
	if runningWindows {
		// Windows用cmd
		cmd = exec.Command("cmd.exe", "/c", sysCmd)
	} else {
		cmd = exec.Command("bash", "-c", sysCmd)
	}
	
	cmd.Dir = currentDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s执行命令失败: %v%s\n", RED, err, RESET)
	}
}

func createProject(proj, tag string) { //
	projDir := filepath.Join(baseDir, proj)
	if _, err := os.Stat(projDir); err == nil {
		if tag != "" {
			iniPath := filepath.Join(projDir, proj+".ini")
			props := readIni(iniPath)
			props["tag"] = tag
			writeIni(iniPath, props)
			fmt.Printf("%s已为项目 %s 添加标签: %s%s\n", GREEN, proj, tag, RESET)
		} else {
			fmt.Printf("项目 %s 已存在，正在为您切换。\n", proj)
		}
		switchProject(proj)
		return
	}

	_ = os.MkdirAll(projDir, 0755)
	cppCode := "#include<iostream>\nusing namespace std;\n\nint main(){\n    \n    return 0;\n}\n"
	_ = os.WriteFile(filepath.Join(projDir, proj+".cpp"), []byte(cppCode), 0644)

	props := map[string]string{
		"time_limit":   "1.00",
		"memory_limit": "125",
		"version":      "c++17",
		"o2":           "1",
	}
	if tag != "" {
		props["tag"] = tag
	}
	writeIni(filepath.Join(projDir, proj+".ini"), props)

	fmt.Println(GREEN + "成功创建项目: " + proj + RESET)
	if tag != "" {
		fmt.Printf("  标签: %s%s%s\n", CYAN, tag, RESET)
	}
	switchProject(proj)
}

func switchProject(proj string) { //
	if proj == "~" {
		currentDir = baseDir
		currentProject = ""
		fmt.Println("已返回根目录")
		return
	}
	projDir := filepath.Join(baseDir, proj)
	if fi, err := os.Stat(projDir); err == nil && fi.IsDir() && proj!="." && proj!=".." {
		currentDir = projDir
		currentProject = proj
		fmt.Println("已切换至: " + proj)
	} else {
		fmt.Println(RED + "项目 " + proj + " 不存在！" + RESET)
	}
}

func deleteProject(proj string) { //
	projDir := filepath.Join(baseDir, proj)
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		fmt.Println(RED + "项目不存在！" + RESET)
		return
	}
	_ = os.RemoveAll(projDir)
	fmt.Println(GREEN + "已删除项目: " + proj + RESET)
	if currentProject == proj {
		currentDir = baseDir
		currentProject = ""
	}
}

func searchProjects(pattern string) {
	fmt.Printf("=== 搜索项目 (模式: %s) ===\n", pattern)

	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Printf("%s错误: 正则表达式无效 - %v%s\n", RED, err, RESET)
		return
	}

	files, err := os.ReadDir(baseDir)
	if err != nil {
		fmt.Println(RED + "无法读取目录: " + err.Error() + RESET)
		return
	}

	count := 0
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		projName := f.Name()
		tag := ""
		iniPath := filepath.Join(baseDir, projName, projName+".ini")
		if props := readIni(iniPath); props["tag"] != "" {
			tag = props["tag"]
		}

		if re.MatchString(projName) || (tag != "" && re.MatchString(tag)) {
			if tag != "" {
				fmt.Printf("  * %s %s[%s]%s\n", projName, CYAN, tag, RESET)
			} else {
				fmt.Println("  * " + projName)
			}
			count++
		}
	}

	if count == 0 {
		fmt.Println("未找到匹配的项目。")
	} else {
		fmt.Printf("找到 %d 个匹配的项目。\n", count)
	}
}

func readIni(path string) map[string]string {
	props := make(map[string]string)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return props
	}
	lines := strings.Split(string(bytes), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ";") {
			continue
		}
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			props[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return props
}

func writeIni(path string, props map[string]string) {
	var lines []string
	lines = append(lines, "# Updated by otk (Go version)")
	for k, v := range props {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func listInfo(listProject bool) { //
	if  listProject {
		fmt.Println("=== 现有项目列表 ===")
		files, err := os.ReadDir(baseDir)
		if err != nil {
			fmt.Println(RED + "无法读取目录: " + err.Error() + RESET)
			return
		}
		count := 0
		for _, f := range files {
			if f.IsDir() {
				projName := f.Name()
				tag := ""
				iniPath := filepath.Join(baseDir, projName, projName+".ini")
				if props := readIni(iniPath); props["tag"] != "" {
					tag = props["tag"]
				}
				if tag != "" {
					fmt.Printf("  * %s %s[%s]%s\n", projName, CYAN, tag, RESET)
				} else {
					fmt.Println("  * " + projName)
				}
				count++
			}
		}
		if count == 0 {
			fmt.Println("（暂无项目，使用 p n [PROJECT] 新建一个吧）")
		}
	} else {
		fmt.Printf("=== 项目 [%s] 的详细状态 ===\n", currentProject)
		iniPath := filepath.Join(currentDir, currentProject+".ini")
		props := readIni(iniPath)
		fmt.Println("可用配置项:")
		getTime := props["time_limit"]
		if getTime == "" { getTime = "1.00" }
		getMem := props["memory_limit"]
		if getMem == "" { getMem = "125" }
		getVer := props["version"]
		if getVer == "" { getVer = "c++17" }
		getO2 := props["o2"]
		if getO2 == "" { getO2 = "1" }

		fmt.Printf("  time_limit   : %ss\n", getTime)
		fmt.Printf("  memory_limit : %sMB\n", getMem)
		fmt.Printf("  version      : %s\n", getVer)
		fmt.Printf("  o2           : %s\n", getO2)

		files, _ := os.ReadDir(currentDir)
		var inFiles []string
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".in") {
				inFiles = append(inFiles, f.Name())
			}
		}
		sort.Strings(inFiles)
		fmt.Println("已录入样例:")
		if len(inFiles) == 0 {
			fmt.Println("  （无可用样例，使用 'e n' 录入）")
		} else {
			for _, f := range inFiles {
				id := f[:strings.LastIndex(f, ".")]
				fmt.Println("  样例 #" + id)
			}
		}
	}
}

// ========== 新增配置管理函数 ==========

func showConfig() {
	if currentProject == "" {
		fmt.Println(RED + "请先进入一个项目！" + RESET)
		return
	}
	iniPath := filepath.Join(currentDir, currentProject+".ini")
	props := readIni(iniPath)
	
	fmt.Printf("=== 项目 [%s] 配置 ===\n", currentProject)
	
	timeLimit := props["time_limit"]
	if timeLimit == "" { timeLimit = "1.00" }
	memoryLimit := props["memory_limit"]
	if memoryLimit == "" { memoryLimit = "125" }
	version := props["version"]
	if version == "" { version = "c++17" }
	o2 := props["o2"]
	if o2 == "" { o2 = "1" }
	
	fmt.Printf("  %-12s: %s (秒)\n", "time", timeLimit)
	fmt.Printf("  %-12s: %s (MB)\n", "memory", memoryLimit)
	fmt.Printf("  %-12s: %s\n", "version", version)
	fmt.Printf("  %-12s: %s (0=关闭, 1=开启)\n", "o2", o2)
	fmt.Println("  使用 'c s [ITEM] [VALUE]' 修改配置")
}

func setConfig(item, value string) {
	if currentProject == "" {
		fmt.Println(RED + "请先进入一个项目！" + RESET)
		return
	}
	
	item = strings.ToLower(item)
	iniPath := filepath.Join(currentDir, currentProject+".ini")
	props := readIni(iniPath)
	
	// 验证配置项和值
	switch item {
	case "time":
		// 验证是否为有效数字
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			fmt.Printf("%s错误: time 必须是数字 (如: 1.00)%s\n", RED, RESET)
			return
		}
		props["time_limit"] = value
		fmt.Printf("%s已设置 time = %s 秒%s\n", GREEN, value, RESET)
		
	case "memory", "mem":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			fmt.Printf("%s错误: memory 必须是数字 (如: 125)%s\n", RED, RESET)
			return
		}
		props["memory_limit"] = value
		fmt.Printf("%s已设置 memory = %s MB%s\n", GREEN, value, RESET)
		
	case "version", "ver":
		// 与时俱进(^^;)
		validVersions := []string{"c++98", "c++11", "c++14", "c++17", "c++20", "c++23", "c++26"}
		valid := false
		for _, v := range validVersions {
			if value == v {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Printf("%s警告: '%s' 不是标准C++版本，但会尝试使用%s\n", YELLOW, value, RESET)
		}
		props["version"] = value
		fmt.Printf("%s已设置 version = %s%s\n", GREEN, value, RESET)
		
	case "o2":
		val := strings.ToLower(value)
		if val != "0" && val != "1" && val != "on" && val != "off" && val != "true" && val != "false" {
			fmt.Printf("%s错误: o2 必须是 0/1 或 on/off%s\n", RED, RESET)
			return
		}
		// 统一为 0 或 1
		if val == "on" || val == "true" {
			val = "1"
		} else if val == "off" || val == "false" {
			val = "0"
		}
		props["o2"] = val
		fmt.Printf("%s已设置 o2 = %s%s\n", GREEN, val, RESET)
		
	default:
		fmt.Printf("%s错误: 未知配置项 '%s'%s\n", RED, item, RESET)
		fmt.Println("  可用配置项: time, memory, version, o2")
		return
	}
	
	writeIni(iniPath, props)
}


func createSample(reader *bufio.Reader) { 
	id := 1
	for {
		inPath := filepath.Join(currentDir, fmt.Sprintf("%d.in", id))
		if _, err := os.Stat(inPath); os.IsNotExist(err) {
			break
		}
		id++
	}

	fmt.Println("请输入样例输入 (输入完成后换行，并输入EOF提交):")
	inputData := readUntilEOF(reader)

	fmt.Println("请输入样例输出 (输入完成后换行，并输入EOF提交):")
	inputDataOut := readUntilEOF(reader)

	_ = os.WriteFile(filepath.Join(currentDir, fmt.Sprintf("%d.in", id)), []byte(inputData), 0644)
	_ = os.WriteFile(filepath.Join(currentDir, fmt.Sprintf("%d.out", id)), []byte(inputDataOut), 0644)
	fmt.Printf("%s成功添加样例 #%d%s\n", GREEN, id, RESET)
}

func createSampleFromFiles(inputPath, answerPath string) {
	id := 1
	for {
		inPath := filepath.Join(currentDir, fmt.Sprintf("%d.in", id))
		if _, err := os.Stat(inPath); os.IsNotExist(err) {
			break
		}
		id++
	}

	inBytes, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("%s无法读取输入文件: %v%s\n", RED, err, RESET)
		return
	}
	outBytes, err := os.ReadFile(answerPath)
	if err != nil {
		fmt.Printf("%s无法读取答案文件: %v%s\n", RED, err, RESET)
		return
	}
	_ = os.WriteFile(filepath.Join(currentDir, fmt.Sprintf("%d.in", id)), inBytes, 0644)
	_ = os.WriteFile(filepath.Join(currentDir, fmt.Sprintf("%d.out", id)), outBytes, 0644)
	fmt.Printf("%s成功从文件添加样例 #%d%s\n", GREEN, id, RESET)
}

func viewSample(id string) {
	inFile := filepath.Join(currentDir, id+".in")
	outFile := filepath.Join(currentDir, id+".out")
	if _, err := os.Stat(inFile); os.IsNotExist(err) {
		fmt.Println(YELLOW + "未找到指定样例输入文件" + RESET)
		return
	}
	inB, _ := os.ReadFile(inFile)
	fmt.Printf("---- 样例 #%s 输入 ----\n%s\n", id, string(inB))
	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		fmt.Println(YELLOW + "未找到指定样例输出文件" + RESET)
		return
	}
	outB, _ := os.ReadFile(outFile)
	fmt.Printf("---- 样例 #%s 输出 ----\n%s\n", id, string(outB))
}

func clearSamples() {
	files, _ := os.ReadDir(currentDir)
	count := 0
	for _, f := range files {
		if !f.IsDir() && (strings.HasSuffix(f.Name(), ".in") || strings.HasSuffix(f.Name(), ".out")) {
			_ = os.Remove(filepath.Join(currentDir, f.Name()))
			count++
		}
	}
	fmt.Printf("%s已删除 %d 个样例相关文件%s\n", GREEN, count, RESET)
}

func listSamples() {
	files, _ := os.ReadDir(currentDir)
	var inFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".in") {
			inFiles = append(inFiles, f.Name())
		}
	}
	sort.Strings(inFiles)
	if len(inFiles) == 0 {
		fmt.Println("（无可用样例，使用 'e n' 录入）")
		return
	}
	fmt.Println("已录入样例:")
	for _, f := range inFiles {
		id := f[:strings.LastIndex(f, ".")]
		fmt.Println("  样例 #" + id)
	}
}

func readUntilEOF(reader *bufio.Reader) string {
	var sb strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				sb.WriteString(line) 
				break
			}
			return ""
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func runTest() {
	cppName := currentProject + ".cpp"
	cppPath := filepath.Join(currentDir, cppName)
	exePath := filepath.Join(currentDir, currentProject)
	if runningWindows {
		exePath+=".exe"
	}
	timeTmpPath := filepath.Join(currentDir, ".time.tmp")

	if _, err := os.Stat(cppPath); os.IsNotExist(err) {
		fmt.Printf("%s错误: 未找到源码文件 %s%s\n", RED, cppName, RESET)
		return
	}

	fmt.Printf("正在编译 %s... ", cppName)
	iniPath := filepath.Join(currentDir, currentProject+".ini")
	props := readIni(iniPath)

	cppVersion := props["version"]
	if cppVersion == "" { cppVersion = "c++17" }
	o2Switch := props["o2"]
	if o2Switch == "" { o2Switch = "1" }

	var compileArgs []string
	compileArgs = append(compileArgs, "-std="+cppVersion)
	if o2Switch == "1" {
		compileArgs = append(compileArgs, "-O2")
	}
	compileArgs = append(compileArgs, cppPath, "-o", exePath)

	cmdCompile := exec.Command("g++", compileArgs...)
	var errBuf strings.Builder
	cmdCompile.Stderr = &errBuf

	if err := cmdCompile.Run(); err != nil {
		fmt.Println(RED + "[ CE ] 编译失败！" + RESET)
		fmt.Println(RED + errBuf.String() + RESET)
		return
	}
	fmt.Println(GREEN + "编译成功" + RESET)

	timeLimit := 1.00
	if tl, err := strconv.ParseFloat(props["time_limit"], 64); err == nil {
		timeLimit = tl
	}
	memLimit := 125.0
	if ml, err := strconv.ParseFloat(props["memory_limit"], 64); err == nil {
		memLimit = ml
	}

	files, _ := os.ReadDir(currentDir)
	var inFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".in") {
			inFiles = append(inFiles, f.Name())
		}
	}
	if len(inFiles) == 0 {
		fmt.Println(YELLOW + "提示: 未找到测试样例。请先使用 'e n' 创建。" + RESET)
		return
	}
	sort.Strings(inFiles)
	fmt.Printf("开始评测 (限制: %.2fs / %.0fMB):\n", timeLimit, memLimit)

	for _, name := range inFiles {
		id := name[:strings.LastIndex(name, ".")]
		outFile := filepath.Join(currentDir, id+".out")
		inFile := filepath.Join(currentDir, name)

		fmt.Printf("  样例 #%s : ", id)
		if _, err := os.Stat(outFile); os.IsNotExist(err) {
			fmt.Println(YELLOW + "SKIP (缺少对应的 .out 文件)" + RESET)
			continue
		}

		// 调用 Linux time 工具压榨进程开销
		var cmdRun *exec.Cmd
		if runningWindows {
			// Windows 使用 -ExecutionPolicy Bypass 绕过策略限制，或直接使用 measure-command
			cmdRun = exec.Command("powershell", "-Command", 
				fmt.Sprintf("Measure-Command { %s | Set-Content .time.tmp }", exePath))
		} else {
			cmdRun = exec.Command("time", "-f", "%e %M", "-o", timeTmpPath, exePath)
		}
		
		// 重定向文件输入
		fIn, _ := os.Open(inFile)
		cmdRun.Stdin = fIn

		var userOutBuf strings.Builder
		cmdRun.Stdout = &userOutBuf
		if !runningWindows {
			cmdRun.Stderr = &userOutBuf
		}

		startNano := time.Now()
		if err := cmdRun.Start(); err != nil {
			fmt.Println(RED + "RE (System Error)" + RESET)
			fIn.Close()
			continue
		}

		done := make(chan error, 1)
		go func() { done <- cmdRun.Wait() }()

		var exited bool
		var runErr error
		select {
		case runErr = <-done:
			exited = true
		case <-time.After(time.Duration(timeLimit * float64(time.Second))):
			_ = cmdRun.Process.Kill()
			exited = false
		}
		endNano := time.Now()
		fIn.Close()

		if !exited {
			fmt.Printf("%sTLE%s (>%.0fms)\n", RED, RESET, timeLimit*1000)
			continue
		}

		if runErr != nil {
			fmt.Printf("%sRE%s (Runtime Error)\n", RED, RESET)
			continue
		}

		runTimeSec := endNano.Sub(startNano).Seconds()
		runMemMB := 0.0

		if runningWindows {
			// Windows PowerShell Measure-Command 输出解析
			if tBytes, err := os.ReadFile(".time.tmp"); err == nil {
				output := string(tBytes)
				// 提取 TotalSeconds
				lines := strings.Split(output, "\n")
				for _, line := range lines {
					if strings.Contains(line, "TotalSeconds") {
						parts := strings.Split(line, ":")
						if len(parts) >= 2 {
							if val, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
								runTimeSec = val
							}
						}
					}
				}
			}
			_ = os.Remove(".time.tmp")
		} else {
			if tBytes, err := os.ReadFile(timeTmpPath); err == nil {
				tParts := strings.Fields(string(tBytes))
				if len(tParts) >= 2 {
					if rts, err := strconv.ParseFloat(tParts[0], 64); err == nil {
						runTimeSec = rts
					}
					if rmm, err := strconv.ParseFloat(tParts[1], 64); err == nil {
						runMemMB = rmm / 1024.0
					}
				}
			}
			_ = os.Remove(timeTmpPath)
		}

		runTimeMs := int64(runTimeSec * 1000)
		metricsStr := fmt.Sprintf(" (%dms, %.2fMB)", runTimeMs, runMemMB)

		if runMemMB > memLimit { 
			fmt.Println(RED + "MLE" + RESET + metricsStr)
			continue
		}

		userOut := strings.TrimSpace(strings.ReplaceAll(userOutBuf.String(), "\r\n", "\n"))
		stdBytes, _ := os.ReadFile(outFile)
		stdOut := strings.TrimSpace(strings.ReplaceAll(string(stdBytes), "\r\n", "\n"))

		if userOut == stdOut {
			fmt.Println(GREEN + "AC" + RESET + metricsStr)
		} else {
			fmt.Println(RED + "WA" + RESET + metricsStr)
			fmt.Println("    -----------------------------------------")
			fmt.Printf("    %s[样例输出]%s\n    %s\n", GREEN, RESET, strings.ReplaceAll(stdOut, "\n", "\n    "))
			fmt.Printf("    %s[你的输出]%s\n    %s\n", RED, RESET, strings.ReplaceAll(userOut, "\n", "\n    "))
			fmt.Println("    -----------------------------------------")
		}
	}
}

func runOnly() {
	cppName := currentProject + ".cpp"
	cppPath := filepath.Join(currentDir, cppName)
	exePath := filepath.Join(currentDir, currentProject)
	if runningWindows {
		exePath+=".exe"
	}

	if _, err := os.Stat(cppPath); os.IsNotExist(err) {
		fmt.Printf("%s错误: 未找到源码文件 %s%s\n", RED, cppName, RESET)
		return
	}

	fmt.Printf("正在编译 %s... ", cppName)
	iniPath := filepath.Join(currentDir, currentProject+".ini")
	props := readIni(iniPath)

	cppVersion := props["version"]
	if cppVersion == "" { cppVersion = "c++17" }
	o2Switch := props["o2"]
	if o2Switch == "" { o2Switch = "1" }

	var compileArgs []string
	compileArgs = append(compileArgs, "-std="+cppVersion)
	if o2Switch == "1" {
		compileArgs = append(compileArgs, "-O2")
	}
	compileArgs = append(compileArgs, cppPath, "-o", exePath)

	cmdCompile := exec.Command("g++", compileArgs...)
	var errBuf strings.Builder
	cmdCompile.Stderr = &errBuf

	if err := cmdCompile.Run(); err != nil {
		fmt.Println(RED + "[ CE ] 编译失败！" + RESET)
		fmt.Println(RED + errBuf.String() + RESET)
		return
	}
	fmt.Println(GREEN + "编译成功" + RESET)

	var cmdRun *exec.Cmd
	if runningWindows {
		cmdRun = exec.Command(exePath)
	} else {
		cmdRun = exec.Command(exePath)
	}
	cmdRun.Dir = currentDir
	cmdRun.Stdin = os.Stdin
	cmdRun.Stdout = os.Stdout
	cmdRun.Stderr = os.Stderr
	if err := cmdRun.Run(); err != nil {
		fmt.Printf("%s运行失败: %v%s\n", RED, err, RESET)
	}
}

func debugCurrent() {
	exePath := filepath.Join(currentDir, currentProject)
	if runningWindows {
		exePath += ".exe"
	}
	
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		fmt.Println(YELLOW + "未找到可执行文件，请先运行 r 或 j 以编译程序" + RESET)
		return
	}
	
	var cmd *exec.Cmd
	if runningWindows {
		cmd = exec.Command("gdb", exePath)
	} else {
		cmd = exec.Command("gdb", "--args", exePath)
	}
	
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = currentDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s启动 gdb 失败: %v%s\n", RED, err, RESET)
		fmt.Println(YELLOW + "提示: 请确保 gdb 已安装并在 PATH 中" + RESET)
	}
}

func editCurrent() {
	src := filepath.Join(currentDir, currentProject+".cpp")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		fmt.Println(YELLOW + "未找到源码文件，无法编辑" + RESET)
		return
	}

	editor := otkEditor
	if editor == "" {
		if runningWindows {
			editor = "notepad.exe"
		} else {
			editor = "vim"
		}
	}

	cmd := exec.Command(editor, src)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = currentDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s打开编辑器 %s 失败: %v%s\n", RED, editor, err, RESET)
	}
}


// ========== 对拍功能 (stress-test) ==========

func compileCpp(srcPath, exePath, version, o2 string) bool {
	args := []string{"-std=" + version}
	if o2 == "1" {
		args = append(args, "-O2")
	}
	args = append(args, srcPath, "-o", exePath)
	cmd := exec.Command("g++", args...)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		fmt.Println(RED + "[ CE ] 编译失败！" + RESET)
		fmt.Println(RED + errBuf.String() + RESET)
		return false
	}
	fmt.Println(GREEN + "编译成功" + RESET)
	return true
}

func execCmdCapture(exePath, inputStr, workDir string) (string, error) {
	cmd := exec.Command(exePath)
	cmd.Dir = workDir
	if inputStr != "" {
		cmd.Stdin = strings.NewReader(inputStr)
	}
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	err := cmd.Run()
	return outBuf.String(), err
}

func stressTest(count int) {
	if currentProject == "" {
		fmt.Println(RED + "请先进入一个项目！" + RESET)
		return
	}

	projName := currentProject
	projCpp := filepath.Join(currentDir, projName+".cpp")
	genCpp := filepath.Join(currentDir, "gen.cpp")
	bruteCpp := filepath.Join(currentDir, "brute.cpp")

	// 检查文件是否存在
	missing := false
	for _, pair := range [][2]string{{genCpp, "gen.cpp"}, {bruteCpp, "brute.cpp"}, {projCpp, projName + ".cpp"}} {
		if _, err := os.Stat(pair[0]); os.IsNotExist(err) {
			fmt.Printf("%s错误: 未找到 %s%s\n", RED, pair[1], RESET)
			missing = true
		}
	}
	if missing {
		return
	}

	// 读取项目配置
	iniPath := filepath.Join(currentDir, projName+".ini")
	props := readIni(iniPath)
	cppVersion := props["version"]
	if cppVersion == "" {
		cppVersion = "c++17"
	}
	o2Switch := props["o2"]
	if o2Switch == "" {
		o2Switch = "1"
	}

	// 编译 gen.cpp
	fmt.Print("正在编译 gen.cpp... ")
	genExe := filepath.Join(currentDir, "gen")
	if runningWindows {
		genExe += ".exe"
	}
	if !compileCpp(genCpp, genExe, "c++17", "0") {
		return
	}

	// 编译 brute.cpp
	fmt.Print("正在编译 brute.cpp... ")
	bruteExe := filepath.Join(currentDir, "brute")
	if runningWindows {
		bruteExe += ".exe"
	}
	if !compileCpp(bruteCpp, bruteExe, "c++17", "0") {
		return
	}

	// 编译项目代码
	fmt.Printf("正在编译 %s... ", projName+".cpp")
	projExe := filepath.Join(currentDir, projName)
	if runningWindows {
		projExe += ".exe"
	}
	if !compileCpp(projCpp, projExe, cppVersion, o2Switch) {
		return
	}

	// 创建对拍输出目录
	stressDir := filepath.Join(currentDir, ".stress")
	os.MkdirAll(stressDir, 0755)

	fmt.Printf("开始对拍, 共 %d 轮...\n", count)
	passed := 0
	for i := 1; i <= count; i++ {
		// 运行 gen 产生输入数据
		genOut, err := execCmdCapture(genExe, "", currentDir)
		if err != nil {
			fmt.Printf("\n%s第 %d 轮: gen 运行时错误 (RE)%s\n", RED, i, RESET)
			return
		}
		inputData := genOut

		// 运行 brute 获取正确输出
		bruteOut, err := execCmdCapture(bruteExe, inputData, currentDir)
		if err != nil {
			fmt.Printf("\n%s第 %d 轮: brute 运行时错误 (RE)%s\n", RED, i, RESET)
			return
		}
		expectedOut := strings.TrimSpace(strings.ReplaceAll(bruteOut, "\r\n", "\n"))

		// 运行项目代码获取待检验输出
		projOut, err := execCmdCapture(projExe, inputData, currentDir)
		if err != nil {
			fmt.Printf("\n%s第 %d 轮: 项目程序运行时错误 (RE)%s\n", RED, i, RESET)
			return
		}
		userOut := strings.TrimSpace(strings.ReplaceAll(projOut, "\r\n", "\n"))

		// 比较
		if userOut == expectedOut {
			passed++
			fmt.Printf("\r  第 %d/%d 轮通过", i, count)
		} else {
			fmt.Printf("\n%s第 %d 轮: 答案不正确 (WA)%s\n", RED, i, RESET)
			// 保存测试数据
			inFile := filepath.Join(stressDir, fmt.Sprintf("%d.in", i))
			ansFile := filepath.Join(stressDir, fmt.Sprintf("%d.ans", i))
			outFile := filepath.Join(stressDir, fmt.Sprintf("%d.out", i))
			os.WriteFile(inFile, []byte(inputData), 0644)
			os.WriteFile(ansFile, []byte(expectedOut), 0644)
			os.WriteFile(outFile, []byte(userOut), 0644)
			fmt.Printf("  已保存测试数据至 %s\n", stressDir)
			fmt.Println("  -----------------------------------------")
			fmt.Printf("  %s[正确输出]%s\n  %s\n", GREEN, RESET, expectedOut)
			fmt.Printf("  %s[你的输出]%s\n  %s\n", RED, RESET, userOut)
			fmt.Println("  -----------------------------------------")
		}
	}
	fmt.Println()
	if passed == count {
		fmt.Printf("%s全部 %d 轮通过！%s\n", GREEN, count, RESET)
	} else {
		fmt.Printf("%s%d/%d 轮通过, %d 轮失败%s\n", YELLOW, passed, count, count-passed, RESET)
	}
}
