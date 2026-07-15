# Enterprise Governance Queue Replay Coverage Matrix

日期：2026-07-16

对应任务：EG-TMP-201

## 当前已覆盖 endpoint

实现位置：`service/enterprise_governance_queue_replay.go` → `isEnterpriseGovernanceQueueReplayPathSupported`

| Path | Content-Type | Payload 存储 | 风险 | 优先级 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `/v1/chat/completions` | `application/json` | DB / object payload | 中（可能有副作用重放） | P0 | 已支持 |
| `/v1/completions` | `application/json` | DB / object payload | 中 | P0 | 已支持 |
| `/v1/messages` | `application/json` | DB / object payload | 中 | P0 | 已支持 |
| `/v1/responses` | `application/json` | DB / object payload | 中 | P0 | 已支持 |
| `/v1/responses/compact` | `application/json` | DB / object payload | 中 | P1 | 已支持 |
| `/v1/embeddings` | `application/json` | DB / object payload | 低 | P0 | 已支持 |
| `/v1/rerank` | `application/json` | DB / object payload | 低 | P1 | 已支持 |
| `/v1/moderations` | `application/json` | DB / object payload | 低 | P1 | 已支持 |
| `/v1/edits` | `application/json` | DB / object payload | 中 | P2 | 已支持 |
| `/v1/images/generations` | `application/json` | DB / object payload | 中 | P1 | 已支持 |
| `/v1/images/edits` | `application/json` 或 durable multipart | DB / object payload | 高（multipart 依赖 durable body） | P1 | 已支持 |
| `/v1/audio/transcriptions` | durable multipart | DB / object payload | 高 | P1 | 已支持 |
| `/v1/audio/translations` | durable multipart | DB / object payload | 高 | P1 | 已支持 |
| `/v1/audio/speech` | `application/json` | DB / object payload | 中 | P1 | 已支持 |
| `/v1beta/models/*` | provider-specific | DB / object payload | 中 | P1 | 前缀支持 |
| `/v1beta/openai/models/*` | provider-specific | DB / object payload | 中 | P1 | 前缀支持 |

## Content-Type 规则

`isEnterpriseGovernanceQueueReplayContentTypeSupported`：

- 空 content-type：允许
- `application/json`、`application/x-ndjson`、`text/json`、`*+json`：允许
- `multipart/form-data`：仅当 durable body 且 boundary 存在时允许
- 其他类型：拒绝

## 暂不覆盖 / 暂缓

| Path / 能力 | 原因 | 后续优先级 |
| --- | --- | --- |
| `/v1/files` 上传下载 | 非幂等、大对象、权限边界复杂 | 后置 |
| Realtime / WebSocket | 非 HTTP 可重放请求 | 不做 |
| 任意自定义代理 path | 风险不可控 | 后置 |
| 无 durable body 的 multipart | 原始文件体不可恢复 | 保持拒绝 |

## 测试现状

- 队列 replay 主路径测试集中在 `service/enterprise_governance_queue_test.go`
- 覆盖成功重放、失败、cancel/retry、content-type 与 path 校验

## EG-TMP-202 / 203 状态（2026-07-16）

### EG-TMP-202 已完成最小实现

- admission 新增：`last_replay_status_code`、`last_replay_duration_ms`、`last_failure_stage`、`last_replay_request_id`
- replay 审计 `replay` map 增加：`failure_stage`、`upstream_status`、`duration_ms`、`replay_request_id`
- failure_stage 映射：`payload_missing` / `payload_truncated` / `payload_unsupported` / `token_invalid` / `upstream`
- 队列列表 UI 展示 priority 与最近 replay 摘要

### EG-TMP-203 已完成最小实现

- admission 写入策略 `priority`
- replay 批次排序：`priority desc, next_retry_at asc, created_at asc, id asc`
- live 进程内排队仍为 FIFO；跨进程优先级调度未做
