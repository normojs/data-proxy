# Snapless Connected App V1.3 Release Evidence

本文档记录 Snapless Connected App V1.3 的发布前验收证据。代码功能、本地 preflight、协议合规检查和发布模板已经在本地完成；预发和生产环境需要在对应环境执行后补充真实 request ID、outbox ID、截图或变更单链接。

## 本地核验证据

| 字段 | 记录 |
| --- | --- |
| 记录时间 | 2026-06-21 11:33:23 CST |
| 核验代码 commit | `0dd88e11` |
| 工作区 | `/Users/fushilu/workspace/revocloud/data-proxy/upstream/new-api` |
| 核验命令 | `scripts/snapless-connected-app-preflight.sh` |
| 结果 | 通过 |

覆盖项：

- Connected App 申请、审批、审计和站内通知：`TestConnectedAppRequestApprovalCreatesAppAuditAndNotifications`。
- Device Code Flow：`TestConnectedAppDeveloperAPIAndDeviceFlow` 和 Snapless device flow 测试覆盖 start、status、authorize、poll once、trusted gate 和 consumed session。
- 开发者 key 自助：`TestConnectedAppDeveloperSelfService` 覆盖 SDK config、OpenAPI、key 创建、复用、轮换、一次性明文 key、审计和 attribution。
- Usage 筛选：`TestConnectedAppDeveloperSelfService` 覆盖 `start_time/end_time`、`token_id`、by_model 和 by_token 聚合。
- 授权排障：`TestConnectedAppDeveloperAPIAndDeviceFlow` 覆盖 `/developer/authorizations` 与 `/developer/device-sessions?status=consumed`。
- notification outbox：router/service 测试覆盖审批、设备授权、token lifecycle、email/webhook preference、幂等、webhook HMAC payload 和非法配置拒绝。
- MCP 计费回归：`go test ./pkg/mcp/billing ./pkg/mcp/proxy ./pkg/mcp/executor -count=1`，保持 MCP 按调用次数和 `price_per_call` 语义。
- Profile 和系统设置前端：connected app 相关页面 eslint、`bun run typecheck`、`bun run build`。
- GitHub CI 门禁：`CI` workflow 新增 `Snapless Connected App` job，独立执行 `scripts/snapless-connected-app-preflight.sh`；目标 commit `8f61ab24` 的 run 已通过。
- 发布合规：`LICENSE`、`NOTICE`、`THIRD-PARTY-LICENSES.md`、README new-api attribution、NOTICE Section 7 文案和 Docker `/licenses/` copy 检查。
- 空白和本地构建产物检查：`git diff --check` 和 release artifact check。

## V1.3 功能证据清单

| 功能 | 本地证据 | 结果 |
| --- | --- | --- |
| 应用申请/审批/通知/审计 | router 测试和 connected app notification outbox 测试 | 通过 |
| 通用 device flow | router 测试覆盖 start/status/authorize/poll once/trusted gate | 通过 |
| 开发者自助 key | router 测试覆盖 create/reuse/rotate/API key once/audit/attribution | 通过 |
| OpenAPI/SDK 交付 | router 测试覆盖 SDK env、OpenAPI URL、按 scope 裁剪示例和 OpenAPI paths | 通过 |
| Usage 筛选和历史 token 归属 | router 测试覆盖时间范围、token_id、by_model、by_token 和 historical 状态 | 通过 |
| 授权排障视图数据源 | router 测试覆盖 authorizations 和 device sessions status 筛选 | 通过 |
| email/webhook outbox | router/service 测试覆盖 preference、webhook、HMAC、幂等和 retry 基础状态 | 通过 |
| MCP 计费语义 | MCP billing/proxy/executor 回归测试 | 通过 |
| GitHub CI 门禁 | `CI / Snapless Connected App` job 执行 preflight | 通过，run: `https://github.com/normojs/data-proxy/actions/runs/27892488510` |
| 发布协议合规 | preflight 检查 README、NOTICE、LICENSE、THIRD-PARTY-LICENSES.md 和 Dockerfile | 通过 |

## 预发执行记录

| 字段 | 记录 |
| --- | --- |
| 环境 | preprod |
| 执行人 |  |
| 执行时间 |  |
| 版本或 commit |  |
| 数据库类型 | sqlite / mysql / postgres |
| Connected App slug |  |
| 测试开发者 user ID |  |
| 测试授权用户 ID |  |
| 测试 token ID |  |
| 关联变更单 |  |

### 预发检查项

| 步骤 | request_id / outbox_id / token_id | 检查项 | 预期 | 实际 | 结论 |
| --- | --- | --- | --- | --- | --- |
| 1 |  | 开发者提交应用申请 | request 为 pending，管理员可见，站内通知可见 |  |  |
| 2 |  | 管理员审批应用 | app 为 trusted/enabled，审计和申请人通知写入 |  |  |
| 3 |  | 桌面端 device/start | 返回 device_code/user_code，URL 不含 API key |  |  |
| 4 |  | 浏览器授权 approve | session 为 authorized，设备授权通知 outbox 入队 |  |  |
| 5 |  | 桌面端首次 poll | 返回一次性 `sk-...`，session 可进入 consumed |  |  |
| 6 |  | 桌面端重复 poll | 不再返回明文 key |  |  |
| 7 |  | Profile 创建/轮换 developer key | 创建/轮换响应展示一次性 key，复用不展示 key |  |  |
| 8 |  | SDK env 和示例调用 | env、OpenAPI URL、JS/curl 示例可复制并可完成一次最小调用 |  |  |
| 9 |  | Usage 筛选 | start/end/model/token_id 筛选结果符合日志数据 |  |  |
| 10 |  | 授权排障 | authorizations、devices、device sessions 状态与实际流程一致 |  |  |
| 11 |  | notification outbox | email/webhook pending、失败、retry 和 worker metrics 可见 |  |  |
| 12 |  | 发布协议检查 | README、LICENSE、NOTICE、THIRD-PARTY-LICENSES.md 和 Docker `/licenses/` 保留 |  |  |

## 生产发布记录

生产执行前复制预发表格，使用生产变更单编号记录。扩大范围前必须保留：

- 发布 commit、tag、CI run URL 和 Docker workflow run URL。
- 镜像 tag、digest 和回滚镜像 digest。
- 数据库迁移说明和回滚负责人。
- 关闭 email/webhook preference 的操作路径。
- 最近一次 `scripts/snapless-connected-app-preflight.sh` 通过记录。

可运行 `make snapless-connected-app-release-evidence` 生成当前 commit 的 CI、job URL、tag、Docker workflow 和 digest 快照，再补充到本文件或变更单。
