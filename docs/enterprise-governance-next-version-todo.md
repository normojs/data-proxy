# Enterprise Governance Next Version TODO

本文档承接企业治理 MVP 之后的开发任务。MVP 当前目标是先完成组织、部门、
策略分组、额度策略、dry-run、hard limit、用量归集、审计和基础管理 UI。
下一版本重点从“能管住”推进到“管得稳、管得细、管得清楚”。

## 当前发布前收口

这些任务优先于新功能，必须先完成再进入下一版本功能开发。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0001 | P0 | 跑企业治理发布前核验脚本 | `scripts/enterprise-governance-preflight.sh` 全部通过 |
| NV-0002 | P0 | 清理发布工作区 | commit 只包含源码、文档、脚本和必要样例；排除 Playwright、日志、本地 DB |
| NV-0003 | P0 | 预发执行 R0-R3 灰度演练 | 记录版本、开关、请求 ID、审计、counter、attribution 和回滚结果 |
| NV-0004 | P0 | 生产执行 R0-R3 灰度演练 | 小范围 hard limit 通过后再扩大到真实部门 |
| NV-0005 | P0 | HStation OAuth 真实环境验证 | 登录、注册、绑定、解绑、回调地址和错误提示全部验证 |
| NV-0006 | P1 | Docker 镜像发布链路固化 | 镜像 tag、构建命令、回滚 tag 和部署记录可追溯 |

## V1.1: 企业治理可运营化

目标：让管理员不只可以配置策略，还能排查、复盘和安全灰度。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0101 | P0 | 审计日志 UI 完整化 | 可按动作、目标、操作人、request_id、时间筛选并查看 before/after |
| NV-0102 | P0 | 拒绝原因解释面板 | 403 能关联策略 ID，管理员 UI 可看到命中策略、额度周期、当前 used/reserved |
| NV-0103 | P0 | dry-run 观测面板 | 展示 would reject 请求、影响用户、影响部门、命中策略和建议阈值 |
| NV-0104 | P1 | 首条策略引导 | 空状态引导创建部门、分配成员、创建策略分组、创建额度策略 |
| NV-0105 | P1 | 策略启停安全确认 | 停用/启用 hard limit 前展示影响范围和最近命中量 |
| NV-0106 | P1 | 报表筛选增强 | 支持模型、渠道、API key、时间粒度筛选 |
| NV-0107 | P1 | 拒绝提示 i18n 分层 | 用户提示保持简洁，管理员日志保留详细策略信息 |

当前进度：

- NV-0001 已完成基础交付：`scripts/enterprise-governance-preflight.sh` 已执行通过，覆盖
  `go test ./model ./controller ./service ./router`、R0-R3 runbook smoke、管理 API/报表
  定向测试、前端 typecheck/build 和 `git diff --check`。脚本增强后已再次执行完整核验通过。
- NV-0002 已完成基础交付：preflight 新增发布工作区 artifact 检查，阻止 Playwright 输出、
  本地 DB、日志、缓存和前端构建产物混入发布候选；`.playwright-cli/` 已加入 `.gitignore`。
  当前脚本支持 `--artifact-check-only`，并同时检查未跟踪、已暂存和已跟踪的发布产物，避免
  `dist`、本地 DB、日志或 Playwright 输出误进入 release commit。

待执行发布收口：

- NV-0003 预发 R0-R3 演练：按 `docs/enterprise-org-quota-rollout-runbook.md` 的真实环境证据模板
  记录环境、版本、开关、请求 ID、审计、counter、attribution、日志脱敏和回滚结果。通过后把证据链接
  或变更单编号补回本文档。
- NV-0004 生产 R0-R3 演练：先执行 R0/R1，只在测试用户或低风险部门执行 R2/R3；R3 第二次请求必须
  证明没有打到上游。生产扩大范围前需要保留 hard-limit 策略快照和回滚人。
- NV-0005 HStation OAuth 真实环境验证：确认回调地址、登录、注册、已有账号绑定、解绑、错误提示、
  取消授权和重复绑定场景；当前 FRP 公网域名为 `https://newapi.tunnel.runna.cc`，优先验证
  bridge-based custom OAuth provider `dc.hhhl.cc`，回调地址为
  `https://newapi.tunnel.runna.cc/oauth/dc.hhhl.cc`；记录 provider 配置、测试账号和失败截图。
- NV-0006 Docker 镜像发布链路：选择镜像部署作为标准路径，源码部署作为临时应急路径；固定 tag 规则、
  build 命令、镜像摘要、环境变量清单、数据库迁移步骤和回滚 tag。

- NV-0101 已完成基础交付：审计日志 UI 支持动作、目标类型、操作人、request_id
  和时间范围筛选；列表可打开详情弹窗，查看事件元信息以及 before/after JSON。
- NV-0102 已完成基础交付：dry-run 与 hard-limit 拒绝都会写入结构化审计 payload；
  管理员详情面板可查看拒绝类型、命中策略、额度周期、limit/used/reserved/requested
  和模型白名单等排查信息。
- NV-0103 已完成基础交付：审计页新增最近 7 天 dry-run 观测面板，展示 would reject
  请求、影响用户、模型、命中策略、用量构成和建议阈值，并可打开完整审计详情。
- NV-0104 已完成基础交付：额度策略空状态新增首条策略引导，按部门、成员、策略分组、
  额度策略四步展示准备状态，并可直接进入创建策略。
- NV-0105 已完成基础交付：禁用额度策略和编辑策略启停状态时新增安全确认弹窗，展示
  目标、周期、当前用量、影响风险和最近 dry-run 命中量。
- NV-0106 已完成基础交付：用量报表支持模型、渠道 ID、API Key ID、状态筛选，聚合维度
  支持渠道、API Key 和时间维度，时间维度支持按日/月粒度查看。
- NV-0107 已完成基础交付：用户侧 hard-limit 拒绝只返回 i18n 简洁提示和错误码，不暴露
  policy_id、额度计数和模型白名单等内部字段；管理员审计 payload 保留 user_message_key、
  error_code、deny_reason 和完整策略命中细节。

## V1.2: 项目和成本中心

目标：把企业用量从“人和部门”扩展到“业务项目、应用、成本中心”。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0201 | P0 | 新增项目/成本中心模型 | 项目属于企业，可绑定部门、负责人和状态 |
| NV-0202 | P0 | API Key 绑定默认项目 | 请求可继承 key 的默认项目归属 |
| NV-0203 | P0 | 请求传入项目 ID | 支持 header 或 metadata 指定项目，并校验用户可用范围 |
| NV-0204 | P0 | 项目维度额度策略 | 策略 target 支持 project，能 dry-run 和 hard limit |
| NV-0205 | P1 | 项目用量报表 | 按项目、部门、模型、渠道展示请求数、quota、token、成本 |
| NV-0206 | P1 | 项目归属审计 | 项目创建、成员/部门绑定、key 绑定和策略变更可审计 |

当前进度：

- NV-0201 已完成基础后端交付：新增 `enterprise_projects` 和 `enterprise_project_org_units`
  数据模型，支持项目归属企业、绑定部门、负责人、状态、创建/更新/停用、列表筛选、审计记录；项目
  已加入 quota policy target 基础枚举和校验。已补充 controller 定向测试。
- NV-0201 前端已接入项目/成本中心管理页签：支持列表、关键词/状态/部门筛选、创建、编辑、绑定部门、
  负责人、停用确认和分页。
- NV-0202 已完成基础交付：`tokens` 新增 `default_project_id`，API Key 创建/更新支持绑定默认项目；
  普通用户只能选择自己所属部门可用的启用项目；新增 `/api/token/enterprise-projects` 供 API Key 表单
  获取可选项目；前端 API Key 表单已支持默认项目下拉。
- NV-0203 已完成基础交付：relay 请求会优先读取可选 `X-Data-Proxy-Project-ID`，否则继承 API Key
  的 `default_project_id`；项目必须启用且绑定到用户部门路径，非法或越权项目返回 403，不进入上游。
- NV-0204 已完成基础 relay 决策：quota policy target 支持 `project`，项目策略参与匹配、dry-run、
  hard-limit 预留/结算，CEL 条件可读取 `org.project_id`。
- NV-0205 已完成基础交付：成功请求归集写入 `project_id`，用量 summary 支持 `project_id` 过滤，
  breakdown 支持 `project` 维度；前端统计页支持项目维度、项目筛选，并可从项目列表一键钻取到
  对应用量视图。
- NV-0206 已完成基础审计：项目创建/更新/停用已有审计；API Key 默认项目创建和变更会写入
  `token.default_project.update` 企业审计；项目策略变更沿用 quota policy 审计。
- 下一步优先级：进入 V1.3 审批和临时额度；同时可补充项目详情侧栏、项目 owner 选择器和更细的
  审计筛选体验。

## V1.3: 审批和临时额度

目标：超额后不是只有拒绝，也可以走受控放行。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0301 | P0 | 超额申请模型 | 记录申请人、目标、额度、周期、原因、过期时间和状态 |
| NV-0302 | P0 | 审批流 API | 支持提交、批准、拒绝、撤回、过期 |
| NV-0303 | P0 | 临时额度包 | hard limit 计算时叠加未过期的已批准额度 |
| NV-0304 | P1 | 审批 UI | 用户可申请，管理员可审批和查看历史 |
| NV-0305 | P1 | 通知接入 | 申请、批准、拒绝和即将过期可通知相关人 |
| NV-0306 | P1 | 审批审计 | 全流程可追踪，不允许无记录修改额度 |

当前进度：

- NV-0301 已完成基础模型：新增 `enterprise_quota_requests`，记录申请人、审批人、policy、目标、指标、
  周期、临时额度、原因、决策原因、生效/过期时间和状态。
- NV-0302 已完成基础 API：支持提交、列表、批准、拒绝、撤回；提交以 quota policy 为锚点继承目标、
  指标和周期，避免临时额度与策略口径不一致。
- NV-0303 已完成后端硬限制叠加：dry-run 检查和 hard-limit 预留都会把当前周期内已批准、已生效、未过期
  的临时额度叠加到有效上限；过期临时额度不生效。
- NV-0304 已完成基础 UI：企业治理新增 Quota Requests 页签，支持状态/策略筛选、提交临时额度申请、
  管理员批准/拒绝、待审批撤回和分页列表；Dry-run Observations 和 hard-limit 审计详情支持从超额审计
  一键预填临时额度申请。
- NV-0304 已完成基础角色边界：普通登录用户可提交、查看和撤回自己的临时额度申请；管理员可查看全部申请
  并批准/拒绝；前端列表会按角色隐藏审批按钮。
- NV-0304 已补普通用户侧入口：新增 `/quota-requests` 页面和 Console 侧边栏入口，员工可查看自己的申请、
  按状态筛选、提交临时额度申请并撤回待审批申请。
- NV-0304 已补可申请策略：新增 `/api/enterprise/quota-requests/policies`，普通用户提交申请时从当前可命中
  的启用计数策略中选择；后端提交校验复用同一范围，避免手输 policy ID 越权申请。
- NV-0304 已补项目策略申请上下文：可申请策略接口和提交申请支持可选 `project_id`，员工在申请页选择
  可用项目后，项目 target 策略会进入可申请列表；后端提交校验按同一项目上下文防止越权。
- NV-0306 已完成基础审计：提交、批准、拒绝、撤回和审批时发现过期都会写企业审计。
- 下一步优先级：审批通知、即将过期提醒，以及项目专属申请的历史筛选和详情展示。

## V1.4: SSO 组织同步

目标：从企业身份源同步部门和成员，减少手工维护。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0401 | P0 | 组织同步接口抽象 | LDAP、企业微信、飞书、钉钉、Okta 可复用同一套 importer |
| NV-0402 | P0 | 增量同步和 dry-run | 同步前可预览新增、移动、停用、冲突 |
| NV-0403 | P0 | 离职和禁用策略 | 离职用户可停用成员关系、key 或只移出策略分组 |
| NV-0404 | P1 | 冲突处理 UI | 邮箱/用户名/外部 ID 冲突可人工确认 |
| NV-0405 | P1 | 同步审计和回滚 | 每次同步有批次号，可查看变更并回滚安全子集 |

## V1.5: 高并发和精细额度

目标：提高计数性能，并从 request/quota 扩展到 token 和更多指标。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0501 | P0 | Redis 原子计数方案 | 高并发下 request_count 和 quota 预占不明显突破上限 |
| NV-0502 | P0 | DB/Redis 对账任务 | Redis 计数可周期落库，异常可修复 |
| NV-0503 | P1 | token 级硬限制 | 基于 attribution 数据校准后支持 input/output/total token 限制 |
| NV-0504 | P1 | 失败补偿队列 | 上游失败、结算失败、进程中断后的 reservation 可补偿 |
| NV-0505 | P1 | 大客户压测脚本 | 覆盖并发、streaming、失败回滚、dry-run 和 hard limit |

## V1.6: 高级策略动作

目标：策略命中后不只拒绝，还能降级、排队、告警或走共享池。

| ID | 优先级 | 任务 | 验收 |
| --- | --- | --- | --- |
| NV-0601 | P1 | 策略 action 扩展 | MVP 已支持 reject、alert、fallback_model、queue、shared_pool 的配置、命中观测、响应提示和审计；unknown action 保守按 reject 处理 |
| NV-0602 | P1 | 模型自动降级 | 已完成基础交付：fallback_model 命中后会改写 relay 模型、JSON 请求体、重选渠道并按降级模型重新估算预扣费；审计和响应 header 保留降级提示 |
| NV-0603 | P1 | 低优先级排队 | 已完成基础交付：queue 命中后进入企业维度同步 admission queue，先写入 `queued`，拿到队列槽后更新为 `admitted` 并继续 relay，请求结束释放后更新为 `released` 并记录运行耗时；admission 会保存 method、path、query、content type、模型、relay mode、channel id 和最多 32 KiB 的请求体快照，超限时标记 `body_truncated`，且不保存 Authorization/API key 等敏感 header；等待超时返回 429 并更新为 `timeout`，排队阶段请求取消或管理员取消更新为 `canceled`；队列审计、响应 header、管理员取消接口、主节点 stale admission 后台恢复和 `enterprise_governance_queue_admissions` 持久化生命周期记录已可在企业治理 API 和审计页按状态、请求 ID、模型、策略、项目和日期范围分页查看；真正的异步 relay 执行、后台重试、大 payload 重放和后台延迟执行仍保留为后续增强 |
| NV-0604 | P2 | 企业共享池 | 已完成 MVP+：shared_pool 配额超限命中后计算本次请求实际借用量，写入独立池状态 `enterprise_governance_shared_pools` 和借用归属 `enterprise_governance_shared_pool_borrows`；容量不足会在预扣费前阻断并审计；成功借用写入 borrowed/remaining header，结算按实际用量归还未使用借用量，失败或预扣费错误全量退款，并记录 reserve/settle/refund 审计；支持按 `enterprise + policy + metric` 独立配置池容量，reserve 优先使用启用配置，未配置时回退策略 limit；企业治理 API 与审计页可查看池容量状态、容量配置、借用流水和趋势摘要，并支持过滤分页 |
| NV-0605 | P2 | 异常检测后自动限流 | 已完成基础交付：基于企业最近窗口和基线窗口检测请求突增、quota 成本突增，以及 consume/error 日志中的异常失败率；命中后进入短时保护，返回 429、写入异常响应 header 和 `enterprise_governance.anomaly_throttle` 审计；保护状态已写入 `enterprise_governance_anomaly_protections` 并可在进程重启后恢复；保护 key 和检测窗口已按 project、org_unit、enterprise 优先级收敛，避免单项目/单部门异常扩大为全企业保护；企业设置支持配置启用状态、窗口、冷却时间、请求/成本突增阈值和失败率阈值；dry-run 只记录 would-throttle 观测；企业治理 API 和 Audit 页可按 scope 查看保护记录和趋势摘要。项目/部门动作编排仍保留为后续增强 |

## V1.7: 多级管理员和财务视图

目标：让企业治理适合真实组织分权。

| ID | 优先级 | 状态 | 任务 | 验收 |
| --- | --- | --- | --- | --- |
| NV-0701 | P0 | Done (MVP) | 企业治理 RBAC 模型 | 已支持企业管理员、部门管理员、财务查看员、审计员、项目管理员的基础 capability 映射；系统 `admin/root` 保持全权限兼容 |
| NV-0702 | P0 | Done (MVP+) | 部门管理员权限边界 | 已支持按主部门展开本部门及子部门 scope，限制成员列表/归属更新、策略组、额度策略、临时额度审批、审批通知和用量报表；项目级细粒度边界仍待增强 |
| NV-0703 | P1 | Done (MVP+) | 财务视图 | 已支持 `finance_viewer` 只读访问全局 usage summary/breakdown；`project_admin` 按 owner/admin/member 项目 scope 查看/导出项目用量；财务视图支持按当前筛选导出 CSV |
| NV-0704 | P1 | Done (MVP+) | 审计员视图 | 已支持 `auditor` 只读访问审计日志、通知 outbox 和 worker metrics，不能修改配置；部门管理员和项目 read/admin scope 可查看 scope 内审计日志，notification outbox 仍保持全局审计员可见 |
| NV-0705 | P1 | Done (MVP+) | 权限回归测试 | 已覆盖财务/审计只读边界、企业管理员管理能力、普通用户隔离、部门管理员跨部门越权、部门策略组边界、策略组成员角色、跨部门共享策略组、部门用量过滤、scoped 审批边界、项目管理员跨项目越权、项目 member 只读边界、空项目 scope 防泄漏、CSV 导出和 scoped 审计可见性 |
| NV-0706 | P1 | Done (MVP+) | 策略组共享有效期 | 策略组共享支持 `shared_expires_at`；`0` 表示永久有效，过期共享不再进入共享部门可见范围，也不能作为共享部门额度策略目标 |
| NV-0707 | P1 | Done (MVP) | 跨部门策略组共享审批流 | 策略组归属部门可发起共享申请，目标部门或企业管理员可批准/拒绝；批准后复用既有共享表生效，申请和审批写入企业审计，发起部门和目标部门均可 scoped 可见 |
| NV-0708 | P1 | Done (MVP) | 跨部门共享角色权限 | 策略组共享和共享审批申请支持 `viewer/editor`；共享部门 `viewer` 可查看共享策略组并作为额度策略目标，`editor` 才可维护本部门 scope 内成员；旧共享默认 `editor` 兼容既有行为 |

当前 V1.7 已交付最小可用 RBAC 闭环：后端企业治理 API 改为 capability 分组鉴权，前端入口和页签按 `/api/user/self` 的 `permissions.enterprise_governance` 控制；审批、财务和审计入口可分别授权；部门管理员按本部门及子部门 scope 管理成员、部门 scoped 策略组、共享策略组、额度策略、审批、用量和审计日志；策略组成员支持 `viewer/editor` 角色；跨部门共享策略组支持有效期控制、共享申请审批和共享 viewer/editor 角色权限；项目管理员按 owner 或项目 admin 成员 scope 管理项目，项目 member 成员仅能查看 scope 内项目、成员、用量和审计；财务视图支持按筛选导出 CSV。V1.6 shared_pool 持久化增强已补齐独立池状态、借用归属、容量不足阻断、结算/退款归还、池状态/借用流水可见性、独立容量配置和趋势摘要；queue 已补齐异常遗留 admission 的后台恢复审计和 bounded 请求载荷持久化；异常检测已补齐 scope-aware 保护记录和趋势摘要可见性，保护 key 优先按项目、部门、企业收敛。下一步可继续做真正异步 relay 执行，或在 scoped anomaly protection 基础上补项目/部门动作编排。

## 推荐排期

1. 第一批：NV-0001 到 NV-0006，完成发布收口。
2. 第二批：V1.1 可运营化，补齐审计 UI、拒绝解释和 dry-run 观测。
3. 第三批：V1.2 项目/成本中心，把归属从人扩展到业务。
4. 第四批：V1.3 审批和临时额度，降低硬拒绝带来的业务中断。
5. 第五批：V1.4 SSO 同步和 V1.7 RBAC，适配真实企业组织。
6. 第六批：V1.5 高并发和 V1.6 高级动作，面向规模化和精细运营。

## 下一步开发目标

- 短期目标：完成发布前收口和 V1.1，让企业治理可以安全上线、可观测、可排查。
- 中期目标：完成项目/成本中心和审批，让企业内部成本归属和临时放行闭环。
- 长期目标：完成 SSO、RBAC、Redis 计数和高级策略动作，让 Data Proxy 成为企业级模型流量治理平面。
