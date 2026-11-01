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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 项目模板：新建项目时可选的一套"起手式"。
//
// 一个模板 = 源码骨架 + 一组 .otk/config 预设值 + 可选的附加文件（对拍的
// gen.cpp / brute.cpp）与初始样例。内置模板写在代码里，用户也可以把调好的
// 项目"另存为模板"，落在 ~/.otkprofile/templates/<id>/template.json。
//
// 存成单文件 JSON 而不是散装目录：模板整体很小（几 KB），单文件好复制、
// 好整份删掉，也不会出现"样例文件在一处、配置在另一处"的半截状态。

// templateNameTag 是源码骨架里的项目名占位符，写入时换成目录名。
// 文件读写模板要用它拼 <题目名>.in，不写死成某一个题目。
const templateNameTag = "{{name}}"

// defaultSourceSkeleton 是没有任何模板时的空白骨架，也是内置"空白"模板。
const defaultSourceSkeleton = "#include<iostream>\nusing namespace std;\n\nint main(){\n    \n    return 0;\n}\n"

// projectTemplate 是一个模板的完整定义，也是 template.json 的格式。
type projectTemplate struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Desc    string            `json:"desc"`
	Source  string            `json:"source"`
	Props   map[string]string `json:"props,omitempty"`
	Files   map[string]string `json:"files,omitempty"`   // 附加源码：文件名 -> 内容
	Samples []templateSample  `json:"samples,omitempty"` // 初始样例，按顺序编号 1、2、…
}

// templateSample 是模板自带的一组样例（项目根下的 <id>.in / <id>.out）。
type templateSample struct {
	In  string `json:"in"`
	Out string `json:"out"`
}

// builtinTemplates 返回内置模板。每次调用重新构造，因为描述里要用 tr()——
// 包级变量初始化时 baseDir 与词条都还没就绪。
func builtinTemplates() []projectTemplate {
	return []projectTemplate{
		{
			ID:     "blank",
			Name:   "空白",
			Desc:   "只有一段空的 main。",
			Source: defaultSourceSkeleton,
			Props:  map[string]string{"time_limit": "1.00", "memory_limit": "125"},
		},
		{
			ID:   "io",
			Name: "读入输出",
			Desc: "关同步流的标准读写骨架，带一组样例。",
			Source: `#include <bits/stdc++.h>
using namespace std;

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);

    int n;
    cin >> n;

    cout << n << '\n';
    return 0;
}
`,
			Props:   map[string]string{"time_limit": "1.00", "memory_limit": "256"},
			Samples: []templateSample{{In: "3\n", Out: "3\n"}},
		},
		{
			ID:   "multi",
			Name: "多测",
			Desc: "先读 T 再循环 solve()。",
			Source: `#include <bits/stdc++.h>
using namespace std;

void solve() {
    int n;
    cin >> n;
    cout << n << '\n';
}

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);

    int T;
    cin >> T;
    while (T--) solve();
    return 0;
}
`,
			Props: map[string]string{"time_limit": "2.00", "memory_limit": "256"},
		},
		{
			ID:   "fileio",
			Name: "文件读写",
			Desc: "freopen 直接读写 <题目名>.in / <题目名>.out。",
			Source: `#include <bits/stdc++.h>
using namespace std;

int main() {
    freopen("{{name}}.in", "r", stdin);
    freopen("{{name}}.out", "w", stdout);

    int n;
    scanf("%d", &n);
    printf("%d\n", n);
    return 0;
}
`,
			Props: map[string]string{
				"time_limit":   "1.00",
				"memory_limit": "256",
				"io":           "file",
				"io_in":        "{{name}}.in",
				"io_out":       "{{name}}.out",
			},
		},
		{
			ID:   "stress",
			Name: "对拍三件套",
			Desc: "正解 + gen.cpp + brute.cpp 一起生成，建好就能直接对拍。",
			Source: `#include <bits/stdc++.h>
using namespace std;

int main() {
    int n;
    scanf("%d", &n);
    long long sum = 0;
    for (int i = 0; i < n; i++) {
        int x;
        scanf("%d", &x);
        sum += x;
    }
    printf("%lld\n", sum);
    return 0;
}
`,
			Props: map[string]string{"time_limit": "1.00", "memory_limit": "256"},
			Files: map[string]string{
				"gen.cpp": `#include <bits/stdc++.h>
using namespace std;

// 每次运行换一组数据；想复现某一轮的输入，把种子固定成常量即可。
int main() {
    mt19937_64 rng(chrono::steady_clock::now().time_since_epoch().count());
    int n = int(rng() % 10) + 1;
    printf("%d\n", n);
    for (int i = 0; i < n; i++) {
        printf("%lld%c", (long long)(rng() % 100) + 1, " \n"[i + 1 == n]);
    }
    return 0;
}
`,
				"brute.cpp": `#include <bits/stdc++.h>
using namespace std;

// 暴力：枚举每个数的起点往后加，结果与正解一致，只是慢得多。
int main() {
    int n;
    if (scanf("%d", &n) != 1) return 0;
    vector<int> a(n);
    for (int i = 0; i < n; i++) scanf("%d", &a[i]);

    long long sum = 0;
    for (int i = 0; i < n; i++) {
        long long cur = 0;
        for (int j = i; j < n; j++) {
            cur += a[j];
            if (j == i) sum += cur;
        }
    }
    printf("%lld\n", sum);
    return 0;
}
`,
			},
		},
	}
}

// templateLabel 返回下拉框里显示的文案。内置模板的名称要翻译，自定义模板
// 是用户自己起的名字，原样显示。
func templateLabel(t projectTemplate) string {
	switch t.ID {
	case "blank":
		return tr("空白")
	case "io":
		return tr("读入输出")
	case "multi":
		return tr("多测")
	case "fileio":
		return tr("文件读写")
	case "stress":
		return tr("对拍三件套")
	}
	return t.Name
}

// templateDesc 返回模板的一句话说明（内置的走翻译）。
func templateDesc(t projectTemplate) string {
	switch t.ID {
	case "blank":
		return tr("只有一段空的 main。")
	case "io":
		return tr("关同步流的标准读写骨架，带一组样例。")
	case "multi":
		return tr("先读 T 再循环 solve()。")
	case "fileio":
		return tr("freopen 直接读写 <题目名>.in / <题目名>.out。")
	case "stress":
		return tr("正解 + gen.cpp + brute.cpp 一起生成，建好就能直接对拍。")
	}
	return t.Desc
}

// templatePropsSummary 把模板的预设值拼成一行说明，例如
// "1.00s · 256MB · 文件读写 · 2 组样例"。
func templatePropsSummary(t projectTemplate) string {
	var parts []string
	if v := strings.TrimSpace(t.Props["time_limit"]); v != "" {
		parts = append(parts, v+"s")
	}
	if v := strings.TrimSpace(t.Props["memory_limit"]); v != "" {
		parts = append(parts, v+"MB")
	}
	if strings.EqualFold(t.Props["io"], "file") {
		parts = append(parts, tr("文件读写"))
	}
	if n := len(t.Files); n > 0 {
		parts = append(parts, fmt.Sprintf(tr("%d 个附加源文件"), n))
	}
	if n := len(t.Samples); n > 0 {
		parts = append(parts, fmt.Sprintf(tr("%d 组样例"), n))
	}
	return strings.Join(parts, " · ")
}

// templatesRoot 是自定义模板的存放目录。
func templatesRoot() string { return filepath.Join(baseDir, "templates") }

// customTemplatePath 是某个自定义模板的 template.json 路径。
func customTemplatePath(id string) string {
	return filepath.Join(templatesRoot(), id, "template.json")
}

// loadCustomTemplates 读入全部自定义模板，按名称排序。
// 单个文件坏掉不影响其余：坏的那个直接跳过，不在这里报错打断列表。
func loadCustomTemplates() []projectTemplate {
	entries, err := os.ReadDir(templatesRoot())
	if err != nil {
		return nil
	}
	var out []projectTemplate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(customTemplatePath(e.Name()))
		if err != nil {
			continue
		}
		var t projectTemplate
		if err := json.Unmarshal(b, &t); err != nil {
			continue
		}
		if t.ID == "" {
			t.ID = e.Name()
		}
		if strings.TrimSpace(t.Name) == "" {
			t.Name = e.Name()
		}
		// 与内置模板撞名的手工文件直接忽略，免得列表里出现两个同名模板。
		if shadowsBuiltin(t) {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// shadowsBuiltin 判断一个模板是否与内置模板撞名。ID 与显示名都要看：
// ID 是 ascii（blank/io/…），显示名是中文，只挡其中一个都会让下拉框出现
// 两个同名项，而 findTemplate 永远只认内置那个。
func shadowsBuiltin(t projectTemplate) bool {
	for _, b := range builtinTemplates() {
		if strings.EqualFold(t.ID, b.ID) || strings.EqualFold(t.Name, templateLabel(b)) {
			return true
		}
	}
	return false
}

// isBuiltinTemplateID 判断 ID 是否被内置模板占用。
func isBuiltinTemplateID(id string) bool {
	for _, t := range builtinTemplates() {
		if t.ID == id {
			return true
		}
	}
	return false
}

// allTemplates 返回内置 + 自定义，内置在前。
func allTemplates() []projectTemplate {
	return append(builtinTemplates(), loadCustomTemplates()...)
}

// findTemplate 按 ID 或名称（忽略大小写）找模板。
func findTemplate(key string) (projectTemplate, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return projectTemplate{}, false
	}
	list := allTemplates()
	for _, t := range list {
		if strings.EqualFold(t.ID, key) {
			return t, true
		}
	}
	for _, t := range list {
		if strings.EqualFold(t.Name, key) || strings.EqualFold(templateLabel(t), key) {
			return t, true
		}
	}
	return projectTemplate{}, false
}

// templateNames 拼接出可读的模板名列表，用于错误提示。
func templateNames() string {
	var names []string
	for _, t := range allTemplates() {
		names = append(names, templateLabel(t))
	}
	return strings.Join(names, " / ")
}

// saveCustomTemplate 写一个自定义模板。ID 由名称派生，重名直接覆盖。
func saveCustomTemplate(t projectTemplate) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%s", tr("模板名称不能为空"))
	}
	// sanitizeFileName 兜底返回 "unnamed"，这里不接受这种退化结果：
	// 名字全是不合法字符时应当报错，而不是悄悄存成 unnamed。
	if id := sanitizeFileName(t.Name); id != "" && id != "unnamed" {
		t.ID = id
	} else {
		return fmt.Errorf("%s", tr("模板名称不能全是不合法字符"))
	}
	if shadowsBuiltin(t) {
		return fmt.Errorf(tr("模板名 %s 与内置模板重名，换一个"), t.Name)
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	path := customTemplatePath(t.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

// deleteCustomTemplate 删掉一个自定义模板；内置模板不允许删。
func deleteCustomTemplate(id string) error {
	if isBuiltinTemplateID(id) {
		return fmt.Errorf("%s", tr("内置模板不能删除"))
	}
	return os.RemoveAll(filepath.Join(templatesRoot(), id))
}

// templateFromProject 把当前项目提炼成模板：源码骨架 + 编译配置 + 样例。
// 只保留可复用的项，名称、远端、题目 ID 这类绑死单个项目的字段不带。
func templateFromProject(p project, name string, withSamples bool) (projectTemplate, error) {
	src, err := os.ReadFile(p.sourcePath())
	if err != nil {
		return projectTemplate{}, fmt.Errorf(tr("读不到源码文件 %s"), p.codeName()+".cpp")
	}

	// 白名单而不是黑名单：新加的配置项默认不跟着模板走，
	// 免得某天多出个 remote_token 之类的字段被顺手带进模板。
	keep := []string{"time_limit", "memory_limit", "version", "compiler", "o2", "cflags", "threads", "io", "io_in", "io_out"}
	props := map[string]string{}
	cfg := p.cfg()
	for _, k := range keep {
		if v := strings.TrimSpace(cfg[k]); v != "" {
			props[k] = v
		}
	}

	t := projectTemplate{
		Name:   strings.TrimSpace(name),
		Source: string(src),
		Props:  props,
		Desc:   tr("由项目另存"),
	}

	// 对拍用的 gen.cpp / brute.cpp 跟着一起走，否则模板建出来的项目
	// 还得手工补两个文件，失去"建好就能对拍"的意义。
	files := map[string]string{}
	for _, fname := range []string{"gen.cpp", "brute.cpp"} {
		if b, err := os.ReadFile(filepath.Join(p.Dir, fname)); err == nil {
			files[fname] = string(b)
		}
	}
	if len(files) > 0 {
		t.Files = files
	}

	if withSamples {
		const maxSampleBytes = 64 << 10 // 单个样例文件上限 64KB
		for _, id := range readIndex(p) {
			in, errIn := os.ReadFile(filepath.Join(p.Dir, id+".in"))
			outPath := filepath.Join(p.Dir, id+".out")
			if !fileExists(outPath) {
				outPath = filepath.Join(p.Dir, id+".ans")
			}
			out, errOut := os.ReadFile(outPath)
			if errIn != nil || errOut != nil {
				continue
			}
			if len(in) > maxSampleBytes || len(out) > maxSampleBytes {
				continue // 大样例塞进模板会把 JSON 撑得没法看
			}
			t.Samples = append(t.Samples, templateSample{In: string(in), Out: string(out)})
			if len(t.Samples) >= 10 {
				break
			}
		}
	}
	return t, nil
}

// render 把骨架里的 {{name}} 换成项目名（源码基名）。
func (t projectTemplate) render(s, codeName string) string {
	return strings.ReplaceAll(s, templateNameTag, codeName)
}

// applyTemplateExtras 写模板附带的文件与样例。只在目标文件不存在时写，
// 绝不覆盖已经存在的文件——模板只是起手式，用户改过的东西不能被冲掉。
func applyTemplateExtras(dir string, t projectTemplate) error {
	names := make([]string, 0, len(t.Files))
	for fname := range t.Files {
		names = append(names, fname)
	}
	sort.Strings(names) // map 遍历无序，排序后行为稳定、出错也好定位
	for _, fname := range names {
		// 模板文件只允许落在项目根，挡掉 ../ 之类写坏目录的路径。
		if strings.ContainsAny(fname, `/\`) {
			return fmt.Errorf(tr("模板里的文件名不合法: %s"), fname)
		}
		path := filepath.Join(dir, fname)
		if fileExists(path) {
			continue
		}
		content := t.render(t.Files[fname], filepath.Base(dir))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf(tr("写入 %s 失败: %v"), fname, err)
		}
	}

	for i, s := range t.Samples {
		id := fmt.Sprintf("%d", i+1)
		inPath := filepath.Join(dir, id+".in")
		outPath := filepath.Join(dir, id+".out")
		if !fileExists(inPath) {
			if err := os.WriteFile(inPath, []byte(s.In), 0644); err != nil {
				return fmt.Errorf(tr("写入样例 %s.in 失败: %v"), id, err)
			}
		}
		if !fileExists(outPath) {
			if err := os.WriteFile(outPath, []byte(s.Out), 0644); err != nil {
				return fmt.Errorf(tr("写入样例 %s.out 失败: %v"), id, err)
			}
		}
	}
	return nil
}
