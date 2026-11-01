package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

// fyneInternalKeys 是 Fyne 控件自己会去查的语言键（对话框按钮、文件选择器、
// 日期控件等），缺了就会在日志里刷 "Translation failure"。
var fyneInternalKeys = []string{
	"(Select one)",
	"Advanced", "Cancel", "Confirm", "Copy", "Create Folder", "Cut",
	"Enter filename", "Error", "Favourites", "File", "Folder",
	"New Folder", "No", "OK", "Open", "Paste", "Quit", "Redo",
	"Save", "Select all", "Show Hidden Files", "Undo", "Yes",
	"label.description",
}

// TestGuiTranslations 确认内置中文词条覆盖了 Fyne 内部会用到的全部 key。
func TestGuiTranslations(t *testing.T) {
	// 语言包本身是纯数据，这部分与系统语言无关，任何机器上都必须成立。
	// 值可能是字符串，也可能是嵌套对象（Fyne 的 "file.name" 这类带点的键
	// 就展开成 {"file":{"name":...}}），所以只取键名、不关心值的类型。
	var bundle map[string]json.RawMessage
	if err := json.Unmarshal(guiZhLangData, &bundle); err != nil {
		t.Fatalf("内置语言包不是合法 JSON：%v", err)
	}
	for _, k := range fyneInternalKeys {
		if _, ok := bundle[k]; !ok {
			t.Errorf("语言包缺少词条：%q", k)
		}
	}

	// 运行时校验只在系统语言是中文时才有意义：Fyne 按 locale 挑词条，
	// 英文机器上 lang.L 会原样返回 key、lang.X 返回 fallback，这不算缺词条。
	// 原先没有这层判断，同一份代码在英文 Linux 上必红、中文 Windows 上常绿。
	installTranslations()
	loc := lang.SystemLocale().String()
	if !strings.HasPrefix(strings.ToLower(loc), "zh") {
		t.Skipf("系统语言为 %s，Fyne 不启用中文词条，跳过运行时校验", loc)
	}

	for _, k := range fyneInternalKeys {
		got := lang.L(k)
		if got == k {
			t.Errorf("词条缺失：%q（仍是英文原文）", k)
			continue
		}
		fmt.Printf("%-20s -> %s\n", k, got)
	}

	if got := lang.X("file.name", "Name"); got != "名称" {
		t.Errorf("file.name 翻译错误：%q", got)
	}
	if got := lang.X("file.parent", "Parent"); got != "上级目录" {
		t.Errorf("file.parent 翻译错误：%q", got)
	}
	if got := lang.X("monday.short", "Mon"); got != "周一" {
		t.Errorf("monday.short 翻译错误：%q", got)
	}
	_ = fyne.Locale("")
}
