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
  echo "╔══════════════════════════════════════╗"
  echo "║      ts-panel 管理脚本               ║"
  echo "╠══════════════════════════════════════╣"
  echo "║  用法:                               ║"
  echo "║    ./deploy.sh start                ║"
  echo "║    ./deploy.sh stop                 ║"
  echo "╚══════════════════════════════════════╝"
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
  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║    ts-panel 一键启动脚本             ║"
  echo "╚══════════════════════════════════════╝"
  echo ""

  cd "$SCRIPT_DIR"

  # === 输入公网 IP ===
  read -rp "请输入服务器公网 IP: " PUBLIC_IP
  if [[ -z "$PUBLIC_IP" ]]; then
    echo "❌ 公网 IP 不能为空"; exit 1
  fi

  # === 输入 Admin Token ===
  read -rsp "请输入 Admin Token（不会显示）: " ADMIN_TOKEN
  echo ""
  if [[ -z "$ADMIN_TOKEN" ]]; then
    echo "❌ Admin Token 不能为空"; exit 1
  fi

  # === 部署模式 ===
  echo ""
  echo "选择部署模式:"
  echo "  1) IP 直连模式（HTTP，适合测试）"
  echo "  2) 域名模式（Caddy 自动 HTTPS）"
  read -rp "请输入 1 或 2 [默认 1]: " MODE
  MODE=${MODE:-1}

  DOMAIN=""
  if [[ "$MODE" == "2" ]]; then
    read -rp "请输入域名（如 ts.example.com）: " DOMAIN
    if [[ -z "$DOMAIN" ]]; then
      echo "❌ 域名不能为空"; exit 1
    fi
  fi

  # === 生成 .env ===
  cat > .env <<EOF
PUBLIC_IP=${PUBLIC_IP}
ADMIN_TOKEN=${ADMIN_TOKEN}
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

  # === 启动 ===
  echo ""
  echo "🚀 正在构建并启动服务..."
  docker compose up -d --build

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
    cmd_start
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
