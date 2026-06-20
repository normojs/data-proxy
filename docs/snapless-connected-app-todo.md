# Snapless Connected App TODO

本文档记录 Snapless 接入在 Device Code Flow 之后的开发顺序。Data Proxy 基于 new-api，后续所有改动必须继续保留 AGPLv3、NOTICE 和上游 attribution。

## 当前基线

- 已完成 Connected App 最小内核：`connected_apps`、`connected_app_grants`、`connected_app_token_bindings`。
- 已完成 Snapless 原生 token 创建、复用、轮换、撤销和 health 检查。
- 已完成 Device Code Flow：桌面端 `device/start`，浏览器 `/snapless/device` 登录授权，桌面端 `device/poll` 一次性取回 key。
- 已完成用户控制台 Snapless Connected App 卡片：可查看 grant、设备、最近使用时间、token 状态，并可轮换或撤销单台设备。
- 已完成 Snapless 状态与充值闭环：health/config/devices/device status 返回可操作 `actions`，前端可提示余额不足、用户禁用和模型不可用并跳转对应入口。
- MCP 计费语义不改，仍按工具调用次数和 `price_per_call` 扣费。

## 开发顺序

| 顺序 | ID | 优先级 | 状态 | 任务 | 验收标准 |
| --- | --- | --- | --- | --- | --- |
| 1 | SNAPLESS-001 | P0 | Done | Connected App 最小 token 闭环 | 登录用户可为 Snapless 创建设备 token；复用不返回明文 key；轮换返回新 key；撤销最后设备时撤销 grant；health 能识别 token、用户、grant、binding 和模型可用性。 |
| 2 | SNAPLESS-002 | P0 | Done | Device Code Flow | API key 不进入 URL；浏览器只批准/拒绝；桌面端凭 `device_code` 首次 poll 取 key；重复 poll 不再返回明文；后端测试覆盖 pending、authorize、once、health。 |
| 3 | SNAPLESS-003 | P1 | Done | 用户控制台 Connected App 卡片 | 用户能在控制台查看 Snapless grant、设备列表、最近使用时间、token 状态；能轮换或撤销单台设备；状态与 `/api/snapless/health` 语义一致。 |
| 4 | SNAPLESS-004 | P1 | Done | 余额和充值闭环 | health/config 返回余额不足、用户禁用、模型不可用等可操作状态；前端在 Snapless 卡片和授权页显示明确状态，并能跳到充值或设置入口。 |
| 5 | SNAPLESS-005 | P1 | Next | Connected App 应用管理 MVP | 管理员可新增/停用应用、配置 allowed/default scopes、trusted 状态和授权方式；Snapless 从内置 app 过渡为可管理 app，保留 migration 兼容。 |
| 6 | SNAPLESS-006 | P2 | Planned | 应用申请和权限审批 | 第三方应用可提交接入申请；管理员审核 scopes、回调/设备流能力和展示信息；审批结果写入审计和站内通知。 |
| 7 | SNAPLESS-007 | P2 | Planned | 应用开发者 API | 获批应用可创建自己的 device sessions、查看授权状态和查询允许的 API endpoints；只暴露与自身 app 相关的 grant/binding/session。 |
| 8 | SNAPLESS-008 | P2 | Planned | 邮件/Webhook 通知扩展 | 在站内通知和审计可见后，再为应用授权、撤销、异常 health 状态补邮件或 webhook。 |

## 立即下一步

1. 设计 `SNAPLESS-005` 数据模型：保留内置 Snapless seed，同时让管理员可配置更多应用。
2. 开发 `SNAPLESS-005` 管理端 MVP：先支持新增、停用和 scopes 配置，再迁移 Snapless 到可管理 app。
3. 开发 `SNAPLESS-006`：在应用申请/权限审批中复用已完成的站内通知和审计事件可见能力。
