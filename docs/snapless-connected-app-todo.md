# Snapless Connected App TODO

本文档记录 Snapless 接入在 Device Code Flow 之后的开发顺序。Data Proxy 基于 new-api，后续所有改动必须继续保留 AGPLv3、NOTICE 和上游 attribution。

## 当前基线

- 已完成 Connected App 最小内核：`connected_apps`、`connected_app_grants`、`connected_app_token_bindings`。
- 已完成 Snapless 原生 token 创建、复用、轮换、撤销和 health 检查。
- 已完成 Device Code Flow：桌面端 `device/start`，浏览器 `/snapless/device` 登录授权，桌面端 `device/poll` 一次性取回 key。
- 已完成用户控制台 Snapless Connected App 卡片：可查看 grant、设备、最近使用时间、token 状态，并可轮换或撤销单台设备。
- 已完成 Snapless 状态与充值闭环：health/config/devices/device status 返回可操作 `actions`，前端可提示余额不足、用户禁用和模型不可用并跳转对应入口。
- 已完成 Connected App 应用管理 MVP：管理员可在系统设置中查看、新增、编辑、停用应用，并配置 allowed/default scopes 与 trusted 状态。
- 已完成应用申请和权限审批 MVP：第三方应用可提交接入申请，管理员可审批并生成 trusted app，审批状态进入站内通知和 connected app 审计。
- 已完成应用开发者 API MVP：获批申请人可查看自身 app 配置、允许 endpoint、授权用户/设备和 device session 状态；trusted app 可使用通用 device code flow 创建设备授权。
- 已完成 Connected App 外部通知 MVP：审批结果、设备授权批准/拒绝、异常 health 状态可按 preference 写入 email/webhook outbox；投递失败可重试且不阻断审批或授权。
- 已完成 Connected App 通知管理前端：系统设置页可管理全局 preference、webhook、outbox、worker metrics 和 retry；Profile 开发者卡片可按获批应用管理 app 级 preference、webhook 和 outbox。
- 已完成外部通知演练文档：审批结果、设备授权、health warning 均提供 curl 触发和 HMAC webhook 验签样例。
- 已完成撤销/轮换通知事件：Snapless 授权、token rotate/revoke、设备撤销和最后设备触发的 grant 撤销均可写入 connected app notification outbox。
- 已完成 Connected App scope 强约束：绑定 token 访问 relay/usage endpoint 时校验 binding、app、grant 和 required scope；普通 token 不受影响，未映射 token endpoint 默认拒绝。
- 已完成应用级自助能力 MVP：获批 app 开发者可拉取 SDK/OpenAPI 配置，在 `token.manage` 范围内创建或轮换自己的开发者 key，并在 `quota.read` 范围内查看跨 token 轮换的 usage 聚合。
- 已完成 Connected App token 历史归属：`connected_app_token_attributions` 不可变记录 token/app/binding/device 归属，developer usage 可统计当前 token 和轮换前 token 的消费历史；旧库当前 binding 仍有 fallback。
- 已完成 Profile 开发者自助入口：获批 app 开发者可在 Connected App Developer 卡片下载 SDK/OpenAPI 配置，创建/轮换一次性 developer key，并查看 total、by_model、by_token usage，历史 token 标记为 `historical`。
- MCP 计费语义不改，仍按工具调用次数和 `price_per_call` 扣费。

## 开发顺序

| 顺序 | ID | 优先级 | 状态 | 任务 | 验收标准 |
| --- | --- | --- | --- | --- | --- |
| 1 | SNAPLESS-001 | P0 | Done | Connected App 最小 token 闭环 | 登录用户可为 Snapless 创建设备 token；复用不返回明文 key；轮换返回新 key；撤销最后设备时撤销 grant；health 能识别 token、用户、grant、binding 和模型可用性。 |
| 2 | SNAPLESS-002 | P0 | Done | Device Code Flow | API key 不进入 URL；浏览器只批准/拒绝；桌面端凭 `device_code` 首次 poll 取 key；重复 poll 不再返回明文；后端测试覆盖 pending、authorize、once、health。 |
| 3 | SNAPLESS-003 | P1 | Done | 用户控制台 Connected App 卡片 | 用户能在控制台查看 Snapless grant、设备列表、最近使用时间、token 状态；能轮换或撤销单台设备；状态与 `/api/snapless/health` 语义一致。 |
| 4 | SNAPLESS-004 | P1 | Done | 余额和充值闭环 | health/config 返回余额不足、用户禁用、模型不可用等可操作状态；前端在 Snapless 卡片和授权页显示明确状态，并能跳到充值或设置入口。 |
| 5 | SNAPLESS-005 | P1 | Done | Connected App 应用管理 MVP | 管理员可新增/停用应用、配置 allowed/default scopes、trusted 状态和授权方式；Snapless 从内置 app 过渡为可管理 app，保留 migration 兼容。 |
| 6 | SNAPLESS-006 | P2 | Done | 应用申请和权限审批 | 第三方应用可提交接入申请；管理员审核 scopes、回调/设备流能力和展示信息；审批结果写入审计和站内通知。 |
| 7 | SNAPLESS-007 | P2 | Done | 应用开发者 API | 获批应用可创建自己的 device sessions、查看授权状态和查询允许的 API endpoints；只暴露与自身 app 相关的 grant/binding/session。 |
| 8 | SNAPLESS-008 | P2 | Done | 邮件/Webhook 通知扩展 | 审批结果、设备授权批准/拒绝和异常 health 状态可写入 connected app notification outbox；email/webhook 由 preference 控制，投递 worker 支持重试和 metrics，失败不阻断主流程。 |
| 9 | SNAPLESS-009 | P2 | Done | 通知管理前端 | 管理员系统设置页可管理全局 preference、webhook、outbox、worker metrics 和 retry；获批应用开发者可在 Profile 开发者卡片中管理 app 级 preference、webhook 和 outbox。 |
| 10 | SNAPLESS-010 | P2 | Done | 外部通知演练文档 | 审批结果、设备授权、health warning 三类事件均有 curl 触发流程和 webhook HMAC 验签 receiver 样例。 |
| 11 | SNAPLESS-011 | P2 | Done | 撤销/轮换通知事件 | Snapless 授权批准/拒绝、token rotate/revoke、设备撤销和最后设备触发的 grant 撤销均写入 connected app notification outbox；通知失败不阻断主流程。 |
| 12 | SNAPLESS-012 | P1 | Done | Connected App scope 强约束 | 绑定 token 只能访问 app allowed scopes 与 grant scopes 同时允许的 endpoint；binding/app/grant 异常或未映射 token endpoint 均拒绝；普通 token 保持兼容。 |
| 13 | SNAPLESS-013 | P1 | Done | 应用级自助能力 MVP | 获批 app 开发者可拉取 SDK/OpenAPI 配置；具备 `token.manage` 可自助创建/轮换当前登录用户自己的开发者 key；具备 `quota.read` 可查看当前 app usage 聚合；创建/轮换写入 connected app audit。 |
| 14 | SNAPLESS-014 | P2 | Done | Connected App token 历史归属 | `connected_app_token_attributions` 记录 token/app/binding/device 不可变归属；developer usage 能跨 token 轮换统计完整历史，并保留当前 binding fallback。 |
| 15 | SNAPLESS-015 | P2 | Done | 自助能力前端入口 | Profile 开发者卡片展示 SDK/OpenAPI 下载、自助 key 创建/轮换、usage summary 与按模型/token 聚合。 |

## 立即下一步

1. 开发者自助筛选增强：给 Profile usage 面板增加时间范围、模型和 token 筛选，参数直连 `/developer/usage`。
2. 开发者授权排障视图：在 Profile 开发者卡片中补充授权用户、设备和最近 device session 状态，方便 app 开发者排查授权失败或设备未消费。
3. OpenAPI/SDK 交付增强：补充最小 SDK 示例代码和复制环境变量入口，保持 key 明文只在创建/轮换响应中展示一次。
