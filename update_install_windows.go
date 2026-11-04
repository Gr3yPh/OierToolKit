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

//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// finishUpdateInstall 在 Windows 上不能在程序运行时覆盖 exe，所以留下一个
// 批处理：等当前 PID 退出后复制新文件、启动新版，再删除临时目录。
func finishUpdateInstall(dir, _ string, tempDir string, _ map[string]string) error {
	script := filepath.Join(tempDir, "update.bat")
	content := `@echo off
setlocal
:otk_wait
tasklist /FI "PID eq %~1" 2>nul | find "%~1" >nul
if not errorlevel 1 (
  ping 127.0.0.1 -n 2 >nul
  goto otk_wait
)
copy /Y "%~2\otk.exe" "%~3\otk.exe" >nul 2>&1
copy /Y "%~2\otk-gui.exe" "%~3\otk-gui.exe" >nul 2>&1
start "" "%~3\otk-gui.exe"
rmdir /S /Q "%~2" >nul 2>&1
`
	if err := os.WriteFile(script, []byte(content), 0644); err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/c", script,
		strconv.Itoa(os.Getpid()), tempDir, dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start update helper: %w", err)
	}
	return nil
}
