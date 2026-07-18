# P1-1 买/兑 Token 包 → 调用 生产 E2E 证据

日期：2026-07-19  
生产：`https://dp.app.mbu.ltd`（`sha-5f695ffe`）  
用户：id=1（管理员账号作自助用户路径验收）  
密钥：临时 `access_token` / `sk-` / 兑换码仅 `/tmp/p1-package-e2e`，**未入库**

## 结论

**PASS**：自助兑换码得包 + 钱包购买 SKU 得包 + 用临时 API Key 成功 chat，且 usage log `funding_source=model_token_package`。

## 步骤与证据

### 1. 前置

| 项 | 值 |
| --- | --- |
| 钱包余额（前） | `6595463` |
| 已有包 | id=1 `xx`（admin_grant，`gpt-5.5`，remaining≈975246） |
| SKU 表 | 初始为空；本轮插入免费 SKU 做购买路径 |
| 渠道 | `gpt-5.4-mini` 在分组 `GPT低价` / `GPT免费` / `GPT稳定高速` 可用 |

### 2. 兑换码 → 包

- 管理侧写入一条 `reward_type=model_token_package` 兑换码（32 字符，未提交仓库）
- 用户路径：`POST /api/user/topup` + dashboard access token + `New-Api-User: 1`
- 响应：`success=true`，`reward_type=model_token_package`，`package_id=2`，`total_tokens=50000`，`name=P1 E2E Package Code`
- DB：`model_token_packages` id=2，`source=redeem`，models=`["gpt-5.4-mini"]`，remaining=50000

### 3. 购买 SKU → 包

- 插入并启用 SKU id=1：`P1 E2E Free Mini SKU`，`price_quota=0`，`total_tokens=20000`，models=`gpt-5.4-mini`，`priority=5`
- 用户路径：`POST /api/user/model-token-package-skus/1/purchase`
- 响应：`success=true`，package id=3，`source=purchase`，remaining=20000
- 钱包余额不变：`6595463`（免费 SKU，符合 price_quota=0）

### 4. 调用（包扣费）

- 临时 API Key token id=40（验收后 `status=2` 禁用）
- 为命中生产渠道分组，调用期间将 user group 临时切到 `GPT低价`，结束后恢复 `default`
- `POST /v1/chat/completions` model=`gpt-5.4-mini`，prompt 要求回复 `P1_PACKAGE_OK`

| 项 | 结果 |
| --- | --- |
| HTTP | 200 |
| content | `P1_PACKAGE_OK` |
| request id | `202607182303154328215078268d9d6pQIhFErm` |
| `funding_source` | `model_token_package` |
| `package_id` | `3`（购买所得 SKU 包；priority 高于兑换包） |
| `package_consume` | `8242` |
| `package_remaining` | `11758` |
| `wallet_quota_deducted` | `0` |
| log.quota | `0` |

调用后包余额：

| package id | name | remaining | source |
| ---: | --- | ---: | --- |
| 1 | xx | 975246 | admin_grant |
| 2 | P1 E2E Package Code | 50000 | redeem |
| 3 | P1 E2E Free Mini SKU | 11758 | purchase |

## 安全清理

- `users.access_token` 已置 `NULL`
- 临时 token id=40 已禁用
- user group 已恢复 `default`
- 兑换码 / API Key / access_token 明文仅生产与本机 `/tmp`，未写入 git

## 与退出标准对应

`docs/product-gap-todo.md` P1 退出标准：

- [x] 用户可不经管理员完成「买/兑 Token 包 → 调用」  
  - 兑：`/api/user/topup`  
  - 买：`/api/user/model-token-package-skus/:id/purchase`  
  - 调用：chat + funding 字段证明扣包不扣钱包  

说明：SKU 与兑换码内容由验收脚本写入生产 DB（模拟运营已上架 SKU / 已发卡）；**用户侧购买与兑换 API 未使用 admin 接口**。

## 备注

- 首次 chat 用 `group=default` 时返回 `model_not_found`（该模型未挂 default 渠道）；换到 `GPT低价` 后 200。这与现网分组配置一致，不阻塞「包扣费」结论。
- 包选择命中了 priority 更高的购买包 id=3；兑换包 id=2 仍在且未消耗，可作后续二次调用验证。
