# Snapless Connected App Integration

本文档记录 Data Proxy 对 Snapless Desktop 的最小接入闭环。Data Proxy 基于 new-api，继续保留 AGPLv3、NOTICE 和上游归属要求。

## V1.3 已实现

本阶段不把 Snapless 写成一次性特殊逻辑，而是新增最小 Connected App 内核：

- `connected_apps`：登记可连接应用，内置 trusted app `snapless`。
- `connected_app_grants`：记录用户授权和 scopes。
- `connected_app_token_bindings`：记录 app、用户、设备和原生 Token 的绑定。

内置 Snapless scopes：

- `openai.models`
- `openai.chat`
- `openai.audio.transcriptions`
- `quota.read`
- `token.manage`

## Snapless API

登录用户接口：

- `GET /api/snapless/config`
- `POST /api/snapless/tokens/ensure`
- `POST /api/snapless/tokens/rotate`
- `DELETE /api/snapless/tokens/current`

客户端健康检查：

- `GET /api/snapless/health`

`ensure` 会为同一用户、同一设备复用已有 active binding。只有首次创建或 rotate 时返回 `api_key` 明文；复用已有 binding 时只返回 token 摘要。

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

1. Device Code Flow：`device/start`、授权页、`device/poll`，保证 API key 不进入 URL。
2. 用户控制台 Connected App 卡片：展示 Snapless 连接、设备、状态、轮换和撤销入口。
3. 状态与充值闭环：health/config 对接前端充值入口和余额不足提示。
4. 多应用扩展：把内置 Snapless 方案抽象成可配置 Connected App 管理能力。
