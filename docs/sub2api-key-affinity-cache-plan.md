# sub2api 多 Key 粘性分配与缓存命中优化方案

这份文档是“小白可读版”。它先讲这个功能到底解决什么问题、管理员应该怎么用，再讲开发怎么做。

## 0. 先给结论

我们要在“一个渠道配置多个上游 key”的场景里新增一个策略：

```text
负载保护粘性分配
```

它的目标是：

```text
同一个用户 / 会话 / 项目，尽量稳定使用同一个上游 key。
某个 key 明显比其他 key 忙时，新会话优先分配到别的 key。
已有会话不轻易切换，避免影响缓存命中。
```

这个策略特别适合：

- sub2api 多 key 渠道。
- Codex、Claude Code、长上下文工具。
- 你希望尽量提高上游缓存命中的场景。

不建议把它做成很多高级开关。第一版只给管理员一个容易理解的选择项：

```text
随机
轮询
负载保护粘性分配
```

## 1. 要解决的问题

现在 new-api / data-proxy 的多 key 模式主要是随机或轮询。

比如一个渠道里有 3 个 sub2api key：

```text
key 1 = sub2api-key-a
key 2 = sub2api-key-b
key 3 = sub2api-key-c
```

用户 A 连续问同一个项目，如果每次都随机：

```text
第 1 次 -> key 1
第 2 次 -> key 3
第 3 次 -> key 2
第 4 次 -> key 1
```

如果 sub2api 内部会按 key 分配上游账号，那么这些请求可能被打散到不同账号上，上游缓存就不容易命中。

我们希望变成：

```text
用户 A / 项目 X -> 尽量一直使用 key 2
用户 B / 项目 Y -> 尽量一直使用 key 3
用户 C / 项目 Z -> 尽量一直使用 key 1
```

这样更容易让同一个项目、同一个会话的请求落到同一个上游 key 后面。

## 2. 一句话理解“粘性”

可以把它理解成餐厅排队：

```text
老顾客如果已经在 2 号窗口排队，尽量继续走 2 号窗口。
新顾客来的时候，如果 2 号窗口特别挤，就安排到 1 号或 3 号窗口。
如果 2 号窗口坏了，再让老顾客换窗口。
```

所以它不是“永远固定”，而是：

```text
优先稳定
新会话避开过载
老会话少切换
坏了或严重过载才迁移
```

## 3. data-proxy 能做到什么，不能做到什么

### 能做到

data-proxy 可以控制：

- 当前请求选择哪个渠道。
- 当前渠道里的多个 key，选择哪一个 key。
- 选择结果写进日志，方便按 request id 排查。
- 如果某个 key 被禁用，选择其他可用 key。
- 如果某个 key 明显过载，新会话尽量绕开。

### 做不到

data-proxy 不能控制：

- sub2api 内部到底用哪个真实账号。
- sub2api 是否真的按 key 绑定账号。
- 上游模型服务自己的缓存策略。

所以这个功能的正确预期是：

```text
data-proxy 尽量稳定选择同一个 sub2api key。
如果 sub2api 会根据 key 分配账号，缓存命中会更好。
如果 sub2api 内部仍然随机，data-proxy 侧也无法强制提高缓存命中。
```

## 4. 管理员应该怎么使用

在渠道创建或编辑页：

1. 添加模式选择“多秘钥模式（多个秘钥，一个通道）”。
2. 每行填写一个 sub2api key。
3. 多秘钥策略选择“负载保护粘性分配”。
4. 保存渠道。

第一版不要求管理员理解哈希、TTL、RPM 这些概念。界面说明建议写成：

```text
适合 sub2api 多 key、Codex、Claude Code 和长上下文场景。
系统会尽量让同一个用户或会话稳定使用同一个上游 key。
当某个 key 明显比其他 key 更忙时，新会话会自动分配到其他 key，已有会话不会频繁切换。
```

## 5. 第一版到底做哪些

第一版只做最重要的闭环：

| 功能 | 第一版是否做 | 说明 |
| --- | --- | --- |
| 新增策略“负载保护粘性分配” | 做 | 管理员可在渠道里选择 |
| 同一个会话稳定选同一个 key | 做 | 通过稳定 seed + 绑定实现 |
| 新会话避开明显过载 key | 做 | 用 RPM + Inflight 判断 |
| 老会话软过载不频繁迁移 | 做 | 保护缓存命中 |
| 禁用 key 后自动选其他 key | 做 | 只从启用 key 中选择 |
| 日志记录选择原因 | 做 | request trace 可排查 |
| TPM 负载统计 | 暂不做 | 后续增强 |
| 自定义 seed 规则 | 暂不做 | 第一版自动提取 |
| 多 key 实时负载大面板 | 暂不做 | 先写日志，后续再做 UI |
| 多节点全局负载均衡 | 暂不做 | 当前项目先按单机部署设计 |

## 6. 系统如何判断“同一个用户或会话”

系统会从请求里提取一个稳定标识，叫做 `affinity seed`。

它不是 request id。request id 每次都变，不能用来做粘性。

默认优先级：

1. `prompt_cache_key`
2. `metadata.user_id`
3. 请求头 `Session_id`
4. 请求头 `Originator`
5. 请求头 `X-Data-Proxy-Affinity-Key`
6. 当前 API key 的 token id
7. 当前用户 id

为什么这么排：

- `prompt_cache_key` 更接近项目或工作区，最适合提高缓存命中。
- 没有项目标识时，再用会话标识。
- 还没有时，退化到 token id 或 user id。

需要注意：

```text
如果很多客户端共用同一个 data-proxy API key，并且请求里没有 prompt_cache_key、metadata.user_id 或会话头，
系统只能退化到 token id，这些客户端可能会被当成同一个 seed。
```

这种情况下，建议客户端传：

```text
X-Data-Proxy-Affinity-Key: 项目ID或设备ID
```

## 7. 系统如何选择 key

### 第一步：为每个 seed 生成稳定排序

系统会根据：

```text
渠道 ID + 分组 + 模型 + affinity seed
```

给这个渠道里的 key 生成一个稳定排序。

例子：

```text
key 3 -> key 1 -> key 2
```

意思是：

```text
优先用 key 3。
key 3 不可用时，用 key 1。
key 1 也不可用时，用 key 2。
```

技术上使用 Rendezvous Hash。它的好处是：新增、删除、禁用某个 key 时，不会让所有用户全部重新分配。

### 第二步：保存短期绑定

系统会保存一个绑定：

```text
seed 指纹 -> key 指纹
```

默认有效期：

```text
1 小时
```

这里记录的是指纹，不是原始 seed，也不是原始上游 key。

### 第三步：判断 key 是否过载

第一版只看两个指标：

```text
RPM：最近 1 分钟请求数
Inflight：当前还没结束的请求数
```

`Inflight` 不跟随分钟窗口清零。长流式请求跨过分钟边界时，仍然会继续计入当前并发，直到请求结束释放。

为什么先不用 TPM：

- token 数通常要等响应结束后才知道。
- 流式请求中途统计 token 会更复杂。
- 第一版先用 RPM + Inflight，已经能解决“大量请求集中到某个 key”的主要问题。

## 8. 什么叫“超过平均负载”

假设 3 个 key 当前 RPM 是：

```text
key 1: 100 rpm
key 2: 40 rpm
key 3: 40 rpm
平均值: 60 rpm
```

key 1 的负载倍数：

```text
100 / 60 = 1.67
```

默认规则：

```text
软过载：超过平均负载 1.25 倍
硬过载：超过平均负载 1.80 倍
```

处理方式：

| 情况 | 处理 |
| --- | --- |
| 新 seed 遇到软过载 key | 尽量选择其他 key |
| 已绑定 seed 遇到软过载 key | 继续使用原 key |
| 已绑定 seed 遇到硬过载 key | 冷却期结束后允许迁移 |
| key 被禁用 | 直接选择其他启用 key |

为了避免低流量误判，第一版要加最小门槛：

```text
请求太少时，不判断过载。
例如总请求很少、当前 key 只有 1 个请求时，不应该因为比例波动就迁移。
```

## 9. 为什么新会话能切，老会话不要频繁切

因为我们的目标是提高缓存命中。

如果老会话一看到负载波动就切 key：

```text
第 1 次 -> key 2
第 2 次 -> key 1
第 3 次 -> key 3
```

那就又变成随机了，缓存命中会变差。

所以策略是：

```text
新会话负责分流。
老会话负责稳定。
严重过载或 key 不可用时，老会话才迁移。
```

迁移冷却默认：

```text
15 分钟
```

意思是：刚迁移过的 seed，短时间内不要因为负载波动再次迁移。

## 10. 日志应该记录什么

日志里要能看到为什么选了这个 key，但不能泄露敏感信息。

示例：

```json
{
  "multi_key_affinity": {
    "enabled": true,
    "mode": "sticky_hash_bounded",
    "seed_source": "prompt_cache_key",
    "seed_fp": "ab12cd34",
    "binding_hit": true,
    "selected_key_index": 2,
    "selected_key_fp": "ef56ab90",
    "primary_key_index": 2,
    "load_state": "normal",
    "key_load": 1,
    "avg_rpm": 12,
    "avg_inflight": 1,
    "fallback_reason": "binding"
  }
}
```

必须遵守：

- 不记录原始 `prompt_cache_key`。
- 不记录原始上游 key。
- 不记录完整用户敏感标识。
- 只记录 HMAC 后的短指纹和 key index。

## 11. UI 设计

### 渠道创建 / 编辑页

多秘钥策略显示：

```text
随机
轮询
负载保护粘性分配
```

选择“负载保护粘性分配”后，显示一段说明：

```text
适合 sub2api 多 key、Codex、Claude Code 和长上下文场景。
系统会尽量让同一个用户或会话稳定使用同一个上游 key。
当某个 key 明显过载时，新会话会自动分配到其他 key，已有会话不会频繁切换。
```

第一版不展示高级参数，避免小白管理员误调。

### 多 key 管理弹窗

第一版只需要继续展示：

- key index。
- 启用 / 禁用状态。
- 操作按钮。

实时负载状态可以后续再做，不放进第一版硬要求。

### 日志详情页

request trace 里显示：

- seed 来源。
- seed 短指纹。
- 选中的 key index。
- 是否命中绑定。
- 是否因为软过载绕开。

## 12. 技术实现方案

### 新增多 key 模式

内部值：

```go
const MultiKeyModeStickyHashBounded MultiKeyMode = "sticky_hash_bounded"
```

UI 不直接显示这个英文名，只显示中文：

```text
负载保护粘性分配
```

### 新增渠道配置

在 `ChannelInfo` 中新增：

```go
type MultiKeyAffinityPolicy struct {
    Enabled                           bool    `json:"enabled"`
    BindingTTLSeconds                 int     `json:"binding_ttl_seconds,omitempty"`
    MoveCooldownSeconds               int     `json:"move_cooldown_seconds,omitempty"`
    SoftLoadFactor                    float64 `json:"soft_load_factor,omitempty"`
    HardLoadFactor                    float64 `json:"hard_load_factor,omitempty"`
    ExistingBindingStayOnSoftOverload *bool   `json:"existing_binding_stay_on_soft_overload,omitempty"`
}
```

第一版可以不在 UI 暴露这些字段，只用默认值：

```text
binding_ttl_seconds = 3600
move_cooldown_seconds = 900
soft_load_factor = 1.25
hard_load_factor = 1.80
existing_binding_stay_on_soft_overload = true
```

这里用 `*bool` 是为了区分“管理员没有配置”和“明确配置为 false”。没有配置时默认保护已有绑定，明确配置为 false 时，已有绑定遇到软过载也可以迁移。

### 新增选择服务

建议集中在一个服务模块里：

```text
service/multi_key_affinity.go
```

职责：

- 提取 affinity seed。
- 生成 seed 指纹。
- 生成 key 指纹。
- 使用 Rendezvous Hash 排序。
- 读取 / 写入绑定。
- 统计 RPM 和 Inflight。
- 返回最终 key 和 key index。
- 写入日志信息。

第一版不需要拆太多文件。功能稳定后再拆 `seed`、`load`、`binding` 子模块。

### 接入点

在渠道已选定之后、请求发往上游之前：

```text
选择渠道 -> 在该渠道内选择 key -> 发起上游请求
```

非 `sticky_hash_bounded` 的渠道继续走原来的随机 / 轮询逻辑。

## 13. 开发顺序

### P0：文档与产品边界

目标：先明确第一版做什么，不做什么。

验收：

- 小白管理员知道什么时候启用。
- 开发知道哪些是 MVP，哪些是后续增强。

### P1：后端选择逻辑

任务：

- 新增 `sticky_hash_bounded` 模式。
- 提取默认 affinity seed。
- 实现 Rendezvous Hash 排序。
- 实现 Redis 优先、内存兜底的绑定。
- 实现 RPM + Inflight 过载判断。
- 写入 `multi_key_affinity` 日志。

验收：

- 同一 seed 多次请求选择同一 key index。
- 新 seed 避开软过载 key。
- 老 seed 软过载时继续用原 key。
- key 禁用后选择其他 key。

### P2：前端可用 UI

任务：

- 渠道多秘钥策略新增“负载保护粘性分配”。
- 选择该策略后显示简单说明。
- 渠道列表 / 多 key 弹窗能正确显示策略名称。

验收：

- 管理员能创建和编辑该策略。
- 不展示难懂的内部英文值。

### P3：日志和真实验证

任务：

- request trace 显示 `multi_key_affinity`。
- 使用 sub2api 多 key 渠道真实请求。
- 连续请求同一个 Codex 项目，检查 key index 是否稳定。
- 人为制造一个 key 请求数偏高，检查新 seed 是否绕开。

验收：

- 能按 request id 看出选 key 原因。
- 同一个 seed 稳定。
- 新 seed 会分流。

### P4：后续增强

这些不放进第一版：

- TPM 负载统计。
- 自定义 seed 提取规则。
- key 级缓存命中率面板。
- 多 key 实时负载面板。
- 自动分析 sub2api 是否内部随机。
- 多节点全局负载统计。
- 更复杂的迁移预算。

## 14. 原规划中不合理的地方，已经如何修改

### 1. 原规划概念太多，小白难理解

问题：

```text
一开始就讲 affinity seed、Rendezvous Hash、RPM、Inflight、迁移预算。
```

修改：

```text
先讲业务目标和使用方式，再讲技术细节。
```

### 2. 不应该第一版暴露太多高级参数

问题：

```text
软过载阈值、硬过载阈值、TTL、迁移冷却都放到 UI，容易被误调。
```

修改：

```text
第一版 UI 只给一个策略选择和说明文案。高级参数先用默认值。
```

### 3. “key 故障立即换 key”说得太满

问题：

```text
如果上游请求已经发出后才失败，是否重试、是否换 key，取决于现有 retry 机制。
```

修改：

```text
第一版明确只保证选择阶段跳过禁用 key。
请求失败后的自动重试和换渠道，继续依赖系统 RetryTimes 和现有故障判断。
```

### 4. “多 key 实时负载面板”不应该进 MVP

问题：

```text
面板需要更多接口、状态聚合和展示设计，会拖慢第一版上线。
```

修改：

```text
第一版先把选择原因写进 request trace。
管理面板放到后续增强。
```

### 5. 纯粘性策略不建议单独暴露

问题：

```text
纯 sticky_hash 容易让所有活跃用户集中到少数 key，且没有保护。
```

修改：

```text
只暴露“负载保护粘性分配”。
内部实现可以叫 sticky_hash_bounded。
```

### 6. 平均负载需要最小门槛

问题：

```text
低流量下，1 个请求和 0 个请求也可能算出很高倍数，导致误判过载。
```

修改：

```text
增加最小请求数 / 最小并发门槛。
请求太少时不做过载迁移。
```

### 7. 单机边界要写清楚

问题：

```text
如果文档不说明，容易误以为这是多节点全局负载均衡。
```

修改：

```text
当前按单机 data-proxy 设计。
绑定可用 Redis 保存，但实时 RPM / Inflight 第一版按当前进程统计。
```

## 15. 最终验收标准

1. 管理员可以在渠道多秘钥策略中选择“负载保护粘性分配”。
2. 同一 `prompt_cache_key` / 会话 / API key 会稳定选择同一个 key index。
3. 某 key 超过平均负载 1.25 倍，并且达到最小流量门槛时，新 seed 会绕开它。
4. 已有 seed 不会因为软过载频繁迁移。
5. key 禁用后能自动选择其他启用 key。
6. request trace 中能看到 seed 来源、seed 指纹、key index、负载状态和选择原因。
7. 日志不泄露原始上游 key、原始 prompt cache key 或用户敏感信息。
