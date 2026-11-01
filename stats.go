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
	"strings"
)

// codeStats 是源码的静态统计结果：体量、括号配对与常见输入法导致的
// 全角字符问题。括号不配对或出现全角分号/引号等都是常见的编译错误来源。
type codeStats struct {
	Lines     int    // 总行数
	NonEmpty  int    // 非空行数
	Chars     int    // 字符数（rune）
	Bytes     int    // 字节数
	CommentLn int    // 注释行数（含 // 或 /* */
	OpenBrace int    // 未闭合的 {
	OpenParen int    // 未闭合的 (
	OpenBrack int    // 未闭合的 [
	FullWidth string // 检测到的全角字符及行号，空串表示没有
	Warnings  []string
}

// codeStatsOf 统计一段源码。字符串字面量、字符常量与注释里的内容
// 不参与括号配对，也不参与全角字符检测（注释里的中文标点是正常的）。
func codeStatsOf(src string) codeStats {
	cs := codeStats{Bytes: len(src)}
	cs.Chars = len([]rune(src))
	if strings.TrimSpace(src) == "" {
		return cs
	}
	cs.Lines = strings.Count(strings.TrimSuffix(src, "\n"), "\n") + 1

	inBlock := false // 跨行块注释 /* ... */
	var fwLines []string

	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cs.NonEmpty++
		}
		if inBlock || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") {
			cs.CommentLn++
		}

		runes := []rune(line)
		comment := false
		inStr := false
		inChar := false
		for j := 0; j < len(runes); j++ {
			r := runes[j]
			switch {
			case inBlock:
				if r == '*' && j+1 < len(runes) && runes[j+1] == '/' {
					inBlock = false
					j++
				}
				continue
			case inStr:
				if r == '\\' {
					j++
				} else if r == '"' {
					inStr = false
				}
				continue
			case inChar:
				if r == '\\' {
					j++
				} else if r == '\'' {
					inChar = false
				}
				continue
			case comment:
				continue
			case r == '/' && j+1 < len(runes) && runes[j+1] == '/':
				comment = true // 行注释到行尾为止，不再统计
			case r == '/' && j+1 < len(runes) && runes[j+1] == '*':
				inBlock = true
				j++
			case r == '"':
				inStr = true
			case r == '\'':
				inChar = true
			case r == '{':
				cs.OpenBrace++
			case r == '}':
				cs.OpenBrace--
			case r == '(':
				cs.OpenParen++
			case r == ')':
				cs.OpenParen--
			case r == '[':
				cs.OpenBrack++
			case r == ']':
				cs.OpenBrack--
			}
			if !comment && !inBlock && !inStr && !inChar && isFullWidthPunct(r) {
				fwLines = append(fwLines, fmt.Sprintf("%d", i+1))
			}
		}
	}

	if len(fwLines) > 0 {
		// 去重相邻重复（同一行多个全角字符只报一次行号）
		var uniq []string
		for _, s := range fwLines {
			if len(uniq) == 0 || uniq[len(uniq)-1] != s {
				uniq = append(uniq, s)
			}
		}
		cs.FullWidth = strings.Join(uniq, ", ")
	}

	if cs.OpenBrace != 0 {
		cs.Warnings = append(cs.Warnings, fmt.Sprintf(tr("大括号不配对（差 %d 个）"), cs.OpenBrace))
	}
	if cs.OpenParen != 0 {
		cs.Warnings = append(cs.Warnings, fmt.Sprintf(tr("圆括号不配对（差 %d 个）"), cs.OpenParen))
	}
	if cs.OpenBrack != 0 {
		cs.Warnings = append(cs.Warnings, fmt.Sprintf(tr("方括号不配对（差 %d 个）"), cs.OpenBrack))
	}
	if cs.FullWidth != "" {
		cs.Warnings = append(cs.Warnings, fmt.Sprintf(tr("第 %s 行疑似出现全角标点/汉字（常见于中文输入法），会导致编译错误"), cs.FullWidth))
	}
	return cs
}

// isFullWidthPunct 判断是否为中文输入法常见的全角标点。
func isFullWidthPunct(r rune) bool {
	switch r {
	case '；', '：', '，', '。', '（', '）', '「', '」', '『', '』',
		'“', '”', '‘', '’', '【', '】', '《', '》', '？', '！':
		return true
	}
	return false
}

// statsSummary 生成一段可展示的统计摘要文本。
func statsSummary(cs codeStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, tr("行数 %d（非空 %d，注释 %d）    字符 %d / %d 字节\n\n"),
		cs.Lines, cs.NonEmpty, cs.CommentLn, cs.Chars, cs.Bytes)
	fmt.Fprintf(&b, tr("括号配对  { %d    ( %d    [ %d\n"), cs.OpenBrace, cs.OpenParen, cs.OpenBrack)
	if len(cs.Warnings) == 0 {
		b.WriteString(tr("\n没有发现问题。"))
	} else {
		b.WriteString(tr("\n⚠ 检查结果："))
		for _, w := range cs.Warnings {
			b.WriteString("\n  · " + w)
		}
	}
	return b.String()
}
