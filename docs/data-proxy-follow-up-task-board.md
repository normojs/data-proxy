# Data Proxy 后续任务看板

日期：2026-06-26

本项目基于 `new-api`。后续开发、发布和部署都必须继续保留上游
AGPLv3 开源协议、`NOTICE`、attribution 文案和第三方许可证信息。

## 当前决策

当前阶段只做单机可部署版本的收口：可回滚、可诊断、可运营。

下一阶段可执行任务入口见
`docs/data-proxy-next-iteration-task-plan.md`。本文保留完整看板、验收和
工作区拆分参考。

协议转换长尾不进入当前版本，也不进入紧接着的 vNext 主线。等 vNext
稳定发布后，再单独重新评审 hosted tool、本地执行器、MCP bridge、检索、
computer use、code interpreter、image generation 等能力。

当前版本只允许做协议转换的窄修复：

- 修复已经上线路径的空响应、异常流、计费、日志、request id、诊断问题；
- 保留原生 Responses 渠道的原生转发；
- 保留 Chat-only 渠道的现有 Responses/Chat 兼容转换；
- 保留已经显式配置并有边界的 webhook `web_search` MVP；
- 不新增新的 hosted-tool executor 或本地执行语义。

## 不进入当前队列

这些任务全部放到 vNext 稳定之后：

- `file_search`；
- `computer` / `computer_use`；
- `code_interpreter`；
- `image_generation`；
- hosted `mcp` 到 Data Proxy MCP Gateway 的自动转换；
- `shell` 或其他本地 runtime executor；
- 通用 hosted-tool executor 框架扩展；
- 多节点 Data Proxy；
- 分布式 Tunnel 路由、限流和带宽结算；
- 公网 raw TCP listener、端口池和连接复用。

## 执行顺序

### 当前执行批次

这几个批次是接下来真正要开发和验收的主线。协议转换长尾只保留在
“不进入当前队列”和“Post-vNext 停车场”里，不进入下面任一批次。

| 批次 | 优先级 | 目标 | 交付物 | 不做 |
| --- | --- | --- | --- | --- |
| RC0 | P0 | 先拿到可部署、可回滚、可诊断的单机版本 | 干净提交、CI/Docker 镜像、服务器部署、生产 smoke、回滚记录 | 不扩新协议转换能力 |
| A | P1 | 解决同模型渠道故障切换和用户分组限制 | 渠道重试/熔断 UI、request trace、用户绑定多分组、Key 分组限制 | 不做多节点路由 |
| B | P2 | 解决线上问题定位和私密捕获安全性 | request trace 快捷入口、诊断候选、诊断包下载、capture fail-open/retry/恢复 | 不默认保存所有原始数据 |
| C | P3 | 让 Tunnel/MCP Gateway/`dpa` 能给早期用户试用 | Tunnel App UI、HTTP/TCP Tunnel 单机稳定性、MCP 审计、`dpa` 安装注册命令 | 不做分布式 Tunnel |
| D | P4 | 让请求数据进入可审核训练样本流程 | 训练数据 review UI、approved-only export、样本追溯和脱敏版本 | 不做自动训练闭环 |
| E | P5 | 打磨发布和安装体验 | 历史镜像回滚、`dpa` checksum/manifest、安装包和升级链路 | 不阻塞 RC0 发布 |

立即开始顺序：

1. 先跑 P1/P2/P3 聚焦回归，确认当前大工作区里哪些已经可用。
2. 按 RC0 -> A -> B -> C -> D -> E 拆提交，避免一个提交混入多条产品线。
3. 每完成一个批次都跑对应 focused regression，再跑 release gate。
4. RC0 部署后先给服务器做 smoke；后续功能以小步升级方式继续。

### P0：发布基线

目标：先得到一个可以部署、可以回滚、出问题可以按 request id 排查的版本。

任务：

- 清理当前未提交工作区，按功能线拆提交。
- 每个提交只包含一个功能线，避免把协议转换长尾混进发布 commit。
- 跑 release gate、聚焦回归、前端类型检查。
- 确认不提交 `.env.production`、证书、微信商户私钥、API Key、诊断包、
  raw capture、对象存储数据。
- 确认 GitHub Actions 能构建生产 Docker 镜像。
- 部署到服务器，记录镜像 tag、digest、上一版回滚镜像和回滚命令。
- 跑生产 smoke：`/api/status`、Chat、Responses、request trace、
  diagnostic bundle、同模型备用渠道切换、Tunnel 连接和 `dpa status --json`。

验收：

- 服务器运行最新镜像。
- 旧镜像可以按文档回滚。
- smoke 记录包含 request id 和镜像 digest。

### P1：渠道故障切换和用户分组限制

目标：两条同模型渠道中一条临时故障时，可以自动切备用；用户绑定分组后，
模型和 Key 权限不会越界。

任务：

- 完成渠道重试、临时故障、硬故障、熔断阈值、窗口、冷却时间、手动恢复的
  管理 UI 和中文说明。
- 验证 5xx/429 等临时故障能切到备用渠道。
- 验证 401、认证失败、额度耗尽等硬故障不会无限重试。
- request trace 展示失败渠道、重试决策、最终渠道和熔断动作。
- 完成用户绑定一个或多个模型分组。
- 用户分组为空时保持不限制。
- 创建 Key、修改 Key、使用 Key、模型列表都遵守用户分组。
- 绑定前创建但现在不可用的 Key 显示“分组不可用”。

验收：

- 同模型备用渠道自动切换有测试和生产 smoke 证据。
- 被分组限制的用户不能创建或使用分组外模型。

### P2：请求诊断、capture 和排障体验

目标：生产问题可以从控制台按时间和 request id 定位，不依赖只 SSH 看日志。

任务：

- README 或排障文档写清 request trace API/UI 的用法。
- common logs 的 request id 可以直接跳详情或过滤。
- 诊断候选列表支持时间范围、模型、渠道、异常类型、转换/capture/failover
  标记。
- 管理员可以生成并下载诊断包。
- raw capture 和 sanitized diagnostic bundle 明确隔离。
- 验证 capture spool 重启恢复、finalizer retry backoff、cleanup、流式
  fail-open、字节截断。
- capture 默认关闭，只能按用户、token、模型、渠道、路径、时间窗口、
  异常类型窄范围开启。

验收：

- 管理员能从异常候选找到 request id，生成诊断报告并下载诊断包。
- capture 失败不会影响用户正常请求或流式响应。

### P3：Tunnel、MCP Gateway 和 `dpa`

目标：早期用户可以创建 Tunnel App，安装 `dpa`，暴露本地 MCP/HTTP/TCP
服务，并能看见审计和计费结果。

任务：

- Tunnel App UI 展示用户专属连接、随机前缀、删除/吊销、route、在线状态。
- MCP Gateway 展示 tool/resource/prompt 的允许、拒绝和原因。
- 审计按 session id、connection id、route id 聚合。
- HTTP Tunnel 覆盖 WebSocket、SSE、大文件上传/下载、客户端断开。
- TCP Tunnel 保持 TCP-over-WebSocket MVP。
- Tunnel 计费先做单机风险控制：postpaid 幂等结算、可选正余额检查、
  可选并发限制、denied/failed/charged 审计。
- `dpa` 控制台给出安装、注册、添加 route、测试 route 的命令。
- `dpa status --json`、`dpa doctor --json`、`dpa tunnel route test` 可用。

验收：

- 用户可以从控制台完成创建 Tunnel App、注册 `dpa`、添加 route、测试、
  查看审计和基础计费。

### P4：训练数据湖和样本审核

目标：请求和响应可以作为未来训练数据来源，但必须经过存储、脱敏、审核、
版本和导出控制。

任务：

- 使用轻量对象存储保存 raw request/response bundle，优先 SeaweedFS /
  S3-compatible provider，不自研对象存储。
- 流式数据异步、分片、限字节、fail-open 写入。
- 建立统一训练样本 schema，覆盖 Chat 和 Responses。
- 记录 request id、用户/租户、模型、渠道、脱敏版本、source hash。
- 管理员 UI 支持 dataset build、样本预览、批准、拒绝、approved-only export。
- 导出格式先使用版本化 `jsonl.zst`。
- 保留周期、权限和可训练范围必须单独配置。

验收：

- 导出只包含已审核样本。
- 每条样本可以追溯到 request id 和脱敏版本。

### P5：发布和安装包打磨

目标：核心单机能力稳定后，再提升安装、升级、回滚和跨平台体验。

任务：

- Docker 镜像继续作为服务器主发布路径。
- 保留历史镜像 tag、digest 和回滚记录。
- `dpa` 产物生成 manifest、sha256 校验。
- 继续 Homebrew、deb、rpm、MSI、Windows service helper、macOS notarization。
- 打磨系统 keychain/secret store 和 PTY 终端体验。

验收：

- 服务器可以按 runbook 部署、升级和回滚。
- `dpa` 安装包和升级包可校验、可追溯。

## 工作区拆分建议

1. P1：渠道故障切换、熔断、用户分组、Key 限制。
2. P2：request trace、诊断包、capture 安全性、诊断 UI。
3. P4：训练数据 review API 和 UI。
4. P3：Tunnel、MCP Gateway、HTTP/TCP Tunnel、`dpa`。
5. 协议转换窄回归守护，只在确实有生产问题或已触碰文件时提交。
6. CI、README、`.env.example`、Docker、安装脚本和发布文档。

### 当前工作区拆分审计

2026-06-26 当前工作区仍较大，提交时按下面的功能线拆分，不要整包提交：

| 分组 | 代表路径 | 提交策略 |
| --- | --- | --- |
| 发布基线/CI/文档 | `.github/workflows/data-proxy-agent.yml`、`README.md`、`.env.example`、`makefile`、`scripts/*`、`docs/data-proxy-*` | 放在 RC0 提交；提交前再跑 release gate 和 secret scan。 |
| P1 渠道/分组 UI | `dto/channel_settings.go`、`web/default/src/features/channels/*` | 和渠道 failover/熔断配置一起提交；验证 P1 focused regression。 |
| P2 诊断/capture/训练 review | `docs/request-capture-*`、`web/default/src/features/training-data/*`、`web/default/src/routes/_authenticated/training-data/*`、sidebar/route/i18n 相关文件 | 和训练数据 review UI 一起提交；注意 i18n 文件可能包含其他功能翻译，必要时按 hunk staging。 |
| P3 Tunnel/MCP/`dpa` | `controller/tunnel_*`、`dto/bridge.go`、`dto/mcp.go`、`model/tunnel.go`、`pkg/dpagent/*`、`pkg/mcpgateway/*`、`router/tunnel-router.go`、`service/tunnel*`、`service/bridge*`、`web/default/src/features/mcp/*`、`docs/tunnel-apps-architecture.md`、`docs/data-proxy-agent-cli-design.md` | 单独提交；验证 P3 focused regression 和 `dpa` 包测试。 |
| 计费/定价 | `controller/pricing.go`、`model/pricing.go`、`model/*billing_event*`、`service/pricing_actual*`、`service/billing_event_source_matrix*`、`web/default/src/features/pricing/*` | 可独立提交，或并入 Tunnel billing 风控提交；需要计费相关测试。 |
| 协议转换窄回归守护 | `relay/*`、`service/openaicompat/*`、`service/hosted_tool_executor*`、`dto/openai_response*`、相关 testdata、`docs/23-responses-chat-compatibility.md`、`docs/openai-hosted-tools-support-plan.md`、`docs/responses-chat-completions-conversion-plan.md` | 当前不扩功能；只有生产窄修复或已触碰文件的回归守护才提交。 |
| benchmark 工具 | `tools/fusion-benchmark*` | 单独提交；不和发布主线混在一起。 |

提交前固定检查：

- `scripts/data-proxy-worktree-audit.sh`；
- `scripts/data-proxy-worktree-audit.sh --staged`；
- `git diff --check`；
- `scripts/data-proxy-release-gate.sh --scan-all`；
- 对应分组的 focused regression；
- 确认 `.env.production`、证书、微信商户私钥、API Key、诊断包、
  raw capture、对象存储数据没有进入 staging。

## 推荐验证命令

发布门禁：

```bash
scripts/data-proxy-release-gate.sh
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
```

聚焦回归：

```bash
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
scripts/data-proxy-focused-regression.sh --all --frontend
```

仅当触碰协议转换文件时，跑窄回归：

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

## 当前验证记录

2026-06-26 当前工作区已通过以下验证：

```bash
scripts/data-proxy-focused-regression.sh --all --frontend
scripts/data-proxy-release-gate.sh --with-docker-config
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --scan-all
```

覆盖结果：

- P1：渠道 failover、熔断状态、用户绑定分组、Key 分组限制聚焦测试通过。
- P2：request trace、诊断候选、诊断包、capture、训练数据 API 聚焦测试通过。
- P3：Tunnel、MCP Gateway、`dpa`、Bridge policy、HTTP/TCP tunnel 聚焦测试通过。
- 前端 TypeScript build mode 通过。
- 生产 compose 配置校验通过。
- 全量 release gate secret/path/license/whitespace 扫描通过。

2026-06-26 P1 UI 文案收口：

- 渠道编辑抽屉中的 Responses 协议、推理适配和托管工具策略文案已统一使用
  i18n key。
- 中文显示收口为“响应接口模式”、“托管工具策略”、“默认兼容”、
  “过滤并直答”等控制台术语。
- 源码中渠道抽屉相关中文 fallback key 已清空。
- 已通过：

```bash
PATH="/Users/fushilu/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH" ./node_modules/.bin/tsc -b
scripts/data-proxy-focused-regression.sh --p1 --frontend
```

2026-06-26 工作区清理工具：

- 新增 `scripts/data-proxy-worktree-audit.sh`，只读输出当前改动的分组：
  RC0、P1、P2/P4、P3、计费、协议窄回归、benchmark 和 mixed shared。
- `--staged` 模式用于提交前复核 staging 区是否混入错误功能线。
- mixed shared 文件会显式提示按 hunk staging，当前包括 i18n、route tree
  和 sidebar 配置。
- 已通过：

```bash
bash -n scripts/data-proxy-worktree-audit.sh
scripts/data-proxy-worktree-audit.sh
scripts/data-proxy-worktree-audit.sh --staged
git diff --check -- scripts/data-proxy-worktree-audit.sh
```

## 下一步

马上执行：

1. 按上面的拆分顺序清理工作区。
2. 先完成 P0 发布基线，部署并记录 smoke。
3. 继续 P1 渠道 failover 和用户分组限制。
4. 再进入 P2 诊断/capture 和 P3 Tunnel/`dpa`。
5. P4 训练数据审核 UI 可以并行准备，但不阻塞 P0 发布。

协议转换长尾在这条队列里只作为“不做清单”和“窄回归守护”存在。
