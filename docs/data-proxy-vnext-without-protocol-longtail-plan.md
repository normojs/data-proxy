# Data Proxy vNext 前后续任务规划

日期：2026-06-26

本项目基于 `new-api`。后续开发、发布、部署和文档都必须继续保留
`new-api` 上游 AGPLv3 开源协议、`NOTICE`、attribution 文案和第三方许可证
信息。

## 当前范围决策

协议转换长尾放到 vNext 发布稳定之后再执行。当前阶段先忽略，不作为开发、
验收、部署和提交拆分的阻塞项。

当前阶段允许保留或修复的协议转换内容只有生产窄回归：

- 已有 Chat-only 渠道的 Responses/Chat 兼容路径不能退化；
- 已有原生 Responses 渠道继续原生转发；
- 空响应、request id 缺失、计费异常、诊断信息缺失可以修；
- 已经上线的安全策略、拒绝策略、日志字段可以补测试和文档。

当前阶段不做：

- `file_search`、`web_search` 更完整执行器；
- `computer` / `computer_use`；
- `code_interpreter`；
- `image_generation`；
- hosted `mcp` 自动桥接到本地 MCP Gateway；
- `shell` 或其他本地 runtime executor；
- provider-specific 长尾 SSE 事件模拟；
- 通用 hosted-tool executor 框架扩展。

这些能力统一进入 vNext 稳定后的独立评审，届时需要重新讨论安全边界、
用户授权、审计、计费、沙盒和失败兜底。

## 已完成到当前节点

- 已完成 vNext 范围边界文档：协议转换长尾进入 vNext 稳定后的停车场。
- 已完成 request diagnostic console 基础流程。
- 已完成 Tunnel Gateway、MCP Gateway、`dpa` agent 的单机基础能力加固。
- 已完成训练数据 review console：dataset build、样本预览、approve/reject、
  approved-only export 的控制台入口。
- 已完成价格页平台实际成交均价展示和 billing event source matrix 补充。

这些已完成项后续只做生产回归修复、文档补充和部署 smoke，不再作为当前
工作区拆分主线重复开发。

## 当前开发顺序

### P0：发布基线和工作区收口

目标：先做一个可以部署、可以回滚、可以排障的单机版本。

任务：

- 把当前未提交工作区按功能线拆开，避免一个提交混入多条产品线；
- 协议转换长尾相关文件暂不提交，除非是生产窄回归修复；
- 每次提交前运行 staged audit、release gate、focused regression；
- 确认 `.env.production`、证书、微信商户私钥、API Key、诊断包、raw capture、
  对象存储数据不进入 Git；
- 确认 GitHub Actions 可以构建生产 Docker 镜像；
- 部署后记录镜像 tag、digest、上一版镜像和回滚命令；
- 做生产 smoke：`/api/status`、Chat、Responses、request trace、诊断包、
  同模型备用渠道切换、Tunnel 连接、`dpa status --json`。

验收：

- 服务器运行最新镜像；
- 有明确回滚命令；
- smoke 记录包含 request id 和镜像 digest。

### P1：渠道自动切换和用户分组限制

目标：解决两个上游同模型渠道中一个故障时不能自动切换的问题，同时完成用户
绑定分组和 Key 限制。

任务：

- 完成渠道 retry、临时故障、硬故障、熔断阈值、窗口、冷却时间、手动恢复 UI；
- 管理员可以配置哪些错误进入临时故障、哪些错误属于硬故障；
- 5xx、429、上游超时等临时故障按配置切到同模型备用渠道；
- 401、认证失败、账号不可用等硬故障不无限重试；
- request trace 展示失败渠道、重试决策、最终渠道和熔断动作；
- 用户分组为空表示不限制；
- 用户可以绑定一个或多个分组；
- 创建 Key、修改 Key、模型列表、实际调用都遵守用户绑定分组；
- 绑定前创建但现在不可用的 Key 显示“分组不可用”。

验收：

- 两条同模型渠道的备用切换有测试和生产 smoke 证据；
- 用户绑定分组后不能创建或使用分组外模型。

### P2：请求诊断、capture 和训练数据审核

目标：生产问题可以从控制台按时间、异常类型和 request id 定位；请求数据可以
进入私密存储和审核流程，但不能影响主请求。

任务：

- README 或排障文档补 request trace API/UI 和诊断包下载说明；
- common logs 的 request id 可以直接跳转 trace 或过滤日志；
- 增加诊断候选列表：时间范围、模型、渠道、状态、failover、capture 标记；
- 诊断包默认脱敏，和 raw capture 严格隔离；
- capture 默认关闭，只能按用户、token、模型、渠道、路径、时间窗口、
  异常类型窄范围开启；
- 验证 capture spool 重启恢复、finalizer 单进程 retry backoff、cleanup；
- 验证流式捕获 fail-open、字节上限、截断标记；
- 训练数据 review UI 已完成，当前只补生产 smoke 和导出边界文档；
- 导出先使用版本化 `jsonl.zst`，后续数据湖增强放到 P4/P5 之后。

验收：

- 管理员能从异常请求生成并下载诊断包；
- capture 失败不会影响用户请求和流式输出；
- 训练导出只包含已审核样本，并能追溯到 request id。

### P3：Tunnel、MCP Gateway 和 `dpa` 单机产品化

目标：早期用户可以创建 Tunnel App，安装 `dpa`，暴露本地 MCP/HTTP/TCP 服务，
并看到审计和基础计费。

任务：

- Tunnel App UI 展示用户专属连接、随机前缀、删除/吊销、route 和在线状态；
- MCP Gateway 展示 tool/resource/prompt 的允许、拒绝和原因；
- 审计按 session id、connection id、route id 聚合；
- HTTP Tunnel 覆盖 WebSocket、SSE、大文件上传下载、客户端断开；
- TCP Tunnel 保持 TCP-over-WebSocket MVP；
- Tunnel 计费先做单机风险控制：postpaid 幂等结算、可选余额预检、
  可选并发限制、denied/failed/charged 审计；
- `dpa` 控制台给出安装、注册、添加 route、测试 route 命令；
- `dpa status --json`、`dpa doctor --json`、`dpa tunnel route test` 可用。

验收：

- 用户可以从控制台完成 Tunnel App 创建、`dpa` 注册、route 暴露、测试、
  审计查看和基础计费确认。

### P4：支付、定价和计费核对

目标：在线充值、模型价格展示和实际扣费一致，Tunnel/MCP 等新能力的费用来源
可审计。

任务：

- 核对微信 Native 支付配置、回调、余额入账和未点击完成时的轮询体验；
- 支付模态框在待支付状态下不能被误点关闭；
- 价格页已展示理论价格和最近平台实际成交均价；
- 计费事件已补 request id、来源、模型、渠道、用量、折扣和最终金额字段；
- Tunnel/MCP 计费事件已进入统一 billing event source matrix；
- 明确折扣最低充值金额规则，并在 UI 上用中文说明。

验收：

- 支付成功后余额可自动确认；
- 用户看到的价格、折扣和实际扣费一致；
- 管理员可以按 request id 或 billing event 追溯费用来源。

### P5：发布和安装包打磨

目标：核心功能稳定后，再打磨安装、升级、回滚和跨平台分发。

任务：

- Docker 镜像继续作为服务器主发布路径；
- 保留历史镜像 tag、digest 和回滚记录；
- `dpa` release manifest 和 sha256 校验由 CI 生成；
- Homebrew、deb、rpm、MSI、Windows service helper、macOS notarization 后续补齐；
- 系统 keychain/secret store 和 PTY 终端体验继续打磨。

验收：

- 服务器可以按 runbook 部署、升级、回滚；
- `dpa` 安装包和升级包可校验、可追溯。

## 立即执行清单

1. 跑工作区审计，确认当前未提交文件属于哪个批次。
2. 先收口 P0：`README.md`、`.env.example`、CI/Docker、部署和回滚说明。
3. 单独处理 P1：渠道自动切换、熔断配置 UI、用户分组限制。
4. 单独处理 P2：request trace 文档、诊断候选列表、capture 安全性。
5. 单独处理 P3：Tunnel/MCP Gateway/`dpa` 的生产 smoke 和单机计费风险控制。
6. 单独确认 benchmark 工具是否进入当前版本；不进入则保留为后续工具改动。
7. 暂存或保留协议转换长尾改动，不进入当前发布提交。
8. 每个批次提交后跑对应 focused regression 和 release gate。
9. 部署到服务器并记录 smoke 结果、镜像 digest 和回滚命令。

## 验证命令

工作区和 staged audit：

```bash
scripts/data-proxy-worktree-audit.sh
scripts/data-proxy-worktree-audit.sh --staged
git diff --check
```

发布门禁：

```bash
scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
```

聚焦回归：

```bash
scripts/data-proxy-focused-regression.sh --p1 --frontend
scripts/data-proxy-focused-regression.sh --p2 --frontend
scripts/data-proxy-focused-regression.sh --p3 --frontend
```

只有生产窄回归触碰协议转换文件时，才跑：

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

## Post-vNext 停车场

vNext 部署并稳定后，再重新评审：

- full hosted tool capability model；
- `file_search` / retrieval bridge；
- `web_search` executor bridge 或外部搜索服务；
- `computer_use`；
- `code_interpreter`；
- `image_generation`；
- hosted MCP 到本地 MCP Gateway 的受控桥接；
- 通用 executor policy、用户确认、审计和计费；
- 多节点 Data Proxy；
- 分布式 Tunnel 路由、限流和带宽结算；
- 公网 raw TCP listener、端口池和连接复用。
