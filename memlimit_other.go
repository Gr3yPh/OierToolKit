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

//go:build !windows

package main

import "os/exec"

// 非 Windows 平台暂不做内存硬限制（需要 cgo 包裹 setrlimit，
// 复杂度高收益低），维持"事后测量再判 MLE"的行为。

// memLimiter 在不支持硬限制的平台上是空实现。
type memLimiter struct{}

func newMemLimiter(memMB float64) *memLimiter { return nil }

func (m *memLimiter) assign(cmd *exec.Cmd) {}

func (m *memLimiter) hit() bool { return false }

func (m *memLimiter) peakMB() (float64, bool) { return 0, false }

func (m *memLimiter) close() {}
