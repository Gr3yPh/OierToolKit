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
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 模拟赛评测：每一份选手提交独立编译（-O2），再逐测试点跑一遍，
// 按 subtask 规则折算成题目得分。编译与运行都并发，机器越闲越快。

// judgeEvent 评测过程中的进度事件。
type judgeEvent struct {
	Kind    string // "compile" / "run" / "done"
	Player  string
	Problem string
	Message string
	Percent float64 // 0..1，整体进度
}

// 事件类型常量。
const (
	judgeCompile = "compile"
	judgeRun     = "run"
	judgeDone    = "done"
)

// testOutcome 是某个选手某道题某个测试点的结果。
type testOutcome struct {
	ID      string  `json:"id"`
	Verdict string  `json:"verdict"`
	TimeMS  int64   `json:"time_ms"`
	MemMB   float64 `json:"mem_mb"`
	Message string  `json:"message,omitempty"`
}

// subtaskOutcome 是某个子任务的得分情况。
type subtaskOutcome struct {
	Name    string        `json:"name"`
	Score   float64       `json:"score"`
	Awarded float64       `json:"awarded"`
	Mode    string        `json:"mode"`
	Passed  bool          `json:"passed"`
	Tests   []testOutcome `json:"tests"`
}

// problemResult 是一份提交在一道题上的结果。
type problemResult struct {
	Player     string           `json:"player"`
	Problem    string           `json:"problem"`
	Score      float64          `json:"score"`
	CompileLog string           `json:"compile_log,omitempty"`
	CompileOK  bool             `json:"compile_ok"`
	Subtasks   []subtaskOutcome `json:"subtasks"`
	Tests      []testOutcome    `json:"tests,omitempty"`
}

// contestResult 是一整场比赛的评测结果。
type contestResult struct {
	ContestName string          `json:"contest_name"`
	JudgedAt    string          `json:"judged_at"`
	Problems    []string        `json:"problems"`
	Results     []problemResult `json:"results"`
}

// scoreOf 返回某选手某题得分。
func (r contestResult) scoreOf(player, problem string) float64 {
	for _, res := range r.Results {
		if res.Player == player && res.Problem == problem {
			return res.Score
		}
	}
	return 0
}

// resultOf 查找某选手某题的结果。
func (r contestResult) resultOf(player, problem string) (problemResult, bool) {
	for _, res := range r.Results {
		if res.Player == player && res.Problem == problem {
			return res, true
		}
	}
	return problemResult{}, false
}

// totalOf 返回某选手的总分。
func (r contestResult) totalOf(player string) float64 {
	var sum float64
	for _, res := range r.Results {
		if res.Player == player {
			sum += res.Score
		}
	}
	return sum
}

// contestParallelism 决定并发度。默认「自动」：留一个核心给系统，上限 8，
// 最少 1。设置里可改成固定值（1~32），GUI 与 CLI 读同一份全局配置。
func contestParallelism() int {
	if n := guiPrefInt(prefJudgeParallel, 0); n > 0 {
		if n > 32 {
			n = 32
		}
		return n
	}
	n := runtime.NumCPU() - 1
	if n > 8 {
		n = 8
	}
	if n < 1 {
		n = 1
	}
	return n
}

// judgeContest 评测整场比赛。每个 (选手, 题目) 组合并发编译并跑完全部
// 测试点；emit 可能被任意 goroutine 调用，调用方需自行同步。
func judgeContest(c contest, emit func(judgeEvent), stopped func() bool) (contestResult, error) {
	// 统一转绝对路径：exec 的相对 cmd.Dir 在 Windows 上会直接失败
	// （fork/exec cannot find the path），也不能依赖进程当前目录不变。
	if abs, err := filepath.Abs(c.Dir); err == nil {
		c.Dir = abs
	}

	res := contestResult{
		ContestName: c.Cfg.Name,
		JudgedAt:    nowStamp(),
	}
	for _, p := range c.Cfg.Problems {
		res.Problems = append(res.Problems, p.Name)
	}

	sources := c.Cfg.Sources
	if len(sources) == 0 {
		return res, fmt.Errorf("%s", tr("这场比赛没有任何选手提交"))
	}
	problems := map[string]contestProblem{}
	for _, p := range c.Cfg.Problems {
		problems[p.Name] = p
	}

	buildDir := filepath.Join(c.Dir, ".build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return res, fmt.Errorf(tr("无法创建构建目录: %v"), err)
	}
	// 停止时直接把整个构建目录删掉：Windows 上正在运行的 exe 删不掉，
	// 但编译产物通常已经写完，删不掉也无妨，下次评测会覆盖。
	defer os.RemoveAll(buildDir)

	// 特殊判题：每道开了 SPJ 的题编译一次 checker，所有选手共用。
	checkers := map[string]string{}
	for _, p := range c.Cfg.Problems {
		if !c.spjActive(p) {
			continue
		}
		exe := filepath.Join(buildDir, "checker__"+sanitizeFileName(p.Name)+exeSuffix())
		log, ok, _ := compileProjectStopped(c.checkerSourceOf(p.Name), exe,
			map[string]string{"version": "c++17"}, stopped)
		if ok {
			checkers[p.Name] = exe
		} else if emit != nil {
			emit(judgeEvent{Kind: judgeCompile, Problem: p.Name,
				Message: fmt.Sprintf(tr("%s：checker 编译失败，本题退回普通比对\n%s"), p.Name, log)})
		}
	}

	parallel := contestParallelism()
	total := len(sources)
	var done int64
	var mu sync.Mutex
	results := make([]problemResult, 0, total)

	halted := func() bool { return stopped != nil && stopped() }

	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	// 停止监听 goroutine 的退出口：wg.Wait 之后它们就没有存在意义了，
	// 不关掉的话每份提交泄漏一个 50ms ticker，长期运行会持续累积。
	cancelWatch := make(chan struct{})
	defer close(cancelWatch)
	for _, src := range sources {
		if halted() {
			break
		}
		wg.Add(1)
		go func(src contestSource) {
			defer wg.Done()
			// 拿到并发槽位后要再确认一次：排队期间可能已经被取消。
			select {
			case sem <- struct{}{}:
			case <-stopCh(stopped, cancelWatch):
				return
			}
			defer func() { <-sem }()
			if halted() {
				return
			}

			prob, ok := problems[src.Problem]
			if !ok {
				return
			}
			out, aborted := judgeSubmission(c, prob, src, buildDir, checkers[prob.Name], stopped)
			if aborted {
				return
			}
			mu.Lock()
			results = append(results, out)
			mu.Unlock()

			n := atomic.AddInt64(&done, 1)
			if emit != nil {
				emit(judgeEvent{
					Kind:    judgeDone,
					Player:  src.Player,
					Problem: src.Problem,
					Message: fmt.Sprintf(tr("%s / %s：%.0f 分"), src.Player, src.Problem, out.Score),
					Percent: float64(n) / float64(total),
				})
			}
		}(src)
	}
	wg.Wait()

	// 结果按选手、题目顺序稳定输出，方便对比与导出。
	sort.Slice(results, func(i, j int) bool {
		if results[i].Player != results[j].Player {
			return results[i].Player < results[j].Player
		}
		return results[i].Problem < results[j].Problem
	})
	res.Results = results
	return res, nil
}

// judgeSubmission 编译并评测一份提交。第二个返回值表示是否被中途停止。
// checker 非空时按特殊判题判定（SPJ 关闭或 checker 缺失时为空串）。
func judgeSubmission(c contest, prob contestProblem, src contestSource, buildDir, checker string, stopped func() bool) (problemResult, bool) {
	out := problemResult{Player: src.Player, Problem: src.Problem}

	srcPath := filepath.Join(c.Dir, filepath.FromSlash(src.Path))
	if !fileExists(srcPath) {
		out.CompileLog = tr("源码缺失: ") + src.Path
		return out, false
	}

	// 可执行文件名带上选手与题目，避免并发编译互相覆盖。
	exe := filepath.Join(buildDir,
		sanitizeFileName(src.Player)+"__"+sanitizeFileName(src.Problem)+exeSuffix())

	props := map[string]string{
		"version": "c++17",
		"o2":      "1",
	}
	if prob.Compiler != "" {
		props["compiler"] = prob.Compiler
	}
	log, ok, aborted := compileProjectStopped(srcPath, exe, props, stopped)
	if aborted {
		return out, true
	}
	out.CompileLog = log
	out.CompileOK = ok
	if !ok {
		return out, false
	}
	defer os.Remove(exe)

	timeLimit := float64(prob.TimeMS) / 1000.0
	if timeLimit <= 0 {
		timeLimit = 1.0
	}
	memLimit := prob.MemMB
	if memLimit <= 0 {
		memLimit = 256
	}

	verdicts := map[string]testOutcome{}
	dataDir := c.problemsDir(prob.Name)

	// 文件读写题：输入复制为 <题目>.in（或自定义名），从输出文件读答案。
	var fio *fileIO
	if prob.IOFile {
		inName := strings.TrimSpace(prob.IOIn)
		if inName == "" {
			inName = prob.Name + ".in"
		}
		outName := strings.TrimSpace(prob.IOOut)
		if outName == "" {
			outName = prob.Name + ".out"
		}
		fio = &fileIO{In: inName, Out: outName}
	}

	for _, t := range prob.Tests {
		if stopped != nil && stopped() {
			return out, true
		}
		inPath := filepath.Join(dataDir, t.ID+".in")
		ansPath := contestAnswerPath(dataDir, t.ID)
		to := testOutcome{ID: t.ID}

		if !fileExists(inPath) || ansPath == "" {
			to.Verdict = verdictMissing
			verdicts[t.ID] = to
			out.Tests = append(out.Tests, to)
			continue
		}

		limit := timeLimit
		if t.TimeMS > 0 {
			limit = float64(t.TimeMS) / 1000.0
		}
		run := runMeasuredStopped(exe, inPath, c.Dir, limit, memLimit, stopped, fio)
		if run.aborted {
			return out, true
		}
		to.TimeMS, to.MemMB = run.wallMS, run.memMB

		memCap := memLimit
		if t.MemMB > 0 {
			memCap = t.MemMB
		}
		switch {
		case run.timedOut:
			to.Verdict = verdictTLE
		case run.runErr != nil && run.memLimitHit:
			// Windows 的内存硬限制通常以进程崩溃收场，光看 runErr 会把
			// MLE 误判成 RE（与本地评测 judge.go 的判定保持一致）。
			to.Verdict = verdictMLE
		case run.runErr != nil:
			to.Verdict = verdictRE
			to.Message = strings.TrimSpace(run.stderr)
		case run.memOK && run.memMB > memCap:
			to.Verdict = verdictMLE
		default:
			if checker != "" {
				// 特殊判题：checker 认可才算通过，反馈写进 Message。
				if ok, msg := spjRunOnOutput(checker, inPath, run.stdout, ansPath); ok {
					to.Verdict = verdictAC
					if msg != "" {
						to.Message = msg
					}
				} else {
					to.Verdict = verdictWA
					to.Message = msg
				}
			} else if normalizeOutput(run.stdout) == normalizeOutput(readFile(ansPath)) {
				to.Verdict = verdictAC
			} else {
				to.Verdict = verdictWA
			}
		}
		verdicts[t.ID] = to
		out.Tests = append(out.Tests, to)
	}

	out.Score, out.Subtasks = scoreContestProblem(prob, verdicts)
	return out, false
}

// stopCh 把轮询式的停止信号包装成一个 channel，用于在排队等待并发槽位时
// 及时退出（不额外开 goroutine 时，select 无法直接监视函数返回值）。
// done 关闭时监听 goroutine 自行退出，避免泄漏；正常跑完的评测从不触发
// stopped，靠的就是这个出口。
func stopCh(stopped func() bool, done <-chan struct{}) <-chan struct{} {
	ch := make(chan struct{})
	if stopped == nil {
		// 永不触发。
		return ch
	}
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if stopped() {
					close(ch)
					return
				}
			}
		}
	}()
	return ch
}

// contestAnswerPath 返回测试点的答案文件路径，优先 .ans 再 .out。
func contestAnswerPath(dir, id string) string {
	if fileExists(filepath.Join(dir, id+".ans")) {
		return filepath.Join(dir, id+".ans")
	}
	if fileExists(filepath.Join(dir, id+".out")) {
		return filepath.Join(dir, id+".out")
	}
	return ""
}

// scoreContestProblem 按子任务规则计算题目得分。
//
// 没有配置子任务时，每个测试点独立计分（各自 Score 相加）。
// 配了子任务时，按 all / any / sum 折算该子任务的得分：
//
//	all —— 全部通过才得满分（捆绑测试）；
//	any —— 任一通过即得满分；
//	sum —— 按通过比例给分。
func scoreContestProblem(prob contestProblem, verdicts map[string]testOutcome) (float64, []subtaskOutcome) {
	// 没有子任务：逐点累加。
	if len(prob.Subtasks) == 0 {
		var sum float64
		for _, t := range prob.Tests {
			if v, ok := verdicts[t.ID]; ok && v.Verdict == verdictAC {
				sum += t.Score
			}
		}
		return sum, nil
	}

	var total float64
	var subs []subtaskOutcome
	for _, st := range prob.Subtasks {
		o := subtaskOutcome{Name: st.Name, Score: st.Score, Mode: st.Mode}
		if o.Mode == "" {
			o.Mode = subtaskAll
		}

		passedCount := 0
		for _, id := range st.Tests {
			v, ok := verdicts[id]
			if !ok {
				v = testOutcome{ID: id, Verdict: verdictMissing}
			}
			o.Tests = append(o.Tests, v)
			if v.Verdict == verdictAC {
				passedCount++
			}
		}

		switch o.Mode {
		case subtaskAny:
			if passedCount > 0 {
				o.Passed = true
				o.Awarded = st.Score
			}
		case subtaskSum:
			if len(st.Tests) > 0 {
				o.Awarded = st.Score * float64(passedCount) / float64(len(st.Tests))
				o.Passed = passedCount == len(st.Tests)
			}
		default: // subtaskAll
			if len(st.Tests) > 0 && passedCount == len(st.Tests) {
				o.Passed = true
				o.Awarded = st.Score
			}
		}
		total += o.Awarded
		subs = append(subs, o)
	}
	return total, subs
}

// exeSuffix 返回当前平台的可执行文件后缀。
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// saveContestResult 把评测结果写入 result.json。
func saveContestResult(c contest, res contestResult) error {
	return writeJSON(c.resultPath(), res)
}

// loadContestResult 读取上次的评测结果，没有则返回空结果。
func loadContestResult(c contest) (contestResult, bool) {
	var res contestResult
	if !fileExists(c.resultPath()) {
		return res, false
	}
	if err := readJSON(c.resultPath(), &res); err != nil {
		return contestResult{}, false
	}
	return res, true
}
