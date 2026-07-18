# 生产固化安全故障切换预设（RetryTimes≥1）

日期：2026-07-19  
生产：`https://dp.app.mbu.ltd`（`sha-5f695ffe`）  
依据：`docs/channel-failover-and-circuit-breaker.md`「安全故障切换预设」

## 结论

**PASS**：生产 `options` 已从「仅 `RetryTimes=0`」升级为文档推荐的安全预设；`data-proxy` 容器日志持续出现 `syncing options from database`，进程无需重启即可在约 60s 内加载。

此前 P1-4 演练结束后把 `RetryTimes` 恢复为 `0`，长期默认关闭跨渠道重试。本轮为**运维固化**，不是再演练一次坏渠道。

## 变更前

`options` 中与 failover 相关的键：

| key | value |
| --- | --- |
| `RetryTimes` | `0` |
| 其余健康/熔断键 | **无行**（走进程默认：`AutomaticDisableChannelEnabled=false` 等） |

## 写入内容（与 UI「Apply safe failover preset」一致）

| key | value |
| --- | --- |
| `RetryTimes` | `1` |
| `AutomaticDisableChannelEnabled` | `true` |
| `AutomaticEnableChannelEnabled` | `true` |
| `AutomaticDisableStatusCodes` | `401` |
| `ChannelHealthTransientStatusCodes` | `408,429,500-599` |
| `ChannelHealthFailureThreshold` | `3` |
| `ChannelHealthFailureWindowMinutes` | `5` |
| `ChannelHealthCooldownMinutes` | `2` |
| `ChannelHealthMaxCooldownMinutes` | `10` |
| `AutomaticRetryStatusCodes` | `408,429,500-599` |

方式：生产 MySQL `options` 表 upsert（host 侧运维通道；密钥未入库）。  
未改渠道、用户、Token；未再插入坏渠道。

## 变更后复验

| 检查 | 结果 |
| --- | --- |
| `SELECT` 上述 10 键 | 与表一致，`RetryTimes=1` |
| 容器 `docker logs` | 连续 `syncing options from database`（约每 60s） |
| `/api/status` / 版本头 | 正常；`x-new-api-version: sha-5f695ffe` |
| 业务数据 | 未改渠道列表 / 用户组 |

## 生效说明

- `common.SyncFrequency` 默认 60s：`model.SyncOptions` 周期 `loadOptionsFromDatabase` → `updateOptionMap`。
- 写库后约 1 分钟内内存 `common.RetryTimes` 等与 DB 对齐；**无需**为本次单独滚动部署。
- 管理端 Monitoring 页若已打开，刷新后应显示 Retry Times = 1 与 Disable on failure 开启。

## 与 P1-4 关系

- P1-4 证明：配置开启后坏渠道可自动切走（证据 `docs/p1-channel-failover-e2e-evidence-2026-07-19.md`）。
- 本记录：把演练时临时打开的 `RetryTimes=1` **长期保留**，并补齐预设中的硬/临时故障与熔断参数。

## 回滚（如需）

```sql
UPDATE options SET value='0' WHERE `key`='RetryTimes';
-- 可选：删除本轮新增的健康/熔断键，或改回 false / 空
```

回滚后等待 ≤60s 同步，或重启 `data-proxy` 容器。

## 备注

- 硬故障状态码仅 `401`；`429/5xx` 走临时熔断，避免把上游抖动写成永久禁渠道。
- 同模型仍需 ≥2 渠道 + 渠道 `auto_ban`，failover 才有备份可选。
- 本机产物路径（生产机 `/tmp`，未提交）：`/tmp/p1-retrytimes-persist/{before,after}.tsv`、`summary.json`。
