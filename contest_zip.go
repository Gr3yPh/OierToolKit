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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// 模拟赛数据包（.zip）用来把一整场比赛搬到另一台机器：结构跟上传目录
// 一致，只多一个 contest.json——它带着时限、子任务、题面、选手与提交
// 清单，所以对方导入后不用再手填一遍配置。
//
//	contest.json
//	data/<题目>/<测试点>.in 与 .ans（或 .out）
//	source/<选手>/<题目>.cpp
//
// 故意不打包 result.json：评测结果跟本机的编译器与 CPU 速度绑定，搬过去
// 只会让人误以为已经测过，重新评测才是对的。.build/ 这类编译产物同样跳过。
const (
	// contestArchiveMaxBytes 限制单个数据包解压后的总大小，挡 zip 炸弹。
	contestArchiveMaxBytes = int64(8) << 30
	// contestArchiveMaxFiles 限制条目数，避免海量小文件把磁盘写满。
	contestArchiveMaxFiles = 200000
)

// exportContestArchive 把一场比赛打包成 zip，返回实际写入的路径。
// zipPath 为空时写到比赛目录下，与别的导出（HTML/CSV）保持一致。
func exportContestArchive(c contest, zipPath string) (string, error) {
	if zipPath == "" {
		zipPath = filepath.Join(c.Dir, sanitizeFileName(c.Cfg.Name)+".zip")
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

	// 先写 .part 再改名：中途失败（磁盘满、被占用）不会留下一个
	// 看起来正常、其实缺文件的包。
	tmp := zipPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf(tr("创建压缩包失败: %v"), err)
	}
	zw := zip.NewWriter(f)

	count := 0
	walkErr := filepath.Walk(c.Dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(c.Dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			// .build 是评测时的编译产物，几百 MB 且可重建。
			if rel == ".build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// result.json 是本机评测的产物，重新导出的旧包也不该被套娃打进来。
		if rel == "result.json" || strings.HasSuffix(rel, ".zip") || strings.HasSuffix(rel, ".part") {
			return nil
		}

		hdr := &zip.FileHeader{Name: rel, Method: zip.Deflate, Modified: info.ModTime()}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
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
		return "", fmt.Errorf("%s", tr("这场比赛没有可打包的文件"))
	}
	if err := os.Rename(tmp, zipPath); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf(tr("写入压缩包失败: %v"), err)
	}
	return zipPath, nil
}

// importContestArchive 从 .zip 数据包导入一场比赛。
//
// 包里带 contest.json 时直接用它还原配置（保留时限、子任务、题面、选手
// 与提交清单）；没带的话就当成普通上传目录，走 importContest 重新扫描。
func importContestArchive(zipPath, name string) (contest, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return contest{}, fmt.Errorf(tr("打不开数据包: %v"), err)
	}
	defer zr.Close()

	prefix := archiveRootPrefix(zr.File)
	if err := os.MkdirAll(contestsDir(), 0755); err != nil {
		return contest{}, fmt.Errorf(tr("创建比赛目录失败: %v"), err)
	}
	tmp, err := os.MkdirTemp(contestsDir(), ".unzip-")
	if err != nil {
		return contest{}, fmt.Errorf(tr("创建临时目录失败: %v"), err)
	}
	defer os.RemoveAll(tmp)

	if err := extractContestArchive(zr.File, prefix, tmp); err != nil {
		return contest{}, err
	}

	if fileExists(filepath.Join(tmp, contestConfigName)) {
		return restoreContestFromArchive(tmp, zipPath, name)
	}

	// 没有 contest.json：当成上传目录，重新扫描测试点与源码。
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}
	c, err := importContest(tmp, name)
	if err != nil {
		return contest{}, err
	}
	// importContest 记的是临时目录，那个目录马上就被删了，改记包路径。
	c.Cfg.SourceDir = zipPath
	if err := c.save(); err != nil {
		_ = os.RemoveAll(c.Dir)
		return contest{}, fmt.Errorf(tr("写入比赛配置失败: %v"), err)
	}
	return c, nil
}

// restoreContestFromArchive 按包里的 contest.json 还原一场比赛。
func restoreContestFromArchive(tmp, zipPath, name string) (contest, error) {
	raw, err := os.ReadFile(filepath.Join(tmp, contestConfigName))
	if err != nil {
		return contest{}, fmt.Errorf(tr("读取数据包里的比赛配置失败: %v"), err)
	}
	var cfg contestConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return contest{}, fmt.Errorf(tr("数据包里的比赛配置损坏: %v"), err)
	}
	if name == "" {
		name = strings.TrimSpace(cfg.Name)
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}

	dest := filepath.Join(contestsDir(), contestID(name))
	if dirExists(dest) {
		return contest{}, fmt.Errorf(tr("已存在同名模拟赛 %s（%s）"), name, dest)
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return contest{}, fmt.Errorf(tr("创建比赛目录失败: %v"), err)
	}
	for _, sub := range []string{"data", "source"} {
		src := filepath.Join(tmp, sub)
		if !dirExists(src) {
			continue
		}
		if err := copyDir(src, filepath.Join(dest, sub)); err != nil {
			_ = os.RemoveAll(dest)
			return contest{}, fmt.Errorf(tr("解压 %s/ 失败: %v"), sub, err)
		}
	}
	if !dirExists(filepath.Join(dest, "data")) {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf("%s", tr("数据包里没有 data/ 文件夹"))
	}

	cfg.Name = name
	cfg.ImportedAt = nowStamp()
	cfg.SourceDir = zipPath
	c := contest{Dir: dest, Cfg: cfg}
	if err := c.save(); err != nil {
		_ = os.RemoveAll(dest)
		return contest{}, fmt.Errorf(tr("写入比赛配置失败: %v"), err)
	}
	return c, nil
}

// extractContestArchive 把包内与比赛相关的条目解到 dst。
// 只取 contest.json、data/、source/，其余（__MACOSX、.DS_Store 之类）忽略。
func extractContestArchive(files []*zip.File, prefix, dst string) error {
	var total int64
	count := 0
	for _, f := range files {
		rel, ok := archiveEntryPath(f.Name)
		if !ok {
			return fmt.Errorf(tr("数据包里有非法路径: %s"), f.Name)
		}
		if prefix != "" {
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			rel = strings.TrimPrefix(rel, prefix)
		}
		if rel == "" || rel == "." {
			continue
		}
		if rel != contestConfigName && !strings.HasPrefix(rel, "data/") && !strings.HasPrefix(rel, "source/") {
			continue
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(filepath.Join(dst, filepath.FromSlash(rel)), 0755); err != nil {
				return fmt.Errorf(tr("解压失败: %v"), err)
			}
			continue
		}

		count++
		if count > contestArchiveMaxFiles {
			return fmt.Errorf("%s", tr("数据包条目过多，已中止导入"))
		}
		declared := int64(f.UncompressedSize64)
		total += declared
		if declared < 0 || total > contestArchiveMaxBytes {
			return fmt.Errorf(tr("数据包过大（解压后超过 %d GB）"), contestArchiveMaxBytes>>30)
		}

		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf(tr("解压失败: %v"), err)
		}
		if err := extractZipFile(f, target, declared); err != nil {
			return err
		}
	}
	return nil
}

// extractZipFile 解出一个文件，并核对实际字节数不超过声明值——
// 声明值来自包本身，骗人的头会让解压量失控。
func extractZipFile(f *zip.File, target string, declared int64) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf(tr("解压失败: %v"), err)
	}
	defer rc.Close()

	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf(tr("解压失败: %v"), err)
	}
	written, err := io.Copy(out, io.LimitReader(rc, declared+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(target)
		return fmt.Errorf(tr("解压失败: %v"), err)
	}
	if written > declared {
		_ = os.Remove(target)
		return fmt.Errorf(tr("数据包内容与声明大小不符: %s"), f.Name)
	}
	return nil
}

// archiveRootPrefix 找出比赛内容在包里的位置，返回前缀（"" 或 "xxx/"）。
//
// 两种打包方式都认：三样东西直接在包根，或被包进一个顶层目录
// （Windows 里选中文件夹「压缩到 zip」就是这个样子）。
func archiveRootPrefix(files []*zip.File) string {
	best := ""
	found := false
	for _, f := range files {
		rel, ok := archiveEntryPath(f.Name)
		if !ok {
			continue
		}
		if rel != contestConfigName && !strings.HasSuffix(rel, "/"+contestConfigName) {
			continue
		}
		p := strings.TrimSuffix(rel, contestConfigName)
		if !found || len(p) < len(best) {
			best, found = p, true
		}
	}
	if found {
		return best
	}

	// 没有 contest.json：退而求其次，找 data/ 所在的那一层。
	for _, f := range files {
		rel, ok := archiveEntryPath(f.Name)
		if !ok {
			continue
		}
		segs := strings.Split(rel, "/")
		for i, seg := range segs {
			if seg != "data" {
				continue
			}
			p := strings.Join(segs[:i], "/")
			if p != "" {
				p += "/"
			}
			if best == "" || len(p) < len(best) {
				best = p
			}
			break
		}
	}
	return best
}

// archiveEntryPath 规范化包内路径，并挡掉越界条目（zip slip）：
// 绝对路径、盘符、以及带 .. 的路径一律拒绝。
func archiveEntryPath(name string) (string, bool) {
	n := strings.ReplaceAll(name, "\\", "/")
	n = strings.TrimPrefix(n, "./")
	clean := path.Clean(n)
	if clean == "." {
		return "", true
	}
	if path.IsAbs(clean) || strings.HasPrefix(clean, "/") {
		return "", false
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	if len(clean) >= 2 && clean[1] == ':' {
		return "", false
	}
	return clean, true
}
