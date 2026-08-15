<#
.SYNOPSIS
    jm - 多版本 JDK/Maven 管理工具 Windows 一键安装脚本
.DESCRIPTION
    从 GitHub Releases 下载预编译的 jm 二进制 (Windows amd64/arm64)，
    安装到用户目录，并配置环境变量。
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File install.ps1
    powershell -ExecutionPolicy Bypass -File install.ps1 -Version v0.1.0
#>

param(
    [string]$Version = "latest",
    [string]$RepoOwner = $env:JVMTOOL_REPO_OWNER,
    [string]$RepoName = $env:JVMTOOL_REPO_NAME
)

$ErrorActionPreference = "Stop"

# ---------- 可配置项 ----------
if (-not $RepoOwner) { $RepoOwner = "dowdsa" }
if (-not $RepoName)  { $RepoName  = "jvmtool" }

$ToolName    = "jm"
$BinDir      = Join-Path $HOME (Join-Path ".jvmtool" "bin")
$JvmToolHome = if ($env:JVMTOOL_HOME) { $env:JVMTOOL_HOME } else { Join-Path $HOME ".jvmtool" }
$BaseUrl     = "https://github.com/$RepoOwner/$RepoName/releases/download"
$ApiUrl      = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
$ProfilePath = $PROFILE.CurrentUserAllHosts

function Write-Ok   { Write-Host "[OK]   " -ForegroundColor Green -NoNewline; Write-Host $args }
function Write-Warn { Write-Host "[!!]   " -ForegroundColor Yellow -NoNewline; Write-Host $args }
function Write-Fail { Write-Host "[FAIL] " -ForegroundColor Red -NoNewline; Write-Host $args; exit 1 }

# ---------- 1. 平台检测 ----------
function Get-Platform {
    $arch = $env:PROCESSOR_ARCHITECTURE
    if (-not $arch) {
        $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    }
    if ($arch -eq "AMD64" -or $arch -eq "X64") { return "windows_amd64" }
    if ($arch -eq "ARM64") { return "windows_arm64" }
    Write-Fail "不支持的架构: $arch"
}

# ---------- 2. 版本解析 ----------
function Resolve-Version {
    if ($Version -ne "latest") {
        return $Version.TrimStart("v")
    }
    Write-Ok "查询最新版本..."
    try {
        $resp = Invoke-RestMethod -Uri $ApiUrl -Headers @{ "User-Agent" = "jm-installer" }
        $script:Version = $resp.tag_name.TrimStart("v")
        if (-not $script:Version) { throw "empty tag" }
    } catch {
        Write-Fail "无法获取最新版本，请检查 RepoOwner/RepoName 配置或网络。$($_.Exception.Message)"
    }
}

# ---------- 3. 下载并安装 ----------
function Install-Binary {
    $platform = Get-Platform
    Resolve-Version

    $asset = "$ToolName`_$platform.exe"
    $url   = "$BaseUrl/v$Version/$asset"

    Write-Ok "下载 jm $Version ($platform)"
    Write-Ok "来源: $url"

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $tmp = Join-Path $env:TEMP "$ToolName-installer"
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    $tmpFile = Join-Path $tmp $asset

    $ProgressPreference = "SilentlyContinue"
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmpFile -UseBasicParsing
    } catch {
        Write-Fail "下载失败: $($_.Exception.Message)"
    }
    $ProgressPreference = "Continue"

    # 校验 SHA256
    Verify-Checksum $tmpFile $platform

    Move-Item -Force $tmpFile (Join-Path $BinDir "$ToolName.exe")
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    Write-Ok "已安装到 $BinDir\$ToolName.exe"
}

# ---------- 4. 校验和 ----------
function Verify-Checksum {
    param([string]$File, [string]$Platform)
    $sumsUrl = "$BaseUrl/v$Version/SHA256SUMS.txt"
    try {
        $sums = (Invoke-WebRequest -Uri $sumsUrl -UseBasicParsing).Content
        $line = ($sums -split "`n") | Where-Object { $_ -match "$ToolName`_$Platform" } | Select-Object -First 1
        if ($line) {
            $expected = ($line -split "\s+")[0].ToLower()
            $actual   = (Get-FileHash -Path $File -Algorithm SHA256).Hash.ToLower()
            if ($expected -ne $actual) {
                Write-Fail "SHA256 校验失败，请重新运行安装"
            }
            Write-Ok "SHA256 校验通过"
        }
    } catch {
        Write-Warn "跳过 SHA256 校验 ($($_.Exception.Message))"
    }
}

# ---------- 5. 配置用户环境变量 ----------
function Set-UserEnv {
    $env:JVMTOOL_HOME = $JvmToolHome
    [Environment]::SetEnvironmentVariable("JVMTOOL_HOME", $JvmToolHome, "User")

    # 将 bin 目录加入用户 PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) { $userPath = "" }
    $paths = $userPath -split ";" | Where-Object { $_ -ne "" }
    if ($paths -notcontains $BinDir) {
        $paths += $BinDir
        [Environment]::SetEnvironmentVariable("Path", ($paths -join ";"), "User")
        Write-Ok "已将 $BinDir 加入用户 PATH"
    } else {
        Write-Ok "用户 PATH 已包含 $BinDir"
    }

    # 写入 PowerShell profile 自动设置 JAVA_HOME / M2_HOME
    $marker = "# >>> jm >>>"
    $block  = @"
$marker
`$env:JVMTOOL_HOME = "$JvmToolHome"
`$jdk = Join-Path `$env:JVMTOOL_HOME "jdk\current"
`$mvn = Join-Path `$env:JVMTOOL_HOME "maven\current"
if (Test-Path `$jdk) {
    `$env:JAVA_HOME = `$jdk
    `$env:Path = "`$jdk\bin;`$env:Path"
}
if (Test-Path `$mvn) {
    `$env:M2_HOME = `$mvn
    `$env:MAVEN_HOME = `$mvn
    `$env:Path = "`$mvn\bin;`$env:Path"
}
$marker
"@

    if (-not (Test-Path $ProfilePath)) {
        New-Item -ItemType File -Force -Path $ProfilePath | Out-Null
    }
    $content = Get-Content $ProfilePath -Raw -ErrorAction SilentlyContinue
    $lines   = $content -split "`r?`n"
    $out     = @()
    $skip    = $false
    foreach ($l in $lines) {
        if ($l -match $marker) {
            $skip = -not $skip
            continue
        }
        if (-not $skip) { $out += $l }
    }
    $out += ""
    $out += $block
    Set-Content -Path $ProfilePath -Value ($out -join "`r`n") -Encoding UTF8

    Write-Ok "环境变量已写入用户环境 + PowerShell Profile ($ProfilePath)"
    Write-Ok "JVMTOOL_HOME=$JvmToolHome"
}

# ---------- 6. 创建目录结构 ----------
function Init-Dirs {
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "jdk")    | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "maven")  | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "cache")  | Out-Null
    Write-Ok "目录结构已创建: $JvmToolHome"
}

# ---------- 主流程 ----------
Write-Ok "开始安装 jm (版本: $Version)"
Install-Binary
Set-UserEnv
Init-Dirs

Write-Host ""
Write-Ok "安装完成！"
Write-Host ""
Write-Host "  使用方法:"
Write-Host "    打开新 PowerShell 窗口后即可使用"
Write-Host "    jm jdk search 21     # 搜索版本"
Write-Host "    jm jdk install 21    # 安装"
Write-Host "    jm jdk use 21        # 切换版本"
Write-Host ""
