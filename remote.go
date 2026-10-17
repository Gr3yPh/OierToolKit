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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// otk-remote 协议（规范见 PROTOCOL.md）：OJ 侧实现三个 HTTP 端点，
// otk 负责拉题与提交。数据结构与端点一一对应，字段只增不减。

const remoteAPIPrefix = "/api/v1"

var remoteHTTP = &http.Client{Timeout: 15 * time.Second}

type remoteProblem struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	TimeLimitMS   int            `json:"time_limit_ms"`
	MemoryLimitMB float64        `json:"memory_limit_mb"`
	Samples       []remoteSample `json:"samples"`
}

type remoteSample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type remoteSubmission struct {
	Problem  string `json:"problem"`
	Language string `json:"language"`
	Source   string `json:"source"`
}

type remoteSubmissionID struct {
	ID string `json:"id"`
}

type remoteVerdict struct {
	ID       string            `json:"id"`
	Verdict  string            `json:"verdict"`
	Score    int               `json:"score"`
	MaxScore int               `json:"max_score"`
	TimeMS   int64             `json:"time_ms"`
	MemoryKB int64             `json:"memory_kb"`
	Detail   string            `json:"detail"`
	Points   []remoteTestPoint `json:"test_points"`
}

type remoteTestPoint struct {
	Index    int    `json:"index"`
	Verdict  string `json:"verdict"`
	Score    int    `json:"score"`
	TimeMS   int64  `json:"time_ms"`
	MemoryKB int64  `json:"memory_kb"`
	Detail   string `json:"detail"`
}

// remoteToken 取 Bearer token：环境变量优先，其次全局配置。
func remoteToken() string {
	if t := os.Getenv("OTK_TOKEN"); t != "" {
		return t
	}
	return propsOr(readIni(globalConfigPath()), "remote_token", "")
}

// remoteCall 调一次远端接口：自动带 token，非 2xx 报错退出（优先取
// 响应体里的 error 字段），out 非空时把响应体解析进去。
func remoteCall(method, rawURL, token string, body, out any) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			fatal("请求序列化失败: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, rawURL, rd)
	if err != nil {
		fatal("请求地址无效: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := remoteHTTP.Do(req)
	if err != nil {
		fatal("连接远端失败: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		fatal("远端 %s: %s", resp.Status, msg)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			fatal("远端响应解析失败: %v", err)
		}
	}
}

// cmdFetch 从支持 otk-remote 协议的 OJ 拉取题目：时限内存进项目配置，
// 样例落盘入库，并记下远端地址与题号供 submit 使用。
func cmdFetch(args []string) {
	var base, id string
	switch len(args) {
	case 1:
		id = args[0]
		base = propsOr(readIni(globalConfigPath()), "remote", "")
		if base == "" {
			fatal("未配置远程地址：otk config remote <地址>，或直接 otk fetch <地址> <题号>")
		}
	case 2:
		base, id = args[0], args[1]
	default:
		fatal("用法: otk fetch [<地址>] <题号>")
	}
	base = strings.TrimRight(base, "/")

	var prob remoteProblem
	remoteCall("GET", base+remoteAPIPrefix+"/problems/"+url.PathEscape(id), remoteToken(), nil, &prob)
	if prob.ID == "" {
		prob.ID = id
	}

	fmt.Printf("题目 %s", prob.ID)
	if prob.Title != "" {
		fmt.Printf(" %s", prob.Title)
	}
	fmt.Println()
	var parts []string
	if prob.TimeLimitMS > 0 {
		parts = append(parts, fmt.Sprintf("时限 %.2fs", float64(prob.TimeLimitMS)/1000))
	}
	if prob.MemoryLimitMB > 0 {
		parts = append(parts, fmt.Sprintf("内存 %.0fMB", prob.MemoryLimitMB))
	}
	if len(parts) > 0 {
		fmt.Println(strings.Join(parts, "  "))
	}

	p, ok := findProject()
	if !ok {
		cmdInit([]string{prob.ID})
		p = requireProject()
	}
	props := p.cfg()
	if prob.TimeLimitMS > 0 {
		props["time_limit"] = strconv.FormatFloat(float64(prob.TimeLimitMS)/1000, 'f', 2, 64)
	}
	if prob.MemoryLimitMB > 0 {
		props["memory_limit"] = strconv.FormatFloat(prob.MemoryLimitMB, 'f', -1, 64)
	}
	props["remote_base"] = base
	props["remote_id"] = prob.ID
	writeIni(p.configPath(), props)

	for i, s := range prob.Samples {
		_ = os.WriteFile(filepath.Join(p.Dir, strconv.Itoa(i+1)+".in"), []byte(s.Input), 0644)
		_ = os.WriteFile(filepath.Join(p.Dir, strconv.Itoa(i+1)+".out"), []byte(s.Output), 0644)
	}
	fmt.Printf("样例 %d 组，远端 %s\n", len(prob.Samples), base)
	cmdRefresh()
}

// cmdSubmit 把当前项目源码提交到远端评测。默认每 2 秒轮询到终态
// verdict（120 秒超时），-q 只提交拿提交号。
func cmdSubmit(args []string) {
	wait := true
	for _, a := range args {
		switch a {
		case "-q", "--quiet":
			wait = false
		default:
			fatal("用法: otk submit [-q]")
		}
	}

	p := requireProject()
	if !fileExists(p.sourcePath()) {
		fatal("未找到 %s", p.codeName()+".cpp")
	}
	props := p.cfg()
	base := strings.TrimRight(props["remote_base"], "/")
	if base == "" {
		fatal("该项目未配置远端（otk fetch 建的项目会自动配置；也可 otk config remote_base <地址> 与 otk config remote_id <题号>）")
	}
	probID := props["remote_id"]
	if probID == "" {
		fatal("该项目未配置远端题号（otk config remote_id <题号>）")
	}
	src, err := os.ReadFile(p.sourcePath())
	if err != nil {
		fatal("读取源码失败: %v", err)
	}

	sub := remoteSubmission{Problem: probID, Language: "cpp", Source: string(src)}
	var got remoteSubmissionID
	remoteCall("POST", base+remoteAPIPrefix+"/submissions", remoteToken(), sub, &got)
	if got.ID == "" {
		fatal("远端未返回提交号")
	}
	fmt.Printf("已提交 %s（%s @ %s）\n", got.ID, probID, base)
	if !wait {
		return
	}

	deadline := time.Now().Add(120 * time.Second)
	last := "（无）"
	for {
		time.Sleep(2 * time.Second)
		var v remoteVerdict
		remoteCall("GET", base+remoteAPIPrefix+"/submissions/"+url.PathEscape(got.ID), remoteToken(), nil, &v)
		verd := strings.ToUpper(strings.TrimSpace(v.Verdict))
		if verd != "" {
			last = verd
		}
		switch verd {
		case "PENDING", "JUDGING", "":
			if time.Now().After(deadline) {
				fmt.Printf("等待超时（最后状态 %s），提交号 %s，可到 OJ 页面查看\n", last, got.ID)
				otkExit(1)
			}
			continue
		}
		// 总体 verdict（AC 绿字其余红字）：计分 OJ 附总分，随后逐测试点明细。
		color := red
		if verd == "AC" {
			color = green
		}
		fmt.Printf("%s%s%s", color, verd, reset)
		if v.MaxScore > 0 || v.Score > 0 {
			fmt.Printf("  %d/%d分", v.Score, v.MaxScore)
		}
		if v.TimeMS > 0 || v.MemoryKB > 0 {
			fmt.Printf("  %dms %dKB", v.TimeMS, v.MemoryKB)
		}
		fmt.Println()
		printRemotePoints(v.Points, v.MaxScore > 0)
		if s := strings.TrimSpace(v.Detail); s != "" {
			printIndented(firstLines(s, 12), "  ")
		}
		if verd != "AC" {
			otkExit(1)
		}
		return
	}
}

// printRemotePoints 逐行打印测试点明细，行首 #i 风格与本地 judge 一致。
func printRemotePoints(points []remoteTestPoint, scored bool) {
	width := 1
	for _, t := range points {
		if n := len(strconv.Itoa(t.Index)); n > width {
			width = n
		}
	}
	for _, t := range points {
		verd := strings.ToUpper(strings.TrimSpace(t.Verdict))
		color := red
		if verd == "AC" {
			color = green
		}
		fmt.Printf("  #%-*s %s%-4s%s %dms %dKB", width, strconv.Itoa(t.Index), color, verd, reset, t.TimeMS, t.MemoryKB)
		if scored {
			fmt.Printf("  %d分", t.Score)
		}
		fmt.Println()
		if s := strings.TrimSpace(t.Detail); s != "" {
			printIndented(firstLines(s, 6), "    ")
		}
	}
}
