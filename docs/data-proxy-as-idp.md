# Data Proxy 作为第三方登录/授权提供方（IdP）

Data Proxy 可把本站账号授权给外部产品，并下发可调用本站 API 的 `sk-` Key。

## 接入方式

### 1) Device Code（桌面 / CLI）

1. `POST /api/connected-apps/:slug/device/start`
2. 用户打开 `verification_uri`（`/connect/device?user_code=...`）并批准
3. `POST /api/connected-apps/:slug/device/poll` 一次拿到 `api_key`

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

## 安全

- redirect_uri 精确匹配白名单
- 强制 PKCE S256
- authorization code 短 TTL、一次性
- API Key 明文只返回一次；用户可在 Profile 撤销
