package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestSheetConfirmCanOpenNextSheet(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	base := widget.NewLabel("base")
	win := test.NewTempWindow(t, base)
	g := &guiApp{win: win}

	g.showSheet(sheetOptions{
		Title:   "first",
		Confirm: "next",
		Dismiss: "cancel",
		Content: widget.NewLabel("first content"),
		OnConfirm: func() {
			g.showSheet(sheetOptions{
				Title:   "second",
				Confirm: "done",
				Dismiss: "cancel",
				Content: widget.NewLabel("second content"),
			})
		},
	})

	firstConfirm := findSheetButton(g.sheet, "next")
	if firstConfirm == nil {
		t.Fatal("first sheet confirm button not found")
	}
	test.Tap(firstConfirm)
	if g.sheet == nil {
		t.Fatal("confirm callback did not leave the next sheet open")
	}

	secondDismiss := findSheetButton(g.sheet, "cancel")
	if secondDismiss == nil {
		t.Fatal("next sheet dismiss button not found")
	}
	test.Tap(secondDismiss)
	if g.sheet != nil || win.Content() != base {
		t.Fatal("dismissing the next sheet did not restore the original window content")
	}
}

func findSheetButton(object fyne.CanvasObject, text string) *widget.Button {
	if button, ok := object.(*widget.Button); ok && button.Text == text {
		return button
	}
	if container, ok := object.(*fyne.Container); ok {
		for _, child := range container.Objects {
			if button := findSheetButton(child, text); button != nil {
				return button
			}
		}
	}
	return nil
}