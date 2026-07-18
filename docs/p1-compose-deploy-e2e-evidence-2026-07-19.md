# P1-3 一条 compose 部署路径复验证据

日期：2026-07-19  
范围：`docs/one-click-deploy.md` 路径 A（本机最快）+ 路径 B compose 配置可解析 + 公网文档可达  
主机：开发机 Docker（非生产 47.122.29.88）；与现有 `data-proxy-dev:3000` 隔离

## 结论

**PASS（部署路径）**：按文档可完成「compose 配置校验 → 拉起服务 → `/api/status` 健康 → 初始化管理员 → 登录 → 创建 API Key → Key 鉴权生效」。

未接上游渠道时 chat 返回 `model_not_found` 为预期；文档验收清单中的「渠道 + 成功 chat」依赖运营配置上游，不阻塞「一条 compose 起服务」退出标准。

## 复验方式说明

| 项 | 说明 |
| --- | --- |
| 文档路径 A | `docker compose up -d --build` / `scripts/quickstart.sh` |
| 本轮隔离 | 宿主机 `3000` 已被 `data-proxy-dev` 占用，使用 **等价 compose** 映射 `13003:3000`、独立 data/logs 目录与容器名，避免污染现有实例 |
| 镜像 | 使用本地已构建镜像 tag 为 `data-proxy:p1-compose-e2e`（对应 compose 在 build/pull 之后的运行阶段） |
| 数据库 | 默认 SQLite（文档允许：先 up 再向导；本轮 `/api/setup` 直接建管理员） |

## 步骤与证据

### 1. 文档与脚本可达

| 检查 | 结果 |
| --- | --- |
| 公网 `https://dp.app.mbu.ltd/docs/one-click-deploy.md` | HTTP 200，2498B，正文以「# 一键部署 Data Proxy」开头 |
| `scripts/quickstart.sh` | `bash -n` 语法 OK |
| `scripts/prod-compose.sh` | `bash -n` 语法 OK |

### 2. Compose 配置可解析

```bash
docker compose -f docker-compose.yml config
docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml config
```

| 文件 | 结果 |
| --- | --- |
| `docker-compose.yml`（路径 A） | config 成功（44 lines rendered） |
| `docker-compose.prod.yml` + `docker-compose.wechat-pay.yml`（路径 B） | config 成功（54 lines rendered） |

### 3. 拉起服务

```bash
# 隔离 compose（端口 13003，独立 /tmp/data-proxy-p1-compose-e2e/{data,logs}）
docker compose -f /tmp/data-proxy-p1-compose-e2e/compose.e2e.yml up -d
```

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy-p1-compose-e2e` Up |
| Docker health | `healthy` |
| `GET http://127.0.0.1:13003/api/status` | `success=true` |
| 数据文件 | `/data/one-api.db`（SQLite）已创建 |

### 4. 初始化与控制台路径

| 步骤 | 结果 |
| --- | --- |
| `GET /api/setup`（初始） | `status=false`，`database_type=sqlite`，`database_source=sqlite-default` |
| `POST /api/setup` 创建管理员 | `success=true`，message「系统初始化成功」 |
| `GET /api/setup`（之后） | `status=true` |
| `POST /api/user/login` | `success=true`，role=100 |
| `GET /api/user/self` | `success=true`，username=`e2eadmin` |

### 5. API Key 路径（文档「创建 Key 再请求」）

| 步骤 | 结果 |
| --- | --- |
| `POST /api/token/` | 创建成功（列表可见 token id=1/2） |
| `POST /api/token/:id/key` | 返回完整 key（仅 /tmp，未入库） |
| `GET /v1/models` + Key | HTTP 200，`data=[]`（尚无渠道/模型） |
| `POST /v1/chat/completions` + Key | HTTP 503，`model_not_found` / no available channel（**鉴权通过**，缺渠道） |
| 无效 Key | HTTP 401 Invalid token |

## 清理

- `docker compose -f compose.e2e.yml down` 已执行
- 临时 cookie / API key 文件已删除
- 未改动生产环境与现有 `data-proxy-dev`

## 与退出标准对应

- [x] 新人按文档一条 compose 路径完成部署  
  - 文档公网可达  
  - compose 文件可解析  
  - 等价 compose up → status 健康 → setup/login/token 闭环  
  - chat 成功依赖添加上游渠道（文档清单后续步骤，非 compose 本身阻塞项）

## 备注 / 建议

1. 若本机 3000 已被占用，文档可补一句「改 `ports` 或停旧实例」；本轮用 13003 验证。  
2. 路径 B 生产 compose 本轮做了 **config 级**校验；真实镜像拉取/`prod-compose.sh up` 仍建议在干净空机器或预发再跑一次（需 `DATA_PROXY_IMAGE` / SQL_DSN）。  
3. `scripts/quickstart.sh` 固定探活 `127.0.0.1:3000`，与路径 A 默认端口一致。  
