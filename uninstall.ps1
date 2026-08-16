<#
.SYNOPSIS
    jm 卸载脚本 (Windows PowerShell)
.DESCRIPTION
    清理 jm CLI: 二进制、用户环境变量 (JVMTOOL_HOME/JAVA_HOME/M2_HOME/MAVEN_HOME/PATH)、
    PowerShell Profile 环境变量块、以及数据目录。
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File uninstall.ps1
    # 保留已安装的 JDK/Maven 数据: 加 -KeepData
    powershell -ExecutionPolicy Bypass -File uninstall.ps1 -KeepData
#>

param(
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

$ToolName    = "jm"
$BinDir      = Join-Path $HOME (Join-Path ".jvmtool" "bin")
$JvmToolHome = if ($env:JVMTOOL_HOME) { $env:JVMTOOL_HOME } else { Join-Path $HOME ".jvmtool" }
$ProfilePath = $PROFILE.CurrentUserAllHosts

function Write-Ok   { Write-Host "[OK]   " -ForegroundColor Green -NoNewline; Write-Host $args }
function Write-Warn { Write-Host "[!!]   " -ForegroundColor Yellow -NoNewline; Write-Host $args }

# ---------- 1. 删除二进制 ----------
$exe = Join-Path $BinDir "$ToolName.exe"
if (Test-Path $exe) {
    Remove-Item -Force $exe
    Write-Ok "Removed binary: $exe"
} else {
    Write-Warn "Binary not found: $exe"
}

# ---------- 2. 清理用户环境变量 ----------
foreach ($name in @("JVMTOOL_HOME", "JAVA_HOME", "M2_HOME", "MAVEN_HOME")) {
    [Environment]::SetEnvironmentVariable($name, $null, "User")
}
Write-Ok "Cleared user env vars (JVMTOOL_HOME/JAVA_HOME/M2_HOME/MAVEN_HOME)"

# 从用户 PATH 中移除 bin 目录
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $paths = $userPath -split ";" | Where-Object { $_ -and ($_ -ne $BinDir) }
    [Environment]::SetEnvironmentVariable("Path", ($paths -join ";"), "User")
    Write-Ok "Removed $BinDir from user PATH"
}

# ---------- 3. 清理 PowerShell Profile 环境变量块 ----------
$marker = "# >>> jm >>>"
if (Test-Path $ProfilePath) {
    $content = Get-Content $ProfilePath -Raw -ErrorAction SilentlyContinue
    if ($content -match $marker) {
        $lines = $content -split "`r?`n"
        $out = @()
        $skip = $false
        foreach ($l in $lines) {
            if ($l -match $marker) { $skip = -not $skip; continue }
            if (-not $skip) { $out += $l }
        }
        Set-Content -Path $ProfilePath -Value ($out -join "`r`n") -Encoding UTF8
        Write-Ok "Cleaned env block from PowerShell Profile"
    }
}

# ---------- 4. 清理数据目录 ----------
if ($KeepData) {
    Write-Warn "Keeping data directory: $JvmToolHome"
} else {
    if (Test-Path $JvmToolHome) {
        Remove-Item -Recurse -Force $JvmToolHome
        Write-Ok "Removed data directory: $JvmToolHome"
    }
}

Write-Host ""
Write-Ok "Uninstall complete."
Write-Host "  Tip: use -KeepData to keep installed JDK/Maven data."
