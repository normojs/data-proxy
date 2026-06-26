# Data Proxy 当前后续开发任务

日期：2026-06-26

本项目基于 `new-api`，后续开发、发布和部署都必须继续保留上游
AGPLv3 开源协议、`NOTICE`、attribution 文案和第三方许可证信息。

## 当前决策

当前版本先做可部署、可回滚、可诊断的单机生产版本。

当前版本的可执行排序以
`docs/data-proxy-vnext-without-protocol-longtail-plan.md`、
`docs/data-proxy-next-iteration-task-plan.md`、
`docs/data-proxy-follow-up-task-board.md` 和
`docs/data-proxy-current-release-execution-plan.md` 为准；本文保留更详细的
任务拆分、验证命令和工作区拆分参考。

协议转换长尾能力不进入当前版本，也不作为当前迭代阻塞项。当前只保留
已经实现的安全策略和生产回归修复：

- 原生 Responses 渠道继续走原生 Responses；
- Chat-only 渠道继续使用当前 Responses/Chat 兼容转换；
- OpenAI hosted tool 在 Chat-only 转换路径上继续按渠道策略过滤、拒绝、
  要求原生 Responses，或使用已显式配置的 webhook `web_search` MVP；
- 只修复会导致空响应、计费错误、日志缺失、生产请求失败的回归问题。

当前版本不开发这些协议转换长尾：

- `file_search`；
- `computer` / `computer_use`；
- `code_interpreter`；
- `image_generation`；
- hosted `mcp` 到本地 MCP Gateway 的自动转换；
- `shell` 或任何本地执行器语义；
- 新的 hosted-tool executor 框架扩展。

这些能力放到 vNext 稳定发布之后再单独规划，届时需要重新设计安全策略、
执行器边界、审计、授权、费用和用户确认流程。

## 开发顺序

当前版本的开发顺序只围绕可部署、可诊断、可运营的单机版本。协议转换长尾
不占用当前版本排期；如果工作区已有协议转换相关改动，只允许做回归守护、
测试固化、文档边界说明和明显生产 bug 修复。

### P0：发布基线和生产可部署

目标：先拿到一个能放心部署、能快速回滚、能排障的版本。

任务：

- 梳理当前未提交工作区，按功能线拆分提交。
- 每个提交前运行 release gate 和对应聚焦测试。
- 确认不提交 `.env.production`、证书、微信商户私钥、API Key、诊断包、
  capture 原始数据、本地对象存储数据。
- 确认 GitHub Actions 可以构建生产 Docker 镜像。
- 记录镜像 tag、digest、上一版回滚镜像和回滚命令。
- 部署到单机服务器后跑生产 smoke：
  - `/api/status`；
  - `/v1/chat/completions`；
  - `/v1/responses` 原生路径和 Chat-only 转换路径；
  - common logs 能看到 request id；
  - request trace 和 diagnostic bundle 可用；
  - 两条同模型渠道的故障切换可复现；
  - Tunnel 连接列表和一次 `dpa status --json` 报告；
  - 如启用支付，补一次充值/回调 happy path。

验收：

- 最新镜像已部署，旧镜像可一条命令回滚。
- 生产 smoke 有 request id、镜像 digest 和结果记录。
- 当前版本没有新增协议转换长尾范围。

### P1：渠道故障切换和用户分组限制

目标：让模型路由可控，坏渠道能自动切换，同一用户能被限制到指定模型分组。

任务：

- 完成渠道故障切换和熔断配置 UI 的中文化与可理解性：
  - retry 次数；
  - 临时故障规则；
  - 硬故障规则；
  - 阈值、窗口、冷却时间、最大冷却时间；
  - 管理员手动恢复。
- 验证同模型两条渠道时，临时 5xx/429 等错误能按配置切到备用渠道。
- 验证 401、认证失败、额度耗尽等硬故障不会被错误地无限重试。
- request trace 中展示选中渠道、失败渠道、重试决策和熔断动作。
- 完成用户绑定模型分组闭环：
  - 用户分组为空时保持不限制；
  - 用户可绑定一个或多个分组；
  - 创建 Key 时只能选择用户可用分组；
  - 使用 Key 时只能访问用户可用分组；
  - 绑定前创建但现在不可用的 Key 显示“分组不可用”。

验收：

- 同模型备用渠道自动切换可在测试和生产 smoke 中证明。
- 用户绑定分组后，模型列表、Key 创建、Key 修改和实际 relay 都不会越权。

### P2：请求诊断、capture 和运维可见性

目标：生产问题不用只靠 SSH 看日志，能从控制台按时间和 request id 定位。

任务：

- 完善 request trace API/UI 文档，把 README 或排障文档补齐。
- 从 common logs request id 直接跳转到 trace 详情或过滤视图。
- 诊断候选列表支持按时间范围、异常类型、渠道、模型筛选。
- 诊断包下载保持权限控制，不在普通日志中泄露原始 prompt/response。
- capture 默认关闭，支持按用户、token、模型、渠道、路径、时间窗口、
  异常类型窄范围开启。
- 验证 capture spool 重启恢复、finalizer 单进程 retry backoff、
  cleanup、流式 fail-open 和字节截断行为。
- 明确 raw capture 与 sanitized diagnostic bundle 的差异：
  - raw capture 用于私密训练数据湖；
  - diagnostic bundle 用于排障，默认尽量脱敏；
  - 两者都不能进入 Git。

验收：

- 管理员能从“最近异常请求”列表生成诊断包并下载。
- capture 失败不会影响用户正常请求和流式响应。

### P3：Tunnel 和 `dpa` 单机产品化

目标：让早期用户可以创建 Tunnel App，安装 `dpa`，暴露本地 MCP/HTTP/TCP
服务，并能看到审计和费用。

任务：

- Tunnel App UI 展示每个用户专属连接、随机连接前缀、删除/吊销能力。
- MCP Gateway 展示允许/拒绝的 tool、resource、prompt，并按 session id
  和 connection id 聚合审计。
- HTTP Tunnel 做 WebSocket、SSE、大文件上传/下载、客户端断开压测。
- TCP Tunnel 保持 TCP-over-WebSocket MVP，先不做公网 raw TCP 端口池。
- Tunnel 计费先做单机风险控制：
  - 默认 postpaid/idempotent 结算；
  - 可选正余额/足额余额预检；
  - 可选并发硬限制；
  - denied/failed/charged 都可审计。
- `dpa` 完成单机产品化：
  - 控制台生成安装、注册、添加 route 的命令；
  - `dpa status --json`、`dpa doctor --json`、`dpa tunnel route test` 可用；
  - 显示 agent 版本、平台、健康、HTTP/TCP/MCP route 和本地 MCP 进程状态；
  - 安装包、Homebrew/deb/rpm/MSI、notarization 属于发布打磨，排在 P4。

验收：

- 一个用户能从控制台完成 Tunnel App 创建、`dpa` 注册、route 暴露、测试、
  审计查看和基础计费确认。

### P4：训练数据湖和样本审核

目标：把全量请求保存用于未来模型训练，但必须先有存储、隐私、审核和导出边界。

任务：

- 使用轻量对象存储方案保存 raw request/response bundle，优先复用
  SeaweedFS/S3-compatible provider，不自研对象存储。
- 流式响应采用异步、分片、限字节、fail-open 的写入路径。
- 建立训练样本版本：
  - 统一 Chat/Responses schema；
  - 记录 request id、用户/租户、模型、渠道、脱敏版本；
  - 输出 JSONL.zst 或 Parquet shard；
  - 导出前必须经过审核。
- 管理员 UI 支持样本预览、批准、拒绝、导出版本。
- 训练数据权限和保留周期要单独配置，默认不暴露给普通管理员。

验收：

- raw capture 可以安全落盘或落对象存储。
- 训练导出只包含已审核样本，并可追溯到来源 request id。

### P5：发布和安装包打磨

目标：核心单机功能稳定后，再提升分发体验。

任务：

- Docker 镜像继续作为服务器主发布路径。
- 保留历史镜像 tag 和回滚记录。
- `dpa` 发布 manifest、sha256 校验和镜像/二进制产物保持可追溯。
- Homebrew tap 自动化、deb/rpm、MSI、Windows service helper、macOS
  notarization 和系统 keychain/secret store 继续打磨。

验收：

- 用户可以用稳定命令安装或升级 `dpa`。
- 服务器可以按 runbook 部署、升级、回滚。

## 暂不进入当前版本

- 协议转换长尾和本地/hosted tool 执行器扩展。
- 多节点 Data Proxy。
- 跨节点 Tunnel SSE 路由。
- 分布式 Tunnel 限流、带宽结算和会话状态。
- 公网 raw TCP listener、端口池和连接复用。
- 企业级集中下发 `dpa` 配置。

## 下一步执行清单

1. 先做 P0：拆提交、跑 release gate、确认 CI/Docker、部署并 smoke。
2. 再做 P1：渠道 failover/circuit breaker 和用户分组限制的 UI/测试收口。
3. 然后做 P2：request trace、诊断候选、诊断包和 capture 安全性打磨。
4. 接着做 P3：Tunnel/dpa 的单机可用性和计费风险控制。
5. 最后做 P4/P5：训练数据湖审核流和安装包发布体验。

不在这条队列里继续推进协议转换长尾。相关需求统一进入 vNext 稳定发布
之后的独立方案评审。

推荐每次提交前运行：

```bash
scripts/data-proxy-release-gate.sh
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
```

P1/P2/P3 聚焦验证：

```bash
scripts/data-proxy-focused-regression.sh
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
```

需要带前端类型检查时：

```bash
scripts/data-proxy-focused-regression.sh --all --frontend
```

## 当前验证记录

2026-06-26：

- 新增 `scripts/data-proxy-focused-regression.sh`，用于按 P1/P2/P3 维度
  跑渠道故障切换、用户分组限制、请求诊断、capture、训练数据 API、
  Tunnel、MCP Gateway、`dpa`、Bridge policy 和 HTTP/TCP Tunnel 回归。
- 已通过脚本语法检查：

```bash
bash -n scripts/data-proxy-focused-regression.sh scripts/data-proxy-release-gate.sh scripts/data-proxy-production-smoke.sh
```

- 已通过默认 P1/P2/P3 聚焦回归：

```bash
scripts/data-proxy-focused-regression.sh
```

- 训练数据 review UI 接入控制台后，已通过 P2 回归和前端类型检查：

```bash
scripts/data-proxy-focused-regression.sh --p2 --frontend
```

- 已通过 P3 Tunnel/MCP/dpa 聚焦回归：

```bash
scripts/data-proxy-focused-regression.sh --p3
```

- 已通过默认发布门禁：

```bash
scripts/data-proxy-release-gate.sh
```

- 已通过轻量发布门禁、生产 compose 配置校验和 whitespace 检查：

```bash
scripts/data-proxy-release-gate.sh
scripts/data-proxy-release-gate.sh --with-docker-config
git diff --check -- scripts/data-proxy-focused-regression.sh docs/data-proxy-current-development-tasks.md docs/data-proxy-near-term-development-plan.md docs/data-proxy-single-node-development-roadmap.md docs/openai-hosted-tools-support-plan.md
```

## 剩余工作区拆分顺序

当前工作区仍有大量 P1/P2/P3 功能改动。后续不要把它们一次性混进一个
commit，建议按下面顺序拆分和验证。

### Commit A：P1 渠道故障切换和用户分组限制

范围：

- `service/channel*`、`service/group.go`、`service/channel_health.go`、
  `service/channel_failover_admin.go`；
- `controller/channel*`、`controller/model*`、`controller/token*`、
  `controller/user*`、`controller/group.go`；
- `model/channel*`、`model/user*`、`model/token.go`、`model/ability.go`；
- `middleware/distributor.go`、`middleware/auth.go`；
- `setting/operation_setting/status_code_ranges.go`；
- `web/default/src/features/channels/`、`keys/`、`users/` 相关分组 UI；
- `docs/channel-failover-and-circuit-breaker.md`。

验证：

```bash
scripts/data-proxy-focused-regression.sh --p1
```

### Commit B：P2 请求诊断、request trace、capture 安全性

范围：

- `controller/log_diagnostic.go`、`controller/log_request_trace_test.go`、
  `router/request_diagnostic_routes_test.go`；
- `service/request_capture_*`、`model/request_capture.go`；
- `web/default/src/features/usage-logs/` 诊断候选和详情入口；
- `docs/request-trace-troubleshooting.md`；
- `docs/request-capture-diagnostics-architecture.md`；
- `docs/request-capture-diagnostics-implementation-plan.md`。

验证：

```bash
scripts/data-proxy-focused-regression.sh --p2
```

### Commit C：P2/P3 训练数据 review API 和 UI

范围：

- `controller/training_dataset.go`、`controller/training_dataset_test.go`；
- `service/request_training_corpus.go`、
  `service/training_dataset_export.go`、
  `service/training_sample_preview.go`；
- `web/default/src/features/training-data/`。

验证：

```bash
go test ./service -run 'Test(BuildTrainingCorpus|Training)' -count=1
go test ./controller -run 'Test(Training|DownloadTrainingDataset|GetTrainingSample|ReviewTrainingSample)' -count=1
```

### Commit D：P2 Tunnel、MCP Gateway、HTTP/TCP Tunnel、`dpa`

范围：

- `service/tunnel*`、`controller/tunnel*`、`router/tunnel-router.go`；
- `dto/bridge.go`、`dto/mcp.go`、`pkg/bridgepolicy/`、
  `pkg/mcpgateway/`；
- `pkg/dpagent/`；
- `web/default/src/features/mcp/`；
- `docs/tunnel-apps-architecture.md`、
  `docs/data-proxy-agent-cli-design.md`。

验证：

```bash
scripts/data-proxy-focused-regression.sh --p3
```

### Commit E：可选清理，协议转换回归守护，不做长尾

范围：

- `dto/openai_response.go`；
- `service/openaicompat/`；
- `relay/responses_via_chat.go`；
- `relay/channel/openai/relay_responses*.go`；
- `service/hosted_tool_executor.go` 仅保留当前 webhook `web_search` MVP；
- `docs/23-responses-chat-compatibility.md`；
- `docs/responses-chat-completions-conversion-plan.md`；
- `docs/openai-hosted-tools-support-plan.md`。

边界：

- 不新增 `file_search`、`computer_use`、`code_interpreter`、
  `image_generation`、hosted `mcp`、`shell` 执行器。
- 不以协议转换为当前版本功能开发主线。
- 只在已经触碰到相关文件、或生产请求出现明确回归时，保留已有安全策略、
  诊断元数据、空响应/异常流修复和 webhook `web_search` MVP。
- 如果没有必须提交的修复，这个 commit 可以跳过，直接进入发布和部署验证。

验证：

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

### Commit F：dpa 发布资产和安装包打磨

范围：

- `.github/workflows/data-proxy-agent.yml`；
- `scripts/generate-data-proxy-agent-manifest.sh`；
- `scripts/install-data-proxy-agent.sh`；
- `docs/data-proxy-agent-cli-design.md` 中发布/manifest 相关段落。

验证：

```bash
bash -n scripts/generate-data-proxy-agent-manifest.sh scripts/install-data-proxy-agent.sh
```

### Commit G：价格展示、README、环境样例和杂项

范围：

- `README.md`、`.env.example`、`makefile`；
- `controller/pricing.go`、`model/pricing.go`、`service/pricing_actual.go`；
- `web/default/src/features/pricing/`；
- `tools/fusion-benchmark.*` 的剩余差异需要单独确认是否仍属于当前版本。

验证：

```bash
go test ./controller ./service -run 'Test.*Pricing|TestGetPlatformPricingActualPrices' -count=1
```
