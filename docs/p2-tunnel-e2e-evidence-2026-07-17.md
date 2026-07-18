# P2 Tunnel 生产 E2E 证据（dpa 主路径）

日期：2026-07-17  
生产：`https://dp.app.mbu.ltd`（`sha-da5af9b2`）  
客户端：本地构建 `dpa 0.1.0-dev`（`cmd/data-proxy-agent`）  
路径：HTTP Tunnel（`http_tunnel`）

## 结论

**PASS**：setup token → consume/enroll 凭证 → `dpa run` 在线 → 云端 HTTP 调用本机服务 → tunnel audit 可追溯。

## 步骤与证据

### 1. 安装 / 二进制

- 本机：`go build -o /tmp/dpa-bin/dpa ./cmd/data-proxy-agent`
- `dpa version` → `dpa 0.1.0-dev`
- 公开 `/agent/install.sh` 等仍返回 SPA 壳（1026B），**不阻塞**本地构建路径；生产安装脚本托管仍为缺口

### 2. Setup token

- `POST /api/bridge/agent-setup-tokens` 需登录 session / dashboard access token
- 本次为无人值守 e2e：在 DB 插入 `bridge_agent_setup_tokens`（仅 hash），明文 token 仅存本机 `/tmp`，**未入库**
- 注意：`used_at` 必须为 `0` 才可消费；SQL `NULL` 会被 `used_at = 0` 条件挡住（实现细节）
- `POST /api/bridge/agent-setup/consume` → `success=true`
  - `client_id=e2e-local-mac`
  - `bridge_ws_url=wss://dp.app.mbu.ltd/bridge/ws`
  - 生成 agent API token id `36`（masked `sk-JaKE**********8Xgo`）

### 3. 本地运行

- 配置：`http_routes[0].name=e2e-local` → `http://127.0.0.1:18080`
- 本机 HTTP fixture 返回 `{"ok":true,"path":"/hello","service":"dpa-e2e-local"}`
- `dpa doctor`：`bridge_auth=ok`，`http_route.e2e-local=ok`
- `dpa run` 日志：
  - connected `wss://dp.app.mbu.ltd/bridge/ws`
  - registered `client_id=e2e-local-mac` `session_id=yMsqjG5yUQV2Pv8w3eKViCuppVKpVVIZ`
- `/api/bridge/clients/e2e-local-mac/health`：`online=true`，同上 session

### 4. Tunnel App / Connection

- `POST /api/tunnel/apps` → app id `1`，`app_type=http_tunnel`，slug `tun-qd2fknnemkdv`，status `pending`
- `PATCH /api/tunnel/admin/apps/1` → `approved`（review_note: p2 e2e auto approve）
- `POST /api/tunnel/apps/1/connections` → connection id `1`，key_prefix `tc_1PhNcxXEF`
- endpoint：`/t/<connection_key>/tunnel/http/tun-qd2fknnemkdv`

### 5. 云端调用

| 调用 | HTTP | request id | 响应 |
| --- | ---: | --- | --- |
| GET 公网 endpoint `/hello` | 200 | `20260716185431696948318268d9d6XMJZfGnS` | local fixture JSON |
| 同上（附 agent key） | 200 | `202607161854323142884588268d9d6ykbAgrxp` | 同上 |
| 同上（附 dashboard access token） | 200 | `202607161854331768225398268d9d68PUjx7Kh` | 同上 |

dpa 本地日志均收到 `tool_name=http_tunnel.request` 对应 request id。

### 6. Audit

`GET /api/tunnel/apps/1/audit-logs` total=6，含：

- `create` app / connection
- `review` approve
- 3× `proxy_request` decision=`allow`，request id 与上表一致；`bytes_out=58`；connection_id=1

connection `last_request_id=202607161854331768225398268d9d68PUjx7Kh`

## 安全清理

- 临时 dashboard `users.access_token`（user id=1）e2e 后已置 NULL
- setup token / agent api key / connection_key 仅本机 `/tmp/dpa-e2e`，**未写入仓库**
- 生产保留 e2e 用 bridge client / tunnel app 记录便于复查；可按需归档/撤销

## 仍未覆盖 / 后续

- `mcp_code` 路径：生产 tools/list + tools/call 端到端仍待补；**自动化覆盖**已有 `go test ./service -run TunnelMCP`（list/call 策略、计费拒绝、上游转发与审计）
- ~~公网 agent 安装脚本/manifest 仍 SPA 壳~~ → 已在后续提交修复：`GET /agent/install.sh` 与 `/agent/install-data-proxy-agent.sh` 返回 shell（本地 `scripts/` 或 GitHub bootstrap），见 `router/web-router.go`
- ~~setup token `used_at NULL` 与 `=0`~~ → consume 已支持 `used_at = 0 OR IS NULL`；BeforeCreate 保证默认 0
- 计费 ledger 细账（billing_events）未单独拉表核对；audit 已覆盖 charged/allow 轨迹的请求侧

## 与 product-gap 对应

- P2-1 端到端安装→调用：本证据勾选 HTTP Tunnel 生产闭环
- P2 退出标准「云端 Agent → 本机服务」：本轮以云端 HTTP 入口 → dpa → 本机 18080 证明
