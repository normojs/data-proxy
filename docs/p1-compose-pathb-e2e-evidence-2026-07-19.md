# P1-3 路径 B：生产 compose 运行复验证据

日期：2026-07-19  
范围：`docs/one-click-deploy.md` 路径 B（`docker-compose.prod.yml` + `docker-compose.wechat-pay.yml` + `.env.production`）  
主机：开发机 Docker；与 `data-proxy-dev:3000`、路径 A 复验实例隔离  
镜像：本地已有 `data-proxy:p1-compose-e2e`（不拉 ghcr，避免无凭据/污染）

## 结论

**PASS（路径 B 运行路径）**：在隔离目录用与文档相同的 compose 文件集合启动后，服务 healthy，初始化管理员 → 登录 → 创建 API Key → Key 鉴权生效；微信 secrets 卷已挂载；未触碰仓库内真实 `.env.production`（含生产 SQL/Redis）。

此前 P1-3 仅对路径 B 做了 `docker compose config`；本轮补齐 **up + 健康 + 控制台闭环**。

## 复验方式

| 项 | 说明 |
| --- | --- |
| 工作目录 | `/tmp/data-proxy-p1-pathb-e2e`（独立 data/logs/secrets，结束后 down） |
| Compose 文件 | `docker-compose.prod.yml` + `docker-compose.wechat-pay.yml` + 端口覆盖 |
| 端口 | 宿主机 `127.0.0.1:13004` → 容器 `13002`（文档默认 health 形态） |
| 环境 | 临时 `.env.production`：仅 `SESSION_SECRET` / `FRONTEND_BASE_URL` / `TZ`；**无** `SQL_DSN` → SQLite |
| 微信 | 空目录 `secrets/wechatpay`（文档允许无微信支付） |
| 镜像 | `DATA_PROXY_IMAGE=data-proxy:p1-compose-e2e` |
| 项目名 | `data-proxy-p1-pathb`（避免容器名 `data-proxy` 冲突） |

未使用仓库根目录 `.env.production`，避免把本机开发会话接到真实生产 DSN。

## 步骤与证据

### 1. Compose config

```text
docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml \
  -f docker-compose.e2e-override.yml --env-file .env.production config
→ 50 lines rendered
```

### 2. 拉起与健康

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy-p1-pathb-e2e` Up |
| Docker health | `healthy` |
| `GET http://127.0.0.1:13004/api/status` | `success=true` |
| mounts | `./data`→`/data`，`./logs`→`/app/logs`，`./secrets/wechatpay`→`/run/secrets/data-proxy/wechatpay` |

### 3. 初始化与 API Key

| 步骤 | 结果 |
| --- | --- |
| `GET /api/setup` | `status=false`，`database_type=sqlite`，`database_source=sqlite-default` |
| `POST /api/setup`（含 `confirmPassword`） | `success=true`，「系统初始化成功」 |
| `POST /api/user/login` | `success=true`，role=100 |
| `GET /api/user/self` | `success=true`（需 session + `New-Api-User`） |
| `POST /api/token/` + `POST /api/token/:id/key` | 创建成功，完整 key 仅写 `/tmp` |
| `GET /v1/models` + Key | HTTP 200，`data=[]` |
| `POST /v1/chat/completions` + Key | HTTP 503，`model_not_found` / no available channel（鉴权通过） |
| 无效 Key | HTTP 401 Invalid token |

### 4. 脚本说明

- 仓库内 `scripts/prod-compose.sh` 会 `cd` 到**仓库根**再拼 compose 文件；必须在代码树执行，不能只拷贝脚本到 `/tmp`。
- 本轮等价命令：与 `prod-compose.sh` 相同的 `-f docker-compose.prod.yml -f docker-compose.wechat-pay.yml`，外加隔离 project/port/env。
- `bash -n scripts/prod-compose.sh` / 拷贝脚本语法：OK。

## 清理

- `docker compose … down` 已执行，容器与 network 已删  
- 临时 `.env.production`、`api_key` 已删  
- 未修改生产 `47.122.29.88` 与仓库 `.env.production`

## 与退出标准 / 文档

- 路径 A：见 `docs/p1-compose-deploy-e2e-evidence-2026-07-19.md`  
- 路径 B（SQLite 默认）：本文件 — **运行级 PASS**（不仅 config）  
- 路径 B + 外部 MySQL/Redis：见 `docs/p1-compose-pathb-mysql-redis-e2e-evidence-2026-07-19.md`  
- 仍未覆盖：从 ghcr 拉官方 `DATA_PROXY_IMAGE`（需 registry 凭据）

## 备注

1. 默认绑定文档为 `127.0.0.1:13002`；本机若占用可改 `DATA_PROXY_HOST_PORT` 或 compose `ports`。  
2. setup 请求体需 `confirmPassword` 与 `password` 一致。  
3. 会话 API 除 Cookie 外需 `New-Api-User` 用户 id 头（与控制台一致）。
