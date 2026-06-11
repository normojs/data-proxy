# AI Code Rules for data-proxy

本文档是当前 data-proxy 开发任务的 AI coding rules。它补充顶层
`AGENTS.md` / `CLAUDE.md` 的通用项目规范，重点约束 MCP、Bridge、
OpenAPI、计费流水和运营 Dashboard 的后续开发。

## 1. 工作边界

- 当前 Git 仓库是 `upstream/new-api`，不是上一级 `data-proxy` 目录。
- 当前任务属于 data-proxy 服务端与本仓库内的本地测试工具。
- 不要在 QidianBrowser 仓库或真实产品客户端中开发。
- `tools/bridge_client_daemon.mjs` 是本仓库内的真实本地 Bridge Client
  daemon，用于验证 data-proxy 协议，不等同于 QidianBrowser 产品实现。
- `qidian_browser` transport 只是兼容已有协议命名；实现仍应落在本仓库的
  data-proxy 侧。

## 2. 开工前检查

每次开始实现前先做最小上下文确认：

```bash
pwd
git status --short --branch
sed -n '1,220p' todo.md
sed -n '1,220p' docs/ai-code-rules.md
```

涉及 MCP / Bridge 时同时阅读：

```bash
sed -n '1,360p' docs/mcp-bridge-smoke.md
```

如果 worktree 有 dirty / untracked 文件：

- 先判断是否是自己本轮产生的改动。
- 不要 `git reset --hard`，不要 checkout 覆盖用户改动。
- 和当前任务无关的 dirty 文件保持不动。
- 与当前任务有关但来源不明的改动，先读懂再叠加修改；必要时向用户确认。

## 3. 架构与模块地图

后端遵循现有层次：

- `router/`: 路由注册。
- `controller/`: HTTP 参数绑定、鉴权上下文、响应封装。
- `service/`: 业务流程、事务、跨模型编排。
- `model/`: GORM model、查询、迁移、跨数据库兼容。
- `dto/`: 请求/响应结构。
- `pkg/mcp/`: MCP 执行器、OpenAPI parser/store、Proxy、Bridge 协议逻辑。
- `tools/`: 本地 smoke、daemon、自检脚本。
- `docs/`: 交接、协议、验证命令。
- `web/default/`: 主要前端，React 19 + TypeScript + Rsbuild + Base UI。
- `web/classic/`: 旧版前端；除非用户明确要求，不主动改这里。

MCP / Bridge 常用文件：

- `router/mcp-router.go`, `router/bridge-router.go`
- `controller/mcp*.go`, `controller/bridge.go`
- `service/mcp*.go`, `service/bridge*.go`
- `model/mcp*.go`, `model/bridge*.go`
- `pkg/mcp/executor/*`, `pkg/mcp/proxy/*`, `pkg/mcp/openapi/*`
- `web/default/src/features/mcp/*`

## 4. 后端规则

- JSON marshal/unmarshal 必须优先使用 `common/json.go` 的包装函数：
  `common.Marshal`, `common.Unmarshal`, `common.DecodeJson` 等。只在类型定义
  或底层 wrapper 内直接使用 `encoding/json`。
- 数据库逻辑必须同时兼容 SQLite、MySQL、PostgreSQL。优先 GORM；必须 raw
  SQL 时按顶层 `AGENTS.md` 的跨库规则处理。
- 新增 model 字段或迁移时，检查 `model/main.go` 中的 auto migrate 与历史迁移
  模式，避免只在单一数据库可用。
- 不要把 provider-specific 逻辑写进 MCP 通用执行路径；多模型/多供应商能力
  应复用 new-api 已有 channel/relay 抽象。
- 不要绕开已有鉴权 middleware；下载、清理、管理接口必须区分 user/admin。
- 错误码要稳定、可测试。Bridge/MCP/OpenAPI 的错误应能在调用记录、审计日志
  或响应 metadata 中定位。

## 5. MCP / Bridge 规则

- Bridge session replacement 必须让 pending calls 以稳定错误结束，不能悬挂。
  当前约定客户端断开映射为 `BRIDGE_CLIENT_DISCONNECTED`。
- Bridge 审计日志、`mcp_tool_calls`、计费事件需要保持一致：成功、失败、超时、
  断开、退款路径都要能追踪。
- 本地 daemon 默认禁止写入；启用写工具必须显式传 `--enable-write`。
- 本地 daemon 的 MCP Proxy target 默认只允许 loopback：
  `localhost`, `127.0.0.1`, `::1`。只有测试或明确需求才允许
  `--allow-non-loopback-mcp`。
- 文件工具必须保留 workspace 边界、路径穿越防护、结果大小限制、scan limit。
- 心跳、重连、server close、clean close 都应进入本地 JSONL audit，便于 smoke
  诊断。
- 改动 Bridge daemon 后至少跑：

```bash
make mcp-bridge-check
```

需要真实并发验证时跑：

```bash
MCP_BRIDGE_SMOKE_CONCURRENCY=4 \
MCP_BRIDGE_SMOKE_ITERATIONS=1 \
MCP_BRIDGE_SMOKE_TIMEOUT=120000 \
make mcp-bridge-smoke
```

## 6. OpenAPI MCP 规则

- OpenAPI import preview 只能预览，不持久化；import 才写入 MCP tool 与 mapping。
- schema 解析、ref 展开、去重统计放在 `pkg/mcp/openapi`，service 只做编排和
  DTO 映射。
- 非文本 response 必须走 binary object store，MCP 文本结果只返回摘要和
  metadata，不把二进制正文塞进 text。
- binary download 必须校验：
  owner 可下载、admin 可下载、其他用户不可下载、过期对象不可下载。
- TTL、清理、download URL、object metadata 变动要覆盖 service 测试。
- request body 的 JSON、form、multipart、binary/base64 策略要和 parser 生成的
  input schema 一致。

常用测试：

```bash
go test ./pkg/mcp/openapi ./service \
  -run 'TestPreviewMCPOpenAPIForAdmin|Test.*OpenAPI|TestDownloadMCPOpenAPIBinaryObject' \
  -count=1 -timeout=120s
```

## 7. MCP 计费与流水规则

- MCP tool call 的预扣费、结算、退款必须成对可追踪。
- executor 前置失败、鉴权失败、proxy discovery 失败不应产生错误扣费。
- 任何扣费路径要检查：
  `mcp_tool_calls`, `billing_events`, `billing_event_relations`,
  `mcp_user_daily_quota`。
- 修改计费事件 source/type/status 时，同步检查 source matrix、health、backfill、
  relation inspection。
- 失败路径测试要覆盖 refund，不只覆盖错误响应。

## 8. 前端 Dashboard 规则

- 默认前端是 `web/default`；使用 Bun，组件遵循 `web/default/AGENTS.md`。
- 所有用户可见文案使用 `useTranslation()` 和 `t('English key')`。
- 新增 route 或 section 后检查 TanStack route tree 是否生成/引用正确。
- MCP Dashboard 的数据 panel 必须能处理空数组、缺失字段、partial response，
  不允许因为 `undefined.length` 或缺 totals 崩溃。
- 纯数据归一化逻辑优先放到 `src/features/<feature>/lib/`，再用 Node smoke 或
  单元测试覆盖。

常用验证：

```bash
cd web/default
npm run smoke:mcp-routes --silent
npm run smoke:mcp-trends --silent
npm run typecheck --silent
```

如果新增 UI 文案：

```bash
cd web/default
bun run i18n:sync
```

## 9. 测试策略

- 小改动跑精准测试；跨模块改动跑相关包测试。
- MCP / Bridge / OpenAPI / Dashboard 综合回归优先跑：

```bash
make mcp-regression
```

- 后端 MCP 相关基础回归：

```bash
go test ./pkg/mcp/... ./model ./service ./controller ./router \
  -count=1 -timeout=180s
```

- Bridge/daemon 改动至少跑 `make mcp-bridge-check`。
- 真实并发或 reconnect 改动跑 `make mcp-bridge-smoke`。
- 前端 TS/TSX 改动至少跑 `npm run typecheck --silent`，MCP Dashboard 改动加跑
  对应 smoke。
- 提交前跑：

```bash
git diff --check
git status --short --branch
```

## 10. 提交与 TODO

- `todo.md` 是当前 data-proxy 开发清单。完成一项就把对应 checkbox 标为 `[x]`。
- 每个可验证功能点单独提交，提交信息用简洁英文：
  `feat: ...`, `fix: ...`, `test: ...`, `docs: ...`, `chore: ...`。
- 不要把无关格式化、生成文件、用户 dirty 改动混进同一个提交。
- 提交前必须说明并实际跑过相关测试；无法跑的测试要在回复里说明原因。

## 11. 安全红线

- 不提交 token、secret、真实 cookie、生产 DSN。
- 不默认开放非 loopback MCP target。
- 不移除 workspace 边界和路径穿越防护。
- 不绕过 binary download owner/admin/expiry 校验。
- 不把非文本或二进制响应直接暴露给模型文本输出。
- 不修改或删除顶层 `AGENTS.md` 中声明受保护的项目身份与版权信息。
