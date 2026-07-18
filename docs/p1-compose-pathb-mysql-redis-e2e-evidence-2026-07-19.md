# P1-3 路径 B：外部 MySQL + Redis 运行复验证据

日期：2026-07-19  
范围：`docs/one-click-deploy.md` 路径 B（prod compose + wechat-pay）+ 真实 `SQL_DSN` / `REDIS_CONN_STRING`  
主机：开发机 Docker（无独立空机器；用容器模拟外部依赖）  
镜像：`data-proxy:p1-compose-e2e`（本地 tag，未拉 ghcr）

## 结论

**PASS**：在 Docker 中用**外部** MySQL 与 Redis（环境变量注入，非默认 SQLite）拉起路径 B 服务后：

- 容器 **healthy**
- `/api/setup` 报告 `database_type=mysql`、`database_source=env`、`redis_enabled=true`、`redis_source=env`
- 初始化管理员 → 登录 → 创建 API Key → Key 鉴权生效
- chat 无渠道时 503 `model_not_found`（鉴权通过）
- 结束后 down 容器并删除 e2e 库/用户；临时密钥未入库

## 依赖拓扑

| 组件 | 实现 |
| --- | --- |
| MySQL | 复用本机已有 `examples-mysql-primary-1`（MySQL **8.0**，宿主机 `13306`）上新建库 `data_proxy_pathb` / 用户 `pathb` |
| Redis | 临时容器 `data-proxy-p1-pathb-redis`（`redis:7-alpine`，宿主机 `16380`） |
| Data Proxy | `docker-compose.prod.yml` + `docker-compose.wechat-pay.yml` + 端口覆盖，`127.0.0.1:13005→13002` |
| 连法 | 容器内 `host.docker.internal:13306` / `:16380`（compose 已有 `extra_hosts: host-gateway`） |

未使用仓库根 `.env.production`（避免接到真实生产 DSN）。临时 env 仅在 `/tmp`，已删。

## 尝试与修正

1. 先起独立 `mysql:8`（实际 **8.4**）容器：  
   - `default-authentication-plugin` 在 8.4 非法 → 去掉后仍因**半初始化 volume** 报 `mysql.user` 不存在。  
2. 改用本机已健康的 **MySQL 8.0** 实例 + 专用库/用户，Redis 单独容器 → **成功**。

说明：路径 B 文档允许「外部 MySQL/Redis 配置」；本轮验证的是 **DSN/Redis 环境变量路径**，不是「必须再起一套 compose 内置 DB」。

## 步骤与证据

### 1. 外部依赖

| 检查 | 结果 |
| --- | --- |
| `CREATE DATABASE data_proxy_pathb` + user `pathb` | PASS |
| Redis `PING` / 端口 `16380` | PASS |

### 2. Compose up

| 检查 | 结果 |
| --- | --- |
| config（prod + wechat + override） | OK |
| 容器 `data-proxy-p1-pathb-full` | Up **healthy** |
| `GET http://127.0.0.1:13005/api/status` | `success=true` |
| 容器 env（脱敏） | `SQL_DSN=pathb:***@tcp(host.docker.internal:13306)/data_proxy_pathb?...`；`REDIS_CONN_STRING=redis://host.docker.internal:16380/0` |

### 3. Setup / 控制台 / Key

| 步骤 | 结果 |
| --- | --- |
| `GET /api/setup` | `status=false`，**mysql** + **redis_enabled=true**（source=env） |
| `POST /api/setup` | `success=true`，「系统初始化成功」 |
| login / self | role=100 |
| create token + reveal key | PASS（key 仅 `/tmp`） |
| `GET /v1/models` + Key | 200，`data=[]` |
| chat | 503 `model_not_found`（无渠道，鉴权 OK） |
| 无效 Key | 401 |

### 4. 库表侧证

| 检查 | 结果 |
| --- | --- |
| `information_schema.tables` in `data_proxy_pathb` | **99** 张表（迁移已跑） |
| Redis `DBSIZE`（初始化后） | **5**（会话/缓存有写入） |

## 清理

- `docker compose … down`；删除 `data-proxy-p1-pathb-redis`
- `DROP DATABASE data_proxy_pathb`；`DROP USER pathb`
- 删除临时 `.env.production` / `api_key`
- 未改生产 `47.122.29.88`、未改仓库密钥文件

## 与前序证据关系

| 证据 | 覆盖 |
| --- | --- |
| `docs/p1-compose-deploy-e2e-evidence-2026-07-19.md` | 路径 A（开发 compose / SQLite） |
| `docs/p1-compose-pathb-e2e-evidence-2026-07-19.md` | 路径 B 文件集 + **SQLite** 默认 |
| **本文件** | 路径 B + **外部 MySQL + Redis** |

仍未覆盖：从 **ghcr 拉官方镜像**（需 registry 凭据）。本地 `DATA_PROXY_IMAGE` tag 已等价验证 compose 运行路径。

## 备注

1. MySQL 8.4 官方镜像与 8.0 初始化参数不同；文档示例 DSN 不绑定次要版本，但运维应用 8.0/8.4 时注意鉴权插件与空 volume 初始化。  
2. `prod-compose.sh` 须在仓库根执行；本轮用同文件集合的 `docker compose -f …` 隔离 project 名。  
3. 无上游渠道时 chat 503 符合预期，不阻塞「一条 compose + 外部库起服务」。
