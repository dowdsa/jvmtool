<#
.SYNOPSIS
    jm - Multi-version JDK/Maven manager - Windows installer
.DESCRIPTION
    Downloads the prebuilt jm binary (Windows amd64/arm64) from GitHub Releases,
    installs it, and configures environment variables.
    Two ways to run:
      Way 1 (recommended, direct pipe):
        iwr https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.ps1 -useb | iex
      Way 2 (download then run):
        powershell -ExecutionPolicy Bypass -File install.ps1
      Specify version: set env var JVMTOOL_VERSION (default latest)
.EXAMPLE
    iwr https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.ps1 -useb | iex
    $env:JVMTOOL_VERSION = "v0.1.0"; iwr https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.ps1 -useb | iex
#>

$ErrorActionPreference = "Stop"

# Set UTF-8 console output encoding
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
$OutputEncoding = [System.Text.Encoding]::UTF8

# ---------- Config ----------
# NOTE: param() block is intentionally NOT used for compatibility with
# "irm <url> | iex" pipe execution. Version can be set via JVMTOOL_VERSION.
$Version    = if ($env:JVMTOOL_VERSION) { $env:JVMTOOL_VERSION } else { "latest" }
$RepoOwner  = if ($env:JVMTOOL_REPO_OWNER) { $env:JVMTOOL_REPO_OWNER } else { "dowdsa" }
$RepoName   = if ($env:JVMTOOL_REPO_NAME) { $env:JVMTOOL_REPO_NAME } else { "jvmtool" }

$ToolName    = "jm"
$BinDir      = Join-Path $HOME (Join-Path ".jvmtool" "bin")
$JvmToolHome = if ($env:JVMTOOL_HOME) { $env:JVMTOOL_HOME } else { Join-Path $HOME ".jvmtool" }
$BaseUrl     = "https://github.com/$RepoOwner/$RepoName/releases/download"
$LatestUrl   = "https://github.com/$RepoOwner/$RepoName/releases/latest/download"
$ProfilePath = $PROFILE.CurrentUserAllHosts

function Write-Ok   { Write-Host "[OK]   " -ForegroundColor Green -NoNewline; Write-Host $args }
function Write-Warn { Write-Host "[!!]   " -ForegroundColor Yellow -NoNewline; Write-Host $args }
function Write-Fail { Write-Host "[FAIL] " -ForegroundColor Red -NoNewline; Write-Host $args; throw $args }

# ---------- 1. Platform detection ----------
function Get-Platform {
    $arch = $env:PROCESSOR_ARCHITECTURE
    if (-not $arch) {
        $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    }
    if ($arch -eq "AMD64" -or $arch -eq "X64") { return "windows_amd64" }
    if ($arch -eq "ARM64") { return "windows_arm64" }
    Write-Fail "Unsupported architecture: $arch"
}

# ---------- 2. Version resolution ----------
# 使用 GitHub 的 /releases/latest/download/ 免 API 下载 latest，
# 避免 api.github.com 被代理/防火墙拦截导致失败。
function Resolve-Version {
    if ($Version -eq "latest") {
        return "latest"
    }
    return $Version.TrimStart("v")
}

# ---------- 3. Download and install ----------
function Install-Binary {
    $platform = Get-Platform
    $Version   = Resolve-Version

    $asset = "$ToolName`_$platform.exe"
    if ($Version -eq "latest") {
        $url = "$LatestUrl/$asset"
    } else {
        $url = "$BaseUrl/v$Version/$asset"
    }

    Write-Ok "Downloading jm $Version ($platform)"
    Write-Ok "URL: $url"

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $tmp = Join-Path $env:TEMP "$ToolName-installer"
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    $tmpFile = Join-Path $tmp $asset

    $ProgressPreference = "SilentlyContinue"
    try {
        iwr -Uri $url -OutFile $tmpFile -UseBasicParsing
    } catch {
        Write-Fail "Download failed: $($_.Exception.Message)"
    }
    $ProgressPreference = "Continue"

    # Verify SHA256
    Verify-Checksum $tmpFile $platform

    Move-Item -Force $tmpFile (Join-Path $BinDir "$ToolName.exe")
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    Write-Ok "Installed to $BinDir\$ToolName.exe"
}

# ---------- 4. Checksum verification ----------
function Verify-Checksum {
    param([string]$File, [string]$Platform)
    if ($Version -eq "latest") {
        $sumsUrl = "$LatestUrl/SHA256SUMS.txt"
    } else {
        $sumsUrl = "$BaseUrl/v$Version/SHA256SUMS.txt"
    }
    try {
        $sums = (iwr -Uri $sumsUrl -UseBasicParsing).Content
        $line = ($sums -split "`n") | Where-Object { $_ -match "$ToolName`_$Platform" } | Select-Object -First 1
        if ($line) {
            $expected = ($line -split "\s+")[0].ToLower()
            $actual   = (Get-FileHash -Path $File -Algorithm SHA256).Hash.ToLower()
            if ($expected -ne $actual) {
                Write-Fail "SHA256 checksum mismatch. Please re-run the installer."
            }
            Write-Ok "SHA256 checksum verified"
        }
    } catch {
        Write-Warn "Skipped SHA256 verification ($($_.Exception.Message))"
    }
}

# ---------- 5. Configure user environment ----------
function Set-UserEnv {
    $env:JVMTOOL_HOME = $JvmToolHome
    [Environment]::SetEnvironmentVariable("JVMTOOL_HOME", $JvmToolHome, "User")

    # Add bin dir to user PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) { $userPath = "" }
    $paths = $userPath -split ";" | Where-Object { $_ -ne "" }
    if ($paths -notcontains $BinDir) {
        $paths += $BinDir
        [Environment]::SetEnvironmentVariable("Path", ($paths -join ";"), "User")
        Write-Ok "Added $BinDir to user PATH"
    } else {
        Write-Ok "User PATH already contains $BinDir"
    }

    # Write PowerShell profile to auto-set JAVA_HOME / M2_HOME
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

    Write-Ok "Environment variables written to user env + PowerShell Profile ($ProfilePath)"
    Write-Ok "JVMTOOL_HOME=$JvmToolHome"
}

# ---------- 6. Create directories ----------
function Init-Dirs {
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "jdk")    | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "maven")  | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $JvmToolHome "cache")  | Out-Null
    Write-Ok "Directory structure created: $JvmToolHome"
}

# ---------- Main flow ----------
try {
    Write-Ok "Installing jm (version: $Version)"
    Install-Binary
    Set-UserEnv
    Init-Dirs

    Write-Host ""
    Write-Ok "Installation completed!"
    Write-Host ""
    Write-Host "  Usage:"
    Write-Host "    Open a new PowerShell window, then:"
    Write-Host "    jm jdk search 21     # search versions"
    Write-Host "    jm jdk install 21    # install a version"
    Write-Host "    jm jdk use 21        # switch active version"
    Write-Host ""
} catch {
    # Show the error without re-throwing so the session is not closed
    # when running under "irm | iex" pipe execution.
    Write-Host ""
    Write-Host "Installation aborted: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "For help, visit https://github.com/dowdsa/jvmtool" -ForegroundColor Cyan
}
