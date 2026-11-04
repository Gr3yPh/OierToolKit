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
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	texlayout "github.com/luo-studio/go-tex/tex/layout"
	"github.com/luo-studio/go-tex/tex/mathstyle"
	"github.com/luo-studio/go-tex/tex/parser"
	"github.com/luo-studio/go-tex/tex/render"
)

const (
	mathInlineFontSize  = 18.0
	mathDisplayFontSize = 20.0

	// go-tex 的 canvas 渲染器把 TeX 的“1 用户单位”直接当成 1pt，
	// 而 tdewolff/canvas 的坐标单位是毫米；结果字形只有布局尺寸的
	// 25.4/72。这里把 display list 的位置/宽度提前缩回去，再按
	// 相同的比例放大字号，使字形和公式宽度都回到正常比例。
	mathCanvasUnit = 25.4 / 72.0
)

type mathRendered struct {
	img      image.Image
	size     fyne.Size
	baseline float32
	err      error
}

var mathRenderCache sync.Map // map[string]*mathRendered

// mathSegment 是 RichText 里的一个公式。公式在第一次真正绘制时才排版，
// 避免打开题面时一次性卡住；渲染结果按公式/字号/前景色缓存。
type mathSegment struct {
	formula mathFormula
}

func newMathSegment(f mathFormula) *mathSegment {
	return &mathSegment{formula: f}
}

func (m *mathSegment) Inline() bool {
	return m.formula.kind != mathDisplay
}

func (m *mathSegment) Textual() string {
	return m.formula.tex
}

func (m *mathSegment) Visual() fyne.CanvasObject {
	r := m.render()
	if r.err != nil {
		return widget.NewLabel("$" + m.formula.tex + "$")
	}
	img := canvas.NewImageFromImage(r.img)
	img.FillMode = canvas.ImageFillContain
	img.ScaleMode = canvas.ImageScaleSmooth
	if m.formula.kind == mathDisplay {
		return container.New(&mathBlockLayout{size: r.size}, img)
	}
	return container.New(&mathInlineLayout{
		size:   r.size,
		offset: r.baseline,
	}, img)
}

func (m *mathSegment) Update(fyne.CanvasObject) {
	// 公式段是不可变的；主题切换需要重启，因此不需要在这里重绘。
}

func (m *mathSegment) Select(_, _ fyne.Position) {}

func (m *mathSegment) SelectedText() string {
	return m.Textual()
}

func (m *mathSegment) Unselect() {}

func (m *mathSegment) render() *mathRendered {
	fg := mathForeground()
	scale := mathScale()
	key := fmt.Sprintf("%d|%d,%d,%d|%.3f|%s", m.formula.kind, fg.R, fg.G, fg.B, scale, m.formula.tex)
	if v, ok := mathRenderCache.Load(key); ok {
		return v.(*mathRendered)
	}
	r := renderMath(m.formula, fg, scale)
	actual, _ := mathRenderCache.LoadOrStore(key, r)
	return actual.(*mathRendered)
}

func mathScale() float64 {
	scale := 1.0
	if app := fyne.CurrentApp(); app != nil {
		if s := app.Settings().Scale(); s > 0 {
			scale = float64(s)
		}
	}
	// 至少按 2x 栅格化，避免 1x 屏幕上公式先被画成低分辨率再放大。
	if scale < 2 {
		return 2
	}
	return scale
}

func mathForeground() color.NRGBA {
	if app := fyne.CurrentApp(); app != nil {
		variant := app.Settings().ThemeVariant()
		c := app.Settings().Theme().Color(theme.ColorNameForeground, variant)
		r, g, b, a := c.RGBA()
		return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	return colorFgLight
}

func renderMath(f mathFormula, fg color.NRGBA, scale float64) (result *mathRendered) {
	result = &mathRendered{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = &mathRendered{err: fmt.Errorf("latex: %v", recovered)}
		}
	}()

	nodes, err := parser.Parse(f.tex)
	if err != nil {
		result.err = err
		return result
	}
	opts := texlayout.DefaultOptions()
	size := mathInlineFontSize
	if f.kind == mathDisplay {
		size = mathDisplayFontSize
	} else {
		opts = opts.WithStyle(mathstyle.Text)
	}
	dl := texlayout.ToDisplayList(texlayout.Layout(nodes, opts))
	dl = normalizeMathDisplayList(dl)
	fontSize := size / mathCanvasUnit

	var buf bytes.Buffer
	if err := render.PNGTo(&buf, dl,
		render.WithFontSize(fontSize),
		render.WithPadding(2),
		render.WithScale(scale),
	); err != nil {
		result.err = err
		return result
	}
	src, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		result.err = err
		return result
	}
	b := src.Bounds()
	result.img = tintMathImage(src, fg)
	result.size = fyne.NewSize(
		float32(float64(b.Dx())/scale),
		float32(float64(b.Dy())/scale),
	)
	result.baseline = float32(2 + dl.Height*fontSize)
	return result
}

// normalizeMathDisplayList 修正 go-tex canvasr 的 pt/mm 单位混用。
// 位置、线段和路径按 25.4/72 缩小，字形 Scale 保持不变；随后
// render 的 FontSize 按 72/25.4 放大，二者相乘正好回到正常尺寸。
func normalizeMathDisplayList(dl texlayout.DisplayList) texlayout.DisplayList {
	k := mathCanvasUnit
	dl.Width *= k
	dl.Height *= k
	dl.Depth *= k
	for i, item := range dl.Items {
		switch v := item.(type) {
		case texlayout.GlyphPath:
			v.X *= k
			v.Y *= k
			dl.Items[i] = v
		case texlayout.Line:
			v.X *= k
			v.Y *= k
			v.Width *= k
			v.Thickness *= k
			dl.Items[i] = v
		case texlayout.Rect:
			v.X *= k
			v.Y *= k
			v.Width *= k
			v.Height *= k
			dl.Items[i] = v
		case texlayout.PathItem:
			v.X *= k
			v.Y *= k
			for j := range v.Commands {
				c := &v.Commands[j]
				c.X *= k
				c.Y *= k
				c.X1 *= k
				c.Y1 *= k
				c.X2 *= k
				c.Y2 *= k
			}
			dl.Items[i] = v
		}
	}
	return dl
}

// tintMathImage 把 go-tex 的黑色字形改成当前主题的前景色。
func tintMathImage(src image.Image, fg color.NRGBA) image.Image {
	b := src.Bounds()
	maxAlpha := uint32(0)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			_, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a > maxAlpha {
				maxAlpha = a
			}
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if maxAlpha == 0 {
		return dst
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			_, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a == 0 {
				continue
			}
			alpha := uint8((a*255 + maxAlpha/2) / maxAlpha)
			dst.SetNRGBA(x, y, color.NRGBA{R: fg.R, G: fg.G, B: fg.B, A: alpha})
		}
	}
	return dst
}

// mathInlineLayout 把行内公式略微上移，让公式的基线大致贴近正文基线。
// Fyne 的行布局会把没有文字基线的对象按正文基线整体下移，这里用负
// 偏移抵消这部分位移。
type mathInlineLayout struct {
	size   fyne.Size
	offset float32
}

func (l *mathInlineLayout) Layout(objects []fyne.CanvasObject, _ fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(l.size)
	objects[0].Move(fyne.NewPos(0, -l.offset))
}

func (l *mathInlineLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.size
}

// mathBlockLayout 让独立公式居中；公式比可用宽度更宽时等比缩小。
type mathBlockLayout struct {
	size fyne.Size
}

func (l *mathBlockLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	w, h := l.size.Width, l.size.Height
	if w > size.Width && w > 0 {
		h *= size.Width / w
		w = size.Width
	}
	objects[0].Resize(fyne.NewSize(w, h))
	objects[0].Move(fyne.NewPos((size.Width-w)/2, 0))
}

func (l *mathBlockLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.size
}
