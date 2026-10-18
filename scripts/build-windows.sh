#!/usr/bin/env bash
# 构建 OierToolKit 的 Windows 入口。
#
# 为什么不能只编一次：
#   链接器把可执行文件定成 console 还是 windows 子系统是**构建期**决定，
#   同一份二进制没法在启动时按文件名切换。而两者的差别很实在：
#     - console 子系统：双击会先弹一个黑框（系统自动分配控制台）；
#     - windows 子系统：没有控制台，双击直接出窗口。
#   所以命令行版必须编成 console，图形版必须编成 windows。
#
# 关于 resource_windows_amd64.syso（goversioninfo 生成的图标 + 版本信息）：
#   两份都要带上，它的 .rsrc 段不会盖掉 -H windowsgui —— 实测同一份源码
#   加 -H windowsgui 编出来的子系统和段表都正确（Subsystem=2 且 .rsrc 在）。
#   所以图形版既有资源管理器里的图标与文件属性，又不会弹黑框。
#
# 用法：scripts/build-windows.sh [版本号]
#   版本号写进 -X main.otkVersion，默认 v2.5.0。

set -euo pipefail

cd "$(dirname "$0")/.."

VER="${1:-v2.5.0}"
FLAGS="-s -w -X main.otkVersion=${VER}"
SYSO="resource_windows_amd64.syso"

if [ ! -f "${SYSO}" ]; then
	echo "缺少 ${SYSO}，先生成图标与版本信息：" >&2
	echo "  go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.7.0 -o ${SYSO} versioninfo.json" >&2
	exit 1
fi

echo "==> otk-gui.exe（windows 子系统 + 图标/版本信息，双击不弹控制台）"
go build -ldflags "${FLAGS} -H windowsgui" -o otk-gui.exe .

echo "==> otk.exe（console 子系统 + 图标/版本信息）"
go build -ldflags "${FLAGS}" -o otk.exe .

echo "==> 自检"
for f in otk.exe otk-gui.exe; do
	printf '    %-12s %-6s subsystem=%s\n' "${f}" "$(du -h "${f}" | cut -f1)" \
		"$(python -c "
import struct
d = open('${f}', 'rb').read()
o = struct.unpack_from('<I', d, 0x3c)[0]
print(struct.unpack_from('<H', d, o + 24 + 68)[0])
" 2>/dev/null || echo '?')"
done
echo "    （3 = console，2 = windows GUI）"
