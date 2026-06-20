# Enterprise Organization And Quota Task Plan

本文档把 `docs/enterprise-org-quota-plan.md` 拆成后续可执行任务。规划原则是：

- 先做不会影响现有用户额度、令牌额度和渠道路由的基础设施。
- 所有企业治理能力默认关闭，打开后才进入请求链路。
- 每一阶段都要能独立验证，不把“组织管理 UI”和“调用链路拦截”绑死在同一个大改里。
- 第一版优先支持请求次数和系统 quota，token 明细、项目、审批、SSO 后置。

工程实施细节见 `docs/enterprise-org-quota-implementation-blueprint.md`。
MVP 交付总入口见 `docs/enterprise-org-quota-mvp-delivery-plan.md`。
CEL 条件层和策略引擎开发目标见 `docs/enterprise-org-quota-cel-policy-engine-plan.md`。
发布、灰度和回滚流程见 `docs/enterprise-org-quota-rollout-runbook.md`。
管理员首条策略配置和拒绝排查见 `docs/enterprise-governance-admin-guide.md`。

## 任务总览

| 阶段 | 目标 | 主要交付 |
| --- | --- | --- |
| E0 | 设计冻结和风险收口 | 口径确认、数据模型定稿、服务接口草案、开关策略 |
| E1 | 数据底座 | 企业、部门、成员、策略分组、策略、计数器、审计表 |
| E2 | 后端管理 API | 组织、成员、策略分组、额度策略 CRUD 和审计 |
| E3 | 策略引擎 | 上下文解析、CEL 条件层、策略匹配、权限判断、额度预占和结算 |
| E4 | Relay 接入 | 请求前检查、请求后归集、默认关闭、可回滚 |
| E5 | 管理端 UI | 企业治理模块、组织架构、策略分组、额度策略、报表 |
| E6 | 测试和发布 | 并发测试、回归测试、迁移验证、灰度开关、操作文档 |
| E7 | 下一阶段能力 | 项目成本中心、审批、SSO、降级、异常检测 |

## 可排期 Backlog

下面的 Backlog 用于把阶段拆成可以进入迭代的工单。建议每个任务独立 PR，
除非任务本身是纯文档或纯类型补全。

| ID | 优先级 | 任务 | 依赖 | 主要验收 |
| --- | --- | --- | --- | --- |
| EG-001 | P0 | 冻结企业治理 MVP 口径和功能开关 | 无 | 明确默认关闭、dry-run、首版指标、超额动作 |
| EG-002 | P0 | 定义企业治理后端类型和策略服务接口 | EG-001 | relay 可调用接口，不依赖 controller |
| EG-003 | P0 | 新增企业治理系统选项 | EG-001 | `EnterpriseGovernanceEnabled` 默认关闭 |
| EG-004 | P0 | 新增核心数据模型和迁移 | EG-001 | 企业、部门、成员、分组、策略、计数器、归集、审计表可迁移 |
| EG-005 | P0 | 默认企业初始化 | EG-004 | 新库/老库启动都有默认企业 |
| EG-006 | P0 | 企业审计日志基础函数 | EG-004 | 管理操作可记录 before/after |
| EG-007 | P1 | 企业和部门 CRUD API | EG-004, EG-006 | 部门树创建、编辑、移动、停用和环检测可用 |
| EG-008 | P1 | 成员部门归属 API | EG-007 | 用户可绑定主部门，可查询未分配成员 |
| EG-009 | P1 | 策略分组 CRUD 和成员 API | EG-004, EG-006 | 分组可维护成员，停用后不参与策略 |
| EG-010 | P1 | 额度策略 CRUD API | EG-004, EG-006 | 企业/部门/分组/用户策略可保存和停用 |
| EG-011 | P0 | 组织上下文解析服务 | EG-005, EG-008, EG-009 | 可解析企业、部门祖先、策略分组 |
| EG-012 | P0 | CEL 条件层和结构化条件 schema | EG-010, EG-011 | UI 条件可生成 CEL，保存前编译校验 |
| EG-013 | P0 | 策略候选加载和条件匹配服务 | EG-012 | 命中企业、部门祖先、分组、用户策略，并按 CEL 条件过滤 |
| EG-014 | P0 | 模型权限判断 | EG-013 | 多策略模型范围取交集，拒绝原因可解释 |
| EG-015 | P0 | 周期计数器和预占/结算 | EG-013 | 请求前预占、完成后结算、失败回滚 |
| EG-016 | P0 | dry-run 决策日志 | EG-003, EG-015 | dry-run 记录本应拒绝，不影响请求 |
| EG-017 | P0 | relay 请求前企业治理检查 | EG-003, EG-015 | 开关关闭零变化，开启后可拒绝超额请求 |
| EG-018 | P0 | relay 请求后企业用量归集 | EG-017 | 真实 quota 写入归集表，可按 request ID 追踪 |
| EG-019 | P1 | 企业治理错误码和 i18n | EG-017 | 用户看到明确原因，管理员日志含策略 ID |
| EG-020 | P1 | 用量 summary/breakdown API | EG-018 | 可按部门、分组、用户聚合请求数和 quota |
| EG-021 | P1 | 审计日志查询 API | EG-006 | 可筛选操作人、目标、动作、时间 |
| EG-022 | P1 | 管理端导航和模块壳 | EG-007 | 管理员可见“企业治理”，非管理员不可见 |
| EG-023 | P1 | 组织架构 UI | EG-007, EG-008, EG-022 | 部门树和成员移动可用 |
| EG-024 | P1 | 策略分组 UI | EG-009, EG-022 | 分组和成员维护可用 |
| EG-025 | P1 | 额度策略 UI | EG-010, EG-012, EG-022 | 策略创建、编辑、启停和结构化条件可用 |
| EG-026 | P1 | 用量报表 UI | EG-020, EG-022 | 部门/分组/用户排行可用 |
| EG-027 | P2 | 审计日志 UI | EG-021, EG-022 | 可查看管理变更和策略拒绝 |
| EG-028 | P0 | 后端并发和回归测试 | EG-015, EG-017 | 并发不会明显突破上限，关闭开关零变化 |
| EG-029 | P1 | 前端类型、布局和空状态检查 | EG-023, EG-024, EG-025, EG-026 | typecheck 通过，关键页面无溢出 |
| EG-030 | P1 | 迁移、灰度和回滚验证 | EG-017, EG-018 | 老库升级、dry-run、关闭开关可回滚 |
| EG-031 | P1 | 企业治理操作文档 | EG-025, EG-026 | 管理员可按文档配置第一个策略 |

## 建议 PR 切分

### PR 1: 数据底座和开关

范围：

- EG-001
- EG-003
- EG-004
- EG-005
- EG-006

不做：

- 不接 relay。
- 不做 UI。
- 不做策略匹配。

验收命令：

```bash
go test ./model ./common
git diff --check
```

### PR 2: 组织和策略管理 API

范围：

- EG-007
- EG-008
- EG-009
- EG-010
- EG-021

不做：

- 不做请求拦截。
- 不做用量报表。

验收命令：

```bash
go test ./model ./controller ./router
git diff --check
```

当前进度：

- 已补部门移动环检测、部门停用保护、成员主部门分配、策略分组引用保护、额度策略审计、审计筛选和用量报表 controller 测试。
- 已补企业治理路由 `AdminAuth` 边界测试，覆盖未登录、普通用户和管理员访问 `/api/enterprise/current`。
- 已覆盖 `go test ./controller -run 'TestEnterprise' -count=1`。
- 已覆盖 `go test ./router -run TestEnterpriseRoutesRequireAdminAuth -count=1`。

### PR 3: 策略引擎内核

范围：

- EG-002
- EG-011
- EG-012
- EG-013
- EG-014
- EG-015
- EG-016

不做：

- 不接真实 relay，只做服务和单元测试。
- 不做前端。

验收命令：

```bash
go test ./model ./service
git diff --check
```

### PR 4: Relay dry-run 接入

范围：

- EG-017 的 dry-run 路径。
- EG-018 的归集雏形。
- EG-019 的基础错误码。

不做：

- 不默认开启硬拒绝。
- 不做 UI 报表。
- 不在 PR 4 完成 reservation 的硬限制结算生命周期。

验收命令：

```bash
go test ./controller ./service ./relay ./router
git diff --check
```

如果仓库没有独立 `./relay` package，则按实际 relay 所在 package 替换命令。

当前进度：

- 已完成 relay dry-run 前置接入：`controller/relay.go` 在价格预估后调用服务层检查。
- 已完成 dry-run would reject 审计和成功请求 attribution 雏形。
- 已完成只读 `CheckEnterpriseQuota`，用于 dry-run 判断且不写 counter。
- 已覆盖 service/controller/model/router 定向测试；hard limit reservation、报表 API 和发布前核验脚本进入 PR 5/7 收口。

### PR 5: 硬限制和报表 API

范围：

- EG-017 的硬拒绝路径。
- EG-018 完整结算。
- EG-020。
- EG-028。
- EG-030 的后端部分。

验收命令：

```bash
go test ./model ./controller ./service ./router
git diff --check
```

当前进度：

- 已实现 hard-limit 模式下的企业 reservation 预占。
- 已实现 relay 错误路径 reservation refund，包含现有 billing preconsume 失败场景。
- 已实现成功请求后的企业 reservation settle。
- 已实现 summary/breakdown API，支持时间范围、部门、策略分组、用户、模型和状态筛选，并支持按 quota/request_count 等字段排序。
- 已增加 hard-limit 企业治理错误码，并按策略目标区分企业、部门、策略分组和用户额度超限。
- 已补企业治理拒绝的中文、英文和繁体中文 i18n 文案，用户消息保持简洁，内部诊断保留策略 ID。
- 已补模型权限白名单交集判断，模型不允许返回专用错误码且不进入额度预占。
- 已补后端并发边界单测，验证并发 reservation 不突破策略 limit。
- 已补 R0-R3 本地 runbook smoke 和 `scripts/enterprise-governance-preflight.sh` 发布前核验脚本，真实预发/生产灰度验证仍需在上线窗口执行。

### PR 6: 管理端 UI

范围：

- EG-022
- EG-023
- EG-024
- EG-025
- EG-026
- EG-027
- EG-029

验收命令：

```bash
cd web/default && pnpm typecheck
git diff --check
```

### PR 7: 发布收口

范围：

- EG-030。
- EG-031。
- 更新 operator 文档入口。
- 按 `docs/enterprise-org-quota-rollout-runbook.md` 完成 dry-run、硬限制和回滚演练。

验收命令：

```bash
go test ./model ./controller ./service ./router
env GOCACHE=$PWD/.cache/go-build GOTMPDIR=$PWD/.cache/go-tmp go test ./service -run TestEnterpriseGovernanceRolloutRunbookR0ToR3 -count=1
cd web/default && pnpm typecheck
git diff --check
```

## E0: 设计冻结

### E0-1 确认业务口径

目标：在实现前冻结最容易返工的口径。

任务：

- 确认 MVP 只支持单企业，但所有表保留 `enterprise_id`。
- 确认组织部门和现有 `User.Group` 分离。
- 确认策略分组命名为 `policy_group`，不使用 `group` 做表名或 API 主概念。
- 确认第一版指标：`request_count`、`quota`。
- 确认 token 明细只记录在归集表，暂不作为第一版强拦截指标。
- 确认超额动作第一版只有 `deny`。
- 确认企业治理默认关闭，关闭时请求链路零行为变化。

验收：

- `docs/enterprise-org-quota-plan.md` 中 MVP 口径和本任务一致。
- 没有未收口决策会阻塞 E1 数据模型设计。

### E0-2 定义策略服务接口

目标：先定义调用链路需要的服务边界，避免 CRUD 和 relay 互相渗透。

建议接口：

```go
type EnterprisePolicyService interface {
    ResolveContext(ctx context.Context, userID int, tokenID int, modelName string) (*EnterpriseContext, error)
    Evaluate(ctx context.Context, req PolicyEvaluationRequest) (*PolicyDecision, error)
    Reserve(ctx context.Context, decision *PolicyDecision, estimated UsageAmount) (*Reservation, error)
    Settle(ctx context.Context, reservation *Reservation, actual UsageAmount) error
    Refund(ctx context.Context, reservation *Reservation, reason string) error
}
```

任务：

- 定义 `EnterpriseContext`：用户、企业、主部门、部门祖先、策略分组、API Key。
- 定义 `UsageAmount`：请求次数、quota、prompt tokens、completion tokens、total tokens。
- 定义 `PolicyDecision`：允许/拒绝、命中策略、拒绝原因、模型权限结果。
- 定义 `Reservation`：预占到的策略计数器和预估量。

验收：

- 接口可以被 relay 调用，不依赖 Web 控制器。
- 接口可以在企业治理关闭时快速返回允许。
- 接口可以被单元测试直接构造上下文验证。

### E0-3 定义功能开关

目标：确保上线风险可控。

任务：

- 新增系统选项：`EnterpriseGovernanceEnabled`。
- 新增可选调试选项：`EnterpriseGovernanceDryRunEnabled`。
- dry-run 模式只记录会命中的策略和会拒绝的原因，不实际拒绝请求。
- 状态接口和管理设置页后续可读取开关。

验收：

- 默认关闭。
- 关闭时不会查询企业策略表。
- dry-run 可用于灰度观察。

## E1: 数据底座

### E1-1 新增模型和迁移

目标：建立企业治理核心表。

任务：

- 新增 `Enterprise` 模型。
- 新增 `OrgUnit` 模型。
- 新增 `OrgMembership` 模型。
- 新增 `PolicyGroup` 模型。
- 新增 `PolicyGroupMember` 模型。
- 新增 `QuotaPolicy` 模型。
- 新增 `QuotaCounter` 模型。
- 新增 `UsageAttribution` 模型。
- 新增 `EnterpriseAuditLog` 模型。
- 加入 `AutoMigrate`。

验收：

- SQLite、MySQL、PostgreSQL 自动迁移通过。
- 表名和字段不与现有 `group`、`quota` 概念冲突。
- 必要索引存在：`enterprise_id`、`user_id`、`org_unit_id`、`policy_id`、周期字段。

### E1-2 初始化默认企业

目标：兼容现有单租户部署。

任务：

- 启动时确保存在默认企业。
- 可选创建默认根部门。
- 新用户默认归属默认企业，部门可为空。
- 已有用户不强制迁移到具体部门，避免破坏现有用户列表。

验收：

- 新库启动自动生成默认企业。
- 老库升级后不需要手工补数据即可启动。
- 没有部门的用户在企业治理开启后仍可按企业级策略命中。

### E1-3 审计写入基础函数

目标：后续所有管理操作共用审计入口。

任务：

- 新增 `RecordEnterpriseAuditLog`。
- 支持记录 actor、action、target、before、after。
- JSON 序列化使用项目统一 helper。
- 对敏感字段做脱敏，例如未来的同步凭证。

验收：

- 单元测试覆盖 create/update/delete 三类审计。
- 审计写入失败不应导致主要业务半提交；需要明确事务策略。

## E2: 后端管理 API

### E2-1 企业和部门 API

目标：提供组织架构管理能力。

任务：

- `GET /api/enterprise/current`
- `PUT /api/enterprise/current`
- `GET /api/enterprise/org-units`
- `POST /api/enterprise/org-units`
- `PUT /api/enterprise/org-units/:id`
- `DELETE /api/enterprise/org-units/:id`
- 支持部门停用，不建议物理删除有成员或子部门的节点。
- 支持移动部门时更新 `path`、`depth`。

验收：

- root 或管理员可管理。
- 非管理员不可管理。
- 移动部门不能形成环。
- 删除有子部门或成员的部门会给明确错误。
- 所有变更写审计日志。

### E2-2 成员归属 API

目标：允许管理员把用户放入部门。

任务：

- `GET /api/enterprise/members`
- `PUT /api/enterprise/members/:user_id/org-unit`
- 支持按部门、用户名、邮箱筛选成员。
- 支持未分配部门用户列表。
- 第一版只允许一个主部门。

验收：

- 用户移动部门后，后续策略上下文能解析新部门。
- 用户禁用或删除时不破坏成员关系查询。
- 成员变更写审计日志。

### E2-3 策略分组 API

目标：支持跨部门用户集合。

任务：

- `GET /api/enterprise/policy-groups`
- `POST /api/enterprise/policy-groups`
- `PUT /api/enterprise/policy-groups/:id`
- `DELETE /api/enterprise/policy-groups/:id`
- `POST /api/enterprise/policy-groups/:id/members`
- `DELETE /api/enterprise/policy-groups/:id/members/:user_id`
- 支持查看分组成员和成员数量。

验收：

- 不允许重名或 slug 冲突。
- 停用分组后策略匹配不再使用该分组。
- 成员增删写审计日志。

### E2-4 额度策略 API

目标：支持 MVP 策略配置。

任务：

- `GET /api/enterprise/quota-policies`
- `POST /api/enterprise/quota-policies`
- `PUT /api/enterprise/quota-policies/:id`
- `DELETE /api/enterprise/quota-policies/:id`
- 支持目标类型：企业、部门、策略分组、用户。
- 支持周期：每日、每月。
- 支持指标：请求次数、quota。
- 支持模型范围：全部、指定模型列表。
- 支持动作：拒绝。

验收：

- limit 必须为正数。
- target 必须存在且属于同一企业。
- 指定模型列表为空时不能保存为指定模型范围。
- 策略停用后不参与匹配。
- 策略变更写审计日志。

### E2-5 用量和审计查询 API

目标：给 UI 和运维排查提供基础数据。

任务：

- `GET /api/enterprise/usage/summary`
- `GET /api/enterprise/usage/breakdown`
- `GET /api/enterprise/audit-logs`
- 支持周期筛选。
- 支持部门、分组、用户筛选。
- 支持按 quota、请求数排序。

验收：

- 可看部门、分组、用户排行。
- 查询不会暴露非管理员不可见数据。
- 大时间范围有分页或聚合保护。

## E3: 策略引擎

### E3-1 组织上下文解析

目标：从用户请求解析出策略所需上下文。

任务：

- 根据用户 ID 获取企业。
- 获取主部门。
- 获取部门祖先链。
- 获取策略分组。
- 获取 API Key 和现有运行分组。
- 缓存短期上下文，成员或策略变更后失效。

验收：

- 无部门用户仍能解析企业上下文。
- 分组停用后不会进入上下文。
- 上下文解析有单元测试。

### E3-2 CEL 条件层

目标：让 UI 结构化条件可以生成、校验并执行 CEL 表达式。

任务：

- 引入 CEL 依赖，并封装在 `service/enterprise_policy_condition.go`。
- 新增结构化条件 schema，如 ability、runtime group、channel、playground、模型前缀。
- 为 `EnterpriseQuotaPolicy` 增加 `condition_mode`、`condition_json`、`condition_expr`、`condition_hash`。
- 将 UI 表单条件生成稳定 CEL 表达式。
- 保存策略前编译和类型检查表达式，要求返回 bool。
- 运行时缓存已编译表达式。

验收：

- 空结构化条件生成 `true`。
- 非 bool 表达式不能保存。
- 条件数组去重并稳定排序。
- CEL input 不包含 prompt、completion、API key 或密钥。

### E3-3 策略候选和条件匹配

目标：找出一次请求适用的所有策略，并按 CEL 条件过滤。

任务：

- 匹配企业策略。
- 匹配主部门及祖先部门策略。
- 匹配策略分组策略。
- 匹配用户策略。
- 过滤停用、未开始、已过期策略。
- 对候选策略执行 CEL condition eval。
- 过滤模型范围不匹配策略。

验收：

- 父部门策略会对子部门用户生效。
- 用户同时在多个策略分组时全部匹配。
- CEL 条件不匹配的策略不会进入 counter。
- 策略匹配结果稳定排序，便于测试和审计。

### E3-4 模型权限判断

目标：在额度前先判断能不能用这个模型。

任务：

- 支持全部模型。
- 支持指定模型列表。
- 支持多个策略取交集。
- 记录拒绝原因，例如“当前用户不在高阶模型白名单”。

验收：

- 白名单外模型会拒绝。
- 没有模型限制的策略不会误拒绝。
- 拒绝原因可显示给管理员，用户看到简洁原因。
- 模型不允许不会进入额度预占。

### E3-5 额度预占和结算

目标：并发安全地控制周期额度。

任务：

- 计算当前周期开始和结束时间，默认使用系统时区或策略时区。
- 对每个命中策略建立或获取计数器。
- 请求前预占 `request_count=1` 和预估 quota。
- 请求完成后用实际 quota 修正。
- 请求失败时回滚预占或按现有计费结果结算。
- Redis 可用时使用原子计数；第一版也要有数据库事务回退。

验收：

- 并发请求不会明显突破上限。
- 预占失败会回滚已预占的其他计数器。
- 结算失败有日志并可重试或补偿。
- 单元测试覆盖日/月周期边界。

### E3-6 决策审计和 dry-run

目标：让策略结果可解释。

任务：

- 记录命中策略 ID。
- 记录拒绝策略 ID 和原因。
- dry-run 模式写入“本应拒绝”的审计或观测日志。
- 不在普通日志里泄露敏感 prompt。

验收：

- 管理员能从 request ID 查到策略命中链路。
- dry-run 不影响请求成功率。

## E4: Relay 接入

### E4-1 请求前接入

目标：在模型请求发出前执行企业策略检查。

任务：

- 在现有 relay 计费预扣前后选择稳定插入点。
- 企业治理关闭时直接跳过。
- 企业治理开启时调用策略服务。
- 拒绝时返回明确错误码和用户可读信息。

验收：

- 关闭开关时现有 relay 测试不变。
- dry-run 开启后，超额请求仍继续打上游，但会记录 would reject。
- hard limit 开启后，超额请求不会打到上游。
- API Key 调用和 Web 调用都覆盖。

### E4-2 请求后接入

目标：按实际消耗结算并归集。

任务：

- 请求成功后 settle。
- 请求失败后 refund 或 settle 为失败消耗。
- 写入 `enterprise_usage_attributions`。
- 关联现有日志 request ID。

验收：

- attribution 中能按 request ID 看到成功请求的真实消耗。
- 失败请求的处理和现有计费规则一致。
- 结算异常不会导致响应悬挂。

### E4-3 错误和国际化

目标：错误信息对用户清楚，对管理员可诊断。

任务：

- 新增错误码：企业治理拒绝、部门额度不足、策略分组额度不足、模型不允许。
- 中文、英文、繁体中文 i18n。
- 用户消息简洁，管理员日志包含策略 ID。

验收：

- 前端 toast 或 API 响应能展示明确原因。
- 日志中能定位策略。

## E5: 管理端 UI

### E5-1 导航和模块入口

目标：新增企业治理管理模块。

任务：

- 管理侧边栏加入“企业治理”。
- 子页面：组织架构、策略分组、额度策略、用量报表、审计日志。
- 只对管理员显示。

验收：

- 非管理员不可见且接口不可访问。
- 页面空状态清晰。

### E5-2 组织架构页面

目标：管理部门树和成员。

任务：

- 左侧部门树。
- 右侧部门详情。
- 成员列表和搜索。
- 移动用户到部门。
- 创建、编辑、停用部门。

验收：

- 长部门名和深层级在桌面/移动都不溢出。
- 移动用户后页面立即刷新。

### E5-3 策略分组页面

目标：管理跨部门分组。

任务：

- 分组列表。
- 创建/编辑分组。
- 成员管理弹窗或抽屉。
- 显示关联策略数量。

验收：

- 可快速查找用户并加入分组。
- 停用分组有确认提示。

### E5-4 额度策略页面

目标：让管理员配置 MVP 策略。

任务：

- 策略表格。
- 创建/编辑抽屉。
- 目标类型和目标对象联动选择。
- 周期、指标、上限、模型范围、状态字段。
- 策略启停。

验收：

- 表单校验覆盖必填和正数。
- 指定模型范围可多选。
- 保存后策略立即进入后端匹配。

### E5-5 用量报表页面

目标：查看部门、分组、用户消耗。

任务：

- 周期筛选。
- 维度切换：部门、策略分组、用户。
- 指标切换：请求数、quota。
- 排行表格。
- 超额/接近超额标识。

验收：

- 数据和后端 summary 一致。
- 无数据时有明确空状态。

### E5-6 审计日志页面

目标：追踪管理变更和策略拒绝。

任务：

- 操作人、操作类型、目标类型筛选。
- 展示 before/after 摘要。
- 支持 request ID 搜索策略拒绝记录。

验收：

- 关键变更可追踪到操作者和时间。
- JSON 详情不会撑破页面。

## E6: 测试和发布

### E6-1 后端测试

任务：

- 模型迁移测试。
- 组织树移动和环检测测试。
- 策略匹配测试。
- 模型权限交集测试。
- 日/月周期计数测试。
- 并发预占测试。
- relay 关闭开关回归测试。
- relay 开启拦截测试。

验收：

- `go test ./model ./controller ./service ./router` 通过。
- 企业治理关闭时关键现有测试不需要改断言。

### E6-2 前端测试和类型检查

任务：

- 企业治理 API 类型。
- 表单校验。
- 空状态和错误状态。
- 响应式布局检查。

验收：

- `cd web/default && pnpm typecheck` 通过。
- 关键页面没有明显溢出和不可点击控件。

### E6-3 迁移和回滚验证

任务：

- 老库升级验证。
- 新库初始化验证。
- 企业治理关闭回滚验证。
- dry-run 灰度验证。

验收：

- 迁移不会要求现有用户手工补字段。
- 关闭开关后请求链路恢复现状。

### E6-4 操作文档

任务：

- 更新企业治理配置说明。
- 写入常见策略示例。
- 写入排障说明：为什么请求被拒绝、如何查命中策略。
- 写入灰度建议。

验收：

- 管理员可以按文档创建第一个部门和策略。
- 文档包含关闭/回滚路径。

## E7: MVP 后任务池

这些任务不要进入第一版主线，但设计时要留接口余量。

### E7-1 项目和成本中心

- 新增项目表。
- API Key 绑定默认项目。
- 请求可传项目 ID。
- 报表增加项目维度。

### E7-2 审批和临时额度

- 超额申请。
- 审批人配置。
- 临时额度包。
- 到期自动失效。

### E7-3 SSO 组织同步

- LDAP。
- 企业微信。
- 飞书。
- 钉钉。
- Okta。
- 同步冲突和离职处理。

### E7-4 高级策略动作

- 自动降级模型。
- 低优先级排队。
- 借用企业共享池。
- 异常检测后自动限流。

### E7-5 更细 RBAC

- 企业管理员。
- 部门管理员。
- 财务查看员。
- 审计员。
- 项目管理员。

## 推荐执行顺序

第一批建议只做到“后端可验证”：

1. E0-1 到 E0-3：冻结口径和开关。
2. E1-1 到 E1-3：建表、默认企业、审计基础。
3. E2-1 到 E2-4：组织和策略 CRUD。
4. E3-1 到 E3-4：策略引擎单元测试跑通。

第二批再接入真实请求链路：

1. E4-1：请求前检查，先 dry-run。
2. E4-2：请求后结算和归集。
3. E4-3：错误码和 i18n。
4. E6-1：并发和回归测试。

第三批做管理 UI：

1. E5-1：导航入口。
2. E5-2：组织架构。
3. E5-3：策略分组。
4. E5-4：额度策略。
5. E5-5：用量报表。
6. E5-6：审计日志。

最后做发布收口：

1. E6-2：前端类型和布局检查。
2. E6-3：迁移、灰度、回滚验证。
3. E6-4：操作文档。

## 第一轮开发建议

如果要现在开工，建议第一个代码任务是：

> 添加企业治理数据模型、默认企业初始化、基础审计写入函数，并保持功能开关默认关闭。

原因：

- 不触碰 relay 主链路，风险低。
- 后续 API、策略引擎、UI 都依赖这些表。
- 可以较早发现迁移和命名冲突问题。

第一轮验收命令建议：

```bash
go test ./model ./common
git diff --check
```
