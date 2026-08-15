#!/usr/bin/env bash
#
# jm 一键安装脚本
# 用法: bash <(curl -fsSL https://raw.githubusercontent.com/dowdsa/jvmtool/main/install.sh)
#   或: bash install.sh [版本号]
#
# 功能:
#   1. 按系统/架构从 GitHub Releases 下载预编译二进制
#   2. 安装到可执行目录
#   3. 在 ~/.bashrc (或 ~/.zshrc) 中写入环境变量
#   4. 创建默认目录结构
#
# 版本号缺省时使用 latest release。

set -euo pipefail

# ---------- 可配置项 ----------
REPO_OWNER="${JVMTOOL_REPO_OWNER:-dowdsa}"
REPO_NAME="${JVMTOOL_REPO_NAME:-jvmtool}"
VERSION="${1:-latest}"
PREFIX="${JVMTOOL_PREFIX:-/usr/local}"
BIN_DIR="${JVMTOOL_BIN_DIR:-$PREFIX/bin}"
TOOL_NAME="jm"
BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download"
LATEST_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest/download"

# ---------- shell 配置检测 ----------
detect_rc() {
  if [ -n "${ZSH_VERSION:-}" ]; then
    echo "$HOME/.zshrc"
  else
    echo "$HOME/.bashrc"
  fi
}

RC_FILE="$(detect_rc)"
JVMTOOL_HOME="${JVMTOOL_HOME:-$HOME/.jvmtool}"

log()  { printf '\033[1;32m[OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[FAIL]\033[0m %s\n' "$*" >&2; exit 1; }

# ---------- 1. 解析平台信息 ----------
detect_platform() {
  local os arch
  case "$(uname -s)" in
    Linux)  os="linux" ;;
    Darwin) os="darwin" ;;
    *) die "不支持的 OS: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64)  arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) die "不支持的架构: $(uname -m)" ;;
  esac
  echo "${os}_${arch}"
}

# ---------- 2. 解析版本 ----------
# 使用 GitHub 的 /releases/latest/download/ 免 API 下载 latest，
# 避免 api.github.com 被代理/防火墙拦截导致失败。
resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    return 0
  fi
  # 去掉前缀 v (如 v0.1.0 -> 0.1.0)
  VERSION="${VERSION#v}"
}

# ---------- 3. 下载并安装二进制 ----------
download_binary() {
  platform="$(detect_platform)"
  local asset url
  asset="${TOOL_NAME}_${platform}"
  url="${BASE_URL}/v${VERSION}/${asset}"
  [ "$VERSION" = "latest" ] && url="${LATEST_URL}/${asset}"

  log "下载 ${TOOL_NAME} ${VERSION} (${platform})"
  log "来源: ${url}"
  mkdir -p "$BIN_DIR"

  local tmp
  tmp="$(mktemp -d)"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 -o "$tmp/$TOOL_NAME" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp/$TOOL_NAME" "$url"
  else
    die "需要 curl 或 wget，请先安装其一"
  fi

  chmod +x "$tmp/$TOOL_NAME"
  install -m 0755 "$tmp/$TOOL_NAME" "$BIN_DIR/$TOOL_NAME"
  rm -rf "$tmp"
  log "已安装到 $BIN_DIR/$TOOL_NAME"
}

# ---------- 3.5 校验下载文件 (可选) ----------
verify_checksum() {
  # checksum 文件同 release 一起发布，失败仅告警不阻断
  local checksum_url="${BASE_URL}/v${VERSION}/SHA256SUMS.txt"
  [ "$VERSION" = "latest" ] && checksum_url="${LATEST_URL}/SHA256SUMS.txt"
  if command -v sha256sum >/dev/null 2>&1; then
    if sums="$(curl -fsSL --max-time 20 "$checksum_url" 2>/dev/null)" && [ -n "$sums" ]; then
      if [ -n "$(printf '%s\n' "$sums" | grep "^[0-9a-f]\{64\}.*${TOOL_NAME}_${platform}" )" ]; then
        local expected
        expected="$(printf '%s\n' "$sums" | grep "${TOOL_NAME}_${platform}" | head -1 | awk '{print $1}')"
        local actual
        actual="$(sha256sum "$BIN_DIR/$TOOL_NAME" | awk '{print $1}')"
        if [ "$expected" != "$actual" ]; then
          die "校验和不匹配，请重新运行安装"
        fi
        log "SHA256 校验通过"
      fi
    fi
  fi
}

# ---------- 4. 写入环境变量 ----------
setup_env() {
  mkdir -p "$JVMTOOL_HOME"

  local marker="# >>> jm >>>"
  local block
  block=$(cat <<EOF
$marker
export JVMTOOL_HOME="\${JVMTOOL_HOME:-$JVMTOOL_HOME}"
if [ -d "\$JVMTOOL_HOME/jdk/current" ]; then
    export JAVA_HOME="\$JVMTOOL_HOME/jdk/current"
    export PATH="\$JAVA_HOME/bin:\$PATH"
fi
if [ -d "\$JVMTOOL_HOME/maven/current" ]; then
    export M2_HOME="\$JVMTOOL_HOME/maven/current"
    export MAVEN_HOME="\$M2_HOME"
    export PATH="\$M2_HOME/bin:\$PATH"
fi
$marker
EOF
  )

  # 去除旧块，避免重复
  local content=""
  if [ -f "$RC_FILE" ]; then
    content="$(grep -v "$marker" "$RC_FILE" || true)"
  fi
  {
    printf '%s\n' "$content"
    printf '\n%s\n' "$block"
  } > "$RC_FILE.tmp"
  mv "$RC_FILE.tmp" "$RC_FILE"

  log "环境变量已写入 $RC_FILE"
  log "JVMTOOL_HOME=$JVMTOOL_HOME"
}

# ---------- 5. 创建目录结构 ----------
init_dirs() {
  mkdir -p "$JVMTOOL_HOME/jdk" "$JVMTOOL_HOME/maven" "$JVMTOOL_HOME/cache"
  log "目录结构已创建: $JVMTOOL_HOME"
}

# ---------- 主流程 ----------
main() {
  log "开始安装 jm (版本: $VERSION)"

  resolve_version
  download_binary
  verify_checksum
  setup_env
  init_dirs

  if ! echo "$PATH" | grep -q "$BIN_DIR"; then
    warn "请将 $BIN_DIR 加入 PATH:  export PATH=$BIN_DIR:\$PATH"
  fi

  printf '\n'
  log "安装完成！"
  printf '\n'
  printf '  使用方法:\n'
  printf '    source %s              # 立即加载环境变量\n' "$RC_FILE"
  printf '    jm jdk search 21     # 搜索版本\n'
  printf '    jm jdk install 21    # 安装\n'
  printf '    jm jdk use 21        # 切换版本\n'
  printf '\n'
}

main "$@"
