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
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// contestsDir 是全部模拟赛的存放目录。
func contestsDir() string { return filepath.Join(baseDir, "contests") }

// contestEntries 列出全部已导入的模拟赛（按导入时间倒序）。
func contestEntries() []contest {
	dirs, err := os.ReadDir(contestsDir())
	if err != nil {
		return nil
	}
	var list []contest
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		c, err := loadContest(filepath.Join(contestsDir(), d.Name()))
		if err != nil {
			continue
		}
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Cfg.ImportedAt != list[j].Cfg.ImportedAt {
			return list[i].Cfg.ImportedAt > list[j].Cfg.ImportedAt
		}
		return list[i].Cfg.Name < list[j].Cfg.Name
	})
	return list
}

// findContest 按名字或目录 id 找一场比赛。
func findContest(key string) (contest, error) {
	for _, c := range contestEntries() {
		if c.Cfg.Name == key || filepath.Base(c.Dir) == key {
			return c, nil
		}
	}
	return contest{}, errNotFound(tr("模拟赛 ") + key)
}

// nowStamp 返回统一格式的时间戳字符串。
func nowStamp() string { return time.Now().Format("2006-01-02 15:04:05") }

// copyDir 递归复制目录（src → dst），保留文件权限。
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target)
	})
}

// copyFile 复制单个文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// errNotFound 统一构造"找不到"错误。
type notFoundError struct{ What string }

func (e notFoundError) Error() string { return tr("找不到") + e.What }

func errNotFound(what string) error { return notFoundError{What: what} }

// atoiDefault 解析整数，失败时返回默认值。
func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
