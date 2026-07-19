# 一键部署 Data Proxy

面向首次部署：先选档位，再选对应 compose 文件。

| 场景 | 文件 | 依赖 |
| --- | --- | --- |
| **自用 / 试用（lite）** | [`docker-compose.lite.yml`](../docker-compose.lite.yml) | 仅应用；SQLite + 进程内缓存 |
| **标准（PG + Redis）** | [`docker-compose.pg-redis.yml`](../docker-compose.pg-redis.yml) | 内置 PostgreSQL + Redis + 应用 |
| **已有外部库的生产** | [`docker-compose.prod.yml`](../docker-compose.prod.yml) + `scripts/prod-compose.sh` | 你自备 MySQL/PG + Redis |

档位细节：[deploy-profiles.md](./deploy-profiles.md)

默认 `docker-compose.yml` 与 **lite** 等价，便于 `docker compose up`。

---

## 路径 A1：lite（推荐自用）

```bash
git clone <your-repo-url> data-proxy
cd data-proxy/upstream/new-api   # 或代码根目录

docker compose -f docker-compose.lite.yml up -d --build
# 或：
# docker compose up -d --build
# 或：
# ./scripts/quickstart.sh lite
```

1. 打开 `http://localhost:3000`
2. 完成初始化向导（保持 SQLite 即可）
3. 创建管理员 → 添加渠道与模型 → 按 [user-quickstart.md](./user-quickstart.md) 调一次 API
4. 健康检查：`curl -s http://127.0.0.1:3000/api/status`

无 `SQL_DSN` / `REDIS_CONN_STRING`：SQLite + 进程内缓存（日志可见 `cache backend=memory`）。

---

## 路径 A2：PostgreSQL + Redis（一体）

```bash
cp .env.example.pg-redis .env.pg-redis
# 编辑 .env.pg-redis：SESSION_SECRET、POSTGRES_PASSWORD、REDIS_PASSWORD

docker compose -f docker-compose.pg-redis.yml --env-file .env.pg-redis up -d --build
# 或：./scripts/quickstart.sh pg-redis
```

1. 打开 `http://localhost:3000`（首次可能需等 PG/Redis healthy）
2. 初始化管理员（库已由 `SQL_DSN` 指向内置 Postgres）
3. 添加渠道 → 验证请求
4. `curl -s http://127.0.0.1:3000/api/status`

密码勿含未转义的 `@` `:` `#`（会写进 DSN URL）。需要换端口：`DATA_PROXY_HOST_PORT=3001`。

停止：

```bash
docker compose -f docker-compose.pg-redis.yml --env-file .env.pg-redis down
# 连同数据卷：down -v
```

---

## 路径 B：已有外部数据库的生产

文件：

- `docker-compose.prod.yml`（主服务，默认 `127.0.0.1:13002`）
- `docker-compose.wechat-pay.yml`（可选；无微信支付可准备空 `secrets/wechatpay`）

```bash
cp .env.example.minimal .env.production
# 编辑：SESSION_SECRET、SQL_DSN、REDIS_CONN_STRING、FRONTEND_BASE_URL
# 建议：DATA_PROXY_PROFILE=standard

mkdir -p secrets/wechatpay data logs
export DATA_PROXY_IMAGE=ghcr.io/normojs/data-proxy:<tag>
./scripts/prod-compose.sh up -d
```

健康检查：

```bash
curl -s http://127.0.0.1:13002/api/status
```

容器访问宿主机上的库可用 `host.docker.internal`（prod compose 已配 `extra_hosts`）。  
脚本：`scripts/prod-deploy.sh`、`scripts/prod-rollback.sh`。

---

## 环境变量（按路径）

| 变量 | lite | pg-redis | 外部 prod |
|------|------|----------|-----------|
| `SESSION_SECRET` | 可选（试用可随机） | **必改** | **必改** |
| `SQL_DSN` | 省略 → SQLite | compose 注入 PG | 自备 |
| `REDIS_CONN_STRING` | 省略 → 内存 | compose 注入 Redis | 推荐 |
| `DATA_PROXY_PROFILE` | `lite` | `standard` | `standard`/`ha` |
| `FRONTEND_BASE_URL` | 可选 | 建议设置 | 必填（支付等） |

模板：

- lite / 通用最小：[`.env.example.minimal`](../.env.example.minimal)
- pg-redis：[`.env.example.pg-redis`](../.env.example.pg-redis)
- 全量：[`.env.example`](../.env.example)

---

## 验收清单

- [ ] `/api/status` 返回 `success: true`
- [ ] 管理员可登录控制台
- [ ] 至少 1 个渠道 + 模型可用
- [ ] 用户 Key 可调用 `/v1/chat/completions`
- [ ] （可选）Monitoring 应用「安全故障切换预设」

## 相关文档

- 部署档位：[deploy-profiles.md](./deploy-profiles.md)
- 终端用户接入：[user-quickstart.md](./user-quickstart.md)
- 渠道故障切换：[channel-failover-and-circuit-breaker.md](./channel-failover-and-circuit-breaker.md)
- 运维手册：[data-proxy-operator-guide.md](./data-proxy-operator-guide.md)
