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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 评测判定取值：AC 通过、WA 输出不一致、TLE 超时、MLE 超内存、
// RE 运行时错误、MISSING 样例文件缺失。
const (
	verdictAC      = "AC"
	verdictWA      = "WA"
	verdictTLE     = "TLE"
	verdictMLE     = "MLE"
	verdictRE      = "RE"
	verdictMissing = "MISSING"
)

// pointResult 是单个测试点的评测结果。Detail/Expected/Actual 只在对应
// 判定下有内容，供 CLI 与 GUI 各自展示。
type pointResult struct {
	ID       string
	Verdict  string
	WallMS   int64
	MemMB    float64
	MemOK    bool
	Detail   string
	Expected string
	Actual   string
}

// judgeReport 是一次评测的完整记录。Warnings 是编译警告（: warning: 行），
// 单独列出，不影响 verdict。
type judgeReport struct {
	CompileLog string
	Warnings   []string
	TimeLimit  float64
	MemLimit   float64
	Points     []pointResult
}

// allAC 表示全部测试点通过（无测试点视为未通过）。
func (r judgeReport) allAC() bool {
	if len(r.Points) == 0 {
		return false
	}
	for _, pt := range r.Points {
		if pt.Verdict != verdictAC {
			return false
		}
	}
	return true
}

// passed 统计通过的测试点数。
func (r judgeReport) passed() int {
	n := 0
	for _, pt := range r.Points {
		if pt.Verdict == verdictAC {
			n++
		}
	}
	return n
}

// judgeLimits 从项目配置解析时限（秒）与内存限制（MB）。
func judgeLimits(props map[string]string) (float64, float64) {
	return floatProp(props, "time_limit", 1.00), floatProp(props, "memory_limit", 125.0)
}

// fileIOFromProps 依据项目配置构造文件读写设置；未启用文件模式返回 nil。
// io_in/io_out 缺省时用源码名：codeName.in / codeName.out。
func fileIOFromProps(props map[string]string, codeName string) *fileIO {
	if strings.ToLower(strings.TrimSpace(propsOr(props, "io_mode", ""))) != "file" {
		return nil
	}
	in := strings.TrimSpace(props["io_in"])
	if in == "" {
		in = codeName + ".in"
	}
	out := strings.TrimSpace(props["io_out"])
	if out == "" {
		out = codeName + ".out"
	}
	return &fileIO{In: in, Out: out}
}

// judgeThreads 解析并发测试点数：默认按 CPU 核心数（上限 8），
// threads=1 时串行。
func judgeThreads(props map[string]string) int {
	n := atoiDefault(strings.TrimSpace(propsOr(props, "threads", "0")), 0)
	if n <= 0 {
		n = runtime.NumCPU()
		if n > 8 {
			n = 8
		}
	}
	if n < 1 {
		n = 1
	}
	return n
}

// judgeJob 描述一次评测的对象。
type judgeJob struct {
	ExePath   string
	Dir       string
	Index     []string
	Threads   int
	TimeLimit float64
	MemLimit  float64
	// FileIO 非 nil 时按文件读写题评测；nil 表示标准输入输出。
	FileIO *fileIO
	// Checker 非空时启用特殊判题：该值是已编译 checker 的路径。
	Checker string
	// Stopped 可为 nil；返回 true 时正在运行的测试点会被立即杀掉并丢弃结果。
	Stopped func() bool
}

// pointJob 是单个测试点的运行计划。
type pointJob struct {
	index int
	id    string
	in    string
	out   string
}

// evalSamples 并发跑完全部样例，每完成一个测试点回调一次（可能来自多个
// goroutine，调用方需自行同步）。返回的 Points 按样例顺序排列。
func evalSamples(job judgeJob, emit func(pointResult), stopped func() bool) judgeReport {
	rep := judgeReport{TimeLimit: job.TimeLimit, MemLimit: job.MemLimit}
	rep.Points = make([]pointResult, len(job.Index))

	var todo []pointJob
	for i, id := range job.Index {
		inPath := filepath.Join(job.Dir, id+".in")
		outPath := filepath.Join(job.Dir, id+expectedExt(job.Dir, id))
		if !fileExists(inPath) || !fileExists(outPath) {
			pt := pointResult{ID: id, Verdict: verdictMissing}
			rep.Points[i] = pt
			if emit != nil {
				emit(pt)
			}
			continue
		}
		todo = append(todo, pointJob{index: i, id: id, in: inPath, out: outPath})
	}

	workers := job.Threads
	if workers < 1 {
		workers = 1
	}
	if workers > len(todo) {
		workers = len(todo)
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		stopMu sync.Mutex
		halted bool
	)
	halt := func() bool {
		stopMu.Lock()
		defer stopMu.Unlock()
		if halted {
			return true
		}
		if stopped != nil && stopped() {
			halted = true
		}
		return halted
	}

	tasks := make(chan pointJob)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				pt, aborted := runJudgePoint(job, task)
				if aborted {
					// 被中断的测试点结果不可用，留空即可（后续会被过滤掉）。
					continue
				}
				mu.Lock()
				rep.Points[task.index] = pt
				mu.Unlock()
				if emit != nil {
					emit(pt)
				}
			}
		}()
	}

	for _, task := range todo {
		if halt() {
			break
		}
		tasks <- task
	}
	close(tasks)
	wg.Wait()

	kept := rep.Points[:0]
	for _, pt := range rep.Points {
		if pt.Verdict != "" {
			kept = append(kept, pt)
		}
	}
	rep.Points = kept
	return rep
}

// runJudgePoint 运行单个测试点并给出判定。第二个返回值表示本次运行
// 是否因调用方请求停止而被中断（此时结果不可用）。
func runJudgePoint(job judgeJob, task pointJob) (pointResult, bool) {
	pt := pointResult{ID: task.id}
	res := runMeasuredStopped(job.ExePath, task.in, job.Dir, job.TimeLimit, job.MemLimit, job.Stopped, job.FileIO)
	if res.aborted {
		return pt, true
	}
	pt.WallMS, pt.MemMB, pt.MemOK = res.wallMS, res.memMB, res.memOK

	switch {
	case res.timedOut:
		pt.Verdict = verdictTLE
	case res.runErr != nil && res.memLimitHit:
		// Windows 硬限制下超内存的程序大概率是分配失败崩掉的，
		// 先于 RE 判 MLE（细节仍是 stderr，便于排查）。
		pt.Verdict = verdictMLE
		pt.Detail = strings.TrimSpace(res.stderr)
	case res.runErr != nil:
		pt.Verdict = verdictRE
		pt.Detail = strings.TrimSpace(res.stderr)
	case res.memLimitHit || (res.memOK && res.memMB > job.MemLimit):
		pt.Verdict = verdictMLE
	default:
		expected := normalizeOutput(readFile(task.out))
		actual := normalizeOutput(res.stdout)
		if expected == actual && job.Checker == "" {
			pt.Verdict = verdictAC
		} else if job.Checker != "" {
			judgeWithChecker(job, &pt, task, res.stdout)
		} else {
			pt.Verdict = verdictWA
			pt.Expected, pt.Actual = expected, actual
		}
	}
	return pt, false
}

// cmdJudge 编译并评测样例。默认只显示各测试点结果；-v 附带报错信息
// （RE 的 stderr）与 WA 的期望/实际对比。位置参数可选样例编号（带不带 #
// 均可），只评测指定的测试点。
func cmdJudge(args []string) {
	verbose := false
	var only []string
	for _, a := range args {
		switch {
		case a == "-v", a == "--verbose":
			verbose = true
		default:
			only = append(only, strings.TrimPrefix(a, "#"))
		}
	}

	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal(tr("未找到 %s"), p.codeName()+".cpp")
	}
	props := p.cfg()

	fmt.Printf(tr("编译 %s ... "), p.codeName()+".cpp")
	log, ok := compileProjectTo(p.sourcePath(), p.exePath(), props)
	if !ok {
		fmt.Fprintf(os.Stderr, tr("%s[CE] 编译失败%s\n"), red, reset)
		fmt.Fprint(os.Stderr, log)
		otkExit(1)
	}
	fmt.Println(green + "ok" + reset)
	warnings := parseWarnings(log)

	checkerExe, ceLog, checkerOK := prepareChecker(p, props)
	if !checkerOK {
		fmt.Fprintf(os.Stderr, tr("%s[CE] checker 编译失败%s\n"), red, reset)
		fmt.Fprint(os.Stderr, ceLog)
		otkExit(1)
	}

	timeLimit, memLimit := judgeLimits(props)
	index := readIndex(p)
	if len(only) > 0 {
		index = filterIndex(index, only)
		if len(index) == 0 {
			fatal(tr("没有匹配的样例"))
		}
	}
	if len(index) == 0 {
		fatal(tr("无样例（放入 .in 与 .out/.ans 后运行 otk refresh）"))
	}

	threads := judgeThreads(props)
	threadNote := ""
	if threads > 1 {
		threadNote = fmt.Sprintf(tr(" · %d 线程"), threads)
	}
	spjNote := ""
	if checkerExe != "" {
		spjNote = tr(" · 特殊判题")
	}
	fmt.Printf(tr("评测 %s (%.2fs / %.0fMB%s%s)\n"), p.codeName(), timeLimit, memLimit, threadNote, spjNote)

	job := judgeJob{
		ExePath:   p.exePath(),
		Dir:       p.Dir,
		Index:     index,
		Threads:   threads,
		TimeLimit: timeLimit,
		MemLimit:  memLimit,
		FileIO:    fileIOFromProps(props, p.codeName()),
		Checker:   checkerExe,
	}
	var mu sync.Mutex // 并发回调下保证每行输出完整
	rep := evalSamples(job, func(pt pointResult) {
		mu.Lock()
		defer mu.Unlock()
		printJudgePoint(pt, widest(index, 3), timeLimit, verbose)
	}, nil)
	rep.CompileLog = log
	rep.Warnings = warnings

	if len(warnings) > 0 {
		fmt.Printf(tr("%s编译警告 %d 条%s\n"), yellow, len(warnings), reset)
		if verbose {
			for _, w := range warnings {
				printIndented(w, "  ")
			}
		}
	}
	if len(only) == 0 {
		// 单点重测不进历史，完整评测才记录。
		appendHistory(p, rep)
	}

	if !rep.allAC() {
		otkExit(1)
	}
}

// filterIndex 从样例索引里挑出指定的编号，保持索引顺序。未知的编号报错。
func filterIndex(index, want []string) []string {
	inIndex := map[string]bool{}
	for _, id := range index {
		inIndex[id] = true
	}
	for _, id := range want {
		if !inIndex[id] {
			fatal(tr("样例 #%s 不存在（可用: %s）"), id, strings.Join(index, " "))
		}
	}
	var kept []string
	for _, id := range index {
		for _, w := range want {
			if id == w {
				kept = append(kept, id)
				break
			}
		}
	}
	return kept
}

// parseWarnings 从编译输出里挑出警告行（file:line:col: warning: ...）。
func parseWarnings(log string) []string {
	var out []string
	for _, l := range strings.Split(log, "\n") {
		if strings.Contains(l, ": warning:") {
			out = append(out, strings.TrimRight(l, "\r"))
		}
	}
	return out
}

// printJudgePoint 以 CLI 风格输出一个测试点。
func printJudgePoint(pt pointResult, width int, timeLimit float64, verbose bool) {
	metrics := fmt.Sprintf("%dms %s", pt.WallMS, memStr(pt))
	switch pt.Verdict {
	case verdictMissing:
		fmt.Printf(tr("  #%-*s %s缺失%s\n"), width, pt.ID, yellow, reset)
	case verdictTLE:
		fmt.Printf("  #%-*s %sTLE%s >%.0fms\n", width, pt.ID, red, reset, timeLimit*1000)
	case verdictRE:
		fmt.Printf("  #%-*s %sRE%s   %s\n", width, pt.ID, red, reset, metrics)
		if verbose && pt.Detail != "" {
			printIndented(firstLines(pt.Detail, 6), "      ")
		}
	case verdictMLE:
		fmt.Printf("  #%-*s %sMLE%s   %s\n", width, pt.ID, red, reset, metrics)
	case verdictAC:
		fmt.Printf("  #%-*s %sAC%s    %s\n", width, pt.ID, green, reset, metrics)
	default:
		fmt.Printf("  #%-*s %sWA%s    %s\n", width, pt.ID, red, reset, metrics)
		if verbose {
			if pt.Detail != "" && pt.Expected == "" && pt.Actual == "" {
				// 特殊判题：展示 checker 的反馈而不是 diff。
				printIndented(pt.Detail, "    ")
			} else {
				printDiff(pt.Expected, pt.Actual)
			}
		}
	}
}

func cmdRun() {
	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal(tr("未找到 %s"), p.codeName()+".cpp")
	}
	props := p.cfg()

	fmt.Printf(tr("编译 %s ... "), p.codeName()+".cpp")
	if !compileProject(p.sourcePath(), p.exePath(), props) {
		otkExit(1)
	}
	fmt.Println(green + "ok" + reset)

	cmd := exec.Command(p.exePath())
	cmd.Dir = p.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(tr("运行失败: %v"), err)
	}
}

func cmdDebug() {
	p := requireProject()
	if !fileExists(p.exePath()) {
		fatal(tr("未找到可执行文件，先运行 otk run 或 otk judge"))
	}

	cmd := exec.Command("gdb", "--args", p.exePath())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = p.Dir
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, tr("%s启动 gdb 失败: %v%s\n"), red, err, reset)
		fmt.Fprintln(os.Stderr, yellow+tr("提示: 请确保 gdb 已安装并在 PATH 中")+reset)
		otkExit(1)
	}
}

func cmdEdit() {
	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal(tr("未找到 %s"), p.codeName()+".cpp")
	}

	editor := propsOr(readIni(globalConfigPath()), "editor", defaultEditorCommand())

	parts := splitQuoted(fmt.Sprintf(editor, p.sourcePath()))
	if len(parts) == 0 {
		fatal(tr("无效的编辑器命令"))
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = p.Dir
	if err := cmd.Run(); err != nil {
		fatal(tr("打开编辑器 %s 失败: %v"), parts[0], err)
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

// compilerOf 返回项目使用的编译器：gcc 或 g++（默认）。
func compilerOf(props map[string]string) string {
	if propsOr(props, "compiler", "g++") == "gcc" {
		return "gcc"
	}
	return "g++"
}

// compileArgs 依据项目配置组装编译参数：语言标准、O2、用户自定义额外
// 参数（cflags）与调试信息。gcc 只切换编译器二进制并链接 -lm，
// 不注入任何额外开关；语言标准错配时（gcc 配了 c++*）回退 c17。
func compileArgs(srcPath, exePath string, props map[string]string) []string {
	compiler := compilerOf(props)
	version := propsOr(props, "version", "c++17")
	if compiler == "gcc" && strings.HasPrefix(version, "c++") {
		version = "c17"
	}
	args := []string{"-std=" + version}
	if propsOr(props, "o2", "1") == "1" {
		args = append(args, "-O2")
	}
	if extra := strings.TrimSpace(props["cflags"]); extra != "" {
		args = append(args, splitQuoted(extra)...)
	}
	args = append(args, srcPath, "-o", exePath, "-g")
	if compiler == "gcc" {
		args = append(args, "-lm")
	}
	return args
}

// compileProjectTo 编译源码，返回编译器输出与是否成功。
// 用 CombinedOutput 同时收 stdout 与 stderr：不同平台/编译器把诊断信息
// 写到哪一路并不一致，只接 stderr 偶尔会拿不到错误原因。
func compileProjectTo(srcPath, exePath string, props map[string]string) (string, bool) {
	log, ok, _ := compileProjectStopped(srcPath, exePath, props, nil)
	return log, ok
}

// compileProjectStopped 在编译过程中响应停止信号：一旦 stopped 返回 true
// 就杀掉 g++。第三个返回值表示是否被中断（此时 ok 无意义）。
func compileProjectStopped(srcPath, exePath string, props map[string]string, stopped func() bool) (string, bool, bool) {
	args := compileArgs(srcPath, exePath, props)

	cmd := exec.Command(compilerOf(props), args...)
	hideProcWindow(cmd)
	var buf syncBuffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return err.Error(), false, false
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// 编译（尤其是 -O2 下的大文件）可能耗时几十秒，必须能被打断。
	var ticker *time.Ticker
	var tick <-chan time.Time
	if stopped != nil {
		ticker = time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		tick = ticker.C
	}
	for {
		select {
		case err := <-done:
			out := buf.String()
			if err != nil && out == "" {
				out = err.Error()
			}
			return out, err == nil, false
		case <-tick:
			if stopped() {
				_ = cmd.Process.Kill()
				<-done
				return buf.String(), false, true
			}
		}
	}
}

// compileProject 使用项目配置编译源码，始终附带 -g 以便 gdb 调试。
func compileProject(srcPath, exePath string, props map[string]string) bool {
	cmd := exec.Command(compilerOf(props), compileArgs(srcPath, exePath, props)...)
	hideProcWindow(cmd)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, tr("%s[CE] 编译失败%s\n"), red, reset)
		fmt.Fprint(os.Stderr, errBuf.String())
		return false
	}
	return true
}

// normalizeOutput 统一换行并去除首尾空白，作为比较口径。
func normalizeOutput(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

// memStr 格式化峰值内存，测量不可用时显示 N/A。
func memStr(pt pointResult) string {
	if !pt.MemOK {
		return "N/A"
	}
	return fmt.Sprintf("%.2fMB", pt.MemMB)
}

func printDiff(stdOut, userOut string) {
	fmt.Printf(tr("    %s期望%s %s\n"), green, reset, strings.ReplaceAll(strings.TrimRight(stdOut, "\n"), "\n", "\n    "))
	fmt.Printf(tr("    %s实际%s %s\n"), red, reset, strings.ReplaceAll(strings.TrimRight(userOut, "\n"), "\n", "\n    "))
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
