# Data Proxy 后续开发计划

日期：2026-06-28

本项目基于 `new-api`。所有后续开发、发布、部署和文档必须继续保留
`new-api` 上游 AGPLv3 协议、`NOTICE`、attribution 文案和第三方许可证信息。
不要提交 `.env.production`、证书、微信商户私钥、API Key、诊断包、raw capture
bundle、对象存储数据或本地运行数据。

## 当前基线

已完成：

- 本地同模型渠道故障切换 smoke：`d2ad15b7 test: add local channel failover smoke`。
- 本地 smoke 可以用临时 SQLite 和两个本地 httptest 上游验证：
  - 坏渠道返回 502；
  - 系统记录 `retry_planned=true`；
  - 自动切到备用同模型渠道；
  - request trace 和 diagnostic candidates 都能看到 `channel_failover` 证据。

协议转换长尾继续暂缓。当前阶段只允许做生产窄回归修复，不新增
`file_search`、`computer_use`、`code_interpreter`、`image_generation`、
hosted MCP 自动桥接或通用本地执行器。

## 执行顺序

### P0：发布基线收口

目标：先保证当前单机版本可部署、可回滚、可诊断。

任务：

- 推送当前已提交内容到 `normojs/data-proxy.git`。
- 确认 GitHub Actions 能构建生产 Docker 镜像。
- 本地或 CI 产出镜像后，按本地上传部署流程更新服务器。
- 记录镜像 tag、digest、上一版镜像、回滚命令和 smoke 结果。
- 跑生产 smoke：
  - `/api/status`；
  - Chat Completions；
  - Responses 原生或兼容路径；
  - request trace；
  - diagnostic candidates / bundle；
  - 同模型渠道 failover；
  - Tunnel / `dpa status --json`，如果本次部署包含相关改动。

验收：

- 服务器运行最新镜像。
- 有明确回滚命令。
- smoke 记录包含 request id、镜像 tag 和 digest。

### P1：渠道故障切换生产化

目标：解决“两个上游渠道使用同一个模型，其中一个坏了不能自动切备用”的生产问题。

任务：

- 将本地 smoke 纳入常用发布验证清单。
- 增加生产同模型 failover smoke 的固定配置说明和一次真实演练记录。
- 检查管理 UI 的中文配置：
  - Retry Times；
  - 临时故障状态码/关键词；
  - 硬故障状态码/关键词；
  - 熔断阈值、窗口、冷却时间、最大冷却时间；
  - 手动清除临时熔断。
- 明确推荐规则：
  - `429`、`5xx`、超时优先作为临时故障；
  - `401`、认证失败、账号不可用优先作为硬故障；
  - `RetryTimes >= 1` 才能尝试备用渠道。
- 在 request trace / usage log detail 中继续完善失败渠道、最终渠道、重试原因、
  熔断动作的展示。

验收：

- 本地 smoke 通过。
- 生产 smoke 能证明坏渠道到备用渠道的切换。
- 管理员能通过 request id 看到完整 failover 证据。

### P2：用户分组和 Key 权限

目标：用户绑定分组后，只能看到、创建和使用允许分组内的模型和 Key。

任务：

- 用户绑定分组支持空值、单分组和多分组。
- 空值表示不限制，保持兼容。
- 创建 Key 时只能选择用户允许的分组。
- 修改 Key 时不能切到用户不允许的分组。
- 绑定前创建但现在越权的 Key 显示“分组不可用”。
- 模型列表、实际调用、日志筛选和错误提示都使用一致的分组判断。

验收：

- 普通用户不能创建或调用分组外模型。
- 管理员可以清空用户绑定分组恢复不限制。
- 老 Key 的不可用状态可见且错误提示清楚。

### P3：请求诊断和 capture 安全性

目标：生产异常能从控制台按时间、模型、渠道、request id 定位，并能下载诊断包离线分析。

任务：

- 在 README 或排障文档补充 request trace API/UI 和诊断包下载说明。
- usage logs 的 request id 支持直接跳转详情或自动过滤。
- 诊断候选列表继续完善筛选：
  - 时间范围；
  - 模型；
  - 渠道；
  - 分组；
  - 异常类型；
  - failover；
  - capture；
  - protocol conversion metadata。
- 继续验证 capture：
  - spool 重启恢复；
  - finalizer 单进程 retry backoff；
  - cleanup；
  - 流式 fail-open；
  - 字节上限和截断标记。
- raw capture 和 sanitized diagnostic bundle 严格隔离。

验收：

- 管理员能从异常候选生成并下载诊断包。
- capture 失败不影响用户正常请求和流式响应。
- 诊断包不泄露未授权 raw 数据。

### P4：Tunnel / MCP Gateway / `dpa` 单机产品化

目标：早期用户可以通过控制台创建 Tunnel App，安装 `dpa`，暴露本地 HTTP/TCP/MCP
能力，并查看审计和基础计费。

任务：

- Tunnel App UI 展示用户专属连接、随机前缀、删除/吊销、route 和在线状态。
- MCP Gateway 展示 tool/resource/prompt 的允许、拒绝和原因。
- HTTP Tunnel 继续压测：
  - WebSocket；
  - SSE；
  - 大文件上传/下载；
  - 客户端断开；
  - 字节计数；
  - close reason。
- TCP Tunnel 保持 TCP-over-WebSocket MVP。
- Tunnel 计费先做单机风险控制：
  - postpaid 幂等结算；
  - 可选余额预检；
  - 可选并发限制；
  - denied/failed/charged 审计。
- `dpa` 继续补齐：
  - install；
  - enroll；
  - route add/test；
  - `status --json`；
  - `doctor --json`；
  - manifest / sha256。

验收：

- 用户能完成 create app -> enroll `dpa` -> add route -> test route -> 查看审计。
- 单机计费事件可追溯且不会重复扣费。

### P5：训练数据和样本审核

目标：请求/响应数据可进入私密存储和审核流程，但导出必须 approved-only。

任务：

- 使用 SeaweedFS 或 S3-compatible provider 保存 raw bundle，不自研对象存储。
- 流式数据异步、分片、限字节、fail-open 写入。
- 统一 Chat / Responses 训练样本 schema。
- 记录 request id、用户/租户、模型、渠道、脱敏版本、source hash。
- 管理员 UI 支持 dataset build、样本预览、批准、拒绝、approved-only export。
- 导出格式先使用版本化 `jsonl.zst`。

验收：

- 导出只包含已审核样本。
- 每条样本能追溯到 request id 和脱敏版本。

### P6：发布和安装包打磨

目标：核心单机能力稳定后，继续打磨服务器发布和 `dpa` 跨平台安装体验。

任务：

- Docker 镜像继续作为服务器主发布路径。
- 保留历史镜像 tag、digest 和回滚记录。
- `dpa` release manifest 和 sha256 由 CI 生成。
- 后续补 Homebrew、deb、rpm、MSI、Windows service helper、macOS notarization。
- 打磨系统 keychain/secret store 和 PTY 终端体验。

验收：

- 服务器可按 runbook 部署、升级和回滚。
- `dpa` 安装包和升级包可校验、可追溯。

## 固定验证命令

本地 failover smoke：

```bash
scripts/data-proxy-local-channel-failover-smoke.sh
```

工作区检查：

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

仅当生产窄回归触碰协议转换文件时，才跑：

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

## 下一步建议

1. 推送 `d2ad15b7` 和本计划文档提交。
2. 触发 GitHub Actions 构建镜像。
3. 部署服务器并做 P0 smoke。
4. 开始 P1：生产同模型 failover 演练和 UI/trace 细节收口。
