# jvmtool 项目记忆

## 项目概述

Go 编写的多版本 JDK / Maven 管理工具，CLI 命令为 `jm`。支持 Temurin (Adoptium) 和 Zulu (Azul) 两种 JDK 发行版。包含 Windows 桌面端（Wails GUI）。

## 技术栈

- **语言**: Go 1.25
- **CLI**: spf13/cobra
- **桌面端**: Wails v2 (Go + 前端)
- **构建**: CGO_ENABLED=0, 交叉编译 6 个平台
- **CI**: GitHub Actions (`release.yml` — tag push 触发)
- **模块名**: `jm`

## 项目结构

```
main.go                    # 入口，调用 cmd.Execute()
cmd/
  root.go                  # Cobra 根命令、注册子命令、--distro 标志
  tools.go                 # search/install/list/use/uninstall/current/import 命令实现
  status.go                # jm status 全局状态命令
  info.go                  # jm jdk info / jm maven info 版本详情
  use.go                   # jm use (无参数，读 .jvmtoolrc)
  cache.go                 # jm cache list/size/clean
  env.go                   # jm env show/clean
  doctor.go                # jm doctor 环境诊断
  completion.go            # shell 补全生成
pkg/
  config/
    config.go              # Config 结构、HTTPClient 缓存、代理解析
    socks5.go              # SOCKS5 代理实现（标准库不支持）
    proxy_bypass.go        # NO_PROXY 绕过逻辑
    system_proxy_windows.go # Windows Internet Settings 代理读取
    system_proxy_other.go   # Unix 系统代理（空实现）
  download/
    download.go            # 下载器（断点续传、重试、进度条）、HumanSize
  env/
    env.go                 # shell rc 文件读写（Block/ApplyBlock/RemoveBlock）
    env_windows.go         # Windows 注册表操作（用户环境变量、PATH）
    env_other.go           # Unix 空实现
  manager/
    manager.go             # 核心逻辑：Install/Use/Uninstall/Import、文件锁、matchInstalled
    extract.go             # tar.gz / zip 解压、符号链接安全校验
    import.go              # 已有 JDK/Maven 目录导入
  project/
    project.go             # .jvmtoolrc 解析（从 CWD 向上查找）
  update/
    update.go              # GitHub Release 检查、CLI/桌面端自更新
  version/
    version.go             # Artifact 结构、Source 接口、CompareVersions/VersionParts
    jdk.go                 # Adoptium API (Temurin)、NewJDKSourceForDistro 工厂
    zulu.go                # Azul Zulu API
    maven.go               # Maven Central metadata XML
desktop/
  jm-desktop/
    main.go                # Wails 应用入口
    app.go                 # GUI 业务逻辑（List/Search/Install/Use/Uninstall）
    autostart_windows.go   # 开机自启（注册表 Run 键）
    frontend/              # Vite + 原生 JS 前端
```

## 关键设计决策

1. **HTTP Client 缓存**: `config.HTTPClient()` 返回缓存的单例，代理变更时自动重建。`HTTPClientWithTimeout` 共享 Transport 但不污染缓存。
2. **文件锁**: `O_CREATE|O_EXCL` 原子创建 `.jm.lock`，写入 PID。首次遇到已有锁时检查文件年龄，超过 5 分钟自动清理（stale lock 检测）。
3. **版本匹配**: `matchInstalled` 三级优先级 — 精确 > 前缀+点 > 唯一前缀匹配，防止 `jm jdk use 1` 静默选中 11。
4. **版本比较**: 统一用 `version.CompareVersions`（导出），`update` 包和 `manager` 包共享。
5. **镜像优先**: 安装时先尝试 TUNA/华为云镜像，失败再回退官方源。
6. **卸载回退**: 卸载当前版本时自动切换到剩余最高版本，避免 `java`/`mvn` 命令失效。
7. **`.jvmtoolrc`**: 从 CWD 向上查找，支持 `jdk=` 和 `maven=`，`jm use` 无参数时读取。
8. **多发行版**: `--distro` 持久标志挂在 `jdk` 子命令组上，通过 `NewJDKSourceForDistro` 工厂分发到对应 Source。

## 当前版本: v0.5.2

### v0.5.2 变更

**新功能:**
- `jm status` — 全局状态一览
- `jm jdk info <ver>` / `jm maven info <ver>` — 版本详情（大小、校验和、镜像）
- `.jvmtoolrc` 项目级版本切换 — `jm use` 无参数自动读取
- 卸载当前版本自动回退到剩余最新版本
- `--distro zulu` 支持 Azul Zulu JDK 发行版

**优化:**
- 统一 `humanSize` → `download.HumanSize`（消除重复）
- 统一 `compareVersions`/`parseParts` → `version.CompareVersions`/`VersionParts`
- 删除 `plusRe` 死代码
- HTTP Client 缓存复用连接
- 文件锁 stale 检测（5 分钟阈值）
- `matchInstalled` 精确化（三级优先级）
- `resolveExactVersion` 传 `limit=1`（减少 API 分页）

## 已知限制 / TODO

- `go.mod` 写的 `go 1.25.13`（patch 版本），Go 惯例应只写 minor `go 1.25`
- `install.sh` 的 `detect_rc()` 只检测 zsh/bash，fish 用户会写到 `.bashrc`
- Maven metadata 每次 List/Resolve 都重新请求，可加 5 分钟内存缓存
- `io.ReadAll` 无上限限制，可加 `io.LimitReader` 防御
- 测试覆盖率仍较低（download、extract、socks5、env shell 读写零覆盖）
- 桌面端仅支持 Windows（Wails 本身支持跨平台，但 CI 只构建 Windows 安装包）
- 仅支持 Temurin 和 Zulu，用户可能需要 GraalVM / Corretto

## 构建与发布

```bash
# 本地构建（当前平台）
go build -ldflags "-X jm/pkg/version.Version=v0.5.2" -o jm .

# 运行测试
go test ./...
go test -race ./...
go vet ./...

# 格式检查
gofmt -l $(git ls-files '*.go')

# 发布: 打 tag 推送后 GitHub Actions 自动构建 6 平台 CLI + Windows 桌面端安装包
git tag v0.5.2 && git push origin v0.5.2
```

## 目录约定

- 安装根目录: `$JVMTOOL_HOME` 或 `$HOME/.jvmtool`
- JDK 安装: `<root>/jdk/<version>/`
- Maven 安装: `<root>/maven/<version>/`
- 当前链接: `<root>/jdk/current`、`<root>/maven/current`
- 下载缓存: `<root>/cache/`
- 配置文件: `<root>/config.json`（代理、跳过版本）
- 锁文件: `<root>/.jm.lock`
