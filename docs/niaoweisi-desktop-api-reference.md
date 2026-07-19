# 鸟维斯桌面 Agent — Data Proxy API 参考

面向 **鸟维斯（`niaoweisi`）桌面版** 的对接文档。  
读者：桌面客户端工程师。服务端为 Data Proxy（本站 IdP + OpenAI 兼容网关）。

**配套文档**

| 文档 | 内容 |
| --- | --- |
| [niaoweisi-desktop-integration.md](./niaoweisi-desktop-integration.md) | Device Code 登录、poll 状态机、应用配置 |
| [data-proxy-as-idp.md](./data-proxy-as-idp.md) | IdP 总览（Device + 网站 OAuth） |
| **本文** | 登录后：额度 / 用量 / 价格 / 模型调用的完整接口说明 |

---

## 0. 约定

### 0.1 Base URL

```text
SITE = https://dp.app.mbu.ltd    # 生产示例，无尾斜杠
V1   = {SITE}/v1
```

本地或其它环境把 `SITE` 换成实际域名即可。

### 0.2 鉴权

Device Code 登录成功后，桌面只持有一把 **API Key**：

```http
Authorization: Bearer sk-<key>
```

- 不要把 `sk-` 写进 URL、日志、崩溃上报。
- 同一 Key 用于模型调用、Key 额度查询、（可选）OpenAI 兼容 billing。
- Connected App 签发的 Key 会做 **scope 校验**；缺少 scope 时返回 403。

### 0.3 应用与默认 scopes

| 项 | 值 |
| --- | --- |
| App slug | `niaoweisi` |
| 默认 scopes | `openai.models` `openai.chat` `openai.responses` `quota.read` `token.manage` |

登录流程见 [niaoweisi-desktop-integration.md](./niaoweisi-desktop-integration.md)。

### 0.4 响应信封差异（重要）

不同接口返回形状不完全统一，桌面必须按接口解析：

| 接口族 | 成功特征 | 失败特征 |
| --- | --- | --- |
| Device / 多数 `/api/*` | `{ "success": true, "data": … }` | `{ "success": false, "message": "…" }` |
| `/api/usage/token` | `{ "code": true, "data": … }` | `{ "code": false }` 或 401 / 业务失败 |
| `/v1/*` 模型 | OpenAI 风格 JSON | `{ "error": { "message", "type", … } }` |
| `/v1/dashboard/billing/*` | 顶层业务字段 | `{ "error": { … } }`（HTTP 常仍为 200） |
| `/api/pricing` | `{ "success": true, "data": […], … }` | `success: false` 或 401/403 |

---

## 1. 能力总表（桌面必读）

### 1.1 用 `sk-` 即可（鸟维斯主路径）

| # | 能力 | Method | Path | Scope | 文档章节 |
| --- | ---: | --- | --- | --- | --- |
| 1 | 模型列表 | `GET` | `/v1/models` | `openai.models` | §3 |
| 2 | 聊天补全 | `POST` | `/v1/chat/completions` | `openai.chat` | §4 |
| 3 | Responses API | `POST` | `/v1/responses` | `openai.responses` | §5 |
| 4 | **Key 额度 / 用量** | `GET` | `/api/usage/token` | `quota.read` | §2 |
| 5 | OpenAI 式额度（兼容） | `GET` | `/v1/dashboard/billing/subscription` | Token 鉴权 | §6 |
| 6 | OpenAI 式累计用量（兼容） | `GET` | `/v1/dashboard/billing/usage` | Token 鉴权 | §6 |
| 7 | **模型价格表** | `GET` | `/api/pricing` | 定价模块开关；默认可匿名 | §7 |

Device poll 成功时会返回 `base_url` 与 `endpoints`，例如：

```json
{
  "base_url": "https://dp.app.mbu.ltd/v1",
  "endpoints": {
    "models": "https://dp.app.mbu.ltd/v1/models",
    "chat_completions": "https://dp.app.mbu.ltd/v1/chat/completions",
    "responses": "https://dp.app.mbu.ltd/v1/responses",
    "token_usage": "https://dp.app.mbu.ltd/api/usage/token"
  },
  "api_key": "sk-..."
}
```

桌面应优先使用 poll 返回的 URL，而不是写死路径。

### 1.2 需要网站用户会话（非 Device `sk-` 主路径）

下列接口使用 **UserAuth**（浏览器 Cookie / 用户 access token），**不能**指望仅用 Device 下发的 `sk-` 稳定访问：

| 能力 | Method | Path | 说明 |
| --- | --- | --- | --- |
| 用户资料 | `GET` | `/api/user/self` | 含 `quota` / `used_quota` 等 |
| 统一额度总览 | `GET` | `/api/user/quota-overview` | 钱包 + 套餐 + 订阅（最完整） |
| 模型套餐 | `GET` | `/api/user/model-token-packages` | 套餐包列表 |
| 调用日志 | `GET` | `/api/log/self` | 分页日志 |
| 按日额度 | `GET` | `/api/data/self` | 统计 |
| 撤销应用授权 | `DELETE` | `/api/user/connected-app-grants/:app_id` | 用户在网页撤销 |

**鸟维斯 MVP 可不实现 1.2。** 账户级「钱包余额」若必须在桌面展示，需另开网页登录或后续服务端扩展；当前文档以 1.1 为准。

---

## 2. Key 额度查询 — `GET /api/usage/token`（首选）

查询 **当前这把 API Key** 的额度与用量。  
Connected App Key 需要 scope **`quota.read`**（`niaoweisi` 默认已包含）。

### 2.1 请求

```http
GET /api/usage/token HTTP/1.1
Host: dp.app.mbu.ltd
Authorization: Bearer sk-<key>
```

```bash
curl -sS "${SITE}/api/usage/token" \
  -H "Authorization: Bearer ${API_KEY}"
```

路径兼容：`/api/usage/token` 与 `/api/usage/token/`。

### 2.2 成功响应

```json
{
  "code": true,
  "message": "ok",
  "data": {
    "object": "token_usage",
    "name": "鸟维斯桌面版 - MacBook Pro",
    "total_granted": 1000000,
    "total_used": 12345,
    "total_available": 987655,
    "unlimited_quota": true,
    "quota_hard_limit_enabled": false,
    "model_limits": {},
    "model_limits_enabled": false,
    "expires_at": 0
  }
}
```

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `code` | bool | **`true` 表示成功**（不是 `success`） |
| `data.name` | string | Key 显示名 |
| `data.total_granted` | int | 已用 + 剩余（Key 维度） |
| `data.total_used` | int | 该 Key 累计已用额度（站点 quota 单位） |
| `data.total_available` | int | 该 Key 剩余额度 |
| `data.unlimited_quota` | bool | Key 是否不限额度 |
| `data.quota_hard_limit_enabled` | bool | 是否启用 Key 硬上限 |
| `data.model_limits` | object | Key 级模型限制 map（若启用） |
| `data.model_limits_enabled` | bool | 是否启用模型限制 |
| `data.expires_at` | int64 | 过期 Unix 秒；**`0` = 不过期**（内部 `-1` 会映射为 `0`） |

### 2.3 桌面展示建议

Device 登录签发的 Key **通常** `unlimited_quota = true`：

- Key 本身不设硬顶，实际扣费仍受 **用户钱包 / 套餐 / 订阅** 约束。
- UI 可展示：`total_used`（本机 Key 累计消耗）。
- `total_available` 在 unlimited 时参考意义有限；账户总余额见 §1.2（会话）或依赖模型调用失败提示充值。
- 若 `unlimited_quota = false`：用 `total_available` 作为「Key 剩余」。

额度单位为站点内部 **quota**，不是直接美元。换算依赖站点 `QuotaPerUnit` / 展示类型（USD / CNY / Tokens），与 §6 一致。

### 2.4 错误

| 场景 | 表现 |
| --- | --- |
| 无 Authorization | 401，`No Authorization header` |
| 非法 Bearer | 401，`Invalid Bearer token` |
| Key 无效 | 业务失败 / 取 token 失败 |
| 缺 `quota.read`（Connected App） | 403，OpenAI 风格 error，提示 requires scope |

---

## 3. 模型列表 — `GET /v1/models`

### 3.1 请求

```http
GET /v1/models HTTP/1.1
Authorization: Bearer sk-<key>
```

```bash
curl -sS "${SITE}/v1/models" \
  -H "Authorization: Bearer ${API_KEY}"
```

Scope：`openai.models`。

### 3.2 响应

OpenAI 兼容：

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "created": 0,
      "owned_by": "…"
    }
  ]
}
```

桌面：用 `data[].id` 作为可选模型列表；可与 §7 价格表按 `model_name` 做 join 展示单价。

### 3.3 错误

- 401：Key 无效 / 未传
- 403：缺 scope / grant 撤销 / binding 失效
- body 多为 `{ "error": { "message": "…", "type": "…" } }`

---

## 4. 聊天补全 — `POST /v1/chat/completions`

### 4.1 请求

```http
POST /v1/chat/completions HTTP/1.1
Authorization: Bearer sk-<key>
Content-Type: application/json
```

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    { "role": "user", "content": "你好" }
  ],
  "stream": false
}
```

```bash
curl -sS "${SITE}/v1/chat/completions" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

Scope：`openai.chat`。

### 4.2 响应

标准 OpenAI Chat Completions JSON（`choices`、`usage` 等）。  
`stream: true` 时为 SSE，与 OpenAI 客户端兼容。

### 4.3 常见失败

| 原因 | 桌面处理 |
| --- | --- |
| 额度不足 | 提示打开 `{SITE}/wallet` 充值，或稍后重试 |
| 模型不存在 / 无权限 | 刷新 `/v1/models`，换模型 |
| Key 禁用 / grant 撤销 | 引导重新 Device Login |
| 上游错误 | 展示 `error.message`，可重试 |

---

## 5. Responses API — `POST /v1/responses`

### 5.1 请求

```http
POST /v1/responses HTTP/1.1
Authorization: Bearer sk-<key>
Content-Type: application/json
```

Scope：`openai.responses`。  
请求/响应遵循 OpenAI Responses API 形态（具体字段随上游与网关版本演进；与站内 Playground 一致）。

```bash
curl -sS "${SITE}/v1/responses" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","input":"hi"}'
```

（`input` / 其它字段以当前站点上游兼容为准；联调时以实际 4xx 提示调整。）

### 5.2 相关路径

- `POST /v1/responses`
- `POST /v1/responses/compact`（若启用；同样需要 `openai.responses`）

---

## 6. OpenAI 兼容 Billing（可选）

适合已对接 OpenAI Dashboard Billing 的客户端壳。  
**新写鸟维斯优先用 §2 `/api/usage/token`。**

鉴权：`Authorization: Bearer sk-...`（`TokenAuth`）。

### 6.1 `GET /v1/dashboard/billing/subscription`

（同路径别名：`/dashboard/billing/subscription`）

**成功示例：**

```json
{
  "object": "billing_subscription",
  "has_payment_method": true,
  "soft_limit_usd": 12.34,
  "hard_limit_usd": 12.34,
  "system_hard_limit_usd": 12.34,
  "access_until": 0
}
```

| 字段 | 含义 |
| --- | --- |
| `soft_limit_usd` / `hard_limit_usd` / `system_hard_limit_usd` | 额度换算后的展示值；**名称带 USD，实际随站点展示类型（USD/CNY/Tokens）** |
| `access_until` | Key 过期时间；`0` 表示不限制或未设置 |
| unlimited Key | 内部会把 amount 抬到很大值（如 `1e8`） |

换算逻辑概要：

- 站点 USD：`quota / QuotaPerUnit`
- 站点 CNY：先转 USD 再乘汇率
- 站点 Tokens：保持 token 数值

### 6.2 `GET /v1/dashboard/billing/usage`

（同路径别名：`/dashboard/billing/usage`）

**成功示例：**

```json
{
  "object": "list",
  "total_usage": 123.45
}
```

| 字段 | 含义 |
| --- | --- |
| `total_usage` | 累计用量；**单位为「0.01 展示货币」**（即内部 amount × 100，对齐 OpenAI 习惯） |

### 6.3 错误形态

```json
{
  "error": {
    "message": "…",
    "type": "upstream_error"
  }
}
```

注意：部分错误 **HTTP 仍为 200**，桌面要看是否存在 `error` 字段。

---

## 7. 模型价格 — `GET /api/pricing`

### 7.1 请求

```http
GET /api/pricing HTTP/1.1
```

```bash
# 默认定价页公开时可匿名
curl -sS "${SITE}/api/pricing"

# 若站点开启「定价需登录」
curl -sS "${SITE}/api/pricing" \
  -H "Authorization: Bearer ${API_KEY}"
```

中间件：`HeaderNavModuleAuth("pricing")`。

| 站点配置 | 行为 |
| --- | --- |
| 模块 enabled + requireAuth=false（默认常见） | **可匿名** |
| requireAuth=true | 需登录；未登录 401 |
| 模块 disabled | 403 |

桌面：启动时拉一次并缓存（建议 TTL 5–30 分钟）；失败时降级为只显示 `/v1/models` 的 id。

### 7.2 成功响应（结构）

```json
{
  "success": true,
  "data": [
    {
      "model_name": "gpt-4o-mini",
      "display_name": "GPT-4o mini",
      "description": "",
      "icon": "",
      "tags": "",
      "vendor_id": 1,
      "quota_type": 0,
      "model_ratio": 0.15,
      "model_price": 0,
      "owner_by": "",
      "completion_ratio": 4,
      "cache_ratio": null,
      "enable_groups": ["default"],
      "supported_endpoint_types": ["openai"],
      "billing_mode": "",
      "billing_expr": "",
      "actual_price": {
        "window_seconds": 3600,
        "request_count": 10,
        "total_tokens": 1000,
        "effective_price_per_1m_tokens": 0.5,
        "effective_price_per_1k_tokens": 0.0005,
        "effective_price_per_request": 0,
        "price_unit": "USD"
      },
      "actual_price_by_group": {},
      "pricing_version": ""
    }
  ],
  "vendors": [
    { "id": 1, "name": "OpenAI", "description": "", "icon": "" }
  ],
  "group_ratio": { "default": 1 },
  "usable_group": { "default": "默认分组" },
  "supported_endpoint": {},
  "auto_groups": ["default"]
}
```

### 7.3 `data[]` 字段说明

| 字段 | 含义 |
| --- | --- |
| `model_name` | 模型 ID（与 `/v1/models` 的 `id` 对齐） |
| `display_name` | UI 展示名 |
| `quota_type` | 计费类型（倍率 / 固定价等，站点定义） |
| `model_ratio` | 模型倍率 |
| `completion_ratio` | 补全倍率 |
| `model_price` | 固定价格（若适用） |
| `enable_groups` | 对该模型开放的用户分组 |
| `supported_endpoint_types` | 支持的端点类型 |
| `billing_mode` / `billing_expr` | 表达式计费（若开启） |
| `actual_price` | 基于近期真实调用的有效价（可能为空） |
| `actual_price.effective_price_per_1m_tokens` | 每 1M tokens 有效价 |
| `actual_price.effective_price_per_1k_tokens` | 每 1K tokens 有效价 |
| `actual_price.price_unit` | 价格单位字符串 |
| `vendors` | 厂商列表（顶层） |
| `group_ratio` | 分组倍率 |
| `usable_group` | 当前可见分组（登录用户会按分组过滤） |

### 7.4 桌面用法示例

```text
models = GET /v1/models          # 用户当前 Key 可用模型
pricing = GET /api/pricing       # 全站价格元数据
for m in models.data:
  p = pricing.data.find(x => x.model_name == m.id)
  show(m.id, p?.display_name, p?.actual_price?.effective_price_per_1m_tokens)
```

---

## 8. Scope 与端点强制关系

对 **Connected App 签发的 Key**，网关在部分路径上强制 scope：

| Method + Path | 需要 scope |
| --- | --- |
| `GET /v1/models`、`GET /v1/models/:model` | `openai.models` |
| `POST /v1/chat/completions` | `openai.chat` |
| `POST /v1/responses`、`POST /v1/responses/compact` | `openai.responses` |
| `POST /v1/audio/transcriptions` | `openai.audio.transcriptions`（鸟维斯默认未授权） |
| `GET /api/usage/token` | `quota.read` |

- App 的 `allowed_scopes` 与用户 `grant.scopes` **都要**包含该 scope。
- 普通用户在网页自建的 Key 不受 Connected App scope 中间件约束。
- 用户撤销 grant / 禁用 app / 禁用 binding 后，上述调用会 403。

`niaoweisi` **默认没有** `openai.audio.transcriptions`；若桌面要 ASR，需管理员在 Connected App 中追加 scope 并让用户重新授权。

---

## 9. 错误与 HTTP 状态（桌面统一处理）

| HTTP | 常见原因 | 建议 |
| ---: | --- | --- |
| 401 | 未带 Key、Key 无效、过期 | 重新 Device Login |
| 403 | 缺 scope、grant 撤销、app 禁用、pricing 模块关闭 | 提示权限/联系管理员；或打开网站检查授权 |
| 429 | 限流 | 退避重试 |
| 5xx | 服务端 / 上游 | 重试 + 展示错误信息 |
| 200 + `error` | Billing 等兼容接口 | 解析 body 的 `error` |
| 200 + `success:false` | 业务错误 | 读 `message` |
| 200 + `code:false` | usage/token 风格 | 读 `message` |

额度不足时模型接口通常返回带 message 的 error；桌面应提供「打开充值页」：

```text
{SITE}/wallet
```

---

## 10. 推荐集成顺序（鸟维斯）

```text
1. Device Code 登录
   → 存 api_key, base_url, endpoints
2. GET endpoints.token_usage 或 /api/usage/token
   → 设置页展示本 Key 用量
3. GET /api/pricing（可匿名）
   → 缓存价格
4. GET /v1/models
   → 与 pricing join，展示可选模型
5. POST /v1/chat/completions 或 /v1/responses
   → 主业务
6. 401/403 on 模型调用
   → 清 Key，引导重新登录
```

### 10.1 最小自检脚本

```bash
SITE=https://dp.app.mbu.ltd
API_KEY=sk-xxxx   # poll 得到

# 额度
curl -sS "$SITE/api/usage/token" -H "Authorization: Bearer $API_KEY" | jq .

# 价格
curl -sS "$SITE/api/pricing" | jq '.success, (.data|length)'

# 模型
curl -sS "$SITE/v1/models" -H "Authorization: Bearer $API_KEY" | jq '.data[:3]'

# 调用
curl -sS "$SITE/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}' | jq .

# 可选 billing
curl -sS "$SITE/v1/dashboard/billing/subscription" -H "Authorization: Bearer $API_KEY" | jq .
curl -sS "$SITE/v1/dashboard/billing/usage" -H "Authorization: Bearer $API_KEY" | jq .
```

---

## 11. 安全与合规（桌面必须）

1. `api_key` 仅存 OS 安全存储（Keychain / Credential Manager / libsecret）。
2. 日志中只打 Key 后 4 位或完全不打。
3. 不要实现「把 Key 同步到云端明文」。
4. 用户可在网站 **Profile → Authorized applications** 撤销 `niaoweisi`；撤销后 Key 失效，桌面应走重新登录。
5. `device_code` / `api_key` 禁止进入 `verification_uri` 或自定义 URL scheme 查询串。

---

## 12. 文档与代码索引

| 主题 | 位置 |
| --- | --- |
| 内置 app `niaoweisi` | `model/connected_app.go` → `EnsureBuiltinConnectedApps` |
| Device start/poll | `controller/connected_app_developer.go` |
| Scope 强制 | `middleware/connected_app_scope.go` |
| Key 用量 | `controller/token.go` → `GetTokenUsage` |
| OpenAI billing | `controller/billing.go`；路由 `router/dashboard.go` |
| 定价 | `controller/pricing.go`；`model/pricing.go` |
| 授权页 | `/connect/device` |
| 开发者落地页 | `/developers` |

---

## 13. 修订记录

| 日期 | 说明 |
| --- | --- |
| 2026-07-18 | 初版：额度 / 用量 / 价格 / 模型 API 供鸟维斯桌面阅读对接 |

## Device poll 用户摘要（DP-1 / 2026-07-20）

成功 poll（与 `api_key` 同次）返回 `user.id` / `user.username` / `user.display_name` / `user.group`。
`verification_uri` 含 `signup_app=<slug>`（DP-2）以便未登录注册归因。详见 `docs/data-proxy-as-idp.md`。
