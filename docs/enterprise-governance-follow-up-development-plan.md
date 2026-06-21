# Enterprise Governance Follow-up Development Plan

本文档用于承接 V1.3 通知闭环完成 MVP 之后的后续开发任务。目标是把当前“审批状态站内通知/审计事件可见”的最小闭环，逐步推进到可上线、可运营、可扩展到邮件和 webhook 的企业治理能力。

## 当前基线

已完成的基础能力：

- 企业治理 V1.1 可运营化基础能力：审计日志、dry-run 观测、拒绝解释、报表筛选、策略启停确认。
- V1.2 项目/成本中心基础能力：项目模型、API Key 默认项目、请求项目归属、项目策略、项目报表、项目审计。
- V1.3 审批和临时额度基础能力：申请模型、提交/批准/拒绝/撤回 API、临时额度叠加 hard limit、管理员审批 UI、普通用户申请入口、审批审计。
- V1.3 通知 MVP：审批状态站内通知、已读状态、通知红点、通知弹层 Approvals tab、审批/审计 deep link。

本地代码已完成的关键能力：

- 即将过期提醒、主动过期扫描和过期审计。
- 邮件和 webhook 外部通知，均通过持久 notification outbox 异步投递。
- 通知偏好、投递日志、失败重试、worker 指标和 webhook 测试发送。
- 项目专属申请详情、历史筛选、批量审批、审批风险摘要和普通用户申请引导。
- 发布前本地 preflight、管理员文档、Docker 镜像链路和发布证据模板。

仍需真实环境完成的运营动作：

- 预发 R0-R3 演练，补充真实 request ID、outbox ID、截图或变更单链接。
- 生产小流量灰度，保留策略快照、回滚负责人和外部通知关闭路径。

## P0: V1.3 通知闭环补强

这些任务优先级最高，用于把当前站内通知 MVP 变成可运营闭环。

| ID | 任务 | 说明 | 验收 |
| --- | --- | --- | --- |
| EG-FU-001 | ✅ 即将过期提醒事件 | 已批准且 24 小时内过期的临时额度会生成 `expiring_soon` 站内通知。 | 管理员和申请人能看到即将过期通知；同一申请在同一提醒窗口内不重复刷屏；通知可标记已读。 |
| EG-FU-002 | ✅ 过期状态扫描任务 | 主节点维护任务会把到期 pending/approved 申请更新为 expired，并写入审计和 outbox。 | 到期后无需用户访问审批接口，也能落库 expired；审计中可看到过期动作和 request ID。 |
| EG-FU-003 | ✅ 通知来源统一封装 | 审批通知派生逻辑已封装到 service，controller 只保留鉴权和响应组装。 | controller 只负责鉴权和响应；通知列表、key 生成、read 状态合并有单元测试。 |
| EG-FU-004 | ✅ 通知列表筛选和分页 | 通知接口支持 page/page_size/limit、has_more 和 unread_only。 | 通知弹层保持轻量；后端接口可分页拉取历史通知；未读数准确。 |
| EG-FU-005 | ✅ 通知文案 i18n | 后端返回稳定 title/content key 和 params，前端按语言渲染并保留 fallback。 | 中英文/繁中可以本地化；后端 payload 不依赖展示语言。 |
| EG-FU-006 | ✅ 通知 deep link 回归测试 | `smoke:approval-notification-links` 覆盖 pending、decision、expiring soon 和 expired audit link。 | 通知点击能打开正确页面、筛选正确 request/status/audit target，并自动关闭 popover。 |

建议实现顺序：EG-FU-003 → EG-FU-001 → EG-FU-002 → EG-FU-004 → EG-FU-005 → EG-FU-006。

当前进度：

- EG-FU-001 到 EG-FU-006 已完成本地交付，并通过 service/controller/router 定向测试、前端 typecheck 和 deep link smoke。
- 即将过期与已过期事件继续以审计和业务状态为事实源，站内通知可派生展示，外部通知走 outbox。

## P1: 邮件和 Webhook 外部通知

这些任务在站内通知稳定后启动。重点是先建 outbox，再接投递渠道，避免直接在审批 API 请求链路中发送外部通知。

| ID | 任务 | 说明 | 验收 |
| --- | --- | --- | --- |
| EG-FU-101 | ✅ Notification Outbox 模型 | 新增企业通知事件表，记录事件类型、收件人、目标、payload、状态、重试次数、下一次重试时间。 | 审批提交/批准/拒绝/撤回/过期/即将过期都能写入 outbox；重复事件有幂等 key。 |
| EG-FU-102 | ✅ Outbox Worker | 后台 worker 负责投递邮件/webhook，支持批量、重试、失败记录和最大重试次数。 | 审批 API 不等待外部投递；失败可重试；永久失败可在审计或运维日志中查询。 |
| EG-FU-103 | ✅ 邮件通知渠道 | 接入现有 `common.SendEmail` 能力，发送审批状态邮件。 | 申请人收到批准/拒绝/撤回/过期邮件；管理员收到新申请邮件；可通过配置关闭。 |
| EG-FU-104 | ✅ Webhook 通知渠道 | 支持企业级 webhook URL、secret、事件类型订阅，payload 使用 HMAC-SHA256 签名。 | 外部系统收到签名 webhook；失败重试；后台能看到最近投递结果。 |
| EG-FU-105 | ✅ 通知偏好配置 | 管理员可配置 email/webhook 开关和收件范围；普通用户可关闭审批结果邮件。 | 配置变更可审计；默认不破坏现有站内通知。 |
| EG-FU-106 | ✅ 投递安全和脱敏 | webhook secret、URL query、邮箱和失败摘要已在 API/UI/日志路径脱敏。 | 日志中不出现 webhook secret；payload 字段清单可审阅。 |

建议实现顺序：EG-FU-101 → EG-FU-102 → EG-FU-103 → EG-FU-104 → EG-FU-105 → EG-FU-106。

## P1: 审批体验增强

这些任务提升日常使用效率，不阻塞通知外部化。

| ID | 任务 | 说明 | 验收 |
| --- | --- | --- | --- |
| EG-FU-201 | 申请详情侧栏 | ✅ 已完成：审批列表和用户申请列表支持打开详情侧栏，展示策略、目标、申请原因、决策原因、生命周期时间和审计定位；管理员可从侧栏跳转到 `quota_request` 审计筛选。 | 用户无需离开列表即可理解一条申请的完整上下文。 |
| EG-FU-202 | 项目专属申请历史筛选 | ✅ 已完成：申请列表支持 project_id、target_type、target_id、applicant_user_id 筛选；申请记录持久化 project_id，并兼容旧项目目标申请的 target_type=project/target_id 筛选。 | 管理员可快速定位某项目、某用户、某策略的历史申请。 |
| EG-FU-203 | 审批批量操作 | ✅ 已完成：管理员可勾选当前页 pending 申请并批量批准/拒绝，确认弹窗复用决策原因；后端逐条处理并返回成功/失败明细。 | 批量操作每条申请都有独立审计；失败项可单独展示。 |
| EG-FU-204 | 审批风险摘要 | ✅ 已完成：审批列表返回策略 limit、当前用量、批准后叠加 limit、近 7 天策略命中和 dry-run 命中次数；单条批准/拒绝弹窗展示风险摘要和剩余有效期。 | 管理员审批前能看到影响范围，不只是看到申请理由。 |
| EG-FU-205 | 普通用户申请引导 | ✅ 已完成：hard limit 错误返回可申请 metadata hint；前端错误 toast 提供 `Request quota` 动作，跳转 `/quota-requests` 并预填可申请 policy、项目、建议 extra limit 和原因；不可申请 policy 不会提交。 | 从拒绝提示到提交申请的路径更短；没有策略详情泄露给普通用户。 |

## P0: 发布前收口

这些任务属于上线质量门槛，优先级高于继续扩功能。

| ID | 任务 | 说明 | 验收 |
| --- | --- | --- | --- |
| EG-REL-001 | ✅ 最终 preflight | 已在发布证据中记录 `scripts/enterprise-governance-preflight.sh` 通过。 | `scripts/enterprise-governance-preflight.sh` 通过，并记录命令输出摘要。 |
| EG-REL-002 | ✅ 工作区整理 | 发布脚本包含 artifact check，避免本地 DB、日志、Playwright 输出、构建产物混入 release commit。 | release commit 只包含源码、文档、脚本和必要样例。 |
| EG-REL-003 | 预发 R0-R3 演练 | 真实环境动作，按 rollout runbook 记录开关、请求 ID、审计、counter、attribution、回滚证据。 | 预发证据链接或变更单编号回填到发布记录。 |
| EG-REL-004 | 生产 R0-R3 演练 | 真实环境动作，小范围执行 hard-limit 灰度，确认 R3 第二次请求不进入上游。 | 保留策略快照、回滚人、日志脱敏和结果记录。 |
| EG-REL-005 | ✅ Docker 镜像链路固化 | `docs/data-proxy-release-runbook.md` 已记录 tag 规则、GHCR 镜像、digest、迁移说明和回滚路径。 | 后续发布能通过镜像路径复现，不依赖临时源码部署。 |

## P2: 后续版本路线

这些任务不是 V1.3 通知闭环的直接后续，但应进入后续版本规划。

| 版本 | 方向 | 关键任务 |
| --- | --- | --- |
| V1.4 | SSO 组织同步 | importer 抽象、增量同步、dry-run、冲突处理 UI、同步审计和安全回滚。 |
| V1.5 | 高并发和精细额度 | Redis 原子计数、DB/Redis 对账、token 级硬限制、失败补偿队列、压测脚本。 |
| V1.6 | 高级策略动作 | alert、fallback_model、queue、shared_pool、异常检测自动限流。 |
| V1.7 | 多级管理员和财务视图 | 企业 RBAC、部门管理员、财务查看员、审计员、项目管理员和权限回归测试。 |

## 推荐迭代拆分

### Iteration A: 站内通知补强

范围：EG-FU-001 到 EG-FU-006。

目标：站内通知覆盖 pending、decision、即将过期、已过期，并具备分页、i18n 和回归测试。

完成标准：不依赖邮件/webhook，也能让管理员和申请人完成审批状态感知闭环。

### Iteration B: Outbox 和邮件

范围：EG-FU-101 到 EG-FU-103。

目标：建立持久通知事件和异步投递基础，先接入邮件。

完成标准：审批事件写入 outbox，worker 可发送邮件并处理失败重试。

### Iteration C: Webhook 和通知偏好

范围：EG-FU-104 到 EG-FU-106。

目标：让企业外部系统接收审批事件，同时保证签名、重试和脱敏。

完成标准：webhook 可配置、可签名、可重试、可审计。

### Iteration D: 审批体验增强

范围：EG-FU-201 到 EG-FU-205。

目标：提升审批效率、历史查询和用户自助申请体验。

完成标准：管理员能快速判断风险，用户能从超额拒绝顺滑进入申请。

### Iteration E: 发布收口

范围：EG-REL-001 到 EG-REL-005。

目标：完成最终发布质量门槛。

完成标准：preflight、预发演练、生产灰度和镜像发布链路都有可追溯记录。

## 风险和决策点

- 是否保留“派生式通知”作为长期方案：如果只做站内通知可以继续保留；如果上邮件/webhook，建议引入 outbox。
- 即将过期提醒窗口如何配置：初期可固定 24 小时，后续再做企业级配置。
- 通知收件人范围：pending 默认通知管理员；decision 默认通知申请人；即将过期建议同时通知申请人和管理员。
- 邮件和 webhook 是否默认开启：建议默认关闭外部投递，只默认开启站内通知。
- 多节点部署下 worker 如何避免重复投递：outbox 需要基于状态锁、`next_retry_at` 和幂等 key 设计。

## 下一步建议

1. 在预发环境执行 V1.3 R0-R3 演练，回填 `docs/enterprise-governance-v1.3-release-evidence.md` 的真实 request ID、outbox ID、截图或变更单链接。
2. 预发通过后执行生产小流量灰度，先只开启站内通知，再按单企业开启邮件/webhook，并保留关闭开关和回滚负责人。
3. 后续版本继续沿 `docs/data-proxy-post-v1.3-todo.md` 推进 V1.4+ 企业治理增强。
