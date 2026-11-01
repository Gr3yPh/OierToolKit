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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// untranslatable 是允许不翻译的词条：纯技术片段、文件名后缀、版权行等。
// 这些内容中英文一致或本身就是符号，翻译反而会破坏行为。
var untranslatable = map[string]bool{
	"...（已截断）":                           true, // 代码里是字符串截断占位符
	"-排名.html":                           true, // 导出文件名后缀
	"-排名.csv":                            true,
	"-排名.md":                             true,
	"Copyright (C) 2026 魇珩Gr3yPh4ntom\n": true,
	"\n[otk] 程序未写出 ":                     true, // 与旧输出保持一致的诊断前缀
}

// trKeys 扫描源码里所有 tr()/trSelect() 的字面量参数。
func trKeys(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "i18n_dict.go" {
			continue
		}
		af, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || (id.Name != "tr" && id.Name != "trSelect") || len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if _, dup := keys[s]; !dup {
				keys[s] = fset.Position(call.Pos()).String()
			}
			return true
		})
	}
	return keys
}

// TestEnUICoversAllKeys 保证每个 tr() 词条都有英文，避免英文界面混入中文。
func TestEnUICoversAllKeys(t *testing.T) {
	keys := trKeys(t)
	var miss []string
	for k, pos := range keys {
		if untranslatable[k] {
			continue
		}
		if _, ok := enUI[k]; !ok {
			miss = append(miss, pos+"  "+strconv.Quote(k))
		}
	}
	sort.Strings(miss)
	if len(miss) > 0 {
		t.Fatalf("%d 个词条缺少英文翻译:\n%s", len(miss), strings.Join(miss, "\n"))
	}
}

// TestEnUINoUnusedKeys 保证词条表里没有谁也不用的死条目。
func TestEnUINoUnusedKeys(t *testing.T) {
	keys := trKeys(t)
	var unused []string
	for k := range enUI {
		if _, ok := keys[k]; !ok {
			unused = append(unused, strconv.Quote(k))
		}
	}
	sort.Strings(unused)
	if len(unused) > 0 {
		t.Fatalf("%d 个词条没有任何 tr() 引用（可能是拼写漂移）:\n%s", len(unused), strings.Join(unused, "\n"))
	}
}

// placeholderRe 匹配 printf 占位符，包括 %d %s %v %q %x %.2f %-*s 等形态。
var placeholderRe = regexp.MustCompile(`%(?:[-+ #0]*)(?:\*|\d+)?(?:\.(?:\*|\d+))?[a-zA-Z]`)

// TestEnUIPlaceholdersMatch 保证译文与原文的占位符一一对应，
// 否则英文模式下会出现「%!d(MISSING)」或多余的实参。
func TestEnUIPlaceholdersMatch(t *testing.T) {
	bad := 0
	for cn, en := range enUI {
		want := placeholderRe.FindAllString(cn, -1)
		got := placeholderRe.FindAllString(en, -1)
		if len(want) != len(got) {
			t.Errorf("占位符数量不一致:\n  中文 %q -> %v\n  英文 %q -> %v", cn, want, en, got)
			bad++
			continue
		}
		// 逐位比较动词类型（%d 不能换成 %s 之外的类型差异会被 Sprintf 容忍，
		// 但 %s 换成 %d 会输出 %!d(...)），这里只允许同类动词顺序一致。
		for i := range want {
			if want[i][len(want[i])-1] != got[i][len(got[i])-1] {
				t.Errorf("第 %d 个占位符动词不一致:\n  中文 %q -> %s\n  英文 %q -> %s", i+1, cn, want[i], en, got[i])
				bad++
				break
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d 条词条占位符不匹配", bad)
	}
}

// TestEnUINonEmpty 防止手滑写出空译文（英文模式下会显示空白）。
func TestEnUINonEmpty(t *testing.T) {
	for cn, en := range enUI {
		if strings.TrimSpace(en) == "" {
			t.Errorf("词条译文为空: %q", cn)
		}
	}
}
