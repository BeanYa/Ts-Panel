#!/usr/bin/env bash
set -e

echo ""
echo "╔══════════════════════════════════════╗"
echo "║    ts-panel 一键部署脚本             ║"
echo "╚══════════════════════════════════════╝"
echo ""

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
