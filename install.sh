#!/usr/bin/env bash
# OierToolKit 一键安装脚本，覆盖 GNU/Linux 与 macOS（amd64/arm64）。
#
#   curl -fsSL https://otk.gr3yph4ntom.cn/install.sh | bash
#
# 环境变量：
#   OTK_INSTALL_DIR  指定安装目录（默认 /usr/local/bin，无权限时回退 ~/.local/bin）
#   OTK_SOURCE       ghproxy / github 强制下载源（默认 gh-proxy，失败自动回退 GitHub）
set -euo pipefail

REPO="Gr3yPh/OierToolKit"
GHPROXY="https://gh-proxy.com"

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
# 下载时保留进度条：30MB 安装包在慢速网络下要等很久，
# 静默下载很容易让人以为脚本卡死。
fetch() {
    if command -v curl >/dev/null 2>&1; then
        if [ -t 2 ]; then
            # 终端里给进度条；重定向到日志时保持安静，避免刷屏。
            curl -fL --retry 2 --progress-bar --connect-timeout 15 "$1" -o "$2"
        else
            curl -fsSL --retry 2 --connect-timeout 15 "$1" -o "$2"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -t 2 ]; then
            wget --tries=2 --timeout=15 -O "$2" "$1"
        else
            wget -q --tries=2 --timeout=15 -O "$2" "$1"
        fi
    else
        die "需要 curl 或 wget"
    fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

case "${OTK_SOURCE:-}" in
    github) urls="https://github.com/$REPO/releases/latest/download/$asset" ;;
    mirror|ghproxy) urls="$GHPROXY/https://github.com/$REPO/releases/latest/download/$asset" ;;
    *)      urls="$GHPROXY/https://github.com/$REPO/releases/latest/download/$asset https://github.com/$REPO/releases/latest/download/$asset" ;;
esac

ok=
for url in $urls; do
    echo "正在下载 $asset ..."
    echo "  源: $url"
    if fetch "$url" "$tmp/$asset"; then
        ok=1
        break
    fi
    echo "下载失败，尝试下一个源 ..." >&2
done
[ -n "$ok" ] || die "下载失败（$asset）；可用 OTK_SOURCE=github 强制从 GitHub 下载"

# --- 解包 ---
echo "下载完成，正在解压 ..."
if ! tar -xzf "$tmp/$asset" -C "$tmp"; then
    die "解压失败（$asset 可能不完整，请重试）"
fi

# 正式包内通常是 otk；兼容旧包里带平台后缀的文件名。
src="$tmp/otk"
if [ ! -f "$src" ]; then
    for candidate in "$tmp/otk-$platform-$arch" "$tmp/otk-gui"; do
        if [ -f "$candidate" ]; then
            src="$candidate"
            break
        fi
    done
fi
if [ ! -f "$src" ]; then
    echo "otk-install: 归档内容：" >&2
    tar -tzf "$tmp/$asset" 2>/dev/null | sed 's/^/  /' >&2
    die "归档里没有可执行文件 otk；请重试，或用 OTK_SOURCE=github 从 GitHub 下载"
fi
chmod +x "$src"

# --- 安装 ---
if [ -n "${OTK_INSTALL_DIR:-}" ]; then
    install_dir="$OTK_INSTALL_DIR"
    mkdir -p "$install_dir"
    echo "正在安装到 $install_dir ..."
    cp "$src" "$install_dir/otk"
elif [ -w /usr/local/bin ]; then
    install_dir=/usr/local/bin
    echo "正在安装到 $install_dir ..."
    cp "$src" "$install_dir/otk"
elif command -v sudo >/dev/null 2>&1; then
    install_dir=/usr/local/bin
    echo "正在安装到 $install_dir（需要 sudo）..."
    sudo mkdir -p /usr/local/bin
    sudo cp "$src" /usr/local/bin/otk
else
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    echo "正在安装到 $install_dir ..."
    cp "$src" "$install_dir/otk"
    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *) echo "提示: 把 export PATH=\"\$HOME/.local/bin:\$PATH\" 加进 ~/.bashrc 或 ~/.zshrc" ;;
    esac
fi

version=$("$install_dir/otk" version 2>&1) || die "安装后无法运行 $install_dir/otk：$version"
echo "已安装 $version → $install_dir/otk"

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
