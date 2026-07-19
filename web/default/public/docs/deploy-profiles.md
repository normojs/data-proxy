# 部署档位（Deploy Profiles）

用户在安装时**二选一**（或三选一生产变体）：

| 档位 | Compose 文件 | 典型依赖 | 适用 |
| --- | --- | --- | --- |
| **lite** | [`docker-compose.lite.yml`](../docker-compose.lite.yml)（默认 `docker-compose.yml` 同义） | SQLite + **进程内缓存** | 自用、试用、单机 demo |
| **standard** | [`docker-compose.pg-redis.yml`](../docker-compose.pg-redis.yml) **或** 外部库 + [`docker-compose.prod.yml`](../docker-compose.prod.yml) | PostgreSQL/MySQL + **Redis** | 小团队 / 单机生产 |
| **ha** | 多节点共享同一 DB + Redis（仍用 prod compose 变体） | 同上 + 多副本约定 | 滚动发布（需共享状态） |

环境变量 `DATA_PROXY_PROFILE` 标明意图；**未设置时**按依赖推断（见文末）。  
别名：`self` / `self-use` → `lite`；`prod` / `production` → `standard`。

一键命令总览见 [one-click-deploy.md](./one-click-deploy.md)。

```bash
# lite
docker compose -f docker-compose.lite.yml up -d --build
# 或
./scripts/quickstart.sh lite

# PostgreSQL + Redis 一体
cp .env.example.pg-redis .env.pg-redis   # 改密钥
docker compose -f docker-compose.pg-redis.yml --env-file .env.pg-redis up -d --build
# 或
./scripts/quickstart.sh pg-redis
```

## lite（自用 / 极简）

```bash
export DATA_PROXY_PROFILE=lite   # compose 文件已写入；可省略
docker compose -f docker-compose.lite.yml up -d --build
```

行为：

- 数据库：默认 SQLite（`./data` 或 `SQLITE_PATH`）
- 缓存：`MEMORY_CACHE_ENABLED` 自动视为开启（渠道内存缓存等）；**不是**嵌入式 Redis 进程
- 已有 `pkg/cachex.HybridCache`：无 Redis 时回退进程内 LRU
- **用户基础信息**（`GetUserCache`）：无 Redis 时写入进程内 `user_base:v1` 缓存（含 group/status/quota 展示字段）；有 Redis 时仍用 HASH + `HINCRBY`
- **API Token**（`GetTokenByKey`）：无 Redis 时写入进程内 `token:v1`（按 key HMAC，不存明文 secret）；有 Redis 时仍用 HASH
- **HTTP 限流**（全局 / 模型 / 邮件验证等）：无 Redis 时已用 `common.InMemoryRateLimiter`（进程内滑动窗口）；有 Redis 时用 list/incr
- **站内通知限流**：无 Redis 时用 `service` 包内 `sync.Map` 计数（`CheckNotificationLimit`）
- **MCP 结算后 token 缓存刷新**、**禁用/批量删 token 缓存失效**：无 Redis 时同样更新/删除进程内 token 缓存
- 限制：
  - **单节点 only**；多副本不共享限流/缓存/部分亲和状态
  - 进程重启后纯缓存丢失；**额度与业务真相在 SQLite**（扣费仍以 DB 事务为准，内存 quota 仅为加速读）
  - 性能看板跨实例合并（`perf_metrics` Redis 桶）在无 Redis 时跳过，仅本机指标
  - 需要 PG/Redis 或支付生产请改用 standard

关闭自动内存缓存（不推荐自用）：

```bash
MEMORY_CACHE_ENABLED=false
```

## standard（PostgreSQL/MySQL + Redis）

### 一体 compose（推荐首次上标准档）

```bash
cp .env.example.pg-redis .env.pg-redis
# 修改 SESSION_SECRET、POSTGRES_PASSWORD、REDIS_PASSWORD
docker compose -f docker-compose.pg-redis.yml --env-file .env.pg-redis up -d --build
```

内置服务名：`postgres`、`redis`；应用环境自动带 `SQL_DSN` / `REDIS_CONN_STRING` 与 `DATA_PROXY_PROFILE=standard`。

### 外部数据库（已有实例）

```bash
export DATA_PROXY_PROFILE=standard
# .env.production：SESSION_SECRET、SQL_DSN、REDIS_CONN_STRING、FRONTEND_BASE_URL
./scripts/prod-compose.sh up -d
```

见 [one-click-deploy.md](./one-click-deploy.md) 路径 B。
见 [one-click-deploy.md](./one-click-deploy.md) 路径 B。

## ha（多节点）

在 standard 基础上：

- 所有 app 节点同一 `SQL_DSN` + 同一 `REDIS_CONN_STRING`
- `SESSION_SECRET` 一致
- 临时渠道熔断等**部分**状态仍为单机内存（见 [channel-failover-and-circuit-breaker.md](./channel-failover-and-circuit-breaker.md)）
- 跨节点 SSE / 分布式 Tunnel 限流等**不在**本档位承诺内（见 product-gap 后置项）

## 与代码开关的关系

| 开关 | 含义 |
| --- | --- |
| `DATA_PROXY_PROFILE` | 部署意图；影响默认缓存策略与文档路径 |
| `SQL_DSN` | 空 → SQLite；MySQL/PG 连接串 → 对应驱动 |
| `REDIS_CONN_STRING` | 空 → `RedisEnabled=false`；非空 → Redis |
| `MEMORY_CACHE_ENABLED` | 渠道等进程内缓存；Redis 开启时强制 true；lite/SQLite 默认 true |
| `SelfUseModeEnabled` | **产品**开关（模型倍率等展示），不是部署拓扑 |

## 自动规则（实现）

`common.ApplyCacheDefaults()`（启动时，在 DB/Redis 初始化之后）：

1. 若 Redis 已启用 → 打开 memory channel cache（兼容旧行为）
2. 若 `MEMORY_CACHE_ENABLED=false` → 保持关闭
3. 若 `DATA_PROXY_PROFILE=lite` 或（未设 profile 且当前为 SQLite）→ 打开 memory cache，日志：`cache backend=memory (single-node; Redis disabled)`

## 明确不做（本档位）

- 进程内再嵌一套 Redis 协议服务（miniredis 常驻）
- 用 SQLite 表模拟通用 Redis KV
- 多节点共享「纯内存」熔断状态（需后续专项）

## 相关文档

- 一键部署：[one-click-deploy.md](./one-click-deploy.md)
- 用户 3 分钟接入：[user-quickstart.md](./user-quickstart.md)
- 路径 B + 外部库复验：`docs/p1-compose-pathb-mysql-redis-e2e-evidence-2026-07-19.md`
