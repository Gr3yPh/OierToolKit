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
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

func cmdStress(args []string) {
	if len(args) > 1 {
		fatal("用法: otk stress [轮数]")
	}
	count := 100
	if len(args) == 1 {
		c, err := strconv.Atoi(args[0])
		if err != nil || c <= 0 {
			fatal("轮数必须是正整数")
		}
		count = c
	}

	p := requireProject()
	projCpp := p.sourcePath()
	genCpp := filepath.Join(p.Dir, "gen.cpp")
	bruteCpp := filepath.Join(p.Dir, "brute.cpp")

	for _, pair := range [][2]string{{projCpp, p.codeName() + ".cpp"}, {genCpp, "gen.cpp"}, {bruteCpp, "brute.cpp"}} {
		if !fileExists(pair[0]) {
			fatal("未找到 %s", pair[1])
		}
	}

	props := p.cfg()
	noOpt := map[string]string{"version": "c++17", "o2": "0"}

	fmt.Print("编译 gen.cpp ... ")
	genExe := exePathIn(p.Dir, "gen")
	if !compileProject(genCpp, genExe, noOpt) {
		otkExit(1)
	}
	fmt.Println(green + "ok" + reset)

	fmt.Print("编译 brute.cpp ... ")
	bruteExe := exePathIn(p.Dir, "brute")
	if !compileProject(bruteCpp, bruteExe, noOpt) {
		otkExit(1)
	}
	fmt.Println(green + "ok" + reset)

	fmt.Printf("编译 %s ... ", p.codeName()+".cpp")
	projExe := p.exePath()
	if !compileProject(projCpp, projExe, props) {
		otkExit(1)
	}
	fmt.Println(green + "ok" + reset)

	stressDir := filepath.Join(p.Dir, ".stress")
	_ = os.MkdirAll(stressDir, 0755)

	// 对拍期间 Ctrl+C 用于优雅中断并清理临时可执行文件。
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	fmt.Printf("对拍 %d 轮... (Ctrl+C 中断)\n", count)
	passed := 0

	for i := 1; i <= count; i++ {
		select {
		case <-sigChan:
			fmt.Println("\n" + yellow + "对拍被中断" + reset)
			cleanupStressFiles(genExe, bruteExe, projExe)
			otkExit(130)
		default:
		}

		inputData, err := execCmdCapture(genExe, "", p.Dir)
		if err != nil {
			cleanupStressFiles(genExe, bruteExe, projExe)
			fatal("第 %d 轮: gen 运行时错误 (RE)", i)
		}

		bruteOut, err := execCmdCapture(bruteExe, inputData, p.Dir)
		if err != nil {
			cleanupStressFiles(genExe, bruteExe, projExe)
			fatal("第 %d 轮: brute 运行时错误 (RE)", i)
		}
		expectedOut := normalizeOutput(bruteOut)

		projOut, err := execCmdCapture(projExe, inputData, p.Dir)
		if err != nil {
			cleanupStressFiles(genExe, bruteExe, projExe)
			fatal("第 %d 轮: 项目程序运行时错误 (RE)", i)
		}
		userOut := normalizeOutput(projOut)

		if userOut != expectedOut {
			fmt.Printf("%s第 %d 轮 WA%s\n", red, i, reset)
			inFile := filepath.Join(stressDir, fmt.Sprintf("%d.in", i))
			ansFile := filepath.Join(stressDir, fmt.Sprintf("%d.ans", i))
			outFile := filepath.Join(stressDir, fmt.Sprintf("%d.out", i))
			_ = os.WriteFile(inFile, []byte(inputData), 0644)
			_ = os.WriteFile(ansFile, []byte(expectedOut), 0644)
			_ = os.WriteFile(outFile, []byte(userOut), 0644)
			fmt.Printf("  已保存测试数据至 %s\n", stressDir)
			printDiff(expectedOut, userOut)
			cleanupStressFiles(genExe, bruteExe, projExe)
			otkExit(1)
		}
		passed++
		fmt.Printf("\r  %d/%d", i, count)
	}
	fmt.Println()

	cleanupStressFiles(genExe, bruteExe, projExe)
	if passed == count {
		fmt.Printf("%s全部通过 (%d/%d)%s\n", green, count, count, reset)
	} else {
		fmt.Printf("%s%d/%d 通过%s\n", yellow, passed, count, reset)
		otkExit(1)
	}
}

// execCmdCapture 以 inputStr 为标准输入运行程序，返回其标准输出。
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

func cleanupStressFiles(files ...string) {
	for _, f := range files {
		if f != "" {
			_ = os.Remove(f)
		}
	}
}
