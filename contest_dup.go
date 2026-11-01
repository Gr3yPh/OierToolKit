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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// duplicateContest 把一场比赛复制成另一场：contest.json、data/、source/
// 都复制一份，result.json 与 .build/ 不复制——它们绑定本机的编译器与
// CPU 速度，跟着复制过去只会让人误以为新比赛也测过了。
//
// newName 为空时自动取「<原名> 副本」，再撞名就追加序号，做到点一下就有结果。
func duplicateContest(c contest, newName string) (contest, error) {
	if !dirExists(c.Dir) {
		return contest{}, fmt.Errorf(tr("比赛目录不存在: %s"), c.Dir)
	}
	if !dirExists(filepath.Join(c.Dir, "data")) {
		return contest{}, fmt.Errorf("%s", tr("比赛目录里没有 data/"))
	}

	name := strings.TrimSpace(newName)
	auto := name == ""
	if auto {
		name = strings.TrimSpace(c.Cfg.Name)
		if name == "" {
			name = filepath.Base(c.Dir)
		}
		name += " " + tr("副本")
	}

	// 自动取名时一直加到不重名为止；用户自己填的名字撞了就直接报错，
	// 免得悄悄改掉他要的名字。
	base := name
	for i := 2; ; i++ {
		if !dirExists(filepath.Join(contestsDir(), contestID(name))) {
			break
		}
		if !auto {
			return contest{}, fmt.Errorf(tr("已存在同名模拟赛 %s"), name)
		}
		name = fmt.Sprintf("%s %d", base, i)
	}

	dest := filepath.Join(contestsDir(), contestID(name))
	if err := os.MkdirAll(dest, 0755); err != nil {
		return contest{}, fmt.Errorf(tr("创建比赛目录失败: %v"), err)
	}

	for _, sub := range []string{"data", "source"} {
		src := filepath.Join(c.Dir, sub)
		if !dirExists(src) {
			continue
		}
		if err := copyDir(src, filepath.Join(dest, sub)); err != nil {
			_ = os.RemoveAll(dest)
			return contest{}, fmt.Errorf(tr("复制 %s/ 失败: %v"), sub, err)
		}
	}

	cfg := c.Cfg
	cfg.Name = name
	cfg.ImportedAt = nowStamp()
	dup := contest{Dir: dest, Cfg: cfg}
	if err := dup.save(); err != nil {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf(tr("写入比赛配置失败: %v"), err)
	}
	return dup, nil
}

// cmdContestDup 实现 otk contest dup <名称> [新名称]。
func cmdContestDup(args []string) {
	if len(args) == 0 || len(args) > 2 {
		fatal(tr("用法: otk contest dup <名称> [新名称]"))
	}
	c, err := findContest(args[0])
	if err != nil {
		fatal("%v", err)
	}
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	dup, err := duplicateContest(c, name)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf(tr("已复制为 %s（%s）")+"\n", dup.Cfg.Name, dup.Dir)
}
