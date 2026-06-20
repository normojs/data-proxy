# Enterprise Governance V1.3 Release Evidence

本文档记录 V1.3 通知闭环的发布证据。代码功能、本地核验和发布模板在本地完成；预发和生产 R0-R3 需要在对应环境执行后补充真实 request ID、截图或变更单链接。

## 本地核验证据

| 字段 | 记录 |
| --- | --- |
| 记录时间 | 2026-06-20 09:14:47 CST |
| git commit | `d07a3442` |
| 工作区 | `/Users/fushilu/workspace/revocloud/data-proxy/upstream/new-api` |
| 核验命令 | `scripts/enterprise-governance-preflight.sh` |
| 结果 | 通过 |

覆盖项：

- `go test ./model ./controller ./service ./router`
- `go test ./service -run TestEnterpriseGovernanceRolloutRunbookR0ToR3 -count=1`
- `go test ./controller -run 'TestEnterprise(QuotaPolicyCreateWritesAuditLog|AuditLogFilters|UsageSummaryAndBreakdown)' -count=1`
- `cd web/default && pnpm typecheck`
- `cd web/default && pnpm build`
- `git diff --check`
- release artifact check

补充核验：

- `cd web/default && bun run smoke:approval-notification-links` 通过。
- 普通用户审批邮件偏好 gate 覆盖：`TestEnqueueEnterpriseQuotaRequestOutboxRespectsUserEmailPreference` 通过。

## V1.3 功能证据清单

| 功能 | 本地证据 | 结果 |
| --- | --- | --- |
| 站内审批通知、已读、分页、deep link | service/controller 测试和 `smoke:approval-notification-links` | 通过 |
| 审批审计可见 | controller/service 测试，admin guide 更新 | 通过 |
| notification outbox、email、webhook worker | service 测试和 preflight | 通过 |
| 通知偏好管理 API/UI | controller/router/service 测试，前端 typecheck/build | 通过 |
| Webhook 管理和测试发送 UI | controller/router 测试，前端 typecheck/build | 通过 |
| 投递日志、筛选、重试和 worker 指标 UI | controller/service 测试，前端 typecheck/build | 通过 |
| 普通用户审批邮件偏好 | service 测试，Profile 前端 typecheck/build | 通过 |
| 管理员操作文档 | `docs/enterprise-governance-admin-guide.md` | 完成 |

## 预发 R0-R3 证据

| 字段 | 记录 |
| --- | --- |
| 环境 | preprod |
| 执行人 |  |
| 执行时间 |  |
| 版本或 commit |  |
| 数据库类型 | sqlite / mysql / postgres |
| 测试用户 ID |  |
| 测试 token ID |  |
| 测试部门/分组 |  |
| 关联变更单 |  |

### 开关状态

| 阶段 | EnterpriseGovernanceEnabled | EnterpriseGovernanceDryRunEnabled | 结论 |
| --- | --- | --- | --- |
| R0 发布后旧行为 | false | false |  |
| R1 初始化配置 | false | false |  |
| R2 dry-run 观测 | true | true |  |
| R3 小范围 hard limit | true | false |  |

### 请求和通知证据

| 阶段 | request_id / outbox_id | 检查项 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- | --- | --- |
| R0 |  | 普通请求 | 成功，旧行为一致 |  |  |
| R2 |  | dry-run 超低额度请求 | 成功，不拒绝，写 dry-run 审计 |  |  |
| R3 |  | 第一次硬限制请求 | 成功并结算 |  |  |
| R3 |  | 第二次硬限制请求 | 403，不打上游 |  |  |
| 通知 |  | 站内 submit/approve/reject | 可见、可跳转、可标记已读 |  |  |
| 通知 |  | email outbox | 按 Notifications 偏好入队并投递 |  |  |
| 通知 |  | webhook test | 签名、status、duration、错误摘要可见 |  |  |
| 通知 |  | failed/permanent_failed retry | 可手动重试并写审计 |  |  |

### 回滚证据

| 操作 | 记录 |
| --- | --- |
| 关闭 `EnterpriseGovernanceEnabled` 的时间 |  |
| 关闭后普通请求 request_id |  |
| 关闭后响应是否恢复旧行为 |  |
| 关闭后是否停止新增 enterprise counter |  |
| 关闭后是否停止新增 enterprise attribution |  |

## 生产 R0-R3 证据

生产执行前复制预发表格，使用生产变更单编号记录。生产扩大范围前必须保留：

- hard-limit 策略快照。
- 回滚负责人。
- 关闭外部 email/webhook 的操作路径。
- 最近一次 `scripts/enterprise-governance-preflight.sh` 通过记录。
