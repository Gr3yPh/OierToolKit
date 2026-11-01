package main

import "testing"

// TestParseSemver 验证版本号解析的宽松程度。
func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"v2.6.0", [3]int{2, 6, 0}, true},
		{"2.6", [3]int{2, 6, 0}, true},
		{"1.2.3-beta", [3]int{1, 2, 3}, true},
		{"", [3]int{}, false},
		{"abc", [3]int{}, false},
		{"1.x.3", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parseSemver(c.in)
		if ok != c.ok {
			t.Fatalf("parseSemver(%q) ok = %v，期望 %v", c.in, ok, c.ok)
		}
		if ok && got != c.want {
			t.Fatalf("parseSemver(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}

// TestIsNewerVersion 验证更新比较：只有严格更新才返回 true。
func TestIsNewerVersion(t *testing.T) {
	yes := [][2]string{
		{"v2.6.0", "v2.5.0"},
		{"2.6", "2.5.9"},
		{"v3.0.0", "v2.99.99"},
	}
	for _, c := range yes {
		if !isNewerVersion(c[0], c[1]) {
			t.Fatalf("%s 应比 %s 新", c[0], c[1])
		}
	}
	no := [][2]string{
		{"v2.5.0", "v2.5.0"},
		{"v2.4.9", "v2.5.0"},
		{"garbage", "v2.5.0"},
		{"", "v2.5.0"},
		{"v2.5.0", "not-a-version"}, // 本地格式非法时不误报
	}
	for _, c := range no {
		if isNewerVersion(c[0], c[1]) {
			t.Fatalf("%s 不应比 %s 新", c[0], c[1])
		}
	}
}

// TestSparkline 验证趋势小方块的比例映射。
func TestSparkline(t *testing.T) {
	if s := sparkline(nil); s != "" {
		t.Fatalf("空输入期望空串，实际 %q", s)
	}
	if s := sparkline([]int64{0, 0}); s != "" {
		t.Fatalf("全 0 期望空串，实际 %q", s)
	}
	s := sparkline([]int64{10, 20, 30})
	if len([]rune(s)) != 3 {
		t.Fatalf("长度期望 3，实际 %q", s)
	}
	runes := []rune(s)
	if runes[0] >= runes[2] {
		t.Fatalf("递增序列首字符应小于尾字符：%q", s)
	}
}
