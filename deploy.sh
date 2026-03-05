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
  echo "╔══════════════════════════════════════════════════════════════════╗"
  echo "║      ts-panel 管理脚本                                          ║"
  echo "╠══════════════════════════════════════════════════════════════════╣"
  echo "║  用法:                                                          ║"
  echo "║    ./deploy.sh start [选项]     启动服务                        ║"
  echo "║    ./deploy.sh stop             停止服务                        ║"
  echo "╠══════════════════════════════════════════════════════════════════╣"
  echo "║  选项 (均可选，未指定则交互式输入):                              ║"
  echo "║    --ip       <IP>       服务器公网 IP                          ║"
  echo "║    --token    <TOKEN>    Admin Token                            ║"
  echo "║    --port     <PORT>     面板端口 (默认 80)                     ║"
  echo "║    --mode     <1|2>      1=IP直连  2=域名模式                   ║"
  echo "║    --domain   <DOMAIN>   域名 (mode=2 时必填)                   ║"
  echo "║    --goproxy  <URL>      Go 代理地址                            ║"
  echo "║    --test     <true|false>  测试模式(使用SQLite) [默认false]    ║"
  echo "║    --db-type  <mysql|sqlite>  数据库类型 [默认mysql]            ║"
  echo "║    --db-host  <HOST>     MySQL主机(外部数据库时填写)            ║"
  echo "║    --db-port  <PORT>     MySQL端口 [默认3306]                   ║"
  echo "║    --db-user  <USER>     MySQL用户 [默认tspanel]                ║"
  echo "║    --db-pass  <PASS>     MySQL密码                              ║"
  echo "║    --db-name  <NAME>     MySQL数据库名 [默认tspanel]            ║"
  echo "╠══════════════════════════════════════════════════════════════════╣"
  echo "║  示例:                                                          ║"
  echo "║    ./deploy.sh start --ip 1.2.3.4 --token abc --port 80         ║"
  echo "║    ./deploy.sh start --ip 1.2.3.4 --token abc --test true       ║"
  echo "╚══════════════════════════════════════════════════════════════════╝"
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
  local ARG_TEST="" ARG_DB_TYPE="" ARG_DB_HOST="" ARG_DB_PORT="" ARG_DB_USER="" ARG_DB_PASS="" ARG_DB_NAME=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --ip)         ARG_IP="$2";       shift 2 ;;
      --token)      ARG_TOKEN="$2";    shift 2 ;;
      --mode)       ARG_MODE="$2";     shift 2 ;;
      --domain)     ARG_DOMAIN="$2";   shift 2 ;;
      --goproxy)    ARG_GOPROXY="$2";  shift 2 ;;
      --port)       ARG_PORT="$2";     shift 2 ;;
      --test)       ARG_TEST="$2";     shift 2 ;;
      --db-type)    ARG_DB_TYPE="$2";  shift 2 ;;
      --db-host)    ARG_DB_HOST="$2";  shift 2 ;;
      --db-port)    ARG_DB_PORT="$2";  shift 2 ;;
      --db-user)    ARG_DB_USER="$2";  shift 2 ;;
      --db-pass)    ARG_DB_PASS="$2";  shift 2 ;;
      --db-name)    ARG_DB_NAME="$2";  shift 2 ;;
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

  # === 数据库配置 ===
  echo ""
  echo "═══════════════════════════════════════"
  echo "      数据库配置"
  echo "═══════════════════════════════════════"

  # 测试模式
  TEST_MODE="false"
  if [[ -n "$ARG_TEST" ]]; then
    TEST_MODE="$ARG_TEST"
    echo "📌 测试模式: $TEST_MODE"
  else
    echo ""
    echo "选择运行模式:"
    echo "  1) 生产模式（使用 MySQL，数据持久化）"
    echo "  2) 测试模式（使用 SQLite，快速部署）"
    read -rp "请输入 1 或 2 [默认 1]: " DB_CHOICE
    DB_CHOICE=${DB_CHOICE:-1}
    if [[ "$DB_CHOICE" == "2" ]]; then
      TEST_MODE="true"
    fi
  fi

  # 初始化数据库配置变量
  DB_TYPE="sqlite"
  DB_HOST=""
  DB_PORT="3306"
  DB_USER=""
  DB_PASSWORD=""
  DB_NAME=""
  MYSQL_ROOT_PASSWORD=""
  USE_EXTERNAL_MYSQL="false"

  # 非测试模式：配置MySQL
  if [[ "$TEST_MODE" != "true" ]]; then
    DB_TYPE="mysql"

    if [[ -n "$ARG_DB_TYPE" ]]; then
      DB_TYPE="$ARG_DB_TYPE"
    fi

    # 检查是否使用外部MySQL
    if [[ -n "$ARG_DB_HOST" ]]; then
      USE_EXTERNAL_MYSQL="true"
      DB_HOST="$ARG_DB_HOST"
      DB_PORT="${ARG_DB_PORT:-3306}"
      DB_USER="${ARG_DB_USER:-tspanel}"
      DB_PASSWORD="$ARG_DB_PASS"
      DB_NAME="${ARG_DB_NAME:-tspanel}"

      echo "📌 使用外部 MySQL: $DB_HOST:$DB_PORT"

      if [[ -z "$DB_PASSWORD" ]]; then
        read -rsp "请输入 MySQL 密码: " DB_PASSWORD
        echo ""
      fi
    else
      echo ""
      echo "是否使用外部 MySQL 数据库?"
      read -rp "输入 y 使用外部数据库，或直接回车启动内置 MySQL [默认内置]: " USE_EXTERNAL_INPUT

      if [[ "$USE_EXTERNAL_INPUT" =~ ^[Yy]$ ]]; then
        USE_EXTERNAL_MYSQL="true"
        read -rp "MySQL 主机地址: " DB_HOST
        read -rp "MySQL 端口 [默认3306]: " DB_PORT_INPUT
        DB_PORT=${DB_PORT_INPUT:-3306}
        read -rp "MySQL 用户名 [默认tspanel]: " DB_USER_INPUT
        DB_USER=${DB_USER_INPUT:-tspanel}
        read -rsp "MySQL 密码: " DB_PASSWORD
        echo ""
        read -rp "MySQL 数据库名 [默认tspanel]: " DB_NAME_INPUT
        DB_NAME=${DB_NAME_INPUT:-tspanel}

        if [[ -z "$DB_HOST" || -z "$DB_PASSWORD" ]]; then
          echo "❌ 外部 MySQL 需要填写主机地址和密码"; exit 1
        fi
      else
        # 使用内置 MySQL
        echo "📌 将自动启动内置 MySQL 容器"
        DB_HOST="mysql"
        DB_PORT="3306"
        DB_USER="tspanel"
        DB_PASSWORD="tspanel_secret"
        DB_NAME="tspanel"
        MYSQL_ROOT_PASSWORD="root_secret"
      fi
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
# 数据库配置
TEST_MODE=${TEST_MODE}
DB_TYPE=${DB_TYPE}
DB_HOST=${DB_HOST}
DB_PORT=${DB_PORT}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_NAME=${DB_NAME}
# MySQL root密码（仅内置MySQL使用）
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
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

  # 根据配置决定启动哪些服务
  if [[ "$TEST_MODE" == "true" ]]; then
    echo "🧪 测试模式：使用 SQLite，不启动 MySQL"
    docker compose build --no-cache --pull
    docker compose up -d
  elif [[ "$USE_EXTERNAL_MYSQL" == "true" ]]; then
    echo "🔗 外部 MySQL 模式：连接至 $DB_HOST:$DB_PORT"
    docker compose build --no-cache --pull
    docker compose up -d
  else
    echo "🐬 内置 MySQL 模式：启动 MySQL 容器"
    docker compose --profile mysql build --no-cache --pull
    docker compose --profile mysql up -d
  fi

  echo ""
  echo "╔══════════════════════════════════════╗"
  echo "║  🎉 部署完成！                       ║"
  echo "╠══════════════════════════════════════╣"
  if [[ "$MODE" == "2" ]]; then
    echo "║  面板地址: https://${DOMAIN}"
  else
    echo "║  面板地址: http://${PUBLIC_IP}:${PANEL_HTTP_PORT}"
  fi
  if [[ "$TEST_MODE" == "true" ]]; then
    echo "║  数据库: SQLite（测试模式）"
  elif [[ "$USE_EXTERNAL_MYSQL" == "true" ]]; then
    echo "║  数据库: MySQL ($DB_HOST:$DB_PORT)"
  else
    echo "║  数据库: MySQL（内置容器）"
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
