# Enterprise Governance V1.3 Next Development Plan

本文档承接 `enterprise-governance-v1.3-notification-development-tasks.md`，用于规划 V1.3 通知闭环之后的后续开发任务。当前主线已经完成站内审批通知、审计可见、基础 outbox、邮件投递骨架和 webhook 投递骨架；下一步重点是把外部通知变成可配置、可排查、可灰度上线的产品能力。

## 当前判断

已可认为完成的能力：

- 审批状态站内通知闭环：pending、approve、reject、withdraw、expiring soon、expired 均已在站内可见。
- 通知已读状态持久化：支持分页、未读筛选和加载更多。
- 审批通知 i18n：后端返回 message key 和 params，前端本地化渲染并保留 fallback。
- 审计可见：提交、批准、拒绝、撤回、过期均写企业审计，管理员可追踪 request ID。
- 主动过期扫描：后台任务扫描 pending/approved 到期申请并幂等写过期审计。
- Notification outbox 基础：审批状态变更写 outbox，worker 支持状态流转、重试和永久失败。
- Email 基础：审批结果类事件可为申请人写入 email outbox，并由 worker 调用现有邮件发送能力。
- Webhook 基础：已具备 webhook 配置模型、签名、payload、outbox 生成和 worker 投递基础。

已在后续收口中补齐的能力：

- 通知偏好管理 API/UI：企业治理页新增 `Notifications` 页签，可配置 email/webhook 开关和邮件收件范围；配置变更写审计且不泄露邮箱明文。
- Webhook 管理 UI：企业治理页新增 `Webhooks` 页签，可新增、编辑、停用和测试发送 webhook。
- 投递日志 UI：企业治理页新增 `Deliveries` 页签，可筛选 outbox、查看失败摘要、worker 指标和手动重试。
- 管理员文档：`docs/enterprise-governance-admin-guide.md` 已补充通知偏好、webhook、测试发送、投递日志、手动重试和灰度建议。
- 发布前 preflight：`scripts/enterprise-governance-preflight.sh` 已重新执行通过。

仍需真实环境补证据的能力：

- 预发和生产灰度演练证据需要在真实环境执行后补回变更单或记录链接。

## 开发原则

- 默认站内开启，外部通知显式配置后开启。
- 所有外部投递必须经过 outbox，不在审批请求链路中直接发送邮件或 webhook。
- 配置变更必须写审计，secret、URL token、邮箱列表等敏感信息在日志和响应中脱敏。
- 所有周期任务和 outbox 写入必须有稳定 event key，允许重复调用但不能重复刷屏。
- 优先补齐最小管理面，再做漂亮 UI；先让管理员能配置、测试、排查。

## Milestone 1: 收口当前代码和验证基线

目标：确认现有 V1.3 通知/outbox/webhook 基础代码稳定，形成后续开发的可信基线。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| V13-NEXT-001 | P0 | 运行当前定向后端测试 | `go test ./service -run 'TestEnterpriseWebhook|TestSendEnterpriseWebhook|TestDeliverEnterpriseWebhook|TestEnqueueEnterpriseNotificationOutbox|TestDeliverEnterpriseNotificationOutboxEmail|TestProcessEnterpriseNotificationOutbox|TestListEnterpriseQuotaRequestNotifications|TestExpireDueEnterpriseQuotaRequests|TestEnterprisePolicy'` 通过。 |
| V13-NEXT-002 | P0 | 运行当前定向 controller/router/model 测试 | `go test ./controller -run 'TestEnterprise'`、`go test ./router -run 'TestEnterpriseQuotaRequest'`、`go test ./model -run 'TestRecordEnterpriseAuditLog'` 通过。 |
| V13-NEXT-003 | P0 | 运行前端通知相关检查 | `cd web/default && bun run typecheck`、`bun run smoke:approval-notification-links` 通过；若全量 lint 有历史债，记录非本轮问题。 |
| V13-NEXT-004 | P0 | 代码格式和补丁卫生检查 | `gofmt` 已覆盖修改文件，`git diff --check` 通过，无构建产物、日志、本地 DB 混入。 |

建议先完成本里程碑，再继续改新功能。当前工作区改动较多，先稳住基线能减少后续排查成本。

## Milestone 2: 通知偏好和收件范围

目标：把邮件和 webhook 从“技术可投递”变成“管理员可控制”。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-PREF-001 | P0 | 新增企业通知偏好模型 | Milestone 1 | 支持 enterprise_id、channel、event_type、enabled、recipient_scope、created/updated；迁移和模型测试通过。 |
| V13-PREF-002 | P0 | 邮件启用 gate | V13-PREF-001 | 审批结果邮件、管理员新申请邮件均受偏好控制；默认关闭或仅保留明确安全默认值。 |
| V13-PREF-003 | P0 | 管理员收件范围 | V13-PREF-001 | 新申请邮件可配置收件范围：企业管理员、指定邮箱、申请所属部门管理员预留；首版至少支持企业管理员和指定邮箱。 |
| V13-PREF-004 | P1 | 用户邮件退订/接收偏好 | V13-PREF-001 | 普通用户可控制是否接收审批结果邮件；站内通知不允许关闭。 |
| V13-PREF-005 | P1 | 偏好配置审计 | V13-PREF-001 | 通知偏好创建、更新、启停写企业审计，审计 payload 不泄露敏感收件人完整列表。 |

当前进度：

- V13-PREF-001 已完成基础交付：新增 `enterprise_notification_preferences` 模型和迁移，按 enterprise/channel/event 唯一配置启用状态和 `recipient_scope_json`。
- V13-PREF-002 已完成基础交付：email/webhook outbox 生成已接入偏好 gate；没有配置时外部渠道默认不入队，站内通知不受影响。
- V13-PREF-003 已完成：email recipient scope 支持 `applicant`、`enterprise_admins` 和 `explicit_emails`；新申请邮件可投递给企业管理员和显式邮箱，管理员可在 `Notifications` 页签配置收件范围。
- V13-PREF-004 已完成基础交付：企业治理页新增 `Notifications` 设置区，管理员可配置 email/webhook 开关和邮件收件范围。
- V13-PREF-005 已完成基础交付：Profile 通知设置新增 `Approval Result Emails` 开关；关闭后不再为该用户写入申请人审批邮件 outbox，站内通知保持不可关闭。
- 偏好配置审计已在管理员管理接口中完成：`notification_preference.update` 审计只记录显式邮箱数量，不记录邮箱明文。

实现建议：

- 模型可命名为 `enterprise_notification_preferences`。
- `event_type` 支持 `quota_request.submit`、`quota_request.approve`、`quota_request.reject`、`quota_request.withdraw`、`quota_request.expire`、`quota_request.expiring_soon`。
- `recipient_scope` 首版可使用 JSON，保存 `applicant`、`enterprise_admins`、`explicit_emails` 等开关和列表。
- outbox 生成阶段读取偏好；worker 阶段只负责投递，不再重新决定业务收件范围。

## Milestone 3: Webhook 管理和测试发送

目标：让企业管理员可以管理 webhook，并在上线前自助验证签名、payload 和响应。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-WH-001 | P0 | Webhook 管理 API | Milestone 1 | 支持 list/create/update/disable/delete；secret 只在创建/重置时接收，不在响应中返回。 |
| V13-WH-002 | P0 | Webhook 配置审计 | V13-WH-001 | URL、订阅事件、启停、secret 重置均写审计；URL 查询参数脱敏。 |
| V13-WH-003 | P0 | Webhook 测试发送 API | V13-WH-001 | 管理员可发送 `enterprise.webhook.test` 或选择样例审批事件；返回 HTTP 状态、耗时、错误摘要和签名 header。 |
| V13-WH-004 | P1 | 最近投递结果 API | V13-WH-001 | 可按 webhook、事件类型、状态筛选最近 outbox；展示 sent/failed/permanent_failed、retry_count、next_retry_at、last_error。 |
| V13-WH-005 | P1 | Webhook 管理 UI | V13-WH-001/V13-WH-003/V13-WH-004 | 企业治理页可新增、编辑、启停、测试发送和查看最近投递结果。 |

当前进度：

- V13-WH-001 已完成后端基础交付：新增 `/api/enterprise/webhooks` 管理 API，支持列表、创建、更新和停用；响应只返回 `has_secret`，不返回 secret 明文。
- V13-WH-002 已完成后端基础交付：webhook 创建、更新、停用、测试发送均写企业审计；审计 payload 会脱敏 URL query，并只记录是否设置 secret。
- V13-WH-003 已完成后端基础交付：新增 `/api/enterprise/webhooks/:id/test` 测试发送 API，返回 success、HTTP status、duration、错误摘要和签名 header 是否生成。
- V13-WH-004 已完成后端基础交付：新增 `/api/enterprise/notification-outbox`，可按 channel、event_type、status、target、webhook_id 和时间范围查看最近投递结果；邮箱和失败摘要已脱敏。
- V13-WH-005 已完成基础交付：企业治理页新增 Webhooks tab，支持新增、编辑、停用、测试发送和查看测试结果摘要。

实现建议：

- 测试发送可以直接走发送函数，但也应记录一条测试审计；是否写 outbox 可由实现选择，首版建议直接发送并保存结果到审计 payload。
- 最近投递结果优先复用 `enterprise_notification_outbox`，避免新增日志表。
- UI 中 secret 使用“已设置/未设置”和“重置”交互，不展示明文。

## Milestone 4: Expiring Soon Outbox 和外部提醒补齐

目标：让“即将过期”不仅站内可见，也能按配置进入邮件/webhook 投递链路。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-EXP-001 | P0 | `expiring_soon` outbox 写入 | Milestone 2 | 维护任务扫描 24 小时内到期的 approved 申请，写入稳定 event key 的 outbox；重复扫描不重复写。 |
| V13-EXP-002 | P0 | 提醒窗口 key 设计 | V13-EXP-001 | event_key 至少包含 request_id、expires_at、window，例如 `quota_request.expiring_soon:{request_id}:{expires_at}:24h`。 |
| V13-EXP-003 | P1 | 邮件/webhook payload 补齐 | V13-EXP-001 | payload 包含申请 ID、策略 ID、目标、临时额度、过期时间、剩余窗口和跳转链接。 |
| V13-EXP-004 | P1 | 测试覆盖 | V13-EXP-001 | 覆盖未到窗口不写、进入窗口写一次、重复扫描不重复、延期后生成新 key。 |

当前进度：

- V13-EXP-001 已完成后端基础交付：维护任务会扫描 24 小时内到期的 approved 申请，并按通知偏好写入 email/webhook outbox。
- V13-EXP-002 已完成：event_key 使用 `quota_request.expiring_soon:{request_id}:{expires_at}:24h:{recipient_hash}`，同一申请同一过期时间和窗口重复扫描不重复写。
- V13-EXP-003 已完成基础 payload：payload 包含申请 ID、策略 ID、目标、metric、period、临时额度、申请人、审批人、effective_at、expires_at 和 i18n key。
- V13-EXP-004 已完成基础测试：覆盖未到窗口不写、进入窗口写一次、重复扫描不重复、延期后因 expires_at 变化生成新 key。

注意：站内 `expiring_soon` 当前可以派生展示，不一定需要 outbox 驱动；本里程碑只补外部通知链路。

## Milestone 5: 投递可观测和运维排查

目标：让失败通知能被看见、理解和处理，避免外部通知静默失效。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-OBS-001 | P0 | Outbox 查询 API | Milestone 1 | 管理员可按 channel、event_type、status、enterprise_id、时间查询 outbox。 |
| V13-OBS-002 | P0 | 失败原因脱敏和截断 | V13-OBS-001 | last_error 不包含 secret/token，不超过固定长度；HTTP 响应体只保留摘要。 |
| V13-OBS-003 | P1 | 手动重试入口 | V13-OBS-001 | failed/permanent_failed 可由管理员手动重置为 pending；操作写审计。 |
| V13-OBS-004 | P1 | Worker 运行指标 | Milestone 1 | 日志或 metrics 至少包含 claimed、sent、failed、permanent_failed、duration。 |
| V13-OBS-005 | P1 | 管理 UI 投递日志页 | V13-OBS-001 | 企业治理页可查看最近投递、失败详情、下一次重试和手动重试。 |

当前进度：

- V13-OBS-001 已完成后端基础交付：新增企业通知 outbox 查询 service 和管理员 API，支持 channel、event_type、status、target_type、target_id、webhook_id、start_time、end_time 过滤和分页。
- V13-OBS-002 已完成基础交付：列表 DTO 不返回 payload，email 收件人会掩码，last_error 会截断并脱敏 token/secret/key/signature/password query 参数。
- V13-OBS-003 已完成后端基础交付：新增 `/api/enterprise/notification-outbox/:id/retry`，可将 failed/permanent_failed 重置为 pending，清空错误和重试计数，并写 `notification_outbox.retry` 审计。
- V13-OBS-004 已完成后端基础交付：outbox worker 批处理会记录 claimed、sent、failed、permanent_failed、duration_ms、started_at、finished_at，并累计 total 指标；新增 `/api/enterprise/notification-outbox/worker-metrics` 可读取快照。
- V13-OBS-005 已完成基础交付：企业治理页新增 Deliveries tab，支持 outbox 筛选、失败摘要、下一次重试、手动重试和 worker 指标摘要。

## Milestone 6: 发布收口

目标：把 V1.3 通知闭环作为可发布能力收口，而不是停留在开发分支状态。

| ID | 优先级 | 任务 | 依赖 | 验收 |
| --- | --- | --- | --- | --- |
| V13-REL-001 | P0 | 完整 enterprise governance preflight | Milestone 2-5 | `scripts/enterprise-governance-preflight.sh` 通过，或记录明确的历史债豁免。 |
| V13-REL-002 | P0 | 灰度配置文档 | V13-REL-001 | 记录站内通知、邮件、webhook 的默认开关、环境变量、回滚方式。 |
| V13-REL-003 | P0 | 预发演练 | V13-REL-001 | 在预发完成申请、审批、拒绝、撤回、过期、邮件、webhook 测试发送和失败重试演练。 |
| V13-REL-004 | P1 | 管理员操作说明 | V13-REL-002 | 更新 admin guide，说明如何配置通知偏好、webhook、测试发送和排查失败。 |
| V13-REL-005 | P1 | 生产小流量启用 | V13-REL-003 | 先只开启站内通知，再开启单企业邮件/webhook；保留关闭开关和回滚负责人。 |

## 推荐开发顺序

1. 先做 Milestone 1，确认当前 webhook/outbox 改动的测试基线。
2. 接着做 Milestone 2，补通知偏好和收件范围。这是开启邮件、webhook 前最重要的产品边界。
3. 然后做 Milestone 3，交付 webhook 管理 API、测试发送和最近投递查询。
4. 再补 Milestone 4，把 `expiring_soon` 纳入 outbox 外部通知。
5. 最后做 Milestone 5 和 Milestone 6，补排查入口、手动重试、preflight、预发演练和发布文档。

## 最小下一批开发包

如果希望下一轮尽快交付一个可演示版本，建议只取以下 P0 范围：

- V13-NEXT-001 到 V13-NEXT-004：验证现有基线。
- V13-PREF-001 到 V13-PREF-003：通知偏好、邮件 gate、管理员收件范围。
- V13-WH-001 到 V13-WH-003：webhook 管理 API、审计、测试发送。
- V13-EXP-001 到 V13-EXP-002：`expiring_soon` outbox 幂等写入。
- V13-REL-001：跑完整 preflight。

这批完成后，V1.3 的通知闭环可以达到“站内可用、外部可配置、投递可验证”的上线门槛；UI 投递日志、手动重试和用户级邮件偏好可以作为下一批增强。

## 风险和注意事项

- 当前工作区有大量未跟踪文件和跨模块改动，提交前必须做 artifact 检查，避免把本地输出混入 release。
- Webhook worker 复用现有 `WorkerRequest` 时要确认 SSRF 防护边界；测试环境可关闭保护，生产路径不能绕过安全校验。
- 邮件发送依赖 SMTP 配置，缺配置时应进入可理解的失败/待配置状态，而不是无声丢弃。
- 通知偏好默认值要保守，尤其是管理员收件范围和外部 webhook，不应因为迁移自动向未知外部地址发送数据。
- 审计 payload 和 outbox payload 要区分：审计服务于排查，outbox 服务于投递；两者都要避免泄露 secret。

## 建议验证命令

```bash
go test ./service -run 'TestEnterpriseWebhook|TestSendEnterpriseWebhook|TestDeliverEnterpriseWebhook|TestEnqueueEnterpriseNotificationOutbox|TestDeliverEnterpriseNotificationOutboxEmail|TestProcessEnterpriseNotificationOutbox|TestListEnterpriseQuotaRequestNotifications|TestExpireDueEnterpriseQuotaRequests|TestEnterprisePolicy'
go test ./controller -run 'TestEnterprise'
go test ./router -run 'TestEnterpriseQuotaRequest'
go test ./model -run 'TestRecordEnterpriseAuditLog'
cd web/default && bun run typecheck
cd web/default && bun run smoke:approval-notification-links
git diff --check
```

发布前再运行：

```bash
scripts/enterprise-governance-preflight.sh
```
