# OierToolKit 一键安装脚本，Windows（x64 / ARM64）。
#
#   powershell -NoProfile -c "iwr -UseBasicParsing https://otk.gr3yph4ntom.cn/install.ps1 | iex"
#
# 上面这条 cmd 与 PowerShell 里都能直接粘；在 PowerShell 里也可以简写成
#   iwr -UseBasicParsing https://otk.gr3yph4ntom.cn/install.ps1 | iex
#
# 环境变量：
#   OTK_INSTALL_DIR  指定安装目录（默认 %LOCALAPPDATA%\Programs\otk）
#   OTK_SOURCE       mirror / github 强制下载源（默认镜像优先，失败回退 GitHub）

& {
    $ErrorActionPreference = 'Stop'
    $repo   = 'Gr3yPh/OierToolKit'
    $mirror = 'https://otk.gr3yph4ntom.cn'

    # --- 架构（32 位 PowerShell 跑在 64 位系统上时取原生架构） ---
    $raw = $env:PROCESSOR_ARCHITECTURE
    if ($raw -eq 'x86' -and $env:PROCESSOR_ARCHITEW6432) { $raw = $env:PROCESSOR_ARCHITEW6432 }
    switch ($raw) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'amd64' }   # 暂无 Windows arm64 构建，经 x64 模拟层运行
        default { throw "otk-install: unsupported architecture $raw" }
    }
    $asset = "otk-windows-$arch.exe.zip"

    # --- 下载（镜像优先，失败自动回退 GitHub） ---
    switch ($env:OTK_SOURCE) {
        'github' { $urls = @("https://github.com/$repo/releases/latest/download/$asset") }
        'mirror' { $urls = @("$mirror/releases/$asset") }
        default  { $urls = @("$mirror/releases/$asset", "https://github.com/$repo/releases/latest/download/$asset") }
    }
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch {}

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('otk-install-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        $zip = Join-Path $tmp $asset
        $ok = $false
        foreach ($u in $urls) {
            try {
                Invoke-WebRequest -Uri $u -OutFile $zip -UseBasicParsing -TimeoutSec 120
                $ok = $true
                break
            } catch {}
        }
        if (-not $ok) { throw "otk-install: download failed: $asset" }

        Expand-Archive -Path $zip -DestinationPath $tmp -Force

        # 归档内是 otk-windows-<arch>.exe，安装为 otk.exe。
        # 兜底查找时要排除 otk-gui.exe：那份是 windows 子系统的图形版，
        # 拿它充当命令行程式会看不到任何输出。
        $src = Join-Path $tmp "otk-windows-$arch.exe"
        if (-not (Test-Path $src)) {
            $found = Get-ChildItem -Path $tmp -Filter '*.exe' |
                Where-Object { $_.Name -ne 'otk-gui.exe' } |
                Select-Object -First 1
            if (-not $found) { throw 'otk-install: otk.exe not found in archive' }
            $src = $found.FullName
        }

        # --- 安装到用户目录，无需管理员权限 ---
        $installDir = if ($env:OTK_INSTALL_DIR) { $env:OTK_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\otk' }
        New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        Copy-Item -Path $src -Destination (Join-Path $installDir 'otk.exe') -Force

        # --- 确保安装目录在用户 PATH 里（保留注册表值类型） ---
        $reg = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
        if ($null -eq $reg) { throw 'otk-install: cannot open user environment (HKCU\Environment)' }
        try {
            $old = [string]$reg.GetValue('Path', '')
            $kind = $reg.GetValueKind('Path')
        } catch {
            $old = ''
            $kind = [Microsoft.Win32.RegistryValueKind]::String
        }
        $entries = @($old -split ';' | Where-Object { $_ })
        if ($entries -notcontains $installDir) {
            $reg.SetValue('Path', (@($entries) + $installDir) -join ';', $kind)
            # 广播环境变量变更，新开的终端立即生效
            try {
                if (-not ('OtkEnv' -as [type])) {
                    Add-Type -Namespace Otk -Name Env -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
                }
                $r = [UIntPtr]::Zero
                [void][Otk.Env]::SendMessageTimeout([IntPtr]0xffff, 0x1a, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$r)
            } catch {}
            Write-Host "added $installDir to user PATH (new terminals pick it up)"
        }
        $reg.Close()

        Write-Host "installed: $(Join-Path $installDir 'otk.exe')"
        & (Join-Path $installDir 'otk.exe') version

        # --- GUI 入口 ---
        # 发行包里的 otk-gui.exe 是单独编的（windows 子系统，双击不会先弹黑框），
        # 优先用它；老版本只有单个 exe 时才退回复制一份——那种副本与命令行版
        # 子系统相同，双击会闪一下控制台，但功能不受影响。
        $guiSrc = Join-Path $tmp 'otk-gui.exe'
        if (-not (Test-Path $guiSrc)) { $guiSrc = $src }
        try {
            Copy-Item -Path $guiSrc -Destination (Join-Path $installDir 'otk-gui.exe') -Force
            Write-Host "gui: $(Join-Path $installDir 'otk-gui.exe')  (双击即可，或在设置里创建桌面快捷方式)"
        } catch {
            Write-Host 'warn: could not create otk-gui.exe; use "otk gui" instead'
        }

        # 当前窗口立即可用。直接在 PowerShell 里跑时改的是用户自己的
        # 会话；经 powershell -c 嵌套执行时子进程改不了父窗口的环境块
        # （Windows 进程环境私有），只能给出补救的一行。
        $env:Path = "$installDir;$env:Path"
        try {
            $nested = (Get-CimInstance Win32_Process -Filter "ProcessId = $PID").CommandLine -match 'install\.ps1'
        } catch {
            $nested = $true
        }
        if ($nested) {
            Write-Host ('this window (cmd):        set "PATH=%PATH%;{0}"' -f $installDir)
            Write-Host ("this window (PowerShell): `$env:Path += ';{0}'" -f $installDir)
        } else {
            Write-Host 'this session: otk is ready'
        }
    } finally {
        Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}
