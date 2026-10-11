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
	missingOut []string // 有 .in 缺 .out
	missingIn  []string // 有 .out 缺 .in
}

// syncSamples 扫描项目目录中的 .in/.out 文件对并同步样例索引。
// 项目目录里放好文件对，refresh 一下即入数据库。
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
		case strings.HasSuffix(name, ".out"):
			outSet[strings.TrimSuffix(name, ".out")] = true
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

func printPairWarnings(res refreshResult) {
	for _, id := range res.missingOut {
		fmt.Printf("! %s.in 缺少 .out\n", id)
	}
	for _, id := range res.missingIn {
		fmt.Printf("! %s.out 缺少 .in\n", id)
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
	printPairWarnings(res)
}

// cmdSamples 列出样例库。
func cmdSamples() {
	p := requireProject()
	index := readIndex(p)
	if len(index) == 0 {
		fmt.Println("（无样例，放入 .in/.out 文件后运行 otk refresh）")
		return
	}
	width := 3
	for _, id := range index {
		if len(id) > width {
			width = len(id)
		}
	}
	for _, id := range index {
		fmt.Printf("#%-*s in %s  out %s\n", width, id, sizeStr(p.Dir, id, ".in"), sizeStr(p.Dir, id, ".out"))
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
