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
| 10 | DP-V15-003 | P2 | Pending | V1.5 token 级 hard limit | token 模型支持硬额度上限；relay 前置校验、超限错误、结算回滚和单测覆盖完整。 |
| 11 | DP-V15-004 | P2 | Pending | V1.5 并发压测脚本 | 提供企业额度 reserve/settle/refund 并发一致性脚本，并在文档中记录 Redis/DB 两种模式的运行方式。 |
| 12 | DP-V15-005 | P2 | Pending | V1.5 Redis-only 崩溃恢复 | 扫描 Redis-only counter key、补建缺失 DB mirror，并评估操作级幂等补偿队列。 |
| 13 | DP-V16-001 | P2 | Pending | V1.6 高级策略动作 | 支持 alert、fallback_model、queue、shared_pool 等动作，并保留审计和用户提示。 |
| 14 | DP-V17-001 | P2 | Pending | V1.7 企业治理 RBAC/财务视图 | 企业管理员、部门管理员、财务查看员、审计员、项目管理员的权限边界和回归测试。 |

## 当前开始项：DP-V15-003

V1.5 高并发和精细额度继续按以下顺序拆分，避免额度主链路一次性改动过大：

1. 已完成：梳理 `service/pre_consume_quota.go`、`service/quota.go`、`service/enterprise_policy_counter.go` 和 `model/enterprise_quota_counter.go` 的额度扣减边界，先把企业策略 counter 抽成可切换实现。
2. 已完成：新增 Redis 原子计数抽象，先覆盖企业策略 counter 的 request_count/quota reserve/settle/refund，不改变用户余额主逻辑。
3. 已完成：增加 DB/Redis 对账补偿 MVP，管理员可 dry-run/repair，主节点周期任务可将 Redis 快照幂等修复到 DB 当前值，并写入 `quota_counter.reconcile` 审计。
4. 下一步：补 token 级 hard limit 的模型、校验和回归测试。
5. 后续：增加轻量并发测试或压测脚本，先验证计数一致性，再扩展到真实 relay 压测。
6. 后续：补 Redis-only 崩溃恢复，扫描 Redis 里存在但 DB mirror 缺失的 counter key，并评估是否需要操作级幂等补偿队列。

## 当前进展

- DP-CI-001 已完成并在 GitHub Actions 通过：Backend 覆盖 Go 测试、企业治理 smoke 和 whitespace；Frontend 覆盖 typecheck、审批通知 deep link smoke 和 build。
- DP-REL-001 已完成：`v1.3.0` tag 触发 GHCR Docker 发布，Docker workflow run `27858433012` 成功；镜像 digest 为 `sha256:7650bff674c4a2b070197feba382414c47285de0578ddb2749dbbb84996046ac`，发布证据已写入 `docs/data-proxy-release-runbook.md`。
- DP-OAUTH-001 已完成基础交付：新增 HStation OAuth provider、启用配置校验、登录/注册/绑定入口、系统设置页和管理员绑定查看能力。
- DP-OAUTH-002 已完成：新增 `oauth/hstation_test.go` 和 `controller/oauth_test.go`，覆盖 HStation token/userinfo、登录、注册、绑定、当前用户解绑、取消授权、重复绑定、state 错误和 token 错误；前端个人资料页已接入 HStation 解绑；CI Backend 已纳入 `./oauth`。本地已通过 `go test ./model ./controller ./service ./router ./oauth`、`cd web/default && bun run typecheck`、`cd web/default && bun run smoke:approval-notification-links`、`cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build`，提交 `1f6be929` 已推送到 `normojs/main`，GitHub Actions run `27858100588` 全部通过。真实回调地址仍需在预发或 FRP 环境执行并记录到发布证据，不再作为自动化任务阻塞。
- DP-BENCH-001/002 已完成：fusion-benchmark CLI、README、config、fresh/code 数据集、fixtures、测试文件和评估文档已收口入库；新增 `scripts/fusion-benchmark-check.sh`，离线覆盖 config 校验、fresh/code 数据集校验、内置 self-test 和常见密钥模式扫描；CI 新增 `Fusion Benchmark` job 调用该脚本。提交 `a764b6ca` 已推送到 `normojs/main`，GitHub Actions run `27857644905` 的 Frontend、Backend、Fusion Benchmark job 全部通过。
- DP-V14-001 已完成最小可用闭环：新增 SSO 组织同步 payload importer、dry-run preview、冲突列表、apply 事务、`org_sync.apply` 审计事件和 Organization 页同步面板；支持 HStation/OIDC/GitHub 等 provider user id 映射，未冲突行可选择性应用。本地已通过 `go test ./model ./controller ./service ./router ./oauth`、`cd web/default && bun run typecheck`、`cd web/default && bun run smoke:approval-notification-links`、`cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build`。
- DP-V15-001 已完成最小可用闭环：新增默认关闭的 `EnterpriseQuotaRedisCounterEnabled` 配置项；企业额度 reserve 优先走 Redis Lua 原子计数，quota exceeded 保持硬拒绝，Redis/后端异常时降级 DB 原路径；Redis 首次 key 使用 DB counter seed，避免中途启用后从零计数；reserve 成功后同步 DB counter 用于审计、可见性和后续对账；settle/refund 会同步 Redis 和 DB。新增回归覆盖默认关闭配置、Redis reserve/settle/refund、DB seed 和现有并发 DB 限额。
- DP-V15-002 已完成最小可用闭环：新增 `POST /api/enterprise/quota-counters/reconcile`，支持管理员 dry-run 和 repair；新增主节点周期任务，在 `EnterpriseQuotaRedisCounterEnabled` 且 Redis 可用时每 5 分钟对账活跃 DB counter；repair 使用 Redis `SetSnapshot` 幂等修复到 DB 当前快照，并写入 `quota_counter.reconcile` 审计；新增回归覆盖差异发现、修复和审计可见性。Redis-only 崩溃恢复和操作级补偿队列拆到 DP-V15-005。

## 提交和发布规则

- 每条功能线独立提交：CI/发布文档、HStation OAuth、fusion-benchmark、后续企业治理版本不得混在一个 commit。
- 提交前必须确认 `git status --short` 中 staged 文件只属于当前任务。
- 外部通知、OAuth、benchmark 数据都要做敏感信息检查，不提交 token、secret、真实用户邮箱或真实业务 payload。
- 对 new-api 的开源协议要求保持可见合规：保留 `LICENSE`、`NOTICE`、原项目链接和 `Frontend design and development by New API contributors.` 文案。
