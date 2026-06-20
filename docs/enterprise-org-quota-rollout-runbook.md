# Enterprise Organization And Quota Rollout Runbook

本文档描述企业组织与额度治理从开发完成到生产启用的发布、灰度、观测和回滚流程。
它不替代产品方案或工程蓝图，而是给实际上线值班、管理员配置和故障处理使用。

配套文档：

- `docs/enterprise-org-quota-plan.md`：整体方案和 MVP 范围。
- `docs/enterprise-org-quota-mvp-delivery-plan.md`：MVP 交付总入口、里程碑和完成定义。
- `docs/enterprise-org-quota-cel-policy-engine-plan.md`：CEL 条件层和策略引擎开发目标。
- `docs/enterprise-org-quota-task-plan.md`：阶段、Backlog、PR 切分和验收。
- `docs/enterprise-org-quota-implementation-blueprint.md`：工程实现细节。
- `docs/enterprise-governance-admin-guide.md`：管理员首条策略配置、拒绝排查和回滚指南。

## 上线原则

- 默认关闭：新版本发布后不应自动改变任何 relay 行为。
- 先观测后拦截：先打开 dry-run，确认命中结果和报表后再启用硬限制。
- 小范围验证：先对测试企业、测试部门或少量用户验证，再扩大范围。
- 一键回退：任何异常都应能通过关闭开关恢复到旧行为。
- 账实一致：企业归集数据要能解释现有 billing/session/log 的结果。

## 关键开关

| 开关 | 默认值 | 所属权限 | 作用 |
| --- | --- | --- | --- |
| `EnterpriseGovernanceEnabled` | false | root | 是否启用企业治理链路 |
| `EnterpriseGovernanceDryRunEnabled` | false | root | 是否只观测不拒绝 |
| `EnterpriseGovernanceRootBypassEnabled` | 可后置 | root | root 是否绕过策略 |

推荐状态流转：

1. 发布代码：enabled=false, dry-run=false。
2. 初始化配置：enabled=false, dry-run=false。
3. 观测模式：enabled=true, dry-run=true。
4. 小范围硬限制：enabled=true, dry-run=false，并只配置测试策略。
5. 全量硬限制：enabled=true, dry-run=false，并配置正式策略。
6. 回滚：enabled=false。

## 发布前检查

推荐先执行一键核验脚本：

```bash
./scripts/enterprise-governance-preflight.sh
```

该脚本会依次执行后端定向测试、R0-R3 本地 smoke、管理 API/报表局部测试、前端
typecheck/build 和 `git diff --check`。下面的命令可用于分阶段手工排查。

### 代码检查

后端：

```bash
go test ./model ./controller ./service ./router
git diff --check
```

前端：

```bash
cd web/default && pnpm typecheck
git diff --check
```

如果本次只发布 PR 1 数据底座：

```bash
go test ./model ./common
git diff --check
```

### 迁移检查

发布前确认：

- 新表可以在空库创建。
- 老库升级后不会要求人工补非空字段。
- 重复启动 `AutoMigrate` 不报错。
- 默认企业 `slug=default` 只创建一次。
- 现有 `User.Group`、用户 quota、token quota 数据未被修改。

建议在预发库执行：

```sql
select id, name, slug, status from enterprises where slug = 'default';
select count(*) from enterprise_org_units;
select count(*) from enterprise_quota_policies;
select count(*) from enterprise_quota_counters;
```

### 配置检查

发布前确认：

- `EnterpriseGovernanceEnabled=false`。
- `EnterpriseGovernanceDryRunEnabled=false`。
- 管理员账号可访问企业治理管理 API。
- 审计日志表可写。
- relay 基础回归通过。

## 本地演练证据

R0 到 R3 已有一条可重复的本地 smoke 测试，用于在预发环境不可用时先验证发布链路的核心语义。

执行命令：

```bash
mkdir -p .cache/go-build .cache/go-tmp
env GOCACHE=$PWD/.cache/go-build GOTMPDIR=$PWD/.cache/go-tmp go test ./service -run TestEnterpriseGovernanceRolloutRunbookR0ToR3 -count=1
```

覆盖范围：

- R0：`EnterpriseGovernanceEnabled=false` 时，relay 前置检查和成功归集都不产生 enterprise counter、attribution、audit 副作用。
- R1：默认企业、部门、成员主部门、策略分组、分组成员和第一条策略可初始化，审计日志可写可查。
- R2：`enabled=true, dry-run=true` 时，超低额度策略只记录 would reject，真实请求继续成功，成功请求写入 attribution，不写 counter。
- R3：`enabled=true, dry-run=false` 时，小范围 request_count 硬限制第一次请求成功结算，第二次请求返回 403，且不会产生 rejected request attribution。

管理 API 和审计筛选的局部证据：

```bash
env GOCACHE=$PWD/.cache/go-build GOTMPDIR=$PWD/.cache/go-tmp go test ./controller -run 'TestEnterprise(QuotaPolicyCreateWritesAuditLog|AuditLogFilters|UsageSummaryAndBreakdown)' -count=1
```

发布前完整本地核验：

```bash
./scripts/enterprise-governance-preflight.sh
```

注意：以上是本地集成演练证据。预发和生产仍需要按本 runbook 在真实环境执行 R0-R3，并记录实际版本、配置、请求 ID 和观测截图。

## Docker 镜像发布记录

企业治理发布建议以镜像部署作为标准路径，源码部署只作为临时应急路径。原因是镜像可以固定
前端产物、后端二进制、运行时依赖和版本摘要，便于预发到生产复用同一个 release candidate。

发布前记录：

| 字段 | 记录 |
| --- | --- |
| release tag |  |
| git commit |  |
| image repository |  |
| image tag |  |
| image digest |  |
| build command |  |
| preflight command | `./scripts/enterprise-governance-preflight.sh` |
| Docker gate | `DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight` |
| database type | sqlite / mysql / postgres |
| migration strategy | AutoMigrate / external migration / other |
| operator |  |
| rollback image tag |  |

推荐命令：

```bash
./scripts/enterprise-governance-preflight.sh
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight
docker build -t data-proxy:<release-tag> .
docker image inspect data-proxy:<release-tag> --format '{{index .RepoDigests 0}}'
```

如果本地镜像没有 registry digest，至少记录 `docker image inspect` 中的 image ID，并在推送到 registry 后补充
registry digest。

部署前确认：

- 镜像 tag 不使用裸 `latest` 作为唯一发布标识。
- `SESSION_SECRET`、数据库连接、Redis 连接和 `NODE_NAME` 在多节点环境中已固定。
- 发布开始时 `EnterpriseGovernanceEnabled=false`，`EnterpriseGovernanceDryRunEnabled=false`。
- R0-R3 真实环境证据模板已准备好。
- 回滚 tag 已在目标环境可拉取。

回滚命令按实际编排系统记录。例如 Docker Compose 环境：

```bash
docker compose pull data-proxy
docker compose up -d data-proxy
```

回滚后仍按本文档的“回滚证据”表记录关闭开关、请求 ID、counter/attribution 是否停止新增。

## HStation OAuth 验证记录

当前本地 FRP 环境：

| 项目 | 值 |
| --- | --- |
| Data Proxy 本地端口 | `127.0.0.1:13000` |
| H 站 OAuth bridge 本地端口 | `127.0.0.1:18092` |
| 公网域名 | `https://newapi.tunnel.runna.cc` |
| Data Proxy 公网健康检查 | `https://newapi.tunnel.runna.cc/api/status` |
| H 站 OAuth bridge 公网健康检查 | `https://newapi.tunnel.runna.cc/dc-oauth/health` |

当前线上建议优先按 bridge-based custom OAuth 验证。Data Proxy 自定义 OAuth provider 的回调地址为：

```text
<Data Proxy ServerAddress>/oauth/<provider-slug>
```

当前 provider slug 是 `dc.hhhl.cc` 时，H 站 OAuth 应用中登记：

```text
https://newapi.tunnel.runna.cc/oauth/dc.hhhl.cc
```

bridge 端点记录为：

| 字段 | 值 |
| --- | --- |
| Provider Name | `dc.hhhl.cc` |
| Provider Slug | `dc.hhhl.cc` |
| Client ID | `dc-hhhl-app-auth` |
| Client Secret | 自定义固定随机值，需与 bridge 的 app secret 一致 |
| Authorization Endpoint | `https://newapi.tunnel.runna.cc/dc-oauth/authorize` |
| Token Endpoint | `https://newapi.tunnel.runna.cc/dc-oauth/token` |
| User Info Endpoint | `https://newapi.tunnel.runna.cc/dc-oauth/userinfo` |
| Scopes | `read:account` |

内置 HStation provider 名称为 `hstation`，如果不走 custom OAuth bridge，而是启用内置 HStation OAuth，
Data Proxy 回调地址为：

```text
<Data Proxy ServerAddress>/oauth/hstation
```

例如生产域名是 `https://data-proxy.example.com` 时，需要在 H 站 OAuth 应用中登记：

```text
https://data-proxy.example.com/oauth/hstation
```

内置 provider 的 client ID 和 client secret 也可以使用双方约定的固定随机值；关键是 Data Proxy 配置、
bridge 或 H 站应用侧保存的值必须一致。

验证记录：

| 场景 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- |
| OAuth 配置保存 | `HStationOAuthEnabled=true` 且 client、endpoint 完整 |  |  |
| 新用户登录/注册 | 首次 H 站授权后创建 Data Proxy 用户并写入 `h_station_id` |  |  |
| 已有用户绑定 | 登录态用户可绑定 H 站账号 |  |  |
| 重复绑定 | 已被其他用户绑定的 H 站账号会被拒绝 |  |  |
| 解绑 | 用户或管理员解绑后不可再通过旧绑定直接登录 |  |  |
| 取消授权 | 前端显示简洁 OAuth 失败提示，不暴露 token 或 secret |  |  |
| 错误 client secret | token exchange 失败，日志只记录必要错误信息 |  |  |
| 错误回调地址 | H 站拒绝或 Data Proxy state 校验失败，可定位配置错误 |  |  |

发布记录至少保留：H 站 OAuth 应用 ID、Data Proxy `ServerAddress`、回调 URL、测试账号、失败截图或请求 ID。

## 灰度流程

### R0: 代码发布但不开启

目标：确认新版本不影响现有系统。

操作：

- 部署包含企业治理代码的新版本。
- 保持 enabled=false。
- 跑一次普通 chat/completions 请求。
- 跑一次 streaming 请求。
- 跑一次余额不足或现有 quota 拦截场景。

通过标准：

- relay 请求行为和旧版本一致。
- 没有新增企业 counter。
- 没有新增企业 attribution。
- 日志中没有企业治理错误。

### R1: 初始化默认企业

目标：确认治理底座可用。

操作：

- 检查默认企业。
- 创建少量部门和成员归属。
- 创建一个策略分组。
- 创建一个不会触发拒绝的测试策略。

通过标准：

- 写接口都有审计日志。
- 组织树 path/depth 正确。
- 成员、分组、策略列表可查询。

### R2: Dry-run 观测

目标：验证策略命中、报表归集和 would reject 判断。

操作：

- 设置 enabled=true。
- 设置 dry-run=true。
- 创建一条极低额度的测试策略。
- 用测试用户发起请求。
- 查看 dry-run would reject 日志。
- 查看 usage attribution。

通过标准：

- 请求不被拒绝。
- dry-run 记录能看到命中策略 ID。
- 用量归集能关联 user、org unit、policy group、policy。
- prompt 内容没有进入审计或普通日志。

### R3: 小范围硬限制

目标：确认超额拒绝不会打到上游。

操作：

- 保持 enabled=true。
- 设置 dry-run=false。
- 只保留测试用户或测试分组策略。
- 配置 request_count=1 的日策略。
- 连续发送两次请求。

通过标准：

- 第一次请求成功。
- 第二次请求返回 403 企业治理错误。
- 第二次请求不产生上游消耗。
- counter 保持在预期值。
- 现有用户钱包和 token 额度没有异常扣费。

## R0-R3 真实环境证据模板

预发和生产执行 R0-R3 时，建议每一轮都复制本模板保存到发布记录或变更单。
本地 smoke 只能证明代码语义，真实环境证据用于证明配置、迁移、观测和回滚链路都可用。

### 基础信息

| 字段 | 记录 |
| --- | --- |
| 环境 | preprod / production |
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

### 请求证据

| 阶段 | request_id | 请求类型 | 预期 | 实际响应 | 是否打上游 | 结论 |
| --- | --- | --- | --- | --- | --- | --- |
| R0 |  | 普通请求 | 成功，旧行为一致 |  |  |  |
| R0 |  | streaming 请求 | 成功，旧行为一致 |  |  |  |
| R2 |  | dry-run 超低额度请求 | 成功，不拒绝 |  |  |  |
| R3 |  | 第一次硬限制请求 | 成功并结算 |  | 是 |  |
| R3 |  | 第二次硬限制请求 | 403 企业治理错误 |  | 否 |  |

### 数据库和审计证据

| 阶段 | 检查项 | 查询结果或截图链接 | 结论 |
| --- | --- | --- | --- |
| R0 | `enterprise_quota_counters` 无新增 |  |  |
| R0 | `enterprise_usage_attributions` 无新增 |  |  |
| R1 | 默认企业存在 |  |  |
| R1 | 部门 path/depth 正确 |  |  |
| R1 | 成员主部门正确 |  |  |
| R1 | 策略分组成员正确 |  |  |
| R1 | 策略创建审计存在 |  |  |
| R2 | dry-run would reject 审计存在 |  |  |
| R2 | attribution 可关联用户、部门、分组、策略 |  |  |
| R2 | 审计和普通日志不包含 prompt/API key |  |  |
| R3 | counter used/reserved 符合预期 |  |  |
| R3 | rejected request 不产生 attribution |  |  |

### 回滚证据

| 操作 | 记录 |
| --- | --- |
| 关闭 `EnterpriseGovernanceEnabled` 的时间 |  |
| 关闭后普通请求 request_id |  |
| 关闭后响应是否恢复旧行为 |  |
| 关闭后是否停止新增 enterprise counter |  |
| 关闭后是否停止新增 enterprise attribution |  |
| 是否需要数据库回滚 |  |
| 最终结论 |  |

### R4: 部门级灰度

目标：按真实组织结构验证预算控制。

操作：

- 选择一个低风险部门。
- 设置每日 request count 或 quota 策略。
- 启用一到两个工作日。
- 对比企业归集数据和现有日志/账单。

通过标准：

- 归集数据与现有 billing/log 趋势一致。
- 没有大量误拒绝。
- 管理员能解释每一次拒绝的策略原因。
- 用户反馈可接受。

### R5: 全量启用

目标：进入正式治理。

操作：

- 为所有部门配置默认策略。
- 为特殊用户配置用户级或策略分组策略。
- 开启报表定期检查。
- 记录首次全量启用时间和策略版本。

通过标准：

- 每个活跃用户都有企业归属或默认企业兜底。
- 关键部门都有策略。
- 拒绝率、错误率、上游成本都在预期范围。

## 观测指标

建议至少观测：

| 指标 | 说明 | 异常含义 |
| --- | --- | --- |
| enterprise policy decision count | allow/reject/dry-run 次数 | 策略配置或流量变化 |
| enterprise reject rate | 企业治理拒绝率 | 误配额度或真实超额 |
| enterprise dry-run would reject count | dry-run 下本应拒绝次数 | 启用硬限制前的影响评估 |
| enterprise reservation rollback count | 预占回滚次数 | 上游失败、计费失败或并发冲突 |
| enterprise settlement error count | 结算失败次数 | counter/attribution 不一致风险 |
| enterprise attribution count | 成功归集条数 | 报表数据完整性 |
| relay success/error rate | 原有 relay 成功率 | 企业治理是否引入回归 |
| billing event count | 原有计费事件数 | 账实核对 |

日志字段建议：

- `request_id`
- `user_id`
- `token_id`
- `enterprise_id`
- `org_unit_id`
- `policy_group_ids`
- `policy_ids`
- `decision`
- `reject_reason`
- `dry_run`
- `reserved_quota`
- `actual_quota`

不要记录：

- prompt 明文。
- completion 明文。
- 用户密钥。
- 上游 API key。

## 数据核对

### 每日核对

建议每天检查：

- 企业 attribution 总 quota 与 billing event 总 quota 的差异。
- request count 与 relay 日志条数差异。
- counter used value 是否大于 attribution 汇总值很多。
- dry-run would reject 是否异常升高。
- reject 是否集中在某个部门、分组或策略。

差异可接受原因：

- 预估 quota 和实际 quota 有 delta。
- 失败请求是否写 attribution 的口径不同。
- 流式请求结算延迟。
- 旧版本请求没有 attribution。

不可接受信号：

- enabled=false 时仍产生 counter。
- dry-run=true 时 counter 被增加。
- 同一 request ID 写入多条成功 attribution。
- 上游失败但企业 counter 没有回滚。

### SQL 排查示例

按策略看当日 counter：

```sql
select policy_id, metric, used_value, period_start, period_end
from enterprise_quota_counters
where period_start >= ?
order by used_value desc;
```

按部门看用量：

```sql
select org_unit_id, count(*) as request_count, sum(quota) as quota
from enterprise_usage_attributions
where created_at >= ?
group by org_unit_id
order by quota desc;
```

按请求查归集：

```sql
select *
from enterprise_usage_attributions
where request_id = ?;
```

按对象查审计：

```sql
select actor_user_id, action, before_json, after_json, created_at
from enterprise_audit_logs
where target_type = ? and target_id = ?
order by created_at desc;
```

## 回滚流程

### 快速回滚

适用场景：

- 大量误拒绝。
- relay 错误率上升。
- counter 并发异常。
- 归集或结算出现明显错账。

操作：

1. 设置 `EnterpriseGovernanceEnabled=false`。
2. 保持代码版本不变，观察 relay 是否恢复。
3. 保留企业治理数据，不要删除 counter 或 attribution。
4. 导出异常时间窗口内的审计和日志。
5. 修复后重新从 dry-run 开始。

预期结果：

- 新请求不进入企业治理判断。
- 已有 counter/attribution 不再增加。
- 现有用户钱包、token 额度和渠道路由继续工作。

### 配置回滚

适用场景：

- 某条策略配置错误。
- 某个部门额度过低。
- 分组成员误加。

操作：

1. 停用错误策略或恢复上一版配置。
2. 检查审计日志确认变更人和变更内容。
3. 如需补偿用户，走现有额度调整流程。
4. 记录事故说明。

不建议：

- 直接改 counter 值来“修复”配置错误。
- 直接删除审计日志。
- 直接删除 attribution。

### 代码回滚

适用场景：

- 关闭开关后仍影响 relay。
- 迁移或模型定义导致启动失败。
- 发现严重安全问题。

操作：

1. 先关闭 enabled。
2. 回滚应用版本。
3. 不回滚数据库表，除非确认旧版本无法容忍新增表。
4. 后续修复以兼容已有企业治理表为前提。

## 事故处理

| 现象 | 第一动作 | 排查重点 | 临时处理 |
| --- | --- | --- | --- |
| 大量 403 | 关闭 dry-run 或 enabled 前先确认是否真实超额 | 策略 limit、target、period、timezone | 停用错误策略 |
| dry-run 下用户仍被拒绝 | 检查 caller 是否错误使用 decision | relay 接入点、dry-run 分支 | enabled=false |
| 额度明显超卖 | 检查 counter 事务和并发锁 | unique key、事务隔离、重试 | 降低并发或关闭 enabled |
| attribution 缺失 | 检查请求后结算路径 | stream、错误返回、billing session | 保留日志后补偿归集 |
| counter 高于实际很多 | 检查失败回滚 | upstream error、billing preconsume failure | 暂停硬限制 |
| 管理 API 误开放 | 检查路由 auth | AdminAuth/RootAuth | 下线路由或加鉴权 |

## 管理员操作准则

完整配置步骤见 `docs/enterprise-governance-admin-guide.md`。

创建策略前：

- 先确认目标是企业、部门、策略分组还是单个用户。
- 先用 request count 小范围验证，再扩大到 quota。
- 指定模型列表时确认模型名称和前端展示一致。
- 对核心部门先 dry-run 至少一个工作日。

调整策略时：

- 优先停用旧策略并创建新策略，保留审计链路。
- 避免在周期中频繁降低 limit。
- 如果必须降低 limit，提前通知受影响部门。

处理用户反馈时：

- 用 request ID 查 policy IDs。
- 查用户所属部门和策略分组。
- 查策略周期和已用值。
- 给用户返回简短原因，给管理员保留完整链路。

## MVP 发布验收清单

必须全部满足：

- enabled=false 时现有 relay 回归通过。
- dry-run=true 时不会拒绝用户请求。
- dry-run 记录能说明 would reject 原因。
- hard limit 超额返回 403。
- 超额请求不会打到上游。
- 上游失败会回滚企业预占。
- 成功请求会写 attribution。
- 管理 API 写操作有审计。
- 管理 API 都有鉴权。
- 报表可按部门、分组、用户聚合。

可以后置：

- Redis 原子计数优化。
- 项目/成本中心。
- 审批和临时额度。
- SSO 组织同步。
- token 级硬限制。
- 复杂策略动作，如降级模型和排队。

## 发布沟通模板

发布前：

```text
本次将发布企业组织与额度治理能力。发布后默认关闭，不影响现有调用。
我们会先在 dry-run 模式观测策略命中，再按部门灰度启用硬限制。
```

启用 dry-run：

```text
企业治理已进入 dry-run 观测阶段。请求不会被拒绝，但系统会记录哪些请求在硬限制下会被拦截。
请管理员关注部门额度策略和 would reject 数据。
```

启用硬限制：

```text
企业治理硬限制将在指定部门启用。超过策略额度的请求会返回额度限制错误。
如遇异常，请提供 request ID，管理员可根据审计链路排查。
```

回滚：

```text
企业治理开关已临时关闭，现有调用恢复到原有额度和计费逻辑。
我们会保留相关审计与归集数据，用于定位问题后重新灰度。
```
