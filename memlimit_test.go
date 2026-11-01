package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// crashDir 建一个临时目录，清理失败不算测试失败。
//
// 这里跑的都是会被内存上限掐死、或者自己 crash 的程序，Windows 会把刚结束的
// 进程镜像多按住一会儿（WER/杀软都可能短暂持有），t.TempDir 的自动清理于是
// 报 "Access is denied" 并把测试判红——与被测逻辑无关。
func crashDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "otk-memtest-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestMemoryHardLimit 验证内存硬限制：程序一口气分配远超限额的内存时
// 应判 MLE 而不是把机器内存吃穿（Windows Job Object；其它平台跳过）。
func TestMemoryHardLimit(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := crashDir(t)
	src := filepath.Join(dir, "hog.cpp")
	writeText(t, src, `#include <cstdio>
#include <cstring>
int main(){
    // 每次 1MB 并真的写满（提交+触碰），循环申请 256MB：
    // 在 128MB 硬限制处分配必然失败。
    for (int i = 0; i < 256; i++) {
        char *p = new char[1 << 20];
        memset(p, 1, 1 << 20);
    }
    printf("done\n");
    return 0;
}`)
	exe := exePathIn(dir, "hog")
	mustCompile(t, src, exe)

	// 评测需要成对的输入/答案文件才能构成测试点。
	writeText(t, filepath.Join(dir, "1.in"), "\n")
	writeText(t, filepath.Join(dir, "1.out"), "done\n")

	job := judgeJob{
		ExePath:   exe,
		Dir:       dir,
		Index:     []string{"1"},
		Threads:   1,
		TimeLimit: 10,
		MemLimit:  128, // 远小于 2GB 的申请量
	}
	rep := evalSamples(job, nil, nil)
	pt := rep.Points[0]
	if pt.Verdict != verdictMLE {
		t.Fatalf("内存硬限制期望 MLE，实际 %s（mem=%.0fMB detail=%q）",
			pt.Verdict, pt.MemMB, pt.Detail)
	}
}

// TestMemoryLimitHangsAtCap 验证「顶到内存上限后不退出」的程序判 MLE 而不是 TLE。
//
// 把 bad_alloc 咽下去再空转是很常见的写法：它确实碰到了内存上限，只是没
// 立刻退出。以前先判超时，于是这种程序显示 TLE，让人去查算法复杂度——查反了方向。
func TestMemoryLimitHangsAtCap(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := crashDir(t)
	src := filepath.Join(dir, "spin.cpp")
	writeText(t, src, `#include <cstring>
int main(){
    try {
        // 1MB 一块吃到上限
        for (int i = 0; i < 1024; i++) {
            char *p = new char[1 << 20];
            memset(p, 1, 1 << 20);
        }
    } catch (...) {
        // 吃掉 bad_alloc 后继续空转：内存到顶但进程不退出
        for (volatile long long i = 0;; i++) {}
    }
    return 0;
}`)
	exe := exePathIn(dir, "spin")
	mustCompile(t, src, exe)

	writeText(t, filepath.Join(dir, "1.in"), "\n")
	writeText(t, filepath.Join(dir, "1.out"), "done\n")

	job := judgeJob{
		ExePath:   exe,
		Dir:       dir,
		Index:     []string{"1"},
		Threads:   1,
		TimeLimit: 2,
		MemLimit:  128,
	}
	rep := evalSamples(job, nil, nil)
	pt := rep.Points[0]
	if pt.Verdict != verdictMLE {
		t.Fatalf("顶到内存上限后空转期望 MLE，实际 %s（mem=%.0fMB detail=%q）",
			pt.Verdict, pt.MemMB, pt.Detail)
	}
}
