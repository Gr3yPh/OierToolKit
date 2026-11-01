package main

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

// TestGuiTranslations 确认内置中文词条覆盖了 Fyne 内部会用到的全部 key。
func TestGuiTranslations(t *testing.T) {
	installTranslations()

	keys := []string{
		"(Select one)",
		"Advanced", "Cancel", "Confirm", "Copy", "Create Folder", "Cut",
		"Enter filename", "Error", "Favourites", "File", "Folder",
		"New Folder", "No", "OK", "Open", "Paste", "Quit", "Redo",
		"Save", "Select all", "Show Hidden Files", "Undo", "Yes",
		"label.description",
	}
	for _, k := range keys {
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
