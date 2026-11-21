package main

import (
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func TestSidebarSplitIsResizable(t *testing.T) {
	split, ok := sidebarSplit(widget.NewLabel("sidebar"), widget.NewLabel("content")).(*container.Split)
	if !ok {
		t.Fatal("sidebar split is not a resizable Fyne split container")
	}
	if split.Offset != 0.24 {
		t.Fatalf("initial sidebar offset = %v, want 0.24", split.Offset)
	}
}
