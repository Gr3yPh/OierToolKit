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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCloneProblemImportsTitleSiteAndStatement(t *testing.T) {
	useTempBaseDir(t)

	statement := "# 有偿参观\n\n输出两个整数之和。\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case "/otk/v1/problems/P1007":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":              "P1007",
				"title":           "有偿参观",
				"site_name":       "NLOJ",
				"statement":       statement,
				"time_limit_ms":   1500,
				"memory_limit_mb": 256,
				"samples": []map[string]string{
					{"input": "3 5\n", "output": "8\n"},
				},
			})
		case "/otk/v1/problems/P2":
			// 旧服务端只回题号：客户端应回退成题号标题和一个标题题面。
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "P2"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	parent := t.TempDir()
	p, prob, _, err := cloneProblem(parent, srv.URL, "P1007")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "有偿参观" {
		t.Fatalf("项目显示名 = %q", p.Name)
	}
	if prob.SiteName != "NLOJ" || prob.Statement != statement {
		t.Fatalf("远端字段没有解析完整: %+v", prob)
	}

	props := p.cfg()
	if props["name"] != "有偿参观" {
		t.Fatalf("配置里的 name = %q", props["name"])
	}
	if props["remote_site"] != "NLOJ" {
		t.Fatalf("配置里的 remote_site = %q", props["remote_site"])
	}
	if props["remote_base"] != srv.URL || props["remote_id"] != "P1007" {
		t.Fatalf("远端配置 = %q / %q", props["remote_base"], props["remote_id"])
	}
	if props["time_limit"] != "1.50" || props["memory_limit"] != "256" {
		t.Fatalf("限制 = %q s / %q MB", props["time_limit"], props["memory_limit"])
	}

	data, err := os.ReadFile(filepath.Join(p.Dir, "problem.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != statement {
		t.Fatalf("problem.md = %q", data)
	}
	if data, err := os.ReadFile(filepath.Join(p.Dir, "1.in")); err != nil || string(data) != "3 5\n" {
		t.Fatalf("1.in = %q, err = %v", data, err)
	}

	p2, _, _, err := cloneProblem(t.TempDir(), srv.URL, "P2")
	if err != nil {
		t.Fatal(err)
	}
	if p2.Name != "P2" {
		t.Fatalf("无标题时项目名 = %q", p2.Name)
	}
	if data, err := os.ReadFile(filepath.Join(p2.Dir, "problem.md")); err != nil || string(data) != "# P2\n" {
		t.Fatalf("无题面时 problem.md = %q, err = %v", data, err)
	}
}
