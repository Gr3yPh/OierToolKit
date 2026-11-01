#!/usr/bin/env bash
# 在 WSL(Ubuntu) 内准备 Linux 版的构建环境：
#   - Fyne 需要的 C 工具链与 X11/GL 头文件
#   - 与 Windows 侧一致的 Go 工具链（从阿里云镜像取，go.dev 在国内不可达）
set -euo pipefail

GO_VER="${GO_VER:-1.27.2}"
MIRROR="${MIRROR:-https://mirrors.aliyun.com/golang}"

echo "==> 1/3 安装 C 工具链与图形头文件"
export DEBIAN_FRONTEND=noninteractive
apt-get install -y --no-install-recommends \
	gcc g++ pkg-config \
	libgl1-mesa-dev libglu1-mesa-dev \
	xorg-dev libxkbcommon-dev libx11-dev libxrandr-dev libxcursor-dev \
	libxinerama-dev libxi-dev libxxf86vm-dev \
	libwayland-dev wayland-protocols libxkbcommon-x11-dev \
	ca-certificates curl >/dev/null
echo "    gcc: $(gcc -dumpversion)"

echo "==> 2/3 安装 Go ${GO_VER}"
need=1
if [ -x /usr/local/go/bin/go ]; then
	cur=$(/usr/local/go/bin/go env GOVERSION 2>/dev/null | sed 's/^go//')
	if [ "$cur" = "$GO_VER" ]; then
		need=0
		echo "    已存在 go${cur}，跳过下载"
	fi
fi
if [ "$need" = 1 ]; then
	url="${MIRROR}/go${GO_VER}.linux-amd64.tar.gz"
	echo "    下载 ${url}"
	curl -fsSL --retry 3 -o /tmp/go.tgz "$url"
	rm -rf /usr/local/go
	tar -C /usr/local -xzf /tmp/go.tgz
	rm -f /tmp/go.tgz
fi
/usr/local/go/bin/go version
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

echo "==> 3/3 环境自检"
printf '    CGO: '
echo 'int main(void){return 0;}' >/tmp/t.c && gcc /tmp/t.c -o /tmp/t && echo ok
printf '    GL 头文件: '
[ -f /usr/include/GL/gl.h ] && echo ok || echo 缺
printf '    X11 头文件: '
[ -f /usr/include/X11/Xlib.h ] && echo ok || echo 缺
echo "准备完成。"
