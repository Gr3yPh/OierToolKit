package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(RED + "无法获取用户主目录" + RESET)
		return
	}
	baseDir = filepath.Join(home, "otk")
	currentDir = baseDir

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("%s无法创建根目录 ~/otk/: %v%s\n", RED, err, RESET)
		return
	}
	if runtime.GOOS=="windows" {
		runningWindows=true;
	}
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
	startUpMes := `
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
#      \ |    |___|/               \|___|    |_____|/  |____|/                #
#       \|____|                                                 OierToolKit   #
#                                            v1.0 Go Edition by Gr3yPh4ntom   #
###############################################################################`
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

		tokens := strings.Fields(line)
		cmd := strings.ToLower(tokens[0])

		switch cmd {
		case "c", "create": //
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: c [PROJECT]" + RESET)
			} else {
				createProject(tokens[1])
			}
		case "s", "switch": //
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: s [PROJECT]" + RESET)
			} else {
				switchProject(tokens[1])
			}
		case "d", "delete": //
			if len(tokens) < 2 {
				fmt.Println(YELLOW + "用法: d [PROJECT]" + RESET)
			} else {
				deleteProject(tokens[1])
			}
		case "l","list": //
			listInfo()
		case "set": //
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				handleSetCommand(tokens)
			}
		case "ne": //
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				createSample(reader)
			}
		case "r", "run": //
			if currentProject == "" {
				fmt.Println(RED + "请先进入一个项目！" + RESET)
			} else {
				runTest()
			}
		case "author": //
			printAuthor()
		case "h", "help": //
			printHelp()
		case "q", "exit", "quit": //
			fmt.Println(GREEN + "Goodbye, Oier!" + RESET)
			return
		case "show": //
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
		default:
			fmt.Printf("未知命令: %s。输入 'h' 查看帮助。\n", cmd)
		}
	}
}

func printAuthor() { //
	fmt.Println("OierToolKit v1.0 by 魇珩Gr3yPh4ntom\n" +
		"（严格来讲应该叫OierOperationSimplifier才对吧。。）\n" +
		"repo: https://github.com/Gr3yPh/OierToolKit\nblog: https://gr3yph4ntom.cn")
}

func printHelp() { //
	fmt.Println("可用命令列表:")
	fmt.Println("  c, create [PROJECT]  - 新建一个题目项目")
	fmt.Println("  s, switch [PROJECT]  - 切换题目项目 (s ~ 返回根目录)")
	fmt.Println("  d, delete [PROJECT]  - 删除指定项目")
	fmt.Println("  l, list              - 列出项目、配置与样例")
	fmt.Println("  set [key] [value]    - 设置当前项目的配置 (支持 time / memory / version / o2)")
	fmt.Println("  ne                   - 新建测试样例")
	fmt.Println("  r, run               - 编译并评测当前项目 (高精度时间/内存/Diff)")
	fmt.Println("  cmd [SYS_CMD]        - 直接执行 Linux 系统命令 (e.g., cmd ls -la)")
	fmt.Println("  author               - 查看作者信息")
	fmt.Println("  h, help              - 显示帮助")
	fmt.Println("  show w               - 显示软件免责声明 (Warranty)")
	fmt.Println("  show c               - 显示软件复制与分发条件 (Copying/License)")
	fmt.Println("  q, exit              - 退出")
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

func createProject(proj string) { //
	projDir := filepath.Join(baseDir, proj)
	if _, err := os.Stat(projDir); err == nil {
		fmt.Printf("项目 %s 已存在，正在为您切换。\n", proj)
		switchProject(proj)
		return
	}

	_ = os.MkdirAll(projDir, 0755)
	cppCode := "#include<iostream>\nusing namespace std;\n\nint main(){\n    // Write code here\n    return 0;\n}\n"
	_ = os.WriteFile(filepath.Join(projDir, proj+".cpp"), []byte(cppCode), 0644)

	iniContent := "time_limit=1.00\nmemory_limit=125\nversion=c++17\no2=1"
	_ = os.WriteFile(filepath.Join(projDir, proj+".ini"), []byte(iniContent), 0644)

	fmt.Println(GREEN + "成功创建项目: " + proj + RESET)
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
	if fi, err := os.Stat(projDir); err == nil && fi.IsDir() {
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

func listInfo() { //
	if currentProject == "" {
		fmt.Println("=== 现有项目列表 ===")
		files, err := os.ReadDir(baseDir)
		if err != nil {
			fmt.Println(RED + "无法读取目录: " + err.Error() + RESET)
			return
		}
		count := 0
		for _, f := range files {
			if f.IsDir() {
				fmt.Println("  * " + f.Name())
				count++
			}
		}
		if count == 0 {
			fmt.Println("（暂无项目，使用 create [PROJECT] 新建一个吧）")
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
			fmt.Println("  （无可用样例，使用 'ne' 录入）")
		} else {
			for _, f := range inFiles {
				id := f[:strings.LastIndex(f, ".")]
				fmt.Println("  样例 #" + id)
			}
		}
	}
}

func handleSetCommand(tokens []string) { //
	if len(tokens) < 3 {
		fmt.Println(YELLOW + "用法: set time/memory/version/o2 [VALUE]" + RESET)
		return
	}
	key := strings.ToLower(tokens[1])
	value := tokens[2]
	iniPath := filepath.Join(currentDir, currentProject+".ini")
	props := readIni(iniPath)

	switch key {
	case "time":
		props["time_limit"] = value
	case "memory", "mem":
		props["memory_limit"] = value
	case "version", "ver":
		props["version"] = value
	case "o2":
		props["o2"] = value
	default:
		fmt.Println(YELLOW + "未知配置项: " + key + RESET)
		return
	}
	writeIni(iniPath, props)
}

func createSample(reader *bufio.Reader) { //
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

func readUntilEOF(reader *bufio.Reader) string {
	var sb strings.Builder
	for {
		fmt.Print(" > ")
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

func runTest() { //
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

	// 读取时空限制
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
		fmt.Println(YELLOW + "提示: 未找到测试样例。请先使用 'ne' 创建。" + RESET)
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
		cmdRun := exec.Command("time", "-f", "%e %M", "-o", timeTmpPath, exePath)
		
		// 重定向文件输入
		fIn, _ := os.Open(inFile)
		cmdRun.Stdin = fIn

		var userOutBuf strings.Builder
		cmdRun.Stdout = &userOutBuf

		startNano := time.Now()
		if err := cmdRun.Start(); err != nil {
			fmt.Println(RED + "RE (System Error)" + RESET)
			fIn.Close()
			continue
		}

		// 处理 TLE 计时器
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

		// 解析时间与内存
		runTimeSec := endNano.Sub(startNano).Seconds()
		runMemMB := 0.0

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

		runTimeMs := int64(runTimeSec * 1000)
		metricsStr := fmt.Sprintf(" (%dms, %.2fMB)", runTimeMs, runMemMB)

		if runMemMB > memLimit {
			fmt.Println(RED + "MLE" + RESET + metricsStr)
			continue
		}

		// 标准 Diff 对比（格式化换行符与末尾空格）
		userOut := strings.TrimSpace(strings.ReplaceAll(userOutBuf.String(), "\r\n", "\n"))
		stdBytes, _ := os.ReadFile(outFile)
		stdOut := strings.TrimSpace(strings.ReplaceAll(string(stdBytes), "\r\n", "\n"))

		if userOut == stdOut {
			fmt.Println(GREEN + "AC" + RESET + metricsStr)
		} else {
			fmt.Println(RED + "WA" + RESET + metricsStr)
			fmt.Println("    -----------------------------------------")
			fmt.Printf("    %s[标准输出]%s\n    %s\n", GREEN, RESET, strings.ReplaceAll(stdOut, "\n", "\n    "))
			fmt.Printf("    %s[你的输出]%s\n    %s\n", RED, RESET, strings.ReplaceAll(userOut, "\n", "\n    "))
			fmt.Println("    -----------------------------------------")
		}
	}
}
