package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 测试用的选手名。必须是纯 ASCII：CI 的英文 Windows 上 gcc 无法写入
// 含中文的路径（详见 setupContest 的说明）。
const (
	testPlayerA = "alice"
	testPlayerB = "bob"
)

// 搭建一场最小可用的比赛：2 道题、2 位选手、若干提交。
//
// 目录名与选手名一律用 ASCII：CI 的英文 Windows runner 上 gcc/ld 走 ANSI
// 接口，路径里一旦有非当前代码页的字符（中文），就会变成 '?' 并报
// "cannot open output file ...: Invalid argument"，整场评测算编译失败。
// 中文命名本身由 TestContestChineseNamesRoundTrip 覆盖（不经过编译器）。
func setupContest(t *testing.T) contest {
	t.Helper()
	// 比赛会落到 baseDir/contests 下，必须指到临时目录，否则测试会在
	// 仓库根目录堆一堆比赛目录。
	useTempBaseDir(t)
	root := t.TempDir()
	data := filepath.Join(root, "data")
	src := filepath.Join(root, "source")
	for _, p := range []string{"aplusb", "sortarr"} {
		dir := filepath.Join(data, p)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"1", "2"} {
			os.WriteFile(filepath.Join(dir, id+".in"), []byte("1 2\n"), 0644)
			os.WriteFile(filepath.Join(dir, id+".ans"), []byte("3\n"), 0644)
		}
	}
	for _, pl := range []string{testPlayerA, testPlayerB} {
		dir := filepath.Join(src, pl)
		os.MkdirAll(dir, 0755)
		for _, p := range []string{"aplusb", "sortarr"} {
			os.WriteFile(filepath.Join(dir, p+".cpp"), []byte("int main(){return 0;}\n"), 0644)
		}
	}
	// 比赛目录名来自比赛名，测试之间必须唯一。
	name := fmt.Sprintf("contest-%d", time.Now().UnixNano())
	c, err := importContest(root, name)
	if err != nil {
		t.Fatalf("importContest: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(c.Dir) })
	return c
}

func TestContestEditorSubtaskCRUD(t *testing.T) {
	c := setupContest(t)
	p := &contestPage{g: &guiApp{}, c: c}

	// 添加子任务
	p.addSubtask(0)
	p.flushSave()
	if n := len(p.c.Cfg.Problems[0].Subtasks); n != 1 {
		t.Fatalf("添加后应有 1 个子任务，实际 %d", n)
	}
	// 改名、设分、设测试点、设判定方式
	p.updateProblem(0, func(pr *contestProblem) {
		pr.Subtasks[0].Name = "SubA"
		pr.Subtasks[0].Score = 60
		pr.Subtasks[0].Mode = subtaskSum
		pr.Subtasks[0].Tests = []string{"1", "2"}
	})
	p.flushSave()

	// 重新读盘，确认已落盘
	c2, err := loadContest(c.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st := c2.Cfg.Problems[0].Subtasks[0]
	if st.Name != "SubA" || st.Score != 60 || st.Mode != subtaskSum || len(st.Tests) != 2 {
		t.Fatalf("子任务未正确保存: %+v", st)
	}

	// 删除子任务
	p.removeSubtask(0, 0)
	p.flushSave()
	c3, _ := loadContest(c.Dir)
	if len(c3.Cfg.Problems[0].Subtasks) != 0 {
		t.Fatalf("删除后不应有子任务")
	}
}

func TestContestEditorProblemFields(t *testing.T) {
	c := setupContest(t)
	p := &contestPage{g: &guiApp{}, c: c}
	p.updateProblem(0, func(pr *contestProblem) {
		pr.Title = "A+B Problem"
		pr.TimeMS = 2500
		pr.MemMB = 512
		pr.Tests[0].Score = 37
	})
	p.flushSave()
	c2, _ := loadContest(c.Dir)
	pr := c2.Cfg.Problems[0]
	if pr.Title != "A+B Problem" || pr.TimeMS != 2500 || pr.MemMB != 512 {
		t.Fatalf("题目属性未保存: %+v", pr)
	}
	if pr.Tests[0].Score != 37 {
		t.Fatalf("测试点分值未保存: %v", pr.Tests[0].Score)
	}
}

func TestContestRosterMutations(t *testing.T) {
	c := setupContest(t)
	p := &contestPage{g: &guiApp{}, c: c}

	// 补交：新选手
	tmp := filepath.Join(t.TempDir(), "x.cpp")
	os.WriteFile(tmp, []byte("int main(){return 0;}\n"), 0644)
	if err := p.addSubmission("王五", "aplusb", tmp); err != nil {
		t.Fatalf("addSubmission: %v", err)
	}
	c2, _ := loadContest(c.Dir)
	if len(c2.Cfg.Sources) != 5 {
		t.Fatalf("应有 5 份提交，实际 %d", len(c2.Cfg.Sources))
	}
	want := filepath.Join(c.Dir, "source", "王五", "aplusb.cpp")
	if !fileExists(want) {
		t.Fatalf("源码未复制到 %s", want)
	}

	// 重复补交同一题：应覆盖而不是新增
	if err := p.addSubmission("王五", "aplusb", tmp); err != nil {
		t.Fatal(err)
	}
	c3, _ := loadContest(c.Dir)
	if len(c3.Cfg.Sources) != 5 {
		t.Fatalf("重复补交不应新增记录，实际 %d", len(c3.Cfg.Sources))
	}

	// 改名
	if err := p.renamePlayer("王五", "王五五"); err != nil {
		t.Fatalf("renamePlayer: %v", err)
	}
	c4, _ := loadContest(c.Dir)
	if !dirExists(filepath.Join(c.Dir, "source", "王五五")) {
		t.Fatal("目录未改名")
	}
	found := false
	for _, s := range c4.Cfg.Sources {
		if s.Player == "王五五" && s.Problem == "aplusb" {
			found = true
		}
		if s.Player == "王五" {
			t.Fatal("旧名字仍残留在配置里")
		}
	}
	if !found {
		t.Fatal("改名后提交记录丢失")
	}

	// 删除提交
	if err := p.removeSubmission("王五五", "aplusb"); err != nil {
		t.Fatal(err)
	}
	c5, _ := loadContest(c.Dir)
	if len(c5.Cfg.Sources) != 4 {
		t.Fatalf("删除后应剩 4 份，实际 %d", len(c5.Cfg.Sources))
	}
	for _, pl := range c5.Cfg.Players {
		if pl.Name == "王五五" {
			t.Fatal("没有提交的选手应从选手表移除")
		}
	}
}

func TestContestAutoSubtasksAndScores(t *testing.T) {
	c := setupContest(t)
	p := &contestPage{g: &guiApp{}, c: c}
	p.autoSubtasks()
	c2, _ := loadContest(c.Dir)
	for _, pr := range c2.Cfg.Problems {
		if len(pr.Subtasks) != len(pr.Tests) {
			t.Fatalf("%s 应生成 %d 个子任务，实际 %d", pr.Name, len(pr.Tests), len(pr.Subtasks))
		}
	}
	// 补足 100 分
	p.fillScores()
	c3, _ := loadContest(c.Dir)
	for _, pr := range c3.Cfg.Problems {
		if got := fullScoreOf(c3, pr.Name); got < 99.999 || got > 100.001 {
			t.Fatalf("%s 满分应为 100，实际 %v", pr.Name, got)
		}
	}
}

func TestSplitIDs(t *testing.T) {
	cases := map[string]int{
		"1, 2, 3":   3,
		"1，2，3":     3,
		"":          0,
		" 1  2\t3 ": 3,
		"1,,2":      2,
	}
	for in, want := range cases {
		if got := len(splitIDs(in)); got != want {
			t.Fatalf("splitIDs(%q) = %d 个，期望 %d", in, got, want)
		}
	}
}

// TestContestChineseNamesRoundTrip 覆盖中文比赛名 / 中文选手名的导入与落盘。
//
// 这里刻意不经过编译器：英文 Windows 上 gcc 走 ANSI 接口，含中文的路径会被
// 替换成 '?' 而无法写入，所以「中文名 + 编译评测」只在中文 Windows / Linux
// 上成立（见 setupContest 的说明）。命名、目录推导、配置读写这些与平台无关
// 的部分仍然要保证正确。
func TestContestChineseNamesRoundTrip(t *testing.T) {
	useTempBaseDir(t)
	const name = "测试赛-中文"
	c, err := importContest(makeUploadDir(t), name)
	if err != nil {
		t.Fatalf("importContest: %v", err)
	}
	if c.Cfg.Name != name {
		t.Fatalf("比赛名应为 %q，实际 %q", name, c.Cfg.Name)
	}
	if base := filepath.Base(c.Dir); base != name {
		t.Fatalf("目录名应从比赛名派生，实际 %q", base)
	}
	if !fileExists(filepath.Join(c.sourceDir(), "张三", "aplusb.cpp")) {
		t.Fatal("中文选手目录下的源码应被完整复制")
	}
	// 重新读盘，确认中文名在 contest.json 里往返无损。
	got, err := loadContest(c.Dir)
	if err != nil {
		t.Fatalf("loadContest: %v", err)
	}
	if got.Cfg.Name != name || len(got.Cfg.Players) != 1 || got.Cfg.Players[0].Name != "张三" {
		t.Fatalf("读回的比赛信息不对: name=%q players=%+v", got.Cfg.Name, got.Cfg.Players)
	}
}
