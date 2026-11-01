package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMemoryHardLimit 验证内存硬限制：程序一口气分配远超限额的内存时
// 应判 MLE 而不是把机器内存吃穿（Windows Job Object；其它平台跳过）。
func TestMemoryHardLimit(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()
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
