# Data Proxy vNext 稳定版开发任务表

日期：2026-06-26

本项目基于 `new-api`。当前发布线继续保留上游 AGPLv3 开源协议、
`NOTICE`、attribution 文案和第三方许可证信息；任何密钥、证书、诊断包、
raw capture、对象存储数据都不能提交到 Git。

## 当前总目标

先交付一个单机可部署、可回滚、可诊断、可运营的 vNext 稳定版本。协议转换
长尾放到 vNext 稳定后再做独立设计和开发，不进入当前版本阻塞项。

当前版本只允许修复协议转换的生产窄回归，例如空响应、request id 缺失、
计费异常、trace 缺失、已上线兼容路径失败。不要在当前版本继续扩展
`file_search`、`web_search` 执行器、`computer_use`、`code_interpreter`、
`image_generation`、hosted `mcp`、`shell` 或本地 runtime executor。

## 立即开发顺序

### P0：工作区收口和发布基线

目标：把当前工作区整理成可发布候选版本，确保能部署、能回滚、能排障。

任务：

- 继续使用 `scripts/data-proxy-worktree-audit.sh` 区分当前开发线和协议停车场。
- 协议转换长尾文件保持暂缓，不进入当前发布提交。
- 提交前只 staged 当前功能线相关文件，避免 i18n、路由生成文件和协议文件混入。
- 运行 `scripts/data-proxy-release-gate.sh --scan-all`。
- 确认 `.env.production`、证书、微信支付私钥、API Key、诊断包、capture 数据、
  SeaweedFS 或其他对象存储数据不进入 Git。
- 确认 GitHub Actions 可以构建 Docker 镜像，并记录 tag、digest、上一版镜像、
  回滚命令。
- 部署后做生产 smoke，并记录 `/api/status`、Chat、Responses、request trace、
  诊断包、同模型备用渠道切换、Tunnel、`dpa status --json` 的结果。

验收：

- 服务器运行最新镜像。
- 有上一版镜像和明确回滚命令。
- smoke 记录包含镜像 digest、request id 和关键接口结果。

### P1：渠道故障切换和熔断配置

目标：两个上游渠道提供同一模型时，一个临时故障后可以自动切换到备用渠道。

任务：

- 管理后台提供 retry 次数、临时故障状态码、临时故障关键词、硬故障规则、
  熔断阈值、统计窗口、冷却时间、最大冷却时间和手动恢复入口。
- 临时故障默认覆盖 408、429、5xx、上游超时、连接重置、provider 暂不可用。
- 硬故障默认覆盖认证失败、无效 key、账号禁用、权限不足等不应盲目重试的错误。
- request trace 显示初选渠道、失败渠道、失败原因、retry index、剩余重试、
  备用渠道、熔断动作和最终渠道。
- 补测试：同模型两条渠道，第一条临时失败后命中第二条；硬故障不无限重试。

验收：

- 不需要手动关闭坏渠道，也能自动切到同模型备用渠道。
- 管理员能从 trace 解释为什么重试、为什么不重试、为什么熔断。

### P2：用户绑定分组和 Key 分组限制

目标：用户可被限制到一个或多个模型分组；未绑定表示不限制。

任务：

- 用户绑定分组支持空、单个、多个。
- 创建 Key 时只能选择该用户可用分组。
- 修改 Key 时不能改到用户不可用分组。
- 模型列表只返回用户和 Key 都可用的分组模型。
- 实际 relay 再做一次服务端校验，防止绕过 UI。
- 绑定前创建、现在不可用的 Key 显示“分组不可用”，并阻止继续调用。

验收：

- 用户绑定分组后不能创建、查看或调用分组外模型。
- 旧 Key 的不可用状态在 UI 和 API 调用中都一致。

### P3：请求诊断、capture 和排障闭环

目标：生产问题能从控制台按时间范围、异常类型和 request id 定位，不依赖只看
SSH 日志。

任务：

- README 或排障文档补 request trace API/UI、诊断候选列表、诊断包下载说明。
- common logs 的 request id 可以直接跳转 trace 详情或按 request id 过滤。
- 诊断候选列表支持时间范围、模型、渠道、状态、failover、capture、
  conversion 异常筛选。
- 诊断包默认脱敏，并和 raw capture 严格隔离。
- capture 默认关闭，只能按用户、token、模型、渠道、路径、时间窗口、
  异常类型窄范围开启。
- 验证 capture spool 重启恢复、finalizer 单进程 retry backoff、cleanup、
  流式 fail-open、字节上限和截断标记。

验收：

- 管理员能发现异常 request id、生成诊断包、下载后本地离线分析。
- capture 失败不会影响用户正常请求和流式输出。

### P4：Tunnel、MCP Gateway 和 `dpa` 单机产品化

目标：用户能创建自己的 Tunnel App，使用 `dpa` 暴露本地 MCP/HTTP/TCP 服务，并
看到审计和基础计费。

任务：

- Tunnel App UI 展示用户专属连接、随机公开前缀、删除、吊销、在线状态。
- MCP Gateway 记录 tool/resource/prompt 的允许、拒绝、原因、session id、
  connection id 和 route id。
- HTTP Tunnel 压测 WebSocket、SSE、大文件上传下载、客户端断开和孤儿请求清理。
- TCP Tunnel 保持 TCP-over-WebSocket MVP，暂不做公网 raw TCP 端口池。
- Tunnel 计费先做单机风险控制：postpaid 幂等结算、可选余额预检、
  可选并发限制、denied/failed/charged 审计。
- `dpa` 提供 install/enroll/route add/route test/status/doctor 的控制台命令，
  并支持 `--json` 输出。

验收：

- 一个用户可以从控制台完成创建 App、注册 `dpa`、添加 route、测试 route、
  查看审计和基础计费。

### P5：支付、价格和计费核对

目标：充值、折扣、模型价格和实际扣费一致，管理员能追溯费用来源。

任务：

- 核对微信 Native 支付配置、二维码、回调、轮询确认和余额入账。
- 支付待确认模态框不能被误点关闭。
- 折扣最低充值金额在 UI 中明确说明：达到或超过门槛后如何计算折扣。
- 价格页展示理论价格和最近平台实际成交均价。
- billing event 记录 request id、模型、渠道、用量、折扣、最终金额和来源。
- Tunnel/MCP 计费事件进入统一审计矩阵。

验收：

- 用户支付成功后余额自动更新。
- 管理员能按 request id 或 billing event 追溯扣费。

### P6：训练数据审核和私密数据湖

目标：保存请求/响应用于未来模型训练，但必须先有隐私、审核、导出和保留周期。

任务：

- raw request/response bundle 存到轻量对象存储，优先 SeaweedFS/S3-compatible，
  不自研对象存储。
- 流式数据异步分片写入，设置字节上限，超限只截断捕获数据，不影响主请求。
- 训练样本统一 Chat/Responses schema，记录 request id、用户/租户、模型、渠道、
  脱敏版本、source hash、review status。
- 管理员 UI 支持样本预览、批准、拒绝、dataset version 和 approved-only export。
- 导出先使用版本化 `jsonl.zst`。

验收：

- raw capture 可安全落盘或落对象存储。
- 导出的训练数据只包含已审核样本，并能追溯到来源 request id。

### P7：发布和安装包打磨

目标：核心单机功能稳定后，再打磨安装、升级和跨平台发布体验。

任务：

- Docker 镜像继续作为服务器主发布路径。
- 保留历史镜像 tag、digest 和回滚记录。
- `dpa` release manifest、sha256 校验由 CI 生成。
- Homebrew、deb、rpm、MSI、Windows service helper、macOS notarization、
  系统 keychain/secret store、PTY 终端体验放到核心功能稳定后继续完善。

验收：

- 服务器升级和回滚可重复执行。
- `dpa` 安装包、升级包可校验、可追溯。

## 暂缓到 vNext 稳定后

- 协议转换长尾 executor 和 hosted tool 执行语义。
- `file_search`、完整 `web_search` 执行器、`computer_use`、`code_interpreter`、
  `image_generation`。
- hosted `mcp` 自动桥接到本地 MCP Gateway。
- `shell` 或任何本地 runtime executor。
- 多节点 Data Proxy。
- 分布式 Tunnel 路由、跨节点 SSE、分布式限流和共享带宽结算。
- 公网 raw TCP listener、端口池和连接复用。

## 推荐验证命令

工作区审计：

```bash
TMPDIR=/tmp scripts/data-proxy-worktree-audit.sh
TMPDIR=/tmp scripts/data-proxy-worktree-audit.sh --staged
git diff --check
```

发布门禁：

```bash
TMPDIR=/tmp scripts/data-proxy-release-gate.sh --scan-all
TMPDIR=/tmp scripts/data-proxy-release-gate.sh --with-tests
TMPDIR=/tmp scripts/data-proxy-release-gate.sh --with-docker-config
```

聚焦回归：

```bash
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
scripts/data-proxy-focused-regression.sh --all --frontend
```

协议转换文件只有在生产窄回归修复时才跑：

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

## 下一步执行建议

1. 先完成 P0：工作区拆分、release gate、Docker CI、生产部署和 smoke 记录。
2. 接着完成 P1/P2：渠道自动切换和用户绑定分组，这是当前线上体验最直接的问题。
3. 再完成 P3：request trace、诊断包、capture 安全性，确保后续线上问题能快速定位。
4. 然后推进 P4/P5：Tunnel/`dpa` 单机产品化和支付计费核对。
5. 最后做 P6/P7：训练数据审核、私密数据湖和安装包发布打磨。
