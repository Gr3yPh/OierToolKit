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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 对拍各阶段事件，CLI 与 GUI 共用同一份进度上报。
const (
	stressCompile = "compile" // 编译 gen / brute / 项目源码
	stressStart   = "start"   // 编译完成，对拍开始
	stressRound   = "round"   // 进入第 N 轮
)

// stressEvent 是一次进度回调：Stage 说明阶段，Name 是源文件名，Round/Total
// 是轮次进度，Done 表示累计已编译数量。
type stressEvent struct {
	Stage string
	Name  string
	Done  int
	Round int
	Total int
}

// stressResult 是对拍的可判定结果（不含错误）：Stopped 表示被中断，
// FailRound 为 0 表示全部通过，否则为发现反例的轮次。
type stressResult struct {
	Total     int
	Passed    int
	Stopped   bool
	FailRound int
	Input     string
	Expected  string
	Actual    string
	StressDir string
}

// compileFailure 表示编译阶段失败，Log 是编译器输出。
type compileFailure struct {
	Name string
	Log  string
}

func (e compileFailure) Error() string {
	return fmt.Sprintf("%s 编译失败 (CE)", e.Name)
}

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

	// 对拍期间 Ctrl+C 用于优雅中断并清理临时可执行文件。
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	res, err := runStress(p, count, func(ev stressEvent) {
		switch ev.Stage {
		case stressCompile:
			fmt.Printf("编译 %s ... %sok%s\n", ev.Name, green, reset)
		case stressStart:
			fmt.Printf("对拍 %d 轮... (Ctrl+C 中断)\n", ev.Total)
		case stressRound:
			fmt.Printf("\r  %d/%d", ev.Round, ev.Total)
		}
	}, func() bool {
		select {
		case <-sigChan:
			return true
		default:
			return false
		}
	})

	if err != nil {
		var cf compileFailure
		if errors.As(err, &cf) {
			if s := strings.TrimSpace(cf.Log); s != "" {
				fmt.Println(s)
			}
			otkExit(1)
		}
		fatal("%v", err)
	}

	fmt.Println()
	switch {
	case res.Stopped:
		fmt.Println(yellow + "对拍被中断" + reset)
		otkExit(130)
	case res.FailRound > 0:
		fmt.Printf("%s第 %d 轮 WA%s\n", red, res.FailRound, reset)
		fmt.Printf("  已保存测试数据至 %s\n", res.StressDir)
		printDiff(res.Expected, res.Actual)
		otkExit(1)
	default:
		fmt.Printf("%s全部通过 (%d/%d)%s\n", green, res.Total, res.Total, reset)
	}
}

// runStress 编译 gen/brute/项目源码并逐轮对拍；不打印、不退出，进度经 emit
// 上报，stopped 返回 true 时在当前轮结束后中断。
func runStress(p project, count int, emit func(stressEvent), stopped func() bool) (stressResult, error) {
	res := stressResult{Total: count, StressDir: filepath.Join(p.Dir, ".stress")}
	if emit == nil {
		emit = func(stressEvent) {}
	}
	if stopped == nil {
		stopped = func() bool { return false }
	}

	props := p.cfg()
	projFIO := fileIOFromProps(props, p.codeName()) // 文件读写题：仅项目程序走文件，gen/brute 仍用标准输入输出
	noOpt := map[string]string{"version": "c++17", "o2": "0"}

	genCpp := filepath.Join(p.Dir, "gen.cpp")
	bruteCpp := filepath.Join(p.Dir, "brute.cpp")
	genExe := exePathIn(p.Dir, "gen")
	bruteExe := exePathIn(p.Dir, "brute")
	projExe := p.exePath()
	defer cleanupStressFiles(genExe, bruteExe, projExe)

	tasks := []struct {
		name string
		file string
		exe  string
		opt  map[string]string
	}{
		{"gen.cpp", genCpp, genExe, noOpt},
		{"brute.cpp", bruteCpp, bruteExe, noOpt},
		{p.codeName() + ".cpp", p.sourcePath(), projExe, props},
	}
	for i, t := range tasks {
		emit(stressEvent{Stage: stressCompile, Name: t.name, Done: i})
		log, ok, aborted := compileProjectStopped(t.file, t.exe, t.opt, stopped)
		if aborted {
			res.Stopped = true
			return res, nil
		}
		if !ok {
			return res, compileFailure{Name: t.name, Log: log}
		}
	}

	_ = os.MkdirAll(res.StressDir, 0755)
	emit(stressEvent{Stage: stressStart, Total: count})

	for i := 1; i <= count; i++ {
		if stopped() {
			res.Stopped = true
			return res, nil
		}
		emit(stressEvent{Stage: stressRound, Round: i, Total: count})

		inputData, abort, err := execCmdCapture(genExe, "", p.Dir, stopped, nil)
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("第 %d 轮: gen 运行时错误 (RE)", i)
		}
		bruteOut, abort, err := execCmdCapture(bruteExe, inputData, p.Dir, stopped, nil)
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("第 %d 轮: brute 运行时错误 (RE)", i)
		}
		expectedOut := normalizeOutput(bruteOut)

		projOut, abort, err := execCmdCapture(projExe, inputData, p.Dir, stopped, projFIO)
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("第 %d 轮: 项目程序运行时错误 (RE)", i)
		}
		userOut := normalizeOutput(projOut)

		if userOut != expectedOut {
			res.FailRound = i
			res.Input = inputData
			res.Expected = expectedOut
			res.Actual = userOut
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.in", i)), []byte(inputData), 0644)
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.ans", i)), []byte(expectedOut), 0644)
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.out", i)), []byte(userOut), 0644)
			return res, nil
		}
		res.Passed++
	}
	return res, nil
}

// execCmdCapture 以 inputStr 为标准输入运行程序，返回其标准输出。
// fio 非 nil 时改为文件读写方式：inputStr 写入运行目录下的 fio.In，
// 程序结束后读 fio.Out 作为输出。运行独享临时目录，可安全并发。
// stopped 返回 true 时立即杀掉进程，并把 abort 置为 true。
func execCmdCapture(exePath, inputStr, workDir string, stopped func() bool, fio *fileIO) (string, bool, error) {
	runDir := workDir
	if fio.enabled() {
		tmp, err := os.MkdirTemp(workDir, ".otk-io-")
		if err != nil {
			return "", false, err
		}
		defer os.RemoveAll(tmp)
		if err := os.WriteFile(filepath.Join(tmp, fio.In), []byte(inputStr), 0644); err != nil {
			return "", false, err
		}
		runDir = tmp
	}

	cmd := exec.Command(exePath)
	cmd.Dir = runDir
	hideProcWindow(cmd)
	if inputStr != "" && !fio.enabled() {
		cmd.Stdin = strings.NewReader(inputStr)
	}
	var outBuf syncBuffer
	cmd.Stdout = &outBuf
	if err := cmd.Start(); err != nil {
		return "", false, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	read := func() string {
		if !fio.enabled() {
			return outBuf.String()
		}
		data, err := os.ReadFile(filepath.Join(runDir, fio.Out))
		if err != nil {
			return ""
		}
		return string(data)
	}

	if stopped == nil {
		err := <-done
		return read(), false, err
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return read(), false, err
		case <-ticker.C:
			if stopped() {
				_ = cmd.Process.Kill()
				<-done
				return read(), true, nil
			}
		}
	}
}

func cleanupStressFiles(files ...string) {
	for _, f := range files {
		if f != "" {
			_ = os.Remove(f)
		}
	}
}
