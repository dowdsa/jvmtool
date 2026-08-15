# jvmtool

使用 Go 编写的多版本 JDK (Temurin/Adoptium) 与 Maven 管理工具。

## 功能

- **search** 从远程源搜索可下载版本（JDK 走 Adoptium API，Maven 走 Apache 中央仓库）
- **install** 下载、校验（SHA256/SHA512）并解压安装，支持断点续传与进度条
- **use** 切换当前版本（维护 `current` 符号链接）
- **list** 列出已安装版本
- **uninstall** 卸载指定版本
- **current** 显示当前版本并给出环境变量提示
- **clean** 清理下载缓存

## 安装

### 方式一：一键脚本（下载预编译二进制，无需安装 Go）

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.sh)
```

脚本会自动：
1. 按系统/架构从 GitHub Releases 下载对应二进制（Linux/macOS x64/arm64）
2. 安装到 `/usr/local/bin`
3. 在 `~/.bashrc`/`~/.zshrc` 写入环境变量
4. 创建 `~/.jvmtool` 目录结构
5. 校验 SHA256 校验和

也可指定版本与安装目录：

```bash
bash install.sh v0.1.0          # 指定版本
JVMTOOL_PREFIX=$HOME/.local bash install.sh   # 安装到用户目录
```

### 方式二：源码编译

```bash
git clone https://github.com/dowdsa/jvmtool.git
cd jvmtool
go build -o jm .
sudo install -m 0755 jm /usr/local/bin/
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

## 从源码发布 Release

推送 `v*` 标签会自动触发 GitHub Actions 构建全平台二进制并创建 Release：

```bash
git tag v0.1.0
git push origin v0.1.0
```

构建矩阵: `linux_amd64` `linux_arm64` `darwin_amd64` `darwin_arm64` `windows_amd64`

## 测试

```bash
go test ./...
```

## 许可证

[Apache License 2.0](LICENSE)
