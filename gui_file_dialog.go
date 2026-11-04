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
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// Fyne 默认文件选择器按最小内容尺寸打开，路径一长文件名一多就显得很窄。
const (
	fileDialogWidth  float32 = 780
	fileDialogHeight float32 = 560
)

func newLargeFileOpen(callback func(fyne.URIReadCloser, error), parent fyne.Window) *dialog.FileDialog {
	d := dialog.NewFileOpen(callback, parent)
	d.Resize(fyne.NewSize(fileDialogWidth, fileDialogHeight))
	return d
}

func newLargeFileSave(callback func(fyne.URIWriteCloser, error), parent fyne.Window) *dialog.FileDialog {
	d := dialog.NewFileSave(callback, parent)
	d.Resize(fyne.NewSize(fileDialogWidth, fileDialogHeight))
	return d
}

func newLargeFolderOpen(callback func(fyne.ListableURI, error), parent fyne.Window) *dialog.FileDialog {
	d := dialog.NewFolderOpen(callback, parent)
	d.Resize(fyne.NewSize(fileDialogWidth, fileDialogHeight))
	return d
}
