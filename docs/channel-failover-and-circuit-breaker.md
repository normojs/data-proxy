# Channel Failover and Circuit Breaker

## 10-minute 傻瓜配置（推荐先做）

1. 打开 `System Settings` → `Monitoring & Alerts`
2. 点击 **Apply safe failover preset**（安全故障切换预设）
3. 保存（Save monitoring rules）
4. 为同一模型配置至少 2 个渠道（不同上游/Key），并开启渠道 `auto_ban`
5. 用同一模型连续请求；坏渠道会先被临时熔断/跳过，硬故障可自动禁用
6. 渠道列表可看 Temporary circuit / Degraded / Auto-disabled，并可手动恢复

预设内容（与下文 Recommended starting point 一致）：

```text
RetryTimes=1
Disable on failure = on
Re-enable on success = on
Hard status codes=401
Transient status codes=408,429,500-599
Failure threshold=3
Failure window=5
Cooldown=2
Max cooldown=10
```

主开关默认可关；应用预设会打开「Disable on failure」，请确认后再保存。

生产 `https://dp.app.mbu.ltd`（`sha-5f695ffe`）已于 2026-07-19 按上表写入 `options` 并经 `SyncOptions` 加载；证据见 `docs/p1-retrytimes-persist-evidence-2026-07-19.md`。新环境仍建议在 Monitoring 页点一次预设并保存。

---

Data Proxy supports single-node channel failover for channels that serve the same model. The feature is controlled by global retry settings, channel auto-disable settings, and runtime temporary circuit breaking.

## How failover works

1. The first request selects an enabled channel by group, model, priority, and weight.
2. If the selected channel fails with a retryable error, the failed channel is excluded from the next selection in the same request.
3. The retry selection starts from the highest available priority again, so a same-model backup channel at the same priority can take over immediately.
4. Temporary circuit-broken channels are skipped for later requests until the cooldown expires or an admin clears the runtime state.

`RetryTimes` is the number of extra attempts after the first selected channel fails. Set it to at least `1` to allow one backup channel to be tried.

## Fault classes

Hard faults disable the channel when channel auto-disable is enabled and the channel has auto-ban enabled. These should be used for errors that are not expected to recover quickly, such as invalid credentials or account-level permission problems.

Temporary faults do not hard-disable the channel. They are counted in memory and can open a temporary circuit after repeated failures. These should be used for timeouts, connection problems, rate limits, and upstream 5xx outages.

Other retryable faults only affect the current request retry chain. They do not disable the channel and do not count toward the temporary circuit unless they match the temporary rules.

## Configuration

Admin UI path:

`System Settings` -> `Operations` -> `Monitoring & Alerts`

Relevant options:

- `Retry Times`: Extra attempts after the first channel fails. Use `1` or higher for backup failover.
- `Disable on failure`: Enables hard auto-disable and temporary circuit tracking.
- `Auto-disable status codes`: HTTP status codes/ranges treated as hard faults.
- `Failure keywords`: Error text treated as hard faults.
- `Transient failure status codes`: HTTP status codes/ranges counted as temporary faults.
- `Transient failure keywords`: Error text counted as temporary faults.
- `Failure threshold`: Consecutive temporary failures needed to open the circuit.
- `Failure window (minutes)`: Rolling window for the consecutive failure counter.
- `Cooldown (minutes)`: Initial temporary skip duration after the threshold is hit.
- `Max cooldown (minutes)`: Cap for progressive cooldown backoff.

Recommended starting point:

```text
RetryTimes=1
Auto-disable status codes=401
Transient failure status codes=408,429,500-599
Failure threshold=3
Failure window=5
Cooldown=2
Max cooldown=10
```

Avoid putting broad `5xx` ranges into hard auto-disable rules unless the provider reliably uses those status codes for permanent account errors. For most LLM providers, `429` and `5xx` are better treated as temporary faults.

## Operations

The channel list shows runtime health badges:

- `Temporary circuit`: The channel is currently skipped.
- `Degraded`: The channel has recent temporary failures but has not reached the circuit threshold.

Admins can clear runtime state from the channel row menu with `Clear temporary circuit`.

Runtime health APIs:

```http
GET /api/channel/:id/health
POST /api/channel/:id/health/reset
```

Request trace:

- Usage Logs detail and request trace include admin-only
  `admin_info.channel_failover` when the request selected or failed over
  channels.
- Each event records the selected channel, retry index, remaining retries,
  excluded channel ids, failure status/error code, retry decision, and runtime
  health/circuit action when available.
- User self traces still redact `admin_info`, so channel names, ids, and
  operator-only routing diagnostics are not exposed to ordinary users.

Temporary circuit state is in-memory and single-node only. A process restart clears runtime state. Hard auto-disabled channel status is persisted in the database.

## Local smoke

Use `scripts/data-proxy-local-channel-failover-smoke.sh` to simulate a
production-style same-model failover on the developer machine without touching
production services.

The local smoke creates only temporary resources:

- a temporary SQLite database;
- a temporary admin user and API key;
- two same-model OpenAI-compatible channels;
- one local bad upstream returning HTTP 502;
- one local backup upstream returning a valid Chat Completions response.

It passes only when:

- the client receives the backup response;
- the bad upstream is hit exactly once;
- the backup upstream is hit exactly once;
- request trace contains `admin_info.channel_failover`;
- the failed event has `retry_planned=true` and `status_code=502`;
- the retry-selected event uses the backup channel with `retry_index > 0`;
- diagnostic candidates include the request id with `channel_failover` source.

Example:

```bash
scripts/data-proxy-local-channel-failover-smoke.sh
```

Optional summary output:

```bash
DATA_PROXY_LOCAL_FAILOVER_OUTPUT=/tmp/data-proxy-local-failover-smoke.md \
scripts/data-proxy-local-channel-failover-smoke.sh
```

This smoke disables Redis and data export inside the test process and restores
the runtime globals before exiting. It does not use production API keys,
production Redis, or production database connections.

## Production smoke

Use `scripts/data-proxy-channel-failover-smoke.sh` to prove the behavior without
mutating production channel configuration. The script only sends a normal Chat
Completions request and reads admin request trace / diagnostic candidate APIs.

Before running it, prepare disposable test channels in the admin console:

- two enabled channels serve the same test model name;
- the first channel returns a temporary fault such as 502/503/429 or timeout;
- the second channel returns successfully;
- `RetryTimes >= 1`;
- temporary fault rules include the failing status code or keyword;
- broad 5xx errors are not configured as hard auto-disable rules for this test.

Example:

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_FAILOVER_MODEL='deepseek-ai/DeepSeek-V4-Flash' \
DATA_PROXY_ADMIN_HEADER='Cookie: session=...' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_FAILOVER_EXPECT_FAILED_STATUS_CODE=502 \
scripts/data-proxy-channel-failover-smoke.sh
```

The smoke passes only when the request has a successful consume log, request
trace shows a failed selected channel with `retry_planned=true`, a later
`selected` event with `retry_index > 0`, at least two distinct selected channel
ids, and a failover diagnostic candidate for the same request id.
