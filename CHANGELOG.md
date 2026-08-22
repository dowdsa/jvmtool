# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [0.5.6] - 2026-08-22

### 新增

- 新增 Oracle OpenJDK 发行版支持（`--distro openjdk`），包含 JDK 17/20/21/22/23
- 默认 JDK 发行版改为 OpenJDK（此前为 Temurin）
- 桌面端支持多 JDK 发行版选择：搜索页面新增发行版下拉框
- 下载进度改为独立页面显示，侧边栏新增"下载管理"导航项

### 修复

- 导入目录功能：通过 Go 侧 `SelectDirectory` 绑定方法调用原生目录选择器
- 关闭弹窗布局：复选框左侧、按钮右侧，不再强制勾选

## [0.5.5] - 2026-08-21

### 修复

- **修复桌面端启动崩溃**：`getlantern/systray` 库要求先调用 `systray.Run()` 初始化内部消息循环，v0.5.4 直接调用 `SetIcon` 导致 Windows 上 panic 崩溃，现已修复为正确的初始化流程
- 添加 panic recovery 防止托盘崩溃影响主应用
- 日志重定向到 `%TEMP%/jm-desktop.log` 便于调试

## [0.5.4] - 2026-08-21

### 新增

- 桌面端支持最小化到系统托盘：关闭窗口时隐藏到托盘而非退出，右键菜单可显示窗口或退出
- 桌面端关闭时弹出确认对话框：可选"后台运行"或"退出应用"，含"不再提示"复选框
- 设置面板新增"关闭窗口行为"选项：每次询问 / 最小化到后台 / 直接退出
- 新增 `jm status` 命令：一条命令查看 JDK、Maven 当前版本、安装数量和缓存占用
- 新增 `jm jdk info <ver>` / `jm maven info <ver>` 命令：查看版本大小、校验和、下载链接、镜像源
- 新增 `.jvmtoolrc` 项目级版本切换：在项目目录放置配置文件，`jm use` 无参数即可自动读取并切换
- `jm jdk use` / `jm maven use` 支持无参数模式，自动从 `.jvmtoolrc` 读取版本
- 卸载当前版本时自动回退到剩余的最新版本，避免 `java`/`mvn` 命令失效
- 新增 `--distro` 参数支持 Zulu (Azul) JDK 发行版（`jm jdk search/install/info --distro zulu`）

### 安全性

- **Zip/Tar Bomb 防护**：解压时累计字节数，超过 2GB 立即中断，防止恶意压缩包耗尽磁盘
- **空校验和绕过修复**：`VerifySHA256`/`VerifySHA512` 空值返回 false（此前返回 true），拒绝无校验和安装
- **HTTP 响应体限制**：所有 `io.ReadAll` 加 `io.LimitReader` 防护 OOM（API 响应限制 10MB）
- **文件锁 PID 验证**：检查锁文件持有者进程是否存活，及时清理残留锁
- **Shell 注入防护**：`env.go Block()` 用单引号包裹路径，`install.sh` 拒绝含特殊字符的路径
- **Windows TOCTOU 修复**：更新脚本用 `crypto/rand` 生成不可预测文件名
- **SOCKS5 降级攻击修复**：配置凭据时仅提供 user/pass 认证方法，不接受 no-auth 降级

### 优化

- 统一重复代码：`humanSize` 导出为 `download.HumanSize`，版本比较统一为 `version.CompareVersions`
- 删除未使用的 `plusRe` 死代码
- HTTP Client 缓存复用连接，代理变更时自动重建
- `matchInstalled` 精确化匹配，防止 `jm jdk use 1` 静默选中 11
- `resolveExactVersion` 传 `limit=1`，减少不必要的 API 分页
- Gitee 同步工作流超时从 30 分钟增加到 120 分钟
- 移除 `.pi` 目录版本控制跟踪

### 修复

- 桌面端导入目录报错 `OpenFileDialog is not a function`：改用 `OpenDirectoryDialog`
- 设置面板文字"开机自动启动桌面端"改为"开机自启动"

## [0.5.3] - 2026-08-20

### 修复

- 桌面端导入目录报错 `OpenFileDialog is not a function`：Wails v2 中选目录应用 `OpenDirectoryDialog`，`OpenFileDialog` 是选文件的
- 对话框取消与导入失败分开处理，取消选择时不再误报"导入失败"

## [0.5.2] - 2026-08-18

### 新增

- `jm status` 命令：一条命令查看 JDK、Maven 当前版本、安装数量和缓存占用
- `jm jdk info <ver>` / `jm maven info <ver>` 命令：查看版本大小、校验和、下载链接、镜像源
- `.jvmtoolrc` 项目级版本切换：在项目目录放置 `jdk=17` / `maven=3.9.11`，`jm use` 无参数自动读取
- `jm jdk use` / `jm maven use` 支持无参数模式，从 `.jvmtoolrc` 读取版本
- 卸载当前版本时自动回退到剩余的最新版本
- `--distro` 参数支持 Zulu (Azul) JDK 发行版

### 优化

- 统一重复代码：`humanSize` 导出为 `download.HumanSize`，版本比较统一为 `version.CompareVersions`
- 删除未使用的 `plusRe` 死代码
- HTTP Client 缓存复用连接，代理变更时自动重建
- `matchInstalled` 精确化匹配（三级优先级），防止歧义前缀静默命中
- `resolveExactVersion` 传 `limit=1`，减少不必要的 API 分页请求
- 文件锁增加 stale 检测（5 分钟阈值），自动清理残留锁文件

## [0.5.1] - 2026-08-18

### 修复

- 修复桌面端点击“取消”后更新提示反复弹出的问题
- 取消更新后仅在客户端重新启动时再次提示
- 跳过某个版本后，只有检测到更高版本时才会重新提示更新

## [0.5.0] - 2026-08-18

### 新增

- 新增 `jm cache list`、`jm cache size` 和 `jm cache clean` 缓存管理命令，支持按天数清理旧缓存
- `jm doctor --verbose` 增加配置文件路径和缓存占用诊断
- 桌面端下载队列支持应用重启后恢复排队任务
- 桌面端增加已有 JDK/Maven 导入入口
- Windows 桌面端增加开机自动启动设置

### 安全性与修复

- CLI 和桌面端更新包增加 SHA256SUMS 校验，避免安装损坏或被篡改的更新文件
- JDK/Maven 安装成功后自动清理对应下载缓存
- 卸载 JDK/Maven 时只清理对应版本缓存，不影响其他版本的断点续传文件
- Gitee 发布同步改为等待 GitHub Release 成功后只同步当前 Tag，并增加上传超时与重试
- 修复桌面端取消更新提示后因页面重新渲染而重复弹窗的问题，并优化跳过版本的提示策略

## [0.4.1] - 2026-08-17

### 修复

- `jm doctor` 环境诊断命令和自定义 `jm completion` 补全命令此前已定义但未注册，实际不可用，现已正确注册

## [0.4.0] - 2026-08-17

### 新增

- 桌面端新增下载任务队列，支持多个 JDK/Maven 任务排队并自动串行执行
- 下载任务支持暂停、继续、取消、移除和失败重试
- 下载进度面板同时展示排队中、下载中、已暂停和失败状态
- 桌面端支持直接下载并启动 Windows 安装程序完成客户端升级，不再仅跳转 GitHub 页面
- CLI 的 `jm update` 支持下载对应平台的 CLI 并自动替换当前可执行文件

## [0.3.3] - 2026-08-17

### 修复

- Windows 桌面端自动读取当前用户的 Internet Settings 静态代理，并动态读取用户环境变量，避免必须手动填写代理
- 桌面端下载进度移至右下角固定任务面板，不再与远程版本搜索结果混在一起；页面滚动时进度、速度和操作按钮仍保持可见
- 新增 `jm doctor` 环境诊断命令、已有 JDK/Maven 导入命令和 Bash/Zsh/Fish/PowerShell 补全
- 代理支持 `NO_PROXY` 绕过规则，并在 Windows 下提示检测到的 PAC 地址

## [0.3.2] - 2026-08-17

### 安全性

- 安装程序必须完成 SHA256 校验后才会安装二进制文件，校验失败或缺失时立即终止
- Maven 安装必须获取并校验 SHA512，避免在缺少校验值时无校验安装

### 修复

- Maven 部分版本号现在解析为最新稳定版本，未知版本不再错误回退到 latest
- 安装解压后增加最终目录校验，异常压缩包结构不会被误报为安装成功
- 安装、切换、卸载和清理缓存增加互斥锁，降低并发操作导致的数据损坏风险
- 版本切换使用临时链接原子替换，减少切换过程中的短暂不可用状态
- Unix 卸载当前 JDK 或 Maven 时只更新对应环境配置，不再误删另一工具的配置
- Windows 切换版本时清理旧版本 PATH，避免 PATH 持续累积历史版本目录
- 预发布版本识别增加 `SNAPSHOT`

### 工程质量

- 增加 Maven 版本解析回归测试
- CI 增加格式检查、`go test`、竞态检测、`go vet` 和桌面端前端构建
- 整理 Go 依赖声明

## [0.3.1] - 2026-08-16

### 新增

- 真正支持 socks5 代理（此前设置界面与文档宣称支持，但标准库 http 客户端并不支持该协议）
- 代理地址保存前校验：仅接受 http/https/socks5 且必须包含主机名，非法配置直接报错
- 下载增加超时保护：拨号 30s / 响应头 30s / TLS 握手 15s / 空闲连接 90s（整体下载时长不受限）
- Windows 卸载当前版本时同步清理注册表 JAVA_HOME / M2_HOME / MAVEN_HOME 及用户 PATH 中的 bin 目录
- `jm <tool> uninstall` 支持部分版本号（如 `jm jdk uninstall 21`，与 `use` 行为一致），并提示实际卸载的精确版本
- config.json 写入权限收紧为 0600（该文件可能包含代理凭据）

### 修复

- Windows 切换版本导致用户 PATH 被重写：%VAR% 引用不再展开（恢复 REG_EXPAND_SZ），且系统级条目被误复制进用户 PATH
- Windows 卸载当前版本后环境变量残留，java/mvn 仍指向已删除目录
- 桌面端（长驻进程）切换版本后 `Current()` 读到过期的进程环境变量（改为读取注册表）
- Maven 搜索结果顺序与 “最新在前” 契约不符（补充版本号降序排序）
- 下载过程中断（连接重置 / 流错误）不重试（改为断点续传重试，最多 3 次）
- 下载校验失败后缓存文件残留，导致重装同一版本反复失败（校验失败自动删除缓存）
- 安装解压后重命名失败残留半成品目录（自动清理）
- tar 解压符号链接目标未校验，存在越界/绝对路径风险（现已拒绝，保留目录内相对链接）
- 桌面端下载进度事件过于频繁（约 100ms 节流）
- 桌面端用字符串匹配判断下载结果（改为结构化 status：ok / paused / cancelled / error）

## [0.3.0] - 2026-08-16

### 新增

- 桌面端 (Wails GUI)：已安装列表、切换版本、搜索安装、卸载版本
- 桌面端下载进度条（内嵌显示，非弹窗），支持暂停 / 继续（断点续传）/ 取消
- 桌面端显示下载速率
- 国内镜像加速下载：JDK 走清华 TUNA 镜像，Maven 走华为云镜像
- 桌面端设置界面，可视化配置代理（支持 http/https/socks5）
- 检查更新功能：CLI `jm update` + 桌面端启动检查弹窗（安装/取消/跳过此版本）
- CLI `jm --version` / `jm -v` 查看版本
- 卸载脚本：`uninstall.sh`（Linux/macOS）、`uninstall.ps1`（Windows）

### 修复

- JDK 在 Windows 上安装失败（版本源硬编码 os=linux，改为按平台动态选择）
- JDK 解压不支持 Windows 的 .zip 格式
- Windows 环境变量未配置（切换版本后 java/mvn 命令不可用）
- Windows PowerShell 安装脚本闪退和中文乱码
- 下载走慢速 GitHub 主源而非镜像（改为镜像优先）
- CLI 下载无进度条（重构后丢失）
- Windows 终端进度条滚动刷新（终端检测改用 x/term）

## [0.2.0] - 2026-08-15

### 新增

- 卸载版本时自动清理对应的环境变量
- `jm env show` / `jm env clean` 命令（查看/清理 shell 环境变量配置）
- `jm <tool> use` 切换后自动写入 shell 环境变量

### 修复

- 安装脚本 `latest` 下载路径错误（`/releases/download/latest/` → `/releases/latest/download/`）
- 安装脚本 latest 免 GitHub API（避免 api.github.com 被拦截）

## [0.1.0] - 2026-08-14

### 新增

- 首个版本：多版本 JDK (Temurin) 与 Maven 管理
- `jm jdk|maven` 子命令：search / install / use / list / uninstall / current
- `jm clean` 清理下载缓存
- 下载支持断点续传、SHA256/SHA512 校验、进度条
- 一键安装脚本 `install.sh`（Linux/macOS）、`install.ps1`（Windows）
- GitHub Actions 全平台自动构建发布
