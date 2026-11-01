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
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// cmdContest 是模拟赛的总入口：
//
//	otk contest add <上传目录|数据包.zip> [名称]  导入一场比赛
//	otk contest export <名称> [zip 路径]        打包成数据包
//	otk contest dup <名称> [新名称]             复制一场比赛
//	otk contest ls                              列出全部比赛
//	otk contest judge <名称>                    评测并生成 result.json
//	otk contest rank [-w <文件>] <名称>         打印榜单（-w 导出 HTML）
//	otk contest rm <名称>                       删除比赛
func cmdContest(args []string) {
	if len(args) == 0 {
		printContestHelp()
		otkExit(1)
	}

	switch args[0] {
	case "add", "import":
		cmdContestAdd(args[1:])
	case "export", "pack":
		cmdContestExport(args[1:])
	case "dup", "copy":
		cmdContestDup(args[1:])
	case "ls", "list":
		cmdContestList()
	case "judge", "j":
		cmdContestJudge(args[1:])
	case "rank", "r":
		cmdContestRank(args[1:])
	case "rm", "remove", "delete":
		cmdContestRemove(args[1:])
	case "help", "-h", "--help":
		printContestHelp()
	default:
		// 省略子命令时把参数当比赛名，直接评测——最常用的操作。
		cmdContestJudge(args)
	}
}

func printContestHelp() {
	fmt.Print(`用法: otk contest <子命令> [参数]

  add <上传目录> [名称]   导入一整场模拟赛（data/ + source/）
  add <数据包.zip> [名称] 从 zip 数据包导入（保留时限、子任务、题面）
  export <名称> [zip]     打包成 zip 数据包，默认写到比赛目录下
  dup <名称> [新名称]     复制一场比赛（不含评测结果）
  ls                      列出已导入的比赛
  judge [名称]            评测整场比赛，结果写入比赛目录的 result.json
  rank [名称] [-w 文件]   查看榜单；-w 导出 HTML（默认比赛目录下）
  rm <名称>               删除比赛及其数据

上传目录结构:
  <目录>/data/<题目名>/<测试点>.in
  <目录>/data/<题目名>/<测试点>.ans（或 .out）
  <目录>/source/<选手名>/<题目名>.cpp
`)
}

// isContestArchive 判断路径是不是 zip 数据包。
func isContestArchive(p string) bool {
	return strings.EqualFold(filepath.Ext(p), ".zip")
}

// cmdContestAdd 导入比赛。
func cmdContestAdd(args []string) {
	if len(args) == 0 || len(args) > 2 {
		fatal(tr("用法: otk contest add <上传目录> [名称]"))
	}
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	fmt.Printf(tr("正在导入 %s ...")+"\n", args[0])
	var (
		c   contest
		err error
	)
	if isContestArchive(args[0]) {
		c, err = importContestArchive(args[0], name)
	} else {
		c, err = importContest(args[0], name)
	}
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("%s"+tr("已导入 %s")+"%s（%s）\n", green, c.Cfg.Name, reset, c.Dir)
	fmt.Printf("  "+tr("%d 道题 · %d 位选手 · %d 份提交")+"\n",
		len(c.Cfg.Problems), len(c.Cfg.Players), len(c.Cfg.Sources))
	for _, p := range c.Cfg.Problems {
		fmt.Printf("  · "+tr("%s：%d 个测试点，满分 %.0f")+"\n", p.Name, len(p.Tests), fullScoreOf(c, p.Name))
	}
	for _, w := range validateContest(c) {
		fmt.Printf("%s! %s%s\n", yellow, w, reset)
	}
}

// cmdContestExport 把一场比赛打包成 zip 数据包。
func cmdContestExport(args []string) {
	if len(args) == 0 || len(args) > 2 {
		fatal(tr("用法: otk contest export <名称> [zip 路径]"))
	}
	c, err := resolveContest(args[0])
	if err != nil {
		fatal("%v", err)
	}
	out := ""
	if len(args) == 2 {
		out = args[1]
	}
	fmt.Printf(tr("正在打包 %s ...")+"\n", c.Cfg.Name)
	path, err := exportContestArchive(c, out)
	if err != nil {
		fatal(tr("导出失败: %v"), err)
	}
	size := int64(0)
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	fmt.Printf("%s"+tr("已导出数据包 %s")+"%s（%s）\n", green, path, reset, humanSize(int(size)))
	fmt.Printf(tr("对方用 otk contest add %s 或在界面里「导入 ZIP」即可打开。")+"\n", filepath.Base(path))
}

// cmdContestList 列出比赛。
func cmdContestList() {
	list := contestEntries()
	if len(list) == 0 {
		fmt.Println(tr("还没有导入任何模拟赛。用 otk contest add <上传目录> 导入。"))
		return
	}
	for _, c := range list {
		marked := " "
		if fileExists(c.resultPath()) {
			marked = green + "*" + reset
		}
		fmt.Printf("%s %s%s%s  "+tr("%d 题 · %d 位选手 · 导入于 %s")+"\n",
			marked, cyan, c.Cfg.Name, reset,
			len(c.Cfg.Problems), len(c.Cfg.Players), c.Cfg.ImportedAt)
	}
	fmt.Printf("\n%s*%s "+tr("表示已评测过")+"\n", green, reset)
}

// cmdContestJudge 评测整场比赛。
func cmdContestJudge(args []string) {
	name := ""
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fatal(tr("用法: otk contest judge [名称]"))
		}
		name = a
	}
	c, err := resolveContest(name)
	if err != nil {
		fatal("%v", err)
	}

	var stopped atomic.Bool
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		stopped.Store(true)
	}()

	fmt.Printf(tr("评测 %s（%d 道题 · %d 份提交 · %d 并发）")+"\n",
		c.Cfg.Name, len(c.Cfg.Problems), len(c.Cfg.Sources), contestParallelism())

	var mu = make(chan struct{}, 1) // 串行化输出，避免多行交错
	res, err := judgeContest(c, func(ev judgeEvent) {
		if ev.Kind != judgeDone {
			return
		}
		mu <- struct{}{}
		fmt.Printf("  [%3.0f%%] %s\n", ev.Percent*100, ev.Message)
		<-mu
	}, stopped.Load)
	if err != nil {
		fatal("%v", err)
	}
	if stopped.Load() {
		fmt.Println(yellow + tr("评测被中断，结果未保存") + reset)
		otkExit(130)
	}

	if err := saveContestResult(c, res); err != nil {
		fatal(tr("保存结果失败: %v"), err)
	}
	fmt.Printf("%s"+tr("评测完成，结果已保存")+"%s\n\n", green, reset)
	printRanking(c, res)
}

// cmdContestRank 打印榜单，可选导出 HTML。
func cmdContestRank(args []string) {
	name := ""
	outPath := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-w" || a == "--write" || a == "-o" || a == "--output":
			if i+1 >= len(args) {
				fatal(tr("用法: otk contest rank [名称] [-w 文件]"))
			}
			outPath = args[i+1]
			i++
		case strings.HasPrefix(a, "-"):
			fatal(tr("用法: otk contest rank [名称] [-w 文件]"))
		default:
			name = a
		}
	}

	c, err := resolveContest(name)
	if err != nil {
		fatal("%v", err)
	}
	res, ok := loadContestResult(c)
	if !ok {
		fatal(tr("%s 还没有评测结果，先运行 otk contest judge %s"), c.Cfg.Name, c.Cfg.Name)
	}

	printRanking(c, res)
	if outPath != "" {
		path, err := exportContestHTML(c, res, outPath)
		if err != nil {
			fatal(tr("导出失败: %v"), err)
		}
		fmt.Printf("\n%s"+tr("已导出榜单：")+"%s%s\n", green, path, reset)
	}
}

// cmdContestRemove 删除比赛。
func cmdContestRemove(args []string) {
	if len(args) != 1 {
		fatal(tr("用法: otk contest rm <名称>"))
	}
	c, err := resolveContest(args[0])
	if err != nil {
		fatal("%v", err)
	}
	if err := os.RemoveAll(c.Dir); err != nil {
		fatal(tr("删除失败: %v"), err)
	}
	fmt.Printf(tr("已删除 %s")+"%s%s\n", cyan, c.Cfg.Name, reset)
}

// resolveContest 解析比赛名：留空时仅当只有一场比赛才自动选中。
func resolveContest(name string) (contest, error) {
	if name != "" {
		return findContest(name)
	}
	list := contestEntries()
	switch len(list) {
	case 0:
		return contest{}, fmt.Errorf("%s", tr("还没有导入任何模拟赛，用 otk contest add <上传目录>"))
	case 1:
		return list[0], nil
	default:
		var names []string
		for _, c := range list {
			names = append(names, c.Cfg.Name)
		}
		return contest{}, fmt.Errorf(tr("有多场比赛，请指定名称：%s"), strings.Join(names, " / "))
	}
}

// printRanking 以表格形式打印榜单。
func printRanking(c contest, res contestResult) {
	rows := buildRanking(c, res)
	if len(rows) == 0 {
		fmt.Println(tr("榜单为空"))
		return
	}

	nameWidth := 4
	for _, r := range rows {
		if w := displayWidth(r.Player); w > nameWidth {
			nameWidth = w
		}
	}
	probWidths := make([]int, len(res.Problems))
	for i, p := range res.Problems {
		probWidths[i] = max(displayWidth(p), 6)
	}

	// 表头
	fmt.Printf("%-4s %-*s", tr("名次"), nameWidth+2, tr("选手"))
	for i, p := range res.Problems {
		fmt.Printf(" %*s", probWidths[i], p)
	}
	fmt.Printf(" %6s\n", tr("总分"))

	for _, r := range rows {
		fmt.Printf("%-4d %-*s", r.Rank, nameWidth+2, r.Player)
		for i, s := range r.Scores {
			cell := fmtScore(s)
			color := ""
			if full := fullScoreOf(c, res.Problems[i]); full > 0 && s >= full-1e-9 {
				color = green
			} else if s == 0 {
				color = red
			}
			fmt.Printf(" %s%*s%s", color, probWidths[i], cell, reset)
		}
		fmt.Printf(" %s%6s%s\n", cyan, fmtScore(r.Total), reset)
	}

	fmt.Printf("\n"+tr("%d 位选手 · %d 道题 · 满分 %.0f 分")+"\n", len(rows), len(res.Problems), totalFullScore(c))
}

// displayWidth 粗略估算终端显示宽度：CJK 字符按 2 列。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x1100 && (r <= 0x115f || (r >= 0x2e80 && r <= 0xa4cf) ||
			(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
			(r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffe6)) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// max 返回较大值。
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
