# MCP / Bridge / OpenAPI Operations Runbook

This runbook covers the data-proxy side of MCP tools, Bridge clients, MCP
Proxy, OpenAPI imports, OpenAPI binary objects, billing repair, and dashboard
operations. QidianBrowser real-product client work is out of scope for this
repository; use `tools/bridge_client_daemon.mjs` for local protocol
verification.

## Operating Map

Primary dashboard sections live under the default frontend MCP area:

- Overview: review queue, operations trends, proxy error topN, billing
  anomalies, Bridge online/storage trend signals.
- Tools: built-in, OpenAPI, and proxy-backed MCP tools.
- Proxy Servers and Proxy Tools: discovery, health checks, heartbeat status,
  auth configuration, and per-tool enablement.
- Bridge Clients and Bridge Audit Logs: client status, server-side policy,
  sessions, audit records, and daemon failure paths.
- OpenAPI: import preview/import/diff and binary object cleanup/download
  management.
- Billing Events: ledger health, source coverage, reconciliation repair,
  audit relation preview/selected repair, inspection history, and orphan
  cleanup.

Key backend paths:

- Bridge WebSocket: `/bridge/ws`
- MCP JSON-RPC endpoint: `/mcp/v1`
- Bridge admin: `/api/bridge/clients`, `/api/bridge/audit-logs`
- MCP admin: `/api/mcp/*`
- OpenAPI binary admin/download: `/api/mcp/openapi/binary/*`
- Billing ledger/relation admin: `/api/billing/events/*`

## Command Index

Every command below is backed by an existing Make target or script.

| Command | Source | Use |
| --- | --- | --- |
| `make mcp-regression` | `Makefile:mcp-regression` | Full MCP/OpenAPI/Proxy/Bridge/Dashboard regression. |
| `make mcp-openapi-check` | `Makefile:mcp-openapi-check` | OpenAPI parser/import/binary object service regression. |
| `make mcp-proxy-check` | `Makefile:mcp-proxy-check` | MCP Proxy package, model, and service regression. |
| `make mcp-bridge-check` | `Makefile:mcp-bridge-check` | CI-safe daemon syntax, daemon self-test, and Bridge Go regression. |
| `make mcp-dashboard-check` | `Makefile:mcp-dashboard-check` | MCP route, trend, OpenAPI import summary smoke, and TypeScript build. |
| `make mcp-migration-sqlite` | `Makefile:mcp-migration-sqlite` | Temporary SQLite migration smoke. |
| `make mcp-migration-mysql` | `Makefile:mcp-migration-mysql` | Opt-in MySQL migration smoke using `MCP_MIGRATION_MYSQL_DSN`. |
| `make mcp-migration-postgres` | `Makefile:mcp-migration-postgres` | Opt-in PostgreSQL migration smoke using `MCP_MIGRATION_POSTGRES_DSN`. |
| `make mcp-bridge-smoke` | `Makefile:mcp-bridge-smoke`, `tools/bridge_daemon_concurrency_smoke.mjs` | End-to-end local daemon concurrency smoke. |
| `make mcp-bridge-stress` | `Makefile:mcp-bridge-stress`, `tools/bridge_daemon_concurrency_smoke.mjs` | Heavier local daemon pressure run. |
| `node tools/bridge_client_daemon.mjs --self-test --workspace=<tmp>` | `tools/bridge_client_daemon.mjs` | Offline file-guard self-test without data-proxy. |
| `node tools/bridge_client_daemon.mjs --token=<token> --workspace=<path>` | `tools/bridge_client_daemon.mjs` | Standalone local Bridge daemon. |
| `node tools/mcp_bridge_smoke.mjs` | `tools/mcp_bridge_smoke.mjs` | Legacy mock Bridge smoke for older read-only/refund paths. |

Use `--help` on each Node script to inspect current flags before changing
automation.

## Local Bridge Daemon

Use the daemon in this repo when testing real Bridge behavior locally. It
connects to `/bridge/ws`, advertises file tools, runs heartbeat/reconnect, and
writes optional local JSONL audit records.

Default safety posture:

- Write tools are disabled unless `--enable-write` is set.
- File operations stay inside `--workspace`; absolute outside-workspace access
  requires `--allow-absolute-path`.
- MCP Proxy targets are loopback-only unless `--allow-non-loopback-mcp` is set.
- Concurrent tool execution is capped by `--max-concurrency`.
- Result and scan sizes are capped by `--max-result-bytes`,
  `--max-scan-file-bytes`, `--max-results`, `--tree-depth`, and `--walk-depth`.
- Reconnect is enabled by default; use `--no-reconnect` only when debugging a
  single WebSocket lifecycle.

Minimal local flow:

1. Run the daemon self-test command from the Command Index.
2. Start data-proxy and create a token for the local user.
3. Run the standalone local Bridge daemon command from the Command Index.
4. Verify the client under Bridge Clients and call a built-in remote tool
   through `/mcp/v1` or the dashboard.
5. Run `make mcp-bridge-check` before committing Bridge/daemon changes.

For end-to-end concurrency, policy, billing, reconnect, and MCP Proxy coverage,
prefer `make mcp-bridge-smoke`. Use `make mcp-bridge-stress` only after the
normal smoke is green.

## Production Policies

Bridge has two policy layers:

- Daemon policy: local CLI flags decide what the daemon advertises and what it
  can execute.
- Server policy: the Bridge client policy stored by data-proxy is enforced
  before forwarding any tool call.

Keep production defaults conservative:

- Leave `allow_write=false` unless the client has an explicit approval and
  rollback path.
- Keep `allowed_tools` narrow. Use `mcp_proxy` for the proxy family instead of
  a blanket `*` unless the client is dedicated and trusted.
- Keep `mcp_allowed_targets` empty for loopback-only, or list exact approved
  target URLs. Avoid `*` outside isolated test environments.
- Set result and scan caps for every production client. Treat daemon-reported
  limits as advisory and server policy as authoritative.
- Do not store bearer tokens, OAuth tokens, basic auth passwords, or client
  secrets in database fields. MCP Proxy auth secrets must remain `env:NAME`
  references.

MCP Proxy production defaults:

- Use `auth_type=none`, `bearer`, `basic`, `header`, or `oauth` as supported by
  the downstream server.
- For OAuth, the env var contains JSON credentials. Refreshed access tokens are
  cached in memory only and must not be written to model records, health
  events, discovery events, or logs.
- Run health check/discovery after config changes and inspect Proxy Servers,
  Proxy Tools, and Overview Review Queue.

## OpenAPI Imports And Binary Objects

OpenAPI import rules:

- Preview does not persist tools. Import writes selected MCP tools and mapping
  records.
- Use import preview metrics to check imported tool count, schema count, reused
  schema count, skipped operations, and diff summary before changing production
  tools.
- Do not put provider-specific model/channel behavior in MCP execution code;
  data-proxy's existing channel/relay abstractions own model provider routing.

Binary response rules:

- Non-text responses are stored as OpenAPI binary objects. MCP text output gets
  summary/metadata and a download URL, not the binary body.
- Owner and admin can download active objects. Foreign users and expired
  objects receive the same not-found style result.
- Dashboard binary object cleanup supports dry-run and execute. Scheduled
  cleanup runs only on the master node when `OPENAPI_BINARY_OBJECT_TTL_SECONDS`
  and cleanup interval are configured.

Binary object environment:

- `OPENAPI_BINARY_OBJECT_PROVIDER`: `local` or `s3`.
- `OPENAPI_BINARY_OBJECT_DIR`: local storage directory.
- `OPENAPI_BINARY_OBJECT_TTL_SECONDS`: retention age for cleanup.
- `OPENAPI_BINARY_OBJECT_CLEANUP_INTERVAL_SECONDS`: scheduled cleanup tick.
- `OPENAPI_BINARY_OBJECT_CLEANUP_LIMIT`: maximum objects per cleanup run.
- S3 mode additionally uses endpoint, bucket, region, access key, secret key,
  optional session token, key prefix, and path-style flags from
  `pkg/mcp/openapi/binary_store_s3.go`.

Verify OpenAPI changes with `make mcp-openapi-check`, then `make mcp-regression`
when Bridge/Proxy/Dashboard surfaces are also touched.

## Review Queue

The Overview Review Queue aggregates actionable operations signals:

- `bridge_stale`: Bridge client is marked online in DB but has no live hub
  session.
- `health_check_failed`: latest proxy health check failed.
- `heartbeat_failed`: proxy heartbeat failed.
- `high_error_rate_tool`: proxy-backed tool has high recent errors.
- Existing proxy-server reasons such as server error, schema changed tools,
  transport error, and recent call errors.

Use the queue as a triage entry point. Drill into the target section, then
inspect raw rows in Proxy Servers, Proxy Tools, Bridge Clients, Bridge Audit
Logs, Tool Calls, or Billing Events.

## Billing Repair

Billing repair has two separate surfaces:

- Reconciliation repair/backfill fixes expected ledger events and creates audit
  billing events.
- Audit relation repair fixes missing `billing_event_relations` links between
  repair audit events and their target billing events.

Recommended flow:

1. Open Billing Events and review Billing Health.
2. Use historical backfill/reconciliation review for missing or mismatched
   ledger events.
3. Use Audit Relations Preview to inspect missing relation samples.
4. Select only the relation rows you want to repair and run Repair Selected.
5. Keep broad relation backfill as a fallback for controlled batches.
6. Use orphan cleanup only for broken relation rows whose source or target
   event no longer exists.

Selected relation repair is additive and idempotent. Re-running the same row
should become skipped-existing instead of creating duplicates. It does not
change balances or billing event amounts.

## Error Codes

Common places to inspect errors:

- MCP Tool Calls: `status`, `error_code`, `error_message`, request id, tool id.
- Bridge Audit Logs: Bridge-side status, error code, target client/session.
- Billing Events: debit/refund events, audit repair events, relation links.
- Overview Review Queue: aggregated operational reasons.

Common codes:

| Code | Meaning | First response |
| --- | --- | --- |
| `BRIDGE_CLIENT_NOT_FOUND` | No eligible online Bridge client. | Check client id, transport endpoint, Bridge Clients status, and hub session. |
| `BRIDGE_CLIENT_UNAVAILABLE` | Client exists but cannot accept the call. | Check stale sessions, failover eligibility, and policy/capability match. |
| `BRIDGE_CLIENT_DISCONNECTED` | Session closed while a call was pending. | Confirm daemon reconnect and verify refund/audit rows. |
| `EXECUTOR_TIMEOUT` | Tool execution exceeded timeout. | Check downstream latency, daemon audit, and timeout configuration. |
| `EXECUTOR_FAILED` | Generic executor failure. | Inspect `error_message` and downstream logs. |
| `BRIDGE_POLICY_TOOL_NOT_ALLOWED` | Server policy denied the tool. | Update server policy or use an allowed tool. |
| `BRIDGE_POLICY_WRITE_DISABLED` | Server policy denied write/edit. | Enable write only for approved clients/workspaces. |
| `BRIDGE_POLICY_MCP_TARGET_FORBIDDEN` | Server policy denied the MCP target. | Add exact target allowlist or keep loopback-only. |
| `BRIDGE_POLICY_RESULT_TOO_LARGE` | Result exceeded server policy. | Lower result size or raise policy cap after review. |
| `REMOTE_WRITE_DISABLED` | Daemon advertised write but local write flag is off. | Restart daemon with `--enable-write` only if intended. |
| `REMOTE_WRITE_FORBIDDEN` | Daemon file guard denied the path. | Keep paths inside workspace or review absolute-path policy. |
| `MCP_PROXY_INVALID_TARGET` | Daemon rejected malformed/non-http MCP target. | Fix proxy endpoint target URL. |
| `MCP_PROXY_FORBIDDEN_TARGET` | Daemon rejected non-loopback target. | Use loopback or explicitly allow non-loopback in controlled tests. |
| `MCP_PROXY_HTTP_ERROR` | Downstream MCP HTTP response was non-2xx. | Inspect downstream MCP server. |
| `MCP_PROXY_UPSTREAM_ERROR` | Downstream MCP JSON-RPC error without stable code. | Inspect downstream JSON-RPC error and proxy audit. |
| `MCP_PROXY_UPSTREAM_<code>` | Downstream MCP JSON-RPC error code was preserved. | Debug downstream tool/server behavior. |
| `MCP_PROXY_TOOL_NOT_SUPPORTED` | Daemon received unsupported proxy bridge tool. | Check daemon version/capabilities. |
| `openapi binary object not found` | Missing, expired, unauthorized, or deleted binary object. | Check owner/admin scope, expiry, cleanup history, and registry/storage consistency. |

## Rollback And Cleanup

Bridge rollback:

- Set server policy back to conservative defaults: no write, narrow
  `allowed_tools`, loopback-only MCP targets, and explicit result/scan caps.
- Disable or archive the affected Bridge client or proxy server from the
  dashboard if calls must stop immediately.
- Confirm pending calls resolve as error/timeout and verify debit/refund rows in
  Billing Events.
- Run `make mcp-bridge-check`; run `make mcp-bridge-smoke` before re-enabling a
  changed daemon in production-like environments.

MCP Proxy rollback:

- Disable the proxy server or specific proxy tools.
- Re-run health check/discovery from the dashboard.
- Watch Review Queue for `health_check_failed`, `heartbeat_failed`, and
  high-error tools.
- Run `make mcp-proxy-check` for backend changes and `make mcp-dashboard-check`
  for dashboard changes.

OpenAPI rollback:

- Disable or delete imported tools from the OpenAPI dashboard section.
- For binary object storage pressure, run cleanup dry-run first, then execute
  cleanup with a reviewed TTL/limit.
- If storage and registry disagree, prefer restoring storage or registry from
  backup; expired/foreign downloads intentionally do not reveal object
  existence.
- Run `make mcp-openapi-check`; run `make mcp-regression` after dashboard or
  Bridge/Proxy-adjacent changes.

Billing repair rollback:

- Relation repair is additive and idempotent. Prefer a new corrective audit over
  deleting audit events.
- If a relation source/target is deleted by an external process, use orphan
  cleanup from Billing Events after dry-run review.
- If a ledger event repair was wrong, create a new reconciliation repair with a
  clear reason instead of mutating historical audit metadata by hand.

Smoke data cleanup:

- `make mcp-bridge-smoke`, `make mcp-bridge-stress`, and
  `node tools/mcp_bridge_smoke.mjs` clean their smoke users, tokens, calls,
  sessions, audit logs, proxy servers, and temporary workspaces by default.
- Use each script's `--keep-data` only while actively inspecting rows, then
  clean manually before another run.

## Verification Matrix

- Local docs/command verification: confirm each command in the Command Index
  maps to the listed Make target or script.
- Backend MCP/OpenAPI/Proxy/Bridge changes: `make mcp-regression`.
- Bridge daemon-only changes: `make mcp-bridge-check`; add
  `make mcp-bridge-smoke` for real concurrency/reconnect/policy checks.
- OpenAPI parser/binary changes: `make mcp-openapi-check`.
- Proxy auth/discovery/heartbeat changes: `make mcp-proxy-check`.
- Dashboard changes: `make mcp-dashboard-check`.
- Migration/model changes: `make mcp-migration-sqlite`; add MySQL/PostgreSQL
  targets when DSNs are available.
