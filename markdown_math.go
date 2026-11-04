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
	"strconv"
	"strings"

	"fyne.io/fyne/v2/widget"
)

// mathKind 区分行内公式和独立公式。
type mathKind int

const (
	mathInline mathKind = iota
	mathDisplay
)

type mathFormula struct {
	tex  string
	kind mathKind
}

const mathTokenPrefix = "OTKMATH"
const mathTokenSuffix = "X"

// renderMarkdown 把 Markdown 里的 $...$ / $$...$$ / \(...\) / \[...\]
// 换成占位符，交给 Fyne 解析后再替换成公式段。这样标题、列表、表格、
// 链接等 Markdown 行为仍然完全由 Fyne 负责，不需要另写一套解析器。
func renderMarkdown(rich *widget.RichText, content string) {
	prepared, formulas := prepareMathMarkdown(content)
	rich.ParseMarkdown(prepared)
	if len(formulas) > 0 {
		rich.Segments = injectMathSegments(rich.Segments, formulas)
	}
	rich.Refresh()
}

// prepareMathMarkdown 扫描 Markdown，保护代码块/行内代码，并把数学公式
// 替换成不会触发 Markdown 语法的占位符。
func prepareMathMarkdown(content string) (string, []mathFormula) {
	var out strings.Builder
	var formulas []mathFormula
	out.Grow(len(content))

	addFormula := func(tex string, kind mathKind) string {
		idx := len(formulas)
		formulas = append(formulas, mathFormula{tex: normalizeMath(tex), kind: kind})
		return mathTokenPrefix + strconv.Itoa(idx) + mathTokenSuffix
	}

	for i := 0; i < len(content); {
		// 围栏代码块整段原样保留，避免把代码里的 $ 当成公式。
		if i == 0 || content[i-1] == '\n' {
			if end, ok := fencedCodeEnd(content, i); ok {
				out.WriteString(content[i:end])
				i = end
				continue
			}
			// 兼容不带 $$ 的 \begin{...}...\end{...} 独立公式块。
			if begin, end, ok := displayEnvironment(content, i); ok {
				out.WriteString("\n\n")
				out.WriteString(addFormula(content[begin:end], mathDisplay))
				out.WriteString("\n\n")
				i = end
				continue
			}
		}

		switch {
		case content[i] == '`':
			// 行内代码原样保留。
			end := inlineCodeEnd(content, i)
			out.WriteString(content[i:end])
			i = end

		case strings.HasPrefix(content[i:], "$$"):
			if end, ok := findMathDelimiter(content, i+2, "$$"); ok {
				out.WriteString("\n\n")
				out.WriteString(addFormula(content[i+2:end], mathDisplay))
				out.WriteString("\n\n")
				i = end + 2
			} else {
				out.WriteString(content[i : i+1])
				i++
			}

		case strings.HasPrefix(content[i:], `\[`):
			if end, ok := findMathDelimiter(content, i+2, `\]`); ok {
				out.WriteString("\n\n")
				out.WriteString(addFormula(content[i+2:end], mathDisplay))
				out.WriteString("\n\n")
				i = end + 2
			} else {
				out.WriteString(content[i : i+1])
				i++
			}

		case content[i] == '$' && !escapedAt(content, i):
			if end, ok := findInlineMathEnd(content, i+1); ok {
				out.WriteString(addFormula(content[i+1:end], mathInline))
				i = end + 1
			} else {
				out.WriteString(content[i : i+1])
				i++
			}

		case strings.HasPrefix(content[i:], `\(`):
			if end, ok := findMathDelimiter(content, i+2, `\)`); ok {
				out.WriteString(addFormula(content[i+2:end], mathInline))
				i = end + 2
			} else {
				out.WriteString(content[i : i+1])
				i++
			}

		default:
			// 按字节拷贝即可：UTF-8 多字节序列不会被这里拆开处理。
			out.WriteByte(content[i])
			i++
		}
	}
	return out.String(), formulas
}

// displayEnvironment 识别行首的 \begin{env}...\end{env} 块，返回内容范围。
// 只接受同一环境名；没有闭合时返回 false，让 Markdown 按原文处理。
func displayEnvironment(s string, start int) (int, int, bool) {
	i := start
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	const begin = `\begin{`
	if !strings.HasPrefix(s[i:], begin) {
		return 0, 0, false
	}
	nameStart := i + len(begin)
	nameEnd := strings.IndexByte(s[nameStart:], '}')
	if nameEnd < 0 {
		return 0, 0, false
	}
	nameEnd += nameStart
	name := s[nameStart:nameEnd]
	if name == "" {
		return 0, 0, false
	}
	endTag := `\end{` + name + `}`
	rel := strings.Index(s[nameEnd+1:], endTag)
	if rel < 0 {
		return 0, 0, false
	}
	end := nameEnd + 1 + rel + len(endTag)
	return i, end, true
}

func normalizeMath(tex string) string {
	return strings.TrimSpace(strings.ReplaceAll(tex, "\r\n", "\n"))
}

// escapedAt 判断 pos 前面的反斜杠是否为奇数个。
func escapedAt(s string, pos int) bool {
	n := 0
	for i := pos - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// findMathDelimiter 找到未被转义的结束分隔符。
func findMathDelimiter(s string, start int, delim string) (int, bool) {
	for i := start; i+len(delim) <= len(s); i++ {
		// 先匹配结束分隔符，再跳过转义字符；否则 \] / \) 开头的
		// 结束符会被当成“被转义的字符”而永远找不到。
		if strings.HasPrefix(s[i:], delim) {
			return i, true
		}
		if s[i] == '\\' {
			i++
		}
	}
	return 0, false
}

// findInlineMathEnd 找到行内 $...$ 的结束位置。开头和结尾都不能是空白，
// 避免把普通美元金额吞成公式；公式内不允许换行。
func findInlineMathEnd(s string, start int) (int, bool) {
	if start >= len(s) || isSpaceByte(s[start]) {
		return 0, false
	}
	for i := start; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return 0, false
		}
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '$' && i > start && !isSpaceByte(s[i-1]) {
			return i, true
		}
	}
	return 0, false
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// inlineCodeEnd 返回从 start 开始的行内代码结束位置（包括结束反引号）。
// 没有闭合时只返回开头的一串反引号，保持原文。
func inlineCodeEnd(s string, start int) int {
	n := 0
	for start+n < len(s) && s[start+n] == '`' {
		n++
	}
	openEnd := start + n
	for i := openEnd; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		if j-i >= n {
			return j
		}
		i = j
	}
	return openEnd
}

// fencedCodeEnd 识别从行首开始的围栏代码块，并返回整个代码块结束后的位置。
// 未闭合的围栏按“直到文档末尾都是代码”处理。
func fencedCodeEnd(s string, start int) (int, bool) {
	lineEnd := strings.IndexByte(s[start:], '\n')
	if lineEnd < 0 {
		lineEnd = len(s) - start
	}
	line := s[start : start+lineEnd]
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, false
	}
	fenceChar := trimmed[0]
	n := 0
	for n < len(trimmed) && trimmed[n] == fenceChar {
		n++
	}
	if n < 3 {
		return 0, false
	}
	pos := start + lineEnd
	if pos < len(s) {
		pos++
	}
	for pos <= len(s) {
		next := strings.IndexByte(s[pos:], '\n')
		end := len(s)
		if next >= 0 {
			end = pos + next + 1
		}
		line = strings.TrimSpace(s[pos:end])
		if len(line) >= n {
			k := 0
			for k < len(line) && line[k] == fenceChar {
				k++
			}
			if k >= n && strings.TrimSpace(line[k:]) == "" {
				return end, true
			}
		}
		if next < 0 {
			break
		}
		pos = end
	}
	return len(s), true
}

// findMathToken 在 text 里寻找 OTKMATH<n>X 占位符。
func findMathToken(text string, from int, formulas []mathFormula) (start, idx, end int, ok bool) {
	for i := from; i+len(mathTokenPrefix)+len(mathTokenSuffix) <= len(text); i++ {
		if !strings.HasPrefix(text[i:], mathTokenPrefix) {
			continue
		}
		j := i + len(mathTokenPrefix)
		k := j
		for k < len(text) && text[k] >= '0' && text[k] <= '9' {
			k++
		}
		if k == j || k >= len(text) || text[k] != 'X' {
			continue
		}
		n, err := strconv.Atoi(text[j:k])
		if err != nil || n < 0 || n >= len(formulas) {
			continue
		}
		return i, n, k + 1, true
	}
	return 0, 0, 0, false
}

// injectMathSegments 递归替换 Fyne Markdown 结果里的占位符。
func injectMathSegments(segs []widget.RichTextSegment, formulas []mathFormula) []widget.RichTextSegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]widget.RichTextSegment, 0, len(segs))
	for _, seg := range segs {
		switch s := seg.(type) {
		case *widget.TextSegment:
			out = append(out, splitMathTextSegment(s, formulas)...)
		case *widget.HyperlinkSegment:
			out = append(out, splitMathHyperlinkSegment(s, formulas)...)
		case *widget.ParagraphSegment:
			s.Texts = injectMathSegments(s.Texts, formulas)
			out = append(out, s)
		case *widget.ListSegment:
			s.Items = injectMathSegments(s.Items, formulas)
			out = append(out, s)
		case *widget.CheckBoxSegment:
			s.Text = replaceMathTokensWithText(s.Text, formulas)
			out = append(out, s)
		case *widget.TableSegment:
			for i := range s.Headers {
				s.Headers[i] = injectMathSegments(s.Headers[i], formulas)
			}
			for i := range s.Rows {
				for j := range s.Rows[i] {
					s.Rows[i][j] = injectMathSegments(s.Rows[i][j], formulas)
				}
			}
			out = append(out, s)
		default:
			out = append(out, seg)
		}
	}
	return out
}

func splitMathTextSegment(s *widget.TextSegment, formulas []mathFormula) []widget.RichTextSegment {
	text := s.Text
	var out []widget.RichTextSegment
	pos := 0
	for {
		start, idx, end, ok := findMathToken(text, pos, formulas)
		if !ok {
			break
		}
		if start > pos {
			out = append(out, &widget.TextSegment{Style: s.Style, Text: text[pos:start]})
		}
		out = append(out, newMathSegment(formulas[idx]))
		pos = end
	}
	if len(out) == 0 {
		return []widget.RichTextSegment{s}
	}
	if pos < len(text) {
		out = append(out, &widget.TextSegment{Style: s.Style, Text: text[pos:]})
	}
	return out
}

func splitMathHyperlinkSegment(s *widget.HyperlinkSegment, formulas []mathFormula) []widget.RichTextSegment {
	text := s.Text
	var out []widget.RichTextSegment
	pos := 0
	for {
		start, idx, end, ok := findMathToken(text, pos, formulas)
		if !ok {
			break
		}
		if start > pos {
			copy := *s
			copy.Text = text[pos:start]
			out = append(out, &copy)
		}
		out = append(out, newMathSegment(formulas[idx]))
		pos = end
	}
	if len(out) == 0 {
		return []widget.RichTextSegment{s}
	}
	if pos < len(text) {
		copy := *s
		copy.Text = text[pos:]
		out = append(out, &copy)
	}
	return out
}

func replaceMathTokensWithText(text string, formulas []mathFormula) string {
	var out strings.Builder
	pos := 0
	for {
		start, idx, end, ok := findMathToken(text, pos, formulas)
		if !ok {
			break
		}
		out.WriteString(text[pos:start])
		out.WriteString(formulas[idx].tex)
		pos = end
	}
	if out.Len() == 0 {
		return text
	}
	out.WriteString(text[pos:])
	return out.String()
}
