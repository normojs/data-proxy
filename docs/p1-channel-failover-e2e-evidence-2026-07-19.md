# P1-4 坏渠道自动 failover 演练证据

日期：2026-07-19  
生产：`https://dp.app.mbu.ltd`（`sha-5f695ffe`）  
另附：本地同模型 failover smoke（不触达生产上游）

## 结论

| 层级 | 结果 |
| --- | --- |
| **本地** `scripts/data-proxy-local-channel-failover-smoke.sh` | **PASS** |
| **生产** 临时坏渠道 + `RetryTimes=1` + 同模型备份 | **PASS** |

生产请求先命中坏渠道（id=21）失败并 `retry_planned=true`，再选中备份渠道（id=18）成功返回；audit 写入 `admin_info.channel_failover` 与 error/consume 双日志。

## A. 本地 smoke（确定性）

命令：

```bash
DATA_PROXY_LOCAL_FAILOVER_OUTPUT=/tmp/p1-e2e/local-failover-smoke.md \
  scripts/data-proxy-local-channel-failover-smoke.sh
```

| Field | Value |
| --- | --- |
| model | `gpt-4o-mini` |
| request_id | `202607182308515219460008268d9d6R0TupOz5` |
| bad_channel_id | `101` |
| backup_channel_id | `102` |
| bad_upstream_hits | 1 |
| backup_upstream_hits | 1 |
| diagnostic_candidate | true |
| failover | selected 101 → failed 502 → selected 102 (excluded 101) |
| user_retry_summary | `upstream_busy_retried` |

## B. 生产演练（可控、可逆）

### 前置与配置

1. 临时 `RetryTimes=1`（演练前 options 为空/0；结束后恢复 `0`）
2. 用户 group 临时切到 `GPT低价`（该组已有备份渠道 id=18 `GPT低价-247` 提供 `gpt-5.4-mini`）
3. 管理 API 创建临时坏渠道（非裸 SQL，确保 `InitChannelCache`）：
   - name=`P1 E2E Bad Failover`
   - id=**21**
   - `base_url=http://127.0.0.1:9`（不可达）
   - models=`gpt-5.4-mini`
   - group=`GPT低价`
   - priority/weight=`1000`（高于备份）
   - auto_ban=1
4. 用临时 API Key 发 chat

### 结果

| 项 | 值 |
| --- | --- |
| HTTP | **200** |
| content | `FAILOVER_OK` |
| request id | `202607182314216659897848268d9d6zVjlw3Sh` |
| error log | type=5，channel_id=**21** |
| consume log | type=2，channel_id=**18** |
| use_channel | `["21","18"]`（consume other） |

`admin_info.channel_failover`（consume log 摘要）：

1. **selected** channel=21 `P1 E2E Bad Failover`，`remaining_retries=1`
2. **failed** channel=21，`status_code=500`，`error_code=do_request_failed`，`retry_planned=true`
3. **selected** channel=18 `GPT低价-247`，`retry_index=1`，`excluded_channel_ids=[21]`

### 清理（已执行）

- 删除临时渠道 id=21 及其 abilities
- `RetryTimes` 恢复为 `0`
- user group 恢复 `default`
- 临时 token 禁用；access_token 清空
- 确认无残留 `P1 E2E Bad Failover` 渠道

## 说明

- 生产默认 `RetryTimes=0` 时**不会**跨渠道重试；演练证明「配置开启后」可自动避开坏渠道。运维上需按 `docs/channel-failover-and-circuit-breaker.md` 将 `RetryTimes>=1` 与安全预设固化，否则默认关闭。
- 裸 SQL 插入渠道不会刷新内存 cache，生产演练必须走 `POST /api/channel/`（或等价会 `InitChannelCache` 的路径）。
- 本地 smoke 已覆盖完整 trace / diagnostic candidate；生产侧以 usage log 的 `channel_failover` + 双 channel_id 为审计证据。

## 与退出标准对应

- [x] 坏渠道在配置开启后可自动避开并有审计  
  - 配置：`RetryTimes=1`  
  - 避开：最终成功走备份渠道 18  
  - 审计：error+consume logs 与 `channel_failover` 事件链  
