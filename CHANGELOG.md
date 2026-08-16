# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

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
