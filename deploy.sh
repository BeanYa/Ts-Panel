#!/usr/bin/env bash
set -e

# ================================================================
#  ts-panel 管理脚本
#  用法:
#    ./deploy.sh start    —— 交互式部署并启动所有服务
#    ./deploy.sh stop     —— 停止并移除所有管理的容器
# ================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ----- 帮助 / 默认提示 ------------------------------------------
usage() {
  echo ""
  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║      ts-panel 管理脚本                                  ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║  用法:                                                  ║"
  echo "║    ./deploy.sh start [选项]     启动服务                ║"
  echo "║    ./deploy.sh stop             停止服务                ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║  选项 (均可选，未指定则交互式输入):                      ║"
  echo "║    --ip       <IP>       服务器公网 IP                  ║"
  echo "║    --token    <TOKEN>    Admin Token                    ║"
  echo "║    --port     <PORT>     面板端口 (默认 80)             ║"
  echo "║    --mode     <1|2>      1=IP直连  2=域名模式           ║"
  echo "║    --domain   <DOMAIN>   域名 (mode=2 时必填)           ║"
  echo "║    --goproxy  <URL>      Go 代理地址                    ║"
  echo "╠══════════════════════════════════════════════════════════╣"
  echo "║  示例:                                                  ║"
  echo "║    ./deploy.sh start --ip 1.2.3.4 --token abc --port 80 ║"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""
}

# ----- stop 子命令 ----------------------------------------------
cmd_stop() {
  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║    ts-panel 一键停止脚本             ║"
  echo "╚══════════════════════════════════════╝"
  echo ""

  cd "$SCRIPT_DIR"

  if [[ ! -f "docker-compose.yml" ]]; then
    echo "❌ 未找到 docker-compose.yml，请确认脚本位于项目根目录"
    exit 1
  fi

  echo "🛑 正在停止并移除所有容器..."
  docker compose down

  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║  ✅ 所有容器已停止                   ║"
  echo "║  数据卷与镜像已保留，可随时重启      ║"
  echo "╚══════════════════════════════════════╝"
  echo ""
  echo "提示: 如需彻底清除数据卷，请手动执行:"
  echo "  docker compose down -v"
  echo ""
}

# ----- start 子命令 --------------------------------------------
cmd_start() {
  # --- 解析命令行参数 ---
  local ARG_IP="" ARG_TOKEN="" ARG_MODE="" ARG_DOMAIN="" ARG_GOPROXY="" ARG_PORT=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --ip)         ARG_IP="$2";       shift 2 ;;
      --token)      ARG_TOKEN="$2";    shift 2 ;;
      --mode)       ARG_MODE="$2";     shift 2 ;;
      --domain)     ARG_DOMAIN="$2";   shift 2 ;;
      --goproxy)    ARG_GOPROXY="$2";  shift 2 ;;
      --port)       ARG_PORT="$2";     shift 2 ;;
      *) echo "❌ 未知参数: $1"; exit 1 ;;
    esac
  done

  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║    ts-panel 一键启动脚本             ║"
  echo "╚══════════════════════════════════════╝"
  echo ""

  cd "$SCRIPT_DIR"

  # === 公网 IP ===
  if [[ -n "$ARG_IP" ]]; then
    PUBLIC_IP="$ARG_IP"
    echo "📌 公网 IP: $PUBLIC_IP"
  else
    read -rp "请输入服务器公网 IP: " PUBLIC_IP
  fi
  if [[ -z "$PUBLIC_IP" ]]; then
    echo "❌ 公网 IP 不能为空"; exit 1
  fi

  # === Admin Token ===
  if [[ -n "$ARG_TOKEN" ]]; then
    ADMIN_TOKEN="$ARG_TOKEN"
    echo "📌 Admin Token: [已设置]"
  else
    read -rsp "请输入 Admin Token（不会显示）: " ADMIN_TOKEN
    echo ""
  fi
  if [[ -z "$ADMIN_TOKEN" ]]; then
    echo "❌ Admin Token 不能为空"; exit 1
  fi

  # === 部署模式 ===
  if [[ -n "$ARG_MODE" ]]; then
    MODE="$ARG_MODE"
    echo "📌 部署模式: $MODE"
  else
    echo ""
    echo "选择部署模式:"
    echo "  1) IP 直连模式（HTTP，适合测试）"
    echo "  2) 域名模式（Caddy 自动 HTTPS）"
    read -rp "请输入 1 或 2 [默认 1]: " MODE
    MODE=${MODE:-1}
  fi

  DOMAIN=""
  if [[ "$MODE" == "2" ]]; then
    if [[ -n "$ARG_DOMAIN" ]]; then
      DOMAIN="$ARG_DOMAIN"
      echo "📌 域名: $DOMAIN"
    else
      read -rp "请输入域名（如 ts.example.com）: " DOMAIN
    fi
    if [[ -z "$DOMAIN" ]]; then
      echo "❌ 域名不能为空"; exit 1
    fi
  fi

  # === Go 代理配置 ===
  if [[ -n "$ARG_GOPROXY" ]]; then
    GOPROXY="$ARG_GOPROXY"
    echo "📌 GOPROXY: $GOPROXY"
  else
    echo ""
    echo "是否需要配置 Go 代理 (GOPROXY)?"
    read -rp "请输入代理地址 [默认: https://goproxy.cn,direct]: " GOPROXY
    GOPROXY=${GOPROXY:-"https://goproxy.cn,direct"}
  fi

  # === 端口配置 ===
  if [[ -n "$ARG_PORT" ]]; then
    PANEL_HTTP_PORT="$ARG_PORT"
    echo "📌 面板端口: $PANEL_HTTP_PORT"
  else
    echo ""
    echo "配置面板访问端口 (用于 HTTP/IP 直连):"
    read -rp "请输入端口号 [默认: 80]: " PANEL_HTTP_PORT
    PANEL_HTTP_PORT=${PANEL_HTTP_PORT:-80}
  fi

  # === 生成 .env ===
  cat > .env <<EOF
PUBLIC_IP=${PUBLIC_IP}
ADMIN_TOKEN=${ADMIN_TOKEN}
GOPROXY=${GOPROXY}
PANEL_HTTP_PORT=${PANEL_HTTP_PORT}
HTTP_PORT=8080
PORT_MIN=20000
PORT_MAX=20999
QUERY_PORT_MIN=21000
QUERY_PORT_MAX=21999
DEFAULT_CPU=0.5
DEFAULT_MEMORY=512m
DEFAULT_PIDS=200
CREATE_RETRY=2
SECRETS_RETRY=10
LOG_TAIL=300
DATA_ROOT=/data
DB_PATH=/data/db/app.db
EOF
  echo "✅ .env 已生成"

  # === 域名模式：生成 Caddyfile ===
  if [[ "$MODE" == "2" ]]; then
    echo "⚠️  注意: 域名模式 (Caddy) 强制需要占用 80 和 443 端口以申请证书。"
    echo "    如果这些端口已被占用 (如 Nginx), Caddy 将无法正常工作。"
    read -rp "确认继续? [y/N]: " CONFIRM_CADDY
    if [[ ! "$CONFIRM_CADDY" =~ ^[Yy]$ ]]; then
      echo "❌ 已取消部署"; exit 1
    fi

    cat > Caddyfile <<EOF
${DOMAIN} {
    reverse_proxy ts-panel-ui:80
}
EOF
    # 在 compose 里追加 caddy 服务
    cat >> docker-compose.yml <<EOF

  caddy:
    image: caddy:2-alpine
    container_name: ts-panel-caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      - panel
    networks:
      - ts-net

volumes:
  caddy_data:
  caddy_config:
EOF
    echo "✅ Caddyfile 已生成（域名: $DOMAIN）"
  fi

  # === 创建数据目录 ===
  sudo mkdir -p /data/db /data/instances
  echo "✅ 数据目录已准备"

  # === 配置 Docker 镜像加速 ===
  if ! grep -q "registry-mirrors" /etc/docker/daemon.json 2>/dev/null; then
    echo "🔧 配置 Docker 镜像加速器..."
    sudo mkdir -p /etc/docker
    sudo tee /etc/docker/daemon.json > /dev/null <<'DAEMON'
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://mirror.ccs.tencentyun.com"
  ]
}
DAEMON
    sudo systemctl daemon-reload
    sudo systemctl restart docker
    echo "✅ Docker 镜像加速已配置"
  else
    echo "✅ Docker 镜像加速已存在，跳过配置"
  fi

  # === 启动 ===
  echo ""
  echo "🚀 正在构建并启动服务..."
  docker compose build --no-cache
  docker compose up -d

  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║  🎉 部署完成！                       ║"
  echo "╠══════════════════════════════════════╣"
  if [[ "$MODE" == "2" ]]; then
    echo "║  面板地址: https://${DOMAIN}"
  else
    echo "║  面板地址: http://${PUBLIC_IP}"
  fi
  echo "║  Admin Token: [已安全存储到 .env]"
  echo "║  数据目录: /data"
  echo "╚══════════════════════════════════════╝"
}

# ----- 自动检测 -------------------------------------------------
# 检测逻辑：
#   1. docker-compose.yml 存在 → 说明曾经部署过
#   2. 有正在运行的 ts-panel-* 容器 → 执行 stop
#   3. 否则（未部署或已停止）→ 执行 deploy
cmd_auto() {
  cd "$SCRIPT_DIR"

  if [[ ! -f "docker-compose.yml" ]]; then
    echo "ℹ️  未检测到已有部署，进入启动流程..."
    cmd_start
    return
  fi

  # 检查是否有 running 状态的容器
  RUNNING=$(docker compose ps --status running --quiet 2>/dev/null | wc -l | tr -d ' ')

  if [[ "$RUNNING" -gt 0 ]]; then
    echo "🔍 检测到 ${RUNNING} 个容器正在运行，自动执行 stop..."
    cmd_stop
  else
    echo "🔍 未检测到运行中的容器，自动执行 start..."
    cmd_start
  fi
}

# ----- 命令分发 -------------------------------------------------
case "${1:-}" in
  start)
    shift
    cmd_start "$@"
    ;;
  stop)
    cmd_stop
    ;;
  "")  # 无参数：自动检测
    cmd_auto
    ;;
  *)
    usage
    exit 1
    ;;
esac
