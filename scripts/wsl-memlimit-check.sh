#!/usr/bin/env bash
# Linux 内存硬限制的端到端验证。
set -uo pipefail
OTK=/tmp/otk-test
ROOT=/tmp/limit-e2e
# WSL 空闲关停会清掉 /tmp（tmpfs），所以缺了就现场编一份。
if [ ! -x "$OTK" ]; then
	export PATH=/usr/local/go/bin:$PATH GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local CGO_ENABLED=1
	(cd "$HOME/otkbuild" && go build -trimpath -buildvcs=false -o "$OTK" .) || exit 1
fi
rm -rf "$ROOT"
mkdir -p "$ROOT"

mk() { # mk <名字> <源码>
	mkdir -p "$ROOT/$1"
	cd "$ROOT/$1" || exit 1
	"$OTK" init "$1" >/dev/null
	cat > "$1.cpp"
	printf '1\n' > 1.in
	printf '1\n' > 1.out
	"$OTK" refresh >/dev/null 2>&1
}

echo "===== A. 1MB 一块吃到 400MB（会检查 malloc），限制 125MB ====="
mk bigcheck <<'EOF'
#include <cstdio>
#include <cstdlib>
#include <cstring>
int main() {
	for (int i = 0; i < 400; i++) {
		char *p = (char *)malloc(1 << 20);
		if (!p) { printf("失败于 %d MB\n", i); return 0; }
		memset(p, 1, 1 << 20);
	}
	printf("400MB 全部拿到\n");
	return 0;
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== B. 一次性申请 2GB 且不检查返回值（预期 RE：一次大块申请直接失败） ====="
mk bigsingle <<'EOF'
#include <cstdio>
#include <cstdlib>
int main() {
	size_t n = 2ULL << 30;
	volatile char *p = (volatile char *)malloc(n);
	p[0] = 1;       // 分配失败即空指针解引用
	p[n - 1] = 2;
	printf("%d\n", p[0] + p[n - 1]);
	return 0;
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== C. 正常用 50MB（限制 125MB，应当 AC 且内存读数约 50MB） ====="
mk normal50 <<'EOF'
#include <cstdio>
#include <cstdlib>
#include <cstring>
int main() {
	size_t n = 50 << 20;
	char *p = (char *)malloc(n);
	if (!p) return 1;
	memset(p, 1, n);
	long sum = 0;
	for (size_t i = 0; i < n; i += 4096) sum += p[i];
	printf("1\n");
	return sum == 0 ? 2 : 0;
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== D. 真的要 200MB（超 125MB 限制，应当 MLE） ====="
mk over200 <<'EOF'
#include <cstdio>
#include <cstdlib>
int main() {
	size_t n = 200 << 20;
	char *p = (char *)malloc(n);
	if (!p) { printf("分配失败\n"); return 9; }
	long sum = 0;
	for (size_t i = 0; i < n; i += 4096) { p[i] = (char)(i & 0x7f); sum += p[i]; }
	printf("1\n");
	return sum == 0 ? 2 : 0;
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== E. 不检查 malloc 直接写（撞上限后段错误，应当 MLE 而非 RE） ====="
mk noccheck <<'EOF'
#include <cstdio>
#include <cstdlib>
int main() {
	size_t n = 200 << 20;
	volatile char *p = (volatile char *)malloc(n);
	for (size_t i = 0; i < n; i += 4096) p[i] = 1; // 分配失败就是空指针解引用
	printf("1\n");
	return 0;
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== F. vector 逐步 push_back 撑爆（最常见的写法，应当 MLE） ====="
mk vecpush <<'EOF'
#include <cstdio>
#include <vector>
int main() {
	std::vector<int> v;
	for (;;) v.push_back(1); // 不检查，bad_alloc 未捕获 → abort
}
EOF
/usr/bin/time -f "    墙钟 %es  峰值RSS %MkB" "$OTK" judge 2>&1 | tail -4

echo
echo "===== 内存余量检查（机器没被吃穿） ====="
free -m | head -2
