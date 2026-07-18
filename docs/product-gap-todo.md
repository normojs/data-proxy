# Data Proxy 产品差距与功能规划（对标开源中转站）

日期：2026-07-16  
状态：P0/P1 功能已上线；退出标准部分验收（见 docs/acceptance-evidence-2026-07-17.md）  
代码目录：`upstream/new-api`  
关联：`docs/model-token-package-plan.md`、`docs/data-proxy-near-term-development-plan.md`、`docs/data-proxy-post-v1.3-todo.md`

## 背景

基于 new-api 的 Data Proxy 在企业治理、诊断、MCP、支付、模型 Token 包等方面已强于多数「轻量中转站」，但在**用户主路径、额度解释、开箱交付**上仍偏后台控制台。

本文按 P0 / P1 / P2 列出待做功能；未完成项统一用 `- [ ]`。

## 总原则

- 先做「说得清、接得上、报错懂」再做「买得爽、装得快、运维傻」。
- 不把企业/MCP 能力默认塞进个人用户主路径。
- 每次交付保持 AGPLv3、`NOTICE`、上游 attribution 可见。
- 功能线独立提交，不混入无关改动。

---

## P0 — 立刻影响口碑

目标：个人用户 3 分钟能用、知道扣了什么、403 看得懂。

### P0-1 额度总览（钱包 + Token 包 + 订阅 + Key 硬限）

- [x] 定义「额度总览」统一数据模型（各 funding 的单位、剩余、状态、跳转）
  - 验收：文档写清 wallet / model_token_package / subscription / token_hard_limit 字段与单位，不互相覆盖
  - 文档：`docs/quota-overview.md`
- [x] 后端汇总 API：`GET /api/user/quota-overview`（或等价）
  - 验收：返回钱包余额、Token 包摘要（活跃数/剩余 tokens）、订阅剩余、API Key 硬限列表摘要；仅当前用户
- [x] 前端「额度总览」页或卡片（建议挂 `/wallet` 顶部 + `/profile` 可见）
  - 验收：一页看清四类额度；单位标签明确（金额点 vs LLM tokens）；点进各明细
- [x] 无包 / 无订阅 / 无硬限时展示空态，不报错
  - 验收：纯钱包用户界面干净，不出现误导数字
- [x] i18n（至少 en/zh）与 typecheck
  - 验收：`tsc` 通过；关键文案已翻译

### P0-2 请求扣费解释（这次扣了谁、多少、为什么）

- [x] 统一请求级 funding 元数据写入 usage log / other（已有 package 字段则补齐 wallet/subscription）
  - 验收：consume 日志可区分 `funding_source` 与关键扣减量
- [x] 后端「请求扣费解释」接口或 usage log 详情扩展
  - 验收：按 request_id 返回：模型、渠道（可脱敏）、funding_source、扣减明细、失败原因（若有）
  - 说明：复用 usage log `other` + 既有 request trace；统一写入 `funding_source` / `wallet_quota_deducted` / package 字段
- [x] 用量日志 / 请求详情 UI 展示「扣费解释」区块
  - 验收：用户能看到：扣钱包多少 / 扣哪几个包多少 tokens / 是否命中 Key 硬限；包路径不显示误导的金额扣费
- [x] 覆盖文本主路径与常见错误路径（预扣失败、包不足、企业拒绝）
  - 验收：至少 chat/completions 与包结算路径有用例或手动验收记录
  - 说明：成功路径覆盖 text/audio/realtime；预扣失败仍不写 log（保持 NoRecordErrorLog），由 P0-4 人话错误补齐
- [x] i18n 与 typecheck

### P0-3 用户接入文档（3 分钟跑通）

- [x] 撰写《3 分钟接入》文档（独立 md，面向终端用户）
  - 验收：包含注册/登录 → 充值或获包 → 创建 Key → base_url + 示例 curl/Python → 常见客户端（Cursor/ChatGPT 兼容）配置
  - 文档：`docs/user-quickstart.md`（站内静态 `/docs/user-quickstart.md`）
- [x] 写清当前协议边界（原生 Responses vs Chat-only、不支持的 hosted tools）
  - 验收：不夸大兼容范围
- [x] 文档入口挂到站内（帮助/文档链接或钱包空态 CTA）
  - 验收：登录用户 2 次点击内可达
  - Dashboard 设置向导 / Keys Base URL / 钱包额度总览
- [x] 附错误排查速查（401/403/余额不足/包不足）

### P0-4 错误文案人话化（403 / 包不足 / 企业拒绝）

- [x] 梳理高频错误码与现网 message 映射表
  - 验收：至少覆盖 `insufficient_user_quota`、`insufficient_model_token_package`、`pre_consume_token_quota_failed`、企业 governance 拒绝/排队/异常限流
- [x] 后端用户可见 message 改为稳定、可行动的中文/英文（保留 error code）
  - 验收：message 说明「发生了什么 + 下一步」（如去钱包充值 / 联系管理员发包 / 换模型）
- [x] 前端统一展示 error code + 人话 message（playground、用量日志、toast）
  - 验收：不直接甩原始内部堆栈；关键路径有 i18n
  - playground `message-error` 对三类计费错误展示 code + CTA
- [x] 补充/回归测试：关键错误码文案与 HTTP 状态不变坏
  - 验收：相关 go test 通过

### P0 退出标准

- [x] 新用户按文档 3 分钟完成一次成功请求
  - 2026-07-17：文档可达 + 临时 Key 完成 `/v1/models`、chat、responses（见 acceptance-evidence；request ids 已记录）
- [x] 任意成功/失败请求能在 UI 解释扣费或拒绝原因
  - 2026-07-17：API 层已验证成功请求 `funding_source=wallet` + `wallet_quota_deducted`，失败请求有可读 code/message/request id（`/api/log/token`）。控制台详情弹窗字段同源；登录 UI 手测可选加强
- [x] 额度总览四类资产不互相混淆
  - 2026-07-17：API + Wallet/Profile 卡片已上线；用户手测 `/wallet` 额度总览正确

---

## P1 — 追上主流商业站

目标：能自助买量、能逛模型、能一条命令部署、渠道坏了会自己躲。

### P1-1 Token 包自助购买 / 卡密

- [x] Token 包 SKU 模型与管理端配置（模型列表、token 量、三倍率、价格、有效期）
  - 验收：管理员可 CRUD SKU；默认倍率 1/1/1
  - 说明：`model_token_package_skus` + `POST/PUT /api/user/model-token-package-skus*`、`GET /api/user/model-token-package-skus/all`
- [x] 用户购买下单：复用现有支付（微信/Stripe/Epay 等之一先打通）
  - 验收：支付成功后自动创建 `model_token_packages`；失败不发包
  - 说明：MVP 用**钱包余额扣费购买 SKU**（`POST /api/user/model-token-package-skus/:id/purchase`）；在线渠道充值仍走既有 topup
- [x] 兑换码/卡密兑换 Token 包（可扩展现有 redemption）
  - 验收：兑码成功得包；重复兑换/作废码有明确错误
- [x] 购买/兑换流水与审计
  - 验收：可追溯 order/code → package_id
  - 说明：`result_package_id` + funding billing metadata `redemption_id` / `package_sku_purchase`
- [x] 用户侧购买入口（钱包或模型广场）
  - 验收：非管理员可自助完成买包或兑包
  - 说明：钱包兑换码入口 + SKU 购买列表
- [x] 即将用尽提醒（站内通知，可选 email）
  - 验收：剩余低于阈值触发一次，不刷屏
  - 说明：`NotifyTypeModelTokenPackageLow`，跨阈值触发 + notify limit

### P1-2 模型广场：测通、复制、价格、可用性

- [x] 模型广场信息架构：价格、分组倍率、展示名、可用状态、最近延迟/成功率（有则展示）
  - 验收：列表/详情字段稳定；无数据时降级不崩
  - 说明：沿用 pricing 页既有能力
- [x] 一键复制：base_url、模型名、示例 curl / OpenAI SDK snippet
  - 验收：复制内容可直接跑（占位 Key 提示用户替换）
- [x] 「测通」：登录用户用自己的 Key 或临时探测发起最小请求
  - 验收：成功/失败可读；不泄露上游密钥；有频率限制
  - 说明：跳转 Playground 预填 `?model=`（会话鉴权，无上游密钥）
- [x] 与额度总览/Token 包联动（该模型是否有包覆盖）
  - 验收：模型详情显示「可用包」或「将扣钱包」
- [x] 移动端可用性与 i18n

### P1-3 部署一键化

- [x] 收敛「最小生产 compose」：`docker-compose.prod.yml`（+ 可选 wechat override）一条命令启动说明
  - 验收：干净机器按文档可启动并 `/api/status` 成功（允许外部 MySQL/Redis 配置）
  - 文档：`docs/one-click-deploy.md`
- [x] 环境变量模板 `.env.example` 精简版（必填/可选分层）
  - 验收：必填项 ≤ 一屏；注释中文/英文清晰
  - 文件：`.env.example.minimal`
- [x] 《一键部署》用户向文档（非内部 runbook 堆砌）
  - 验收：安装 → 初始化管理员 → 配渠道 → 验证请求，步骤可勾选
- [x] 可选：安装脚本 `scripts/quickstart.sh`（拉镜像/生成 env/compose up）
  - 验收：脚本非交互或极少交互；失败有明确退出码
- [x] 发布产物与 tag 说明挂到 README 或 docs 入口

### P1-4 渠道健康与自动下线更傻瓜

- [x] 渠道健康摘要：成功率、延迟、最近错误（管理端列表可见）
  - 验收：不 N+1；大数据量可分页/缓存
  - 说明：沿用 `runtime_health` + 既有列表徽章
- [x] 自动/半自动不健康处理：阈值触发临时禁用或降权（可配置、默认可关）
  - 验收：有开关；写审计；可手动恢复
  - 说明：引擎已有；新增「安全故障切换预设」一键填参
- [x] 与现有 failover / stream_error_mapping 文档打通
  - 验收：管理员按文档 10 分钟完成「坏渠道自动切走」配置
- [x] 用户侧可选提示「上游繁忙已重试」类友好信息（不泄露内部渠道名，若产品需要）
  - 验收：文案可配置或有默认；不破坏现有客户端兼容
  - 说明：`other.user_retry_summary` 在 usage log 对用户可见；不含渠道名/id

### P1 退出标准

- [ ] 用户可不经管理员完成「买/兑 Token 包 → 调用」
  - 部分完成：兑码/SKU 购买 API+UI 已上线；缺真实账号闭环证据
- [ ] 模型广场可复制配置并完成一次测通
  - 部分完成：pricing 公开数据可用；测通需登录 Playground
- [ ] 新人按文档一条 compose 路径完成部署
  - 部分完成：仓库文档存在；公网未挂载 one-click-deploy 静态文件；未做干净机器复验
- [ ] 坏渠道在配置开启后可自动避开并有审计
  - 未跑：需生产/预发双渠道演练

---

## P2 — 拉开差距

目标：远程工具链真能用，而不只是服务端协议。

### P2-1 客户端隧道真正可用（dpa / Browser）

- [x] 冻结客户端范围：优先 `dpa` 或 QidianBrowser 其一作为主路径（文档写死）
  - 验收：产品决策记录在本文或独立 ADR
  - 文档：`docs/p2-tunnel-client-decision.md`（主路径 dpa，辅路径 QidianBrowser）
- [x] 端到端：安装 → 注册 → 暴露本地 MCP/HTTP → 云端调用成功
  - 验收：有 smoke 清单与 request_id / audit 证据
  - 2026-07-17 生产 HTTP Tunnel e2e PASS：证据 `docs/p2-tunnel-e2e-evidence-2026-07-17.md`（rid `…XMJZfGnS` 等）
  - 2026-07-19 生产 **mcp_code** tools/list + tools/call PASS：证据 `docs/p2-mcp-code-e2e-evidence-2026-07-19.md`（`sha-5f695ffe`，slug `tun-bqzyhbyt2mmx`）
  - 说明：本轮用本地构建 dpa；公网 `/agent/install.sh` 已改为 shell/bootstrap（见 router/web-router.go）
- [x] 控制台给出可复制安装/注册/route 命令
  - 验收：用户无需读源码即可完成
  - 说明：Tunnel Connections Agent Setup / README / install script 已有
- [x] 权限与安全：路径沙箱、危险操作确认、审计
  - 验收：默认最小权限；写入类能力有确认
  - 说明：dpa 默认关闭 write/exec/arbitrary；审计日志本地+服务端
- [x] 失败诊断：`dpa status/doctor` 或等价 + 控制台在线状态
  - 验收：离线/鉴权失败/路由错误可定位
- [x] 与计费/审计打通（调用可追溯、可拒绝）
  - 验收：denied/failed/charged 可查
  - 说明：生产 e2e 已写 `proxy_request` audit + connection `last_request_id`；billing_events 细表未单拉

### P2 退出标准

- [x] 外部用户按文档完成一次「云端 Agent → 本机服务」闭环
  - 2026-07-17：云端 `https://dp.app.mbu.ltd/t/.../tunnel/http/.../hello` → dpa → 本机 18080（见 e2e 证据）
  - 2026-07-19：云端 `/t/.../tunnel/mcp/...` → dpa `mcp_proxy.*` → 本机 19090 假 MCP（见 mcp_code e2e 证据）
- [x] 不再依赖 mock-only 演示主路径
  - 说明：主路径 dpa 真连生产 bridge；QidianBrowser 仍为辅路径

---

## 明确不做 / 后置（避免范围膨胀）

- [ ] ~~协议转换长尾（file_search / computer_use / code_interpreter 等）~~ → 仍按 `data-proxy-near-term-development-plan.md` 冻结，不进本清单主线
- [ ] ~~多节点 / 跨节点 SSE / 分布式 Tunnel 限流~~ → 单机稳定后再单独立项
- [ ] ~~企业治理 SSO 真实连接器全家桶~~ → 见 enterprise 队列，不阻塞 P0 口碑项

---

## 建议实施顺序（执行指针）

当前推荐从 P0 起串行推进（**功能实现已大部分完成，以下 checkbox 为历史顺序，以验收证据为准**）：

1. [ ] **P0-1** 额度总览 API + 钱包/资料入口  
2. [ ] **P0-2** 请求扣费解释（log + UI）  
3. [ ] **P0-4** 错误文案人话化（可与 P0-2 并行）  
4. [ ] **P0-3** 3 分钟接入文档与站内入口  
5. [ ] **P1-1** Token 包购买/卡密  
6. [ ] **P1-2** 模型广场测通与复制  
7. [ ] **P1-4** 渠道健康傻瓜化  
8. [ ] **P1-3** 部署一键化  
9. [ ] **P2-1** 客户端隧道 E2E  

---

## 验收与发布记录

| 里程碑 | 日期 | 版本/提交 | 备注 |
| --- | --- | --- | --- |
| 规划落盘 | 2026-07-16 | — | 本文创建 |
| P0 完成 | | | |
| P1 完成 | | | |
| P2 完成 | | | |

---

## 变更记录

- 2026-07-16：初版，收录对标讨论中的 P0/P1/P2 与功能拆分。
| 生产部署 sha-da5af9b2 | 2026-07-17 | `sha-da5af9b2` | 文档静态资源上线；公开验收 ALL_PASS |
| API Key 请求面验收 | 2026-07-17 | `sha-da5af9b2` | chat/responses PASS；UI session 手测仍缺 |
| API 层扣费字段验收 | 2026-07-17 | `sha-da5af9b2` | token log 含 funding_source/wallet_quota_deducted |
| P2 dpa HTTP Tunnel 生产 e2e | 2026-07-17 | `sha-da5af9b2` | setup→run→云端调用→audit；见 `docs/p2-tunnel-e2e-evidence-2026-07-17.md` |
| P2 mcp_code 生产 e2e | 2026-07-19 | `sha-5f695ffe` | initialize/tools.list/tools.call + audit；见 `docs/p2-mcp-code-e2e-evidence-2026-07-19.md` |
