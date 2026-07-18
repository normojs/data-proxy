# P2 mcp_code 生产 E2E 证据（dpa 主路径）

日期：2026-07-19  
生产：`https://dp.app.mbu.ltd`（`sha-5f695ffe`）  
客户端：本地构建 `dpa 0.1.0-dev`（`cmd/data-proxy-agent`）  
路径：`mcp_code` Tunnel MCP 网关（`/t/<connection_key>/tunnel/mcp/<slug>`）

## 结论

**PASS**：setup token → consume/enroll 凭证 → `dpa run` 在线 → 云端 `initialize` / `tools/list` / `tools/call` → 本机假 MCP（`127.0.0.1:19090/mcp`）→ Tunnel audit 可追溯。

## 步骤与证据

### 1. 本机 fixture / 二进制

- `go build -o /tmp/dpa-mcp-e2e/dpa ./cmd/data-proxy-agent`
- `dpa version` → `dpa 0.1.0-dev`
- 本机假 MCP：`http://127.0.0.1:19090/mcp`（streamable HTTP）
  - `tools/list` 暴露 `remote_env_info`（`readOnlyHint`）
  - `tools/call remote_env_info` 返回 `{"ok":true,"service":"dpa-mcp-e2e",...}`

### 2. Setup token / Bridge client

- 临时 dashboard `users.access_token`（user id=1，`char(32)`）仅用于控制面 API；e2e 后已置 `NULL`
- `POST /api/bridge/agent-setup-tokens` → `client_id=e2e-mcp-mac`
- `POST /api/bridge/agent-setup/consume` → 生成 agent API token（masked 仅日志；明文仅 `/tmp`）
- Bridge client：`e2e-mcp-mac` / user_id=1 / token_id=39

### 3. Tunnel App / Connection

| 对象 | 值 |
| --- | --- |
| app id | `2` |
| app_type | `mcp_code` |
| status | `approved` |
| public_slug | `tun-bqzyhbyt2mmx` |
| bridge_client_id | `e2e-mcp-mac` |
| target | `127.0.0.1:19090/mcp` |
| permission_mode | `read_only` |
| connection id | `2` |
| key_prefix | `tc_wVrMxwHKe` |
| endpoint | `/t/<connection_key>/tunnel/mcp/tun-bqzyhbyt2mmx` |

审批：`PATCH /api/tunnel/admin/apps/2`（note: p2 mcp_code e2e auto approve）

### 4. 本地 dpa

- `dpa doctor`：`bridge_auth=ok`，`mcp_server.e2e-fake=ok`
- `dpa run` 日志：
  - connected `wss://dp.app.mbu.ltd/bridge/ws`
  - registered `client_id=e2e-mcp-mac` `session_id=zB8qVwbrWn0ovi7jXp0Eoxpoa6pTAksl`
  - received `mcp_proxy.tools_list` / `mcp_proxy.tools_call`

### 5. 云端 MCP 调用

鉴权：`Authorization: Bearer <agent sk- from setup consume>` + path `connection_key`。

| 调用 | 结果 |
| --- | --- |
| `initialize` | HTTP 200；`serverInfo.name=data-proxy-tunnel-mcp` `version=0.2.0`；返回 `Mcp-Session-Id` |
| `tools/list` | HTTP 200；tools=`[remote_env_info]` |
| `tools/call remote_env_info` | HTTP 200；content text 含 `service=dpa-mcp-e2e`；metadata.target=`http://127.0.0.1:19090/mcp` |

dpa 侧对应 request：

- `mcp_proxy.tools_list` request_id=`M5nFNXmzxzjT0qtgA1c1HcKVcYizjSrN`
- `mcp_proxy.tools_list` request_id=`3uDvRMFAFXr6OqQWbKNN3zlbf0pkEnUe`
- `mcp_proxy.tools_call` request_id=`3`

### 6. Audit

`GET /api/tunnel/apps/2/audit-logs` 含（节选）：

| action | decision | method / tool | connection_id |
| --- | --- | --- | --- |
| `create` app | allow | — | 0 |
| `review` approve | allow | — | 0 |
| `create` connection | allow | — | 2 |
| `tools_list` | allow | `tools/list` | 2 |
| `mcp_tool_call`（或等价 tools/call 审计） | allow | `remote_env_info` / `tools/call` | 2 |

## 安全清理

- 临时 dashboard `users.access_token`（user id=1）e2e 后已置 `NULL`
- setup token / agent api key / connection_key 仅本机与生产 `/tmp/dpa-mcp-e2e*`，**未写入仓库**
- 生产保留 e2e 用 bridge client / tunnel app（id=2）/ connection（id=2）便于复查；可按需归档/撤销
- 本机 `dpa run` 已停止；假 MCP 进程可按需保留或杀掉

## 与既有覆盖关系

- HTTP Tunnel 生产 e2e：见 `docs/p2-tunnel-e2e-evidence-2026-07-17.md`
- 单元：`go test ./service -run TunnelMCP`（策略、计费拒绝、转发与审计）
- 本证据补齐 **生产 mcp_code tools/list + tools/call** 闭环

## 与 product-gap 对应

- P2-1 端到端：HTTP Tunnel + **mcp_code** 生产调用均已有证据
- 退出标准「云端 Agent → 本机服务」：云端 MCP 入口 → dpa `mcp_proxy.*` → 本机 19090 假 MCP
