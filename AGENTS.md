# jvmtool 项目记忆（AGENTS.md）

本文件是给 AI 助手的项目记忆，记录架构、约定与坑。修改代码前先读这里。

## 项目概述

Go 编写的多版本 JDK / Maven 管理工具，CLI 命令为 `jm`（Go module 名为 `jm`，非域名路径）。支持三种 JDK 发行版：**Temurin**（默认是 openjdk，注意 v0.6.0 起默认改为 OpenJDK）、**Zulu**、**OpenJDK**，以及 Apache Maven。附带 Windows 桌面端（Wails v2 GUI，独立 go.mod）。

- 语言: Go 1.25（go.mod 只写 minor 版本，遵守惯例）
- CLI 框架: spf13/cobra；UI 文案全部是中文
- 最新版本: **0.6.4**（2026-10-12），代码内 `pkg/version.Version` 默认为 `"dev"`，发布时用 ldflags 注入

## 常用命令

```bash
go build ./...                       # 构建
go test ./...                        # 测试
go test -race ./...                  # 竞态检测
go vet ./...                         # 静态检查
gofmt -l $(git ls-files '*.go')      # 格式检查（CI 会因此失败）
go build -ldflags "-X jm/pkg/version.Version=v0.6.4" -o jm .

# 发布：打 tag 推送后 GitHub Actions 自动构建（release.yml 的 quality 关卡：gofmt/test/-race/vet/前端构建，
# 然后矩阵构建 6 平台 CLI + Windows 桌面端 NSIS 安装包）；
# quality.yml 在 push main / PR 时跑同一套 quality 关卡
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
tools/genicon/          # 图标渲染器（4096px 超采样，衬线字体 go:embed 随工具嵌入），生成
                        # build/appicon.png、build/tray-icon.png、build/windows/icon.ico（多尺寸 PNG-in-ICO）；
                        # 图标设计=黑白字标 "jm."（纸底墨字），依赖 golang.org/x/image/font
install.sh install.ps1  # 一键安装脚本（下载预编译二进制 + 写环境变量）
```

## 核心架构

### version.Source 接口（多发行版的关键抽象）
`List(ctx, query, limit)` + `Resolve(ctx, version) -> *Artifact`。实现：
- **JDKSource (Temurin)**: Adoptium API `api.adoptium.net/v3`；Resolve 走 `/assets/feature_releases/<feature>/ga`（不用 `/assets/version`，有数据一致性缺口）；搜索分页阈值 30 页（JDK 8 等老版本排在后面）
- **ZuluSource**: Azul API `api.azul.com/metadata/v1/zulu/packages`；**2026-10 起 API 变更**：`java_version` 只认裸数字（`17`），旧的数组格式 `[17]` 被静默忽略返回全量；列表端点改为 slim 记录（**没有** `sha256_hash`/`release_status`/`size` 等字段），Resolve 必须再按 `package_uuid` 请求 `/packages/<uuid>` 详情端点拿校验和（校验和缺失=拒绝安装，绝不能用 slim 记录直接构建 Artifact）；Resolve 优先非 crac 包（crac 是 CRIU 特化构建）
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
- SOCKS5 配置了凭据时只提供 user/pass 认证方法（防降级），且服务端回 0x00（无认证）时握手直接拒绝

## 约定与坑

- **CHANGELOG.md 必须更新**（Keep a Changelog 中文格式），提交信息惯例: `feat:/fix:/security:/docs:`
- `.pi/memory.md` 是另一个工具（pi）留下的旧记忆，内容停在 v0.5.4 已过时，**以本文件为准**；`.pi/` 不受版本控制
- 桌面端是独立 Go module，改 `pkg/` 后桌面端要单独重新构建
- **Wails `runtime.Quit` 会重新执行 `OnBeforeClose`**（返回 true 即取消退出）。在"每次询问"模式下直接调 `Quit` 会再弹确认框并中止退出——必须先设置 `App.quitPending` 标志（见 `app.go RequestQuit`），`OnBeforeClose` 见标志直接放行。托盘"退出"菜单同样要走 `RequestQuit`
- **`getlantern/systray` 在 Windows 上只认 .ico**：内部把字节写临时文件后用 `LoadImage(IMAGE_ICON|LR_LOADFROMFILE)` 加载，传 PNG 会静默失败 → 托盘图标空白。`tray.go pngToICO` 把 PNG 包成 ICO 容器（PNG 压缩条目 Vista+ 可用），有 `tray_test.go` 防回归
- 托盘图标是 `//go:embed build/tray-icon.png`（64px，与主图标同源）；**换图标跑 `go run ./tools/genicon -out desktop/jm-desktop/build`**，改设计就改 genicon 里的谓词/颜色再重跑
- **桌面端是独立 module，主模块 `go test ./...` 与 CI quality 关卡都不跑它的单测**（Linux 上编译需 GTK/appindicator 头文件）——改 desktop 后必须手动 `cd desktop/jm-desktop && go test .`（本地需先有 frontend/dist 桩或跑过 wails build）
- 桌面端 `go.mod` 的 go 指令必须是 `go 1.25.0` 与主模块一致——写 `go 1.25` 时因 `replace jm => ../..` 版本不匹配，`go build` 直接报 "updates to go.mod needed"
- **重发 tag 的坑**：`action-gh-release` 对已存在的 release 只覆盖资产、不更新 body——重打 tag 前必须删旧 release。release.yml 已内置 "Remove stale release" 步骤（`gh release delete` || true）保证幂等
- 本地（非 wails）交叉编译桌面端需 `frontend/dist` 存在（`go:embed all:frontend/dist`），dist 是 gitignore 的构建产物，`wails build` 会自动生成；临时桩用后应删除
- 桌面端有单实例保护（`main.go SingleInstanceLock`，UniqueId `dowdsa.jvmtool.jm-desktop`）：第二个进程通知主实例唤起窗口后自动退出
- NSIS 安装/卸载前会 `taskkill /f /im jm-desktop.exe`（`project.nsi` 的 `killRunningInstance` 宏）——否则托盘驻留的旧实例锁住 exe，安装器报 "Error opening file for writing"
- NSIS 升级记忆安装路径：安装时写卸载键 `InstallLocation`（SHCTX + SetRegView 64），`.onInit` 读回预选 `$INSTDIR`，旧安装无此字段时回退读 `DisplayIcon` 取父目录。注意 Wails 写键用 `SetRegView 64`（64 位视图/WOW6432Node 区分），读时必须一致。本地验证脚本可 `apt install nsis` 后用 `makensis -DARG_WAILS_AMD64_BINARY=<stub> project.nsi` 编译（还需桩 `tmp/MicrosoftEdgeWebview2Setup.exe`）；`wails_tools.nsh` 是 wails 每次构建重新生成的，改它无效，只改 `project.nsi`
- 桌面端托盘用 `nicehash/systray`？**不对，实际用的是 `getlantern/systray` v1.2.1**（旧记忆有误）；必须先 `systray.Run()` 初始化再 SetIcon，直接 SetIcon 会 panic（v0.5.5 踩过）
- Wails 选目录 API 是 `OpenDirectoryDialog`（`OpenFileDialog` 是选文件的，v0.5.3 踩过）
- Maven metadata 已做进程内缓存（`NewMavenSource()` 共享 5 分钟 TTL + `If-Modified-Since` 条件请求，304/网络失败复用内存数据）；测试用 `&MavenSource{Client: fake}` 直接构造即绕过缓存
- fish 已支持：`install.sh`/`uninstall.sh` 与 `pkg/env` 按 `$SHELL` 写 `~/.config/fish/config.fish`（`set -gx` 语法，marker 块与 bash/zsh 相同，RemoveBlock 通用）；install/uninstall 现在按成对 marker 删整块（awk toggle），不再只删 marker 行
- 桌面端 CI 只构建 Windows 安装包；测试覆盖集中在 config/env/manager/project/version/update/download/cmd（download 断点续传与 socks5 握手已有 httptest/假代理覆盖），extract 覆盖仍薄

## 运行时目录约定

```
$JVMTOOL_HOME 或 ~/.jvmtool/
  jdk/<version>/  maven/<version>/     # 安装目录
  jdk/current   maven/current          # 指向当前版本的符号链接
  cache/                               # 下载缓存
  config.json                          # 设置: proxy / skip_version / close_action
  .jm.lock                             # 文件锁
```

## Agent skills

### Issue tracker

Issues 跟踪在 GitHub Issues (dowdsa/jvmtool)，通过 `gh` CLI 操作。见 `docs/agents/issue-tracker.md`。

### Triage labels

使用默认五件套标签（needs-triage / needs-info / ready-for-agent / ready-for-human / wontfix），与角色同名。见 `docs/agents/triage-labels.md`。

### Domain docs

单上下文布局：根目录 `GLOSSARY.md` + `docs/adr/`。见 `docs/agents/domain.md`。
