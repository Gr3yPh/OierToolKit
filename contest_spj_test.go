package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestContestSPJ 验证模拟赛的特殊判题：开 SPJ 后 checker.cpp 认可的
// 输出判 AC（即使与 .ans 逐字不同），被拒绝的判 WA 并带上反馈。
func TestContestSPJ(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	c := setupContest(t)

	// aplusb 开 SPJ：checker 只比较数值，容忍行尾空白差异。
	data := c.problemsDir("aplusb")
	writeText(t, filepath.Join(data, checkerSourceName), `#include <cstdio>
int main(int argc, char** argv){
    if (argc != 4) return 2;
    FILE* fout = fopen(argv[2], "r");
    FILE* fans = fopen(argv[3], "r");
    if (!fout || !fans) return 2;
    long long x, y;
    if (fscanf(fout, " %lld", &x) != 1) { printf("输出不是数字\n"); return 1; }
    if (fscanf(fans, " %lld", &y) != 1) return 2;
    if (x == y) return 0;
    printf("期望 %lld，实际 %lld\n", y, x);
    return 1;
}`)

	// 找到 aplusb 的配置，打开 SPJ。
	found := false
	for i := range c.Cfg.Problems {
		if c.Cfg.Problems[i].Name == "aplusb" {
			c.Cfg.Problems[i].SPJ = true
			found = true
		}
	}
	if !found {
		t.Fatal("配置里没有 aplusb")
	}
	if err := c.save(); err != nil {
		t.Fatal(err)
	}

	// 选手程序把 a+b 后面多打一个空格：普通比对会 WA，checker 应给 AC。
	srcDir := filepath.Join(c.sourceDir(), "张三")
	writeText(t, filepath.Join(srcDir, "aplusb.cpp"),
		"#include <cstdio>\nint main(){ printf(\"3 \"); return 0; }")

	res, err := judgeContest(c, nil, nil)
	if err != nil {
		t.Fatalf("judgeContest: %v", err)
	}
	var out problemResult
	for _, r := range res.Results {
		if r.Player == "张三" && r.Problem == "aplusb" {
			out = r
		}
	}
	if len(out.Tests) != 2 {
		t.Fatalf("期望 2 个测试点，实际 %d（compileOK=%v log=%q all=%+v）",
			len(out.Tests), out.CompileOK, out.CompileLog, res.Results)
	}
	for _, tp := range out.Tests {
		if tp.Verdict != verdictAC {
			t.Fatalf("checker 应容忍多余空格，实际 %s（%s）", tp.Verdict, tp.Message)
		}
	}

	// 输出错误答案时应 WA，反馈来自 checker。
	writeText(t, filepath.Join(srcDir, "aplusb.cpp"),
		"#include <cstdio>\nint main(){ printf(\"4\"); return 0; }")
	res, err = judgeContest(c, nil, nil)
	if err != nil {
		t.Fatalf("judgeContest: %v", err)
	}
	for _, r := range res.Results {
		if r.Player == "张三" && r.Problem == "aplusb" {
			if r.Tests[0].Verdict != verdictWA {
				t.Fatalf("错误答案期望 WA，实际 %s", r.Tests[0].Verdict)
			}
			if want := "期望 3，实际 4"; r.Tests[0].Message != want {
				t.Fatalf("checker 反馈期望 %q，实际 %q", want, r.Tests[0].Message)
			}
		}
	}

	// 未开 SPJ 的 sortarr 不受影响，仍按普通比对（源码 return 0，无输出 → WA）。
	for _, r := range res.Results {
		if r.Player == "李四" && r.Problem == "sortarr" && r.Tests[0].Verdict != verdictWA {
			t.Fatalf("未开 SPJ 的题目期望 WA，实际 %s", r.Tests[0].Verdict)
		}
	}
}

// TestContestSPJMissingChecker 开了 SPJ 但没有 checker.cpp 时退回普通比对。
func TestContestSPJMissingChecker(t *testing.T) {
	c := setupContest(t)
	for i := range c.Cfg.Problems {
		c.Cfg.Problems[i].SPJ = true
	}
	// 不放 checker.cpp：spjActive 应为 false，评测正常走普通比对。
	for _, p := range c.Cfg.Problems {
		if c.spjActive(p) {
			t.Fatalf("%s 不应判定为启用 SPJ", p.Name)
		}
	}
	// 评测不应报错。
	if _, err := judgeContest(c, nil, nil); err != nil {
		t.Fatalf("judgeContest: %v", err)
	}
	_ = os.RemoveAll(c.Dir)
}
