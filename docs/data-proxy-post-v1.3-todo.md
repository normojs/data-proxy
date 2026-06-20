# Data Proxy Post V1.3 TODO

本文档承接 V1.3 通知闭环发布后的剩余工作。排序原则是先保证代码进入 GitHub 后能自动验证，再完成发布链路和当前工作区已有功能线，最后进入更大的企业治理后续版本。

## 当前基线

- V1.3 通知闭环代码已推送到 `normojs/main`，最新提交包含站内通知、审计、outbox、email、webhook、通知偏好、投递日志、手动重试、用户邮件偏好和 new-api attribution 合规补丁。
- new-api 的 `LICENSE`、`NOTICE` 和可见 attribution 链路必须持续保留；所有后续改动都不能破坏 AGPLv3 和 NOTICE Section 7 的要求。
- HStation OAuth 和 fusion-benchmark 两条功能线已拆分提交并通过 GitHub CI；当前剩余工作以真实发布证据和后续企业治理版本为主。

## 开发顺序

| 顺序 | ID | 优先级 | 状态 | 任务 | 验收标准 |
| --- | --- | --- | --- | --- | --- |
| 1 | DP-CI-001 | P0 | Done | GitHub Actions 常规 CI | `main` push、PR 和手动触发时运行 Go 测试、企业治理 smoke、前端 typecheck/build、审批通知链接 smoke、artifact/whitespace 检查。 |
| 2 | DP-REL-001 | P0 | Done | 发布证据和 Docker 链路固化 | 预发/生产 R0-R3 证据模板可填写；Docker tag、构建命令、镜像摘要、回滚 tag 和环境变量清单可追溯。 |
| 3 | DP-OAUTH-001 | P0 | Done | HStation OAuth 功能收口 | 后端 provider、前端登录/绑定/系统设置、错误提示、真实回调地址验证完成；相关改动单独提交，不混入 benchmark。 |
| 4 | DP-OAUTH-002 | P0 | Done | HStation OAuth 自动化验证 | 覆盖登录、注册、绑定、解绑、取消授权、重复绑定、回调错误；至少补后端单测和前端 typecheck。 |
| 5 | DP-BENCH-001 | P1 | Done | fusion-benchmark 工具收口 | 明确数据文件和 fixtures 是否入库；CLI、README、测试和样例数据可复现，不泄露密钥或真实隐私数据。 |
| 6 | DP-BENCH-002 | P1 | Done | fusion-benchmark CI/文档策略 | 若工具进入主仓库，增加轻量测试命令和文档；若不进入主仓库，迁移到独立仓库或保持未提交。 |
| 7 | DP-V14-001 | P1 | Done | V1.4 SSO 组织同步方案 | 设计 importer 抽象、增量同步、dry-run、冲突处理 UI、同步审计和回滚边界。 |
| 8 | DP-V15-001 | P2 | Done | V1.5 Redis 原子企业额度计数 MVP | 默认关闭的 `EnterpriseQuotaRedisCounterEnabled` 开关、Redis Lua 原子 reserve/settle/refund、DB seed、防 Redis 异常降级和回归测试。 |
| 9 | DP-V15-002 | P2 | Done | V1.5 DB/Redis 对账补偿 MVP | 管理员 dry-run/repair API、主节点周期对账任务、Redis 快照幂等修复、审计日志和回归测试。 |
| 10 | DP-V15-003 | P2 | Done | V1.5 token 级 hard limit | token 模型支持硬额度上限；relay 前置校验、超限错误、结算回滚和单测覆盖完整。 |
| 11 | DP-V15-004 | P2 | Done | V1.5 并发压测脚本 | 提供企业额度 reserve/settle/refund 并发一致性脚本，并在文档中记录 Redis/DB 两种模式的运行方式。 |
| 12 | DP-V15-005 | P2 | Done | V1.5 Redis-only 崩溃恢复 | 扫描 Redis-only counter key、补建缺失 DB mirror，并评估操作级幂等补偿队列。 |
| 13 | DP-V16-001 | P2 | Done (MVP+) | V1.6 高级策略动作 | 支持 alert、fallback_model、queue、shared_pool 等动作的配置、命中观测、审计事件和响应提示；fallback_model 已能改写模型、重选渠道并按降级模型重新估算预扣费；queue 已有企业维度同步 admission queue、超时 429、响应 header、审计事件、持久化 admission 记录和企业治理页面可见性；shared_pool 已能计算本次借用量并写入响应 header 和审计；异常检测已能基于突增、失败率和成本异常进入企业短时保护限流；异步排队执行、独立共享池容量模型、异常阈值配置和报表归属留给后续增强。 |
| 14 | DP-V17-001 | P2 | Done (RBAC MVP+) | V1.7 企业治理 RBAC/财务视图 | 已完成企业治理 capability 鉴权、前端入口/页签控制、财务/审计只读角色、部门管理员 scoped 边界、部门 scoped 策略组授权、项目成员 read/admin 权限矩阵、财务分摊 CSV 导出、审计 scoped 过滤和基础回归测试。 |

## 当前开始项：DP-V17-001

V1.7 先交付最小可用的企业治理分权闭环，避免所有企业治理操作都只能依赖系统管理员：

1. 已完成：新增企业治理 capability 模型，覆盖 `enterprise.read`、`enterprise.manage`、`enterprise.finance.read`、`enterprise.audit.read`、`enterprise.quota.approve`、`enterprise.project.manage`。
2. 已完成：系统 `admin/root` 继续拥有全部企业治理权限，保持 new-api 管理员兼容；普通用户只有在企业治理开关开启并具备企业成员角色时才获得企业治理权限。
3. 已完成：企业治理 API 按 capability 分组鉴权，企业管理员可管理配置，财务查看员只能读取财务用量，审计员只能读取审计/通知 outbox/worker metrics，项目管理员可管理项目。
4. 已完成：`/api/user/self` 返回 `permissions.enterprise_governance`，前端侧边栏、路由入口、企业编辑按钮、页签和审批按钮按 capability 控制显示，并避免无权限页签主动请求后端。
5. 已完成：新增路由回归测试，覆盖财务/审计只读边界、企业管理员非系统管理员管理能力、普通用户隔离和基础鉴权路径。
6. 已完成：`department_admin` 按主部门展开本部门及子部门 scope，可管理 scope 内成员、部门 scoped 策略组和额度策略、审批 scope 内临时额度申请、查看 scope 内用量；后端会过滤成员列表、策略组、额度策略、审批列表、审批通知和用量报表，并拒绝跨部门写入/审批。
7. 已完成：财务视图支持按当前筛选导出 usage breakdown CSV，导出复用 `enterprise.finance.read` 鉴权和部门管理员 scope，字段包含维度、目标、请求数、quota、prompt/completion/total tokens。
8. 已完成：`project_admin` 使用 `enterprise_projects.owner_user_id` 作为项目 scope，项目列表、项目管理、usage summary/breakdown 和 CSV 导出限制为自己负责的启用项目；跨项目财务查询会被拒绝。
9. 已完成：审计日志新增结构化 scope 字段并在写入时自动推断；部门管理员只能查看本部门及子部门成员、策略组、部门策略、审批、项目和 relay 审计；项目管理员只能查看自己负责项目相关审计；notification outbox 和 worker metrics 仍仅开放给审计员/企业管理员。
10. 已完成：策略组新增可选部门 scope；企业管理员可继续维护全局策略组，部门管理员只能查看、创建、编辑、停用自己部门树内的 scoped 策略组，只能加入 scope 内成员，并可创建指向 scoped 策略组的额度策略；跨部门或全局策略组写入会被拒绝。
11. 已完成：项目成员新增 `admin/member` 角色；项目 owner 或项目 `admin` 成员可维护项目成员，`project_admin` 的项目 scope 已从 owner 项目扩展为 owner 或项目 `admin` 成员项目。
12. 已完成：项目成员权限矩阵最小闭环，`member` 只获得项目列表、项目成员列表、项目用量和项目审计的 scoped 只读权限；`admin`/owner 才获得项目编辑、停用和成员维护权限；没有任何项目 scope 的 `project_admin` 返回空项目/空用量，不会落回全局数据。
13. 已完成：策略组成员角色最小闭环，`enterprise_policy_group_members` 新增 `viewer/editor` 角色；成员列表返回 role，批量添加接口可设置或更新 role，旧请求默认 `viewer`；策略命中仍按成员存在判断，不改变现有额度策略语义。
14. 已完成：策略组跨部门协作最小闭环，策略组可配置 `shared_org_unit_ids` 共享给其他部门；共享部门管理员可查看共享策略组、维护本部门 scope 内成员、创建指向共享策略组的额度策略，但不能编辑/停用策略组本体；共享配置由企业管理员或策略组归属部门管理员维护。
15. 后续：跨部门协作审批流、共享有效期和按角色细分的跨部门编辑权限仍保留为后续增强。

## 已完成项：DP-V16-001

V1.6 高级策略动作先交付最小可用的“策略命中可见”，避免在 relay 已完成渠道选择和计费准备后静默改写请求：

1. 已完成：策略 action 白名单扩展为 `reject`、`alert`、`fallback_model`、`queue`、`shared_pool`，旧数据中的未知 action 保守按 `reject` 处理。
2. 已完成：`reject` 保持硬拒绝；`alert`、`fallback_model`、`queue`、`shared_pool` 作为非阻断动作记录命中，不中断当前请求。
3. 已完成：非阻断动作命中后继续更新策略 counter，用于后续运营看板和审计排查；`fallback_model` 从现有模型范围中给出推荐模型提示。
4. 已完成：relay 响应写入 `X-Data-Proxy-Enterprise-Policy-Actions`、`X-Data-Proxy-Enterprise-Policy-Action-Hint`，必要时写入 `X-Data-Proxy-Enterprise-Fallback-Model`。
5. 已完成：命中动作写入 `enterprise_governance.policy_action` 审计事件，前端审计详情可识别为策略动作观测；配额策略 UI 支持选择 action 并在列表展示。
6. 已完成：`fallback_model` 命中后会同步改写 relay 模型、JSON 请求体和 context 原始模型，按降级模型重选渠道，并在用户预扣费前重新估算 token 与价格；用量归因沿用降级后的模型。
7. 已完成：`queue` 命中后会按企业维度进入同步 admission queue，拿到队列槽后继续 relay 并在请求结束释放；等待超时会在用户预扣费前返回 429，响应写入 `X-Data-Proxy-Enterprise-Queue-Status`、`X-Data-Proxy-Enterprise-Queue-Wait-Ms`、`X-Data-Proxy-Enterprise-Queue-Timeout-Ms`，同时记录 `enterprise_governance.queue_admission` 审计和 `enterprise_governance_queue_admissions` 持久化 admission 记录；企业治理 API 与审计页可查看最近 admission/timeout/canceled 记录。
8. 已完成：`shared_pool` 配额超限命中后会基于结构化 action observation 计算本次请求实际超出软限的借用量，响应写入 `X-Data-Proxy-Enterprise-Shared-Pool-Status`、`X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Quota`、`X-Data-Proxy-Enterprise-Shared-Pool-Borrowed-Requests`，并记录 `enterprise_governance.shared_pool_reserve` 审计。
9. 已完成：异常检测会基于企业最近窗口与基线窗口的请求突增、quota 成本突增，以及 consume/error 日志中的异常失败率进入短时内存保护；触发后在用户预扣费前返回 429，响应写入 `X-Data-Proxy-Enterprise-Anomaly-Status`、`X-Data-Proxy-Enterprise-Anomaly-Reason`、`X-Data-Proxy-Enterprise-Anomaly-Protected-Until`、`X-Data-Proxy-Enterprise-Anomaly-Cooldown-Seconds`，并记录 `enterprise_governance.anomaly_throttle` 审计；dry-run 模式只写 would-throttle 观测，不阻断请求。
10. 后续：`queue` 的异步执行/后台重试/取消语义仍需独立增强；`shared_pool` 需要独立共享池容量模型、借用上限、归还和报表归属；异常检测需要管理员阈值配置、持久化保护状态、按项目/部门的细粒度动作编排和趋势报表。

## 当前进展

- DP-CI-001 已完成并在 GitHub Actions 通过：Backend 覆盖 Go 测试、企业治理 smoke 和 whitespace；Frontend 覆盖 typecheck、审批通知 deep link smoke 和 build。
- DP-REL-001 已完成：`v1.3.0` tag 触发 GHCR Docker 发布，Docker workflow run `27858433012` 成功；镜像 digest 为 `sha256:7650bff674c4a2b070197feba382414c47285de0578ddb2749dbbb84996046ac`，发布证据已写入 `docs/data-proxy-release-runbook.md`。
- DP-OAUTH-001 已完成基础交付：新增 HStation OAuth provider、启用配置校验、登录/注册/绑定入口、系统设置页和管理员绑定查看能力。
- DP-OAUTH-002 已完成：新增 `oauth/hstation_test.go` 和 `controller/oauth_test.go`，覆盖 HStation token/userinfo、登录、注册、绑定、当前用户解绑、取消授权、重复绑定、state 错误和 token 错误；前端个人资料页已接入 HStation 解绑；CI Backend 已纳入 `./oauth`。本地已通过 `go test ./model ./controller ./service ./router ./oauth`、`cd web/default && bun run typecheck`、`cd web/default && bun run smoke:approval-notification-links`、`cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build`，提交 `1f6be929` 已推送到 `normojs/main`，GitHub Actions run `27858100588` 全部通过。真实回调地址仍需在预发或 FRP 环境执行并记录到发布证据，不再作为自动化任务阻塞。
- DP-BENCH-001/002 已完成：fusion-benchmark CLI、README、config、fresh/code 数据集、fixtures、测试文件和评估文档已收口入库；新增 `scripts/fusion-benchmark-check.sh`，离线覆盖 config 校验、fresh/code 数据集校验、内置 self-test 和常见密钥模式扫描；CI 新增 `Fusion Benchmark` job 调用该脚本。提交 `a764b6ca` 已推送到 `normojs/main`，GitHub Actions run `27857644905` 的 Frontend、Backend、Fusion Benchmark job 全部通过。
- DP-V14-001 已完成最小可用闭环：新增 SSO 组织同步 payload importer、dry-run preview、冲突列表、apply 事务、`org_sync.apply` 审计事件和 Organization 页同步面板；支持 HStation/OIDC/GitHub 等 provider user id 映射，未冲突行可选择性应用。本地已通过 `go test ./model ./controller ./service ./router ./oauth`、`cd web/default && bun run typecheck`、`cd web/default && bun run smoke:approval-notification-links`、`cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build`。
- DP-V15-001 已完成最小可用闭环：新增默认关闭的 `EnterpriseQuotaRedisCounterEnabled` 配置项；企业额度 reserve 优先走 Redis Lua 原子计数，quota exceeded 保持硬拒绝，Redis/后端异常时降级 DB 原路径；Redis 首次 key 使用 DB counter seed，避免中途启用后从零计数；reserve 成功后同步 DB counter 用于审计、可见性和后续对账；settle/refund 会同步 Redis 和 DB。新增回归覆盖默认关闭配置、Redis reserve/settle/refund、DB seed 和现有并发 DB 限额。
- DP-V15-002 已完成最小可用闭环：新增 `POST /api/enterprise/quota-counters/reconcile`，支持管理员 dry-run 和 repair；新增主节点周期任务，在 `EnterpriseQuotaRedisCounterEnabled` 且 Redis 可用时每 5 分钟对账活跃 DB counter；repair 使用 Redis `SetSnapshot` 幂等修复到 DB 当前快照，并写入 `quota_counter.reconcile` 审计；新增回归覆盖差异发现、修复和审计可见性。Redis-only 崩溃恢复和操作级补偿队列拆到 DP-V15-005。
- DP-V15-003 已完成最小可用闭环：新增 `quota_hard_limit_enabled` token 字段；API Key 控制台支持 unlimited token 配置硬上限；relay hard limit 会禁用信任额度旁路、前置拒绝超限并在正向补扣时先锁定 token 额度；MCP 继续只按 `price_per_call` 进行按次扣费，并把该按次额度纳入 token hard limit 预检、结算和退款；新增 controller/service 回归测试。
- DP-V15-004 已完成轻量并发压测入口：新增 `scripts/enterprise-quota-counter-stress.sh` 和 `make enterprise-quota-counter-stress`，默认跑 DB 与 Redis-code-path(fake atomic counter) 两种模式；覆盖高上限并发 reserve 后混合 settle/refund 的最终一致性，以及低上限并发抢占的成功/拒绝数量和 refund 后 reserved 归零。常用命令：`scripts/enterprise-quota-counter-stress.sh`；仅 DB：`ENTERPRISE_QUOTA_COUNTER_STRESS_MODE=db scripts/enterprise-quota-counter-stress.sh`；仅 Redis 代码路径：`ENTERPRISE_QUOTA_COUNTER_STRESS_MODE=redis scripts/enterprise-quota-counter-stress.sh`；连接真实 Redis Lua 路径：`REDIS_CONN_STRING=redis://:123456@127.0.0.1:6379/0 ENTERPRISE_QUOTA_COUNTER_STRESS_MODE=redis ENTERPRISE_QUOTA_COUNTER_STRESS_REDIS_BACKEND=real scripts/enterprise-quota-counter-stress.sh`。
- DP-V15-005 已完成最小恢复闭环：`POST /api/enterprise/quota-counters/reconcile` 新增可选 `include_redis_orphans`；后台周期 repair 默认打开 Redis-only 扫描；Redis key 会按 `enterprise_quota_counter:v1:{enterprise}:{policy}:{target_type}:{target_id}:{metric}:{period_start}` 解析，若当前 policy 维度仍匹配且 DB mirror 缺失，则 dry-run 返回 `missing_db`，repair 创建 `enterprise_quota_counters` mirror、保留 Redis used/reserved 快照并写入 `quota_counter.reconcile` 审计。操作级幂等补偿队列仍保留为后续增强项。
- DP-V16-001 已完成增强闭环：配额策略支持 `alert`、`fallback_model`、`queue`、`shared_pool` 非阻断动作；策略命中会保留 counter 观测、响应 header 提示和 `enterprise_governance.policy_action` 审计；`fallback_model` 已从推荐升级为 relay 执行动作，会改写请求模型、重选渠道并按降级模型重新估算预扣费；`queue` 已从可见 MVP 升级为企业维度同步 admission queue，命中后先排队拿槽，超时在用户预扣费前返回 429，并记录 `enterprise_governance.queue_admission` 审计和 `enterprise_governance_queue_admissions` 持久化 admission 记录，企业治理审计页可直接查看最近 admission 状态；`shared_pool` 已能计算本次借用量并记录 `enterprise_governance.shared_pool_reserve` 审计；异常检测已能基于请求突增、失败率和成本突增进入企业短时保护限流，并记录 `enterprise_governance.anomaly_throttle` 审计。异步排队执行、独立共享池容量模型、异常阈值配置和报表归属仍保留为 V1.6 后续任务。
- DP-V17-001 已完成 RBAC MVP+：企业治理后端 API 改为 capability 分组鉴权，前端入口和页签按企业角色权限控制；财务查看员、审计员和项目管理员获得最小只读/管理入口；部门管理员按本部门及子部门 scope 管理成员、策略组、额度策略、审批、用量和审计日志；项目管理员按 owner 或项目 admin 成员 scope 管理项目并查看/导出项目用量和项目审计日志，项目 member 成员仅能 scoped 只读查看项目、成员、用量和审计；策略组成员支持 viewer/editor 角色，策略组支持共享给其他部门进行跨部门协作；财务 usage breakdown 支持按筛选导出 CSV；新增回归覆盖只读角色、企业管理员、普通用户隔离、部门管理员跨部门越权、部门策略组边界、策略组成员角色、跨部门共享策略组、项目成员管理员边界、项目成员只读边界、空项目 scope 防泄漏、部门用量过滤、项目管理员跨项目越权、CSV 导出和 scoped 审计可见性。

## 提交和发布规则

- 每条功能线独立提交：CI/发布文档、HStation OAuth、fusion-benchmark、后续企业治理版本不得混在一个 commit。
- 提交前必须确认 `git status --short` 中 staged 文件只属于当前任务。
- 外部通知、OAuth、benchmark 数据都要做敏感信息检查，不提交 token、secret、真实用户邮箱或真实业务 payload。
- 对 new-api 的开源协议要求保持可见合规：保留 `LICENSE`、`NOTICE`、原项目链接和 `Frontend design and development by New API contributors.` 文案。
