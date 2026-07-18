# AI Code Rules — Connected App / OAuth IdP

本文档约束 Data Proxy 作为 **IdP**（Device Code、OAuth2 授权码 + PKCE、OIDC、Connected App 管理）相关开发。  
补充顶层 `CLAUDE.md` / `AGENTS.md`，与 `docs/ai-code-rules.md`（MCP）并列。

## 1. 工作边界

- 代码根目录：`upstream/new-api`。
- 本站是 **授权服务器 / 密钥签发方**，不是「用 GitHub 登录本站」的 RP 主路径（RP 在 `/oauth-login` 等，勿与 IdP 混淆）。
- 桌面 Agent（如鸟维斯 `niaoweisi`）走 **Device Code**；网站 RP 走 **authorization_code + PKCE**。
- 对接文档：
  - `docs/data-proxy-as-idp.md`
  - `docs/niaoweisi-desktop-integration.md`
  - `docs/niaoweisi-desktop-api-reference.md`
  - `docs/snapless-connected-app-integration.md`（Device 参考实现）

## 2. 开工前检查

```bash
pwd   # 应在 upstream/new-api
git status --short --branch
# 未完成审查债
rg -n "^- \[ \]" todo.md | head -40
# 必读
sed -n '1,120p' docs/ai-code-rules-idp.md
sed -n '1,80p' docs/data-proxy-as-idp.md
```

改动 OAuth/Device 时同时打开：

- `controller/connected_app_oauth.go`
- `controller/connected_app_developer.go`（device start/poll/authorize）
- `service/connected_app_oauth.go`
- `model/connected_app.go`
- `middleware/connected_app_scope.go`

## 3. 安全铁律（必须）

### 3.1 客户端生命周期

1. **禁用客户端不得发码、不得换 token**  
   - `Validate` / `Consent` / `Exchange` 均须检查 `status == enabled`。  
   - Consent / Exchange 还须检查 `SupportsAuthorizationCode()`（或 Device 路径的 `SupportsDeviceCode()` + trusted 规则）。  
   - 禁止只在 validate 检查、consent/exchange 漏检。

2. **redirect_uri 精确匹配白名单**  
   - 使用 `service.RedirectURIAllowed`；禁止前缀/通配模糊匹配。  
   - `authorization_code` / `both` 创建与审批时 **必须** 有至少一条 redirect。

3. **PKCE**  
   - 网站授权强制 S256；`code_verifier` 校验失败 → `invalid_grant`。

4. **client_secret**  
   - 仅 confidential 客户端；哈希存储（`HashConnectedAppOAuthValue`）。  
   - 明文 **只返回一次**（API `client_secret_once` + 管理端 copy-once UI）。  
   - **禁止** toast/日志/URL 展示完整 secret。  
   - 支持 confidential → public 时 **清空** `ClientSecretHash`，否则 public 客户端仍被要求 secret。

5. **API Key (`sk-`)**  
   - Device poll / token exchange 明文只出现一次。  
   - 网站 OAuth exchange **不得**每次无脑新建 unlimited key 而不处理旧 web key（应复用、轮换或吊销同 fingerprint/绑定）。  
   - 用户撤销 grant 必须禁用绑定 token 并清理 access token。

### 3.2 事务与一致性

1. Consent 路径：`UpsertConnectedAppGrant` +（若写）归因 + `CreateConnectedAppAuthCodeRecord` 必须在 **同一 DB 事务**；失败不得半提交。  
2. Device authorize 已在事务内的逻辑保持；新增写库步骤一并纳入。  
3. `SearchUsers` 等带 `connected_app_id` 子查询时，子查询必须使用 **同一 `tx`**，禁止外层 `tx`、内层全局 `DB`。

### 3.3 注册归因 vs 授权关系

| 字段/表 | 含义 | 何时写入 |
| --- | --- | --- |
| `users.signup_connected_app_id` | **账户注册来源**（渠道） | 仅真实注册路径（新用户创建时）；**禁止**在「老用户首次授权某 App」时写入 |
| `connected_app_grants` | 用户授权了哪些 App | Device/OAuth/管理发 key 时 Upsert |

- `MaybeSetUserSignupConnectedApp` **不得**挂在纯授权路径上冒充 signup。  
- 管理端筛选：`signup_app_id` = 注册来源；`connected_app_id` = 当前有效 grant。
- **注册归因入口**：`/sign-up?signup_app=<slug>` 或注册 body `signup_app`；OAuth 经 `/api/oauth/state?signup_app=<slug>` 写入 session。密码注册、通用 OAuth、GitHub/Discord/WeChat/OIDC/LinuxDO 新用户创建均解析 slug/id → `signup_connected_app_id`（helper：`applySignupConnectedAppFromSession/FromRequest`）。

### 3.4 内置 App（snapless / codex-dp / niaoweisi）

- `EnsureBuiltinConnectedApps` 默认应 **insert-if-missing**，或 upsert 时 **不要覆盖**运营可改字段：  
  `allowed_scopes`、`default_scopes`、`trusted`、`client_id`、`authorization_flow`、`status`。  
- 公开 Device 流要求：`enabled` + `trusted` + `SupportsDeviceCode()`。  
- 改 niaoweisi scopes 时同步文档与 `middleware/connected_app_scope.go` 路径映射。

## 4. 分层与文件地图

| 层 | 路径 |
| --- | --- |
| 路由 | `router/api-router.go`（`/api/oauth/*`、`/api/connected-apps/*`、user grants） |
| Controller | `connected_app.go`、`connected_app_oauth.go`、`connected_app_developer.go`、`connected_app_request.go`、`connected_app_user_grants.go` |
| Service | `service/connected_app_oauth.go` |
| Model | `model/connected_app.go`、`model/user.go`（signup 字段） |
| Middleware | `middleware/connected_app_scope.go` |
| 前端 | 管理 Connected Apps、Profile 授权列表、`/connect/device`、`/oauth/authorize`、`/developers` |

## 5. 验证清单（完成前）

按改动面选择（至少一项带 exit 0）：

```bash
# OAuth / Connected App
go test ./controller ./router ./service ./model ./middleware -count=1

# 若有专用测试
go test ./router -run 'ConnectedApp|OAuth|Snapless' -count=1
go test ./controller -run 'ConnectedApp|OAuth|Snapless' -count=1

# 前端（改 web/default 时）
cd web/default && bun run typecheck

# 空白/敏感
git diff --check
# 禁止把 sk- / capp_ 明文写进仓库
```

手动冒烟（改 Device/OAuth 时）：

1. `device/start` → 浏览器批准 → `device/poll` 一次拿到 `sk-`，再 poll 为 consumed。  
2. 禁用 app 后：validate/consent/exchange 均失败。  
3. 撤销 Profile 授权后，旧 `sk-` 不可用。

## 6. 明确禁止

- 把 `client_secret` 或 `sk-` 写进前端打包配置或文档示例真值。  
- 在 QidianBrowser 等产品客户端仓库实现本站 IdP（本站服务端在本仓库）。  
- 用单字段逗号列表代替 `connected_app_grants` 表达多 App 授权。  
- 跳过三库兼容（SQLite / MySQL / PostgreSQL）的迁移与查询。

## 7. 与 todo 的关系

审查未完成项列在仓库根 `todo.md` 章节  
**「Connected App / OAuth IdP — code review 未完成项」**。  
修债时用 skill `data-proxy-review-todo`，并在本规则下逐项关闭。
