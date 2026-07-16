# 额度总览（Quota Overview）

API：`GET /api/user/quota-overview`（需登录，仅当前用户）

## 设计目标

一页说清四类 funding，**单位不互相覆盖**：

| 区块 | `units` 值 | 含义 |
|------|------------|------|
| wallet | `quota_points` | 钱包余额（系统额度点 / 金额点） |
| model_token_packages | `llm_tokens` | 模型 Token 包剩余（LLM tokens） |
| subscriptions | `quota_points` | 订阅计划剩余额度点 |
| api_key_hard_limits | `quota_points` | 带硬限的 API Key 剩余额度点 |

前端展示时必须分别标注单位，禁止把四类加总成一个数字。

## 响应字段摘要

```json
{
  "wallet": {
    "quota": 0,
    "used_quota": 0,
    "request_count": 0,
    "unit": "quota_points",
    "status": "ok|empty"
  },
  "model_token_packages": {
    "active_count": 0,
    "remaining_tokens": 0,
    "used_tokens": 0,
    "total_packages": 0,
    "unit": "llm_tokens",
    "status": "ok|empty",
    "top_packages": []
  },
  "subscriptions": {
    "active_count": 0,
    "remaining_quota": 0,
    "total_quota": 0,
    "used_quota": 0,
    "unit": "quota_points",
    "status": "ok|empty",
    "items": []
  },
  "api_key_hard_limits": {
    "limited_count": 0,
    "remaining_quota": 0,
    "unit": "quota_points",
    "status": "ok|empty",
    "items": []
  },
  "units": { "...": "..." },
  "links": {
    "wallet": "/wallet",
    "model_token_packages": "/wallet#model-token-packages",
    "subscriptions": "/wallet#subscriptions",
    "api_keys": "/keys"
  }
}
```

## 空态

无包 / 无订阅 / 无硬限 Key 时，对应区块 `status=empty`，计数为 0，列表为空数组，不报错。

## UI

- `/wallet` 顶部：`QuotaOverviewCard`
- `/profile`：`QuotaOverviewCard compact`（隐藏 top packages 明细条）
