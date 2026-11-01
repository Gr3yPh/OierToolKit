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
	_ "embed"
	"image/color"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// themeValues 是存进偏好文件的值；auto 表示交给系统。
// 显示文案由 themeLabels 在调用时翻译——包级变量里不能调 tr()，
// 那时 baseDir 还没初始化。
var themeValues = []string{"auto", "light", "dark"}

// themeLabels 返回设置页下拉框的显示文案，与 themeValues 一一对应。
func themeLabels() []string {
	return []string{tr("跟随系统"), tr("浅色"), tr("深色")}
}

// applyThemeEnv 按偏好决定浅色还是深色。
//
// Fyne 没有公开 API 在运行时改主题变体：变体只在创建 App 时读一次
// FYNE_THEME 环境变量与偏好里的主题名，之后 Settings().ThemeVariant()
// 一直返回那个值。所以这里在 app.NewWithID 之前落一次环境变量，
// 改主题要重启才生效——和界面语言同一个规则，行为一致。
//
// 只写环境变量、不改主题对象：界面里所有取色都问 ThemeVariant()，
// 两条来源必须一致，否则会出现"控件按深色画、背景按浅色画"的花脸。
func applyThemeEnv() {
	switch strings.ToLower(strings.TrimSpace(guiPref(prefTheme, "auto"))) {
	case "light":
		_ = os.Setenv("FYNE_THEME", "light")
	case "dark":
		_ = os.Setenv("FYNE_THEME", "dark")
	}
}

// themeIndexFromValue 把偏好值转成下拉框下标（未知值按"跟随系统"）。
func themeIndexFromValue(v string) int {
	v = strings.ToLower(strings.TrimSpace(v))
	for i, val := range themeValues {
		if val == v {
			return i
		}
	}
	return 0
}

// 内置中文字体：Fyne 自带字体不含 CJK 字形，不打字体的话界面会出现豆腐块。
// 这里用的是 Noto Sans SC 的 GB2312 子集（SIL OFL 1.1，见 assets/font/OFL.txt），
// 覆盖 ASCII、常用符号与全部 GB2312 汉字，约 2.4MB。
//
//go:embed assets/font/NotoSansSC-Regular.ttf
var guiFontData []byte

var guiFont = fyne.NewStaticResource("NotoSansSC-Regular.ttf", guiFontData)

// 内置简体中文翻译：Fyne 自带的 zh-Hans 词条很少，像 Select 的占位
// 提示 "(Select one)" 会缺失并在日志里刷 "Translation failure"。这里补全
// 常用词条，启动时装进语言包。
//
//go:embed assets/lang/base.zh_Hans.json
var guiZhLangData []byte

// installTranslations 在 i18n.go 中实现（随界面语言偏好决定是否注册中文）。

// 界面配色：强调色沿用品牌蓝，浅色偏冷灰、深色压暗，避免大块纯白刺眼。
var (
	colorAccentLight = color.NRGBA{R: 0x2F, G: 0x6F, B: 0xE8, A: 0xFF}
	colorAccentDark  = color.NRGBA{R: 0x6A, G: 0x9B, B: 0xFF, A: 0xFF}
	colorBgLight     = color.NRGBA{R: 0xF6, G: 0xF7, B: 0xF9, A: 0xFF}
	colorBgDark      = color.NRGBA{R: 0x15, G: 0x17, B: 0x1A, A: 0xFF}
	colorPanelLight  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colorPanelDark   = color.NRGBA{R: 0x1E, G: 0x21, B: 0x26, A: 0xFF}
	colorLineLight   = color.NRGBA{R: 0xE2, G: 0xE5, B: 0xEA, A: 0xFF}
	colorLineDark    = color.NRGBA{R: 0x2E, G: 0x33, B: 0x3B, A: 0xFF}

	// 正文与各类"弱化但仍需可读"的文字色。Fyne 默认把
	// ColorNameDisabled 当成浅灰（浅色 #E3E3E3 / 深色 #39393A），而 Label 的
	// LowImportance 正是取这个颜色——在我们的背景上几乎完全看不见。
	// 这里把 disabled 重新定义成"次要文字色"，同时禁用态的控件也因此可读。
	colorFgLight     = color.NRGBA{R: 0x1F, G: 0x23, B: 0x2A, A: 0xFF} // 主要文字
	colorFgDark      = color.NRGBA{R: 0xEC, G: 0xEF, B: 0xF3, A: 0xFF}
	colorMutedLight  = color.NRGBA{R: 0x5A, G: 0x63, B: 0x70, A: 0xFF} // 说明文字：6.2:1
	colorMutedDark   = color.NRGBA{R: 0xA3, G: 0xAC, B: 0xB9, A: 0xFF} // 说明文字：7.4:1
	colorHintLight   = color.NRGBA{R: 0x6B, G: 0x74, B: 0x82, A: 0xFF} // 输入框占位符
	colorHintDark    = color.NRGBA{R: 0x9A, G: 0xA3, B: 0xB0, A: 0xFF}
	colorDisTextLit  = color.NRGBA{R: 0x8A, G: 0x92, B: 0x9E, A: 0xFF} // 真正禁用后仍可辨认
	colorDisTextDark = color.NRGBA{R: 0x6F, G: 0x77, B: 0x84, A: 0xFF}
	colorDisBtnLight = color.NRGBA{R: 0xEC, G: 0xEE, B: 0xF2, A: 0xFF}
	colorDisBtnDark  = color.NRGBA{R: 0x27, G: 0x2B, B: 0x31, A: 0xFF}
)

// 评测判定配色：AC 绿、其余红、缺失琥珀、等待灰。
// 深色主题下要提亮，否则同样的红绿压在暗背景上会发闷。
// 浅色版都调到对白底 4.5:1 以上——这些颜色同时用于报告表格里的文字，
// 太浅的绿/橙在白色面板上读起来很吃力。
var (
	colorAC      = color.NRGBA{R: 0x0F, G: 0x7C, B: 0x38, A: 0xFF} // 5.4:1 on white
	colorFail    = color.NRGBA{R: 0xDC, G: 0x26, B: 0x26, A: 0xFF} // 4.8:1
	colorMissing = color.NRGBA{R: 0xA3, G: 0x55, B: 0x02, A: 0xFF} // 5.0:1
	colorIdle    = color.NRGBA{R: 0x6B, G: 0x72, B: 0x80, A: 0xFF}

	colorACDark      = color.NRGBA{R: 0x2E, G: 0xC2, B: 0x6A, A: 0xFF}
	colorFailDark    = color.NRGBA{R: 0xF2, G: 0x5C, B: 0x5C, A: 0xFF}
	colorMissingDark = color.NRGBA{R: 0xF0, G: 0x9A, B: 0x2E, A: 0xFF}
	colorIdleDark    = color.NRGBA{R: 0x9A, G: 0xA3, B: 0xB0, A: 0xFF}
)

// otkTheme 只覆盖字体、强调色与几个面板色，其余交给 Fyne 默认主题，
// 这样浅色/深色两套配色都能自动跟随系统。
type otkTheme struct {
	base fyne.Theme
}

func newOtkTheme() fyne.Theme {
	return &otkTheme{base: theme.DefaultTheme()}
}

func (t *otkTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return t.base.Font(style)
	}
	// 等宽字体没有中文字形，样例与日志里出现中文就会变豆腐块，所以
	// 统一用内置中文字体。
	return guiFont
}

func (t *otkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *otkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameText:
		return 14
	}
	return t.base.Size(name)
}

func (t *otkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	switch name {
	case theme.ColorNamePrimary:
		if dark {
			return colorAccentDark
		}
		return colorAccentLight
	case theme.ColorNameBackground:
		if dark {
			return colorBgDark
		}
		return colorBgLight
	case theme.ColorNameForeground:
		if dark {
			return colorFgDark
		}
		return colorFgLight
	case theme.ColorNameOverlayBackground:
		if dark {
			return colorPanelDark
		}
		return colorPanelLight
	case theme.ColorNameInputBackground:
		if dark {
			return color.NRGBA{R: 0x23, G: 0x27, B: 0x2D, A: 0xFF}
		}
		return color.NRGBA{R: 0xFC, G: 0xFC, B: 0xFD, A: 0xFF}
	case theme.ColorNameInputBorder:
		if dark {
			return colorLineDark
		}
		return color.NRGBA{R: 0xD4, G: 0xD9, B: 0xE0, A: 0xFF}
	case theme.ColorNamePlaceHolder:
		if dark {
			return colorHintDark
		}
		return colorHintLight
	case theme.ColorNameSeparator:
		if dark {
			return colorLineDark
		}
		return colorLineLight
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x22}
	case theme.ColorNameDisabled:
		// Label 的 LowImportance（caption / kv 的标签列 / 设置里的提示）取这个色。
		// Fyne 默认给的是近乎背景的浅灰，在我们的配色上根本读不出来，
		// 这里换成"次要文字色"：既比正文弱，又有足够对比度。
		if dark {
			return colorMutedDark
		}
		return colorMutedLight
	case theme.ColorNameDisabledButton:
		if dark {
			return colorDisBtnDark
		}
		return colorDisBtnLight
	case theme.ColorNameError, theme.ColorNameForegroundOnError:
		return pickColor(dark, colorFailDark, colorFail)
	case theme.ColorNameSuccess, theme.ColorNameForegroundOnSuccess:
		return pickColor(dark, colorACDark, colorAC)
	case theme.ColorNameWarning, theme.ColorNameForegroundOnWarning:
		return pickColor(dark, colorMissingDark, colorMissing)
	case theme.ColorNameHover:
		// 必须是半透明叠加层：Fyne 的按钮 hover 是把它 blend 到按钮底色上
		// （蓝色主按钮 = 主色 × hover），不透明色会把底色整个盖掉。
		// 品牌蓝浅色版 + 较高透明度：蓝主按钮悬停明显变浅，
		// 白底上呈现清晰的浅蓝高亮。
		if dark {
			return color.NRGBA{R: 0x8F, G: 0xB3, B: 0xFF, A: 0x52}
		}
		return color.NRGBA{R: 0x7C, G: 0xA5, B: 0xF7, A: 0x4A}
	case theme.ColorNamePressed:
		// 按下比悬停更浅。
		if dark {
			return color.NRGBA{R: 0xA8, G: 0xC5, B: 0xFF, A: 0x78}
		}
		return color.NRGBA{R: 0x9B, G: 0xBB, B: 0xFA, A: 0x6E}
	}
	// 基础主题只认"是不是浅色"，其余一律按深色取；而上面的判断反过来，
	// 只认"是不是深色"。遇到两边都不认的值（测试驱动给的就是 2），
	// 就会出现我们画浅底、基础主题配深色按钮的割裂。这里先归一化。
	if variant != theme.VariantDark && variant != theme.VariantLight {
		variant = pickVariant(dark)
	}
	return t.base.Color(name, variant)
}

// pickVariant 把"是否深色"翻回变体常量，供回退到基础主题时使用。
func pickVariant(dark bool) fyne.ThemeVariant {
	if dark {
		return theme.VariantDark
	}
	return theme.VariantLight
}

// pickColor 按主题在深色/浅色两套取值中挑一个。
func pickColor(dark bool, darkColor, lightColor color.Color) color.Color {
	if dark {
		return darkColor
	}
	return lightColor
}

// scoreTone 表示分数的"语义色"，等真正画的时候再按主题取具体 RGB。
// 直接存颜色会导致深色主题下停在浅色版的三原色上，所以先存语义。
type scoreTone int

const (
	toneNone scoreTone = iota // 不加颜色，用正文色
	toneAC
	toneFail
	toneMissing
)

// scoreToneColor 把语义色解析成当前主题下的具体颜色。
func scoreToneColor(t scoreTone, dark bool) color.Color {
	switch t {
	case toneAC:
		return pickColor(dark, colorACDark, colorAC)
	case toneFail:
		return pickColor(dark, colorFailDark, colorFail)
	case toneMissing:
		return pickColor(dark, colorMissingDark, colorMissing)
	}
	return theme.Color(theme.ColorNameForeground)
}

// verdictTone 返回判定对应的徽标颜色。
func verdictTone(verdict string) color.Color {
	return verdictToneFor(verdict, false)
}

// verdictToneFor 按主题返回判定徽标颜色：深色背景下用提亮版，避免发闷。
func verdictToneFor(verdict string, dark bool) color.Color {
	switch verdict {
	case verdictAC, "OK":
		return pickColor(dark, colorACDark, colorAC)
	case verdictMissing:
		return pickColor(dark, colorMissingDark, colorMissing)
	case verdictWA, verdictTLE, verdictMLE, verdictRE:
		return pickColor(dark, colorFailDark, colorFail)
	}
	return pickColor(dark, colorIdleDark, colorIdle)
}
