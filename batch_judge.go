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

// 批量评测：把一个项目库里的多个项目串行跑一遍。
//
// 串行而不是并行：每个项目内部已经按 threads 开了并发测试点，再叠一层
// 项目级并发会把机器压满，单点耗时也会飘。串行还有个好处——结果顺序
// 与勾选顺序一致，进度条不会乱跳。

import "strconv"

// batchResult 是批量评测里一个项目的结果。
type batchResult struct {
	Name   string
	Dir    string
	Passed int
	Total  int
	// Err 非空表示没跑起来：没有样例、找不到源码、编译失败等。
	Err string
}

// OK 表示这个项目跑完了（不代表全对）。
func (r batchResult) OK() bool { return r.Err == "" }

// score 是 "通过/总数" 文本；跑不起来时给原因。
func (r batchResult) score() string {
	if !r.OK() {
		return r.Err
	}
	return strconv.Itoa(r.Passed) + "/" + strconv.Itoa(r.Total)
}

// judgeProjectHeadless 编译并评测一个项目的全部样例，结果追加进评测历史。
//
// 与界面里的 judgePage.run 走同一套组件（compileProjectTo / prepareChecker /
// evalSamples），区别只是不需要往控件里画进度，所以能脱离页面单独调用。
// stopped 返回 true 时尽快收工，已跑完的测试点仍计入结果。
func judgeProjectHeadless(p project, stopped func() bool) batchResult {
	res := batchResult{Name: p.Name, Dir: p.Dir}

	props := p.cfg()
	index := readIndex(p)
	if len(index) == 0 {
		res.Err = tr("没有登记样例")
		return res
	}
	src := p.sourcePath()
	if !fileExists(src) {
		res.Err = tr("找不到源码")
		return res
	}
	if stopped != nil && stopped() {
		res.Err = tr("已停止")
		return res
	}

	exe := p.exePath()
	if _, ok := compileProjectTo(src, exe, props); !ok {
		res.Err = tr("编译失败")
		return res
	}
	checkerExe, _, ok := prepareChecker(p, props)
	if !ok {
		res.Err = tr("checker 编译失败")
		return res
	}

	limit, memCap := judgeLimits(props)
	job := judgeJob{
		ExePath:   exe,
		Dir:       p.Dir,
		Index:     index,
		Threads:   judgeThreads(props),
		TimeLimit: limit,
		MemLimit:  memCap,
		FileIO:    fileIOFromProps(props, p.codeName()),
		Checker:   checkerExe,
		Stopped:   stopped,
	}
	rep := evalSamples(job, nil, stopped)
	appendHistory(p, rep)
	res.Passed, res.Total = rep.passed(), len(rep.Points)
	return res
}
