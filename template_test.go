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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinTemplatesInit 逐个内置模板建项目，检查源码、配置与附加文件。
func TestBuiltinTemplatesInit(t *testing.T) {
	useTempBaseDir(t)

	for _, tmpl := range builtinTemplates() {
		tmpl := tmpl
		t.Run(tmpl.ID, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "题目A")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			p, _, created, err := initProjectAtWith(dir, "题目A", &tmpl)
			if err != nil {
				t.Fatalf("init: %v", err)
			}
			if !created {
				t.Fatal("新建目录应当创建配置")
			}

			src, err := os.ReadFile(p.sourcePath())
			if err != nil {
				t.Fatalf("读源码: %v", err)
			}
			if strings.Contains(string(src), templateNameTag) {
				t.Errorf("源码里还留着 %s 占位符", templateNameTag)
			}
			if tmpl.ID == "fileio" && !strings.Contains(string(src), `freopen("题目A.in"`) {
				t.Errorf("文件读写模板应把占位符换成目录名，实际:\n%s", src)
			}

			props := p.cfg()
			for k, v := range tmpl.Props {
				if k == "io_in" || k == "io_out" {
					v = tmpl.render(v, "题目A")
				}
				if props[k] != v {
					t.Errorf("配置 %s = %q，期望 %q", k, props[k], v)
				}
			}

			for fname := range tmpl.Files {
				if !fileExists(filepath.Join(dir, fname)) {
					t.Errorf("缺少附加文件 %s", fname)
				}
			}

			// 模板样例要能被 syncSamples 登记进来。
			if n := len(tmpl.Samples); n > 0 {
				if got := len(readIndex(p)); got != n {
					t.Errorf("登记样例 %d 组，期望 %d 组", got, n)
				}
			}
		})
	}
}

// TestTemplateDoesNotOverwrite 已存在的文件不能被模板覆盖。
func TestTemplateDoesNotOverwrite(t *testing.T) {
	useTempBaseDir(t)
	dir := t.TempDir()

	// 先建一个空白项目并改写源码、把时限调成 3 秒。
	p, _, _, err := initProjectAt(dir, "旧题")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.sourcePath(), []byte("// 我改过的源码\n"), 0644); err != nil {
		t.Fatal(err)
	}
	props := p.cfg()
	props["time_limit"] = "3.00"
	writeIni(p.configPath(), props)

	stress, _ := findTemplate("stress")
	if _, _, _, err := initProjectAtWith(dir, "旧题", &stress); err != nil {
		t.Fatal(err)
	}

	src, _ := os.ReadFile(p.sourcePath())
	if string(src) != "// 我改过的源码\n" {
		t.Errorf("源码被覆盖了: %q", src)
	}
	if v := p.cfg()["time_limit"]; v != "3.00" {
		t.Errorf("时限被覆盖成 %q", v)
	}
	if fileExists(filepath.Join(dir, "gen.cpp")) {
		// 附加文件按"不存在才写"处理：旧项目本来没有 gen.cpp，补上是对的。
		// 断言这一点，防止以后误改成"对已有项目完全不处理附加文件"。
		t.Log("gen.cpp 已生成（旧项目缺这个文件，符合预期）")
	} else {
		t.Error("旧项目缺少的 gen.cpp 应当被补上")
	}
}

// TestSaveTemplateRoundTrip 项目另存为模板后，能被再次用来建项目。
func TestSaveTemplateRoundTrip(t *testing.T) {
	useTempBaseDir(t)

	srcDir := t.TempDir()
	p, _, _, err := initProjectAt(srcDir, "源题")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.sourcePath(), []byte("// 模板骨架\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "1.in"), []byte("1 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "1.out"), []byte("3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	props := p.cfg()
	props["time_limit"] = "2.50"
	props["memory_limit"] = "512"
	props["remote_token"] = " SecretToken " // 凭据不能跟着模板走
	writeIni(p.configPath(), props)
	syncSamples(p, true)

	tmpl, err := templateFromProject(p, "我的模板", true)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Props["remote_token"] != "" {
		t.Fatal("模板里不该出现 remote_token")
	}
	if err := saveCustomTemplate(tmpl); err != nil {
		t.Fatal(err)
	}

	got, ok := findTemplate("我的模板")
	if !ok {
		t.Fatal("存好的模板没被找到")
	}
	if got.Props["time_limit"] != "2.50" || got.Props["memory_limit"] != "512" {
		t.Errorf("配置没带过去: %v", got.Props)
	}

	dst := filepath.Join(t.TempDir(), "新题")
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatal(err)
	}
	np, _, _, err := initProjectAtWith(dst, "新题", &got)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(np.sourcePath())
	if string(body) != "// 模板骨架\n" {
		t.Errorf("源码没按模板写: %q", body)
	}
	if v := np.cfg()["time_limit"]; v != "2.50" {
		t.Errorf("时限没按模板写: %q", v)
	}
	if len(readIndex(np)) != 1 {
		t.Errorf("样例没跟过来: %v", readIndex(np))
	}
	if !isBuiltinTemplateID(got.ID) {
		// 自定义模板要能被删掉，内置的要拦住。
		if err := deleteCustomTemplate(got.ID); err != nil {
			t.Errorf("删除自定义模板失败: %v", err)
		}
		if _, ok := findTemplate("我的模板"); ok {
			t.Error("删除后仍能找到该模板")
		}
	}
}

// TestBultinTemplateGuards 内置模板不能被删除、不能被同名覆盖。
func TestBuiltinTemplateGuards(t *testing.T) {
	useTempBaseDir(t)

	if err := deleteCustomTemplate("blank"); err == nil {
		t.Error("内置模板应当拒绝删除")
	}
	bad := projectTemplate{Name: "空白", Source: "int main(){}"}
	if err := saveCustomTemplate(bad); err == nil {
		t.Error("与内置模板重名时应当报错")
	}
	if err := saveCustomTemplate(projectTemplate{Name: "   "}); err == nil {
		t.Error("空名称应当报错")
	}
}

// TestTemplateExtrasPathGuard 模板里的文件名不允许跳出项目根。
func TestTemplateExtrasPathGuard(t *testing.T) {
	dir := t.TempDir()
	tmpl := projectTemplate{Files: map[string]string{"../escape.cpp": "x"}}
	if err := applyTemplateExtras(dir, tmpl); err == nil {
		t.Fatal("带 .. 的文件名应当被拒绝")
	}
}
