# Enterprise Governance Temporary Development Queue

日期：2026-07-03

本文档只记录当前企业功能的未完成项和开发顺序。已完成的 MVP/MVP+ 功能不再重复列出。当前约束：不做线上验证、不部署生产；需要预发或生产环境证据的任务先保留在后置队列。

## 开发原则

1. 先做本地可验证、低风险、能补齐运营闭环的功能。
2. 再做需要较多 UI/权限边界的增强。
3. 最后做需要真实外部系统、预发或生产环境的验证项。
4. 每个功能线独立提交，避免混入无关改动。
5. 当前已有 `VERSION` 本地改动，不纳入企业功能提交。

## 当前开发指针

- [ ] EG-TMP-301 SSO 批量冲突确认 UI（或外部连接器）

## 本轮完成记录（2026-07-16）

- [x] EG-TMP-004 项目 owner 选择器
- [x] EG-TMP-005 项目审批历史筛选与详情增强
- [x] EG-TMP-006 即将过期临时额度提醒
- [x] EG-TMP-101 alert → 通知 outbox
- [x] EG-TMP-102 异常检测动作增强
  - 完成内容：anomaly 在已有 policy action 时按 `queue > fallback_model > shared_pool > alert` 编排，不再一律 429；审计写入真实 `orchestration_action`。
- [x] EG-TMP-103 策略影响面预估
  - 完成内容：`GET /quota-policies/:id/impact`；禁用/启停确认弹窗展示 hard-limit / dry-run / action 命中与风险。
- [x] EG-TMP-104 CEL 条件可视化辅助
  - 完成内容：CEL 模式变量 chip 一键插入常用字段。
- [x] EG-TMP-201 replay 覆盖清单
- [x] EG-TMP-202 replay 细粒度可观测
  - 完成内容：admission 增加 priority/replay 字段；审计补 failure_stage、upstream_status、replay_request_id、duration；队列列表展示。
- [x] EG-TMP-203 队列优先级（replay）
  - 完成内容：admission 写入策略 priority；replay 批次 `priority desc` 排序；live 队列仍 FIFO。
- [x] EG-TMP-401 角色管理基础产品化
- [x] EG-TMP-001~003 此前完成项

## 第一批：本地可开发的运营闭环

- [x] EG-TMP-001 审计日志 CSV 导出
- [x] EG-TMP-002 审计长期归档任务设计与最小实现
- [x] EG-TMP-003 项目详情侧栏
- [x] EG-TMP-004 项目 owner 选择器体验增强
- [x] EG-TMP-005 项目专属审批历史筛选与详情增强
- [x] EG-TMP-006 即将过期临时额度提醒

## 第二批：策略和告警增强

- [x] EG-TMP-101 alert 动作接入通知/告警渠道
- [x] EG-TMP-102 异常检测触发后的 fallback/shared_pool 等动作执行增强
- [x] EG-TMP-103 策略影响面预估
- [x] EG-TMP-104 策略条件可视化编辑器增强（变量 chip MVP）

## 第三批：队列和重放增强

- [x] EG-TMP-201 更多 replay endpoint 覆盖清单和测试矩阵
- [x] EG-TMP-202 replay 结果更细粒度可观测
- [x] EG-TMP-203 队列优先级/权重调度（replay 优先；live 仍 FIFO）

## 第四批：SSO 和组织同步增强

- [ ] EG-TMP-301 批量冲突确认 UI
- [ ] EG-TMP-302 部门结构变更安全回滚增强
- [ ] EG-TMP-303 外部身份源定时同步框架
- [ ] EG-TMP-304 LDAP 真实连接器
- [ ] EG-TMP-305 企业微信真实连接器
- [ ] EG-TMP-306 飞书真实连接器
- [ ] EG-TMP-307 钉钉真实连接器
- [ ] EG-TMP-308 Okta 真实连接器

## 第五批：RBAC 和角色产品化

- [x] EG-TMP-401 角色管理 UI 独立化（Members 表内角色分配 + API + last-admin 保护）
- [ ] EG-TMP-402 项目级更细粒度权限边界
- [ ] EG-TMP-403 自定义角色/权限模板

## 第六批：共享池和财务增强

- [ ] EG-TMP-501 共享池周期重置策略
- [ ] EG-TMP-502 共享池借用审批或借用上限
- [ ] EG-TMP-503 成本中心分摊报表增强
- [ ] EG-TMP-504 定时报表/邮件报表

## 第七批：高并发和压测证据

- [ ] EG-TMP-601 操作级幂等补偿队列增强
- [ ] EG-TMP-602 真实 Redis Lua 路径持续压测证据（需真实 Redis 环境）
- [ ] EG-TMP-603 大客户规模压测报告（需压测环境）

## 第八批：需要预发或生产环境的收口

- [ ] EG-TMP-701 当前最新企业全量 preflight 重新执行记录
- [ ] EG-TMP-702 预发 R0-R3 灰度演练证据
- [ ] EG-TMP-703 生产 R0-R3 灰度演练证据
- [ ] EG-TMP-704 生产 email/webhook 真实投递证据
- [ ] EG-TMP-705 HStation OAuth 真实环境验证

## 暂不进入近期主线

- [ ] 多节点企业治理一致性增强
- [ ] 跨节点 SSE
- [ ] 分布式 Tunnel 限流
- [ ] 分布式带宽统计

## 说明（2026-07-16）

本轮优先完成了本地可验证的运营闭环与高价值缺口：owner 选择器、审批项目上下文、alert 通知、角色分配、replay 覆盖文档。

仍未完成且明显依赖更大设计/外部系统的项：

- 策略影响面预估 / CEL 可视化（103/104）
- 队列更细可观测与优先级（202/203）
- SSO 批量确认与真实身份源连接器（301–308）
- 项目级细粒度权限与自定义角色（402/403）
- 共享池/财务报表增强（501–504）
- 幂等补偿与压测证据（601–603）
- 预发/生产证据（701–705）

这些项未强行“做完”，避免半成品进入主线。
