# Data Proxy 作为第三方登录/授权提供方（IdP）

Data Proxy 可把本站账号授权给外部产品，并下发可调用本站 API 的 `sk-` Key。

## 接入方式

### 1) Device Code（桌面 / CLI）

1. `POST /api/connected-apps/:slug/device/start`
2. 用户打开 `verification_uri`（`/connect/device?user_code=...`）并批准
3. `POST /api/connected-apps/:slug/device/poll` 一次拿到 `api_key`

内置桌面示例：

| slug | 说明 |
| --- | --- |
| `snapless` | 桌面语音/划词 |
| `codex-dp` | Agent management token |
| `niaoweisi` | 鸟维斯桌面 Agent（平台 `sk-`） |

鸟维斯对接：

- Device 登录 / poll 状态机：[niaoweisi-desktop-integration.md](./niaoweisi-desktop-integration.md)
- 登录后 API（额度 / 用量 / 价格 / 模型）：[niaoweisi-desktop-api-reference.md](./niaoweisi-desktop-api-reference.md)

### 2) 网站跳转（OAuth 2.0 授权码 + PKCE）

1. 浏览器打开：

```text
GET /oauth/authorize
  ?client_id=<slug>
  &redirect_uri=<registered>
  &response_type=code
  &scope=openai.chat%20quota.read%20openid
  &state=<csrf>
  &code_challenge=<S256>
  &code_challenge_method=S256
  &nonce=<optional>
```

2. 用户登录并在同意页批准
3. 回调：`redirect_uri?code=...&state=...`
4. 换 token：

```bash
POST /api/oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code
&code=...
&redirect_uri=...
&client_id=...
&code_verifier=...
```

响应：

```json
{
  "access_token": "sk-...",
  "token_type": "Bearer",
  "scope": "openai.chat quota.read",
  "api_key": "sk-...",
  "api_key_once": true,
  "id_token": "<jwt>"
}
```

用 `Authorization: Bearer sk-...` 调用 `/v1/*`。

## OIDC 最小集

- `GET /.well-known/openid-configuration`
- `GET /oauth/jwks.json`
- `GET /api/oauth/userinfo`
- token 响应中的 `id_token`（RS256）

## 开发者入口

- 落地页：`/developers`
- 设备授权：`/connect/device`（旧 `/snapless/device` 会跳转）
- 管理：系统设置 → Connected Apps
- 申请：`POST /api/connected-app-requests`

## 注册渠道归因（signup）

给新用户打上来源 Connected App（**不是**授权 grant）：

```text
https://<host>/sign-up?signup_app=niaoweisi
```

- 前端会把 `signup_app` 存 localStorage，密码注册 body 带 `signup_app`。
- 第三方登录：`/api/oauth/state?signup_app=<slug>` 写入 session，新用户创建时写入 `users.signup_connected_app_id`。
- 无效 slug 忽略，不阻断注册。
- 管理端用户列表可用 `signup_app_id` 筛选注册来源；`connected_app_id` 筛的是当前授权关系。

## 安全

- redirect_uri 精确匹配白名单
- 强制 PKCE S256
- authorization code 短 TTL、一次性
- API Key 明文只返回一次；用户可在 Profile 撤销
- 禁用客户端不得 consent / token exchange
- 网站 OAuth 同 app/user/web 指纹会吊销旧 `sk-` 再发新 key

## Device 新用户注册归因（DP-2）

- `device/start` 返回的 `verification_uri` 对 Connected App（非纯 snapless 路径）包含 `signup_app=<slug>` 与 `app_slug`。
- `/connect/device` 未登录跳转 `/sign-in` 时携带 `signup_app`；sign-in 链到 sign-up / oauth-login 时保留该参数。
- 仅真实新用户创建写 `users.signup_connected_app_id`。

## Device poll 用户摘要（DP-1）

`POST /api/connected-apps/:slug/device/poll`（及 snapless 等价 poll）在**首次**成功返回 `api_key` 时包含：

```json
{
  "user": {
    "id": 12345,
    "username": "abc123",
    "display_name": "",
    "group": "default"
  }
}
```

`pending` / 二次 poll（`consumed`）不返回 `user` / `api_key`。

## 桌面账户额度（DP-4）

- `GET /api/usage/account`：TokenAuth + Connected App scope `quota.read`。
- `GET /api/usage/token` 成功时附带 `data.account` 摘要（不改变原有 Key 字段）。

## 邀请页（DP-5）

- 用户登录后访问 `/invitation` 查看/复制邀请链接；钱包内锚点 `/wallet#invitation`。
