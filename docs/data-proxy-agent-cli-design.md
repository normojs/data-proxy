# Data Proxy Agent CLI Design

## 目标

`dpa` 是 Data Proxy Tunnel 的本地客户端命令。它应该像 `cloudflared`
一样作为跨平台命令行工具运行在用户自己的电脑、开发机、内网服务器或容器里，同时承担两类职责：

- 通用隧道客户端：把本地 HTTP/WebSocket/SSE 服务通过 Data Proxy 暴露给外部访问方。
- MCP 桥接客户端：把网页端 AI 的 MCP 请求转发给用户本机的 `user mcp`，并在本地执行最后一层安全策略。

它不是模型服务，也不是 Data Proxy 服务端。它是一个主动连出公网 Data Proxy 的本地 daemon。

## 参考 cloudflared 的产品形态

Cloudflare Tunnel 的关键设计值得借鉴：

- 单独安装一个轻量 daemon，负责从用户基础设施主动连接云端。
- 控制台创建 tunnel 后，直接给用户一条安装/运行命令。
- 支持 Linux、macOS、Windows 和 Docker。
- 支持系统服务安装，机器重启后自动恢复连接。
- 支持配置文件管理多个本地服务和 ingress 规则。
- 启动时做连通性预检查，并给出可操作的错误说明。

Data Proxy Agent 需要在这些能力上增加 MCP 场景特有的权限、审计和本地执行边界。

## 定位

```text
Web AI / MCP Client
        |
        | Remote MCP / HTTP Tunnel endpoint
        v
Data Proxy Server
        |
        | outbound WebSocket bridge
        v
dpa / data-proxy-agent
        |
        +--> local HTTP service: http://127.0.0.1:3000
        +--> local MCP server:  http://127.0.0.1:30837/mcp
        +--> optional local file/code tools, guarded by local policy
```

组件职责：

| 组件 | 职责 |
| --- | --- |
| Data Proxy Server | 公网入口、用户鉴权、Tunnel App 审批、连接 key、限流、审计、计费 |
| dpa / data-proxy-agent | 主动连出、注册本地能力、转发 HTTP/MCP、执行本地策略、上报健康状态 |
| user mcp | 真正操作用户本地文件、命令、浏览器或其他资源 |
| Web AI / MCP Client | 使用 Data Proxy 暴露的 Remote MCP endpoint 或 HTTP endpoint |

## 技术选型

正式版本建议用 Go 开发单二进制 CLI：

- 和 Data Proxy 主项目语言一致，能复用协议结构、签名、审计、策略代码。
- 跨平台构建简单，适合发布 Linux/macOS/Windows 的 amd64/arm64 包。
- 不要求用户安装 Node.js。
- 可以稳定作为 systemd、launchd、Windows Service 运行。

当前 `tools/bridge_client_daemon.mjs` 保留为原型和 smoke test 工具，后续逐步把能力迁移到 Go CLI。

推荐 binary 名称：

- 主命令：`dpa`
- 兼容命令：`data-proxy-agent`
- 包名、服务名、配置目录和镜像名继续使用 `data-proxy-agent`，避免已有部署升级时断开。

## 命令设计

### 常用命令

```bash
dpa version
dpa enroll --server https://<data-proxy-host> --setup-token <one-time-token>
dpa enroll --server https://<data-proxy-host> --access-token <dashboard-access-token> --user-id <id>
dpa run
dpa status
dpa status --json
dpa doctor
dpa doctor --json
dpa doctor --check-update
dpa report --output ./agent-diagnostic.zip
dpa report --check-update --output ./agent-diagnostic.zip
```

当前 `enroll` 支持两种绑定方式：

- 推荐：控制台调用 `POST /api/bridge/agent-setup-tokens` 生成一次性 setup token，本地 agent 调用匿名接口 `POST /api/bridge/agent-setup/consume` 换取 bridge client 和 agent token。
- 兼容：本地 agent 直接调用 `/api/bridge/agent-setup`，使用控制台 access token 和 `New-Api-User` 对应的 user id 完成注册。

setup token 只保存 hash，默认 10 分钟过期，只能消费一次。控制台的一键命令可以收敛为：

```bash
dpa enroll --server https://<data-proxy-host> --setup-token <one-time-token>
```

用户在控制台复制的一键命令应该接近：

```bash
curl -fsSL https://<data-proxy-host>/agent/install.sh | sh
dpa enroll --server https://<data-proxy-host> --setup-token <one-time-token>
dpa service install
dpa service start
```

当前仓库已提供 GitHub Release 安装脚本，控制台 `/agent/install.sh`
后续可以直接代理或渲染同等脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/normojs/data-proxy/main/scripts/install-data-proxy-agent.sh | sh
dpa update --dry-run
dpa update
```

如果服务器或对象存储已经镜像了 release asset 和 manifest，安装脚本也可以直接走
manifest，不依赖 GitHub Release API：

```bash
curl -fsSL https://<data-proxy-host>/agent/install-data-proxy-agent.sh | \
  DATA_PROXY_AGENT_MANIFEST_URL=https://<data-proxy-host>/agent/releases/data-proxy-agent-manifest.json sh
```

正式 tag 发布时，`Data Proxy Agent` workflow 会同时上传
`data-proxy-agent-manifest.json`。这个 manifest 是机器可读的更新清单，
包含各平台 tar/zip 资产 URL 和 sha256，可用于控制台代理、自建下载源或灰度发布：

```bash
dpa update --manifest-url https://<data-proxy-host>/agent/releases/data-proxy-agent-manifest.json --dry-run
dpa update --manifest-url https://<data-proxy-host>/agent/releases/data-proxy-agent-manifest.json
```

默认 GitHub Release 更新路径仍然可用；manifest 路径适合服务器先缓存 release
资产，再由国内用户从 Data Proxy 域名下载。

### 配置命令

```bash
dpa config path
dpa config show
dpa config validate
dpa config export --redact
```

### 隧道命令

```bash
dpa tunnel list
dpa tunnel run <name>
dpa tunnel route add http <name> --url http://127.0.0.1:3000
dpa tunnel route add tcp <name> --host 127.0.0.1 --port 5432
dpa tunnel route test <name>
dpa tunnel route remove <name>
```

当前单机版已实现 HTTP/WebSocket/SSE route 和 TCP-over-WebSocket route；`route test` 可在暴露隧道前主动探测本地目标是否可连。

### MCP 命令

```bash
dpa mcp list
dpa mcp add coding --transport streamable-http --url http://127.0.0.1:30837/mcp
dpa mcp add filesystem --transport stdio --command "npx -y @modelcontextprotocol/server-filesystem /Users/me/project"
dpa mcp test coding
dpa mcp test filesystem
dpa mcp remove coding
```

`stdio` MCP 由本机 agent 配置的 command 启动和复用。Data Proxy 服务端只能通过 MCP Server 名称引用本地配置，不能在桥接请求里下发任意 command。

### 服务命令

```bash
dpa service install
dpa service uninstall
dpa service start
dpa service stop
dpa service restart
dpa service status
```

平台映射：

| 平台 | 服务方式 |
| --- | --- |
| Linux | systemd |
| macOS | launchd |
| Windows | Windows Service |
| Docker | foreground `run` |

### 诊断命令

```bash
dpa status --json
dpa doctor
dpa doctor --json
dpa doctor --check-update
dpa logs path
dpa logs tail --lines 100
dpa logs tail --follow
dpa self-test
dpa report --output ./agent-diagnostic.zip
dpa report --check-update --output ./agent-diagnostic.zip
```

`status --json` 默认不做网络请求，只输出脱敏的本地配置摘要，包括 client id、
Bridge URL、token 是否配置、token 来源、能力列表、MCP/HTTP/TCP route 数量和静态 route 列表。
加上 `--health` 后会额外探测本地 HTTP/TCP/MCP 目标，把结果放进 `local_health`，
适合安装脚本、控制台采集和用户排障第一步使用。

`doctor --json` 输出结构化诊断结果，包含配置是否加载、配置校验结果、DNS、
Bridge token 握手、本地 workspace、本地审计路径、HTTP/MCP route、系统服务状态和可选更新检查。
它不输出 agent token，适合控制台或脚本采集后做自动排障。

`doctor --check-update` 和 `report --check-update` 会查询 GitHub Release 或自定义
manifest 的 release metadata，只解析当前平台资产名称和版本，不下载或安装包。发现新
版本时记录为 `warn`，不让本地健康检查失败；离线环境可以不传该参数。

当前 `report` 会生成脱敏 zip，包含版本/平台、配置路径、脱敏配置、`status.json`、`status.txt`、
`local_health.json`、校验结果、可选网络检查结果、远端 Bridge token 握手校验结果、系统服务状态诊断，以及 `doctor` 同源的本地健康检查结果。
它不采集原始用户请求、响应、MCP 工具参数或本地文件内容。
`logs path/tail` 读取 `logging.local_audit_jsonl`，`logs tail --follow` 可持续打印新增 JSONL 行。该本地审计只记录 bridge tool 调用的 request id、tool name、成功/失败、耗时、结果大小、错误码和少量 allowlist metadata，不写原始参数、响应正文或本地文件内容。

`doctor` 和 `report` 需要检查：

- 能否解析 Data Proxy 域名。
- 能否连接 `wss://.../bridge/ws`。
- API token 是否有效。
- 本地 MCP endpoint 是否可访问。
- 本地 HTTP tunnel target 是否可访问。
- 本地策略是否允许目标 host、port、workspace 和 tool。
- 系统服务是否已安装并正在运行。
- 当前版本是否过旧。

当前 `doctor` 已覆盖配置校验、token 是否配置、`/bridge/ws` 远端 Bearer token 握手校验、Bridge DNS、Data Proxy
`/api/status`、系统服务状态、workspace、本地审计文件路径、HTTP route TCP 连通性、TCP route TCP 连通性、HTTP MCP
endpoint TCP 连通性、可选 release metadata 更新检查，以及 stdio MCP 的 shell/命令前缀和已启动进程状态检查。stdio 检查不会主动启动
MCP 进程；真正的协议握手继续使用 `dpa mcp test <name>`。stdio MCP 进程启动、退出和退出后下次调用自动重启会写入本地审计 JSONL，但不会记录完整 command、工具参数或响应正文。

其中系统服务安装/启停命令、版本更新命令、agent 在线健康上报和控制台健康摘要展示已具备基础能力；stdio MCP 已具备基础 stderr 分类，并会在本地审计和 health detail 中标记 `stderr_class`；注册成功后会启动轻量 watchdog，按 `runtime.health_interval_ms` 节奏清理已退出的缓存 stdio MCP 会话，并写入 `mcp_stdio.watchdog_reap` 本地审计事件。watchdog 只做清理和审计，不主动重启本地 MCP 进程；agent health report 现在会附带 `mcp_processes` 结构化摘要，服务端 Bridge Client health API 和控制台详情可展示 stdio MCP 的 running/exited/config_error/not_started、PID、初始化状态、stderr 分类和退出错误。更完整的历史事件聚合仍属于后续产品化阶段。

## 配置文件

默认配置路径：

| 平台 | 用户级配置 | 服务级配置 |
| --- | --- | --- |
| Linux | `~/.config/data-proxy-agent/config.yaml` | `/etc/data-proxy-agent/config.yaml` |
| macOS | `~/Library/Application Support/DataProxyAgent/config.yaml` | `/Library/Application Support/DataProxyAgent/config.yaml` |
| Windows | `%APPDATA%\DataProxyAgent\config.yaml` | `%ProgramData%\DataProxyAgent\config.yaml` |

示例：

```yaml
server:
  base_url: https://<data-proxy-host>
  bridge_ws_url: wss://<data-proxy-host>/bridge/ws

agent:
  client_id: macbook-pro-dev
  name: "MacBook Pro Dev"
  version_channel: stable
  token_ref: keychain://data-proxy-agent/macbook-pro-dev

policy:
  default_permission: read_only
  allow_write: false
  allow_non_loopback_http: false
  allow_non_loopback_mcp: false
  allow_non_loopback_tcp: false
  allowed_workspaces:
    - /Users/me/workspace
  denied_paths:
    - /Users/me/.ssh
    - /Users/me/Library/Keychains
  exec:
    enabled: false
    allow_arbitrary: false
    safe_commands:
      - git status
      - npm test

mcp_servers:
  - name: coding
    transport: streamable_http
    endpoint: http://127.0.0.1:30837/mcp
    permission: read_only

http_routes:
  - name: local-web
    target: http://127.0.0.1:3000
    allow_websocket: true
    allow_sse: true
    max_request_bytes: 8388608
    max_response_bytes: 2097152

tcp_routes:
  - name: local-ssh
    target_host: 127.0.0.1
    target_port: 22

logging:
  level: info
  local_audit_jsonl: ~/.local/share/data-proxy-agent/audit.jsonl

runtime:
  max_results: 200
  tree_depth: 3
  walk_depth: 8
  max_result_bytes: 524288
  max_scan_file_bytes: 2097152
  max_write_bytes: 1048576
```

配置原则：

- API token 和 connection key 不明文写入普通配置文件，优先存 OS Keychain、Windows Credential Manager 或权限收紧的 secret 文件。
- `config show` 默认打码敏感字段。
- `config export --redact` 用于排障和客服支持。

## 凭证模型

建议分成三类凭证：

| 凭证 | 使用方 | 作用 |
| --- | --- | --- |
| setup token | 用户控制台生成，一次性使用 | `enroll` 时换取 agent token 和 client id |
| agent token | 本地 agent 保存 | 连接 `/bridge/ws` 并注册 Bridge client |
| connection key | 外部访问方使用 | 访问 `/t/<connection_key>/tunnel/...` |

这样可以做到：

- 用户可以撤销某条外部连接，不影响本地 agent 继续在线。
- 用户可以轮换 agent token，使本地 agent 重新注册。
- setup token 泄露窗口短，适合控制台复制命令。

当前 MVP 先复用已有的 dashboard access token + user id 调用 setup API；agent token 只在 setup
创建或轮换时返回一次。CLI 会把 agent token 写入权限收紧的配置文件，`config show` 和 `report` 默认脱敏。

## 安全策略

Data Proxy Agent 必须坚持“双层授权”：

- 服务端策略：Data Proxy 根据 Tunnel App、connection、管理员审批和计费状态做第一层限制。
- 本地策略：Agent 根据本机配置做最后一层限制。

默认策略：

- HTTP tunnel 默认只允许 loopback 目标。
- MCP target 默认只允许 loopback。
- 文件工具默认只读。
- 写文件、编辑文件、执行命令默认关闭。
- 非 loopback、写入和执行都必须显式开启。

MCP 权限建议分级：

| 模式 | 含义 |
| --- | --- |
| `read_only` | 只允许读文件、搜索、列目录、只读 MCP tool |
| `write_limited` | 允许写入指定 workspace，拒绝敏感路径 |
| `exec_safe` | 只允许 allowlist 内命令，例如测试、格式化、git status |
| `full_trust` | 高风险模式，只给用户自己的可信环境使用 |

本地 Agent 的审计日志至少记录：

- request id
- tunnel app id / connection id / session id
- MCP method / tool name
- HTTP method / target URL
- bytes in / bytes out
- decision: allow / deny
- deny reason
- duration
- local policy version

## 数据面协议

第一版继续兼容现有 `/bridge/ws` 协议：

- `register`
- `heartbeat`
- `tool_call`
- `tool_result`
- `stream_chunk`
- `stream_end`
- `stream_error`

HTTP Tunnel 能力：

- 普通 HTTP 方法转发
- 流式 request body
- 流式 response body
- SSE flush
- WebSocket frame 代理
- 请求/响应大小限制
- 本地 target allow/deny

MCP Bridge 能力：

- `initialize`
- `tools/list`
- `tools/call`
- `resources/list`
- `resources/read`
- `prompts/list`
- `prompts/get`
- `notifications/*`
- 上游 MCP 错误原样包装

后续可以增加 wire protocol v2：

- 二进制 frame，减少 base64 开销。
- 多路复用，避免大文件传输阻塞 MCP 小请求。
- 更明确的 backpressure 和 cancellation。
- 本地缓存 capability snapshot，减少反复 `tools/list`。

## 控制台配合

Data Proxy 控制台需要提供：

- 创建 Local Agent。
- 显示一键安装命令。
- 轮换 agent token。
- 查看在线状态、版本、平台、IP、最近心跳。
- 查看本地暴露能力：HTTP routes、MCP servers、权限模式。
- 查看 Agent 健康检查结果。
- 从 Tunnel Connection 行复制外部 endpoint。
- 允许管理员按用户、Agent、Tunnel App 禁用。

推荐用户流程：

1. 用户在控制台点击“创建本地 Agent”。
2. 控制台生成一次性 setup token 和安装命令。
3. 用户本机执行安装命令。
4. Agent `enroll` 成功后自动注册为 Bridge Client。
5. 用户在控制台申请 MCP Code Tunnel、HTTP Tunnel 或 TCP Tunnel。
6. 管理员审批。
7. 用户创建 connection key。
8. HTTP/TCP Tunnel 用户从控制台复制 `dpa tunnel route add ...` 本地路由命令，并在运行 agent 的机器上执行；部署前可用 `dpa tunnel route test <name>` 验证本地目标连通性。
9. 外部网页 AI、HTTP 调用方或 TCP-over-WebSocket 客户端使用 connection endpoint。

## 打包与发布

当前使用 GitHub Actions 打包：

- `data-proxy-agent-<version>-linux-amd64.tar.gz`
- `data-proxy-agent-<version>-linux-arm64.tar.gz`
- `data-proxy-agent-<version>-linux-amd64.deb`
- `data-proxy-agent-<version>-linux-arm64.deb`
- `data-proxy-agent-<version>-linux-amd64.rpm`
- `data-proxy-agent-<version>-linux-arm64.rpm`
- `data-proxy-agent-<version>-darwin-amd64.tar.gz`
- `data-proxy-agent-<version>-darwin-arm64.tar.gz`
- `data-proxy-agent-<version>-darwin-amd64-notarized.tar.gz`，需要 Apple signing secrets
- `data-proxy-agent-<version>-darwin-arm64-notarized.tar.gz`，需要 Apple signing secrets
- `data-proxy-agent-<version>-windows-amd64.zip`
- `data-proxy-agent-<version>-windows-arm64.zip`
- `data-proxy-agent-<version>-windows-amd64.msi`
- `data-proxy-agent-<version>-windows-arm64.msi`
- `data-proxy-agent.rb` Homebrew formula
- per-asset `.sha256`
- checksums.txt
- `ghcr.io/normojs/data-proxy-agent:<version>`
- `ghcr.io/normojs/data-proxy-agent:sha-<short-sha>`

tag 以 `v*` push 时，`Data Proxy Agent` workflow 会把二进制压缩包、Linux deb/rpm、Windows MSI、Homebrew formula 和可选 notarized macOS 包上传到 GitHub Release，并使用 `Dockerfile.agent` 发布 linux/amd64、linux/arm64 多架构镜像到 GHCR。tag 发布默认同时更新 `latest`；手动 `workflow_dispatch` 可发布当前 ref 的 `sha-*` 镜像，也可显式选择是否更新 `latest`。

容器默认执行：

```bash
dpa run --config /config/config.yaml
```

推荐挂载：

```bash
docker run -d --name data-proxy-agent --restart unless-stopped \
  -v "$PWD/agent-config:/config" \
  -v "$PWD/workspace:/workspace" \
  ghcr.io/normojs/data-proxy-agent:<version>
```

镜像运行层携带 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`，保证基于 new-api/Data Proxy 分发时保留 AGPL 与第三方许可材料。

本地构建遇到 Go module 下载 EOF 或超时，可通过 `--build-arg GOPROXY=https://goproxy.cn,direct` 覆盖默认代理；CI 默认仍使用 `https://proxy.golang.org,direct`。

`dpa update` 支持两种来源：

- 默认来源：GitHub Release API，仓库为 `normojs/data-proxy`，按当前 OS/ARCH 选择资产，并下载同名 `.sha256` 校验。
- 自定义来源：`--manifest-url`，适合未来由 Data Proxy 控制台、企业内网镜像或对象存储下发。

安装脚本也支持同一个 manifest：设置 `DATA_PROXY_AGENT_MANIFEST_URL` 后会按当前
OS/ARCH 选择资产 URL，并使用 manifest 内的 sha256 校验下载文件。

manifest 格式：

```json
{
  "version": "v1.2.3",
  "assets": [
    {
      "name": "data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
      "url": "https://dp.example.com/agent/data-proxy-agent-v1.2.3-linux-amd64.tar.gz",
      "os": "linux",
      "arch": "amd64",
      "sha256": "<64 hex chars>"
    }
  ]
}
```

升级流程：

1. 解析 release 或 manifest，选择当前平台资产。
2. 下载 archive 和 sha256。
3. 解包并运行下载二进制的 `self-test`。
4. 写入同目录 `.new` 文件。
5. 替换当前 install path，并保留 `.bak` 回滚文件。

Windows 正在运行中的 exe 不能由自身进程直接覆盖，当前实现会生成 `.new.exe`
staged 文件，停止服务后再替换；后续可补 Windows helper 或 MSI。

已支持：

- Homebrew formula release asset；后续可拆出独立 tap 仓库自动提交 formula。
- deb/rpm 包，内置 systemd service 模板和最小 `/etc/data-proxy-agent/config.yaml`。
- Windows MSI，安装二进制与许可文件；系统服务仍由 `dpa service install` 管理。
- macOS notarization 条件执行；需要在 GitHub Secrets 配置 `APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_SPECIFIC_PASSWORD`、`APPLE_SIGNING_IDENTITY`、`APPLE_CERTIFICATE_P12_BASE64`、`APPLE_CERTIFICATE_PASSWORD`。
- 已在 tag release workflow 中对 `checksums.txt` 生成 cosign signature/bundle，并对 GHCR agent 镜像 digest 做 keyless cosign 签名。

## 与当前原型的关系

当前原型：

- `tools/bridge_client_daemon.mjs`
- 支持 `/bridge/ws`
- 支持本地文件只读/可选写入
- 支持 MCP proxy
- 支持 HTTP Tunnel、流式响应、流式上传、WebSocket
- 支持 self-test

正式 CLI 的第一阶段目标不是重写服务端，而是替换这个 Node 原型：

1. Go CLI 使用同样的 Bridge WebSocket 协议。
2. 保持现有服务端 API 不变。
3. 保持现有 Tunnel App、Connection、Audit、Billing event 不变。
4. 先实现与 Node 原型等价的能力。
5. 再增加控制台健康检查等产品化能力。

## 当前实现状态

Go 版 `dpa` 已开始落地：

- 已新增 `cmd/dpa` 主入口，并保留 `cmd/data-proxy-agent` 兼容入口。
- 已新增 `pkg/dpagent`，封装配置、CLI runner 和 Bridge WebSocket 客户端骨架。
- 已实现 `version`、`help`、`config path`、`config show`、`config validate`、`config export`、`status`、`status --json`、`status --health`、`doctor`、`doctor --json`、`self-test`、`update`、`run`，并补齐 `config`、`mcp`、`tunnel`、`logs`、`service` 命令组的 `--help` 输出。
- `doctor` 已能检查本地 workspace、本地审计路径、远端 Bridge token 握手、系统服务状态、HTTP route TCP 连通性、TCP route TCP 连通性、MCP HTTP endpoint 连通性、stdio MCP shell/命令前缀和已启动进程状态。
- 已实现 `enroll`，支持 `/api/bridge/agent-setup/consume` 一次性 setup token 绑定，也可兼容调用 `/api/bridge/agent-setup` 注册 Bridge Client，并默认通过 `--token-store auto` 把 agent token 写入系统 keyring；无可用 keyring 时回退到权限收紧的 secret-file，也可显式用 `--token-store config` 保持兼容。
- 已实现 `report`，可生成脱敏诊断 zip，并记录远端 Bridge token 握手校验结果和系统服务状态。
- 已实现 `logs path/tail`，读取本地 `logging.local_audit_jsonl` 审计 JSONL。
- 已实现 `service install/uninstall/start/stop/restart/status/print`，可生成并管理 Linux systemd、macOS launchd、Windows Service 配置。
- 已实现 `mcp list/add/test/remove`，用于管理本地 Streamable HTTP MCP endpoint 配置。
- 已实现 `tunnel route list/add/test/remove`，用于管理并探测本地 HTTP/WebSocket/SSE 和 TCP route 配置。
- `run` 已能读取配置和环境变量，连接 `/bridge/ws`，携带 Bearer token，发送 `register`，处理 `registered`、`pong`、`close` 和服务端 `error`。
- 已实现心跳 `ping` 和重连退避。
- 已实现 agent 在线健康上报：注册成功后立即通过 Bridge WebSocket 发送 `health` 摘要，之后按 `runtime.health_interval_ms` 定时上报；缺省为 60 秒，设为负数可禁用；服务端仅保存最新一份元数据级健康 JSON，并在 Bridge Client 健康详情 API/控制台展示。
- 已实现敏感 token 脱敏、私有权限配置文件写入和基础配置校验。
- 已实现 `http_tunnel.request` 普通/流式/SSE/WebSocket 转发：目标校验、loopback 默认限制、header 过滤、body base64、流式上传/下载、WebSocket frame 转发、响应截断和服务端兼容的 `http_response` metadata。
- 已实现 MCP bridge 基础能力：`mcp_proxy.test`、`mcp_proxy.tools_list`、`mcp_proxy.tools_call` 和 `mcp_proxy.rpc`，支持 Streamable HTTP 目标、`Mcp-Session-Id` 会话复用、SSE `data:` 响应解析、loopback 默认限制，以及本地 stdio MCP 子进程桥接。stdio command 仅从本机配置读取，远端请求只能按本地 MCP server 名称选择；stdio 子进程启动、退出、退出后下次调用自动重启，以及 watchdog 清理已退出缓存会话，都会写入本地审计 JSONL，退出 stderr 会做基础分类并写入 `stderr_class`。
- 已实现本地文件和项目只读工具：`remote_read`、`remote_tree`、`remote_glob`、`remote_grep`、`remote_env_info`、`remote_project_info`、`remote_get_related_files`、`remote_git_status`、`remote_git_diff`、`remote_git_log` 默认启用；Git 工具只运行固定只读 argv，并关闭外部 diff、pager 和交互提示。写入 `remote_write`、`remote_edit` 已实现但默认关闭，必须本机配置 `policy.allow_write=true` 才会上报和执行。所有路径默认限制在 `agent.workspace` 内，支持 `policy.allowed_workspaces`、`policy.denied_paths`、symlink 防逃逸、常见目录忽略、服务端 policy 限额收紧、本地结果截断和写入大小限制。
- 已实现 `remote_run_tests` 安全测试命令：默认关闭，必须本机配置 `policy.exec.enabled=true` 且命令精确命中 `policy.exec.safe_commands`；执行目录仍限制在 workspace 内，输出受 `max_result_bytes` 限制，非零退出会作为工具结果返回给调用方。
- 已实现 `update` 自动升级命令：支持 GitHub Release 和自定义 manifest，下载后校验 sha256、运行 `self-test`、替换前生成 `.new`、成功后保留 `.bak`。Windows 自替换先 staging，停止服务后再覆盖。
- 已新增 GitHub Actions `Data Proxy Agent` workflow，对 agent 运行测试，构建 Linux/macOS/Windows 的 amd64/arm64 二进制包、Linux deb/rpm、Windows MSI、Homebrew formula 与 sha256 校验文件，在 `v*` tag 上传 GitHub Release 附件，并用 cosign 签名 `checksums.txt`。
- 已新增 `Dockerfile.agent` 和 GHCR 发布任务，发布 linux/amd64、linux/arm64 多架构镜像 `ghcr.io/normojs/data-proxy-agent:<version>`，镜像内保留 AGPL/NOTICE/第三方许可文件。
- 已实现 `remote_exec` 一次性命令执行、基础持久 shell `remote_shell_open` / `remote_shell_eval`、PTY shell 和 `remote_shell_resize`，但默认关闭；必须本机配置 `policy.exec.enabled=true` 且 `policy.exec.allow_arbitrary=true` 才会上报和执行。`remote_exec` 调用本机 shell；持久 shell 默认使用 stdin/stdout，传 `pty=true` 时在 Unix/macOS 使用 PTY，Windows 暂时降级为非 PTY。它们不提供进程级沙盒，只适合用户明确授权的可信工作区。已实现 `remote_install_package`，通过固定包管理器 argv 执行单包安装，除 trusted exec 外还要求 `policy.allow_write=true`。

尚未从 Node 原型迁移到 Go CLI：暂无阻塞项；后续主要是 Windows ConPTY、更完整的终端信号处理、Homebrew tap 仓库自动提交和企业级配置下发。

## 开发顺序

### Phase 1: CLI 骨架

- 新增 `cmd/dpa`，并保留 `cmd/data-proxy-agent` 兼容入口。
- 实现 `version`、`help`、`config path`、`config validate`。
- 实现跨平台配置路径。
- 实现日志、错误码和敏感信息打码。

### Phase 2: Enroll 与凭证

- 服务端已新增一次性 setup token 创建/消费接口。
- CLI 已实现 `enroll --server --setup-token`。
- 已支持保存 agent token 到本地 secret store 或权限收紧的 token file；`config token status/store/migrate/delete` 可管理 `agent.token_ref`。
- 控制台显示一键安装命令。

### Phase 3: Bridge 连接

- CLI 实现 `/bridge/ws` 注册、心跳、重连。
- 兼容现有 `tool_call` / `tool_result` 协议。
- 上报 version、platform、hostname、capabilities。
- 实现 `status` 和 `doctor`。

### Phase 4: MCP Bridge

- 支持 streamable HTTP MCP target。
- 实现 `mcp add/list/test/remove`。
- 转发 `tools/list`、`tools/call`、`resources/*`、`prompts/*`。
- 实现本地 MCP target allowlist 和 loopback 默认限制。

### Phase 5: HTTP Tunnel

- 支持普通 HTTP、流式 body、SSE、WebSocket。
- 实现 target allow/deny、host/port 限制和超时。
- 实现大文件 backpressure 和取消。
- 与现有服务端 HTTP Tunnel 测试对齐。

### Phase 6: 系统服务和安装包

- 已实现 `service install/start/stop/status/uninstall` 基础命令。
- GitHub Actions 跨平台构建、deb/rpm、MSI、Homebrew formula 和可选 macOS notarization。
- 已生成 install script。
- 支持 Docker 镜像。

### Phase 7: 产品化

- 已实现自动更新命令、release/checksum/image 签名、deb/rpm、MSI、Homebrew formula 和可选 macOS notarization，后续可补 Windows helper 与 Homebrew tap 自动提交。
- 已实现本地诊断包基础命令，后续可接控制台上传和版本建议。
- 已实现控制台健康摘要展示、远端 token 校验、系统服务状态诊断、stdio MCP 子进程健康摘要、`mcp_processes` 结构化进程状态展示，以及本地 watchdog 清理已退出 stdio MCP 缓存会话。
- 多 Agent 管理。
- 策略版本和配置下发。
- MCP stdio 的历史事件聚合和更细生命周期视图。
- Windows ConPTY、终端信号处理和更接近云 IDE 的交互式 shell 体验。

## 第一版不做的事情

- 不在 Agent 内直接训练或保存用户模型数据。
- 不默认开启写文件和命令执行。
- 不把所有 MCP Server 内置进 Agent。
- 不要求用户必须使用 `coding-tools-mcp`，只要符合 MCP 协议即可。
- 不在第一版实现 raw TCP listener/端口池；当前先提供 TCP-over-WebSocket MVP。

## 参考资料

- Cloudflare Tunnel downloads: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/downloads/
- Cloudflare Tunnel setup: https://developers.cloudflare.com/tunnel/setup/
- Cloudflare Tunnel configuration file: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/configuration-file/
- Cloudflare Tunnel run parameters: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/configure-tunnels/run-parameters/
- Cloudflare Tunnel useful commands: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/local-management/tunnel-useful-commands/
