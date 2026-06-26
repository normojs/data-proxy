# Tunnel Apps Architecture

Data Proxy 的 Tunnel Apps 分为两类：

- `mcp_code`：面向网页端 AI 应用的远程代码 MCP 隧道。Data Proxy 负责授权、审批、工具过滤、审计和扣费，本地执行由用户机器上的 Bridge/Agent 完成。
- `http_tunnel` / `tcp_tunnel`：面向本地服务的通用流量隧道。Data Proxy 负责入口、鉴权、路由、限速、流量统计、审计和扣费，不解析业务协议。

这个设计对标 Cloudflare Remote MCP / MCP Portals 的控制面思路：集中授权、统一入口、工具过滤、观测日志和策略治理。Data Proxy 不把服务端做成本地代码执行器，执行边界仍在用户本地 Agent。

参考资料：

- Cloudflare MCP Portals: https://developers.cloudflare.com/cloudflare-one/access-controls/ai-controls/mcp-portals/
- Cloudflare Agents MCP: https://developers.cloudflare.com/agents/model-context-protocol/

## 架构边界

| 层级 | Data Proxy 负责 | 本地 Agent/Bridge 负责 |
| --- | --- | --- |
| 控制面 | Tunnel App 申请、审批、状态、策略、路由、计费配置、审计 | 注册本机能力、维持在线会话、暴露本地目标 |
| `mcp_code` 数据面 | MCP 鉴权、工具列表过滤、`tools/call` 策略检查、调用审计、扣费 | 文件读写、命令执行、工作区沙箱、最终二次授权 |
| `http_tunnel` / `tcp_tunnel` 数据面 | 公网入口、token/auth、限速、流量统计、连接审计 | 转发到 `127.0.0.1:port` 或允许的局域网目标 |

通用 HTTP/TCP 隧道是 opaque forwarding。它只能按连接、路径、Host、字节数和时长治理，不能可靠控制 MCP 的 `write` / `exec`。需要控制工具权限时必须走 `mcp_code` L7 网关。

## 当前完成状态

已完成最小控制面与 MCP 网关 MVP：

- 新增数据表模型：`tunnel_apps`、`tunnel_connections`、`tunnel_sessions`、`tunnel_routes`、`tunnel_audit_logs`。
- 新增 API：
  - `POST /api/bridge/agent-setup`
  - `GET /api/tunnel/apps`
  - `POST /api/tunnel/apps`
  - `GET /api/tunnel/apps/:id`
  - `GET /api/tunnel/apps/:id/connections`
  - `POST /api/tunnel/apps/:id/connections`
  - `DELETE /api/tunnel/apps/:id/connections/:connection_id`
  - `POST /api/tunnel/apps/:id/agent-setup`
  - `GET /api/tunnel/apps/:id/sessions`
  - `GET /api/tunnel/apps/:id/audit-logs`
  - `GET /api/tunnel/admin/apps`
  - `GET /api/tunnel/admin/apps/:id`
  - `PATCH /api/tunnel/admin/apps/:id`
- 新增 MCP 网关入口，每条用户连接都有独立 connection key：
  - `GET /t/:connection_key/tunnel/mcp/:slug`
  - `GET /t/:connection_key/tunnel/mcp/:slug/v1`
  - `POST /t/:connection_key/tunnel/mcp/:slug`
  - `POST /t/:connection_key/tunnel/mcp/:slug/`
  - `POST /t/:connection_key/tunnel/mcp/:slug/v1`
  - `DELETE /t/:connection_key/tunnel/mcp/:slug`
  - `DELETE /t/:connection_key/tunnel/mcp/:slug/v1`
  - `POST /t/:connection_key/tunnel/mcp/:slug/message`
  - `POST /t/:connection_key/tunnel/mcp/:slug/v1/message`
- 新增 HTTP Tunnel MVP 公网入口：
  - `/t/:connection_key/tunnel/http/:slug`
  - `/t/:connection_key/tunnel/http/:slug/*proxy_path`
- 管理端 `MCP -> Tunnel Apps` 支持查看、筛选、审批、拒绝和禁用。
- 用户端 `MCP -> My Tunnel Apps` 支持生成/轮换本地 Bridge Agent 设置，预留 offline Bridge Client，并为自己的 Bridge Client 申请 `mcp_code` / `http_tunnel` / `tcp_tunnel` Tunnel App、查看审批状态、审批备注和 public slug。
- 用户端 `MCP -> Tunnel Connections` 支持查看已审批的 `mcp_code` / `http_tunnel` / `tcp_tunnel` 应用、创建专属 connection key、复制 endpoint、撤销连接、查看最近 request id，并从连接行直接打开该 connection 的 Tunnel audit log。
- 用户端 `MCP -> Tunnel Sessions` 支持按已审批 `mcp_code` app、connection、在线状态和关键字查看网关 MCP session，字段包含 gateway session id、connection/key prefix、Bridge client id、client version、client IP、user agent、入/出站字节、连接/最近活跃/断开时间和关闭原因。
- 创建 connection key 必须基于已审批的 Tunnel App；未审批、拒绝、禁用或归档的 App 不能创建新连接。
- 创建 connection key 时可配置连接级基础限流：`max_requests_per_minute`、`max_bytes_in_per_minute`、`max_bytes_out_per_minute`。默认不限制；触发后拒绝当前请求并写入 `rate_limit` 审计和 record-only billing event。当前实现为单节点内存窗口，生产多节点需要后续迁移到 Redis/共享状态。
- 审批通过 `mcp_code` 时，会把 Tunnel App 的权限模式同步到对应 Bridge Client 的 `bridgepolicy.Policy`。
- 审批前会校验 `bridge_client_id` 必须属于申请用户，避免跨用户误绑。
- 用户可为已申请的 Tunnel App 创建/吊销多条连接。连接 key 仅创建时返回一次，数据库只保存 `key_prefix` 和 `key_hash`。
- 每条 Tunnel Connection 可生成/轮换专属本地 agent API token。token id 绑定在 `tunnel_connections.agent_token_id`，完整 `api_key` 仅创建或轮换时返回一次；撤销 connection 会禁用绑定的 agent token。
- 用户端 `MCP -> Tunnel Connections` 的连接行提供 `Agent Setup`，可复制 Bridge WebSocket URL、Bridge register message、环境变量和 agent 配置 JSON；`http_tunnel` / `tcp_tunnel` 还会给出匹配当前 app 目标的 `dpa tunnel route add ...` 本地路由命令。
- `mcp_code` 网关已支持 `initialize`、`ping`、`notifications/*`、`tools/list`、`tools/call`、`resources/list`、`resources/read`、`resources/templates/list`、`prompts/list`、`prompts/get`：通过 Bridge 调 user mcp，按 app owner 做 user-scoped session 选择，按 app 与 connection 中更保守的权限模式过滤工具，调用前授权，写入带 connection id 的 Tunnel audit log。`resources/*` 和 `prompts/*` 走只读 JSON-RPC 透传，并可按 `policy.mcp_gateway` 对资源 URI 前缀、resource template URI template 和 prompt 名称做 allow/deny；Bridge agent 使用 `mcp_proxy.rpc` 转给本机 user mcp。
- `mcp_code` 网关已支持面向客户端的最小 Streamable HTTP/SSE 兼容：`initialize` 返回 `Mcp-Session-Id`，后续请求可携带该 header；`Accept: text/event-stream` 的 POST 会返回单次 SSE `message` 事件；旧 SSE transport 可用 GET endpoint 取得 `endpoint` event，再向 `message?session_id=...` POST JSON-RPC；客户端可用 DELETE 主动关闭 session。网关 session 元信息同步写入 `tunnel_sessions`，节点重启或普通 Streamable HTTP 请求落到其他节点时可从 DB 重建运行态；旧 SSE transport 的实时 channel 仍绑定在持有长连接的节点，开启 Redis 后 `message` 请求可通过 `tunnel:mcp:sse:<session_id>` pub/sub 投递到持有 SSE channel 的节点。未配置 Redis 时保持单节点行为，多节点部署需配置连接粘性。
- `tunnel_audit_logs.session_id` 记录的是 Data Proxy MCP gateway session id；本地 Bridge session id 保存在 audit `metadata.bridge_session_id`。这样同一个网页 AI 连接的 `initialize`、`tools/list`、`tools/call` 和关闭事件可以按 gateway session 聚合，同时仍能追溯实际转发到哪个 Bridge session。
- `http_tunnel` MVP 已支持普通 HTTP 方法通过 Bridge 转发到本地 `target_host:target_port/target_path`，返回状态码、响应头和响应体，记录 `proxy_request` 审计。请求体默认最大 8MB，响应体默认最大 2MB 或受 Bridge policy `max_result_bytes` 限制。数据面会读取 `route.auth_mode`、`route.auth_token`、`route.host`、`route.path_prefix`、`route.max_request_bytes`、`route.max_response_bytes`，支持 connection key 私有访问、额外 bearer token、Host/Path 前缀限制、单 app 请求/响应大小限制和公开只读模式。当前版本已支持流式请求体、流式响应/SSE flush、WebSocket 代理和 WebSocket 请求/响应限额；暂不支持 CONNECT、登录态访问和分布式共享带宽限速。
- `tcp_tunnel` MVP 已支持单机 TCP-over-WebSocket 入口：`GET /t/:connection_key/tunnel/tcp/:slug` 需要 WebSocket upgrade，Data Proxy 按 connection key 找到已审批 TCP App，通过 Bridge `tcp_tunnel.connect` 让本地 `dpa` 拨 `target_host:target_port`，然后双向转发二进制帧，记录 `proxy_request` 审计、字节数、耗时、request id 和 `billing_events`。WebSocket ping/pong 只作为传输层心跳，不会转成 TCP 业务字节。当前 MVP 不是独立公网 TCP 端口映射；raw TCP listener、端口池和连接复用仍是后续任务。

尚未完成的数据面增强：

- `mcp_code` 的旧 SSE 长连接在未配置 Redis 时仍需要集群粘性、更多 raw response schema 兼容样本，以及 Redis 级共享限流状态。
- HTTP Tunnel 的登录态访问、CONNECT 支持、分布式共享带宽限速，以及更长时间的 WebSocket/SSE/大文件压力测试。
- TCP raw listener、端口池、连接复用和更细路由策略。
- Tunnel 余额预扣、欠费停用和更细粒度风控。
- 本地 user mcp 安装包/agent 程序自动下载、自动注册、agent 版本建议和更细粒度的 stdio MCP 子进程健康检查。

## 权限模式

`mcp_code` 使用分级权限。Data Proxy 服务端先按 Bridge policy 做第一层过滤，本地 Agent 仍必须做最终执行保护。

| 模式 | 服务端允许的能力 | 说明 |
| --- | --- | --- |
| `read_only` | `remote_read`、`remote_tree`、`remote_glob`、`remote_grep`、`remote_env_info`、`remote_project_info`、`remote_get_related_files`、`remote_git_status`、`remote_git_diff`、`remote_git_log` | 只读代码浏览、搜索和 Git 状态。 |
| `write` | `read_only` + `remote_write`、`remote_edit` | 允许文件写入和编辑，不开放命令执行。 |
| `exec_safe` | `write` + `remote_run_tests` | 允许较保守的测试执行。服务端不解析命令内容。 |
| `exec_trusted` | `exec_safe` + `remote_exec`、`remote_shell_open`、`remote_shell_eval`、`remote_install_package` | 高危可信模式，只应给明确授权用户和受控工作区；Go agent 现阶段已支持一次性 `remote_exec`、无 PTY 基础持久 shell，以及固定 argv 的 `remote_install_package`，本机仍需额外开启 `policy.exec.allow_arbitrary=true`，安装依赖还需 `policy.allow_write=true`。 |

`http_tunnel` / `tcp_tunnel` 固定为 `traffic` 权限，不能申请代码工具权限。

## API 约定

用户申请示例：

```json
{
  "name": "MacBook workspace",
  "app_type": "mcp_code",
  "permission_mode": "write",
  "bridge_client_id": "macbook-local-agent",
  "target_path": "/mcp",
  "policy": {
    "max_result_bytes": 1048576,
    "max_scan_file_bytes": 262144,
    "max_results": 50,
    "tree_depth": 4,
    "walk_depth": 6
  }
}
```

创建 MCP 连接示例：

```json
{
  "name": "Desktop Codex",
  "permission_mode": "read_only",
  "expires_at": 0,
  "config": {
    "rate_limit": {
      "max_requests_per_minute": 120,
      "max_bytes_in_per_minute": 10485760,
      "max_bytes_out_per_minute": 52428800
    }
  }
}
```

响应中的 `connection_key` 只返回一次，推荐 MCP 入口路径为：

```text
/t/<connection_key>/tunnel/mcp/<public_slug>
```

控制台会展示完整 endpoint：

```text
https://<data-proxy-host>/t/<connection_key>/tunnel/mcp/<public_slug>
```

后续列表只显示 `key_prefix`，用于识别和撤销连接；完整 `connection_key` 不会再次返回。

Data Proxy 仍会校验 API token 用户、Tunnel App owner、Connection owner 和 Bridge Client owner 必须一致。`connection_key` 用于单条连接的吊销、审计和限流，不替代 API token。

首次生成本地 Bridge Agent 设置示例：

```json
{
  "client_name": "Desktop Bridge Agent",
  "platform": "darwin",
  "workspace": "/workspace/project",
  "version": "1.0.0"
}
```

如需为已有 Bridge Client 轮换本地 agent API key：

```json
{
  "client_id": "bridge-local-agent",
  "rotate": true
}
```

`POST /api/bridge/agent-setup` 会创建或复用一个属于当前用户的 offline Bridge Client，并把专属 API token id 绑定到 `bridge_clients.token_id`。完整 `api_key` 仅创建或轮换时返回一次；重复打开只返回 `token_masked_key`。本地 agent 用该 `api_key` 连接 `bridge_ws_url`，首条消息发送响应里的 `register`。

生成或轮换本地 agent 设置示例：

```json
{
  "connection_id": 123,
  "rotate": false,
  "client_name": "Desktop Agent"
}
```

响应会返回：

- `api_key`：本地 agent 连接 `/bridge/ws` 使用的 API token，仅创建或轮换时返回一次。
- `bridge_ws_url`：本地 agent 的 WebSocket 连接地址，例如 `wss://<data-proxy-host>/bridge/ws`。
- `client_id`：必须等于 Tunnel App 已审批绑定的 `bridge_client_id`。
- `register`：Bridge websocket 的首条 register message。
- `mcp_url`：Remote MCP 客户端使用的 endpoint 模板，仍需把 `<connection_key>` 替换为创建 connection 时返回的完整 key。

`connection_key` 和 agent `api_key` 是两套凭证：

- `connection_key` 放在 `/t/<connection_key>/tunnel/mcp/<public_slug>` 路径中，用于区分、吊销和审计网页端 AI 应用连接。
- agent `api_key` 放在 `Authorization: Bearer sk-...` 中，用于本地 agent 注册 Bridge session。

连接级审计查询示例：

```text
GET /api/tunnel/apps/<app_id>/audit-logs?connection_id=<connection_id>&page_size=20
```

返回事件包含 `action`、`decision`、`request_id`、`tool_name`、`connection_key_prefix`、字节数、耗时和 metadata。控制台会在 `MCP -> Tunnel Connections` 的连接行提供审计入口，便于按连接排查 `tools/list`、`tools/call`、`resources/read`、`prompts/get`、策略拒绝和撤销事件。

MCP raw forward 过滤策略写在 Tunnel App `policy` 下：

```json
{
  "mcp_gateway": {
    "allowed_resource_uri_prefixes": ["file:///workspace/"],
    "denied_resource_uri_prefixes": ["file:///workspace/secrets"],
    "allowed_prompt_names": ["summarize", "explain_error"],
    "denied_prompt_names": ["dangerous_local_action"],
    "allowed_prompt_prefixes": ["safe_"],
    "denied_prompt_prefixes": ["admin_"]
  }
}
```

- `resources/list` 会过滤返回列表，只展示允许的资源 URI。
- `resources/read` 在转发前按 `uri` 判断，拒绝时写 `policy_deny` 审计，不把请求发到 user mcp。
- `prompts/list` 会过滤返回列表，只展示允许的 prompt。
- `prompts/get` 在转发前按 `name` 判断，拒绝时写 `policy_deny` 审计。
- 未配置 allow/deny 时保持兼容，默认允许只读 raw forward。

HTTP 隧道申请示例：

```json
{
  "name": "Vite preview",
  "app_type": "http_tunnel",
  "bridge_client_id": "macbook-local-agent",
  "target_host": "127.0.0.1",
  "target_port": 5173,
  "target_path": "/",
  "route": {
    "auth_mode": "private",
    "host": "dp.example.com",
    "path_prefix": "/"
  }
}
```

HTTP Tunnel route 配置字段：

- `auth_mode`: `private`、`token` 或 `public`。当前入口仍然包含 `connection_key`，所以 `public` 表示不额外要求 bearer token，不是无连接 key 的裸公网入口。
- `auth_token`: `auth_mode=token` 时必填。访问方可使用 `Authorization: Bearer <auth_token>` 或 `X-Tunnel-Token: <auth_token>`。
- `host`: 可选，限制请求 Host；配置不带端口时会同时允许同 host 的任意端口。
- `path_prefix`: 可选，限制公网入口下游 path 前缀，例如 `/api` 允许 `/api` 和 `/api/users`，拒绝 `/admin`。
- `max_request_bytes`: 可选，限制单个请求体大小；超过时返回 413，并写入 `request_too_large` 审计。
- `max_response_bytes`: 可选，限制单个响应体大小；实际下发给 Bridge 的值不会超过 Bridge policy `max_result_bytes` 或默认 2MB。

HTTP Tunnel 访问示例：

```bash
curl https://<data-proxy-host>/t/<connection_key>/tunnel/http/<public_slug>/health
```

默认安全边界：

- Data Proxy 端使用 connection key 找到已审批的 `http_tunnel` app 和 active connection。
- Data Proxy 端根据 Tunnel App `route` 执行 auth mode、Host 和 Path prefix 检查；拒绝事件会写入 `tunnel_audit_logs`，reason 包括 `auth_token_required`、`auth_token_invalid`、`route_forbidden` 和 `route_config_invalid`。
- Data Proxy 端检查 Bridge policy：`allowed_tools` 需要包含 `http_tunnel` 或 `http_tunnel.request`，`http_allowed_targets` 为空时只允许 loopback 目标；`http_denied_targets` 和 `http_denied_ports` 会优先拒绝敏感目标。
- 本地 `tools/bridge_client_daemon.mjs` 也默认只允许 loopback HTTP 目标；要访问局域网目标，必须同时配置 Bridge policy `http_allowed_targets` 并在 daemon 启动时加入 `--allow-non-loopback-http`。

TCP 隧道申请示例：

```json
{
  "name": "Local SSH",
  "app_type": "tcp_tunnel",
  "bridge_client_id": "macbook-local-agent",
  "target_host": "127.0.0.1",
  "target_port": 22,
  "route": {
    "max_request_bytes": 104857600,
    "max_response_bytes": 104857600
  }
}
```

TCP Tunnel 当前入口是 WebSocket：

```text
wss://<data-proxy-host>/t/<connection_key>/tunnel/tcp/<public_slug>
```

默认安全边界：

- Data Proxy 端使用 connection key 找到已审批的 `tcp_tunnel` app 和 active connection。
- 审批 `tcp_tunnel` 时会把 `tcp_tunnel` tool family 同步到对应 Bridge Client policy；数据面要求本地 `dpa` session 声明 `tcp_tunnel` capability。
- Data Proxy 端不接受访问方动态传入目标地址，目标只来自已审批 Tunnel App 的 `target_host:target_port`。
- 本地 `dpa` 只允许连接本机配置 `tcp_routes` 中列出的 TCP 目标；默认只允许 loopback TCP 目标。如需访问局域网 TCP 目标，用户本机配置必须同时列出该 `tcp_routes` 目标并显式开启 `policy.allow_non_loopback_tcp=true`。

## 计费建议

当前实现状态：

- `mcp_code`、`http_tunnel` 和 `tcp_tunnel` 的数据面审计会同步写入 `billing_events`，来源分别是 `tunnel_mcp`、`tunnel_http` 和 `tunnel_tcp`。
- 默认只写 `audit` 类型，`amount_quota=0`，不会直接扣用户余额；它们用于账单可见性、价格策略验证和回填样本积累。
- 当 Tunnel App 的 `billing.settlement.enabled=true` 时，数据面审计会额外生成一条幂等的 `debit` 账本事件，`source_id` 与审计事件相同，例如 `audit:123`，`phase=settlement`；首次创建该事件时会在同一事务内扣减 Tunnel App owner 的用户余额。
- 默认不扣 connection 的 agent token 额度；agent token 是本地 `dpa` 连接凭证，默认 unlimited，只作为审计关联。
- 默认是后付费结算：已完成的 Tunnel 调用会被扣费，用户余额不足时允许变为负数。
- 如需在转发前拒绝欠费用户，可在 `billing.settlement` 中启用 `require_positive_balance=true`。开启后 Data Proxy 会在 MCP/HTTP/TCP 数据面转发前检查 Tunnel App owner 的余额；余额小于等于 0 时拒绝请求，写入 `billing_deny` 审计，`reason=billing_insufficient`，HTTP 返回 402，MCP 返回 invalid request，TCP WebSocket 会关闭连接。该检查只在 `enabled=true` 且实际扣余额时生效，`ledger_only=true` 或 `deduct_balance=false` 不会触发拦截。
- 如需更严格的单机预检查，可额外启用 `require_sufficient_balance=true`。开启后 Data Proxy 会按当前动作能预估到的最低扣费做转发前拦截：`mcp_code` 的 `tools/call` 使用 `quota_per_call` / `min_quota`，HTTP/TCP 使用 `quota_per_request`、已知入站字节和 `min_quota`；无法在转发前确定的响应字节、持续时间和并发竞态仍在完成后结算。
- 精确预扣费、并发强限额和欠费自动停用仍属于后续风控任务。
- 如需先观察账本而不扣余额，可配置 `ledger_only=true` 或 `deduct_balance=false`。
- `mcp_code` 成功的 `tools/call` 按次数扣费，使用 `quota_per_call`；`tools/list`、`resources/*`、`prompts/*`、策略拒绝和失败默认只记录审计。
- `http_tunnel` / `tcp_tunnel` 成功的 `proxy_request` 可按 `quota_per_request`、`quota_per_mib_in`、`quota_per_mib_out` 和 `quota_per_second` 组合生成扣费账本，事件元数据保留 `bytes_in`、`bytes_out`、`duration_ms`、`status_code` 或 `target` 等字段。

`billing` 配置示例：

```json
{
  "rate_limit": {
    "max_requests_per_minute": 60,
    "max_bytes_in_per_minute": 10485760,
    "max_bytes_out_per_minute": 10485760
  },
  "settlement": {
    "enabled": true,
    "price_unit": "per_call",
    "quota_per_call": 25,
    "min_quota": 1,
    "deduct_balance": true,
    "require_positive_balance": true,
    "require_sufficient_balance": true
  }
}
```

HTTP Tunnel 可以使用：

```json
{
  "settlement": {
    "enabled": true,
    "price_unit": "request_traffic",
    "quota_per_request": 1,
    "quota_per_mib_in": 2,
    "quota_per_mib_out": 3,
    "quota_per_second": 1,
    "min_quota": 1,
    "deduct_balance": true,
    "require_positive_balance": true,
    "require_sufficient_balance": true
  }
}
```

`mcp_code` 建议按次数 + 可选工具风险系数扣费：

- `tools/list` 不扣费或低价。
- 只读工具按次数扣费。
- 写入和执行工具使用更高倍率。
- 失败、超时和策略拒绝应进入审计，是否扣费由后续 billing policy 决定。

`http_tunnel` / `tcp_tunnel` 建议按连接时长 + 流量扣费：

- 基础包：每月包含一定在线分钟数和流量。
- 超额：按 GB 或按 100MB 阶梯扣费。
- 可叠加最大连接数、最大带宽、最长会话时长限制。

当前基础风控：

- 连接级 `config.rate_limit.max_requests_per_minute` 控制单条 connection 的每分钟请求数。
- `config.rate_limit.max_bytes_in_per_minute` 控制单条 connection 每分钟入站字节数。
- `config.rate_limit.max_bytes_out_per_minute` 控制单条 connection 每分钟出站字节数。
- Tunnel App 的 `billing.rate_limit` 可作为 app 级默认上限，connection 级配置会和 app 级配置取更严格的正数值。
- `mcp_code` 的 `tools/list`、`tools/call`、`resources/*`、`prompts/*` 都计入请求数；`tools/call` 和 raw forward 的结果计入出站字节。
- `http_tunnel` 在进入 Bridge 前检查请求频率/入站字节，响应返回后检查出站字节。出站超限不会把过量响应继续下发给访问方。
- `billing.settlement` 未开启时，`billing.rate_limit` 只做限流，不会触发扣费账本。

## 开发顺序

1. **控制面闭环**
   - 完成 Tunnel App 申请、审批、审计、Bridge policy 同步和管理端列表。
   - 补用户端申请 UI、详情页和 agent token 下发。

2. **MCP Code Tunnel MVP**
   - 新增 `/t/:connection_key/tunnel/mcp/:slug` Remote MCP 入口。
   - 为每个用户/app 创建可吊销的连接 key，并在审计日志中关联 `connection_id`。
   - 复用现有 Bridge session，转发到本地 Agent。
   - 实现 `tools/list` 过滤、`tools/call` 策略拦截，并透传 `resources/*`、`prompts/*`。
   - 写入 `tunnel_audit_logs`，同时关联 Bridge audit logs。
   - 协议层优先评估 `modelcontextprotocol/go-sdk` 和 `mark3labs/mcp-go`；Data Proxy 自己保留 `pkg/mcpgateway` 作为策略、审计、快照和路由内核，不把多租户业务绑定到某个 SDK。
   - 状态：MVP 已完成；session 已有 DB 可观测状态，普通 Streamable HTTP 请求可按 `Mcp-Session-Id` 重建网关运行态；旧 SSE message 在 Redis 可用时可跨节点投递，未配置 Redis 时仍需连接粘性；剩余细粒度 resource/prompt/tool 展示策略和更完整协议边缘兼容。

3. **HTTP Tunnel MVP**
   - 已新增 connection key 公网入口：`/t/:connection_key/tunnel/http/:slug/*proxy_path`。
   - 已实现 HTTP reverse proxy over Bridge，记录字节数、状态码和耗时。
   - 已实现 host/path route resolver、bearer token、connection key 私有访问和公开只读 auth mode。
   - 已实现流式请求体、流式响应/SSE flush、WebSocket 代理和 WebSocket 请求/响应限额。
   - 后续支持登录态访问、CONNECT、分布式共享带宽限速和更完整的长连接压测。

4. **TCP Tunnel MVP**
   - 已实现单机 TCP-over-WebSocket 数据面入口 `/t/:connection_key/tunnel/tcp/:slug`。
   - 已实现 Bridge `tcp_tunnel.connect`、本地 `dpa` TCP 拨号、双向二进制帧转发、connection key 鉴权、Bridge policy/capability 检查、字节/耗时审计和 billing event 记录。
   - WebSocket ping/pong 作为传输层心跳处理，不写入本地 TCP 目标；公网客户端写失败会记录 `reason=client_write_failed` 并取消 pending Bridge stream；`dpa` 在任一 TCP pipe 退出后会关闭 stream input queue，避免本地 goroutine 悬挂。
   - 后续实现 raw TCP listener、端口池、连接复用、主动 backend probe UI 和更细 route/security policy。

5. **计费与风控**
   - `mcp_code` 已支持成功 `tools/call` 按次数即时结算，可选余额不足转发前拦截。
   - `http_tunnel` / `tcp_tunnel` 已支持按 request、bytes、duration 即时结算，可选余额不足转发前拦截。
   - 已添加连接级基础限流；后续继续添加预扣费、欠费自动停用、高错误率自动暂停、超额自动断开、可疑端口和内网目标 deny list。

6. **Cloudflare Remote MCP 对标增强**
   - OAuth/OIDC 授权入口。
   - Tool/prompt/resource 级别的展示过滤。
   - 工具别名和 namespace 管理。
   - 网关日志、DLP/redaction、逐用户授权记录。

## 部署注意

Tunnel 数据面后续会依赖稳定的长连接和流式转发，生产 Nginx 需要继续保留：

```nginx
proxy_http_version 1.1;
proxy_buffering off;
proxy_cache off;
chunked_transfer_encoding on;
proxy_read_timeout 3600s;
proxy_send_timeout 3600s;
```

如果启用 SeaweedFS 请求捕获数据湖，仍需保留 `docker-compose.capture-storage.yml` 中的 volume 映射，Tunnel 相关审计和诊断包才能和 request trace 一起沉淀。
