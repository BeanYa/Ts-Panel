# ts-panel

TeamSpeak 容器管理与发货面板（Go + Gin + Docker + SQLite）。

每个客户 = 一个独立的 TeamSpeak 容器，自动分配端口、抓取密钥、应用 slots 限制，并生成可复制的发货文本。

## 快速开始

### 一键部署（推荐）

```bash
# 克隆项目
git clone <repo> ts-panel && cd ts-panel

# 赋权并执行
chmod +x deploy.sh && ./deploy.sh
```

脚本会交互式引导：输入公网 IP、Admin Token、选择 IP 直连或域名 HTTPS 模式。

---

### 手动部署

```bash
cp .env.example .env
# 编辑 .env 填写 PUBLIC_IP 和 ADMIN_TOKEN

docker compose up -d --build
```

---

## 使用说明

### 访问面板

| 模式 | 地址 |
|------|------|
| IP 直连 | `http://<PUBLIC_IP>` |
| 域名 HTTPS | `https://<DOMAIN>` |

使用 Admin Token 登录后：
1. **发货页**：填写客户信息 → 点击「创建并发货」→ 复制发货文本给买家
2. **实例列表**：查看所有实例状态，支持重启、回收、删除

---

## API 说明

所有 `/api/*` 请求需携带 Header：`X-Admin-Token: <your-token>`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET  | `/healthz` | 健康检查 |
| POST | `/api/instances/checkout` | 创建并发货（幂等） |
| GET  | `/api/instances` | 实例列表 |
| GET  | `/api/instances/:id` | 实例详情 |
| POST | `/api/instances/:id/start` | 启动 |
| POST | `/api/instances/:id/stop` | 停止 |
| POST | `/api/instances/:id/restart` | 重启 |
| POST | `/api/instances/:id/recycle` | 回收（解绑客户） |
| DELETE | `/api/instances/:id?confirm=true` | 彻底删除 |
| POST | `/api/instances/:id/capture-secrets` | 手动抓取密钥 |
| POST | `/api/instances/:id/apply-slots` | 手动应用 slots |

### Checkout 请求格式

```json
{
  "platform": "xianyu",
  "platform_user": "买家用户名",
  "order_no": "订单号（可选，用于幂等）",
  "slots": 15,
  "reuse_recycled": false
}
```

---

## 升级

```bash
git pull origin main
docker compose up -d --build
```

升级不影响运行中的 TeamSpeak 容器（仅重建 controller 和 panel 镜像）。

---

## 备份

重要数据位于 `/data/`：

```bash
# 备份数据库
cp /data/db/app.db /backup/app.db.$(date +%Y%m%d)

# 备份实例数据（TeamSpeak 配置文件）
tar czf /backup/instances-$(date +%Y%m%d).tar.gz /data/instances/
```

建议每日自动备份：

```cron
0 3 * * * cp /data/db/app.db /backup/app.db.$(date +\%Y\%m\%d)
```

---

## 端口规划

| 用途 | 范围 | 说明 |
|------|------|------|
| TeamSpeak 语音（UDP） | 20000-20999 | 公网开放 |
| ServerQuery（TCP） | 21000-21999 | 仅本机 127.0.0.1 |
| ts-panel HTTP | 80 / 443 | 面板访问 |
| controller 内部 | 8080 | 仅容器网络 |

---

## 目录结构

```
ts-panel/
├── src/               # Go 源码
│   ├── main.go
│   ├── config/
│   ├── db/
│   ├── docker/
│   ├── tsquery/
│   ├── port/
│   ├── service/
│   └── api/
├── panel/             # 前端静态文件
│   ├── index.html
│   ├── style.css
│   ├── app.js
│   └── Dockerfile
├── Dockerfile         # controller 多阶段构建
├── docker-compose.yml
├── deploy.sh
└── .env.example
```

---

## 运行测试

```bash
go test ./src/...
```
