# jvmtool

使用 Go 编写的多版本 JDK (Temurin/Adoptium) 与 Maven 管理工具。

## 功能

- **search** 从远程源搜索可下载版本（JDK 走 Adoptium API，Maven 走 Apache 中央仓库）
- **install** 下载、校验（SHA256/SHA512）并解压安装，支持断点续传与进度条
- **use** 切换当前版本（维护 `current` 符号链接），并自动配置 shell 环境变量
- **list** 列出已安装版本
- **uninstall** 卸载指定版本，并自动清理对应的环境变量
- **current** 显示当前版本并给出环境变量提示
- **clean** 清理下载缓存
- **env** 查看/清理 shell 环境变量配置（`jm env show` / `jm env clean`）
- **桌面端** Windows GUI（基于 Wails），可视化列表/切换/搜索安装/卸载

## 安装

### 方式一：一键脚本（下载预编译二进制，无需安装 Go）

**Linux / macOS**

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.sh)
```

**Windows (PowerShell)**

```powershell
# 推荐：直接管道执行，无需下载脚本文件
iwr https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.ps1 -useb | iex
```

或先下载脚本再运行：

```powershell
irm https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.ps1 -o install.ps1
powershell -ExecutionPolicy Bypass -File install.ps1
```

脚本会自动：
1. 按系统/架构从 GitHub Releases 下载对应二进制（Linux/macOS/Windows × x64/arm64）
2. 安装到可执行目录（Unix: `/usr/local/bin`，Windows: `%USERPROFILE%\.jvmtool\bin`）
3. 写入环境变量（Unix: `~/.bashrc`/`~/.zshrc`，Windows: 用户环境变量 + PowerShell Profile）
4. 创建 `~/.jvmtool` 目录结构
5. 校验 SHA256 校验和

也可指定版本与安装目录：

```bash
bash install.sh v0.1.0          # 指定版本
JVMTOOL_PREFIX=$HOME/.local bash install.sh   # 安装到用户目录
```

### 方式二：Windows 桌面端

从 GitHub Releases 下载 `jm-desktop_windows_amd64.exe`（随 v0.3.0 起发布），双击即可使用图形界面。

功能：已安装列表（标记当前版本）、一键切换、远程搜索 + 安装、卸载版本。

### 方式三：源码构建桌面端

```bash
# 需安装 Wails CLI (go install github.com/wailsapp/wails/v2/cmd/wails@latest) 和 Node.js
cd desktop/jm-desktop
wails build -platform windows/amd64
# 产物: build/bin/jm-desktop.exe
```

## 使用

```bash
# 搜索版本
jm jdk search 17
jm maven search 3.9

# 安装
jm jdk install 21      # 支持部分版本号，自动解析最新
jm maven install 3.9.11

# 切换当前版本
jm jdk use 21
jm maven use 3.9.11

# 查看已安装 / 当前版本
jm jdk list
jm jdk current

# 诊断环境、代理和当前版本
jm doctor

# 导入已有安装（不移动原目录）
jm jdk import /path/to/jdk-21.0.1
jm maven import /path/to/apache-maven-3.9.11

# 生成 shell 补全
jm completion bash > ~/.local/share/bash-completion/completions/jm
jm completion zsh > ~/.zfunc/_jm
# PowerShell: jm completion powershell | Out-String | Invoke-Expression

# 卸载
jm jdk uninstall 21.0.12+8

# 清理缓存
jm clean
```

安装脚本会在 shell 中自动配置环境变量，新终端直接可用 `java`/`mvn`：

```bash
export JAVA_HOME=$JVMTOOL_HOME/jdk/current
export M2_HOME=$JVMTOOL_HOME/maven/current
```

切换版本后重新加载 shell 即生效：`source ~/.bashrc`。

## 目录结构

默认安装根目录为 `$HOME/.jvmtool`，可用环境变量 `JVMTOOL_HOME` 覆盖。

```
<root>/
├── jdk/<version>/     已安装的 JDK
├── maven/<version>/   已安装的 Maven
├── jdk/current        指向当前 JDK 的符号链接
├── maven/current      指向当前 Maven 的符号链接
└── cache/             下载缓存 (.tar.gz)
```

## 代理配置

工具自动读取代理环境变量，优先级从高到低：

1. `JVMTOOL_PROXY`（工具专用）
2. `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY`（标准代理变量）

支持 `http://`、`https://`、`socks5://` 协议，例如：

```bash
# Linux / macOS
export JVMTOOL_PROXY=http://127.0.0.1:7890

# Windows PowerShell
$env:JVMTOOL_PROXY = "http://127.0.0.1:7890"
```

设置后无需重启工具，下一次下载即生效。

Windows 桌面端还会自动读取当前用户的系统代理设置（Internet Settings）。
如果代理是在应用启动后才修改的，桌面端会在下一次网络请求创建客户端时重新读取；
PAC 自动配置脚本仍建议转换为固定的 HTTP 代理地址，或使用 `JVMTOOL_PROXY` 明确指定。

## 许可证

[Apache License 2.0](LICENSE)
