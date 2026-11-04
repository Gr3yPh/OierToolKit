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
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestPrepareMathMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		notWant []string
	}{
		{
			name:  "inline",
			input: `设 $a_i$ 为第 i 项，求 $\sum a_i$。`,
			want:  []string{`a_i`, `\sum a_i`},
		},
		{
			name:  "display",
			input: "$$\\frac{a}{b}$$",
			want:  []string{`\frac{a}{b}`},
		},
		{
			name:  "bracket delimiters",
			input: `\(x^2\) 和 \[y^2\]`,
			want:  []string{`x^2`, `y^2`},
		},
		{
			name:  "bare environment",
			input: "\\begin{aligned}\na&=b\\\\\nc&=d\n\\end{aligned}",
			want:  []string{"\\begin{aligned}\na&=b\\\\\nc&=d\n\\end{aligned}"},
		},
		{
			name:    "code spans untouched",
			input:   "`$x$` 和 ```\n$y$\n```",
			notWant: []string{`x`, `y`},
		},
		{
			name:  "escaped dollar",
			input: `价格 \$5，公式 $x$`,
			want:  []string{`x`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepared, formulas := prepareMathMarkdown(tt.input)
			if !strings.Contains(prepared, mathTokenPrefix) && len(tt.want) > 0 {
				t.Fatalf("no math token in %q", prepared)
			}
			var got []string
			for _, f := range formulas {
				got = append(got, f.tex)
			}
			for _, want := range tt.want {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("formula %q not found in %#v", want, got)
				}
			}
			for _, notWant := range tt.notWant {
				for _, g := range got {
					if g == notWant {
						t.Errorf("formula %q should not be parsed (got %#v)", notWant, got)
					}
				}
			}
		})
	}
}

func TestInjectMathSegments(t *testing.T) {
	_, formulas := prepareMathMarkdown(`前 $x$ 后`)
	if len(formulas) != 1 {
		t.Fatalf("formulas = %#v", formulas)
	}
	segs := injectMathSegments([]widget.RichTextSegment{
		&widget.TextSegment{Text: "前 OTKMATH0X 后"},
	}, formulas)
	found := false
	for _, s := range segs {
		if _, ok := s.(*mathSegment); ok {
			found = true
		}
	}
	if !found {
		t.Fatalf("math segment not injected: %#v", segs)
	}
}

func TestInjectMathInHyperlink(t *testing.T) {
	_, formulas := prepareMathMarkdown(`$x$`)
	segs := injectMathSegments([]widget.RichTextSegment{
		&widget.HyperlinkSegment{Text: "前 OTKMATH0X 后"},
	}, formulas)
	links, maths := 0, 0
	for _, s := range segs {
		switch s.(type) {
		case *widget.HyperlinkSegment:
			links++
		case *mathSegment:
			maths++
		}
	}
	if links != 2 || maths != 1 {
		t.Fatalf("link segments = %d, math = %d", links, maths)
	}
}

func TestRenderMathFormula(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	r := renderMath(mathFormula{tex: `x^2+\frac{1}{2}`, kind: mathInline}, colorFgLight, 1)
	if r.err != nil {
		t.Fatalf("render math: %v", r.err)
	}
	if r.size.Width <= 0 || r.size.Height <= 0 {
		t.Fatalf("bad math size: %#v", r.size)
	}
}

func TestRenderMathGlyphHeight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	const scale = 2.0
	r := renderMath(mathFormula{tex: `X`, kind: mathDisplay}, colorFgLight, scale)
	if r.err != nil {
		t.Fatalf("render math: %v", r.err)
	}
	b := r.img.Bounds()
	minY, maxY := b.Dy(), -1
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if _, _, _, a := r.img.At(x, y).RGBA(); a > 0 {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	inkHeight := float32(maxY-minY+1) / scale
	if inkHeight < 10 {
		t.Fatalf("formula glyphs look too small: ink height %.1fpx", inkHeight)
	}
}

func TestRenderMarkdownMathLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	rich := widget.NewRichText()
	rich.Wrapping = fyne.TextWrapWord
	renderMarkdown(rich, "前 $x^2$ 后\n\n$$\\frac{a}{b}$$\n\n尾")
	rich.Resize(fyne.NewSize(420, 300))
	r := rich.CreateRenderer()
	r.Layout(fyne.NewSize(420, 300))
}
