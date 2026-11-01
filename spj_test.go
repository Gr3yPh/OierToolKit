package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestSPJJudge 验证特殊判题的完整链路：checker 以
// <输入> <选手输出> <期望输出> 三个参数被调用，退出码决定 AC/WA。
func TestSPJJudge(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()

	src := filepath.Join(dir, "aplusb.cpp")
	writeText(t, src, `#include <cstdio>
int main(){ int a,b; if(scanf("%d %d",&a,&b)!=2) return 1; printf("%d\n",a+b); return 0; }`)
	exe := exePathIn(dir, "aplusb")
	mustCompile(t, src, exe)

	// 选手输出换行风格故意不同：checker 认为"只要答案对就行"。
	checkerSrc := filepath.Join(dir, checkerSourceName)
	writeText(t, checkerSrc, `#include <cstdio>
#include <cstring>
int main(int argc, char** argv){
    if (argc != 4) return 2;
    FILE* fin = fopen(argv[1], "r");
    FILE* fout = fopen(argv[2], "r");
    FILE* fans = fopen(argv[3], "r");
    if (!fin || !fout || !fans) return 2;
    long long a, b, x, y;
    if (fscanf(fin, "%lld %lld", &a, &b) != 2) return 2;
    if (fscanf(fout, "%lld", &x) != 1) { printf("输出不是数字\n"); return 1; }
    if (fscanf(fans, "%lld", &y) != 1) return 2;
    if (x == y) return 0;
    printf("期望 %lld，实际 %lld\n", y, x);
    return 1;
}`)
	p := project{Name: "aplusb", Dir: dir}
	checkerExe, ceLog, ok := prepareChecker(p, map[string]string{})
	if !ok {
		t.Fatalf("checker 编译失败: %s", ceLog)
	}

	inPath := filepath.Join(dir, "1.in")
	writeText(t, inPath, "2 3\n")
	writeText(t, filepath.Join(dir, "1.out"), "5\n")

	job := judgeJob{
		ExePath:   exe,
		Dir:       dir,
		Index:     []string{"1"},
		Threads:   1,
		TimeLimit: 5,
		MemLimit:  256,
		Checker:   checkerExe,
	}
	rep := evalSamples(job, nil, nil)
	if len(rep.Points) != 1 || rep.Points[0].Verdict != verdictAC {
		t.Fatalf("checker 接受正确答案时期望 AC，实际 %+v", rep.Points)
	}

	// 答案错误时 checker 应判 WA，反馈里带上 checker 的消息。
	writeText(t, filepath.Join(dir, "1.out"), "6\n")
	rep = evalSamples(job, nil, nil)
	if len(rep.Points) != 1 || rep.Points[0].Verdict != verdictWA {
		t.Fatalf("checker 拒绝错误答案时期望 WA，实际 %+v", rep.Points)
	}
	if want := "期望 6，实际 5"; rep.Points[0].Detail != want {
		t.Fatalf("checker 反馈期望 %q，实际 %q", want, rep.Points[0].Detail)
	}
}

// TestSPJDisabled 验证 spj=off 时即使 checker.cpp 存在也不启用。
func TestSPJDisabled(t *testing.T) {
	dir := t.TempDir()
	p := project{Name: "t", Dir: dir}
	writeText(t, filepath.Join(dir, checkerSourceName), "int main(){return 0;}\n")
	if !spjEnabled(p, map[string]string{}) {
		t.Fatal("存在 checker.cpp 时应默认启用 SPJ")
	}
	if spjEnabled(p, map[string]string{"spj": "off"}) {
		t.Fatal("spj=off 时不应启用 SPJ")
	}
	exePath, _, ok := prepareChecker(p, map[string]string{"spj": "off"})
	if !ok || exePath != "" {
		t.Fatalf("spj=off 时 prepareChecker 应跳过编译，实际 exe=%q ok=%v", exePath, ok)
	}
}

// TestHistoryRoundTrip 验证评测历史的写入、读取与条数上限。
func TestHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := project{Name: "t", Dir: dir}

	for i := 0; i < historyMax+5; i++ {
		rep := judgeReport{
			TimeLimit: 1,
			MemLimit:  125,
			Points:    []pointResult{{ID: "1", Verdict: verdictAC, WallMS: 10, MemMB: 4.2, MemOK: true}},
		}
		appendHistory(p, rep)
	}
	entries := readHistory(p, 0)
	if len(entries) != historyMax {
		t.Fatalf("历史条数期望 %d，实际 %d", historyMax, len(entries))
	}
	// 截断后最旧的被丢弃，最新一条是最后一次写入的。
	last := entries[len(entries)-1]
	if last.Passed != 1 || last.Total != 1 || last.Points[0].Verdict != verdictAC {
		t.Fatalf("最新一条内容错误: %+v", last)
	}
	if first := entries[0]; first.Points[0].Verdict != verdictAC {
		t.Fatalf("首条内容错误: %+v", first)
	}

	clearHistory(p)
	if entries := readHistory(p, 0); len(entries) != 0 {
		t.Fatalf("清空后期望 0 条，实际 %d", len(entries))
	}
}

// TestFilterIndex 验证单点评测的编号过滤与错误报告。
func TestFilterIndex(t *testing.T) {
	index := []string{"1", "2", "sample3"}
	got := filterIndex(index, []string{"sample3", "1"})
	if len(got) != 2 || got[0] != "1" || got[1] != "sample3" {
		t.Fatalf("过滤结果错误: %v", got)
	}
	if len(filterIndex(index, nil)) != 0 {
		t.Fatal("空请求应返回空列表")
	}
}

// TestParseWarnings 验证编译警告行的提取。
func TestParseWarnings(t *testing.T) {
	log := "main.cpp:3:9: warning: unused variable 'x' [-Wunused-variable]\n" +
		"main.cpp:5:1: error: expected ';' before '}' token\n" +
		"something else\n"
	w := parseWarnings(log)
	if len(w) != 1 || !containsString(w, "main.cpp:3:9: warning: unused variable 'x' [-Wunused-variable]") {
		t.Fatalf("警告解析错误: %v", w)
	}
}

// TestCompileArgsCflags 验证额外编译参数进入 g++ 命令。
func TestCompileArgsCflags(t *testing.T) {
	args := compileArgs("a.cpp", "a.exe", map[string]string{"cflags": "-Wall -Wextra"})
	joined := ""
	for _, a := range args {
		joined += " " + a
	}
	if !containsString(args, "-Wall") || !containsString(args, "-Wextra") {
		t.Fatalf("cflags 未生效: %s", joined)
	}
	if containsString(args, "-Wall -Wextra") {
		t.Fatal("cflags 应按参数拆分而不是作为单个参数传递")
	}
}

// TestCompileArgsGcc 验证 gcc 模式：只用必要参数（无 -x 之类注入），
// 缺省标准 c17，链接 -lm。
func TestCompileArgsGcc(t *testing.T) {
	props := map[string]string{"compiler": "gcc", "version": "c++17"}
	args := compileArgs("a.cpp", "a.exe", props)
	if !containsString(args, "-std=c17") || containsString(args, "-std=c++17") {
		t.Fatalf("gcc 模式应把 c++ 标准换为 c17: %v", args)
	}
	if containsString(args, "-x") {
		t.Fatalf("不应注入 -x: %v", args)
	}
	if !containsString(args, "-lm") {
		t.Fatalf("gcc 模式缺少 -lm: %v", args)
	}
	if compilerOf(props) != "gcc" {
		t.Fatalf("compilerOf 错误: %s", compilerOf(props))
	}
	if compilerOf(map[string]string{}) != "g++" {
		t.Fatal("缺省编译器应为 g++")
	}
}
