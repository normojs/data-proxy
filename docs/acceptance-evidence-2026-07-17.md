# 生产验收证据 2026-07-17

版本：`sha-03f66c5c`（`https://dp.app.mbu.ltd`）  
方式：公开 HTTP 探针 + 源码/路由对照；登录态/真实扣费路径未持有生产 API Key，单独标注。

## 环境

| 项 | 值 |
| --- | --- |
| Public version header | `x-new-api-version: sha-03f66c5c` |
| `/api/status` | `success=true`，`data.version=sha-03f66c5c` |
| Production smoke | `api_status=passed`；chat/responses/admin diagnostic 因无 Key 跳过 |

## P0 验收

### 代码与可达性（已通过）

| 检查 | 结果 |
| --- | --- |
| `/docs/user-quickstart.md` 可访问且含注册/钱包/Key/curl | PASS（HTTP 200，含 `/wallet` `/keys` `chat/completions`） |
| 站内入口存在（Keys/Dashboard/QuotaOverview 链到 quickstart） | PASS（源码对照） |
| 额度总览 API 存在且未登录 401 | PASS（`GET /api/user/quota-overview` → 401） |
| 额度总览前端卡片存在（wallet + profile） | PASS（源码对照） |
| 扣费解释 UI（Funding Explanation） | PASS（源码对照，usage log 详情） |
| 错误人话化 playground 计费错误引导 | PASS（源码对照） |
| `/v1/models` 无 Key 拒绝 | PASS（401 Invalid token） |

### 需登录/API Key 的退出标准（未完整勾）

| 退出标准 | 状态 | 说明 |
| --- | --- | --- |
| 新用户按文档 3 分钟完成一次成功请求 | PARTIAL | 文档与公开路径齐；未持有生产 `sk-` 完成 chat smoke |
| 任意成功/失败请求能在 UI 解释扣费或拒绝原因 | PARTIAL | UI/后端字段齐；需登录打开 usage log 详情肉眼确认 |
| 额度总览四类资产不互相混淆 | PARTIAL | 卡片与 API 齐；需登录打开 `/wallet` 肉眼确认空态/单位 |

## P1 验收

### 已通过（公开/源码）

| 检查 | 结果 |
| --- | --- |
| SKU 列表/购买 API 未登录 401 | PASS |
| 管理端 `/package-skus` 路由与页面 | PASS（SPA 200 + 源码） |
| 用户钱包购买入口 | PASS（`ModelTokenPackagesCard` + purchase API） |
| 模型广场/pricing 公开数据 | PASS（`/api/pricing` 返回 19 个模型） |
| 上游重试友好信息 | PASS（`user_retry_summary` 写入 + UI） |
| 包低余额通知 | PASS（源码 `NotifyTypeModelTokenPackageLow`） |
| 一键部署文档仓库存在 | PASS（`docs/one-click-deploy.md`） |

### 缺口

| 项 | 状态 | 说明 |
| --- | --- | --- |
| 用户自助买/兑包→调用 | PARTIAL | 能力齐；缺真实账号走通 |
| 模型广场测通 | PARTIAL | pricing 有数据；测通跳转 playground 需登录 |
| 干净机器 compose 部署复验 | NOT RUN | 未另起空机器 |
| 坏渠道自动避开生产演练 | NOT RUN | 需 admin + 双渠道演练 |
| 生产侧栏默认配置含 `package_skus` 模块开关 | NOTE | 线上 `SidebarModulesAdmin.admin` 尚无 `package_skus` 键；路由仍可直达 `/package-skus`，侧栏是否显示取决于默认 merge（前端默认配置含该键） |
| `/docs/one-click-deploy.md` 公网直链 | FAIL/NOTE | 公网返回 SPA shell（1026B），未像 user-quickstart 那样放入 `web/default/public/docs/` |

## 结论

- **可对外宣称**：`sha-03f66c5c` 已部署；公开健康检查通过；用户接入文档可达；P0/P1 相关 API/页面代码已上线且鉴权边界正确。
- **不可勾满退出标准的原因**：缺少生产登录态与 API Key，无法完成“真实成功请求 + UI 肉眼验收”。
- **建议下一步**：提供临时 `DATA_PROXY_API_KEY`（及可选管理员 cookie/token）后补：
  1. chat completions smoke + request id
  2. `/wallet` 额度总览截图/字段核对
  3. usage log Funding Explanation 核对
  4. 兑码或买 SKU → 再请求一次

## 命令摘要

```bash
curl -sI https://dp.app.mbu.ltd/ | grep -i x-new-api-version
curl -fsS https://dp.app.mbu.ltd/api/status | jq .success
curl -fsS https://dp.app.mbu.ltd/docs/user-quickstart.md | head
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd scripts/data-proxy-production-smoke.sh
```

## 跟随修复（待下次部署生效）

已将下列文档复制到 `web/default/public/docs/`，下次镜像部署后公网可直接访问：

- `/docs/one-click-deploy.md`
- `/docs/quota-overview.md`
- `/docs/user-quickstart.md`（刷新）

当前生产仅确认 `user-quickstart.md` 已可访问；one-click/quota-overview 仍返回 SPA shell，直至重新部署。

## 2026-07-17 续：部署 `sha-da5af9b2` 并复验

- 通过本地 VPN `127.0.0.1:7897` 下载 Package 产物 `data-proxy-da5af9b2-linux-amd64`
- Electerm MCP 上传并执行 `data-proxy-remote-deploy-da5af9b2.sh`
- 生产版本：`x-new-api-version: sha-da5af9b2`
- 文档公网：
  - `/docs/user-quickstart.md` 3988B PASS
  - `/docs/one-click-deploy.md` 2498B PASS（此前 SPA 壳问题已随本版修复）
  - `/docs/quota-overview.md` 1808B PASS
- 公开/鉴权探针 ALL_PASS
- production smoke：`api_status=passed`；chat/admin 仍缺 Key 跳过

## 2026-07-17 续：临时 API Key 联调验收（密钥未入库）

版本：`sha-da5af9b2`  
模型：`gpt-5.4-mini`  
说明：使用用户提供的临时 Key 做请求面验收；**密钥不写入仓库**。

### production smoke

| 项 | 结果 |
| --- | --- |
| api_status | passed |
| chat_completions | passed |
| chat_request_id | `202607161756103469800368268d9d6bGXfIAcT` |
| responses | passed |
| responses_request_id | `20260716175615583078728268d9d6wljEfdHV` |
| admin diagnostic/trace | skipped（无管理员认证） |

### 附加探针

| 检查 | HTTP | 结果 | 备注 |
| --- | ---: | --- | --- |
| GET /v1/models | 200 | PASS | 5 models |
| POST /v1/chat/completions 成功 | 200 | PASS | request_id `202607161757115647416548268d9d6ynCgaECE`，返回非空 content |
| POST 无效模型 | 503 | PASS | `model_not_found` / no available channel，含 request id |
| 用 API Key 访问 /api/user/* session 接口 | 200 body success=false | PASS | 返回 invalid access token（Key 不能冒充 session） |

### 对 P0 退出标准的更新

| 退出标准 | 状态 |
| --- | --- |
| 新用户按文档 3 分钟完成一次成功请求 | **PASS（请求面）**：文档可达 + 用 Key 完成 models/chat/responses |
| 任意成功/失败请求能在 UI 解释扣费或拒绝原因 | **PARTIAL**：成功/失败请求均有 request id 与可读 error code；usage log UI 仍需登录 session 手测 |
| 额度总览四类资产不互相混淆 | **PARTIAL**：API/前端已上线；session 手测仍缺 |

### 安全提醒

请尽快在控制台轮换/作废本次临时 Key。

## 2026-07-17 续：错误路径 + 扣费解释字段验收

版本：`sha-da5af9b2`  
鉴权：临时 API Key（未入库）

### 错误路径

| 场景 | HTTP | 结果 | request id / 说明 |
| --- | ---: | --- | --- |
| messages 为空 | 500 | PASS | `202607161803212422585488268d9d6Qa24Vc1R`，`field messages is required` |
| model 为空 | 400 | PASS | `202607161803213949652098268d9d67j17HDcL`，model name cannot be empty |
| 无效 API Key | 401 | PASS | `202607161803215013376338268d9d6NyjYYaVi`，Invalid token |
| 无效模型 | 503 | PASS | 已有 `model_not_found` 证据 |
| 成功 chat | 200 | PASS | `202607161803221093046748268d9d6cFAhoH2p`，content=`验收通过` |

### 扣费解释（API 层，`GET /api/log/token`）

用 Key 可拉取该 token 的用量日志；成功请求已写入 funding 元数据：

| request_id | path | funding_source | wallet_quota_deducted | package |
| --- | --- | --- | ---: | --- |
| `202607161803221093046748268d9d6cFAhoH2p` | `/v1/chat/completions` | wallet | 71 | 无 |
| `2026071618032658360238268d9d6VvUjpAF9` | `/v1/responses` | wallet | 104 | 无 |
| `202607161756103469800368268d9d6bGXfIAcT` | `/v1/chat/completions` | wallet | 71 | 无 |
| `20260716175615583078728268d9d6wljEfdHV` | `/v1/responses` | wallet | 104 | 无 |

附加观察：

- `admin_info` 未出现在 token log 的 other 中（用户侧不泄露渠道细节）
- `user_retry_summary` 本次均为空（未触发 failover，符合预期）
- session 接口 `/api/user/quota-overview` 仍拒绝纯 API Key（需登录 session）

### 对退出标准

| 项 | 状态 |
| --- | --- |
| 3 分钟成功请求 | PASS（请求面） |
| 扣费/拒绝可解释 | **API 层 PASS**：成功请求有 `funding_source`/`wallet_quota_deducted`；失败请求有 code+message+request id。前端详情弹窗需登录 UI 手测一次即可闭环 |
| 额度总览四类资产 | **PASS（用户手测）**：2026-07-17 登录确认 `/wallet` 额度总览正确 |

## 2026-07-17 续：回归探针 + 包/钱包分流对照

版本：`sha-da5af9b2`  
鉴权：临时 API Key（未入库）

### 公开面

| 检查 | 结果 |
| --- | --- |
| `x-new-api-version` | `sha-da5af9b2` |
| `/docs/user-quickstart.md` | 200 / 3988B |
| `/docs/one-click-deploy.md` | 200 / 2498B |
| `/docs/quota-overview.md` | 200 / 1808B |
| `/api/pricing` | 200，19 models |
| `/package-skus` SPA | 200（需登录后使用） |
| Key 可见模型 | 5：`codex-auto-review` / `gpt-5.4` / `gpt-5.4-mini` / `gpt-5.5` / `gpt-5.6-terra` |

### 成功请求（扣费分流）

| 场景 | HTTP | request id | funding | 扣减 |
| --- | ---: | --- | --- | --- |
| chat `gpt-5.4-mini` | 200 | `202607161838156213954278268d9d69CsaR3HC` | wallet | `wallet_quota_deducted=71`，content=`包测OK` |
| responses `gpt-5.4-mini` | 200 | `202607161838222024205098268d9d6uwP8hEjm` | wallet | `wallet_quota_deducted=104`，text=`respOK` |
| chat `gpt-5.4` | 200 | `202607161838596392437008268d9d6W9WtTh4A` | wallet | `wallet_quota_deducted=72` |
| chat `gpt-5.5` | 200 | `202607161838477423841448268d9d6X0V7lded` | **model_token_package** | `package_id=1`，`package_consume=8237`，`package_remaining=975246`，`wallet=0`，`quota=0` |

结论：包覆盖模型走包、未覆盖模型走钱包；API 层 funding 字段一致。

### 错误路径

| 场景 | HTTP | request id / 说明 |
| --- | ---: | --- |
| 无效模型 | 503 | `202607161838191157373218268d9d63MrDqM0z`，`model_not_found` |
| messages 为空 | 500 | `202607161838201314982988268d9d6peC0Lkts`，`field messages is required` |
| model 为空 | 400 | `202607161839453151794288268d9d6Q5GFFBhm` |
| 无效 Key | 401 | `202607161839445770687708268d9d69h2njic8`，Invalid token |

### 鉴权边界

用 API Key 访问 session 接口均拒绝（body `success=false` / invalid access token）：

- `/api/user/quota-overview`
- `/api/user/model-token-package-skus`
- `/api/user/model-token-packages`
- `/api/user/self`
- `/api/user/model-token-package-skus/all`

### 备注

- 历史 token log 中有若干 `gpt-5.5` 记录 `package_consume=0` 且 `package_remaining=1`（rid 如 `…Mx4AvQs0` / `…YXOFlzi5` / `…27snV0uv`），疑似结算边界或缓存命中边角；后续正常请求（`…s7X7FHwF` / `…X0V7lded`）consume 恢复正常。可单独追代码，不阻塞主路径 PASS。
- `/wallet` 额度总览：用户手测 PASS（2026-07-17）。
- 临时 API Key：用户已删除。
- 仍可选：usage log Funding Explanation UI 肉眼再确认一次（API 层字段已 PASS）。

## 2026-07-17 续：部署 `sha-dcc267e8`

- 提交：`dcc267e8` 已 push 到 `normojs/data-proxy` main
- Package CI：run `29534853362` success
- Electerm 上传并执行 `data-proxy-remote-deploy-dcc267e8.sh`
- 生产版本：`x-new-api-version: sha-dcc267e8`
- 本地 health ok；文档三份 200
- 包含：渠道 Key 更新后清熔断；隧道 UI HTTPS/本机协议；P2 e2e 文档

## 2026-07-19：部署 `sha-07847f30`（IdP 加固 + agent install shell）

- 提交：`3a2fa717`（IdP review 债/注册归因）+ `07847f30`（`/agent/install.sh` 不再 SPA）
- Package CI：run `29657633122` success → `data-proxy-07847f30-linux-amd64.tar.gz`
- Electerm 主机：`47.122.29.88`（snsc-prod-应用2），目录 `/root/workspace/dataproxy/data-proxy`
- 执行：`./data-proxy-remote-deploy-07847f30.sh`
- 生产版本：
  - `x-new-api-version: sha-07847f30`
  - `/api/status` → `success=true`，`version=sha-07847f30`
- 安装脚本验收：
  - `GET /agent/install.sh` → `content-type: text/x-shellscript`，body 以 `#!/usr/bin/env sh` 开头，**非** SPA HTML（530B bootstrap）
  - `GET /agent/install-data-proxy-agent.sh` 同上
- 旧镜像归档：`image-archive/20260718T195728Z_data-proxy_dcc267e8.tar`

