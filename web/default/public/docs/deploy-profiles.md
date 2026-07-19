# 部署档位（Deploy Profiles）

Data Proxy 支持三种部署档位。用环境变量 `DATA_PROXY_PROFILE` 标明意图；**未设置时**按依赖推断（见下）。

| 档位 | 典型依赖 | 适用 |
| --- | --- | --- |
| **lite** | SQLite + **进程内缓存**（无 Redis） | 自用、试用、单机 demo |
| **standard** | MySQL 或 PostgreSQL + **Redis** | 小团队 / 单机生产 |
| **ha** | MySQL/PostgreSQL + Redis + 多节点约定 | 多副本 / 滚动发布（需共享状态） |

别名：`self` / `self-use` → `lite`；`prod` / `production` → `standard`。

## lite（自用 / 极简）

```bash
# 路径 A 或单二进制；可不设 SQL_DSN / REDIS_CONN_STRING
export DATA_PROXY_PROFILE=lite   # 可选；SQLite 无 Redis 时也会自动开内存缓存
docker compose up -d --build
# 或：./scripts/quickstart.sh
```

行为：

- 数据库：默认 SQLite（`./data` 或 `SQLITE_PATH`）
- 缓存：`MEMORY_CACHE_ENABLED` 自动视为开启（渠道内存缓存等）；**不是**嵌入式 Redis 进程
- 已有 `pkg/cachex.HybridCache`：无 Redis 时回退进程内 LRU
- **用户基础信息**（`GetUserCache`）：无 Redis 时写入进程内 `user_base:v1` 缓存（含 group/status/quota 展示字段）；有 Redis 时仍用 HASH + `HINCRBY`
- 限制：
  - **单节点 only**；多副本不共享限流/缓存/部分亲和状态
  - 进程重启后纯缓存丢失；**额度与业务真相在 SQLite**（扣费仍以 DB 事务为准，内存 quota 仅为加速读）
  - 生产支付 / 多机请改用 standard 或 ha

关闭自动内存缓存（不推荐自用）：

```bash
MEMORY_CACHE_ENABLED=false
```

## standard（推荐生产最小）

```bash
export DATA_PROXY_PROFILE=standard
# .env.production：SESSION_SECRET、SQL_DSN、REDIS_CONN_STRING、FRONTEND_BASE_URL
./scripts/prod-compose.sh up -d
```

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
