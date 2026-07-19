# lite 档位启动冒烟证据

日期：2026-07-19  
范围：`DATA_PROXY_PROFILE=lite`，无 `SQL_DSN` / 无 `REDIS_CONN_STRING`  
方式：本机 `go build` 二进制（含当日 user/token 内存缓存改动），非生产镜像

## 结论

**PASS**：lite 进程启动日志确认 Redis 关闭 + 进程内缓存开启；SQLite setup → 登录 → 创建 API Key → `/v1/models` 鉴权通过（两次，覆盖缓存读路径）；无效 Key 401。

## 环境

| 项 | 值 |
| --- | --- |
| `DATA_PROXY_PROFILE` | `lite` |
| DB | SQLite（`SQLITE_PATH` 临时目录） |
| Redis | 未设置 |
| 端口 | `127.0.0.1:13006` |
| 产物 | `/tmp/data-proxy-lite-smoke`（结束后进程已停） |

## 日志（摘录）

```text
REDIS_CONN_STRING not set, Redis is not enabled
cache backend=memory (single-node; Redis disabled)
DATA_PROXY_PROFILE=lite: SQLite-friendly process-local cache
memory cache enabled
```

## API

| 步骤 | 结果 |
| --- | --- |
| `GET /api/setup` | `database_type=sqlite`，`redis_enabled=false` |
| `POST /api/setup` | 系统初始化成功 |
| login + self | PASS |
| create token + key | PASS |
| `GET /v1/models` ×2 + Key | 200，`data=[]` |
| 无效 Key | 401 |

## 交付配套（同迭代）

- `docker-compose.yml` 默认 `DATA_PROXY_PROFILE=lite`
- `scripts/quickstart.sh` 默认 export `lite`
- 代码：`ApplyCacheDefaults` + user/token HybridCache 内存路径

## 备注

- 未加上游渠道时 models 为空、chat 会 503，不阻塞 lite 缓存路径验收。  
- 完整 Docker 镜像需重新 build 后才带上本提交；本证据以当前源码二进制为准。
