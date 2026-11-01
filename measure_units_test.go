package main

import (
	"math"
	"testing"
)

// TestMaxrssUnits 钉住 ru_maxrss 的单位换算：只有 macOS 是字节，
// FreeBSD / DragonFly / NetBSD / OpenBSD 与 Linux、AIX 一样都是 KB。
//
// 这条曾经写错过（把 freebsd/dragonfly 也当成字节），后果是 BSD 上
// 内存少报 1024 倍，超限程序一律判 AC。数值取自各自的 getrusage(2)
// 手册与 GNU time 的 configure 探测。
func TestMaxrssUnits(t *testing.T) {
	const mb = 1024 * 1024
	cases := []struct {
		goos string
		raw  int64
		want float64
	}{
		// 512MB 的程序：KB 单位 → 524288；字节单位 → 536870912。
		{"linux", 512 * 1024, 512},
		{"freebsd", 512 * 1024, 512},
		{"dragonfly", 512 * 1024, 512},
		{"netbsd", 512 * 1024, 512},
		{"openbsd", 512 * 1024, 512},
		{"aix", 512 * 1024, 512},
		{"darwin", 512 * mb, 512},
	}
	for _, c := range cases {
		got := maxrssToMB(c.goos, c.raw)
		if math.Abs(got-c.want) > 0.5 {
			t.Errorf("%s: maxrssToMB(%d) = %.2fMB，期望 %.2fMB", c.goos, c.raw, got, c.want)
		}
	}
}

// TestMaxrssUnitsBoundary 确认换算在边界值上不会溢出或丢精度：
// 32 位平台的 Maxrss 是 int32，字节单位下最大值约 2GB。
func TestMaxrssUnitsBoundary(t *testing.T) {
	if got := maxrssToMB("freebsd", 0); got != 0 {
		t.Errorf("0 值应换算成 0，实际 %v", got)
	}
	// 64MB（KB 单位）应得 64.0；换成字节单位的 darwin 同样应得 64.0。
	if got := maxrssToMB("linux", 64*1024); math.Abs(got-64) > 1e-9 {
		t.Errorf("64MB(KB 单位) = %v，期望 64", got)
	}
	if got := maxrssToMB("darwin", 64*1024*1024); math.Abs(got-64) > 1e-9 {
		t.Errorf("64MB(字节单位) = %v，期望 64", got)
	}
}
