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
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 项目数据包（.zip）用来把一个项目的全部可编辑内容搬走或存档：
//
//	<目录名>.cpp            源码（checker.cpp、题面等其它文件也一并带上）
//	*.in / *.out            散在项目目录里的样例数据
//	.otk/config             时限、内存、编译器、文件读写等配置
//	.otk/samples            样例索引
//	.otk/history.json       评测历史
//
// 不打包编译产物：可执行文件、.otk/checker.exe、.build/ 都是本机现编的，
// 换台机器直接重编即可，带过去只是几百 KB 到几 MB 的废数据。
//
// 包内是扁平结构（根下就是项目内容），与模拟赛数据包保持一致：
// 解压出来就是一个能直接「新建项目」指向的目录。

// projectZipSkipDir 判断整个目录是否跳过。
func projectZipSkipDir(rel string) bool {
	switch rel {
	case ".git", ".build", ".svn", ".hg":
		// .git 可能有几百 MB 且与项目内容无关；.build 是评测时的编译产物。
		return true
	}
	return false
}

// projectZipSkipFile 判断单个文件是否跳过。
func projectZipSkipFile(p project, rel string) bool {
	lower := strings.ToLower(rel)
	// 上一次导出的包、写了一半的临时文件：套娃打进去只会让人困惑。
	if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".part") {
		return true
	}
	// 编译产物：项目可执行文件与 checker。
	if rel == relOfDir(p.Dir, p.exePath()) || rel == relOfDir(p.Dir, checkerExePath(p)) {
		return true
	}
	switch lower {
	case ".ds_store", "thumbs.db":
		return true
	}
	return false
}

// relOfDir 返回 path 相对 dir 的斜杠路径；算不出来时返回原值。
func relOfDir(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// exportProjectArchive 把项目打包成 zip，返回实际写入的路径。
// zipPath 为空时写到项目目录下，与比赛数据包的习惯保持一致。
func exportProjectArchive(p project, zipPath string) (string, error) {
	if !dirExists(p.Dir) {
		return "", fmt.Errorf(tr("项目目录不存在: %s"), p.Dir)
	}
	if zipPath == "" {
		zipPath = filepath.Join(p.Dir, sanitizeFileName(p.Name)+".zip")
	}
	if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
		zipPath += ".zip"
	}
	if info, err := os.Stat(zipPath); err == nil && info.IsDir() {
		return "", fmt.Errorf(tr("导出路径是个目录: %s"), zipPath)
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return "", fmt.Errorf(tr("创建导出目录失败: %v"), err)
	}

	// 先写 .part 再改名：中途失败不会留下一个看着正常、其实缺文件的包。
	tmp := zipPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf(tr("创建压缩包失败: %v"), err)
	}
	zw := zip.NewWriter(f)

	count := 0
	walkErr := filepath.Walk(p.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel := relOfDir(p.Dir, path)
		if info.IsDir() {
			if rel != "." && projectZipSkipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || projectZipSkipFile(p, rel) {
			return nil
		}
		hdr := &zip.FileHeader{Name: rel, Method: zip.Deflate, Modified: info.ModTime()}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, cerr := io.Copy(w, in)
		in.Close()
		if cerr != nil {
			return cerr
		}
		count++
		return nil
	})

	if walkErr == nil {
		walkErr = zw.Close()
	} else {
		_ = zw.Close()
	}
	if cerr := f.Close(); walkErr == nil && cerr != nil {
		walkErr = cerr
	}
	if walkErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf(tr("打包失败: %v"), walkErr)
	}
	if count == 0 {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("%s", tr("这个项目没有可打包的文件"))
	}
	if err := os.Rename(tmp, zipPath); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf(tr("写入压缩包失败: %v"), err)
	}
	return zipPath, nil
}

// cmdPack 实现 otk pack [目录] [输出.zip]。
func cmdPack(args []string) {
	if len(args) > 2 {
		fatal(tr("用法: otk pack [项目目录] [输出.zip]"))
	}
	dir := ""
	if len(args) > 0 {
		dir = args[0]
	}
	p, err := projectAt(dir)
	if err != nil {
		fatal("%v", err)
	}
	out := ""
	if len(args) == 2 {
		out = args[1]
	}
	path, err := exportProjectArchive(p, out)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println(tr("项目数据包已写入: ") + path)
}

// projectAt 取指定目录的项目；dir 为空时按当前目录向上找项目根。
func projectAt(dir string) (project, error) {
	if dir == "" {
		p, ok := findProject()
		if !ok {
			return project{}, fmt.Errorf("%s", tr("当前位置不是项目目录（找不到 .otk/ 标记）"))
		}
		return p, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return project{}, err
	}
	if !isProjectRoot(abs) {
		return project{}, fmt.Errorf(tr("%s 不是项目目录（缺少 .otk/ 标记）"), abs)
	}
	return project{Name: projectNameOf(abs), Dir: abs}, nil
}
