# Enterprise Governance V1.3 Follow-up Development Tasks

本文档用于承接 V1.3 通知闭环当前实现状态，规划后续开发任务。当前判断：站内审批通知、审计可见、notification outbox、邮件投递骨架、webhook 投递骨架、投递查询、手动重试和 worker 指标后端能力已经基本具备；后续重点应从“后端可用”推进到“管理员可配置、可观察、可演练、可发布”。

## 当前状态摘要

已完成或基本完成：

- 站内审批状态通知：submit、approve、reject、withdraw、expiring soon、expired 可见。
- 通知已读状态：支持持久化、分页、未读筛选和前端打开入口。
- 审计可见：审批提交、批准、拒绝、撤回、过期、webhook 配置和 outbox 重试等关键动作写入审计。
- Notification outbox：支持 in_app、email、webhook 渠道，具备 pending、processing、sent、failed、permanent_failed 状态流转。
- 邮件后端：通过通知偏好 gate 和收件范围生成 email outbox，worker 调用现有邮件发送能力。
- Webhook 后端：支持 webhook 配置、签名、测试发送、订阅过滤、投递 worker 和敏感信息脱敏。
- 投递可观测后端：支持 outbox 查询、失败摘要脱敏、手动重试和 worker 运行指标快照。
- Expiring soon 外部通知：维护任务可按 24 小时窗口幂等写入 email/webhook outbox。

本轮已继续完成：

- Webhook 管理 UI：企业治理页已新增 `Webhooks` 页签，支持新增、编辑、停用和测试发送。
- 通知投递日志 UI：企业治理页已新增 `Deliveries` 页签，支持筛选、查看失败摘要、worker 指标和手动重试。
- 通知偏好管理 API/UI：新增 `/api/enterprise/notification-preferences` 读取和保存接口，企业治理页已新增 `Notifications` 页签，支持 email/webhook 开关和邮件收件范围配置。
- 普通用户邮件偏好：Profile 通知设置新增 `Approval Result Emails` 开关；关闭后不再为该用户写入申请人审批邮件 outbox，站内通知不受影响。
- 管理员文档：`docs/enterprise-governance-admin-guide.md` 已补充站内通知、通知偏好、webhook、测试发送、投递日志、手动重试和灰度建议。

功能闭环状态：

- V1.3 通知闭环代码、管理 UI、管理员文档和本地发布前核验均已完成。
- 预发和生产 R0-R3 是发布运营动作，不属于本地代码未完成项；真实环境执行记录统一填写到 `docs/enterprise-governance-v1.3-release-evidence.md` 或对应变更单。

## 开发原则

- 站内通知默认开启，邮件和 webhook 默认关闭，由管理员显式配置。
- 外部投递一律经过 outbox，审批请求链路不直接调用外部系统。
- 配置变更、测试发送、手动重试等管理动作必须写企业审计。
- webhook secret、URL query、邮箱、错误响应等敏感信息必须在 API、审计和 UI 中脱敏展示。
- 所有周期任务和 outbox 写入必须保持幂等，避免重复通知。
- UI 优先做紧凑的管理面和排查面，不做营销式或装饰性页面。

## Milestone 1: Webhook 管理 UI

目标：让企业管理员可以在企业治理页完成 webhook 的配置、测试和启停。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| V13-FU-WH-001 | P0 | 补齐前端 API 类型和请求方法 | `web/default/src/features/enterprise/types.ts` 和 `api.ts` 包含 webhook list/create/update/disable/test 的类型与方法，`bun run typecheck` 通过。 |
| V13-FU-WH-002 | P0 | 企业治理页新增 Webhooks 页签 | 企业治理页出现 Webhooks tab，可展示名称、URL、订阅事件、启用状态、secret 设置状态、创建/更新时间。 |
| V13-FU-WH-003 | P0 | 新增和编辑 webhook 弹窗 | 支持 name、url、secret、event_types、status；编辑时 secret 留空表示不重置。 |
| V13-FU-WH-004 | P0 | 测试发送入口 | 每个 webhook 可点击测试，展示 success、status_code、duration_ms 和错误摘要；失败不泄露 secret。 |
| V13-FU-WH-005 | P1 | 启停和删除确认 | disable/delete 操作有明确确认态，成功后刷新列表并保留审计后端行为。 |

建议实现顺序：V13-FU-WH-001 到 V13-FU-WH-005。

当前进度：V13-FU-WH-001 到 V13-FU-WH-005 已完成基础交付，并通过 `bun run typecheck`。

## Milestone 2: 投递日志和运维排查 UI

目标：让管理员不查数据库也能定位外部通知是否发送、为何失败、何时重试。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| V13-FU-OBS-001 | P0 | 补齐 outbox 前端 API 类型和请求方法 | 支持 outbox 列表、筛选、分页、手动重试和 worker metrics 查询。 |
| V13-FU-OBS-002 | P0 | 企业治理页新增 Deliveries 页签 | 可查看 event_type、channel、recipient、status、retry_count、next_retry_at、last_error、created_at。 |
| V13-FU-OBS-003 | P0 | 投递筛选器 | 支持 channel、status、event_type、webhook、target_id 和时间范围筛选；分页稳定。 |
| V13-FU-OBS-004 | P1 | 手动重试按钮 | 仅 failed/permanent_failed 显示 retry 操作；成功后刷新列表并展示新状态。 |
| V13-FU-OBS-005 | P1 | Worker 指标摘要 | 展示最近批次 claimed、sent、failed、permanent_failed、duration 和累计计数。 |

建议实现顺序：先完成列表和筛选，再接重试和指标。

当前进度：V13-FU-OBS-001 到 V13-FU-OBS-005 已完成基础交付，并通过前端 typecheck 和后端 controller/service/router 定向测试。

## Milestone 3: 通知偏好配置产品化

目标：把邮件和 webhook 从“后端有 gate”变成“管理员可控制的企业设置”。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| V13-FU-PREF-001 | P0 | 通知偏好管理 API | 支持按 enterprise/channel/event_type 读取和保存 enabled、recipient_scope；配置变更写审计。 |
| V13-FU-PREF-002 | P0 | 邮件收件范围配置 | 管理员可配置 applicant、enterprise_admins、explicit_emails；默认外部邮件关闭。 |
| V13-FU-PREF-003 | P0 | Webhook 事件订阅配置联动 | webhook 投递必须同时满足通知偏好 enabled 和 webhook 自身启用/订阅。 |
| V13-FU-PREF-004 | P1 | 企业治理页新增 Notifications 设置区 | 用表格或紧凑表单展示各事件的 in_app/email/webhook 开关和收件范围。 |
| V13-FU-PREF-005 | P1 | 普通用户邮件偏好 | 用户可关闭个人审批结果邮件；站内通知不可关闭。 |

首版可以只实现管理员配置 API 和企业治理页 UI，普通用户偏好作为增强项进入下一轮。

当前进度：V13-FU-PREF-001 到 V13-FU-PREF-005 已完成基础交付；普通用户邮件偏好仅影响申请人邮件，不影响站内通知、管理员收件范围或显式邮箱。

## Milestone 4: 文档、灰度和发布收口

目标：确保 V1.3 通知闭环可以被验证、发布、回滚和交接。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| V13-FU-REL-001 | P0 | 执行前端验证 | `cd web/default && bun run typecheck`、`bun run smoke:approval-notification-links` 通过。 |
| V13-FU-REL-002 | P0 | 执行后端定向测试 | enterprise notification、outbox、webhook、controller、router、model 相关测试通过。 |
| V13-FU-REL-003 | P0 | 执行完整 preflight | `scripts/enterprise-governance-preflight.sh` 通过，或记录明确历史债豁免。 |
| V13-FU-REL-004 | P0 | 更新管理员文档 | `docs/enterprise-governance-admin-guide.md` 增加通知偏好、webhook、测试发送、投递日志和重试说明。 |
| V13-FU-REL-005 | P1 | 灰度演练记录模板 | 补充站内通知、邮件、webhook、失败重试、关闭开关和回滚步骤。 |

建议发布顺序：先只开启站内通知，再对单企业开启邮件，最后开启 webhook；每一步都保留关闭开关和回滚负责人。

当前进度：V13-FU-REL-001 到 V13-FU-REL-005 已完成；`scripts/enterprise-governance-preflight.sh` 已通过，真实环境证据模板已落到 `docs/enterprise-governance-v1.3-release-evidence.md`。

## 推荐下一批开发包

若目标是尽快获得可演示、可排查的 V1.3 管理闭环，建议下一批只做以下范围：

1. V13-FU-WH-001 到 V13-FU-WH-004：Webhook 管理和测试发送 UI。
2. V13-FU-OBS-001 到 V13-FU-OBS-004：投递日志、筛选和手动重试 UI。
3. V13-FU-PREF-001 到 V13-FU-PREF-002：通知偏好管理 API 和邮件收件范围配置。
4. V13-FU-REL-001 到 V13-FU-REL-003：完成前端、后端和 preflight 验证。

这批完成后，V1.3 可以达到“站内可见、外部可配置、失败可排查、发布可验证”的最小上线门槛。

## 验证命令建议

```bash
go test ./service -run 'TestEnterpriseWebhook|TestSendEnterpriseWebhook|TestDeliverEnterpriseWebhook|TestListEnterpriseNotificationOutbox|TestRetryEnterpriseNotificationOutbox|TestEnqueueEnterpriseNotificationOutbox|TestEnqueueEnterpriseQuotaRequestOutbox|TestEnqueueExpiringSoonEnterpriseQuotaRequestOutbox|TestDeliverEnterpriseNotificationOutboxEmail|TestProcessEnterpriseNotificationOutbox|TestMarkEnterpriseNotificationOutboxFailedPermanentAfterMaxRetries|TestListEnterpriseQuotaRequestNotifications|TestExpireDueEnterpriseQuotaRequests|TestEnterprisePolicy'
go test ./controller -run 'TestEnterprise'
go test ./router -run 'TestEnterprise'
go test ./model -run 'TestRecordEnterpriseAuditLog'
cd web/default && bun run typecheck
cd web/default && bun run smoke:approval-notification-links
git diff --check
scripts/enterprise-governance-preflight.sh
```

## 风险和注意事项

- 当前工作区包含大量企业治理相关未跟踪文件，提交前需要仔细区分本轮文件和无关改动。
- Webhook 测试发送可能访问外部 URL，生产环境必须保持 SSRF 防护和超时限制。
- 邮件发送依赖 SMTP 配置，缺配置时应进入可理解的失败状态，而不是静默成功。
- 通知偏好默认值必须保守，避免迁移后自动向外部邮箱或 webhook 发送企业数据。
- UI 中不要展示 payload 明文大 JSON，首版只展示必要摘要；详情能力后续再补。
