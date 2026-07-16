# 3 分钟接入 Data Proxy

面向终端用户：注册后创建 Key，用 OpenAI 兼容协议发出第一次成功请求。

## 1. 注册 / 登录

1. 打开站点首页，注册账号并登录。
2. 登录后进入控制台（Dashboard）。

## 2. 准备额度

任选其一即可发请求：

| 方式 | 入口 | 单位 |
|------|------|------|
| 钱包充值 | `/wallet` | 额度点（quota points） |
| 模型 Token 包 | 管理员发放 / 钱包页「模型 Token 包」 | LLM tokens |
| 订阅计划 | `/wallet` 订阅区 | 额度点 |

说明：

- **Token 包优先**：若当前模型有可用包，请求扣包 tokens，不扣钱包。
- **包不足不会自动扣钱包**；需换模型、换包或改用钱包/订阅。
- 额度总览：`/wallet` 顶部可同时看到钱包、包、订阅、Key 硬限。

## 3. 创建 API Key

1. 打开 `/keys`。
2. 创建 Key，复制完整 `sk-...`（只显示一次，请妥善保存）。
3. 页面顶部 **Base URL** 一般为：

```text
https://你的域名/v1
```

可直接复制；客户端里填 Base URL 时通常要带 `/v1`。

## 4. 第一次请求（curl）

把 `BASE` 和 `KEY` 换成你的值；`model` 换成你账号可用的模型名（可在 Playground 或定价页查看）。

```bash
export BASE="https://你的域名/v1"
export KEY="sk-你的密钥"

curl "$BASE/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $KEY" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "用一句话打个招呼"}
    ]
  }'
```

也支持：

```bash
# 列出可用模型
curl "$BASE/models" -H "Authorization: Bearer $KEY"
```

## 5. Python（OpenAI SDK）

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://你的域名/v1",
    api_key="sk-你的密钥",
)

resp = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "用一句话打个招呼"}],
)
print(resp.choices[0].message.content)
```

## 6. 常见客户端（Cursor / OpenAI 兼容）

| 配置项 | 值 |
|--------|-----|
| Base URL / API Base | `https://你的域名/v1` |
| API Key | `sk-...` |
| Model | 控制台可用模型名 |

注意：

- 多数客户端要求 Base URL **包含** `/v1`。
- 不要把 Key 提交到公开仓库或发给他人。

## 7. 协议边界（请勿夸大兼容）

| 能力 | 说明 |
|------|------|
| `/v1/chat/completions` | 主路径，OpenAI 兼容 Chat |
| `/v1/responses` | 部分渠道原生支持；Chat-only 上游会做兼容转换 |
| Hosted tools | 如 `web_search`、`file_search`、`code_interpreter`、hosted `mcp` 等，在 Chat 兼容路径上**可能被过滤**，不保证上游真正执行 |
| Function / custom tools | 一般可按 OpenAI function calling 使用 |

更细的渠道协议说明见运营文档，不在本文展开。

## 8. 错误排查速查

| 现象 | 常见原因 | 你可以做什么 |
|------|----------|--------------|
| `401` | Key 错误 / 未带 `Authorization: Bearer` | 重新复制 Key；检查 Header |
| `403` 余额不足 | 钱包额度不够 | 去 `/wallet` 充值 |
| `403` 包不足 | 当前模型包已用尽且无钱包兜底 | 换模型 / 联系管理员发包 / 改用钱包 |
| `403` Key 硬限 | API Key 设了剩余额度上限 | 在 `/keys` 提高限额或新建 Key |
| `403` 企业拒绝 / 排队 | 企业策略拦截 | 联系企业管理员或查看通知 |
| 模型不存在 / 不可用 | 账号无该模型或渠道下线 | 在 Playground 换可用模型 |
| 超时 / 5xx | 上游或网络抖动 | 重试；持续失败联系管理员 |

成功请求后，可在 **用量日志** 查看「扣费解释」：钱包扣了多少、是否走了 Token 包。

## 9. 推荐路径（控制台）

1. Dashboard 设置向导：创建 Key → 充值/获包 → Playground 试跑  
2. `/keys` 复制 Base URL  
3. 用 curl / Python / Cursor 正式接入  

文档入口（站内静态）：`/docs/user-quickstart.md`
