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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 模拟赛（contest）是一整个比赛目录，布局固定为：
//
//	上传目录/data/<题目名>/<测试点>.in
//	上传目录/data/<题目名>/<测试点>.ans        （或 .out）
//	上传目录/source/<选手名>/<题目名>.cpp
//
// 导入后数据与源码被复制到 ~/.otkprofile/contests/<id>/ 下，比赛配置
// （时限、子任务、选手信息）集中在 contest.json，可直接手改，也可由 GUI
// 修改后回写。

// contestConfigPath/contestDataPath 等描述一场比赛在配置目录里的布局。
const contestConfigName = "contest.json"

// 子任务（subtask）判定方式。
const (
	// subtaskAll：捆绑测试，所有测试点都过才拿分。
	subtaskAll = "all"
	// subtaskAny：任意测试点通过即可拿分。
	subtaskAny = "any"
	// subtaskSum：按通过的测试点比例给分（部分分）。
	subtaskSum = "sum"
)

// testPoint 是比赛里的一个测试点（对应 data/<题目>/ 下一对 .in/.ans）。
type testPoint struct {
	ID      string  `json:"id"`                // 测试点名（不含后缀）
	Subtask int     `json:"subtask"`           // 所属子任务下标，-1 表示独立测试点
	Score   float64 `json:"score"`             // 该测试点分值（子任务内按比例或独立计分）
	TimeMS  int64   `json:"time_ms,omitempty"` // 单独覆盖时限，0 表示用题目时限
	MemMB   float64 `json:"mem_mb,omitempty"`  // 单独覆盖内存，0 表示用题目内存
}

// contestSubtask 描述一个子任务。
type contestSubtask struct {
	Name  string   `json:"name"`
	Score float64  `json:"score"`
	Mode  string   `json:"mode"`  // all / any / sum
	Tests []string `json:"tests"` // 包含的测试点位号
}

// contestProblem 是一道比赛的题。
type contestProblem struct {
	Name   string  `json:"name"`              // 题目名，与 data/<题目名> 同名
	Title  string  `json:"title"`             // 显示用标题，可空
	TimeMS int64   `json:"time_ms"`           // 时限（毫秒）
	MemMB  float64 `json:"mem_mb"`            // 内存限制（MB）
	IOFile bool    `json:"io_file,omitempty"` // 文件读写题
	IOIn   string  `json:"io_in,omitempty"`   // 输入文件名，空为 <题目名>.in
	IOOut  string  `json:"io_out,omitempty"`  // 输出文件名，空为 <题目名>.out
	SPJ    bool    `json:"spj,omitempty"`     // 特殊判题：data/<题目>/checker.cpp
	// Compiler 是本题编译器：g++（缺省）或 gcc（按 C 编译，链接 -lm）。
	Compiler string `json:"compiler,omitempty"`
	// Statement 是题面（Markdown），在题目配置里编辑、榜单明细里渲染。
	Statement string           `json:"statement,omitempty"`
	Tests     []testPoint      `json:"tests"`    // 测试点
	Subtasks  []contestSubtask `json:"subtasks"` // 子任务；为空表示每点独立计分
}

// checkerSourceOf 返回某道题的 checker 源码路径（约定放在 data/<题目>/ 下）。
func (c contest) checkerSourceOf(problem string) string {
	return filepath.Join(c.problemsDir(problem), checkerSourceName)
}

// spjActive 判断某道题是否实际启用特殊判题（开了开关且 checker.cpp 存在）。
func (c contest) spjActive(prob contestProblem) bool {
	return prob.SPJ && fileExists(c.checkerSourceOf(prob.Name))
}

// contestPlayer 是一名选手。
type contestPlayer struct {
	Name string `json:"name"`
}

// contestSource 是一份选手提交。
type contestSource struct {
	Player  string `json:"player"`
	Problem string `json:"problem"`
	Path    string `json:"path"` // 相对比赛目录的源码路径
}

// contestConfig 是 contest.json 的完整内容。
type contestConfig struct {
	Name       string           `json:"name"`
	ImportedAt string           `json:"imported_at"`
	SourceDir  string           `json:"source_dir"`
	Problems   []contestProblem `json:"problems"`
	Players    []contestPlayer  `json:"players"`
	Sources    []contestSource  `json:"sources"`
}

// contest 是一场已导入的模拟赛。
type contest struct {
	Dir string // ~/.otkprofile/contests/<id>
	Cfg contestConfig
}

func (c contest) dataDir() string    { return filepath.Join(c.Dir, "data") }
func (c contest) sourceDir() string  { return filepath.Join(c.Dir, "source") }
func (c contest) cfgPath() string    { return filepath.Join(c.Dir, contestConfigName) }
func (c contest) resultPath() string { return filepath.Join(c.Dir, "result.json") }

// problemsDir 返回某道题的数据目录。
func (c contest) problemsDir(problem string) string {
	return filepath.Join(c.dataDir(), problem)
}

// save 把配置写回 contest.json（缩进格式，方便手改）。
func (c contest) save() error {
	data, err := json.MarshalIndent(c.Cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.cfgPath(), append(data, '\n'), 0644)
}

// loadContest 读取指定目录下的比赛。
func loadContest(dir string) (contest, error) {
	c := contest{Dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, contestConfigName))
	if err != nil {
		return c, fmt.Errorf(tr("读取比赛配置失败: %v"), err)
	}
	if err := json.Unmarshal(data, &c.Cfg); err != nil {
		return c, fmt.Errorf(tr("比赛配置格式错误: %v"), err)
	}
	return c, nil
}

// contestID 把比赛名变成安全的目录名。
func contestID(name string) string {
	id := strings.TrimSpace(name)
	if id == "" {
		id = "contest"
	}
	return sanitizeFileName(id)
}

// sanitizeFileName 去掉路径分隔符与 Windows 保留字符。
func sanitizeFileName(name string) string {
	repl := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	out := strings.TrimSpace(repl.Replace(name))
	out = strings.Trim(out, ". ")
	if out == "" {
		out = "unnamed"
	}
	return out
}

// importContest 把上传目录导入为一场模拟赛，返回新建的比赛。
//
// 目录结构要求（缺 data/ 或 source/ 会报错，便于尽早发现路径填错）：
//
//	srcDir/data/<题目>/xxx.in + xxx.ans|.out
//	srcDir/source/<选手>/<题目>.cpp
func importContest(srcDir, name string) (contest, error) {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return contest{}, fmt.Errorf(tr("上传目录不存在: %s"), srcDir)
	}
	dataDir := filepath.Join(srcDir, "data")
	srcSub := filepath.Join(srcDir, "source")
	if !dirExists(dataDir) {
		return contest{}, fmt.Errorf(tr("上传目录里没有 data/ 文件夹: %s"), dataDir)
	}
	if !dirExists(srcSub) {
		return contest{}, fmt.Errorf(tr("上传目录里没有 source/ 文件夹: %s"), srcSub)
	}

	problems, err := scanContestProblems(dataDir)
	if err != nil {
		return contest{}, err
	}
	if len(problems) == 0 {
		return contest{}, fmt.Errorf("%s", tr("data/ 里没有任何题目（应形如 data/<题目名>/*.in）"))
	}

	sources, players, err := scanContestSources(srcSub, problems)
	if err != nil {
		return contest{}, err
	}
	if len(sources) == 0 {
		return contest{}, fmt.Errorf("%s", tr("source/ 里没有任何选手提交（应形如 source/<选手名>/<题目名>.cpp）"))
	}

	if name == "" {
		name = filepath.Base(filepath.Clean(srcDir))
	}
	id := contestID(name)
	dest := filepath.Join(contestsDir(), id)
	if dirExists(dest) {
		return contest{}, fmt.Errorf(tr("已存在同名模拟赛 %s（%s）"), name, dest)
	}

	// 先复制数据与源码，再写配置：中途失败不会留下半场"可用"的比赛。
	if err := os.MkdirAll(dest, 0755); err != nil {
		return contest{}, fmt.Errorf(tr("创建比赛目录失败: %v"), err)
	}
	if err := copyDir(filepath.Join(dataDir), filepath.Join(dest, "data")); err != nil {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf(tr("复制数据失败: %v"), err)
	}
	if err := copyDir(srcSub, filepath.Join(dest, "source")); err != nil {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf(tr("复制源码失败: %v"), err)
	}

	// 源码路径改为相对比赛目录，便于整个比赛目录搬走。
	for i := range sources {
		rel, err := filepath.Rel(dest, filepath.Join(dest, "source", sources[i].Player, sources[i].Problem+".cpp"))
		if err == nil {
			sources[i].Path = filepath.ToSlash(rel)
		}
	}

	c := contest{
		Dir: dest,
		Cfg: contestConfig{
			Name:       name,
			ImportedAt: nowStamp(),
			SourceDir:  srcDir,
			Problems:   problems,
			Players:    players,
			Sources:    sources,
		},
	}
	if err := c.save(); err != nil {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf(tr("写入比赛配置失败: %v"), err)
	}
	return c, nil
}

// scanContestProblems 扫描 data/ 下的每道题及其测试点。
func scanContestProblems(dataDir string) ([]contestProblem, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf(tr("读取 data/ 失败: %v"), err)
	}

	var problems []contestProblem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		tests, err := scanTestPoints(filepath.Join(dataDir, name))
		if err != nil {
			return nil, err
		}
		if len(tests) == 0 {
			continue // 没有 .in 的目录不算题目
		}
		problems = append(problems, contestProblem{
			Name:   name,
			Title:  name,
			TimeMS: 1000,
			MemMB:  256,
			Tests:  tests,
		})
	}
	sort.Slice(problems, func(i, j int) bool { return problems[i].Name < problems[j].Name })
	return problems, nil
}

// scanTestPoints 列出题目目录里的测试点（有 .in 且有 .ans/.out 才算）。
func scanTestPoints(dir string) ([]testPoint, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf(tr("读取 %s 失败: %v"), dir, err)
	}
	hasOut := map[string]bool{}
	hasIn := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		switch {
		case strings.HasSuffix(n, ".in"):
			hasIn[strings.TrimSuffix(n, ".in")] = true
		case strings.HasSuffix(n, ".ans"):
			hasOut[strings.TrimSuffix(n, ".ans")] = true
		case strings.HasSuffix(n, ".out"):
			hasOut[strings.TrimSuffix(n, ".out")] = true
		}
	}
	var ids []string
	for id := range hasIn {
		if hasOut[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	tests := make([]testPoint, 0, len(ids))
	for _, id := range ids {
		tests = append(tests, testPoint{ID: id, Subtask: -1, Score: 10})
	}
	return tests, nil
}

// scanContestSources 扫描 source/<选手>/<题目>.cpp。
func scanContestSources(srcDir string, problems []contestProblem) ([]contestSource, []contestPlayer, error) {
	known := map[string]bool{}
	for _, p := range problems {
		known[p.Name] = true
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, nil, fmt.Errorf(tr("读取 source/ 失败: %v"), err)
	}

	var sources []contestSource
	var players []contestPlayer
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		player := e.Name()
		players = append(players, contestPlayer{Name: player})
		files, err := os.ReadDir(filepath.Join(srcDir, player))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".cpp") {
				continue
			}
			problem := strings.TrimSuffix(f.Name(), ".cpp")
			if !known[problem] {
				continue // 没对应题目的源码（如随手放的模板）忽略
			}
			sources = append(sources, contestSource{Player: player, Problem: problem})
		}
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Name < players[j].Name })
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Player != sources[j].Player {
			return sources[i].Player < sources[j].Player
		}
		return sources[i].Problem < sources[j].Problem
	})
	return sources, players, nil
}
