# Enterprise Governance Admin Guide

本文档面向企业治理管理员，说明如何在管理端完成第一条组织额度策略，以及请求被拒绝时如何排查。

配套文档：

- `docs/enterprise-org-quota-mvp-delivery-plan.md`：MVP 范围、完成定义和里程碑。
- `docs/enterprise-org-quota-rollout-runbook.md`：发布、灰度、观测和回滚流程。
- `docs/enterprise-org-quota-cel-policy-engine-plan.md`：策略条件和 CEL 能力说明。

## 入口和权限

管理端入口：

- 侧边栏：`Admin` -> `Enterprise Governance`
- 路由：`/enterprise`
- 权限：系统 `admin/root` 继续拥有全部企业治理权限；企业成员角色通过 `enterprise.*` capability 分组访问。

企业治理角色和能力：

| 角色 | 当前能力 |
| --- | --- |
| `owner` / `enterprise_admin` / `admin` | 读取、管理配置、审批临时额度、查看财务和审计、管理项目 |
| `finance_viewer` | 读取企业治理入口和财务用量视图 |
| `auditor` | 读取企业治理入口和审计、通知 outbox、worker metrics |
| `project_admin` | 读取、查看自己负责或被授权项目的项目列表、成员、用量和审计日志；仅能管理自己负责或被授权为项目 admin 的项目 |
| `department_admin` | 读取企业治理入口，管理本部门及子部门成员、策略组和额度策略，审批本部门及子部门临时额度申请，查看本部门及子部门用量和审计日志 |

后端 API 按 `enterprise.read`、`enterprise.manage`、`enterprise.department.manage`、`enterprise.finance.read`、`enterprise.audit.read`、`enterprise.quota.approve`、`enterprise.project.manage` 分组鉴权。前端入口和页签使用 `/api/user/self` 返回的 `permissions.enterprise_governance` 控制可见性。

部门管理员使用主部门作为 scope 根节点。后端会把 scope 展开为“本部门 + 子部门”，并在成员列表、成员归属更新、策略组列表/创建/更新/停用/成员维护、额度策略列表/创建/更新/停用、临时额度审批、审批通知、用量报表和审计日志中自动过滤跨部门数据。企业配置、SSO 同步、webhook、通知偏好、项目管理、notification outbox 和 worker metrics 仍需企业管理员、审计员或系统管理员权限。

策略组成员支持 `viewer` 和 `editor` 角色。旧版或未传角色的成员添加请求默认写入 `viewer`；重复添加同一成员会更新 role。当前额度策略匹配仍按“用户是否在策略组内”判断，`viewer/editor` 用于治理可见性和后续协作权限扩展，不改变已有策略命中语义。

策略组可通过共享部门支持跨部门协作。归属部门或企业管理员可维护共享部门列表；共享部门管理员可以在自己的 scope 内查看该策略组、维护本部门成员、创建指向该策略组的额度策略，但不能编辑或停用策略组本体。跨部门共享目前不改变策略命中逻辑，也不自动授予共享部门外用户成员权限。

项目管理员使用 `enterprise_projects.owner_user_id` 和 `enterprise_project_members` 作为 scope。项目 `member` 只进入只读 scope，可查看项目列表、项目成员、项目用量 summary/breakdown、CSV 导出和项目审计日志；项目 owner 或项目 `admin` 成员进入管理 scope，可编辑/停用项目并维护项目成员。创建项目时 owner 必须是当前项目管理员本人；更新项目时可保持原 owner，不允许转移给 scope 外用户。没有任何项目 scope 的项目管理员只会看到空项目和空用量。

系统开关：

| 开关 | 建议初始值 | 说明 |
| --- | --- | --- |
| `EnterpriseGovernanceEnabled` | `false` | 总开关，关闭时 relay 不进入企业治理判断 |
| `EnterpriseGovernanceDryRunEnabled` | `false` | 观测模式，开启后只记录 would reject，不拒绝请求 |

首次配置时建议保持 `enabled=false`，先完成组织和策略配置，再进入 dry-run。

## 创建第一条策略

### 1. 确认默认企业

进入 `Enterprise Governance` 后，先在 Overview 顶部确认：

- 企业名称。
- 企业 slug。
- 时区，默认建议使用业务主时区，例如 `Asia/Shanghai`。
- 状态为 enabled。

如果企业名称或时区不正确，点击企业卡片右侧编辑按钮修改。

### 2. 创建部门

进入 `Organization`：

1. 点击 `Org Unit`。
2. 输入部门名称，例如 `Engineering`。
3. slug 使用英文小写，例如 `engineering`。
4. 父级选择 `Root` 或已有部门。
5. 保存。

建议先创建 1 到 3 个真实部门，不要一次性导入全部组织。第一轮只验证口径和链路。

### 3. 分配成员主部门

仍在 `Organization`：

1. 在 Members 表格中搜索用户。
2. 在 Org Unit 下拉框选择主部门。
3. 点击 `Save`。

一个用户首版只维护一个主部门。API Key 会继承所属用户的企业、部门和策略分组。

### 4. 创建策略分组

进入 `Policy Groups`：

1. 点击 `Policy Group`。
2. 输入名称，例如 `Pilot Users`。
3. 输入 slug，例如 `pilot-users`。
4. 保存。
5. 在分组行点击成员按钮。
6. 输入用户 ID，支持逗号、空格或换行分隔，例如 `12, 19, 25`。
7. 点击 `Add`。

策略分组适合表达试点用户、外包、实习生、高阶模型权限用户等特殊人群。企业管理员创建的未绑定部门策略组是全局策略组；部门管理员创建的策略组会自动绑定到自己的主部门 scope，只能加入本部门及子部门成员，也只能被本部门 scope 内的额度策略引用。

### 5. 创建额度策略

进入 `Quota Policies`：

1. 点击 `Quota Policy`。
2. 选择目标类型：
   - `Enterprise`：全企业兜底。
   - `Org Unit`：部门及子部门用户。
   - `Policy Group`：跨部门成员集合。
   - `User`：单个用户。
3. 选择指标：
   - `Requests`：请求次数。
   - `Quota`：系统 quota。
4. 选择周期：`Daily` 或 `Monthly`。
5. 输入 limit，例如每日请求数 `100`。
6. 模型范围选择：
   - `All Models`：全部模型。
   - `Specific Models`：输入模型名，逗号分隔。
7. 条件模式：
   - `Structured`：通过 UI 字段配置常见条件。
   - `CEL`：输入 CEL 表达式。
8. 保存。

Structured 支持的条件字段：

| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `Abilities` | 限定请求能力，逗号、空格或换行分隔 | `chat, image` |
| `Runtime Groups` | 限定现有运行分组 | `default, vip` |
| `Model Names` | 限定完整模型名 | `gpt-4o, claude-3-5-sonnet` |
| `Model Prefixes` | 限定模型名前缀 | `gpt-4, claude` |
| `Channel IDs` | 限定渠道 ID | `1, 2, 3` |
| `Playground` | 限定 Playground 或 API Key 来源 | `Playground` / `API Key` |

Structured 表单会生成后端 `condition_json`，后端保存时再生成稳定 CEL 表达式并编译校验。字段留空表示不限制该维度。

第一条策略建议使用 `request_count`，limit 设置很低，目标只选择测试用户或测试策略分组。

## 推荐灰度步骤

### R0: 关闭状态验证

保持：

- `EnterpriseGovernanceEnabled=false`
- `EnterpriseGovernanceDryRunEnabled=false`

发送普通请求，确认现有计费、用户额度和渠道路由没有变化。

### R1: 开启 dry-run

设置：

- `EnterpriseGovernanceEnabled=true`
- `EnterpriseGovernanceDryRunEnabled=true`

用测试用户触发一条低额度策略。通过标准：

- 请求仍成功。
- `Audit` 中可看到 dry-run would reject 记录。
- `Usage` 中可看到 attribution。

### R2: 小范围 hard limit

设置：

- `EnterpriseGovernanceEnabled=true`
- `EnterpriseGovernanceDryRunEnabled=false`

只保留测试用户或测试分组策略，例如每日 request count = 1。连续发送两次请求：

- 第一次应成功。
- 第二次应返回企业治理额度错误。
- 第二次不应产生上游消耗。

### R3: 扩大到真实部门

选择低风险部门，先用 request count 策略验证 1 到 2 个工作日，再切到 quota 策略或扩大范围。

## 排查拒绝请求

用户反馈企业治理拒绝时，按下面顺序排查：

1. 收集 request ID、用户 ID、模型名、请求时间。
2. 在 `Audit` 按目标类型或 request ID 方向查找 dry-run/reject 记录。
3. 在 `Organization` 确认用户主部门是否正确。
4. 在 `Policy Groups` 确认用户是否属于特殊分组。
5. 在 `Quota Policies` 查目标策略：
   - 状态是否 enabled。
   - metric 和 period 是否正确。
   - limit 是否过低。
   - model scope 是否覆盖该模型。
   - CEL/结构化条件是否过窄。
6. 在 `Usage` 切换维度为部门、分组或用户，确认当前周期用量。

常见原因：

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 用户刚开始就被拒绝 | 目标策略 limit 太低或周期未按预期重置 | 调整 limit 或 timezone |
| 只有某模型被拒绝 | model scope 或条件表达式限制 | 检查指定模型列表和条件 |
| 一个部门集中拒绝 | 部门级策略过低 | 先切回 dry-run 或提高部门 limit |
| dry-run 下仍被拒绝 | 总开关/模式配置不一致，或被现有用户额度拒绝 | 检查系统开关和原有额度 |
| 用户不应命中某分组策略 | 策略分组成员误加 | 从分组成员弹窗移除用户 |

## 回滚

快速回滚：

1. 设置 `EnterpriseGovernanceEnabled=false`。
2. 保留企业治理数据，不删除 counter、attribution、audit。
3. 重新发送请求，确认恢复原有额度和计费逻辑。
4. 排查完成后从 dry-run 重新开始。

配置回滚：

- 停用错误策略。
- 从策略分组移除误加成员。
- 调整部门归属。
- 通过审计日志记录变更人和变更时间。

不要直接删除审计日志或 attribution，也不要直接改 counter 来掩盖配置错误。

## 审批通知闭环

V1.3 起，临时额度审批事件会进入站内通知、企业审计和 notification outbox。建议上线顺序是先确认站内通知和审计，再显式开启邮件或 webhook。

### 站内通知

站内通知默认可用，不需要管理员单独开启。审批相关通知包括：

| 事件 | 可见对象 | 说明 |
| --- | --- | --- |
| `quota_request.submit` | 管理员 | 新临时额度申请待审批。 |
| `quota_request.approve` | 申请人 | 申请已批准，临时额度开始生效。 |
| `quota_request.reject` | 申请人 | 申请已拒绝，可查看拒绝原因。 |
| `quota_request.withdraw` | 管理员 | 申请人撤回待审批申请。 |
| `quota_request.expiring_soon` | 申请人、管理员 | 已批准临时额度将在 24 小时内过期。 |
| `quota_request.expire` | 申请人、管理员 | 待审批或已批准申请已过期。 |

通知弹层支持未读状态和分页。管理员从通知进入企业治理审批页，普通用户从通知进入自己的临时额度申请页。审计页可按 request ID 或 target 查找对应事件。

### 配置外部通知偏好

进入 `Enterprise Governance` -> `Notifications`。外部通知默认关闭，需要管理员按事件显式开启。

每个事件可配置：

- `Email`：是否写入邮件 outbox。
- `Email Recipients`：邮件收件范围。
  - `Applicant`：发送给申请人。
  - `Enterprise Admins`：发送给企业管理员。
  - 显式邮箱：用逗号、空格或换行分隔，例如 `ops@example.com, owner@example.com`。
- `Webhook`：是否允许该事件写入 webhook outbox。

注意事项：

- 站内通知不能关闭，它是审批闭环和审计追踪的基础。
- Email 和 Webhook 都只控制 outbox 入队，不在审批 API 请求链路中同步投递。
- Webhook 投递必须同时满足 `Notifications` 中该事件的 webhook 开关已开启、Webhook 配置本身 enabled、且该 webhook 订阅了对应事件。
- 通知偏好变更会写入 `notification_preference.update` 审计；审计只记录显式邮箱数量，不记录邮箱明文。

### 配置 Webhook

进入 `Enterprise Governance` -> `Webhooks`。

新增 webhook：

1. 点击 `Webhook`。
2. 输入名称，例如 `Approval Workflow`。
3. 输入 HTTPS endpoint URL。
4. 输入 secret。secret 用于生成 HMAC-SHA256 签名。
5. 选择订阅事件，例如 `quota_request.submit`、`quota_request.approve`。
6. 状态保持 `Enabled`，保存。

编辑 webhook：

- `secret` 留空表示保留旧 secret。
- 如果需要重置 secret，输入新 secret 并保存。
- 停用 webhook 后不会再为该 endpoint 生成新的投递；已有 outbox 记录仍保留用于审计和排查。

Webhook 请求包含签名 header：

```text
X-Enterprise-Webhook-Signature: sha256=<hex>
```

接收端应使用共享 secret 对原始请求 body 计算 HMAC-SHA256，并与 header 做恒定时间比较。

### 测试 Webhook

在 `Webhooks` 表格中点击测试发送按钮。测试会直接向 endpoint 发送 `enterprise.webhook.test` 样例事件，并展示：

- 成功或失败。
- HTTP status code。
- 请求耗时。
- 错误摘要。

测试发送会写入 `webhook.test` 企业审计。返回错误摘要不会展示 secret；URL query 和敏感字段在审计中会脱敏。

### 查看投递日志和手动重试

进入 `Enterprise Governance` -> `Deliveries`。

页面上方显示 worker 最近批次和累计指标：

- claimed。
- sent。
- failed。
- permanent_failed。
- duration。
- total runs。

投递列表支持按以下条件筛选：

- channel：`in_app`、`email`、`webhook`。
- status：`pending`、`processing`、`sent`、`failed`、`permanent_failed`。
- event type。
- webhook。
- target ID。
- 时间范围。

失败排查建议：

| 状态 | 含义 | 处理 |
| --- | --- | --- |
| `pending` | 等待 worker 投递 | 等待下一轮 worker，或检查 worker 是否启动。 |
| `processing` | worker 已领取 | 若长时间不变，后端会按 stale 规则重新领取。 |
| `failed` | 投递失败，仍可自动重试 | 查看 last_error，确认 SMTP/webhook endpoint/网络配置。 |
| `permanent_failed` | 已达到最大重试次数 | 修复配置后点击重试按钮，重置为 pending。 |
| `sent` | 投递成功 | 无需处理。 |

手动重试只对 `failed` 和 `permanent_failed` 可用。点击重试会清空失败摘要、重试次数和下一次重试时间，并写入 `notification_outbox.retry` 审计。

### 邮件和 Webhook 灰度建议

1. 仅开启站内通知，完成 submit、approve、reject、withdraw、expiring soon、expire 的站内和审计验证。
2. 在 `Notifications` 中只为测试企业或测试事件开启 email，收件范围先使用显式测试邮箱。
3. 确认 SMTP 配置后，通过真实审批事件观察 `Deliveries` 中 email 投递状态。
4. 新建一个测试 webhook，先只订阅 `quota_request.submit`，使用测试发送确认签名和响应。
5. 在 `Notifications` 中开启 webhook 偏好，再触发真实审批事件，确认 outbox、接收端日志和审计一致。
6. 扩大到 approve/reject/expire 等事件前，保留关闭开关和回滚负责人。

快速关闭外部通知：

- 在 `Notifications` 中关闭对应事件的 Email/Webhook。
- 或在 `Webhooks` 中停用具体 endpoint。
- 已经入队的失败记录不会自动删除，可保留用于排查；必要时等待其自然失败或在修复后手动重试。

### 普通用户邮件偏好

普通用户可进入 `Profile` -> `Notifications`，使用 `Approval Result Emails` 开关控制是否接收发送给自己的企业临时额度审批邮件。

规则：

- 默认接收审批结果、即将过期和已过期邮件。
- 关闭后，只影响以申请人为收件人的企业审批邮件。
- 管理员收件范围和显式邮箱由企业管理员在 `Enterprise Governance` -> `Notifications` 中控制，不受个人偏好影响。
- 站内通知不受该开关影响，仍保持可见和可审计。
