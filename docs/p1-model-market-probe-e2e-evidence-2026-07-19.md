# P1-2 模型广场复制配置 + 测通 生产 E2E 证据

日期：2026-07-19  
生产：`https://dp.app.mbu.ltd`（`sha-5f695ffe`）  
密钥：临时 API Key / access_token 仅 `/tmp`，**未入库**

## 结论

**PASS**：公开 pricing 可选模型 → 复制 base_url / 模型名 / curl 片段 → 用用户 Key 完成一次 chat 测通；并额外跑通 Playground provider-check 自检。

## 步骤与证据

### 1. 模型广场公开数据

- `GET /api/pricing` → HTTP 200，**19** 个模型
- 选定：`gpt-5.4-mini`
  - `enable_groups`：`GPT低价` / `GPT免费` / `GPT稳定高速`
  - `supported_endpoint_types`：`openai`

### 2. 可复制配置（与 UI 同源字段）

与 pricing 详情页「Copy model name / Copy Base URL / cURL 示例」一致：

| 字段 | 值 |
| --- | --- |
| Base URL | `https://dp.app.mbu.ltd` |
| OpenAI base_url | `https://dp.app.mbu.ltd/v1` |
| model | `gpt-5.4-mini` |
| Playground | `/playground?model=gpt-5.4-mini` |

cURL 示例（Key 占位，不写真实密钥）：

```bash
curl https://dp.app.mbu.ltd/v1/chat/completions \
  -H "Authorization: Bearer $DATA_PROXY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
     "model": "gpt-5.4-mini",
     "messages": [
       {"role": "user", "content": "ping"}
     ]
   }'
```

Python 示例（与 `model-details-api.tsx` 结构一致）：

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://dp.app.mbu.ltd/v1",
    api_key="<YOUR_API_KEY>",
)

completion = client.chat.completions.create(
    model="gpt-5.4-mini",
    messages=[
        {"role": "user", "content": "ping"}
    ],
)

print(completion.choices[0].message.content)
```

### 3. 测通（等价「Test in Playground」数据面）

- 临时 API Key token id=41（后禁用）
- 为命中现网渠道分组，调用期间 user group 临时切到 `GPT低价`，结束后恢复 `default`
- `POST /v1/chat/completions` model=`gpt-5.4-mini`，body 与复制的 curl 同形

| 项 | 结果 |
| --- | --- |
| HTTP | 200 |
| request id | `202607182310338934997778268d9d6PBr961DX` |
| 响应 | 非空 assistant content（测通成功） |

### 4. Playground provider-check（登录测通 API）

- `POST /api/playground/provider-check`
  - `base_url=https://dp.app.mbu.ltd/v1/chat/completions`
  - model=`gpt-5.4-mini`
  - prompt=`Reply exactly with OK.`
- 结果：`success=true`，`data.ok=true`，`status_code=200`，`output_preview=OK`，`duration_ms≈4836`

## 安全清理

- 临时 token id=41 已 `status=2`
- user group 已恢复 `default`
- access_token 已清空
- 明文 Key 未入库

## 与退出标准对应

- [x] 模型广场可复制配置并完成一次测通  
  - 复制：pricing 公开字段 + UI 同源 curl/python  
  - 测通：chat 200 + provider-check OK  
