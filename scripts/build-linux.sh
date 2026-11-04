#!/usr/bin/env bash
# 在 WSL(Ubuntu) 内构建 Linux 版：otk 与 otk-gui。
#
# 为什么要在 Linux 里编：
#   otk-gui 依赖 Fyne/OpenGL，链接期需要 X11/GL 的头文件与 .so，
#   Windows 上没有 Linux 交叉工具链就编不出来；otk 虽然逻辑上纯 Go，
#   但它和 GUI 同处一个 main 包、无条件 import Fyne，CGO 关掉同样报错。
#
# 为什么先把源码拷到 $HOME：
#   /mnt/d 是 drvfs 挂载，cgo 编译要反复读写大量中间文件，放在上面会慢一个量级。
#
# 用法：scripts/build-linux.sh [版本号]
#   产物：仓库根目录 otk-linux、otk-gui-linux（已 chmod +x）。
set -euo pipefail

SRC=/mnt/d/OierToolKit
WORK="${HOME}/otkbuild"
VER="${1:-v2.10.0}"
export PATH="/usr/local/go/bin:${PATH}"
export GOPROXY=https://goproxy.cn,direct
export GOTOOLCHAIN=local
export CGO_ENABLED=1

echo "==> 1/4 同步源码到 ${WORK}"
rm -rf "${WORK}"
mkdir -p "${WORK}"
tar -C "${SRC}" \
	--exclude='./.git' --exclude='./contests' --exclude='./.workbuddy' \
	--exclude='*.exe' --exclude='*.zip' --exclude='./.build' \
	--exclude='./otk-linux' --exclude='./otk-gui-linux' \
	-cf - . | tar -C "${WORK}" -xf -
echo "    文件数 $(find "${WORK}" -type f | wc -l)"

echo "==> 2/4 依赖检查"
cd "${WORK}"
go mod download 2>&1 | tail -3 || true

echo "==> 3/4 编译"
FLAGS="-s -w -X main.otkVersion=${VER}"
go build -trimpath -buildvcs=false -ldflags "${FLAGS}" -o otk-linux .
go build -trimpath -buildvcs=false -ldflags "${FLAGS}" -o otk-gui-linux .

echo "==> 4/4 回传产物"
cp otk-linux otk-gui-linux "${SRC}/"
chmod +x "${SRC}/otk-linux" "${SRC}/otk-gui-linux"
for f in otk-linux otk-gui-linux; do
	printf '    %-14s %-7s %s\n' "${f}" "$(du -h "${SRC}/${f}" | cut -f1)" "$(file -b "${SRC}/${f}" | cut -c1-60)"
done
