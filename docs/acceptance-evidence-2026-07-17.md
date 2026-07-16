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

