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
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestWheelForwarder(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	scroll := container.NewVScroll(container.NewVBox(
		widget.NewLabel(strings.Repeat("line\n", 100)),
	))
	scroll.Resize(fyne.NewSize(200, 100))
	forwarder := newWheelForwarder(scroll)
	forwarder.Resize(fyne.NewSize(200, 100))
	forwarder.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -50)})
	if scroll.Offset.Y == 0 {
		t.Fatal("wheel event was not forwarded to the target scroll")
	}
}

func TestWheelForwarderCoversNestedScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	rich := widget.NewRichText()
	rich.ParseMarkdown("```\n" + strings.Repeat("long code line ", 40) + "\n```\n\n" + strings.Repeat("text\n\n", 100))
	scroll := container.NewVScroll(rich)
	w := test.NewTempWindow(t, scrollWithWheel(scroll))
	w.Resize(fyne.NewSize(420, 220))
	w.Show()

	// 代码块内部有 HScroll；如果它抢到事件，外层 Offset 不会变化。
	test.Scroll(w.Canvas(), fyne.NewPos(120, 90), 0, -80)
	if scroll.Offset.Y == 0 {
		t.Fatal("nested scrollable swallowed the wheel event")
	}
}

func TestWheelForwarderDoesNotBlockTaps(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	clicked := false
	btn := widget.NewButton("tap", func() { clicked = true })
	w := test.NewTempWindow(t, scrollWithWheel(container.NewVScroll(container.NewVBox(btn))))
	w.Resize(fyne.NewSize(240, 160))
	w.Show()

	test.TapCanvas(w.Canvas(), fyne.NewPos(120, 18))
	if !clicked {
		t.Fatal("wheel overlay blocked a tap on the content")
	}
}
