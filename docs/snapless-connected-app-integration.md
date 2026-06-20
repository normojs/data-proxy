# Snapless Connected App Integration

本文档记录 Data Proxy 对 Snapless Desktop 的最小接入闭环。Data Proxy 基于 new-api，继续保留 AGPLv3、NOTICE 和上游归属要求。

## V1.3 已实现

本阶段不把 Snapless 写成一次性特殊逻辑，而是新增最小 Connected App 内核：

- `connected_apps`：登记可连接应用，内置 trusted app `snapless`。
- `connected_app_grants`：记录用户授权和 scopes。
- `connected_app_token_bindings`：记录 app、用户、设备和原生 Token 的绑定。
- `connected_app_device_sessions`：记录 Device Code Flow 的 device code、user code、设备信息、授权状态和一次性消费状态。

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
- `POST /api/snapless/tokens/ensure`
- `POST /api/snapless/tokens/rotate`
- `DELETE /api/snapless/tokens/current`

客户端健康检查：

- `GET /api/snapless/health`

`ensure` 会为同一用户、同一设备复用已有 active binding。只有首次创建或 rotate 时返回 `api_key` 明文；复用已有 binding 时只返回 token 摘要。Device Code Flow 不把 API key 放入 URL，只允许桌面端凭 `device_code` 轮询一次获取。

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

1. 用户控制台 Connected App 卡片：展示 Snapless 连接、设备、状态、轮换和撤销入口。
2. 状态与充值闭环：health/config 对接前端充值入口和余额不足提示。
3. 应用管理：把内置 Snapless 方案抽象成可配置 Connected App，支持应用申请权限、管理员审核和按 scopes 发放能力。
4. 多应用扩展：复用 Connected App grant、token binding 和 Device Code Flow，支持 Snapless 以外的可信或第三方应用。
