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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func cmdJudge() {
	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal("未找到 %s", p.Name+".cpp")
	}
	props := p.cfg()

	fmt.Printf("编译 %s ... ", p.Name+".cpp")
	if !compileProject(p.sourcePath(), p.exePath(), props) {
		os.Exit(1)
	}
	fmt.Println(green + "ok" + reset)

	timeLimit := floatProp(props, "time_limit", 1.00)
	memLimit := floatProp(props, "memory_limit", 125.0)

	index := readIndex(p)
	if len(index) == 0 {
		fatal("无样例（放入 .in/.out 后运行 otk refresh）")
	}

	fmt.Printf("评测 %s (%.2fs / %.0fMB)\n", p.Name, timeLimit, memLimit)
	width := 3 // #id 至少占 4 列
	for _, id := range index {
		if len(id) > width {
			width = len(id)
		}
	}
	allAC := true
	for _, id := range index {
		inPath := filepath.Join(p.Dir, id+".in")
		outPath := filepath.Join(p.Dir, id+".out")
		if !fileExists(inPath) || !fileExists(outPath) {
			fmt.Printf("  #%-*s %s缺失%s\n", width, id, yellow, reset)
			allAC = false
			continue
		}

		res := runMeasured(p.exePath(), inPath, p.Dir, timeLimit)
		metrics := fmt.Sprintf("%dms %s", res.wallMS, memStr(res))

		switch {
		case res.timedOut:
			fmt.Printf("  #%-*s %sTLE%s >%.0fms\n", width, id, red, reset, timeLimit*1000)
			allAC = false
		case res.runErr != nil:
			fmt.Printf("  #%-*s %sRE%s   %s\n", width, id, red, reset, metrics)
			allAC = false
			if s := strings.TrimSpace(res.stderr); s != "" {
				printIndented(firstLines(s, 6), "      ")
			}
		case res.memOK && res.memMB > memLimit:
			fmt.Printf("  #%-*s %sMLE%s   %s\n", width, id, red, reset, metrics)
			allAC = false
		default:
			userOut := normalizeOutput(res.stdout)
			stdOut := normalizeOutput(readFile(outPath))
			if userOut == stdOut {
				fmt.Printf("  #%-*s %sAC%s    %s\n", width, id, green, reset, metrics)
			} else {
				fmt.Printf("  #%-*s %sWA%s    %s\n", width, id, red, reset, metrics)
				printDiff(stdOut, userOut)
				allAC = false
			}
		}
	}

	if !allAC {
		os.Exit(1)
	}
}

func cmdRun() {
	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal("未找到 %s", p.Name+".cpp")
	}
	props := p.cfg()

	fmt.Printf("编译 %s ... ", p.Name+".cpp")
	if !compileProject(p.sourcePath(), p.exePath(), props) {
		os.Exit(1)
	}
	fmt.Println(green + "ok" + reset)

	cmd := exec.Command(p.exePath())
	cmd.Dir = p.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("运行失败: %v", err)
	}
}

func cmdDebug() {
	p := requireProject()
	if !fileExists(p.exePath()) {
		fatal("未找到可执行文件，先运行 otk run 或 otk judge")
	}

	cmd := exec.Command("gdb", "--args", p.exePath())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = p.Dir
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s启动 gdb 失败: %v%s\n", red, err, reset)
		fmt.Fprintln(os.Stderr, yellow+"提示: 请确保 gdb 已安装并在 PATH 中"+reset)
		os.Exit(1)
	}
}

func cmdEdit() {
	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal("未找到 %s", p.Name+".cpp")
	}

	editor := os.Getenv("OTK_EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad %s"
		} else {
			editor = "vim %s"
		}
	}

	parts := splitQuoted(fmt.Sprintf(editor, p.sourcePath()))
	if len(parts) == 0 {
		fatal("无效的编辑器命令")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = p.Dir
	if err := cmd.Run(); err != nil {
		fatal("打开编辑器 %s 失败: %v", parts[0], err)
	}
}

// exePathIn 返回可执行文件路径（Windows 下补 .exe 后缀）。
func exePathIn(dir, name string) string {
	exe := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	return exe
}

// compileProject 使用项目配置编译源码，始终附带 -g 以便 gdb 调试。
func compileProject(srcPath, exePath string, props map[string]string) bool {
	args := []string{"-std=" + propsOr(props, "version", "c++17")}
	if propsOr(props, "o2", "1") == "1" {
		args = append(args, "-O2")
	}
	args = append(args, srcPath, "-o", exePath, "-g")

	cmd := exec.Command("g++", args...)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s[CE] 编译失败%s\n", red, reset)
		fmt.Fprint(os.Stderr, errBuf.String())
		return false
	}
	return true
}

// normalizeOutput 统一换行并去除首尾空白，作为比较口径。
func normalizeOutput(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

func memStr(res runResult) string {
	if !res.memOK {
		return "N/A"
	}
	return fmt.Sprintf("%.2fMB", res.memMB)
}

func printDiff(stdOut, userOut string) {
	fmt.Printf("    %s期望%s %s\n", green, reset, strings.ReplaceAll(strings.TrimRight(stdOut, "\n"), "\n", "\n    "))
	fmt.Printf("    %s实际%s %s\n", red, reset, strings.ReplaceAll(strings.TrimRight(userOut, "\n"), "\n", "\n    "))
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = append(lines[:n], "...")
	}
	return strings.Join(lines, "\n")
}

func printIndented(s, indent string) {
	fmt.Println(indent + strings.ReplaceAll(s, "\n", "\n"+indent))
}

// splitQuoted 按空白切分参数，支持单/双引号包裹（用于编辑器命令）。
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
