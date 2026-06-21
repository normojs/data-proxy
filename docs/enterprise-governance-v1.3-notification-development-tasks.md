# Enterprise Governance V1.3 Notification Development Tasks

本文档用于承接 V1.3 审批和临时额度的通知闭环后续开发。当前策略是先把最小可用的站内通知和审计可见做稳，再进入邮件、webhook 和更完整的通知投递体系。

## 目标和边界

目标：让审批申请、审批结果、即将过期、已过期这些关键状态在站内可见、可标记已读、可追踪审计，并为后续邮件和 webhook 打好事件模型基础。

本轮优先做：

- 站内审批通知 MVP 补强。
- 审计事件可见和 deep link。
- 即将过期提醒。
- 主动过期扫描和过期审计。
- 通知派生逻辑 service 化。

本轮暂不优先做：

- 邮件实时发送。
- webhook 实时投递。
- 企业级通知偏好完整 UI。
- 多渠道投递失败重试后台。

## 当前状态

已完成：

- 审批申请模型、提交、批准、拒绝、撤回、列表和基础审计。
- 临时额度已接入 hard limit 计算，已批准且未过期的申请会叠加有效上限。
- 站内通知入口已支持审批通知 tab、未读红点、已读标记和审批/审计 deep link。
- 通知读取状态已通过 `enterprise_notification_reads` 持久化。
- 审批通知派生逻辑已从 controller 抽到 service 层，站内通知分页、未读筛选、i18n key 和 deep link smoke 均已完成。

后续收口已完成：

- Webhook 管理 API/UI：管理员可新增、编辑、停用和测试发送 webhook。
- 通知偏好配置：管理员可配置 email/webhook 开关和邮件收件范围。
- 邮件和 webhook worker：均已复用 notification outbox 异步投递和重试框架。
- 投递日志和重试 UI：管理员可查看 outbox、失败摘要、worker 指标并手动重试失败记录。

后续增强已完成：

- 普通用户邮件接收偏好/退订入口已落到 Profile 通知设置；站内通知保持不可关闭。
- `expiring_soon` 外部通知已由维护任务按 24 小时窗口幂等写入 email/webhook outbox。

当前剩余不属于本地代码开发，主要是预发和生产环境的 R0-R3 演练证据回填。

## 开发原则

- 站内通知先派生，外部通知再 outbox：站内 MVP 可以继续从申请表和审计表派生；邮件和 webhook 必须走持久 outbox，避免审批 API 请求链路直接依赖外部投递。
- 审计是事实源：审批提交、批准、拒绝、撤回、过期都要有审计记录；通知可以引用审计或从业务状态派生，但不能替代审计。
- 幂等优先：过期扫描、即将过期提醒、outbox 写入和外部投递都要有稳定 key，避免多节点或重试造成重复刷屏。
- 默认只开启站内通知：邮件和 webhook 后续默认关闭，由管理员显式配置。
- 先做窄闭环再扩渠道：每个阶段都要保持可上线，不能为了后续完整架构阻塞当前站内可用性。

## Phase A: 站内通知补强

目标：审批状态在站内形成完整闭环，不依赖邮件或 webhook。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-NOTIF-001 | P0 | 通知派生 service 化 | 已有通知 MVP | `controller/notification_read.go` 只负责鉴权和响应；通知列表、key、read 合并有 service 测试。 |
| V13-NOTIF-002 | P0 | 即将过期提醒 | V13-NOTIF-001 | 已批准且 24 小时内过期的临时额度在申请人和管理员站内可见；同一申请同一过期时间只出现一条；可标记已读。 |
| V13-NOTIF-003 | P0 | 主动过期扫描 | 审批模型 | 后台任务能把到期的 pending/approved 申请更新为 expired；写入 `quota_request.expire` 审计；重复扫描不重复写审计。 |
| V13-NOTIF-004 | P0 | 过期通知可见 | V13-NOTIF-003 | 申请人能看到 expired 状态通知；管理员可从审计页看到过期事件和 request ID。 |
| V13-NOTIF-005 | P1 | 通知列表分页和未读筛选 | V13-NOTIF-001 | 后端支持 limit/cursor 或 page/page_size；支持 unread_only；弹层默认只取轻量列表，历史可继续加载。 |
| V13-NOTIF-006 | P1 | 通知文案 i18n | V13-NOTIF-001 | 后端返回稳定 message key + params；前端按当前语言渲染中英文/繁中；后端不再拼接展示文案。 |
| V13-NOTIF-007 | P1 | deep link 回归测试 | 站内通知 MVP | 管理员 pending、申请人 decision、管理员 audit link、即将过期入口都能跳到正确页面并带上筛选参数。 |

建议顺序：V13-NOTIF-001 → V13-NOTIF-002 → V13-NOTIF-003 → V13-NOTIF-004 → V13-NOTIF-005 → V13-NOTIF-006 → V13-NOTIF-007。

### Phase A 实现要点

- 即将过期 key 建议使用 `quota_request:expiring_soon:{request_id}:{expires_at}`，这样延期或重新审批后不会错误复用旧提醒。
- 即将过期窗口首版固定 24 小时，后续再进入企业级配置。
- 过期扫描应只更新 `pending` 和 `approved`，不改 `rejected`、`withdrawn`。
- 扫描任务需要主节点保护；多节点下再用数据库条件更新兜底幂等。
- 前端应明确展示 `expiring_soon` 状态文案和样式，避免落到默认 pending 样式。

### Phase A 当前进度

- V13-NOTIF-001 已完成基础交付：通知派生逻辑已抽到 `service/enterprise_quota_request_notification.go`，controller 只保留鉴权和响应组装。
- V13-NOTIF-002 已完成基础交付：已批准且 24 小时内过期的临时额度会派生 `expiring_soon` 站内通知，申请人和管理员都可见，通知 key 包含 request ID 和 expires_at 以保持幂等。
- V13-NOTIF-003 已完成基础交付：新增 `ExpireDueEnterpriseQuotaRequests` 和 `StartEnterpriseQuotaRequestMaintenanceTask`，主节点每分钟扫描到期 pending/approved 申请并写入 `quota_request.expire` 审计。
- V13-NOTIF-004 已完成基础交付：过期审计已进入审批通知派生 actions，申请人可看到 expired 通知，管理员可在审计页按 quota_request 追踪。
- V13-NOTIF-005 已完成基础交付：通知接口支持 page/page_size/limit 和 unread_only，响应返回 page、page_size、has_more；前端 Approvals tab 支持只看未读和加载更多。
- V13-NOTIF-006 已完成基础交付：通知 payload 新增 title_key、content_key、content_params，前端优先按 key/params 渲染并保留 title/content fallback；英文和中文资源已补齐审批通知文案。
- V13-NOTIF-007 已完成基础交付：审批通知打开链接计算已抽到纯函数，并新增 `smoke:approval-notification-links` 覆盖 pending、decision、expiring soon 和 expired audit link。
- 前端已补 `expiring_soon` 标签、样式和打开入口；管理员打开进入企业审批列表，普通用户打开进入自己的申请列表并定位 request ID。

### Phase A 测试清单

- `go test ./service -run 'TestListEnterpriseQuotaRequestNotifications|TestExpireDueEnterpriseQuotaRequests|TestEnterprisePolicy'`
- `go test ./controller -run 'TestEnterprise'`
- `go test ./router -run 'TestEnterpriseQuotaRequest'`
- `cd web/default && bun run typecheck`
- `cd web/default && bunx eslint src/hooks/use-notifications.ts src/components/notification-popover.tsx src/lib/api.ts src/features/quota-requests/index.tsx src/features/enterprise/index.tsx`
- `git diff --check`

## Phase B: Notification Outbox 和邮件

目标：把外部通知从审批 API 中解耦，先建立持久事件和异步投递基础，再接邮件。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-OUTBOX-001 | P0 | Notification Outbox 模型 | Phase A | 新增 outbox 表，包含 event_key、event_type、enterprise_id、recipient、channel、payload、status、retry_count、next_retry_at。 |
| V13-OUTBOX-002 | P0 | 审批事件写 outbox | V13-OUTBOX-001 | submit、approve、reject、withdraw、expire、expiring_soon 都能写入幂等 outbox 事件。 |
| V13-OUTBOX-003 | P0 | Outbox worker | V13-OUTBOX-001 | worker 批量拉取 pending/due 事件，支持 processing/sent/failed/permanent_failed 状态和最大重试次数。 |
| V13-OUTBOX-004 | P1 | 邮件通知渠道 | V13-OUTBOX-003 | 接入现有邮件发送能力；申请人收到审批结果/过期邮件，管理员收到新申请邮件；配置关闭时不发送。 |
| V13-OUTBOX-005 | P1 | 投递日志和排查入口 | V13-OUTBOX-003 | 管理员或运维可查看最近投递状态、失败原因和下一次重试时间。 |

建议顺序：V13-OUTBOX-001 → V13-OUTBOX-002 → V13-OUTBOX-003 → V13-OUTBOX-004 → V13-OUTBOX-005。

### Phase B 设计要点

- outbox 写入应与审批状态变更在同一事务内完成。
- event_key 必须唯一，例如 `quota_request.approve:{request_id}:{audit_log_id}`。
- worker 不应依赖内存状态，重启后可以继续投递。
- 邮件正文先使用简洁模板，不做复杂品牌化；重点保证状态、策略、临时额度、过期时间和链接准确。

### Phase B 当前进度

- V13-OUTBOX-001 已完成基础交付：新增 `enterprise_notification_outbox` 模型和迁移，支持 event_key、event_type、enterprise_id、recipient、channel、payload、status、retry_count、next_retry_at、last_error。
- V13-OUTBOX-002 已完成基础交付：审批 submit、approve、reject、withdraw、expire 会在状态变更/审计同一事务内写入幂等 in_app outbox 事件；主动过期扫描也会写入 `quota_request.expire` outbox；维护任务会按 24 小时窗口幂等写入 `quota_request.expiring_soon` email/webhook outbox。
- V13-OUTBOX-003 已完成基础交付：新增 outbox claim、sent、failed、permanent_failed 状态流转和主节点定时 worker；`in_app` 事件可自动标记 sent，email/webhook 在渠道未配置时进入重试/永久失败框架。
- V13-OUTBOX-004 已完成基础交付：worker 已支持 email channel，SMTP 配置和收件人存在时调用 `common.SendEmail` 异步发送；审批结果类事件可为有邮箱的申请人写入 email outbox，管理员新申请邮件可通过 `Notifications` 页签配置管理员或显式邮箱收件范围后开启。
- V13-OUTBOX-005 已完成基础交付：企业治理页新增 Deliveries tab，支持 outbox 筛选、失败摘要、下一次重试、手动重试和 worker 指标摘要。

## Phase C: Webhook 和通知偏好

目标：让企业外部系统可订阅审批事件，并保证签名、重试、脱敏和配置可审计。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-WEBHOOK-001 | P1 | Webhook 配置模型 | Phase B | 企业可配置 webhook URL、secret、启用状态、订阅事件类型；配置变更写审计。 |
| V13-WEBHOOK-002 | P1 | Webhook 签名和 payload | V13-WEBHOOK-001 | payload 使用稳定 schema；签名使用 HMAC-SHA256；日志脱敏 URL 和 secret。 |
| V13-WEBHOOK-003 | P1 | Webhook 投递 worker | V13-WEBHOOK-002 | 复用 outbox worker；支持超时、重试、失败记录和永久失败。 |
| V13-WEBHOOK-004 | P2 | 通知偏好配置 | V13-WEBHOOK-001 | 管理员可配置站内/邮件/webhook 开关和收件范围；普通用户可选择是否接收邮件。 |
| V13-WEBHOOK-005 | P2 | Webhook 测试发送 | V13-WEBHOOK-003 | 管理员可发送测试事件并看到响应状态、耗时和错误摘要。 |

建议顺序：V13-WEBHOOK-001 → V13-WEBHOOK-002 → V13-WEBHOOK-003 → V13-WEBHOOK-004 → V13-WEBHOOK-005。

### Phase C 当前进度

- V13-WEBHOOK-001 已完成基础交付：新增 `enterprise_webhooks` 配置模型和迁移，支持企业、名称、URL、secret、订阅事件类型和启停状态；secret 不通过 JSON 返回。
- V13-WEBHOOK-002 已完成基础交付：webhook payload 使用稳定 event_id、event_type、enterprise_id、target 和 payload_json schema，签名使用 `X-Enterprise-Webhook-Signature: sha256=<hex>`。
- V13-WEBHOOK-003 已完成基础交付：outbox worker 已支持 webhook channel，按配置订阅事件生成 webhook outbox 并投递；HTTP 直连路径保留 SSRF 防护，worker 转发路径复用现有 WorkerRequest。
- V13-WEBHOOK-004 已完成基础交付：新增通知偏好管理 API/UI，支持管理员配置 email/webhook 开关和邮件收件范围。
- V13-WEBHOOK-005 已完成基础交付：新增 Webhook 管理 UI 和测试发送入口；测试结果展示 success、HTTP status、duration 和错误摘要。

## Phase D: 审批体验增强

目标：让管理员更快判断审批风险，让用户更容易从超额拒绝走到受控申请。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-UX-001 | P1 | 申请详情侧栏 | 审批 UI | 列表可打开详情，展示策略、目标、申请原因、决策原因、审计链路和有效期。 |
| V13-UX-002 | P1 | 项目专属申请筛选 | 项目策略申请 | 支持 project_id、target_type、target_id、applicant_user_id 筛选。 |
| V13-UX-003 | P1 | 审批风险摘要 | 审批 UI | 批准前展示当前用量、叠加后 limit、剩余有效期、近期策略命中和 dry-run 次数。 |
| V13-UX-004 | P2 | 批量审批 | V13-UX-003 | 管理员可批量批准/拒绝同类 pending 申请；每条申请独立审计；失败项单独展示。 |
| V13-UX-005 | P2 | 超额拒绝到申请预填 | hard-limit 审计 | 用户遇到 hard limit 后可进入申请页并预填可公开的策略上下文；普通用户不泄露内部策略细节。 |

### Phase D 当前进度

- V13-UX-001 已完成基础交付：审批列表和用户申请列表支持打开详情侧栏，展示策略、目标、申请原因、决策原因、生命周期时间和审计定位。
- V13-UX-002 已完成基础交付：申请列表支持 project_id、target_type、target_id、applicant_user_id 筛选，并兼容旧项目目标申请。
- V13-UX-003 已完成基础交付：审批申请列表返回策略 limit、当前用量、批准后叠加 limit、近 7 天策略命中和 dry-run 命中次数；单条批准/拒绝弹窗展示风险摘要和剩余有效期。
- V13-UX-004 已完成基础交付：管理员可勾选当前页 pending 申请并批量批准/拒绝，后端逐条处理并返回成功/失败明细。
- V13-UX-005 已完成基础交付：hard limit 错误返回可申请 metadata hint；前端错误 toast 提供 `Request quota` 动作，跳转 `/quota-requests` 并预填可申请 policy、项目、建议 extra limit 和原因；不可申请 policy 不会提交。

## Phase E: 发布收口

目标：在继续扩展前，把企业治理 V1.3 的发布质量门槛固定下来。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-REL-001 | P0 | 最终 preflight | Phase A | `scripts/enterprise-governance-preflight.sh` 通过，并记录摘要。 |
| V13-REL-002 | P0 | 工作区清理 | V13-REL-001 | release commit 不包含本地 DB、日志、Playwright 输出、构建产物或无关样例。 |
| V13-REL-003 | P0 | 预发 R0-R3 演练 | V13-REL-001 | 记录开关、请求 ID、审计、counter、attribution、回滚证据。 |
| V13-REL-004 | P0 | 生产小流量灰度 | V13-REL-003 | R3 第二次请求证明不进入上游；保留策略快照和回滚人。 |
| V13-REL-005 | P1 | Docker 镜像链路固化 | V13-REL-001 | 固定 tag 规则、构建命令、镜像摘要、迁移步骤和回滚 tag。 |

### Phase E 当前进度

- V13-REL-001 已完成：`scripts/enterprise-governance-preflight.sh` 通过记录已写入 `docs/enterprise-governance-v1.3-release-evidence.md`。
- V13-REL-002 已完成：preflight 和 Snapless preflight 均包含 release artifact check，避免本地 DB、日志、缓存和构建产物混入提交。
- V13-REL-005 已完成：`docs/data-proxy-release-runbook.md` 已记录 GitHub CI、tag、GHCR 镜像、digest、迁移说明和回滚路径。
- V13-REL-003/V13-REL-004 属于真实环境运营动作，等待预发和生产执行后回填证据。

## 推荐迭代安排

### Iteration 1: 站内闭环可上线

范围：V13-NOTIF-001 到 V13-NOTIF-004。

交付结果：审批 pending、decision、expiring soon、expired 全部在站内可见，并能在审计中追踪。

### Iteration 2: 通知体验稳定化

范围：V13-NOTIF-005 到 V13-NOTIF-007。

交付结果：通知列表可分页、可筛未读、文案可本地化，关键 deep link 有回归保障。

### Iteration 3: Outbox 和邮件

范围：V13-OUTBOX-001 到 V13-OUTBOX-005。

交付结果：审批事件有持久 outbox，邮件异步投递可重试、可排查。

### Iteration 4: Webhook 和偏好

范围：V13-WEBHOOK-001 到 V13-WEBHOOK-005。

交付结果：企业可配置签名 webhook，外部系统可稳定接收审批事件。

### Iteration 5: 审批效率和发布收口

范围：V13-UX-001 到 V13-UX-005、V13-REL-001 到 V13-REL-005。

交付结果：管理员审批效率提升，发布证据和回滚链路完整。

## 最近下一步

1. 在预发环境执行 `docs/enterprise-governance-v1.3-release-evidence.md` 的 R0-R3 检查表，补充真实 request ID、outbox ID、截图或变更单链接。
2. 预发通过后执行生产小流量灰度，先只开站内通知，再按单企业开启 email/webhook，并保留关闭开关和回滚负责人。
3. 若继续开发新功能，优先从 `docs/data-proxy-post-v1.3-todo.md` 进入后续企业治理版本，而不是重复做 V1.3 通知闭环。
