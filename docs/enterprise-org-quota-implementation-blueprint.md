# Enterprise Organization And Quota Implementation Blueprint

本文档补充 `enterprise-org-quota-task-plan.md` 的工程实施细节，重点服务第一批
开发任务：数据底座、开关、审计、策略服务接口，以及后续 relay 接入点。
MVP 交付总入口见 `enterprise-org-quota-mvp-delivery-plan.md`。
CEL 条件层和策略引擎开发目标见 `enterprise-org-quota-cel-policy-engine-plan.md`。
发布、灰度、观测和回滚操作见 `enterprise-org-quota-rollout-runbook.md`。

## 当前代码基线

现有相关位置：

| 模块 | 当前位置 | 说明 |
| --- | --- | --- |
| 用户模型 | `model/user.go` | 已有 `User.Group`、`Quota`、`UsedQuota`、`RequestCount` |
| 迁移入口 | `model/main.go` | `migrateDB()` 统一 `AutoMigrate` |
| 系统选项 | `common/constants.go`, `model/option.go`, `controller/option.go` | 选项默认值、加载、更新、校验 |
| 状态接口 | `controller/misc.go` | 前端 status 暴露运行状态 |
| API 路由 | `router/api-router.go` | 管理 API 常用 `AdminAuth` 或 `RootAuth` |
| Relay 编排 | `controller/relay.go` | 请求校验、敏感词、token 预估、价格、预扣、上游调用 |
| Relay 适配 | `relay/` | 具体协议、上游渠道和流式响应处理 |
| 计费会话 | `service/billing_session.go` | 钱包/订阅预扣、结算、退款生命周期 |
| 计费事件 | `model/billing_event.go`, `service/billing_event.go` | 可作为用量归集和排障参考 |

重要边界：

- 企业组织分组不要复用 `User.Group`。`User.Group` 继续服务运行分组、模型倍率、
  渠道路由和已有日志筛选。
- 企业治理默认关闭；关闭时不能增加 relay 查询、不能影响计费路径。
- 企业策略预占应发生在价格预估之后，因为第一版需要用 `QuotaToPreConsume`。
- 企业策略结算应跟随现有计费结算结果，不重新计算用户钱包或订阅账。

## 建议文件布局

### model

```text
model/enterprise.go
model/enterprise_org_unit.go
model/enterprise_policy_group.go
model/enterprise_quota_policy.go
model/enterprise_quota_counter.go
model/enterprise_usage_attribution.go
model/enterprise_audit_log.go
model/enterprise_test.go
```

也可以先合并为 `model/enterprise_governance.go`，但长期拆文件更好维护。

### service

```text
service/enterprise_policy.go
service/enterprise_policy_context.go
service/enterprise_policy_condition.go
service/enterprise_policy_match.go
service/enterprise_policy_counter.go
service/enterprise_policy_test.go
service/enterprise_policy_condition_test.go
service/enterprise_policy_counter_test.go
```

服务层负责策略判断，不应该依赖 controller。

### controller

```text
controller/enterprise.go
controller/enterprise_org_unit.go
controller/enterprise_policy_group.go
controller/enterprise_quota_policy.go
controller/enterprise_usage.go
controller/enterprise_audit.go
```

第一版可以先合并为 `controller/enterprise.go`，但 API 增多后建议拆开。

### frontend

```text
web/default/src/features/enterprise/
web/default/src/features/enterprise/api.ts
web/default/src/features/enterprise/types.ts
web/default/src/features/enterprise/routes/...
web/default/src/features/enterprise/components/...
```

企业治理 UI 后置，不进入 PR 1。

## 常量和枚举

建议先在 model 或 service 内定义稳定枚举，避免散落字符串。

```go
const (
    EnterpriseStatusEnabled  = 1
    EnterpriseStatusDisabled = 2

    OrgUnitStatusEnabled  = 1
    OrgUnitStatusDisabled = 2

    PolicyStatusEnabled  = 1
    PolicyStatusDisabled = 2

    PolicyTargetEnterprise  = "enterprise"
    PolicyTargetOrgUnit     = "org_unit"
    PolicyTargetPolicyGroup = "policy_group"
    PolicyTargetUser        = "user"

    PolicyPeriodDaily   = "daily"
    PolicyPeriodMonthly = "monthly"

    PolicyMetricRequestCount = "request_count"
    PolicyMetricQuota        = "quota"

    PolicyActionDeny = "deny"

    ModelScopeAll      = "all"
    ModelScopeSelected = "selected"

    PolicyConditionModeStructured = "structured"
    PolicyConditionModeCEL        = "cel"
)
```

注意：

- 不使用 `department` 作为内部主概念，使用更通用的 `org_unit`。
- 不使用裸 `group` 作为策略分组字段，避免和 SQL 保留字、现有 `User.Group` 混淆。
- period 第一版只做 daily/monthly，后续再加 weekly/custom。

## 数据模型细化

### Enterprise

```go
type Enterprise struct {
    Id        int    `json:"id"`
    Name      string `json:"name" gorm:"type:varchar(128);not null"`
    Slug      string `json:"slug" gorm:"type:varchar(64);uniqueIndex;not null"`
    Status    int    `json:"status" gorm:"type:int;default:1;index"`
    CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

建议默认企业：

```text
name = "Default Enterprise"
slug = "default"
status = enabled
```

### OrgUnit

```go
type OrgUnit struct {
    Id           int    `json:"id"`
    EnterpriseId int    `json:"enterprise_id" gorm:"index;not null"`
    ParentId     int    `json:"parent_id" gorm:"index;default:0"`
    Name         string `json:"name" gorm:"type:varchar(128);not null"`
    Path         string `json:"path" gorm:"type:varchar(512);index"`
    Depth        int    `json:"depth" gorm:"type:int;default:0"`
    SortOrder    int    `json:"sort_order" gorm:"type:int;default:0"`
    Status       int    `json:"status" gorm:"type:int;default:1;index"`
    CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

Path 建议格式：

```text
/1/5/8/
```

优点：

- 查询祖先或子树容易。
- 移动节点后可批量替换子节点 path。
- 不要求递归 CTE，兼容 SQLite/MySQL/PostgreSQL。

### OrgMembership

```go
type OrgMembership struct {
    Id           int    `json:"id"`
    EnterpriseId int    `json:"enterprise_id" gorm:"index;not null"`
    UserId       int    `json:"user_id" gorm:"index;not null"`
    OrgUnitId    int    `json:"org_unit_id" gorm:"index;not null"`
    Role         string `json:"role" gorm:"type:varchar(32);default:'member'"`
    IsPrimary    bool   `json:"is_primary" gorm:"default:true;index"`
    CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

MVP 只允许一个 primary membership。可以通过业务校验保证，避免第一版折腾跨数据库
partial unique index。

### PolicyGroup 和 PolicyGroupMember

```go
type PolicyGroup struct {
    Id           int    `json:"id"`
    EnterpriseId int    `json:"enterprise_id" gorm:"index;not null"`
    Name         string `json:"name" gorm:"type:varchar(128);not null"`
    Slug         string `json:"slug" gorm:"type:varchar(64);index;not null"`
    Description  string `json:"description" gorm:"type:varchar(255)"`
    Status       int    `json:"status" gorm:"type:int;default:1;index"`
    CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type PolicyGroupMember struct {
    Id           int   `json:"id"`
    EnterpriseId int   `json:"enterprise_id" gorm:"index;not null"`
    GroupId      int   `json:"group_id" gorm:"index;not null"`
    UserId       int   `json:"user_id" gorm:"index;not null"`
    CreatedAt    int64 `json:"created_at" gorm:"autoCreateTime"`
}
```

业务层需要保证同一企业内 `slug` 唯一，同一 group/user 不重复。

### QuotaPolicy

```go
type QuotaPolicy struct {
    Id             int    `json:"id"`
    EnterpriseId   int    `json:"enterprise_id" gorm:"index;not null"`
    TargetType     string `json:"target_type" gorm:"type:varchar(32);index;not null"`
    TargetId       int    `json:"target_id" gorm:"index;not null"`
    Name           string `json:"name" gorm:"type:varchar(128);not null"`
    Status         int    `json:"status" gorm:"type:int;default:1;index"`
    Priority       int    `json:"priority" gorm:"type:int;default:0;index"`
    Period         string `json:"period" gorm:"type:varchar(16);not null"`
    Metric         string `json:"metric" gorm:"type:varchar(32);not null"`
    LimitValue     int64  `json:"limit_value" gorm:"not null"`
    Timezone       string `json:"timezone" gorm:"type:varchar(64);default:'Asia/Shanghai'"`
    ModelScope     string `json:"model_scope" gorm:"type:varchar(16);default:'all'"`
    ModelScopeJson string `json:"model_scope_json" gorm:"type:text"`
    ConditionMode  string `json:"condition_mode" gorm:"type:varchar(16);default:'structured'"`
    ConditionJson  string `json:"condition_json" gorm:"type:text"`
    ConditionExpr  string `json:"condition_expr" gorm:"type:text"`
    ConditionHash  string `json:"condition_hash" gorm:"type:varchar(64);index"`
    Action         string `json:"action" gorm:"type:varchar(32);default:'deny'"`
    StartsAt       int64  `json:"starts_at" gorm:"default:0;index"`
    EndsAt         int64  `json:"ends_at" gorm:"default:0;index"`
    CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

ModelScopeJson 示例：

```json
{"models":["gpt-4.1","claude-sonnet-4"]}
```

ConditionJson 示例：

```json
{
  "abilities": ["chat"],
  "runtime_groups": ["default", "vip"],
  "model_prefixes": ["gpt-4"],
  "is_playground": false
}
```

ConditionExpr 示例：

```cel
request.ability in ["chat"] &&
user.runtime_group in ["default", "vip"] &&
["gpt-4"].exists(prefix, request.model.startsWith(prefix)) &&
request.is_playground == false
```

MVP 中 `model_scope`、`model_scope_json` 继续保留为一级字段，CEL 条件层只做
附加条件过滤。后续如果需要更强表达能力，再把模型范围整体迁入 `ConditionJson`。

### QuotaCounter

```go
type QuotaCounter struct {
    Id            int    `json:"id"`
    EnterpriseId  int    `json:"enterprise_id" gorm:"index;not null"`
    PolicyId      int    `json:"policy_id" gorm:"index;not null"`
    TargetType    string `json:"target_type" gorm:"type:varchar(32);index;not null"`
    TargetId      int    `json:"target_id" gorm:"index;not null"`
    Metric        string `json:"metric" gorm:"type:varchar(32);index;not null"`
    PeriodStart   int64  `json:"period_start" gorm:"index;not null"`
    PeriodEnd     int64  `json:"period_end" gorm:"index;not null"`
    UsedValue     int64  `json:"used_value" gorm:"default:0"`
    ReservedValue int64  `json:"reserved_value" gorm:"default:0"`
    CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
    UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

建议业务唯一键：

```text
enterprise_id + policy_id + metric + period_start
```

GORM 跨数据库唯一索引可以后续加，MVP 先用事务内 FirstOrCreate 和重试处理。

### UsageAttribution

```go
type UsageAttribution struct {
    Id                 int    `json:"id"`
    RequestId          string `json:"request_id" gorm:"type:varchar(64);index"`
    UserId             int    `json:"user_id" gorm:"index"`
    EnterpriseId       int    `json:"enterprise_id" gorm:"index"`
    OrgUnitId          int    `json:"org_unit_id" gorm:"index"`
    PolicyGroupIdsJson string `json:"policy_group_ids_json" gorm:"type:text"`
    PolicyIdsJson      string `json:"policy_ids_json" gorm:"type:text"`
    ModelName          string `json:"model_name" gorm:"type:varchar(128);index"`
    ChannelId          int    `json:"channel_id" gorm:"index"`
    PromptTokens       int    `json:"prompt_tokens"`
    CompletionTokens   int    `json:"completion_tokens"`
    TotalTokens        int    `json:"total_tokens"`
    Quota              int    `json:"quota"`
    Status             string `json:"status" gorm:"type:varchar(32);index"`
    CreatedAt          int64  `json:"created_at" gorm:"autoCreateTime;index"`
}
```

### EnterpriseAuditLog

```go
type EnterpriseAuditLog struct {
    Id           int    `json:"id"`
    EnterpriseId int    `json:"enterprise_id" gorm:"index"`
    ActorUserId  int    `json:"actor_user_id" gorm:"index"`
    Action       string `json:"action" gorm:"type:varchar(64);index"`
    TargetType   string `json:"target_type" gorm:"type:varchar(64);index"`
    TargetId     int    `json:"target_id" gorm:"index"`
    BeforeJson   string `json:"before_json" gorm:"type:text"`
    AfterJson    string `json:"after_json" gorm:"type:text"`
    RequestId    string `json:"request_id" gorm:"type:varchar(64);index"`
    CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
}
```

## 迁移和索引建议

新增表要以“可升级老库、可重复启动、可跨数据库”为第一目标。第一版尽量使用
GORM 能稳定生成的字段类型，复杂结构统一落 `text` 保存 JSON 字符串。

建议索引：

| 表 | 索引 | 用途 |
| --- | --- | --- |
| `enterprises` | unique `slug` | 默认企业和后续多企业定位 |
| `enterprise_org_units` | `enterprise_id,status` | 组织树列表 |
| `enterprise_org_units` | `enterprise_id,parent_id` | 查询子部门 |
| `enterprise_org_units` | unique `enterprise_id,slug` | 同企业内部门标识唯一 |
| `enterprise_org_memberships` | unique `enterprise_id,user_id` | MVP 一个用户一个主企业归属 |
| `enterprise_org_memberships` | `enterprise_id,org_unit_id` | 部门成员列表 |
| `enterprise_policy_groups` | unique `enterprise_id,slug` | 同企业内策略分组唯一 |
| `enterprise_policy_group_members` | unique `policy_group_id,user_id` | 分组成员去重 |
| `enterprise_quota_policies` | `enterprise_id,status` | 策略列表和匹配 |
| `enterprise_quota_policies` | `target_type,target_id` | 按目标查策略 |
| `enterprise_quota_counters` | unique `policy_id,target_type,target_id,period_start` | 并发预占的唯一计数器 |
| `enterprise_usage_attributions` | `request_id` | 按请求排障 |
| `enterprise_usage_attributions` | `enterprise_id,created_at` | 报表查询 |
| `enterprise_usage_attributions` | `user_id,created_at` | 用户用量排行 |
| `enterprise_audit_logs` | `enterprise_id,created_at` | 审计列表 |
| `enterprise_audit_logs` | `target_type,target_id` | 对象变更历史 |

跨数据库注意点：

- 不使用数据库保留字作为列名，例如不要新增裸 `group`。
- 不依赖 partial index；停用和软删通过普通字段过滤。
- JSON 字段先用 `text`，由 service 层统一 marshal/unmarshal。
- `path` 建议保存为 `/1/3/9/` 形式，方便前缀查询和环检测。
- SQLite/MySQL/Postgres 对时间精度不同，周期边界测试不要断言纳秒。
- 大表字段避免在 MVP 引入非空且无默认值的迁移。

## 系统选项实现细节

第一批新增：

```go
var EnterpriseGovernanceEnabled = false
var EnterpriseGovernanceDryRunEnabled = false
```

需要同步位置：

- `common/constants.go`
- `model/option.go` 的默认 OptionMap。
- `model/option.go` 的 bool update switch。
- `controller/misc.go` 的 status 字段，建议 key：
  - `enterprise_governance_enabled`
  - `enterprise_governance_dry_run_enabled`
- `web/default` 设置页后续再接入。

建议第一批不加启用校验，因为数据底座和策略可以为空。启用后无策略时应允许请求。

## 初始化默认企业

建议函数：

```go
func EnsureDefaultEnterprise() error
```

调用位置：

- `migrateDB()` 的 `AutoMigrate` 完成之后。
- 必须在创建 root account 之前或之后都可幂等执行。

行为：

- 查找 `slug = "default"` 的企业。
- 不存在则创建。
- 可创建根部门 `name = "Default"`，但不要强制所有旧用户绑定部门。
- 所有函数必须幂等。

测试：

- 空库会创建默认企业。
- 已存在默认企业不会重复创建。
- 默认企业 disabled 时是否重新启用，需要产品口径。建议不自动启用，由管理员处理。

## 策略服务接口细化

建议类型：

```go
type EnterpriseContext struct {
    Enabled          bool
    DryRun           bool
    UserId           int
    TokenId          int
    EnterpriseId     int
    PrimaryOrgUnitId int
    OrgUnitIds       []int
    PolicyGroupIds   []int
    RuntimeGroup     string
}

type UsageAmount struct {
    RequestCount     int64
    Quota            int64
    PromptTokens     int64
    CompletionTokens int64
    TotalTokens      int64
}

type PolicyEvaluationRequest struct {
    EnterpriseContext *EnterpriseContext
    ModelName         string
    Ability           string
    IsPlayground      bool
    ChannelId         int
    Estimated         UsageAmount
    RequestId         string
    Now               time.Time
}

type PolicyDecision struct {
    Allowed          bool
    DryRun           bool
    DenyReason       string
    MatchedPolicyIds []int
    CounterPolicyIds []int
}

type Reservation struct {
    RequestId       string
    EnterpriseId    int
    UserId          int
    PolicyIds       []int
    ReservedAmounts map[int]UsageAmount
}
```

`PolicyEvaluationRequest.Now` 为空时使用 `time.Now()`；测试中固定该字段，避免周期边界
和时区测试不稳定。`Ability` 第一版可用 `chat`、`image`、`audio` 等内部枚举，
便于后续把非 chat relay 一起纳入企业治理。

CEL 条件输入建议固定为独立结构，不直接把完整 user、token、relayInfo 传入表达式：

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
    Role         string
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

第一版可以把模型权限和额度策略都放在 `QuotaPolicy`，但服务内部要分清：

- `model scope = all`：不形成模型白名单约束。
- `model scope = specific`：形成模型白名单约束，所有命中策略的白名单需要取交集。
- 请求模型不在交集内：直接拒绝，不进入额度预占。
- 未知 `model scope`：按策略配置错误返回内部错误，避免脏数据静默放行。
- metric 策略：参与计数器。

### CEL 条件层边界

PR 3 中 CEL 只做“候选策略是否适用于本次请求”的附加条件过滤，不负责：

- 查询候选策略。
- 计算策略优先级。
- 预占、结算、回滚 counter。
- 读取数据库、网络、文件或任何外部状态。
- 处理 prompt、completion、API key、用户密钥等敏感内容。

建议封装在 `service/enterprise_policy_condition.go`：

```go
func NormalizePolicyCondition(policy *model.EnterpriseQuotaPolicy) error
func BuildCELExpressionFromCondition(condition PolicyCondition) (string, error)
func ValidatePolicyConditionExpression(expr string) error
func EvaluatePolicyCondition(policy model.EnterpriseQuotaPolicy, input EnterprisePolicyCELInput) (bool, error)
```

实现规则：

- `condition_mode=structured` 是 MVP 默认路径，UI 保存 `condition_json`，后端生成
  稳定 `condition_expr`。
- `condition_mode=cel` 先只保留 API 能力或 root-only 后置，不在普通管理 UI 开放。
- 保存策略时必须校验 schema、生成表达式、编译并类型检查为 bool。
- CEL env 固定声明允许字段，不允许动态把任意 map 暴露进去。
- 编译结果缓存 key 使用 `policy_id + condition_hash + updated_at`。
- 空条件等价于 `true`，兼容旧策略。
- 运行期 eval error 在 dry-run 中记录 would reject；硬限制模式下 fail closed，
  返回企业治理配置错误，避免静默绕过上限。

策略匹配仍由 Go 完成：

1. SQL 加载企业、部门祖先、策略分组、用户的启用策略。
2. Go 过滤时间窗口、目标范围、`model_scope`。
3. CEL 对候选策略做附加条件过滤。
4. 进入模型权限判断和 counter 预占。

## Relay 接入点

当前 `controller/relay.go` 是统一编排入口，`relay/` 包负责具体上游适配。
企业治理第一版不应进入各个 channel adaptor，而应在 `controller/relay.go`
里统一执行策略预占，并在现有计费会话附近做结算/回滚。当前顺序大致是：

1. 校验请求。
2. 生成 `relayInfo`。
3. 敏感词检查。
4. 估算 token。
5. 计算价格和 `QuotaToPreConsume`。
6. 现有 `service.PreConsumeBilling`。
7. 调用上游。
8. 失败退款或成功由 helper/计费路径结算。

建议企业治理接入：

```text
价格计算完成后
  -> EnterprisePolicyService.ResolveContext
  -> EnterprisePolicyService.Evaluate
  -> EnterprisePolicyService.Reserve
现有 PreConsumeBilling
  -> 上游调用
成功/失败
  -> EnterprisePolicyService.Settle 或 Refund
```

原因：

- 价格计算后有稳定 quota 预估。
- 现有用户钱包/订阅预扣失败时，企业治理预占需要回滚。
- 企业治理拒绝应早于现有钱包预扣，避免无意义扣费。

第一版可以设计一个小包装：

```go
enterpriseReservation, apiErr := service.PreCheckEnterpriseGovernance(c, relayInfo, priceData.QuotaToPreConsume)
if apiErr != nil {
    newAPIError = apiErr
    return
}
defer service.FinalizeEnterpriseGovernance(c, enterpriseReservation, relayInfo, &newAPIError)
```

注意：

- `PreCheckEnterpriseGovernance` 在开关关闭时返回 nil reservation、nil error。
- 如果后续 `PreConsumeBilling` 失败，defer 应 refund enterprise reservation。
- 如果上游失败并触发现有退款，enterprise reservation 也应 refund。
- 如果成功，settle 使用实际消耗；如果暂时拿不到实际值，第一版用预估值结算并在后续补精确。

建议测试命令覆盖：

```bash
go test ./controller ./service ./relay/...
```

## API 权限策略

第一版建议：

- 企业治理配置 API 使用 `AdminAuth`。
- 系统选项开关继续使用 `RootAuth` 的 `/api/option`。
- 审计日志查询使用 `AdminAuth`，但敏感 before/after 字段未来可限制 root。

原因：

- 现有用户管理、分组管理多用 AdminAuth。
- 企业治理是运营管理能力，不一定只允许 root。
- 开关属于系统级风险控制，继续 root 更稳。

## Controller 请求/响应约定

列表接口统一：

```json
{
  "success": true,
  "data": {
    "items": [],
    "total": 0
  }
}
```

写接口统一：

```json
{
  "success": true,
  "data": { "id": 1 }
}
```

错误：

- 参数错误用现有 invalid params 风格。
- 业务冲突给明确 message，如“部门下仍有成员，不能停用”。
- 策略命中拒绝走 relay 错误码，不走普通管理 API message。

## 管理 API Schema 草案

这里的 schema 是给 controller 和前端对齐用，真实字段名可以随代码风格微调，
但语义不要漂移。

### 当前企业

```http
GET /api/enterprise/current
PUT /api/enterprise/current
```

响应：

```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "Default Enterprise",
    "slug": "default",
    "status": 1,
    "timezone": "Asia/Shanghai"
  }
}
```

更新请求：

```json
{
  "name": "RevoCloud",
  "timezone": "Asia/Shanghai"
}
```

### 部门

```http
GET /api/enterprise/org-units?parent_id=0&keyword=&status=1
POST /api/enterprise/org-units
PUT /api/enterprise/org-units/:id
DELETE /api/enterprise/org-units/:id
```

创建请求：

```json
{
  "parent_id": 0,
  "name": "研发部",
  "slug": "engineering",
  "description": "",
  "sort": 100
}
```

更新请求：

```json
{
  "parent_id": 12,
  "name": "平台研发",
  "slug": "platform-engineering",
  "description": "",
  "status": 1,
  "sort": 90
}
```

列表响应中的单项：

```json
{
  "id": 12,
  "parent_id": 0,
  "name": "研发部",
  "slug": "engineering",
  "path": "/12/",
  "depth": 1,
  "status": 1,
  "member_count": 18,
  "children_count": 3
}
```

### 成员归属

```http
GET /api/enterprise/members?org_unit_id=12&keyword=&unassigned=false&page=1&page_size=20
PUT /api/enterprise/members/:user_id/org-unit
```

绑定请求：

```json
{
  "org_unit_id": 12
}
```

解绑请求：

```json
{
  "org_unit_id": 0
}
```

成员列表单项：

```json
{
  "user_id": 1001,
  "username": "alice",
  "display_name": "Alice",
  "email": "alice@example.com",
  "org_unit_id": 12,
  "org_unit_name": "研发部",
  "policy_group_count": 2,
  "status": 1
}
```

### 策略分组

```http
GET /api/enterprise/policy-groups?keyword=&status=1&page=1&page_size=20
POST /api/enterprise/policy-groups
PUT /api/enterprise/policy-groups/:id
DELETE /api/enterprise/policy-groups/:id
GET /api/enterprise/policy-groups/:id/members
POST /api/enterprise/policy-groups/:id/members
DELETE /api/enterprise/policy-groups/:id/members/:user_id
```

创建/更新请求：

```json
{
  "name": "高阶模型试用组",
  "slug": "advanced-model-trial",
  "description": "",
  "status": 1
}
```

批量加成员：

```json
{
  "user_ids": [1001, 1002, 1003]
}
```

### 额度策略

```http
GET /api/enterprise/quota-policies?target_type=&metric=&status=1&page=1&page_size=20
POST /api/enterprise/quota-policies
PUT /api/enterprise/quota-policies/:id
DELETE /api/enterprise/quota-policies/:id
```

创建/更新请求：

```json
{
  "name": "研发部每日额度",
  "description": "",
  "target_type": "org_unit",
  "target_id": 12,
  "metric": "quota",
  "period": "day",
  "limit_value": 500000,
  "model_scope": "specific",
  "models": ["gpt-4o", "claude-sonnet-4"],
  "action": "reject",
  "priority": 100,
  "status": 1,
  "effective_at": 0,
  "expires_at": 0
}
```

响应单项建议额外带目标名称，避免前端二次查询：

```json
{
  "id": 301,
  "name": "研发部每日额度",
  "target_type": "org_unit",
  "target_id": 12,
  "target_name": "研发部",
  "metric": "quota",
  "period": "day",
  "limit_value": 500000,
  "used_value": 123456,
  "model_scope": "specific",
  "models": ["gpt-4o", "claude-sonnet-4"],
  "status": 1
}
```

### 用量和审计

```http
GET /api/enterprise/usage/summary?start=2026-06-01&end=2026-06-18
GET /api/enterprise/usage/breakdown?dimension=org_unit&metric=quota&start=2026-06-01&end=2026-06-18
GET /api/enterprise/audit-logs?target_type=&target_id=&actor_user_id=&page=1&page_size=20
```

用量摘要响应：

```json
{
  "success": true,
  "data": {
    "request_count": 1200,
    "quota": 3456789,
    "prompt_tokens": 800000,
    "completion_tokens": 420000,
    "active_users": 86
  }
}
```

用量 breakdown 单项：

```json
{
  "dimension_id": 12,
  "dimension_name": "研发部",
  "request_count": 420,
  "quota": 1800000,
  "prompt_tokens": 300000,
  "completion_tokens": 120000
}
```

## 策略算法细化

### 组织上下文解析

输入：

- `user_id`
- `token_id`，没有 API Key 时为 0
- 当前请求模型名
- 现有运行分组 `User.Group`

输出：

```go
type EnterpriseContext struct {
    EnterpriseId     int
    UserId           int
    TokenId          int
    PrimaryOrgUnitId int
    OrgUnitPathIds   []int
    PolicyGroupIds   []int
    RuntimeGroup     string
    ModelName        string
}
```

解析顺序：

1. 查询用户企业归属，没有归属则落到默认企业。
2. 查询主部门，没有主部门则 `PrimaryOrgUnitId=0`。
3. 如果有主部门，按 `path` 解析祖先部门 ID，顺序从根到当前部门。
4. 查询启用状态的策略分组成员。
5. 返回上下文；上下文为空不是错误，策略匹配阶段会只命中企业级策略。

缓存建议：

- key: `enterprise_context:{user_id}:{token_id}`。
- TTL: 30 到 120 秒。
- 部门、成员、分组变更后删除对应用户缓存。
- MVP 可以先不加缓存，接口稳定后再补。

### 策略匹配

策略候选来源：

1. 企业策略：`target_type=enterprise,target_id=enterprise_id`。
2. 部门策略：`target_type=org_unit,target_id in OrgUnitPathIds`。
3. 分组策略：`target_type=policy_group,target_id in PolicyGroupIds`。
4. 用户策略：`target_type=user,target_id=user_id`。

过滤条件：

- `status=enabled`。
- `effective_at=0 or effective_at<=now`。
- `expires_at=0 or expires_at>now`。
- `model_scope=all` 或请求模型在 `models` 中。

排序建议：

1. `priority` 数值越大优先级越高。
2. target specificity：user > policy_group > org_unit > enterprise。
3. `id` 升序兜底，保证测试稳定。

MVP 额度策略不做覆盖关系，命中的硬限制全部生效。也就是说，只要任一策略超额，
请求就会被拒绝。

### 周期边界

日周期：

- 使用企业 `timezone`。
- `period_start` 是当地日期 00:00:00 转 UTC 后保存。
- `period_end` 是下一日 00:00:00 转 UTC 后保存。

月周期：

- `period_start` 是当地月份第一天 00:00:00 转 UTC 后保存。
- `period_end` 是下月第一天 00:00:00 转 UTC 后保存。

建议 service 暴露纯函数，便于测试：

```go
func ResolvePolicyPeriod(policy QuotaPolicy, now time.Time, timezone string) (start time.Time, end time.Time, err error)
```

### 额度预占

核心目标：一次 relay 请求可能命中多条策略，要么所有计数器都预占成功，要么全部回滚。

伪代码：

```go
func ReserveEnterpriseUsage(req PolicyEvaluationRequest) (*Reservation, *PolicyDecision, error) {
    if !common.EnterpriseGovernanceEnabled {
        return nil, AllowDecision(), nil
    }

    ctx := ResolveEnterpriseContext(req.UserId, req.TokenId, req.ModelName)
    policies := MatchPolicies(ctx, req.Now)
    decision := EvaluateModelAccess(policies, req.ModelName)
    if decision.Rejected {
        return nil, decision, nil
    }

    amounts := []UsageAmount{
        {Metric: "request_count", Value: 1},
        {Metric: "quota", Value: req.EstimatedQuota},
    }

    tx := model.DB.Begin()
    reservation := NewReservation(req.RequestId, ctx, policies)
    for _, policy := range policies {
        for _, amount := range amounts {
            if policy.Metric != amount.Metric {
                continue
            }
            counter := getOrCreateCounterForUpdate(tx, policy, ctx, req.Now)
            if counter.UsedValue+amount.Value > policy.LimitValue {
                tx.Rollback()
                return nil, RejectDecision(policy, amount), nil
            }
            counter.UsedValue += amount.Value
            tx.Save(counter)
            reservation.Items = append(reservation.Items, ReservedCounterItem{
                PolicyId: policy.Id,
                CounterId: counter.Id,
                Metric: amount.Metric,
                ReservedValue: amount.Value,
            })
        }
    }
    tx.Commit()
    return reservation, AllowDecisionWithPolicies(policies), nil
}
```

`getOrCreateCounterForUpdate` 建议：

- 先用唯一键查计数器。
- 不存在则创建。
- 再用事务锁或等价机制读取当前值。
- 如果数据库不支持 `FOR UPDATE`，至少依赖唯一键和事务重试。

### 结算和回滚

请求成功：

1. 用实际 quota 计算 `delta = actual - reserved`。
2. delta 大于 0 时追加计数；如果追加后超额，MVP 不反向失败用户请求，只记录
   `settlement_over_limit`，因为上游已经成功消耗。
3. delta 小于 0 时释放多预占的额度。
4. 写 `enterprise_usage_attributions`。

请求失败：

1. 如果上游没有成功消耗，回滚 reservation 中所有 counter。
2. 如果现有 billing session 判定要收费，则按实际收费量结算。
3. 写失败态 attribution 可选；MVP 建议只写成功请求，dry-run/拒绝写审计或观测日志。

结算失败处理：

- 不影响已经返回给用户的 relay 响应。
- 记录 error 日志，包含 request ID、reservation ID、policy IDs。
- 后续可加 `enterprise_reservation_events` 做补偿队列；MVP 先不建表。

### Dry-run

dry-run 行为：

- 模型不允许、额度超限时不拒绝真实请求，但会记录 would reject。
- 不增加 quota counter 的 `used_value`。
- 写入审计或观测日志，标记 `dry_run_would_reject=true`。
- usage attribution 仍按真实请求写入，用于评估启用后的影响。

推荐落点：

- `ReserveEnterpriseUsage` 返回 decision，但 caller 在 dry-run 下不阻断。
- service 内部不要依赖 controller 状态码。
- 日志里不记录 prompt 内容。

## 第一批测试清单

PR 1 测试：

- 默认企业创建。
- 默认企业创建幂等。
- 企业审计日志写入。
- 选项默认值关闭。
- 选项更新后内存变量同步。
- `AutoMigrate` 包含新增表。

PR 2 测试：

- 创建部门。
- 移动部门。
- 环检测。
- 停用有成员部门被拒绝。
- 用户绑定主部门。
- 创建策略分组。
- 分组成员去重。
- 创建额度策略。
- 策略目标不存在时报错。

PR 3 测试：

- 用户无部门时解析企业上下文。
- 用户部门祖先策略命中。
- 用户多个策略分组全部命中。
- 模型白名单交集。
- 日/月周期边界。
- 多策略任一超额拒绝。
- dry-run 不拒绝但记录结果。

PR 4/5 测试：

- 企业治理关闭时 relay 行为不变。
- dry-run 模式请求成功但记录本应拒绝。
- 硬限制模式超额请求不打上游。
- 现有 `PreConsumeBilling` 失败时企业预占回滚。
- 上游失败时企业预占回滚。
- 成功请求写 usage attribution。

## 第一轮实现顺序

建议按这个顺序写代码：

1. 添加常量和系统选项。
2. 添加 model 结构体，不加业务逻辑。
3. 加入 `AutoMigrate`。
4. 写默认企业初始化。
5. 写审计日志函数。
6. 写最小测试。
7. 跑 `go test ./model ./common`。

这个顺序最稳，失败时定位清楚。

## PR 1 任务卡

PR 1 只交付数据底座、默认企业、系统开关和审计基础。任何请求拦截、管理 UI、
策略匹配都不进入这一批。

### Card 1: 系统开关

目标：让企业治理能力有默认关闭的全局开关。

建议改动：

- `common/constants.go`
  - 新增 `EnterpriseGovernanceEnabled`
  - 新增 `EnterpriseGovernanceDryRunEnabled`
- `model/option.go`
  - 在 `InitOptionMap` 写入两个默认值。
  - 在 bool update switch 同步两个内存变量。
- `controller/misc.go`
  - 在 status 中暴露 `enterprise_governance_enabled`。
  - 在 status 中暴露 `enterprise_governance_dry_run_enabled`。

不做：

- 不加前端设置页。
- 不在 relay 中读取该开关。

验收：

- 默认值为 false。
- `UpdateOption` 后内存变量同步。
- status 能读到当前值。

### Card 2: 企业治理模型

目标：新增核心表结构。

建议新增：

- `model/enterprise.go`
- `model/enterprise_org_unit.go`
- `model/enterprise_policy_group.go`
- `model/enterprise_quota_policy.go`
- `model/enterprise_quota_counter.go`
- `model/enterprise_usage_attribution.go`
- `model/enterprise_audit_log.go`

建议修改：

- `model/main.go`
  - 把新模型加入 `AutoMigrate`。

不做：

- 不写 CRUD controller。
- 不写策略匹配。

验收：

- `go test ./model` 可通过。
- 新表名和字段不使用裸 `group`。
- `User.Group` 不被修改。

### Card 3: 默认企业初始化

目标：升级老库和新库后都有一个企业上下文。

建议新增函数：

```go
func EnsureDefaultEnterprise() error
func GetDefaultEnterprise() (*Enterprise, error)
```

建议修改：

- `model/main.go`
  - `AutoMigrate` 成功后调用 `EnsureDefaultEnterprise()`。

行为：

- 查找 slug 为 `default` 的企业。
- 不存在则创建。
- 已存在则不重复创建。
- 暂不强制创建部门。
- 暂不批量迁移用户 membership。

验收：

- 空库创建一次。
- 重复执行幂等。
- 失败会阻止启动并暴露错误。

### Card 4: 企业审计日志基础

目标：为后续管理 API 提供统一审计写入。

建议函数：

```go
type EnterpriseAuditInput struct {
    EnterpriseId int
    ActorUserId  int
    Action       string
    TargetType   string
    TargetId     int
    Before       any
    After        any
    RequestId    string
}

func RecordEnterpriseAuditLog(input EnterpriseAuditInput) error
```

行为：

- `Before` / `After` 使用 `common.Marshal`。
- nil 值写空字符串或 `{}`，保持一致即可。
- 记录失败返回 error，由调用方决定是否回滚。

验收：

- before/after 可写入。
- request ID 可写入。
- 使用项目 JSON helper。

### Card 5: PR 1 测试

建议测试文件：

- `model/enterprise_test.go`

测试用例：

- `TestEnsureDefaultEnterpriseCreatesDefault`
- `TestEnsureDefaultEnterpriseIsIdempotent`
- `TestRecordEnterpriseAuditLog`
- `TestEnterpriseGovernanceOptionsDefaultDisabled`

如果现有测试初始化数据库成本较高，可以先把选项测试放在 `model/option` 相关测试中。

验收命令：

```bash
go test ./model ./common
git diff --check
```

### PR 1 Definition Of Done

- 所有新增模型已迁移。
- 默认企业初始化幂等。
- 企业治理和 dry-run 开关默认关闭。
- 审计日志函数可用。
- 没有 relay 行为变化。
- 没有前端行为变化。
- `go test ./model ./common` 通过。
- `git diff --check` 通过。

## PR 2 任务卡

PR 2 只交付后端管理 API，不接 relay，不做硬限制。目标是让管理员能维护组织、
策略分组和额度策略，并为 PR 3 的策略引擎准备稳定数据。

### Card 6: 企业和部门 API

目标：提供组织树的基础 CRUD。

建议改动：

- `controller/enterprise.go`
  - `GetCurrentEnterprise`
  - `UpdateCurrentEnterprise`
- `controller/enterprise_org_unit.go`
  - `ListEnterpriseOrgUnits`
  - `CreateEnterpriseOrgUnit`
  - `UpdateEnterpriseOrgUnit`
  - `DeleteEnterpriseOrgUnit`
- `router/api-router.go`
  - 注册 `/api/enterprise/current`
  - 注册 `/api/enterprise/org-units`

关键逻辑：

- 创建部门时生成 `path` 和 `depth`。
- 移动部门时检测不能移动到自身或子孙节点下。
- 移动部门后批量更新子树 `path` 和 `depth`。
- 有子部门或成员时不物理删除，建议改为停用。
- 写接口都记录审计日志。

验收：

- 部门树能按 `parent_id` 查询。
- 移动部门后子树路径正确。
- 环检测测试通过。
- 有成员部门删除会失败并返回明确 message。

### Card 7: 成员归属 API

目标：把现有用户映射到企业部门。

建议改动：

- `controller/enterprise_member.go`
  - `ListEnterpriseMembers`
  - `UpdateEnterpriseMemberOrgUnit`
- `router/api-router.go`
  - 注册 `/api/enterprise/members`

关键逻辑：

- 第一版一个用户只有一个主部门。
- `org_unit_id=0` 表示解绑部门，但仍属于默认企业。
- 支持未分配用户筛选。
- 列表可 join 用户表返回 username/display name/email/status。

验收：

- 用户绑定部门后能在部门成员列表出现。
- 用户解绑后进入未分配列表。
- 绑定不存在或停用部门会失败。
- 成员变更写审计日志。

### Card 8: 策略分组 API

目标：支持跨部门的策略目标。

建议改动：

- `controller/enterprise_policy_group.go`
  - `ListEnterprisePolicyGroups`
  - `CreateEnterprisePolicyGroup`
  - `UpdateEnterprisePolicyGroup`
  - `DeleteEnterprisePolicyGroup`
  - `ListEnterprisePolicyGroupMembers`
  - `AddEnterprisePolicyGroupMembers`
  - `DeleteEnterprisePolicyGroupMember`
- `router/api-router.go`
  - 注册 `/api/enterprise/policy-groups`

关键逻辑：

- 同企业内 `slug` 唯一。
- 批量加成员要去重，重复成员不应报错。
- 停用分组后不再参与策略匹配。
- 删除有策略引用的分组时建议拒绝，提示先停用或迁移策略。

验收：

- 分组 CRUD 通过。
- 批量加成员幂等。
- 删除被策略引用的分组会失败。
- 成员增删写审计日志。

### Card 9: 额度策略 API

目标：支持 request count 和 quota 两类硬限制策略配置。

建议改动：

- `controller/enterprise_quota_policy.go`
  - `ListEnterpriseQuotaPolicies`
  - `CreateEnterpriseQuotaPolicy`
  - `UpdateEnterpriseQuotaPolicy`
  - `DeleteEnterpriseQuotaPolicy`
- `router/api-router.go`
  - 注册 `/api/enterprise/quota-policies`

关键逻辑：

- 校验 `target_type` 和 `target_id` 指向同一企业下的对象。
- `metric` 第一版只允许 `request_count`、`quota`。
- `period` 第一版只允许 `day`、`month`。
- `model_scope=specific` 时 models 不能为空。
- `limit_value` 必须大于 0。
- 删除策略建议软删除或停用，避免历史 counter 和 attribution 失去解释。

验收：

- 策略目标不存在时报错。
- 模型列表 JSON 可正确保存和读取。
- 停用策略不影响历史记录。
- 策略变更写审计日志。

### Card 10: 用量和审计查询 API

目标：给管理端和排障提供只读查询。

建议改动：

- `controller/enterprise_usage.go`
  - `GetEnterpriseUsageSummary`
  - `GetEnterpriseUsageBreakdown`
- `controller/enterprise_audit.go`
  - `ListEnterpriseAuditLogs`
- `router/api-router.go`
  - 注册 `/api/enterprise/usage/*`
  - 注册 `/api/enterprise/audit-logs`

关键逻辑：

- 用量查询基于 `enterprise_usage_attributions` 聚合。
- 没有 attribution 前返回 0，不报错。
- 大范围查询强制分页或限制最大时间跨度。
- 审计日志默认不展开过大的 before/after，可按详情接口后续补。

验收：

- 空数据返回 0。
- 按部门/分组/用户维度聚合正确。
- 审计日志可按 target 和 actor 筛选。

### PR 2 Definition Of Done

- 所有管理 API 都挂 `AdminAuth`。
- 所有写接口都有参数校验和审计日志。
- 组织、成员、分组、策略的基础测试通过。
- 没有 relay 行为变化。
- 前端不要求完成，但 API schema 已稳定。
- `go test ./controller ./model` 通过。
- `git diff --check` 通过。

## PR 3 任务卡

PR 3 交付策略引擎核心，但仍不强接 relay。可以先通过 service 单元测试和少量
controller dry-run endpoint 验证策略结果。

### Card 11: 组织上下文服务

目标：把用户请求转换为企业策略上下文。

建议改动：

- `service/enterprise_policy_context.go`
- `service/enterprise_policy_test.go`

关键逻辑：

- 用户没有 membership 时回落默认企业。
- 部门 path 解析为祖先链。
- 只返回启用的策略分组。
- 保留 `RuntimeGroup`，但不把它等同企业组织。

验收：

- 无部门用户上下文可解析。
- 子部门用户包含父部门 ID。
- 停用分组不进入上下文。

### Card 12: CEL 条件底座

目标：让 UI 结构化条件可以保存、生成 CEL、编译校验并执行。

建议改动：

- `model/enterprise_quota_policy.go`
  - 增加 `ConditionMode`
  - 增加 `ConditionJson`
  - 增加 `ConditionExpr`
  - 增加 `ConditionHash`
- `controller/enterprise_quota_policy.go` 或 `controller/enterprise.go`
  - 策略创建/更新 API 支持 condition 字段。
  - 普通管理员只提交结构化 `condition_json`。
- `service/enterprise_policy_condition.go`
- `service/enterprise_policy_condition_test.go`

关键逻辑：

- 定义结构化条件 schema：ability、runtime group、model prefix、playground、channel。
- 从结构化条件生成稳定 CEL 表达式。
- 保存策略前编译并类型检查，结果必须是 bool。
- 缓存编译结果，cache key 为 `policy_id + condition_hash + updated_at`。
- CEL input 不包含 prompt、completion、API key、用户密钥。
- 空条件等价于 `true`。

验收：

- 相同 `condition_json` 生成相同 `condition_expr`。
- 非 bool 表达式保存失败。
- 非白名单字段保存失败。
- eval cache 命中可测试。
- 敏感字段不会进入 CEL input。

### Card 13: 策略候选和条件匹配服务

目标：找出一次请求命中的全部有效策略，并按 CEL 条件过滤。

建议改动：

- `service/enterprise_policy_match.go`
- `service/enterprise_policy_test.go`

关键逻辑：

- 命中企业、祖先部门、当前部门、策略分组、用户策略。
- 过滤时间窗口、停用策略、模型不匹配策略。
- 对候选策略执行 `EvaluatePolicyCondition`。
- condition eval error 在 dry-run 中记录 would reject；硬限制模式 fail closed。
- 排序稳定，便于审计和测试。

验收：

- 父部门策略对子部门生效。
- 多个策略分组全部命中。
- 模型白名单过滤正确。
- CEL 条件不匹配的策略不进入 counter。
- CEL 条件运行错误可观测，不会静默放行。

### Card 14: 计数器预占服务

目标：并发安全地执行额度预占。

建议改动：

- `service/enterprise_policy_counter.go`
- `service/enterprise_policy_counter_test.go`

关键逻辑：

- 计算日/月周期边界。
- 获取或创建 counter。
- 在一个事务中预占所有命中策略。
- 任一策略超额时回滚全部预占。
- 返回 reservation，供 PR 4 relay 接入使用。

验收：

- 日/月周期边界测试通过。
- 多策略预占任一失败时全部回滚。
- 并发测试不会明显突破上限。

### Card 15: 策略决策和 dry-run 记录

目标：统一 allow/reject/dry-run 的返回结构。

建议改动：

- `service/enterprise_policy.go`
- `model/enterprise_audit_log.go` 或新增观测日志 helper

关键逻辑：

- `PolicyDecision` 包含 allow/reject、reason、policy IDs。
- dry-run 下返回 would reject，但不阻断调用方。
- 不记录 prompt。
- 拒绝 reason 面向用户要短，面向管理员要可排查。

验收：

- hard limit 返回 reject。
- dry-run 返回 allow 但携带 would reject 信息。
- 决策结果可以序列化到日志。

### PR 3 Definition Of Done

- 策略引擎单元测试覆盖上下文、CEL 条件、匹配、周期、预占、dry-run。
- 结构化条件能稳定生成 CEL。
- CEL 表达式保存前完成编译和 bool 类型检查。
- CEL eval 有缓存，缓存 key 可解释。
- CEL input 不暴露 prompt、completion、API key 或密钥。
- service 层不依赖 controller。
- 企业治理关闭时 service 快速返回 allow。
- 没有 relay 行为变化。
- `go test ./service ./model ./controller` 通过。
- `git diff --check` 通过。

## 实现前验证项

整体规划口径已经在 `enterprise-org-quota-mvp-delivery-plan.md` 收口。下面这些不是
产品方向待定项，而是进入实现时需要用代码验证的技术点：

- 实际 quota 是否始终可以在 relayInfo 或 billing event 中可靠读取。
- 流式响应结束时是否一定能拿到可用于企业结算的实际消耗。
- 上游失败和 billing preconsume 失败是否都有统一错误路径可挂 reservation refund。
- 现有 request ID 是否覆盖所有 relay 分支，是否需要在企业归集前补齐。
- 当前测试数据库是否能覆盖 counter 并发和事务回滚场景。

如果实际 quota 暂时无法在所有分支可靠读取，MVP 可以先用预估 quota 结算，并在
usage attribution 中标记 `settlement_source=estimated`，后续再补精确结算。

## 已确认决策

下面是 MVP 默认决策，和 `enterprise-org-quota-mvp-delivery-plan.md` 保持一致。

### D1: Root 用户默认不豁免，提供独立开关

建议：

- 默认 root 用户也受企业策略限制。
- 后续如有运维需要，再加 `EnterpriseGovernanceRootBypassEnabled`。

原因：

- 企业预算通常希望覆盖所有真实用量。
- root 豁免容易导致报表和实际账单对不上。
- 真正需要排障时可以关闭企业治理或使用 dry-run。

### D2: Playground 纳入治理

建议：

- Playground 请求也纳入企业治理。
- 报表中可以用现有 `IsPlayground` 或请求来源标识区分。

原因：

- 企业内部调试模型同样会产生成本。
- 不纳入治理会成为绕过额度的入口。

### D3: API Key 继承用户组织上下文

建议：

- MVP 中 API Key 完全继承所属用户的企业、部门和策略分组。
- 不给 API Key 单独配置部门和策略分组。

原因：

- 当前系统已有 token/用户关系。
- 单独给 API Key 配组织会显著增加 UI 和审计复杂度。
- 项目/应用额度放到 Phase 2 处理更自然。

### D4: 只限制模型 relay，不限制管理 API

建议：

- 企业治理第一版只管模型、图片、语音、视频等 relay 消耗。
- 不限制用户管理、渠道管理、系统设置等管理 API。

原因：

- 企业治理的核心目标是 AI 使用成本和能力边界。
- 管理 API 已有角色权限系统。
- 把治理策略扩展到管理 API 会与 RBAC 重叠。

### D5: MVP 强限制只做 `request_count` 和 `quota`

建议：

- `request_count` 请求前按 1 预占。
- `quota` 请求前用 `priceData.QuotaToPreConsume` 预占。
- token 字段只写入 usage attribution，暂不做强拦截。

原因：

- request count 和 quota 与现有计费路径最贴近。
- token 精确值跨模型和流式响应差异更大。
- 后续可以基于归集数据再启用 token 策略。

### D6: 先用数据库事务实现，Redis 优化后置

建议：

- MVP 先保证数据库事务/行锁路径正确。
- Redis 原子计数作为性能优化进入后续任务。

原因：

- 当前系统部署可能不强依赖 Redis。
- 数据库路径更容易测试和回放。
- Redis 版本需要补偿和一致性方案，适合在硬限制稳定后再加。

### D7: 策略拒绝返回 403

建议：

- 企业治理硬限制拒绝返回 HTTP 403。
- 错误码新增独立企业治理错误码。
- 用户可见消息简短，日志和审计记录策略 ID。

原因：

- 请求格式正确，但权限/额度不允许。
- 和现有 quota 不足的语义接近，客户端容易处理。
