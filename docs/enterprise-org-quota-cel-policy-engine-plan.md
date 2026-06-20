# Enterprise Organization And Quota CEL Policy Engine Plan

本文档规划企业组织与额度治理的策略引擎开发目标。结论是采用“结构化 UI 配置 +
CEL 条件层 + Go 额度账本”的混合方案。

配套文档：

- `docs/enterprise-org-quota-mvp-delivery-plan.md`：MVP 交付总入口。
- `docs/enterprise-org-quota-task-plan.md`：阶段、Backlog、PR 切分。
- `docs/enterprise-org-quota-implementation-blueprint.md`：模型、API 和 relay 接入细节。

## 开发目标

PR3 策略引擎要达成这些目标：

1. 管理员通过 UI 表单配置策略，不需要理解 CEL。
2. 后端把结构化条件生成可检查、可执行的 CEL 表达式。
3. 高级管理员后续可以选择直接写 CEL，但 MVP 不要求开放编辑器。
4. CEL 只判断“策略是否适用于本次请求”，不处理额度扣减。
5. Go service 继续负责周期、counter、预占、回滚、结算和 dry-run。
6. 策略表达式保存前必须编译和类型检查，运行时只执行已校验表达式。
7. 运行时表达式要可缓存、可解释、可审计。

## 依赖选择

推荐使用 CEL 官方 Go 实现。当前官方仓库已经从 `google/cel-go` 迁移到
`cel-expr/cel-go`，但 Go package 文档仍以 `github.com/google/cel-go/cel`
展示 import 示例。实际实现前要以项目 `go get` 和 `go list -m` 能解析的 module
path 为准。

选择原因：

- CEL 是非图灵完备表达式语言，适合嵌入应用做安全判断。
- 官方定位就是性能关键路径中的快速、安全、可移植表达式。
- 适合“表达式修改少、请求评估频繁”的策略场景。
- 比 OPA/Rego 更轻，比自研表达式语法更稳。

不选择：

- OPA/Rego：适合跨系统 policy-as-code，当前单体网关 MVP 偏重。
- Casbin：适合 RBAC，不适合条件表达式和额度账本。
- Cedar：语义强但引入成本高，当前 Go 生态和 UI 方案不如 CEL 直接。
- Expr：Go 体验好，但跨语言标准化和生态不如 CEL。

## 总体架构

```text
UI structured condition
  -> ConditionJson
  -> backend NormalizeCondition
  -> backend BuildCELExpression
  -> cel Compile + Check
  -> save ConditionExpr + ConditionAstJson optional
  -> runtime cache compiled program
  -> Eval(policy, request context)
  -> matched policies
  -> Go counter reserve/settle/refund
```

边界：

- CEL 返回 bool，表示策略条件是否命中。
- Go 根据命中策略继续做模型权限、额度预占和结算。
- CEL 不直接读数据库。
- CEL 不暴露 prompt、completion、API key、用户密钥。
- CEL 不调用网络、不读文件、不执行副作用函数。

## 数据模型目标

在 `enterprise_quota_policies` 增加字段：

```go
ConditionMode string // structured | cel
ConditionJson string // UI 表单条件 JSON
ConditionExpr string // 生成或手写 CEL
ConditionHash string // 表达式缓存 key，可选
```

MVP 建议：

- `ConditionMode` 默认 `structured`。
- `ConditionJson` 存 UI 表单条件。
- `ConditionExpr` 存后端生成表达式。
- 不急着保存 checked AST；先运行时编译缓存即可。
- 现有 `model_scope`、`models_json` 保留为一级字段，避免 UI 和旧策略迁移复杂化。

后续可以把 `model_scope`、`models_json` 逐步归入 `ConditionJson`，但不在 PR3 做。

## UI 结构化条件

普通管理员只看到表单，不看到 CEL。

第一版条件字段：

```json
{
  "abilities": ["chat", "image"],
  "models": ["gpt-4o", "claude-sonnet-4"],
  "model_prefixes": ["gpt-4", "claude"],
  "runtime_groups": ["default", "vip"],
  "channel_ids": [1, 2, 3],
  "is_playground": false,
  "token_ids": [10, 11],
  "metadata": {
    "source": "api"
  }
}
```

字段语义：

| 字段 | 语义 | MVP |
| --- | --- | --- |
| `abilities` | chat/image/audio/video 等能力类型 | 可做 |
| `models` | 指定模型白名单 | 已有字段优先，ConditionJson 可同步 |
| `model_prefixes` | 模型名前缀匹配 | 可做 |
| `runtime_groups` | 现有 `User.Group` | 可做 |
| `channel_ids` | 指定渠道 | 可做 |
| `is_playground` | 是否 playground | 可做 |
| `token_ids` | 指定 API Key | 可做 |
| `metadata` | 预留扩展 | 后置 |

UI 表单建议：

- “适用能力”：多选。
- “模型范围”：沿用现有全部/指定模型。
- “模型前缀”：高级折叠项。
- “运行分组”：多选现有 runtime group。
- “渠道”：多选或输入 ID。
- “Playground”：全部/仅 playground/排除 playground。
- “API Key”：后置，不进入首版 UI 主路径。

## CEL 上下文

运行时只暴露稳定上下文：

```go
type EnterprisePolicyCELInput struct {
    User    EnterprisePolicyCELUser
    Org     EnterprisePolicyCELOrg
    Request EnterprisePolicyCELRequest
    Token   EnterprisePolicyCELToken
}

type EnterprisePolicyCELUser struct {
    Id           int
    RuntimeGroup string
    Role         int
}

type EnterprisePolicyCELOrg struct {
    EnterpriseId    int
    OrgUnitId       int
    OrgUnitPathIds  []int
    PolicyGroupIds  []int
}

type EnterprisePolicyCELRequest struct {
    Model        string
    Ability      string
    IsPlayground bool
    ChannelId    int
}

type EnterprisePolicyCELToken struct {
    Id int
}
```

CEL 表达式示例：

```cel
request.model in ["gpt-4o", "claude-sonnet-4"] &&
user.runtime_group in ["default", "vip"] &&
request.is_playground == false
```

建议变量名使用 lower snake/camel：

- `user.id`
- `user.runtime_group`
- `user.role`
- `org.enterprise_id`
- `org.org_unit_id`
- `org.org_unit_path_ids`
- `org.policy_group_ids`
- `request.model`
- `request.ability`
- `request.is_playground`
- `request.channel_id`
- `token.id`

## 表达式生成规则

结构化条件转 CEL 时遵循：

- 空条件生成 `true`。
- 多个字段之间使用 `&&`。
- 同一字段多选使用 `in`。
- 模型前缀使用 `exists` 或 `startsWith`。
- 布尔字段显式生成 `== true/false`。
- 所有字符串都通过 JSON/CEL literal 转义，不手写拼接裸字符串。

示例：

```json
{
  "runtime_groups": ["vip"],
  "model_prefixes": ["gpt-4"],
  "is_playground": false
}
```

生成：

```cel
user.runtime_group in ["vip"] &&
["gpt-4"].exists(prefix, request.model.startsWith(prefix)) &&
request.is_playground == false
```

## 编译和缓存

保存策略时：

1. 校验 `ConditionJson` schema。
2. 生成 `ConditionExpr`。
3. 使用固定 CEL env 编译。
4. 要求表达式返回 bool。
5. 编译失败则拒绝保存。

运行时：

- cache key: `policy_id + condition_hash + updated_at`。
- cache value: checked AST/program。
- 策略更新后通过 updated_at 或显式 invalidation 失效。
- 编译失败不应发生；如果发生，策略视为不命中并记录 error。

性能目标：

- 编译只发生在保存或缓存 miss。
- 请求路径只 eval 已编译 program。
- Eval 输入是 map 或 typed adapter，优先选最少反射和最稳定的实现。

## 安全要求

- 不启用自定义函数，除非有明确测试。
- 不暴露 prompt、completion、API key、密钥、请求 body。
- 不允许表达式访问任意 metadata，MVP metadata 后置。
- 高级 CEL 编辑器只能 root 使用；普通管理员只用表单。
- 保存时限制表达式长度，例如 4096 字符。
- 保存时限制结构化条件数组长度，例如每个字段最多 100 项。
- 所有 eval error 写日志和审计，不向用户暴露内部表达式。

## 运行时决策流程

```text
ResolveContext
  -> Load candidate policies by enterprise/org/group/user/time/status
  -> For each policy:
       EvaluatePolicyCondition(policy, celInput)
       false: skip
       true: matched
  -> EvaluateModelAccess(matched policies)
  -> ReserveCounters(matched metric policies)
  -> Return decision/reservation
```

注意：

- target 匹配仍然用 Go SQL 完成，CEL 不负责找候选策略。
- 时间窗口、状态、target type、target id 仍然用 Go 过滤。
- CEL 只做候选策略内的附加条件。
- counter 仍然按命中策略逐条检查。

## 开发任务

### CEL-001: 依赖和封装

目标：引入 CEL 但隔离在 service 内。

任务：

- 增加 CEL 依赖。
- 新增 `service/enterprise_policy_condition.go`。
- 新增 `service/enterprise_policy_condition_test.go`。
- 提供 `CompilePolicyCondition`、`EvalPolicyCondition`。

验收：

- 表达式可编译。
- 非 bool 表达式拒绝。
- eval error 可返回。
- `go test ./service` 通过。

### CEL-002: 数据模型字段

目标：让策略表能保存结构化条件和 CEL 表达式。

任务：

- 给 `EnterpriseQuotaPolicy` 增加 `ConditionMode`、`ConditionJson`、`ConditionExpr`、`ConditionHash`。
- 更新 AutoMigrate。
- 更新创建/更新策略 API request。
- 默认旧策略 `ConditionExpr=true`。

验收：

- 老策略不配置条件仍命中。
- 新策略可保存 condition 字段。
- `go test ./model ./controller` 通过。

### CEL-003: 结构化条件 schema

目标：定义 UI 表单条件的稳定 JSON。

任务：

- 新增 `EnterprisePolicyStructuredCondition`。
- 校验数组长度、字符串长度、枚举值。
- 空条件规范化为 `{}`。
- 去重和排序，保证生成表达式稳定。

验收：

- 重复值被去重。
- 非法 ability/runtime group/channel id 被拒绝。
- 生成结果顺序稳定。

### CEL-004: 条件转表达式

目标：从结构化条件生成 CEL。

任务：

- 实现 `BuildCELExpressionFromCondition`。
- 字符串 literal 安全转义。
- 空条件返回 `true`。
- 支持 ability/model/model_prefix/runtime_group/channel/playground/token。

验收：

- 每个字段有单元测试。
- 多字段组合使用 `&&`。
- prefix 表达式正确。

### CEL-005: 策略匹配接入

目标：候选策略经过 CEL 条件过滤。

任务：

- 在 `MatchPolicies` 中加载候选策略。
- 对每条策略构造 CEL input。
- eval false 的策略跳过。
- eval true 的策略进入后续模型权限和 counter。
- 记录 condition eval error。

验收：

- 条件不匹配的策略不参与额度。
- 条件匹配的策略继续参与额度。
- eval error 不导致 panic。

### CEL-006: UI 配置目标

目标：前端策略表单能配置结构化条件。

任务：

- 在策略表单增加“适用条件”区域。
- 使用多选、开关、输入数组，不显示 CEL。
- 保存为 `condition_json`。
- 展示时反序列化回表单。

验收：

- 管理员无需写表达式。
- 条件保存后再次编辑不丢失。
- 最长文本和多选标签不溢出。

### CEL-007: 高级 CEL 模式后置

目标：为未来 root 高级表达式编辑器留接口。

任务：

- API 支持 `condition_mode=cel`。
- MVP UI 不开放或隐藏在 root-only feature flag。
- 后端校验 direct CEL 表达式。

验收：

- 非 root 不能保存 direct CEL。
- 表达式编译失败不能保存。
- 审计记录 direct CEL 变更。

## PR 拆分建议

建议把 PR3 拆成两段，降低风险：

### PR 3A: CEL 条件底座

范围：

- CEL 依赖。
- condition 数据字段。
- structured condition schema。
- condition -> CEL 生成。
- CEL compile/eval/cache。
- 单元测试。

不做：

- 不接 counter。
- 不接 relay。
- 不开放高级 CEL UI。

验收：

```bash
go test ./model ./controller ./service
git diff --check
```

### PR 3B: 策略匹配和额度预占

范围：

- 组织上下文解析。
- 候选策略加载。
- CEL 条件过滤。
- 模型权限判断。
- 周期 counter 预占/回滚。
- dry-run decision。

不做：

- 不接真实 relay。
- 不做硬限制返回。

验收：

```bash
go test ./model ./service
git diff --check
```

## 完成定义

PR3 完成时必须满足：

- 结构化条件能生成稳定 CEL。
- CEL 表达式保存前可编译和类型检查。
- 运行时能缓存已编译表达式。
- 策略匹配能按 CEL 条件过滤候选策略。
- counter 预占仍由 Go transaction 控制。
- dry-run 能返回 would reject 和命中策略 ID。
- prompt、completion、密钥不会进入 CEL input。
- `go test ./model ./controller ./service` 通过。

## 参考

- CEL 官网：`https://cel.dev/`
- cel-go GitHub：`https://github.com/google/cel-go`
- CEL 迁移公告：`https://opensource.googleblog.com/2026/06/cel-finds-a-new-home-at-githubcomcel-expr.html`
