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
	"sort"
	"strings"
)

// readIndex 读取样例索引（.otk/samples），保持登记顺序。
func readIndex(p project) []string {
	var ids []string
	b, err := os.ReadFile(p.samplesPath())
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			ids = append(ids, l)
		}
	}
	return ids
}

func writeIndex(p project, ids []string) {
	path := p.samplesPath()
	if len(ids) == 0 {
		_ = os.Remove(path)
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(strings.Join(ids, "\n")+"\n"), 0644)
}

type refreshResult struct {
	added      []string // 新入库的成对文件
	removed    []string // 文件消失、移出索引的
	missingOut []string // 有 .in 缺期望输出
	missingIn  []string // 有期望输出缺 .in
}

// expectedExt 返回样例期望输出的后缀：.ans（比赛下发格式，如洛谷样例包）
// 优先，其次 .out；两者都不存在时按 .out 报缺失。
func expectedExt(dir, id string) string {
	if fileExists(filepath.Join(dir, id+".ans")) {
		return ".ans"
	}
	return ".out"
}

// syncSamples 扫描项目目录中的样例文件对并同步样例索引。输入固定 .in，
// 期望输出为 .out 或 .ans，任一即可成对。
func syncSamples(p project, write bool) refreshResult {
	var res refreshResult
	files, _ := os.ReadDir(p.Dir)

	inSet := map[string]bool{}
	outSet := map[string]bool{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		switch {
		case strings.HasSuffix(name, ".in"):
			inSet[strings.TrimSuffix(name, ".in")] = true
		case strings.HasSuffix(name, ".out"), strings.HasSuffix(name, ".ans"):
			outSet[strings.TrimSuffix(name, filepath.Ext(name))] = true
		}
	}

	var pairs []string
	for id := range inSet {
		if outSet[id] {
			pairs = append(pairs, id)
		}
	}
	sort.Strings(pairs)

	index := readIndex(p)
	inIndex := map[string]bool{}
	for _, id := range index {
		inIndex[id] = true
	}
	inPairs := map[string]bool{}
	for _, id := range pairs {
		inPairs[id] = true
	}

	newIndex := append([]string{}, index...)
	for _, id := range pairs {
		if !inIndex[id] {
			res.added = append(res.added, id)
			newIndex = append(newIndex, id)
		}
	}
	var kept []string
	for _, id := range newIndex {
		if inPairs[id] {
			kept = append(kept, id)
		} else {
			res.removed = append(res.removed, id)
		}
	}

	for id := range inSet {
		if !outSet[id] {
			res.missingOut = append(res.missingOut, id)
		}
	}
	sort.Strings(res.missingOut)
	for id := range outSet {
		if !inSet[id] {
			res.missingIn = append(res.missingIn, id)
		}
	}
	sort.Strings(res.missingIn)

	if write {
		writeIndex(p, kept)
	}
	return res
}

// printPairWarnings 提示不成对的文件。p 用于分辨孤立的期望输出是 .out 还是 .ans。
func printPairWarnings(p project, res refreshResult) {
	for _, id := range res.missingOut {
		fmt.Printf("! %s.in 缺少 .out 或 .ans\n", id)
	}
	for _, id := range res.missingIn {
		fmt.Printf("! %s 缺少 .in\n", id+expectedExt(p.Dir, id))
	}
}

// cmdRefresh 同步样例库：文件对入库，失效条目移除。
func cmdRefresh() {
	p := requireProject()
	res := syncSamples(p, true)
	for _, id := range res.added {
		fmt.Printf("%s+ #%s%s\n", green, id, reset)
	}
	for _, id := range res.removed {
		fmt.Printf("%s- #%s%s\n", yellow, id, reset)
	}
	printPairWarnings(p, res)
}

// cmdSamples 列出样例库。
func cmdSamples() {
	p := requireProject()
	index := readIndex(p)
	if len(index) == 0 {
		fmt.Println("（无样例，放入 .in 与 .out/.ans 后运行 otk refresh）")
		return
	}
	width := 3
	for _, id := range index {
		if len(id) > width {
			width = len(id)
		}
	}
	for _, id := range index {
		exp := expectedExt(p.Dir, id)
		fmt.Printf("#%-*s in %s  %s %s\n", width, id,
			sizeStr(p.Dir, id, ".in"), strings.TrimPrefix(exp, "."), sizeStr(p.Dir, id, exp))
	}
}

func sizeStr(dir, id, ext string) string {
	fi, err := os.Stat(filepath.Join(dir, id+ext))
	if err != nil {
		return "缺失"
	}
	return fmt.Sprintf("%dB", fi.Size())
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

// widest 返回样例编号的最大宽度，用于 CLI 输出对齐（至少 min 列）。
func widest(ids []string, min int) int {
	w := min
	for _, id := range ids {
		if len(id) > w {
			w = len(id)
		}
	}
	return w
}

// sampleFilePath 返回样例输入/输出文件路径。
func sampleFilePath(p project, id, kind string) string {
	if kind == "out" {
		return filepath.Join(p.Dir, id+expectedExt(p.Dir, id))
	}
	return filepath.Join(p.Dir, id+".in")
}

// readSample 读取一组样例的输入与期望输出。
func readSample(p project, id string) (input, output string, err error) {
	inPath := sampleFilePath(p, id, "in")
	outPath := sampleFilePath(p, id, "out")
	in, err := os.ReadFile(inPath)
	if err != nil {
		return "", "", fmt.Errorf("读取 %s.in 失败: %v", id, err)
	}
	out, err := os.ReadFile(outPath)
	if err != nil {
		return "", "", fmt.Errorf("读取样例输出失败: %v", err)
	}
	return string(in), string(out), nil
}

// writeSample 写入一组样例并登记到索引。output 沿用已有的 .ans/.out 后缀。
func writeSample(p project, id, input, output string) error {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("非法的样例编号: %q", id)
	}

	ext := ".out"
	if fileExists(filepath.Join(p.Dir, id+".ans")) {
		ext = ".ans"
	}
	if err := os.WriteFile(filepath.Join(p.Dir, id+".in"), []byte(input), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(p.Dir, id+ext), []byte(output), 0644); err != nil {
		return err
	}

	index := readIndex(p)
	for _, have := range index {
		if have == id {
			return nil
		}
	}
	writeIndex(p, append(index, id))
	return nil
}

// deleteSample 删除一组样例的全部文件并从样例库移除。
func deleteSample(p project, id string) error {
	for _, name := range []string{id + ".in", id + ".out", id + ".ans"} {
		if err := removeFileIfExists(filepath.Join(p.Dir, name)); err != nil {
			return err
		}
	}
	var kept []string
	for _, have := range readIndex(p) {
		if have != id {
			kept = append(kept, have)
		}
	}
	writeIndex(p, kept)
	return nil
}
