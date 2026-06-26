# Data Proxy

[![CI](https://github.com/normojs/data-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/normojs/data-proxy/actions/workflows/ci.yml)
[![Docker](https://github.com/normojs/data-proxy/actions/workflows/data-proxy-docker.yml/badge.svg)](https://github.com/normojs/data-proxy/actions/workflows/data-proxy-docker.yml)
[![License: AGPLv3](https://img.shields.io/badge/license-AGPLv3-brightgreen.svg)](./LICENSE)

Data Proxy 是一个面向企业治理场景的 AI API 网关和额度管控平台，基于开源项目 [new-api](https://github.com/QuantumNous/new-api) 开发。

它继承 new-api 的多模型接入、OpenAI 兼容协议、渠道路由、用户与令牌管理、额度和用量统计能力，并在此基础上增加企业组织、策略额度、审批通知、审计可见性、SSO 同步和发布合规链路。

> [!IMPORTANT]
> 本项目是基于 new-api 的二次开发版本。请保留 [LICENSE](./LICENSE)、[NOTICE](./NOTICE)、[THIRD-PARTY-LICENSES.md](./THIRD-PARTY-LICENSES.md)、原项目链接和 NOTICE 中要求的可见 attribution。Data Proxy 继续遵循 AGPLv3 及 NOTICE Section 7 的附加要求。

## 项目定位

Data Proxy 适合需要集中管理大模型 API 资产的团队：

- 在同一入口管理 OpenAI 兼容、Claude、Gemini、Responses、Realtime、Rerank 等模型协议和渠道。
- 为企业、部门、策略分组和用户设置请求数或 quota 策略。
- 通过 dry-run、hard limit、用量归因和审计日志逐步上线企业治理规则。
- 让临时额度申请、审批结果、过期提醒和外部通知形成可追踪闭环。
- 用 GitHub CI、Docker 发布证据和许可文件分发要求保障发布可回溯。

## 当前能力

### 继承自 new-api 的能力

- OpenAI 兼容 API 网关，支持多模型、多渠道和自动重试。
- 用户、令牌、分组、模型权限、额度、计费和统计仪表盘。
- 多种登录和 OAuth/OIDC 接入能力。
- 与 One API 数据结构的兼容迁移基础。
- Docker、Compose、环境变量和初始化向导部署路径。

更多上游能力可以参考 [new-api 官方文档](https://docs.newapi.pro/) 和 [new-api 仓库](https://github.com/QuantumNous/new-api)。

### Data Proxy 增强能力

- 企业治理模型：企业、组织部门、成员、策略分组、额度策略、用量归因和审计日志。
- 额度策略：支持 `request_count` 和 `quota`，可按企业、部门、分组或用户命中，支持 dry-run 与 hard reject。
- 临时额度审批：用户提交、管理员审批、拒绝、撤回、过期和即将过期提醒。
- 通知闭环：站内通知、企业审计事件、email/webhook outbox、通知偏好、投递结果查询、失败重试和 worker 指标。
- Tunnel Apps：规划并开始实现 `mcp_code` 与 `http_tunnel` / `tcp_tunnel` 两类隧道应用，当前已具备申请/审批/审计/Bridge policy 同步、管理端列表、用户专属连接密钥、MCP 网关入口、HTTP 隧道和 TCP-over-WebSocket MVP。
- HStation OAuth：登录、注册、绑定、解绑、管理员配置和自动化测试覆盖。
- SSO 组织同步：支持 payload preview、dry-run、冲突列表、事务 apply 和同步审计。
- 企业额度 Redis 计数：可选 Redis 原子 reserve/settle/refund，DB 降级和 DB/Redis 对账修复。
- 高级治理动作：支持模型降级、企业排队、共享池、异常保护和队列 replay；排队请求可记录审计生命周期，并支持 inline JSON、大 payload DB 或本地/S3 对象存储持久化、payload TTL 清理、multipart/audio upload 重放，以及队列 payload 可见性脱敏。
- fusion-benchmark：离线数据集、配置校验、fixture、自检和 CI 检查脚本。

## 快速开始

### 使用 Docker Compose

```bash
git clone https://github.com/normojs/data-proxy.git
cd data-proxy
docker compose up -d data-proxy
```

启动后访问：

```text
http://localhost:3000
```

首次安装请优先使用初始化向导配置数据库和 Redis，然后创建第一个管理员账号。显式环境变量仍然支持，但更适合高级运维覆盖。

### 使用本地依赖

如果希望 Compose 同时启动 PostgreSQL 和 Redis：

```bash
docker compose --profile local-deps up -d
```

初始化向导中使用：

- PostgreSQL host: `postgres`
- Redis host: `redis`

这些本地依赖默认只在 Compose 网络内可见，不会占用宿主机的 `5432` 或 `6379` 端口。

### 使用已发布镜像

```bash
docker pull ghcr.io/normojs/data-proxy:latest
```

稳定版本示例：

```bash
docker pull ghcr.io/normojs/data-proxy:v1.3.0
```

发布、tag、镜像摘要和回滚流程见 [Data Proxy Release Runbook](./docs/data-proxy-release-runbook.md)。

### 生产部署与回滚

生产服务器建议使用 `scripts/prod-deploy.sh` 和 `scripts/prod-compose.sh`，不要直接手写 Compose 文件组合。脚本会固定加载 `docker-compose.prod.yml` 与 `docker-compose.wechat-pay.yml`，确保微信支付商户私钥目录始终挂载到 `/run/secrets/data-proxy/wechatpay:ro`。

GitHub tag 会触发 `Publish Data Proxy image` workflow，发布
`ghcr.io/normojs/data-proxy:<tag>`、`sha-<short-sha>` 和可选 `latest`。部署时优先记录镜像 tag、digest、commit SHA 和上一个已验证镜像 digest。

```bash
scripts/prod-deploy.sh ./data-proxy-<tag>.tar
scripts/prod-deploy.sh ghcr.io/normojs/data-proxy:<tag>
```

每次部署前会把当前运行镜像保存到 `/root/workspace/dataproxy/image-archive`，默认保留最近 10 份。新镜像异常时可直接回滚：

```bash
scripts/prod-rollback.sh
```

部署后至少运行一次生产 smoke，并把 summary、request id、镜像 digest 和回滚镜像记录到发布证据中：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='gpt-4o-mini' \
scripts/data-proxy-production-smoke.sh
```

需要同时验证 request trace 和诊断包时，传入管理员 header：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_ADMIN_HEADER='Cookie: session=...' \
DATA_PROXY_SMOKE_REQUEST_ID='REQ_ID' \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
scripts/data-proxy-production-smoke.sh
```

## 常用配置

首次安装推荐通过 Web 初始化向导写入 runtime config。高级场景可以使用 `.env.example` 中的环境变量覆盖。

常见变量：

| 变量 | 说明 |
| --- | --- |
| `SQL_DSN` | 数据库连接字符串，高级覆盖项。 |
| `REDIS_CONN_STRING` | Redis 连接字符串，高级覆盖项。 |
| `SESSION_SECRET` | 多节点部署必须设置的会话密钥。 |
| `NODE_TYPE` | 主节点可设为 `master`，用于周期任务。 |
| `NODE_NAME` | 节点名称，会进入审计和运维排查链路。 |
| `DATA_PROXY_SETUP_AUTO_RESTART` | 控制初始化向导保存配置后是否自动触发容器重启。 |
| `ENTERPRISE_QUEUE_PAYLOAD_TTL_SECONDS` | queue replay payload 保留秒数，默认 7 天；只清理已 released 的旧 payload 和旧孤儿 payload。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER` | queue replay 大 payload 外部对象存储 provider；未设置时使用 DB，支持 `local` 或 `s3`。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_DIR` | `local` provider 的对象目录；未设置时使用系统临时目录。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT` / `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_BUCKET` | `s3` provider 的 S3 或 S3-compatible endpoint 和 bucket。 |
| `CAPTURE_ENABLED` | 请求捕获总开关，默认关闭；完整捕获链路完成前生产环境应保持 `false`。 |
| `CAPTURE_LEVEL` / `CAPTURE_SAMPLE_RATE` / `CAPTURE_MODEL_PATTERNS` | 请求捕获策略配置，可先按模型、路径、用户、token、渠道小范围启用。 |
| `CAPTURE_START_TIMESTAMP` / `CAPTURE_END_TIMESTAMP` | 请求捕获时间窗口，Unix 秒；`0` 表示不限制对应边界。 |
| `CAPTURE_SEVERITIES` | 只捕获指定严重级别的显式诊断/异常流量，例如 `warning,error`；正常 relay 未标记 severity 时不会命中该过滤器。 |
| `CAPTURE_MAX_ARTIFACT_BYTES` | 单个请求/响应 artifact 的最大保存字节数，`0` 表示不限制；超出后只截断捕获数据，不影响主请求。 |
| `CAPTURE_OBJECT_BACKEND` / `CAPTURE_S3_ENDPOINT` / `CAPTURE_S3_BUCKET` | 请求捕获私密数据包的 SeaweedFS/S3-compatible 存储配置。 |
| `CAPTURE_BUNDLE_MASTER_KEY` | 请求捕获私密数据包 AES-256-GCM 加密主密钥，支持 `base64:` 或 `hex:` 前缀，必须解码为 32 字节。 |
| `CAPTURE_RETENTION_DAYS` / `CAPTURE_SPOOL_RETENTION_DAYS` | 请求捕获对象和本地 spool 的留存天数，默认分别为 30 天和 7 天；设为 `0` 表示关闭对应清理维度。 |
| `CAPTURE_CLEANUP_ENABLED` / `CAPTURE_CLEANUP_INTERVAL_SECONDS` / `CAPTURE_CLEANUP_LIMIT` | 请求捕获留存清理任务，默认跟随 capture 存储启用，只在 master/single-node 节点执行；管理员也可调用 `POST /api/log/request-capture/cleanup?dry_run=true` 先预览。 |
| `DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES` | 诊断包下载时 raw capture tar 的最大展开字节数，默认 256 MiB，`0` 表示不限制；超限只跳过 zip 内 raw 展开并写说明文件，不影响正常请求或对象存储原始包。 |

完整部署说明见 [Data Proxy Operator Guide](./docs/data-proxy-operator-guide.md)。

请求捕获和诊断数据湖使用 SeaweedFS/S3-compatible 存储；生产部署可通过
`docker-compose.capture-storage.yml` 追加 SeaweedFS 服务和持久化卷映射。

## 管理与验证

### 企业治理入口

- 管理端入口：`Admin` -> `Enterprise Governance`
- 路由：`/enterprise`
- 权限：管理员及以上

建议上线顺序：

1. 保持企业治理关闭，确认现有网关和计费链路不受影响。
2. 开启 dry-run，观察策略命中、would reject 审计和用量归因。
3. 对测试用户或测试分组开启小范围 hard limit。
4. 再扩大到真实部门或企业级策略。

详细操作见 [Enterprise Governance Admin Guide](./docs/enterprise-governance-admin-guide.md)。

### 本地验证

常用验证命令：

```bash
git diff --check
go test ./model ./controller ./service ./router ./oauth
cd web/default && bun run typecheck
cd web/default && bun run smoke:approval-notification-links
cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build
scripts/fusion-benchmark-check.sh
```

发布前建议运行完整预检：

```bash
make deployment-preflight
```

可选 Docker 构建预检：

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight
```

### Request ID 排障

当客户端出现“HTTP 200 但无回复”、流式中断、协议转换异常或计费疑问时，优先使用 request id 查询追踪信息：

```text
GET /api/log/request?request_id=REQ_ID
GET /api/log/self/request?request_id=REQ_ID
```

控制台可在 `Usage Logs -> Common` 按 request id 过滤，也可点击 request id 旁的追踪图标直接打开日志详情，并在 `Request Trace` 区块查看转换链、流状态、上游 request id 和关联错误。完整说明见 [Request ID Trace Troubleshooting](./docs/request-trace-troubleshooting.md)。

### Tunnel Apps

用户先在 `MCP -> My Tunnel Apps` 为本地 Bridge 客户端申请 `mcp_code`、`http_tunnel` 或 `tcp_tunnel` Tunnel App。管理员在 `MCP -> Tunnel Apps` 审批通过后，用户可在 `MCP -> Tunnel Connections` 为该应用创建专属连接。连接 key 只在创建成功时显示一次，MCP 客户端使用：

```text
https://<data-proxy-host>/t/<connection_key>/tunnel/mcp/<public_slug>
```

HTTP/TCP Tunnel 使用同样的 connection key 模型，入口分别是 `/t/<connection_key>/tunnel/http/<public_slug>` 和 `/t/<connection_key>/tunnel/tcp/<public_slug>`；控制台会按已审批 app 的目标地址给出对应的 `dpa tunnel route add ...` 本地路由命令。每条连接都绑定当前用户和 Tunnel App，可单独撤销，并会写入 Tunnel audit log。控制台在 `MCP -> Tunnel Connections` 的连接行提供审计入口，可按 connection 查看 `tools/list`、`tools/call`、HTTP/TCP proxy request、策略拒绝和撤销事件。完整架构见 [Tunnel Apps Architecture](./docs/tunnel-apps-architecture.md)。

### Data Proxy Agent

跨平台本地 Agent 已开始开发，主命令为 `dpa`。源码入口是 `cmd/dpa`，`data-proxy-agent` 作为兼容命令继续保留，避免旧脚本和服务升级时断开。

```bash
go run ./cmd/dpa --help
go run ./cmd/dpa self-test
```

当前 Go 版已具备 `enroll` 注册写配置、控制台一次性 setup token 绑定、`report` 脱敏诊断包、`logs path/tail` 本地审计查看、`service install/start/stop/status/uninstall`、`update` 升级、配置管理、`config token` 系统 keyring/secret-file 凭据管理、`doctor` 本地健康检查、远端 Bridge token 校验和系统服务状态诊断、agent 在线健康上报与 Bridge Client 详情展示、`/bridge/ws` 注册/心跳、本地文件工具 `remote_read` / `remote_tree` / `remote_glob` / `remote_grep` / `remote_env_info` / `remote_project_info` / `remote_get_related_files` / `remote_git_status` / `remote_git_diff` / `remote_git_log` / `remote_write` / `remote_edit`、安全测试命令 `remote_run_tests`、可信一次性命令 `remote_exec`、基础持久 shell `remote_shell_open` / `remote_shell_eval`、PTY shell 和 `remote_shell_resize`、可信依赖安装 `remote_install_package`、`http_tunnel.request` 普通/流式/SSE/WebSocket 转发、`tcp_tunnel.connect` TCP-over-WebSocket 转发，以及 MCP bridge 的 `mcp_proxy.test` / `mcp_proxy.tools_list` / `mcp_proxy.tools_call` / `mcp_proxy.rpc`；MCP bridge 支持本地 Streamable HTTP 和 stdio MCP，stdio 子进程具备启动/退出审计、进程健康展示和退出后下次调用自动重启。stdio 命令只从本机 agent 配置读取，服务端不能动态下发任意 command。其中写入工具默认关闭，需要本机配置 `policy.allow_write=true`；`remote_run_tests` 默认关闭，需要本机配置 `policy.exec.enabled=true` 和 `policy.exec.safe_commands`；`remote_exec`、PTY shell 与基础持久 shell 默认关闭，需要本机额外配置 `policy.exec.allow_arbitrary=true`；`remote_install_package` 还要求 `policy.allow_write=true`；TCP 目标必须匹配本机 `tcp_routes`，非 loopback 目标还需要本机配置 `policy.allow_non_loopback_tcp=true`。Release checksum index 和 GHCR 镜像 digest 会通过 cosign 签名。完整设计和状态见 [Data Proxy Agent CLI Design](./docs/data-proxy-agent-cli-design.md)。

常用本地配置命令：

```bash
go run ./cmd/dpa enroll --server https://<data-proxy-host> --setup-token <one-time-token>
go run ./cmd/dpa enroll --server https://<data-proxy-host> --access-token <dashboard-access-token> --user-id <id>
go run ./cmd/dpa config token status
go run ./cmd/dpa config token migrate --store auto
go run ./cmd/dpa mcp add coding --url http://127.0.0.1:30837/mcp
go run ./cmd/dpa mcp add filesystem --transport stdio --command "npx -y @modelcontextprotocol/server-filesystem /Users/me/project"
go run ./cmd/dpa mcp list
go run ./cmd/dpa tunnel route add http local-web --url http://127.0.0.1:3000 --allow-websocket
go run ./cmd/dpa tunnel route add tcp local-ssh --host 127.0.0.1 --port 22
go run ./cmd/dpa tunnel route list
go run ./cmd/dpa doctor
go run ./cmd/dpa logs path
go run ./cmd/dpa logs tail --lines 100
go run ./cmd/dpa logs tail --follow
go run ./cmd/dpa report --output ./agent-diagnostic.zip
go run ./cmd/dpa service install --dry-run
go run ./cmd/dpa update --dry-run
```

控制台可通过 `POST /api/bridge/agent-setup-tokens` 生成一次性 setup token 和复制命令；本地 agent 使用 `POST /api/bridge/agent-setup/consume` 换取专属 bridge client 与 agent API key。setup token 只保存 hash，默认 10 分钟过期且只能消费一次。

GitHub Actions 中的 `Data Proxy Agent` workflow 会对 agent 包执行测试，并构建 Linux/macOS/Windows 的 amd64/arm64 二进制压缩包、Linux `deb/rpm`、Windows `msi`、Homebrew formula 和 sha256 校验文件；tag 以 `v*` 发布时会上传 Release 附件，并发布多架构 Docker 镜像到 GHCR。macOS notarization 会在配置 Apple 签名 secrets 后自动生成 `*-notarized.tar.gz` 产物，未配置时跳过且不阻塞发布。安装脚本可直接使用 GitHub Release：

```bash
curl -fsSL https://raw.githubusercontent.com/normojs/data-proxy/main/scripts/install-data-proxy-agent.sh | sh
dpa update --dry-run
dpa update
```

Docker 镜像适合服务器或 NAS 上长期运行本地 bridge agent；生产部署建议使用固定版本或 sha 标签，便于回滚：

```bash
docker pull ghcr.io/normojs/data-proxy-agent:v1.3.0
docker run -d --name data-proxy-agent --restart unless-stopped \
  -v "$PWD/agent-config:/config" \
  -v "$PWD/workspace:/workspace" \
  ghcr.io/normojs/data-proxy-agent:v1.3.0
```

默认容器命令为 `dpa run --config /config/config.yaml`。镜像内置 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`，用于保留 new-api/Data Proxy 的 AGPL 与第三方许可信息。

本地构建时如果 Go module 下载网络不稳定，可使用 `docker build -f Dockerfile.agent --build-arg GOPROXY=https://goproxy.cn,direct -t data-proxy-agent:test .`。

`update` 默认从 `normojs/data-proxy` 的 GitHub Release 选择当前平台资产，也支持 `--manifest-url` 接入 Data Proxy 控制台或内网镜像源。Windows 正在运行中的可执行文件无法被自身进程直接替换，命令会先生成 `.new.exe` staged 文件，停止服务后再替换。

## 文档索引

| 文档 | 用途 |
| --- | --- |
| [Data Proxy Operator Guide](./docs/data-proxy-operator-guide.md) | 运行、初始化、依赖和部署交接。 |
| [Data Proxy Release Runbook](./docs/data-proxy-release-runbook.md) | tag、镜像、发布证据、回滚和合规检查。 |
| [Deployment Readiness](./docs/deployment-readiness.md) | 发布前预检命令和当前机器状态记录。 |
| [Request ID Trace Troubleshooting](./docs/request-trace-troubleshooting.md) | 按 request id 查询日志、转换链、流状态和关联错误。 |
| [Request Capture, Diagnostics, and Training Data Architecture](./docs/request-capture-diagnostics-architecture.md) | SeaweedFS 私密数据包、诊断模块和训练数据湖架构。 |
| [Request Capture and Diagnostics Implementation Plan](./docs/request-capture-diagnostics-implementation-plan.md) | 请求捕获、诊断和训练数据湖的分阶段开发顺序与当前状态。 |
| [Tunnel Apps Architecture](./docs/tunnel-apps-architecture.md) | MCP 代码隧道、HTTP/TCP 通用流量隧道、Cloudflare Remote MCP 对标和实施顺序。 |
| [Data Proxy Agent CLI Design](./docs/data-proxy-agent-cli-design.md) | 跨平台本地 Agent CLI 设计，覆盖 cloudflared 类隧道、MCP bridge、配置、服务安装和发布计划。 |
| [Single-Node Development Roadmap](./docs/data-proxy-single-node-development-roadmap.md) | 当前单服务器部署优先的后续开发顺序，暂不纳入多节点/分布式协调。 |
| [Enterprise Governance Admin Guide](./docs/enterprise-governance-admin-guide.md) | 企业治理管理员操作手册。 |
| [Post V1.3 TODO](./docs/data-proxy-post-v1.3-todo.md) | V1.3 之后的开发顺序和剩余任务。 |
| [Branding and Release Policy](./docs/branding-and-release-policy.md) | Data Proxy 品牌边界和 new-api attribution 规则。 |

仓库中的 `README.en.md`、`README.zh_CN.md`、`README.zh_TW.md`、`README.fr.md`、`README.ja.md` 仍保留为上游 new-api 资料和历史 attribution 参考。Data Proxy 的运行和发布入口以本 README 及 `docs/data-proxy-*` 文档为准。

## 开源协议与合规

Data Proxy 基于 [new-api](https://github.com/QuantumNous/new-api) 开发，继续采用 [GNU Affero General Public License v3.0](./LICENSE)。

请注意：

- 分发源码、镜像、二进制、前端 bundle 或桌面安装包时，必须保留 `LICENSE`、`NOTICE` 和 `THIRD-PARTY-LICENSES.md`。
- 修改版不能误导软件来源，需要清楚标记 Data Proxy 的变更来源。
- 带 UI 的修改版必须保留 NOTICE 中要求的可见 attribution 文案和原项目链接。
- Docker 镜像发布链路应继续携带 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`。

如果你的组织不能接受 AGPLv3 或 NOTICE Section 7 的义务，请在部署、分发或提供网络服务前先完成内部法务评估。

## 合法使用

本项目仅适用于合法、授权的 AI API 网关、企业组织认证、多模型管理、用量分析、成本核算和私有化部署场景。

使用者需要自行合法取得上游 API key、账号、模型服务和接口授权，并遵守上游服务条款及适用法律法规。若向公众提供生成式 AI 服务或 API 转售服务，应先完成所在地要求的备案、许可、内容安全、实名、日志留存、税务、支付和上游授权义务。
