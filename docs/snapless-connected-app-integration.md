# Snapless Connected App Integration

本文档记录 Data Proxy 对 Snapless Desktop 的最小接入闭环。Data Proxy 基于 new-api，继续保留 AGPLv3、NOTICE 和上游归属要求。

## V1.3 已实现

本阶段不把 Snapless 写成一次性特殊逻辑，而是新增最小 Connected App 内核：

- `connected_apps`：登记可连接应用，内置 trusted app `snapless`。
- `connected_app_grants`：记录用户授权和 scopes。
- `connected_app_token_bindings`：记录 app、用户、设备和原生 Token 的绑定。
- `connected_app_device_sessions`：记录 Device Code Flow 的 device code、user code、设备信息、授权状态和一次性消费状态。
- 用户控制台 Profile 页提供 Snapless Connected App 卡片，可查看 grant、设备、最近使用时间、token 状态，并可轮换或撤销单台设备。

内置 Snapless scopes：

- `openai.models`
- `openai.chat`
- `openai.audio.transcriptions`
- `quota.read`
- `token.manage`

## Snapless API

Device Code Flow：

- `POST /api/snapless/device/start`
  - 公开接口，由 Snapless Desktop 调用。
  - 请求体包含 `device_id`、`device_name`、`platform`、`app_version`、`client`。
  - 返回 `device_code`、`user_code`、`verification_uri`、`expires_in`、`interval`。
- `GET /api/snapless/device/status`
  - 登录用户接口，浏览器授权页按 `user_code` 查询设备信息和状态。
  - 响应保留 device session 的 `status`，并新增 `readiness` 描述当前账号/模型是否允许批准授权。
- `POST /api/snapless/device/authorize`
  - 登录用户接口，按 `user_code` 批准或拒绝授权。
  - 批准时服务端创建或复用 Snapless 原生 token，并只保存 `token_id` 到 device session。
  - 浏览器响应不返回 `api_key` 明文。
- `POST /api/snapless/device/poll`
  - 公开接口，由持有 `device_code` 的 Snapless Desktop 轮询。
  - `pending`、`expired`、`denied`、`consumed` 只返回状态和下次轮询间隔。
  - 首次从 `authorized` 消费成功时返回完整 token 响应和 `api_key`；随后同一 `device_code` 进入 `consumed`，不再返回明文 key。

登录用户接口：

- `GET /api/snapless/config`
- `GET /api/snapless/devices`
  - 返回当前用户的 Snapless app、grant、设备列表、模型 health、base URL 和 health-like checks。
  - 顶层返回 `ok`、`status`、`checks`、`actions`，用于展示账号余额、用户状态和模型可用性。
  - 每台设备返回 `ok`、`status`、`checks`、设备信息、token 摘要、`last_used_at`、`revoked_at` 等字段。
  - `status` 复用 `/api/snapless/health` 的语义，例如 `ok`、`token_disabled`、`grant_revoked`、`binding_revoked`、`quota_insufficient`、`models_unavailable`。
  - `actions.primary.href` 会指向可操作入口，例如余额不足跳转 `/wallet?source=snapless`，模型不可用跳转 `/system-settings/models`。
- `POST /api/snapless/devices/:fingerprint/rotate`
  - 只轮换指定设备的 Snapless token。
  - 旧 token 会被禁用，新 token 只在本次响应中返回 `api_key` 明文。
- `DELETE /api/snapless/devices/:fingerprint`
  - 只撤销指定设备的 binding 并禁用对应 token。
  - 当用户最后一个 active Snapless 设备被撤销时，同步撤销 grant。
- `POST /api/snapless/tokens/ensure`
- `POST /api/snapless/tokens/rotate`
- `DELETE /api/snapless/tokens/current`

客户端健康检查：

- `GET /api/snapless/health`
  - 返回 `actions` 字段，和 `status` 一起用于客户端或控制台展示下一步操作。

`ensure` 会为同一用户、同一设备复用已有 active binding。只有首次创建或 rotate 时返回 `api_key` 明文；复用已有 binding 时只返回 token 摘要。Device Code Flow 不把 API key 放入 URL，只允许桌面端凭 `device_code` 轮询一次获取。

## Connected App 管理

管理员接口：

- `GET /api/connected-apps`
  - 返回全部 connected apps，包含 `allowed_scopes`、`default_scopes`、`trusted`、`status`、`authorization_flow`、grant 数量和设备数量。
- `POST /api/connected-apps`
  - 新增应用。`slug` 只允许小写字母、数字、下划线和连字符；`allowed_scopes` 至少包含一个 scope；`default_scopes` 必须是 `allowed_scopes` 子集。
- `PUT /api/connected-apps/:id`
  - 更新应用展示信息、allowed/default scopes、trusted 状态和启用状态。

前端入口：

- `/system-settings/operations/connected-apps`
  - `Apps` 页签以表格展示应用状态、trusted 状态、scope、grant/device 数量和更新时间。
  - 使用右侧 Sheet 新增或编辑应用；内置 Snapless app 的 `slug=snapless` 保持不变。
  - `Requests` 页签展示第三方应用接入申请，管理员可批准或拒绝。
  - `Audit` 页签展示 connected app 申请提交、批准、拒绝等审计事件。

内置 seed 仍由 `EnsureBuiltinConnectedApps()` 维护 Snapless 的默认名称、描述、scopes 和 trusted 标记，但保留管理员设置的 `status`，避免升级时把手动停用的 Snapless 重新启用。

## 应用申请、审批和站内通知

`SNAPLESS-006` 提供最小可用的应用申请和权限审批闭环。当前版本先保证站内通知和审计可见，不直接发送 email 或 webhook。

数据表：

- `connected_app_requests`：记录申请人、目标 slug、展示信息、requested/default scopes、device code 授权方式、homepage/callback URL、申请原因、审批状态和审批备注。
- `connected_app_audit_logs`：记录申请提交、批准、拒绝等操作的 actor、target、before/after JSON 和 request ID。
- 已读状态复用现有 `enterprise_notification_reads`，通过稳定 notification key 记录 connected app 申请通知已读状态。

用户接口：

- `POST /api/connected-app-requests`
  - 登录用户提交应用接入申请。
  - `slug` 需符合 connected app slug 规则，且不能与已有 app 或待审批申请冲突。
  - `authorization_flow` 当前只支持 `device_code`。
  - `homepage_url`、`callback_url` 可选，但填写时必须是绝对 `http` 或 `https` URL。
- `GET /api/connected-app-requests/self`
  - 登录用户查看自己提交的应用申请和审批结果。

管理员接口：

- `GET /api/connected-apps/requests`
  - 分页查看全部应用接入申请，可用 `status=pending|approved|rejected` 筛选。
- `POST /api/connected-apps/requests/:id/review`
  - `decision=approved` 时创建 enabled/trusted connected app，并把 request 更新为 approved。
  - `decision=rejected` 时只更新 request 审批状态和备注。
  - 审批动作在同一事务中写入 `connected_app_audit_logs`。
- `GET /api/connected-apps/audit-logs`
  - 查看 connected app 审计事件，支持按 `action`、`target_type`、`target_id`、`actor_user_id`、`request_id` 过滤。

站内通知接口：

- `GET /api/notifications/connected-app-requests`
  - 管理员可看到 pending 申请通知。
  - 申请人可看到 approved/rejected 决策通知。
  - 支持 `page`、`page_size`、`unread_only`，返回 `unread_count` 和 `has_more`。
- `POST /api/notifications/connected-app-requests/read`
  - 请求体使用 `connected_app_request_keys` 标记通知已读。

通知 key 约定：

- 待审批：`connected_app_request:pending:{request_id}`
- 决策：`connected_app_request:{approved|rejected}:{request_id}:{audit_log_id}`

前端通知中心：

- 通知弹层的 `Approvals` 页签会合并企业额度审批和 connected app 审批通知。
- 管理员点击 connected app pending 通知会进入 `/system-settings/operations/connected-apps?tab=requests&connected_app_request_id=...`。
- 管理员可从通知进入 `Audit` 页签查看对应 request 的审计事件。
- 申请人看到批准/拒绝通知后可进入 Profile 查看自身连接状态；后续 `SNAPLESS-007` 会补应用开发者 API 和更完整的开发者视图。

## 可操作状态

Snapless 响应里的 `actions` 采用统一结构：

```json
{
  "severity": "warning",
  "reason": "Your account balance is too low for Snapless requests.",
  "primary": {
    "label": "Recharge",
    "href": "/wallet?source=snapless",
    "intent": "recharge"
  }
}
```

当前最小闭环覆盖：

- `quota_insufficient`：提示充值并跳转钱包。
- `user_disabled`：提示账号不可用并跳转 Profile。
- `models_unavailable`：提示 Snapless 模型不可用，并跳转模型设置/模型目录。
- `token_disabled`、`token_expired`、`grant_revoked`、`binding_revoked`：提示重新授权、轮换或从桌面端发起新的授权流程。

浏览器授权页使用 `readiness.status` 控制是否允许点击 Approve。`pending/authorized/denied/expired/consumed` 仍然只表示 Device Code Flow 自身状态。

## Token 语义

Snapless token 仍然是 new-api 原生 `tokens`：

- 默认 `unlimited_quota=true`。
- 默认 `quota_hard_limit_enabled=false`。
- 启用 `model_limits_enabled=true`。
- 默认模型限制为 `snapless-asr,snapless-polish,snapless-translate,snapless-qa`。

扣费继续走现有 relay 用户余额和模型计费体系。MCP 计费逻辑未改，仍按工具调用次数和 `price_per_call` 语义处理。

## 模型别名

默认配置项：

```json
{
  "asr": "snapless-asr",
  "chat": "snapless-polish",
  "polish": "snapless-polish",
  "translate": "snapless-translate",
  "qa": "snapless-qa"
}
```

服务端读取 `SnaplessModels` option，支持 JSON 或逗号分隔形式覆盖。health 会检查别名是否存在 enabled ability 且关联 channel 处于 enabled 状态。

## 后续顺序

1. 应用开发者 API：获批应用可以创建自己的 device sessions、查看授权状态并查询允许的 API endpoints。
2. 应用级权限边界和开发者视图：只暴露与自身 app 相关的 grant/binding/session，并补充自助排障状态。
3. 邮件/Webhook 通知扩展：在站内通知和审计可见后再扩展外部通知渠道。
