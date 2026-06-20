# Data Proxy Post V1.3 TODO

本文档承接 V1.3 通知闭环发布后的剩余工作。排序原则是先保证代码进入 GitHub 后能自动验证，再完成发布链路和当前工作区已有功能线，最后进入更大的企业治理后续版本。

## 当前基线

- V1.3 通知闭环代码已推送到 `normojs/main`，最新提交包含站内通知、审计、outbox、email、webhook、通知偏好、投递日志、手动重试、用户邮件偏好和 new-api attribution 合规补丁。
- new-api 的 `LICENSE`、`NOTICE` 和可见 attribution 链路必须持续保留；所有后续改动都不能破坏 AGPLv3 和 NOTICE Section 7 的要求。
- 当前本地工作区仍有 HStation OAuth 和 fusion-benchmark 两条未提交功能线，提交前需要分别审查、测试、拆分。

## 开发顺序

| 顺序 | ID | 优先级 | 状态 | 任务 | 验收标准 |
| --- | --- | --- | --- | --- | --- |
| 1 | DP-CI-001 | P0 | Done | GitHub Actions 常规 CI | `main` push、PR 和手动触发时运行 Go 测试、企业治理 smoke、前端 typecheck/build、审批通知链接 smoke、artifact/whitespace 检查。 |
| 2 | DP-REL-001 | P0 | In progress | 发布证据和 Docker 链路固化 | 预发/生产 R0-R3 证据模板可填写；Docker tag、构建命令、镜像摘要、回滚 tag 和环境变量清单可追溯。 |
| 3 | DP-OAUTH-001 | P0 | Done | HStation OAuth 功能收口 | 后端 provider、前端登录/绑定/系统设置、错误提示、真实回调地址验证完成；相关改动单独提交，不混入 benchmark。 |
| 4 | DP-OAUTH-002 | P0 | In progress | HStation OAuth 自动化验证 | 覆盖登录、注册、绑定、解绑、取消授权、重复绑定、回调错误；至少补后端单测和前端 typecheck。 |
| 5 | DP-BENCH-001 | P1 | Done | fusion-benchmark 工具收口 | 明确数据文件和 fixtures 是否入库；CLI、README、测试和样例数据可复现，不泄露密钥或真实隐私数据。 |
| 6 | DP-BENCH-002 | P1 | Done | fusion-benchmark CI/文档策略 | 若工具进入主仓库，增加轻量测试命令和文档；若不进入主仓库，迁移到独立仓库或保持未提交。 |
| 7 | DP-V14-001 | P1 | Pending | V1.4 SSO 组织同步方案 | 设计 importer 抽象、增量同步、dry-run、冲突处理 UI、同步审计和回滚边界。 |
| 8 | DP-V15-001 | P2 | Pending | V1.5 高并发和精细额度 | Redis 原子计数、DB/Redis 对账、token 级 hard limit、失败补偿队列和压测脚本。 |
| 9 | DP-V16-001 | P2 | Pending | V1.6 高级策略动作 | 支持 alert、fallback_model、queue、shared_pool 等动作，并保留审计和用户提示。 |
| 10 | DP-V17-001 | P2 | Pending | V1.7 企业治理 RBAC/财务视图 | 企业管理员、部门管理员、财务查看员、审计员、项目管理员的权限边界和回归测试。 |

## 当前开始项：DP-CI-001

本轮先落地 `.github/workflows/ci.yml`，作为后续开发的保护网。CI 设计为两个 job：

- Backend：Go module 下载、artifact 检查、gofmt、`go test ./model ./controller ./service ./router`、企业治理 rollout/controller smoke、`git diff --check`。
- Frontend：Bun 安装、`bun run typecheck`、`bun run smoke:approval-notification-links`、`bun run build`、`git diff --check`。

DP-CI-001 已新增 `.github/workflows/ci.yml`。当前已开始 DP-REL-001：新增 GHCR Docker 发布 workflow 和 Data Proxy release runbook，下一步是在真实发布环境记录 CI run、镜像 digest 和 R0-R3 演练证据。

## 当前进展

- DP-CI-001 已完成并在 GitHub Actions 通过：Backend 覆盖 Go 测试、企业治理 smoke 和 whitespace；Frontend 覆盖 typecheck、审批通知 deep link smoke 和 build。
- DP-REL-001 已开始：新增 `.github/workflows/data-proxy-docker.yml` 和 `docs/data-proxy-release-runbook.md`；真实镜像 digest 和 R0-R3 证据需在正式发版时补充。
- DP-OAUTH-001 已完成基础交付：新增 HStation OAuth provider、启用配置校验、登录/注册/绑定入口、系统设置页和管理员绑定查看能力。
- DP-OAUTH-002 已完成第一批自动化：新增 `oauth/hstation_test.go`，覆盖 token 表单、userinfo 映射和缺少 provider user id 的错误路径；本地已通过 `go test ./controller ./oauth` 和 `cd web/default && bun run typecheck`。真实回调地址验证仍需在预发或 FRP 环境执行并记录。
- DP-BENCH-001/002 已完成：fusion-benchmark CLI、README、config、fresh/code 数据集、fixtures、测试文件和评估文档已收口入库；新增 `scripts/fusion-benchmark-check.sh`，离线覆盖 config 校验、fresh/code 数据集校验、内置 self-test 和常见密钥模式扫描；CI 新增 `Fusion Benchmark` job 调用该脚本。提交 `a764b6ca` 已推送到 `normojs/main`，GitHub Actions run `27857644905` 的 Frontend、Backend、Fusion Benchmark job 全部通过。

## 提交和发布规则

- 每条功能线独立提交：CI/发布文档、HStation OAuth、fusion-benchmark、后续企业治理版本不得混在一个 commit。
- 提交前必须确认 `git status --short` 中 staged 文件只属于当前任务。
- 外部通知、OAuth、benchmark 数据都要做敏感信息检查，不提交 token、secret、真实用户邮箱或真实业务 payload。
- 对 new-api 的开源协议要求保持可见合规：保留 `LICENSE`、`NOTICE`、原项目链接和 `Frontend design and development by New API contributors.` 文案。
