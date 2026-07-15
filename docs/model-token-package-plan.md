# 模型 Token 包（Model Token Package）模块规划

日期：2026-07-16  
状态：设计定稿，待开发  
代码目录：`/Users/fushilu/workspace/revocloud/data-proxy/upstream/new-api`

## 1. 背景与目标

### 1.1 问题

当前额度体系以 **系统 quota 点（金额点）** 为主：

- `users.quota`：用户钱包余额（金额点，`QuotaPerUnit` 锚定 USD）
- `tokens.remain_quota`：API Key 额度帽（同样是金额点）
- 计费：`token 用量 × model_ratio × group_ratio` → 金额点

无法表达产品需求：

> 用户购买 / 获赠「某模型或某几个模型」的 **Token 用量包**（单位是 LLM token 个数，不是金额）。  
> 有包时优先扣包；没有可用包时再扣钱包金额。

### 1.2 目标

| 目标 | 说明 |
| --- | --- |
| 用量单位 | 包余额为 **token 个数**（输入 / 输出 / 缓存等可加权） |
| 模型范围 | 每包绑定一个或多个模型白名单 |
| 扣费策略 | **有覆盖该模型的可用包 → 扣包；否则 → 扣钱包** |
| 编排统一 | 中继侧统一 Funding 选择；资产账本不合并 |
| 可观测 | usage log / 流水标明 `funding_source`、原始 usage、倍率、折算消耗 |
| 可运营 | 管理员可发放 / 作废；后续可接购买与兑换 |

### 1.3 非目标（本期不做）

- 不把 token 个数写入 `users.quota` 或 `tokens.remain_quota`
- 不做「单次请求包 + 钱包混扣补差」（可作为后续开关，默认关闭）
- 不做多币种、跨节点分布式包余额
- 不替代企业治理 hard-limit / 订阅包（可后续挂到同一编排器）
- P0 不强制用户自助购买（可先管理员发放）

---

## 2. 产品规则

### 2.1 核心策略

```text
请求 model = X（经模型名规范化后匹配）

1. 查找用户 status=active、未过期、models 覆盖 X、remaining_tokens > 0 的包
2. 按 priority DESC, expired_at ASC, id ASC 选中一个包
3. 若选中包：
     - 预检：remaining 足够（P0 可按预估；结算按实际）
     - 结算：remaining -= package_consume（见 2.2）
     - 不扣 users.quota（钱包）作为主扣费
4. 若无可用包：
     - 走现有钱包 / API Key 金额点计费（BillingSession）
5. 包 remaining 不足以覆盖本次实际消耗（P0）：
     - 拒绝并返回包额度不足（不自动切钱包）
```

### 2.2 包消耗口径与倍率

折算消耗：

```text
package_consume = ceil(
    prompt_tokens     * input_ratio
  + completion_tokens * output_ratio
  + cache_tokens      * cache_ratio
)
```

| 字段 | 默认 | 含义 |
| --- | --- | --- |
| `input_ratio` | `1` | 输入 token 倍率 |
| `output_ratio` | `1` | 输出 token 倍率 |
| `cache_tokens` | 以中继最终 usage 中的缓存类用量为准 | 读缓存 / 写缓存等已计入 usage 的部分；P0 可先合并为一项 |
| `cache_ratio` | `1` | 缓存 token 倍率 |

约束：

- 倍率 `>= 0`，建议上限 `<= 10`
- `package_consume` 为非负整数；至少扣 `0`（无用量时）
- **以结算时 usage 为准**，与 usage log 一致，避免自行另算一套
- 其它已进入本次 usage 的 token 类字段：若无法归入 prompt/completion/cache，P0 可并入 `prompt_tokens` 或单独扩展；不得静默丢弃已计费用量

### 2.3 与钱包的关系

| 场景 | 行为 |
| --- | --- |
| 有覆盖模型的可用包 | **只扣包**（token 个数） |
| 无覆盖该模型的包 | **只扣钱包**（金额点，现有逻辑） |
| 包不够本次实际消耗 | **403 包不足**，不切钱包（P0） |
| 包用尽 | `status=exhausted`，后续该模型若无其它包则走钱包 |
| 购买包（P1） | 付款瞬间可用钱包/支付；购入后消耗只动包 |

### 2.4 统一管理边界

| 层 | 是否统一 | 说明 |
| --- | --- | --- |
| 中继 Funding 编排 | **统一** | 唯一入口：先包后钱包 |
| 资产存储 | **不统一** | 钱包金额点 vs 包 token 个数分表 |
| 用户 / 管理 UI 入口 | **可统一入口、分区块** | 「额度中心」同时展示钱包与包 |
| 发放 / 充值 API | **不统一** | 加资金走现有 manage；发包走新 API |

---

## 3. 架构

### 3.1 总览

```text
Relay 请求
  → TokenAuth / Distribute（模型允许性，现有）
  → FundingOrchestrator.Resolve(user, model)
        ├─ PackageFunding  → ModelTokenPackageSession
        └─ WalletFunding   → 现有 BillingSession
  → 上游
  → Settle / Refund（仅当前 funding）
  → usage log + package ledger / billing_events
```

### 3.2 模块划分

| 模块 | 路径建议 | 职责 |
| --- | --- | --- |
| Model | `model/model_token_package.go` | 表结构、CRUD、原子扣减 |
| Service 编排 | `service/model_token_package.go` | 选包、预检、结算、退款 |
| Service 接入 | `service/billing.go` 或 relay 预扣前 | ResolveFunding 接入点 |
| Controller | `controller/model_token_package.go` | 用户列表、管理发放/作废 |
| Router | `router/api-router.go` | 路由注册 |
| 前端用户 | `web/default/src/features/wallet` 或新 feature | 包列表、剩余、流水 |
| 前端管理 | `web/default/src/features/users` | 用户详情发放包 |

### 3.3 与现有组件关系

- **不修改** quota 点的含义（`QuotaPerUnit`）
- **复用** 模型名规范化（与 token `model_limits` / distributor 一致，如 `FormatMatchingModelName`）
- **复用** usage 结算结果中的 prompt/completion/cache 字段
- API Key（`tokens`）仍负责鉴权；包挂在 **用户** 上（P0），不强制 Key 绑定

---

## 4. 数据模型

### 4.1 `model_token_packages`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int PK | |
| `user_id` | int, index | 归属用户 |
| `name` | varchar | 展示名 |
| `models_json` | text | JSON 字符串数组，如 `["gpt-4o","gpt-4o-mini"]` |
| `total_tokens` | bigint | 初始总量 |
| `remaining_tokens` | bigint | 剩余 |
| `used_tokens` | bigint | 已用（折算后累计） |
| `input_ratio` | double | 默认 1 |
| `output_ratio` | double | 默认 1 |
| `cache_ratio` | double | 默认 1 |
| `priority` | int | 默认 0；越大越优先 |
| `status` | varchar/int | `active` / `exhausted` / `expired` / `disabled` |
| `expired_at` | bigint | Unix；`0` 或 `-1` 表示不过期（实现时统一一种） |
| `source` | varchar | `admin_grant` / `purchase` / `redeem` |
| `created_by` | int | 管理员发放时记录 |
| `remark` | text | 可选 |
| `created_at` / `updated_at` | bigint | |

索引建议：

- `(user_id, status, priority)`
- `(user_id, expired_at)`

### 4.2 `model_token_package_ledger`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | bigint PK | |
| `package_id` | int, index | |
| `user_id` | int, index | |
| `request_id` | varchar, index | |
| `model` | varchar | 实际模型名 |
| `prompt_tokens` | int | 原始 |
| `completion_tokens` | int | 原始 |
| `cache_tokens` | int | 原始 |
| `input_ratio` / `output_ratio` / `cache_ratio` | double | 快照 |
| `delta_tokens` | bigint | 消耗为负；发放为正 |
| `reason` | varchar | `consume` / `refund` / `grant` / `adjust` / `expire` |
| `created_at` | bigint | |

### 4.3 可选：`model_token_package_skus`（P1 购买）

商品：模型列表、token 量、三倍率、价格、有效天数。P0 可不建表，管理员手工填发放表单即可。

### 4.4 迁移

- GORM AutoMigrate 注册两张表（SQLite / MySQL / PostgreSQL 兼容）
- 禁止使用仅单库支持的类型；JSON 用 `text` 存

---

## 5. 中继扣费流程

### 5.1 Resolve

```text
func ResolveModelTokenPackageFunding(userId, model string) (*Package, bool)
  - normalize model name
  - list active packages for user
  - filter: not expired, remaining > 0, model in models_json
  - order: priority DESC, expired_at ASC (finite first), id ASC
  - return first
```

### 5.2 PreCheck（P0 简化）

- 有包：`remaining_tokens > 0` 即可放行（或 `remaining >= roughEstimate`）
- 无包：进入现有钱包 PreConsume
- 严格模式后续可加：指定模型强制要求包

### 5.3 Settle（上游成功）

```text
usage = 中继最终 usage
package_consume = ceil(p*in + c*out + cache*cache_ratio)
atomic:
  UPDATE packages
  SET remaining = remaining - consume,
      used = used + consume,
      status = CASE WHEN remaining - consume <= 0 THEN exhausted ELSE status END
  WHERE id=? AND remaining >= consume AND status=active
若 RowsAffected=0 → 记失败策略（P0：打错误日志 + 返回结算错误或标记请求异常，需在实现时与现有 settle 错误处理对齐）
写 ledger（delta = -consume）
usage log: funding_source=model_token_package, package_id, package_consume, ratios, raw tokens
```

### 5.4 Refund（上游失败 / 预占）

- P0 若无预占，失败路径可不写包流水
- P1 预占：`reserved_tokens` 字段或 ledger `reserve` / `release` / `commit`

### 5.5 错误码

| code | HTTP | 场景 |
| --- | --- | --- |
| `insufficient_model_token_package` | 403 | 有包但不足，或 settle 超扣失败 |
| `model_token_package_disabled` | 403 | 包被作废（可选） |
| 现有 `insufficient_user_quota` | 403 | 无包且钱包不足 |
| 现有 `pre_consume_token_quota_failed` | 403 | Key 金额帽不足（钱包路径） |

错误信息需包含：模型名、包 id（可选）、remaining。

---

## 6. API 设计

### 6.1 用户侧（UserAuth）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/user/model-token-packages` | 当前用户包列表 |
| GET | `/api/user/model-token-packages/:id/ledger` | 单包流水（分页） |

### 6.2 管理侧（AdminAuth）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/user/:id/model-token-packages` | 指定用户包列表 |
| POST | `/api/user/:id/model-token-packages` | 发放包 |
| PUT | `/api/user/:id/model-token-packages/:pkg_id` | 调整备注/优先级/倍率/过期（谨慎改 remaining） |
| POST | `/api/user/:id/model-token-packages/:pkg_id/adjust` | 管理员增减 token 量（写 ledger） |
| POST | `/api/user/:id/model-token-packages/:pkg_id/disable` | 作废 |

### 6.3 发放请求体示例

```json
{
  "name": "GPT-4o 100万 Tokens",
  "models": ["gpt-4o", "gpt-4o-mini"],
  "total_tokens": 1000000,
  "input_ratio": 1,
  "output_ratio": 1,
  "cache_ratio": 1,
  "priority": 0,
  "expired_at": -1,
  "remark": "活动赠送"
}
```

校验：

- `models` 非空、去重、trim
- `total_tokens > 0`
- 倍率范围合法
- 目标用户存在且启用

审计：写管理日志（`LogTypeManage` 或 enterprise 无关的平台日志），包含操作者、用户、包内容摘要。

---

## 7. 前端

### 7.1 用户

- 入口：钱包页或独立「模型 Token 包」Tab
- 列表：名称、模型标签、剩余/总量、三倍率、状态、过期
- 详情：流水（request_id、模型、原始 usage、折算消耗）

### 7.2 管理

- 用户管理行操作 / 用户详情：「发放 Token 包」
- 表单：名称、模型多选、总量、三倍率（默认 1）、优先级、过期、备注
- 已有包列表：剩余、作废、调整

### 7.3 i18n

- 所有文案走 `t()`；同步 en/zh 及现有 locale 流程

---

## 8. 可观测与对账

每次走包的成功请求：

| 字段 | 说明 |
| --- | --- |
| `funding_source` | `model_token_package` |
| `package_id` | |
| `prompt_tokens` / `completion_tokens` / `cache_tokens` | 原始 |
| `input_ratio` / `output_ratio` / `cache_ratio` | 快照 |
| `package_consume` | 折算后扣减 |
| `request_id` | 与 usage log 关联 |

钱包路径：`funding_source=wallet`（或保持现有语义并补充字段）。

对账：

- `sum(ledger.delta where reason=consume)` ≈ `-used_tokens`
- `remaining + used` ≈ `total + sum(adjust/grant)`（允许作废后不再消费）

---

## 9. 测试计划

### 9.1 单测 / 服务测

- 选包：优先级、过期、模型匹配、无包
- 折算：默认倍率 1；缓存 0.5；输出 2；向上取整
- 原子扣减：并发不超扣
- 用尽后 status=exhausted
- Resolve：有包不走钱包；无包走钱包

### 9.2 中继集成

- mock usage 结算后 remaining 正确减少
- 失败请求不扣包（P0 无预占）
- 错误码与 HTTP 状态

### 9.3 API / 前端

- 管理员发放权限
- 用户只能看自己的包
- typecheck / 相关 go test

---

## 10. 开发顺序

### Phase 0：基线与约定（0.5d）

- [x] 确认模型名匹配函数与 cache_tokens 在 usage 中的字段来源
- [x] 确认 `expired_at` 哨兵值（`0` = 永不过期）
- [x] 在本文档冻结 P0 行为（包不足不切钱包）

### Phase 1：数据层（1d）

- [x] 新增 `model/model_token_package.go`、ledger model
- [x] AutoMigrate 注册
- [x] 原子扣减 / 发放 / 作废方法
- [x] model 层单测

### Phase 2：Service 选包与结算（1–2d）

- [x] `service/model_token_package.go`：Resolve、ComputeConsume、Settle、Refund 骨架
- [x] 倍率计算与 ceil
- [x] 服务层单测（折算 / usage 解析）

### Phase 3：中继接入（1–2d）

- [x] 在 PreConsumeBilling 接入 Funding 分支
- [x] 有包：跳过钱包主扣费；PostTextConsumeQuota 写 package ledger
- [x] 无包：完全保持现有 BillingSession
- [x] 错误码 `insufficient_model_token_package`
- [x] usage log other 写入 funding_source / package 字段

### Phase 4：管理 API + 审计（1d）

- [x] 发放 / 列表 / 作废 / 调整 API
- [x] 管理操作日志
- [ ] controller 集成测试（可选后续）

### Phase 5：用户 API + 前端（1–2d）

- [x] 用户包列表与流水 API
- [x] 管理端发放对话框（用户行菜单）
- [x] 用户端展示：钱包页 `ModelTokenPackagesCard` + 流水弹窗
- [x] i18n 基础文案
- [x] `tsc` / eslint 触碰文件

### Phase 6：打磨与文档收口（0.5–1d）

- [x] 规划文档阶段勾选更新
- [ ] 操作说明补进 operator guide（可选）
- [ ] 对账查询示例（可选）
- [ ] 回归：纯钱包用户行为不变

**P0 合计约 5–8 人日（视中继接入复杂度）。**

### Phase 7+（P1，不阻塞 P0）

| 序号 | 内容 |
| --- | --- |
| P1-1 | 预占 reserved，防并发超卖 |
| P1-2 | SKU + 支付购买生成包 |
| P1-3 | 兑换码兑换包 |
| P1-4 | 即将用尽通知 |
| P1-5 | 包不足时可选「钱包补差」开关 |
| P1-6 | API Key 绑定指定包 |
| P1-7 | 缓存读/写拆分倍率 |
| P1-8 | 企业项目维度包 |

---

## 11. 风险与对策

| 风险 | 对策 |
| --- | --- |
| 单位与钱包混淆 | 分表、分错误码、UI 明确「Token 个数」 |
| 模型别名匹配失败 | 与 distributor 同一 normalize；测试别名 |
| 并发超扣 | 条件 UPDATE；P1 预占 |
| 缓存字段上游不一致 | 以本系统 settle usage 为准；文档写清映射 |
| 接入破坏纯钱包用户 | 无包时零行为变化；回归测试 |
| 管理员误发放 | 审计日志；作废能力；调整 ledger |

---

## 12. 验收标准（P0）

1. 管理员可给用户发放指定模型列表 + token 总量 + 三倍率（默认 1）的包。  
2. 用户请求包内模型时，优先扣包；`remaining_tokens` 与 ledger、usage 一致。  
3. 消耗 = ceil(输入×in + 输出×out + 缓存×cache)。  
4. 无可用包时，行为与改造前钱包计费一致。  
5. 包用尽后同模型无其它包时回退钱包。  
6. 包不足返回 `insufficient_model_token_package`，不静默扣钱包。  
7. 用户可查看自己的包与流水；管理员可作废。  
8. SQLite / MySQL / PostgreSQL 迁移与基础测试通过。  

---

## 13. 决策记录

| 决策 | 结论 |
| --- | --- |
| 单位 | LLM token 个数，非金额点 |
| 扣费顺序 | 有包扣包，无包扣钱包 |
| 包不足 | P0 不自动切钱包 |
| 倍率 | input / output / cache，默认均为 1 |
| 管理 | 编排统一、账本分离、UI 可同入口 |
| Key 关系 | P0 包挂用户，不强制绑 Key |

---

## 14. 参考代码（现状）

- 用户钱包：`model/user.go`，`controller/user.go` `ManageUser`
- API Key 额度（金额点）：`model/token.go`
- 预扣与结算：`service/billing.go`，`service/billing_session.go`，`service/text_quota.go`
- 计价：`relay/helper/price.go`，`common/constants.go` `QuotaPerUnit`
- 模型限制（仅白名单）：`middleware/distributor.go` token model limits

---

## 15. 实现进度（2026-07-16）

P0 主体已落地：

| 能力 | 状态 |
| --- | --- |
| 表 + 原子扣减 + 发放/作废 | 已完成 |
| 有包扣包 / 无包扣钱包 | 已完成 |
| 输入/输出/缓存倍率（默认 1） | 已完成 |
| 管理 API + 用户 API | 已完成 |
| 管理端发放 UI | 已完成 |
| 用户钱包页包列表 + 流水 | 已完成 |
| 购买 / 兑换 / 预占 | 未做（P1） |

关键代码：

- `model/model_token_package.go`
- `service/model_token_package.go`
- `controller/model_token_package.go`
- `service/billing.go`（PreConsume 分支）
- `service/text_quota.go`（Settle 分支）
- `web/default/src/features/users/components/model-token-package-dialog.tsx`
- `web/default/src/features/wallet/components/model-token-packages-card.tsx`

## 16. 下一步

1. 部署后跑迁移，管理员给用户发放包并验证 relay 扣减。  
2. 可选：用户钱包页文案打磨、admin 用户详情包列表。  
3. P1：购买 SKU、预占、兑换码。
