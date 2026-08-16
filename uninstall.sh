#!/usr/bin/env bash
#
# jm 卸载脚本 (Linux/macOS)
# 用法: bash uninstall.sh
#
# 清理内容:
#   1. 二进制文件 (默认 /usr/local/bin/jm)
#   2. shell 配置中的 jm 环境变量块 (~/.bashrc 或 ~/.zshrc)
#   3. 安装根目录 (默认 ~/.jvmtool, 含已安装的 JDK/Maven/缓存)
#      可用 KEEP_DATA=1 保留已安装的 JDK/Maven 数据目录

set -euo pipefail

PREFIX="${JVMTOOL_PREFIX:-/usr/local}"
BIN_DIR="${JVMTOOL_BIN_DIR:-$PREFIX/bin}"
TOOL_NAME="jm"
JVMTOOL_HOME="${JVMTOOL_HOME:-$HOME/.jvmtool}"
KEEP_DATA="${KEEP_DATA:-0}"

log()  { printf '\033[1;32m[OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

detect_rc() {
  if [ -n "${ZSH_VERSION:-}" ]; then
    echo "$HOME/.zshrc"
  else
    echo "$HOME/.bashrc"
  fi
}
RC_FILE="$(detect_rc)"

# ---------- 1. 删除二进制 ----------
if [ -f "$BIN_DIR/$TOOL_NAME" ]; then
  rm -f "$BIN_DIR/$TOOL_NAME"
  log "已删除二进制: $BIN_DIR/$TOOL_NAME"
else
  warn "未找到二进制: $BIN_DIR/$TOOL_NAME"
fi

# ---------- 2. 清理环境变量块 ----------
marker="# >>> jm >>>"
legacy_marker="# ---- jvmtool ----"
if [ -f "$RC_FILE" ]; then
  if grep -q "$marker" "$RC_FILE" 2>/dev/null; then
    # 移除 marker 包裹的整块
    content="$(grep -v "$marker" "$RC_FILE" || true)"
    printf '%s\n' "$content" > "$RC_FILE.tmp"
    mv "$RC_FILE.tmp" "$RC_FILE"
    log "已清理环境变量块: $RC_FILE"
  elif grep -q "$legacy_marker" "$RC_FILE" 2>/dev/null; then
    warn "检测到旧版环境变量块 ($legacy_marker)，请手动清理 $RC_FILE 中相关行"
  else
    warn "$RC_FILE 中没有 jm 环境变量块"
  fi
fi

# ---------- 3. 清理数据目录 ----------
if [ "$KEEP_DATA" = "1" ]; then
  warn "保留数据目录: $JVMTOOL_HOME"
else
  if [ -d "$JVMTOOL_HOME" ]; then
    rm -rf "$JVMTOOL_HOME"
    log "已删除数据目录: $JVMTOOL_HOME"
  else
    warn "数据目录不存在: $JVMTOOL_HOME"
  fi
fi

printf '\n'
log "卸载完成。"
printf '  提示: 如需保留已安装的 JDK/Maven，请用 KEEP_DATA=1 bash uninstall.sh\n'
