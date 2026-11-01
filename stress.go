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
	// Counterexamples 是发现的反例总数：默认模式恒为 0 或 1，
	// 开启 keepGoing 后会继续对拍并收集多个反例。
	Counterexamples int
	// 三类程序的累计耗时（毫秒）与最慢一轮的轮次、总耗时。
	GenMs       int64
	BruteMs     int64
	ProjMs      int64
	SlowRound   int
	SlowRoundMs int64
	Input       string
	Expected    string
	Actual      string
	StressDir   string
}

// compileFailure 表示编译阶段失败，Log 是编译器输出。
type compileFailure struct {
	Name string
	Log  string
}

func (e compileFailure) Error() string {
	return fmt.Sprintf(tr("%s 编译失败 (CE)"), e.Name)
}

func cmdStress(args []string) {
	if len(args) > 1 {
		fatal(tr("用法: otk stress [轮数]"))
	}
	count := 100
	if len(args) == 1 {
		c, err := strconv.Atoi(args[0])
		if err != nil || c <= 0 {
			fatal(tr("轮数必须是正整数"))
		}
		count = c
	}

	p := requireProject()
	projCpp := p.sourcePath()
	genCpp := filepath.Join(p.Dir, "gen.cpp")
	bruteCpp := filepath.Join(p.Dir, "brute.cpp")

	for _, pair := range [][2]string{{projCpp, p.codeName() + ".cpp"}, {genCpp, "gen.cpp"}, {bruteCpp, "brute.cpp"}} {
		if !fileExists(pair[0]) {
			fatal(tr("未找到 %s"), pair[1])
		}
	}

	// 对拍期间 Ctrl+C 用于优雅中断并清理临时可执行文件。
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	res, err := runStress(p, count, false, func(ev stressEvent) {
		switch ev.Stage {
		case stressCompile:
			fmt.Printf(tr("编译 %s ... %sok%s\n"), ev.Name, green, reset)
		case stressStart:
			fmt.Printf(tr("对拍 %d 轮... (Ctrl+C 中断)\n"), ev.Total)
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
	if res.SlowRound > 0 {
		fmt.Printf(tr("  耗时统计: gen %.1fs / brute %.1fs / 程序 %.1fs  最慢第 %d 轮（%dms）\n"),
			secs(res.GenMs), secs(res.BruteMs), secs(res.ProjMs), res.SlowRound, res.SlowRoundMs)
	}
	switch {
	case res.Stopped:
		fmt.Println(yellow + tr("对拍被中断") + reset)
		otkExit(130)
	case res.FailRound > 0:
		fmt.Printf(tr("%s第 %d 轮 WA%s（共发现 %d 组反例，已保存到 %s）\n"), red, res.FailRound, reset, res.Counterexamples, res.StressDir)
		fmt.Printf(tr("  已保存测试数据至 %s\n"), res.StressDir)
		printDiff(res.Expected, res.Actual)
		otkExit(1)
	default:
		fmt.Printf(tr("%s全部通过 (%d/%d)%s\n"), green, res.Total, res.Total, reset)
	}
}

// runStress 编译 gen/brute/项目源码并逐轮对拍；不打印、不退出，进度经 emit
// 上报，stopped 返回 true 时在当前轮结束后中断。keepGoing 为 true 时发现
// 反例不立即返回，而是保存后继续对拍、收集多个反例（结果里记录首个）。
func runStress(p project, count int, keepGoing bool, emit func(stressEvent), stopped func() bool) (stressResult, error) {
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

	// 时限：gen/brute 给 30s 宽限；项目程序用配置时限的三倍（压测数据通常
	// 比官方数据大，给余量），至少 15s——足够拦住死循环，又不误伤慢程序。
	genLimit := 30 * time.Second
	projLimit := time.Duration(0)
	if t := floatProp(props, "time_limit", 0); t > 0 {
		projLimit = time.Duration(t * 3 * float64(time.Second))
		if min := 15 * time.Second; projLimit < min {
			projLimit = min
		}
	}

	for i := 1; i <= count; i++ {
		if stopped() {
			res.Stopped = true
			return res, nil
		}
		emit(stressEvent{Stage: stressRound, Round: i, Total: count})

		start := time.Now()
		inputData, abort, err := execCmdCapture(genExe, "", p.Dir, stopped, nil, genLimit)
		genMs := time.Since(start).Milliseconds()
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			if errors.Is(err, errStressTimeout) {
				return res, fmt.Errorf(tr("第 %d 轮: gen 超时 (TLE，时限 %s)"), i, genLimit)
			}
			return res, fmt.Errorf(tr("第 %d 轮: gen 运行时错误 (RE)"), i)
		}
		start = time.Now()
		bruteOut, abort, err := execCmdCapture(bruteExe, inputData, p.Dir, stopped, nil, genLimit)
		bruteMs := time.Since(start).Milliseconds()
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			if errors.Is(err, errStressTimeout) {
				return res, fmt.Errorf(tr("第 %d 轮: brute 超时 (TLE，时限 %s)"), i, genLimit)
			}
			return res, fmt.Errorf(tr("第 %d 轮: brute 运行时错误 (RE)"), i)
		}
		expectedOut := normalizeOutput(bruteOut)

		start = time.Now()
		projOut, abort, err := execCmdCapture(projExe, inputData, p.Dir, stopped, projFIO, projLimit)
		projMs := time.Since(start).Milliseconds()
		if abort {
			res.Stopped = true
			return res, nil
		}
		if err != nil {
			if errors.Is(err, errStressTimeout) {
				return res, fmt.Errorf(tr("第 %d 轮: 项目程序超时 (TLE，时限 %s)"), i, projLimit)
			}
			return res, fmt.Errorf(tr("第 %d 轮: 项目程序运行时错误 (RE)"), i)
		}
		userOut := normalizeOutput(projOut)

		res.GenMs += genMs
		res.BruteMs += bruteMs
		res.ProjMs += projMs
		if total := genMs + bruteMs + projMs; total > res.SlowRoundMs {
			res.SlowRound = i
			res.SlowRoundMs = total
		}

		if userOut != expectedOut {
			if res.FailRound == 0 {
				res.FailRound = i
				res.Input = inputData
				res.Expected = expectedOut
				res.Actual = userOut
			}
			res.Counterexamples++
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.in", i)), []byte(inputData), 0644)
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.ans", i)), []byte(expectedOut), 0644)
			_ = os.WriteFile(filepath.Join(res.StressDir, fmt.Sprintf("%d.out", i)), []byte(userOut), 0644)
			if !keepGoing {
				return res, nil
			}
			continue
		}
		res.Passed++
	}
	return res, nil
}

// errStressTimeout 标记被测程序超过时限被强杀，调用方据此把 RE 文案
// 换成 TLE。
var errStressTimeout = errors.New("对拍程序超时")

// execCmdCapture 以 inputStr 为标准输入运行程序，返回其标准输出。
// fio 非 nil 时改为文件读写方式：inputStr 写入运行目录下的 fio.In，
// 程序结束后读 fio.Out 作为输出。运行独享临时目录，可安全并发。
// stopped 返回 true 时立即杀掉进程，并把 abort 置为 true。
// timeLimit 是运行时限，超过即强杀并返回 errStressTimeout（0 表示不限），
// 没有它，选手程序死循环或 gen 卡住时对拍会永久挂起。
func execCmdCapture(exePath, inputStr, workDir string, stopped func() bool, fio *fileIO, timeLimit time.Duration) (string, bool, error) {
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

	if stopped == nil && timeLimit <= 0 {
		err := <-done
		return read(), false, err
	}
	var limitCh <-chan time.Time
	if timeLimit > 0 {
		timer := time.NewTimer(timeLimit)
		defer timer.Stop()
		limitCh = timer.C
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return read(), false, err
		case <-limitCh:
			_ = cmd.Process.Kill()
			<-done
			return read(), false, fmt.Errorf("%w（超过 %s）", errStressTimeout, timeLimit)
		case <-ticker.C:
			if stopped() {
				_ = cmd.Process.Kill()
				<-done
				return read(), true, nil
			}
		}
	}
}

// secs 把毫秒转成带一位小数的秒。
func secs(ms int64) float64 {
	return float64(ms) / 1000
}

// timingSummary 生成对拍耗时统计的展示文本（CLI 与 GUI 共用）。
func timingSummary(res stressResult) string {
	if res.SlowRound <= 0 {
		return ""
	}
	rounds := res.Passed + res.Counterexamples
	if rounds <= 0 {
		rounds = 1
	}
	avg := (res.GenMs + res.BruteMs + res.ProjMs) / int64(rounds)
	return fmt.Sprintf(tr("耗时统计: gen %.1fs / brute %.1fs / 程序 %.1fs  平均 %dms/轮  最慢第 %d 轮（%dms）"),
		secs(res.GenMs), secs(res.BruteMs), secs(res.ProjMs), avg, res.SlowRound, res.SlowRoundMs)
}

func cleanupStressFiles(files ...string) {
	for _, f := range files {
		if f != "" {
			_ = os.Remove(f)
		}
	}
}
