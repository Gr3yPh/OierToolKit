#!/usr/bin/env bash
# OierToolKit 一键安装脚本，覆盖 GNU/Linux 与 macOS（amd64/arm64）。
#
#   curl -fsSL https://otk.gr3yph4ntom.cn/install.sh | bash
#
# 环境变量：
#   OTK_INSTALL_DIR  指定安装目录（默认 /usr/local/bin，无权限时回退 ~/.local/bin）
#   OTK_SOURCE       mirror / github 强制下载源（默认国内镜像，失败自动回退 GitHub）
set -euo pipefail

REPO="Gr3yPh/OierToolKit"
MIRROR="https://otk.gr3yph4ntom.cn"

die() {
    echo "otk-install: $*" >&2
    exit 1
}

# --- 平台与架构 ---
case "$(uname -s)" in
    Linux)  platform=linux ;;
    Darwin) platform=darwin ;;
    *)      die "不支持的系统 $(uname -s)；Windows 请在 PowerShell 运行 install.ps1" ;;
esac
case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *)             die "不支持的架构 $(uname -m)" ;;
esac
asset="otk-$platform-$arch.tar.gz"

# --- 下载 ---
fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 2 "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --tries=2 -O "$2" "$1"
    else
        die "需要 curl 或 wget"
    fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

case "${OTK_SOURCE:-}" in
    github) urls="https://github.com/$REPO/releases/latest/download/$asset" ;;
    mirror) urls="$MIRROR/releases/$asset" ;;
    *)      urls="$MIRROR/releases/$asset https://github.com/$REPO/releases/latest/download/$asset" ;;
esac

ok=
for url in $urls; do
    if fetch "$url" "$tmp/$asset" 2>/dev/null; then
        ok=1
        break
    fi
done
[ -n "$ok" ] || die "下载失败（$asset）"

# --- 解包 ---
tar -xzf "$tmp/$asset" -C "$tmp"
src="$tmp/otk"
[ -f "$src" ] || die "归档里没找到 otk"
chmod +x "$src"

# --- 安装 ---
if [ -n "${OTK_INSTALL_DIR:-}" ]; then
    install_dir="$OTK_INSTALL_DIR"
    mkdir -p "$install_dir"
    cp "$src" "$install_dir/otk"
elif [ -w /usr/local/bin ]; then
    install_dir=/usr/local/bin
    cp "$src" "$install_dir/otk"
elif command -v sudo >/dev/null 2>&1; then
    install_dir=/usr/local/bin
    echo "将用 sudo 安装到 $install_dir ..."
    sudo mkdir -p /usr/local/bin
    sudo cp "$src" /usr/local/bin/otk
else
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    cp "$src" "$install_dir/otk"
    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *) echo "提示: 把 export PATH=\"\$HOME/.local/bin:\$PATH\" 加进 ~/.bashrc 或 ~/.zshrc" ;;
    esac
fi

echo "已安装 $("$install_dir/otk" version) → $install_dir/otk"

# --- GUI 入口：发行包里的 otk-gui 是单独编出的图形版入口（按文件名进 GUI）；
# 老包没有它时退回复制一份主程序，功能等价，只是要用 otk gui 命令打开。
# 有的系统 /usr/local/bin 需要 sudo 才能写，这里失败不算安装失败。
gui_src="$tmp/otk-gui"
[ -f "$gui_src" ] || gui_src="$src"
if cp "$gui_src" "$install_dir/otk-gui" 2>/dev/null || sudo cp "$gui_src" "$install_dir/otk-gui" 2>/dev/null; then
    chmod +x "$install_dir/otk-gui" 2>/dev/null || sudo chmod +x "$install_dir/otk-gui" 2>/dev/null
    echo "图形界面入口: $install_dir/otk-gui（也可以在设置里创建桌面快捷方式）"
else
    echo "提示: 图形界面可用 otk gui 打开；创建 otk-gui 副本失败（权限不足）"
fi
