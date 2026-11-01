package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFileIOMode 验证文件读写题的完整评测路径：
// 输入文件被复制进临时运行目录，程序写出的答案文件被当作输出比较。
func TestFileIOMode(t *testing.T) {
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("没有 g++，跳过")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "apple.cpp")
	writeText(t, src, `#include <cstdio>
int main(){
    freopen("apple.in","r",stdin);
    freopen("apple.out","w",stdout);
    int a,b; if(scanf("%d %d",&a,&b)!=2) return 1;
    printf("%d\n",a+b);
    return 0;
}`)
	exe := exePathIn(dir, "apple")
	mustCompile(t, src, exe)

	inPath := filepath.Join(dir, "1.in")
	writeText(t, inPath, "2 3\n")

	// 标准答案
	ansPath := filepath.Join(dir, "1.out")
	writeText(t, ansPath, "5\n")

	job := judgeJob{
		ExePath:   exe,
		Dir:       dir,
		Index:     []string{"1"},
		Threads:   1,
		TimeLimit: 5,
		MemLimit:  256,
		FileIO:    &fileIO{In: "apple.in", Out: "apple.out"},
	}
	rep := evalSamples(job, nil, nil)
	if len(rep.Points) != 1 {
		t.Fatalf("期望 1 个测试点，实际 %d", len(rep.Points))
	}
	pt := rep.Points[0]
	if pt.Verdict != verdictAC {
		t.Fatalf("文件读写评测期望 AC，实际 %s（detail=%q stdout=%q）", pt.Verdict, pt.Detail, pt.Actual)
	}
	// 运行结束后项目目录里不应残留 .otk-io- 临时目录
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) > 8 && e.Name()[:8] == ".otk-io-" {
			t.Fatalf("临时目录未清理: %s", e.Name())
		}
	}
}

// TestFileIOFromProps 验证配置解析：模式开关与缺省文件名。
func TestFileIOFromProps(t *testing.T) {
	if f := fileIOFromProps(map[string]string{}, "P1001"); f != nil {
		t.Fatalf("未启用时期望 nil，实际 %+v", f)
	}
	f := fileIOFromProps(map[string]string{"io_mode": "file"}, "P1001")
	if f == nil || f.In != "P1001.in" || f.Out != "P1001.out" {
		t.Fatalf("缺省文件名错误: %+v", f)
	}
	f = fileIOFromProps(map[string]string{"io_mode": "file", "io_in": "a.in", "io_out": "a.out"}, "P1001")
	if f == nil || f.In != "a.in" || f.Out != "a.out" {
		t.Fatalf("自定义文件名错误: %+v", f)
	}
}
