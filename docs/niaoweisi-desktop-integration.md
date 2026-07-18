# 鸟维斯（niaoweisi）桌面 Agent 对接清单

Data Proxy 作为 IdP：桌面点「登录」→ 浏览器打开授权页 → 用户批准 → 桌面 poll 一次拿到 `sk-`，之后自由调用本站 API。

**不要**用网站 OAuth 回调到桌面。用 **Device Code**。

**登录后的额度 / 用量 / 价格 / 模型调用完整 API 说明（供鸟维斯直接阅读）：**  
→ **[niaoweisi-desktop-api-reference.md](./niaoweisi-desktop-api-reference.md)**

---

## 1. 应用配置（已内置）

启动时 `EnsureBuiltinConnectedApps` 会 upsert：

| 字段 | 值 |
| --- | --- |
| `slug` / `client_id` | `niaoweisi` |
| `name` | 鸟维斯桌面版 |
| `authorization_flow` | `device_code` |
| `trusted` | `true`（公开 device 接口要求） |
| `status` | enabled (`1`) |
| `client_type` | public（无 `client_secret`） |
| `allowed_scopes` / `default_scopes` | `openai.models openai.chat openai.responses quota.read token.manage` |

说明：

- 含 `token.manage`、不含单独的 management 专用组合 → **签发平台 `sk-` API Key**（与 Snapless 同类），不是 Codex 的 management token。
- 管理员可在 **系统设置 → Connected Apps** 改 scopes / 停用；勿把 `trusted` 关掉，否则公开 device 会失败。
- 若需现场手工创建（无内置时），`POST /api/connected-apps`（Admin）：

```json
{
  "slug": "niaoweisi",
  "name": "鸟维斯桌面版",
  "description": "鸟维斯桌面 Agent",
  "allowed_scopes": [
    "openai.models",
    "openai.chat",
    "openai.responses",
    "quota.read",
    "token.manage"
  ],
  "default_scopes": [
    "openai.models",
    "openai.chat",
    "openai.responses",
    "quota.read",
    "token.manage"
  ],
  "authorization_flow": "device_code",
  "client_type": "public",
  "trusted": true,
  "status": 1
}
```

---

## 2. 端点一览

设 `BASE_URL` 为站点根（生产例：`https://dp.app.mbu.ltd`，无尾斜杠）。

| 步骤 | 方法 | 路径 | 鉴权 |
| --- | --- | --- | --- |
| 开始 | `POST` | `/api/connected-apps/niaoweisi/device/start` | 无 |
| 打开授权页 | 浏览器 | `/connect/device?user_code=…&app_slug=niaoweisi` | 用户登录态 |
| 查询状态（页用） | `GET` | `/api/connected-apps/niaoweisi/device/status?user_code=…` | 用户登录 |
| 批准/拒绝（页用） | `POST` | `/api/connected-apps/niaoweisi/device/authorize` | 用户登录 |
| 轮询取 Key | `POST` | `/api/connected-apps/niaoweisi/device/poll` | 无（凭 `device_code`） |

通用响应信封：

```json
{ "success": true, "message": "", "data": { } }
```

失败时多为 `success: false` + `message`（业务错误文案）。

---

## 3. 请求 / 响应字段

### 3.1 `POST .../device/start`

**Request body**（均可选，建议都填）：

```json
{
  "device_id": "stable-machine-id",
  "device_name": "MacBook Pro",
  "platform": "darwin",
  "app_version": "1.0.0",
  "client": "niaoweisi"
}
```

| 字段 | 说明 |
| --- | --- |
| `device_id` | 设备稳定 ID；用于生成 fingerprint，同机复用绑定 |
| `device_name` | 展示名，默认类似 Desktop |
| `platform` | 如 `darwin` / `windows` / `linux` / `desktop` |
| `app_version` | 客户端版本 |
| `client` | 建议固定 `niaoweisi` |

也可用 Header（与 Snapless 兼容）：`X-Snapless-Device-Id` 等；鸟维斯优先 JSON。

**Success `data`：**

```json
{
  "device_code": "<64-char secret, keep local only>",
  "user_code": "ABCD-1234",
  "verification_uri": "https://…/connect/device?user_code=ABCD-1234&app_slug=niaoweisi",
  "expires_in": 600,
  "interval": 3,
  "app": { "id": 1, "slug": "niaoweisi", "name": "鸟维斯桌面版", "trusted": true, "status": 1, "…": "…" },
  "device": {
    "fingerprint": "…",
    "device_name": "MacBook Pro",
    "platform": "darwin",
    "app_version": "1.0.0",
    "client": "niaoweisi"
  }
}
```

| 字段 | 桌面必须处理 |
| --- | --- |
| `device_code` | 仅存内存/安全存储；**禁止**放进浏览器 URL |
| `user_code` | 可展示给用户对照（页上也会显示） |
| `verification_uri` | 系统浏览器打开 |
| `expires_in` | 秒；超时需重新 start（默认 **600**） |
| `interval` | poll 最小间隔秒（默认 **3**） |

### 3.2 用户浏览器

1. `open(verification_uri)`  
2. 未登录 → `/sign-in?redirect=…` → 回到授权页  
3. 用户点批准 → 页面调 `device/authorize`  
4. 成功后页面提示可返回应用；**不会**把 `sk-` 放进 URL  

桌面**不要**解析浏览器回调；只靠 poll。

### 3.3 `POST .../device/poll`

**Request：**

```json
{ "device_code": "<from start>" }
```

**未批准 / 等待中**（`success: true`）：

```json
{
  "status": "pending",
  "interval": 3
}
```

其它非成功终态同样形状：`status` + 可选 `interval`。

**批准且首次消费成功**（`success: true`，完整 token 载荷）：

```json
{
  "app": { "slug": "niaoweisi", "…": "…" },
  "grant": { "status": "authorized", "scopes": ["…"], "…": "…" },
  "device": { "fingerprint": "…", "…": "…" },
  "token": {
    "id": 123,
    "name": "…",
    "status": 1,
    "masked_key": "sk-****",
    "…": "…"
  },
  "endpoints": { "models": "…/v1/models", "chat": "…", "…": "…" },
  "base_url": "https://…/v1",
  "api_key": "sk-xxxxxxxx",
  "created": true,
  "rotated": false,
  "api_key_once": true,
  "instructions": { }
}
```

| 字段 | 说明 |
| --- | --- |
| `api_key` | **仅首次**从 `authorized` 消费时出现；立刻安全存储 |
| `api_key_once` | 提示明文只此一次 |
| `base_url` | 调 OpenAI 兼容 API 的 base（通常 `…/v1`） |
| `endpoints` | 按 scope 给出的路径提示 |

之后同一 `device_code` 再 poll → `status: consumed`，**不再**返回明文 key。

### 3.4 调用本站 API（模型 + 额度 + 用量 + 价格）

登录 poll 成功后会带：

- `base_url`：一般是 `{SITE}/v1`
- `endpoints`：按 scope 映射好的路径（见下）
- `api_key`：`sk-...`

统一鉴权：

```http
Authorization: Bearer sk-xxxxxxxx
```

#### 3.4.1 桌面 Agent 推荐能力矩阵（用 `sk-` 即可）

| 能力 | 方法 | 路径 | 所需 scope | 说明 |
| --- | --- | --- | --- | --- |
| 模型列表 | `GET` | `{base_url}/models` 即 `/v1/models` | `openai.models` | OpenAI 兼容 |
| 聊天补全 | `POST` | `/v1/chat/completions` | `openai.chat` | OpenAI 兼容 |
| Responses | `POST` | `/v1/responses` | `openai.responses` | OpenAI 兼容 |
| **Key 额度/用量** | `GET` | `/api/usage/token` | `quota.read` | **鸟维斯首选额度接口** |
| OpenAI 式订阅额度 | `GET` | `/v1/dashboard/billing/subscription` | Token 鉴权 | 兼容旧客户端 |
| OpenAI 式累计用量 | `GET` | `/v1/dashboard/billing/usage` | Token 鉴权 | 兼容旧客户端 |
| **模型价格表** | `GET` | `/api/pricing` | 站点「定价页」模块开关；默认可匿名 | 含 model_price / ratio / actual_price |

`niaoweisi` 默认 scopes 已包含：`openai.models openai.chat openai.responses quota.read token.manage`，覆盖上表主路径。

poll 成功时 `endpoints` 示例：

```json
{
  "models": "https://dp…/v1/models",
  "chat_completions": "https://dp…/v1/chat/completions",
  "responses": "https://dp…/v1/responses",
  "token_usage": "https://dp…/api/usage/token"
}
```

#### 3.4.2 `GET /api/usage/token` — Key 额度查询（首选）

```bash
curl -sS "$SITE/api/usage/token" \
  -H "Authorization: Bearer sk-..."
```

响应（注意字段是 `code` 不是 `success`）：

```json
{
  "code": true,
  "message": "ok",
  "data": {
    "object": "token_usage",
    "name": "鸟维斯桌面版 - …",
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

| 字段 | 含义 |
| --- | --- |
| `total_available` | 该 Key 剩余额度（站点 quota 单位） |
| `total_used` | 该 Key 已用 |
| `total_granted` | 已用 + 剩余 |
| `unlimited_quota` | 是否不限 Key 额度（仍受**用户钱包/套餐**约束） |
| `expires_at` | 过期时间戳；`0` 表示不过期 |
| `model_limits*` | Key 级模型白名单（若启用） |

> Device 登录签发的 Key 通常 `unlimited_quota=true`：Key 本身不设硬顶，扣费走用户账户。桌面展示「还能用多少」时：  
> - 若 `unlimited_quota=false` → 用 `total_available`  
> - 若 `unlimited_quota=true` → 仍可展示 `total_used`；账户级余额见 3.4.5（会话接口）或依赖调用失败时的不足错误

#### 3.4.3 OpenAI 兼容 Billing（可选）

```bash
# 剩余/总额（兼容 OpenAI dashboard 字段）
curl -sS "$SITE/v1/dashboard/billing/subscription" \
  -H "Authorization: Bearer sk-..."

# 累计用量
curl -sS "$SITE/v1/dashboard/billing/usage" \
  -H "Authorization: Bearer sk-..."
```

- `subscription`：含 `hard_limit_usd` / `soft_limit_usd` 等（数值按站点展示类型换算，不一定是真美元）  
- `usage`：`total_usage`（×100 的展示单位，与 OpenAI 习惯对齐）

适合已对接 OpenAI Billing API 的 Agent 壳；新写鸟维斯优先用 `/api/usage/token`。

#### 3.4.4 `GET /api/pricing` — 模型价格

```bash
curl -sS "$SITE/api/pricing"
# 若站点开启「定价页需登录」，则：
curl -sS "$SITE/api/pricing" -H "Authorization: Bearer sk-..."
# 或带用户会话 Cookie；sk- 能否通过取决于 HeaderNav 中间件是否识别 Token
```

默认定价模块 **enabled + 不强制登录** 时可匿名拉全表。

`data[]` 每项主要字段：

| 字段 | 含义 |
| --- | --- |
| `model_name` | 模型 ID |
| `display_name` | 展示名 |
| `quota_type` | 计费类型 |
| `model_ratio` / `completion_ratio` | 倍率 |
| `model_price` | 固定价（若适用） |
| `enable_groups` | 可用分组 |
| `supported_endpoint_types` | 端点类型 |
| `actual_price` / `actual_price_by_group` | 实测有效价（含 per 1M tokens 等） |
| `billing_mode` / `billing_expr` | 表达式计费（若开启） |

同响应还有：`vendors`、`group_ratio`、`usable_group`、`supported_endpoint`、`auto_groups`。

桌面建议：启动或设置页缓存一份 pricing，展示模型列表价格；调用前可与 `/v1/models` 做交集。

#### 3.4.5 需要「用户会话」的接口（`sk-` 通常不够）

下列走 `UserAuth`（网站登录 Cookie / access token），**不是** Device poll 下发的 API Key 主路径：

| 能力 | 路径 | 备注 |
| --- | --- | --- |
| 用户资料 | `GET /api/user/self` | 会话 |
| **统一额度总览**（钱包+套餐+订阅） | `GET /api/user/quota-overview` | 会话；最完整账户余额 |
| 模型套餐列表 | `GET /api/user/model-token-packages` | 会话 |
| 调用日志 | `GET /api/log/self` | 会话 |
| 按日额度统计 | `GET /api/data/self` | 会话 |
| 撤销本应用授权 | `DELETE /api/user/connected-app-grants/:app_id` | 会话 |

鸟维斯若只要「用 Key 调模型 + 看 Key 用量 + 看公开价格」，**不必**接会话接口。  
若要做「钱包余额 / 套餐剩余」完整账户页，需要另做网页登录或后续扩展「用 grant 读账户只读 API」（当前未对 `sk-` 开放）。

#### 3.4.6 模型调用示例

```bash
# 模型列表
curl -sS "$SITE/v1/models" -H "Authorization: Bearer sk-..."

# Chat
curl -sS "$SITE/v1/chat/completions" \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

额度不足、Key 禁用、grant 撤销时，调用会失败；桌面应引导重新 Device Login 或打开站点充值页（如 `$SITE/wallet`）。

---

## 4. Poll 状态机

```text
                  start()
                     │
                     ▼
              ┌─────────────┐
              │   pending   │◄── poll（继续等，sleep interval）
              └──────┬──────┘
         用户批准    │    用户拒绝 / 超时
                     ▼
              ┌─────────────┐         ┌──────────┐
              │ authorized  │         │  denied  │ → 失败，可重新 start
              └──────┬──────┘         └──────────┘
                     │ 首次 poll 成功
                     ▼
              ┌─────────────┐         ┌──────────┐
              │  返回 api_key│         │ expired  │ → 失败，重新 start
              │  并变 consumed│         └──────────┘
              └──────┬──────┘
                     ▼
              ┌─────────────┐
              │  consumed   │ → 正常结束；再 poll 无 key
              └─────────────┘
```

### 桌面应识别的 `data.status`（poll）

| status | 含义 | 桌面动作 |
| --- | --- | --- |
| `pending` | 用户尚未操作 | `sleep(interval)` 后继续 poll |
| `authorized` | 本次响应含 key（首次消费） | 存 `api_key`，登录成功 |
| `consumed` | 已取过 key 或竞态已消费 | 若本地已有 key → 成功；否则当失败并提示重登 |
| `denied` | 用户拒绝 | 失败 UI |
| `expired` | 超过 `expires_in` | 失败，重新 start |
| `not_found` | device_code 无效 | 失败，重新 start |
| `invalid_app` | code 不属于 niaoweisi | 配置/环境错误 |
| `missing_user` / `missing_token` | 服务端异常态 | 失败，可重试 start 或报错 |

注意：等待态也是 **HTTP 200 + success true**，用 **`data.status`** 分支，不要只看 HTTP 码。

### 建议 poll 循环

```text
deadline = now + expires_in
interval = max(start.interval, 3)

loop:
  if now > deadline: fail("expired")
  r = poll(device_code)
  switch r.status:
    pending: sleep(interval); continue
    authorized:
      if r.api_key: save; success
      else: sleep(interval); continue   # 极少见竞态
    consumed:
      if has_local_key: success else fail("already used")
    denied | expired | not_found | invalid_app: fail(r.status)
    default: sleep(interval)
```

**禁止**以快于 `interval` 的频率狂 poll（服务端会记 `last_polled_at`，无额外 slow_down 错误码，但浪费且不礼貌）。

---

## 5. 错误与文案（常见）

业务失败：`success: false`，`message` 示例：

| message / 场景 | 原因 |
| --- | --- |
| `connected app not found` | slug 错或未部署内置 |
| `connected app is disabled` | 管理员停用 |
| `connected app is not trusted` | `trusted=false` |
| `connected app authorization_flow is not supported` | 非 device 流 |
| `device_code 不能为空` | poll body 缺字段 |
| `设备授权码不存在` | user_code 错（authorize/status） |
| `设备授权码不属于该应用` | app_slug 与 session 不一致 |
| `设备授权码已完成授权` / `已被使用` / `已过期` / `已拒绝` | authorize 时 session 非 pending |
| `创建设备授权会话失败` | 服务端生成 code 失败 |

Poll 对「找不到 session」返回 **`success: true, data.status=not_found`**（不是 false），桌面按状态表处理。

---

## 6. 安全清单（桌面必须）

1. **`device_code` 永不进日志/URL/剪贴板**（可只展示 `user_code`）。  
2. **`api_key` 只出现在首次成功 poll**；用 OS 安全存储（Keychain / Credential Manager）。  
3. 同 `device_id` 尽量稳定，便于服务端设备绑定与用户在 Profile 撤销。  
4. 用户撤销授权后 key 会失效；401 时引导重新 Device Login。  
5. 不要把 `client_secret` 编进桌面包（本应用为 public，本来就没有）。

用户撤销入口：

- Profile → **Authorized applications**（跨应用 grant）  
- 或设备相关卡片（若绑定展示）

---

## 7. 桌面伪代码（完整）

```text
function login():
  start = POST BASE/api/connected-apps/niaoweisi/device/start
    body: { device_id, device_name, platform, app_version, client: "niaoweisi" }
  if not start.success: showError(start.message); return

  openBrowser(start.data.verification_uri)
  showUI("在浏览器完成授权，用户码: " + start.data.user_code)

  deadline = now() + start.data.expires_in
  interval = start.data.interval or 3
  device_code = start.data.device_code

  while now() < deadline:
    poll = POST BASE/api/connected-apps/niaoweisi/device/poll
      body: { device_code }
    if not poll.success:
      showError(poll.message); return
    st = poll.data.status
    if st == "pending":
      sleep(interval); continue
    if st == "authorized" and poll.data.api_key:
      secureStore(poll.data.api_key)
      setBaseURL(poll.data.base_url)
      return success
    if st in ("denied", "expired", "not_found", "invalid_app"):
      showError(st); return
    if st == "consumed":
      showError("授权码已使用，请重试登录"); return
    sleep(interval)

  showError("授权超时，请重试")
```

---

## 8. 联调自检

```bash
BASE=https://dp.app.mbu.ltd   # 或本地

# 1) start
curl -sS -X POST "$BASE/api/connected-apps/niaoweisi/device/start" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"dev-1","device_name":"Dev","platform":"darwin","app_version":"0.1.0","client":"niaoweisi"}' | jq .

# 2) 浏览器打开 data.verification_uri，登录并批准

# 3) poll（替换 DEVICE_CODE）
curl -sS -X POST "$BASE/api/connected-apps/niaoweisi/device/poll" \
  -H 'Content-Type: application/json' \
  -d '{"device_code":"DEVICE_CODE"}' | jq .

# 4) 额度 / 价格 / 模型 / 调用（详见 api-reference）
curl -sS "$BASE/api/usage/token" -H "Authorization: Bearer sk-..."
curl -sS "$BASE/api/pricing" | jq '.success, (.data|length)'
curl -sS "$BASE/v1/models" -H "Authorization: Bearer sk-..."
curl -sS "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}'
curl -sS "$BASE/v1/dashboard/billing/subscription" -H "Authorization: Bearer sk-..."
curl -sS "$BASE/v1/dashboard/billing/usage" -H "Authorization: Bearer sk-..."
```

完整字段与错误码见 **[niaoweisi-desktop-api-reference.md](./niaoweisi-desktop-api-reference.md)**。

---

## 9. 与其它内置 App 对比

| App | slug | 登录后拿到 | 用途 |
| --- | --- | --- | --- |
| Snapless | `snapless` | `sk-` + 模型限制 | 桌面语音/划词 |
| Codex DP | `codex-dp` | management token | 客户端自管 token |
| **鸟维斯** | **`niaoweisi`** | **`sk-` API Key** | **Agent 自由调本站 API** |

---

## 10. 相关代码 / 页

- 内置配置：`model/connected_app.go` → `EnsureBuiltinConnectedApps`  
- Device API：`controller/connected_app_developer.go`（start/status/authorize/poll）  
- 授权页：`/connect/device`  
- 开发者说明：`/developers`、`docs/data-proxy-as-idp.md`  
- **登录后 API 参考（额度/用量/价格/模型）：** [niaoweisi-desktop-api-reference.md](./niaoweisi-desktop-api-reference.md)
