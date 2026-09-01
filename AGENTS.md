# jvmtool 项目记忆（AGENTS.md）

本文件是给 AI 助手的项目记忆，记录架构、约定与坑。修改代码前先读这里。

## 项目概述

Go 编写的多版本 JDK / Maven 管理工具，CLI 命令为 `jm`（Go module 名为 `jm`，非域名路径）。支持三种 JDK 发行版：**Temurin**（默认是 openjdk，注意 v0.6.0 起默认改为 OpenJDK）、**Zulu**、**OpenJDK**，以及 Apache Maven。附带 Windows 桌面端（Wails v2 GUI，独立 go.mod）。

- 语言: Go 1.25（go.mod 只写 minor 版本，遵守惯例）
- CLI 框架: spf13/cobra；UI 文案全部是中文
- 最新版本: **0.6.2**（2026-09-01），代码内 `pkg/version.Version` 默认为 `"dev"`，发布时用 ldflags 注入

## 常用命令

```bash
go build ./...                       # 构建
go test ./...                        # 测试
go test -race ./...                  # 竞态检测
go vet ./...                         # 静态检查
gofmt -l $(git ls-files '*.go')      # 格式检查（CI 会因此失败）
go build -ldflags "-X jm/pkg/version.Version=v0.6.1" -o jm .

# 发布：打 tag 推送后 GitHub Actions 自动构建（quality 关卡：gofmt/test/-race/vet/前端构建，
# 然后矩阵构建 6 平台 CLI + Windows 桌面端 NSIS 安装包）
git tag v0.6.x && git push origin v0.6.x
```

## 目录结构

```
main.go                 # 入口，cmd.Execute()
cmd/                    # cobra 命令层
  root.go               # 根命令、子命令注册、--distro 持久标志（挂在 jdk 组）、jm update 自更新
  tools.go              # jdk/maven 组的 search/install/list/use/uninstall/current/import
  status.go info.go use.go cache.go env.go doctor.go completion.go
pkg/
  config/               # Config（Root=JVMTOOL_HOME 或 ~/.jvmtool）、config.json 设置、
                        # 代理解析（设置 > JVMTOOL_PROXY > 标准环境变量 > Windows 系统代理）、
                        # HTTPClient 单例缓存（按代理 key 失效重建）、socks5.go 自定义拨号、
                        # proxy_bypass.go NO_PROXY
  version/              # Source 接口 + 各发行版实现（见下）、Artifact 结构、
                        # CompareVersions/VersionParts（全局唯一版本比较实现，勿另写）
  download/             # 下载器（断点续传、重试、进度条）、SHA256/512 校验、HumanSize
  manager/              # 核心编排：Install/Use/Uninstall/Import、文件锁、extract.go（解压+安全防护）、
                        # import.go（导入已有安装，不移动原目录）
  env/                  # Windows: 注册表用户环境变量 + PATH；Unix: shell rc 块（Block/ApplyBlock/RemoveBlock）
  project/              # .jvmtoolrc 项目级版本固定（从 CWD 向上查找，jdk=/maven=）
  update/               # GitHub Releases 检查 + CLI 自更新（DownloadCLI/ApplyCLI）
desktop/jm-desktop/     # Wails v2 桌面端，独立 go.mod，前端 frontend/（Vite + 原生 JS）
  app.go tray.go autostart_windows.go
icons/                  # 未跟踪：图标候选生成（gen_icons.go + 4 张 PNG，供托盘/应用图标选用）
install.sh install.ps1  # 一键安装脚本（下载预编译二进制 + 写环境变量）
```

## 核心架构

### version.Source 接口（多发行版的关键抽象）
`List(ctx, query, limit)` + `Resolve(ctx, version) -> *Artifact`。实现：
- **JDKSource (Temurin)**: Adoptium API `api.adoptium.net/v3`；Resolve 走 `/assets/feature_releases/<feature>/ga`（不用 `/assets/version`，有数据一致性缺口）；搜索分页阈值 30 页（JDK 8 等老版本排在后面）
- **ZuluSource**: Azul API `api.azul.com/metadata/v1/zulu/packages`；`java_version` 参数必须传数组格式 `[17]`；Resolve 优先选 `Latest=true` 的包
- **OpenJDKSource**: 同样走 Adoptium API 拿真实下载链接和 SHA256（v0.6.0 之前 hash 是编造的，已重写）
- **MavenSource**: `repo.maven.apache.org` 的 maven-metadata.xml
- 工厂: `NewJDKSourceForDistro(distro)`，由 cmd 层 `--distro` 标志驱动（jdk 组的持久标志，`getDistro(group)` 读取）

### 安装流程 (manager.InstallWithProgress)
Resolve → 文件锁 → 下载到 cache（**镜像优先**：TUNA Adoptium 镜像 / 华为云 Maven 镜像，失败回退官方源；断点续传保留部分文件）→ SHA256/512 校验（**无校验和直接拒绝**，校验失败删缓存）→ 解压（wrapper 目录 `jdk-<ver>`/`apache-maven-<ver>` 重命名为纯版本号）→ 成功后才删缓存包。

### 关键设计决策
1. **文件锁**: `<root>/.jm.lock`，`O_CREATE|O_EXCL` 原子创建 + 写 PID；stale 检测 = 文件年龄 > 5 分钟 **或** PID 进程已死
2. **matchInstalled 三级匹配**: 精确 > 前缀+点（`17`→`17.0.13+11`）> 唯一前缀（防止 `use 1` 静默选中 11）
3. **Use 切换**: 原子改 `current` 符号链接（Windows 符号链接失败则静默降级，靠环境变量兜底）；Windows 写注册表 JAVA_HOME/M2_HOME/PATH，Unix 写 shell rc 块
4. **Uninstall 回退**: 卸载当前版本自动切到剩余最高版本；Use 失败时清理新旧两版本环境变量避免残留
5. **Windows Current() 读注册表**而非进程环境变量（GUI 长驻进程的环境变量快照会过期）
6. **HTTPClient 缓存**: Transport 复用连接，代理变更时按 key 重建；`HTTPClientWithTimeout` 共享 Transport 但独立 client，不污染缓存

### 安全加固（勿回退）
- 解压：累计字节上限 2GB（zip bomb）、符号链接路径校验
- 校验：空校验和返回 false（拒绝安装）
- 所有 `io.ReadAll` 套 `io.LimitReader`（API 响应 10MB 上限）
- 缓存文件与 config.json（可能含代理凭据）权限 0600
- shell rc 写入用单引号包裹路径（防注入）；Windows 更新用 crypto/rand 文件名（防 TOCTOU）
- SOCKS5 配置了凭据时只提供 user/pass 认证方法（防降级）

## 约定与坑

- **CHANGELOG.md 必须更新**（Keep a Changelog 中文格式），提交信息惯例: `feat:/fix:/security:/docs:`
- `.pi/memory.md` 是另一个工具（pi）留下的旧记忆，内容停在 v0.5.4 已过时，**以本文件为准**；`.pi/` 不受版本控制
- 桌面端是独立 Go module，改 `pkg/` 后桌面端要单独重新构建
- **Wails `runtime.Quit` 会重新执行 `OnBeforeClose`**（返回 true 即取消退出）。在"每次询问"模式下直接调 `Quit` 会再弹确认框并中止退出——必须先设置 `App.quitPending` 标志（见 `app.go RequestQuit`），`OnBeforeClose` 见标志直接放行。托盘"退出"菜单同样要走 `RequestQuit`
- **`getlantern/systray` 在 Windows 上只认 .ico**：内部把字节写临时文件后用 `LoadImage(IMAGE_ICON|LR_LOADFROMFILE)` 加载，传 PNG 会静默失败 → 托盘图标空白。`tray.go pngToICO` 把 PNG 包成 ICO 容器（PNG 压缩条目 Vista+ 可用），有 `tray_test.go` 防回归
- 桌面端 `go.mod` 的 go 指令必须是 `go 1.25.0` 与主模块一致——写 `go 1.25` 时因 `replace jm => ../..` 版本不匹配，`go build` 直接报 "updates to go.mod needed"
- 本地（非 wails）交叉编译桌面端需 `frontend/dist` 存在（`go:embed all:frontend/dist`），dist 是 gitignore 的构建产物，`wails build` 会自动生成；临时桩用后应删除
- 桌面端托盘用 `nicehash/systray`？**不对，实际用的是 `getlantern/systray` v1.2.1**（旧记忆有误）；必须先 `systray.Run()` 初始化再 SetIcon，直接 SetIcon 会 panic（v0.5.5 踩过）
- Wails 选目录 API 是 `OpenDirectoryDialog`（`OpenFileDialog` 是选文件的，v0.5.3 踩过）
- Maven metadata 每次 List/Resolve 都重新请求（可加 5 分钟内存缓存，已知 TODO）
- fish 用户会被 install.sh 写入 .bashrc（detect_rc 只认 zsh/bash，已知限制）
- 桌面端 CI 只构建 Windows 安装包；测试覆盖集中在 config/env/manager/project/version/update，download/extract/socks5 覆盖薄

## 运行时目录约定

```
$JVMTOOL_HOME 或 ~/.jvmtool/
  jdk/<version>/  maven/<version>/     # 安装目录
  jdk/current   maven/current          # 指向当前版本的符号链接
  cache/                               # 下载缓存
  config.json                          # 设置: proxy / skip_version / close_action
  .jm.lock                             # 文件锁
```
