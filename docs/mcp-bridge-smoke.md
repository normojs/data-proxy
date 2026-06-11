# MCP Bridge Smoke Testing

This document records the local verification flow for MCP remote tools,
Bridge clients, audit logs, and quota settlement.

## Current Scope

This repository owns the data-proxy side plus local test harnesses for the
Bridge protocol. QidianBrowser real-product client implementation is
intentionally out of scope for this repository phase.

Bridge verification now has two local paths:

- `tools/bridge_client_daemon.mjs`: a real local Bridge client daemon in this
  repository. It connects to `/bridge/ws`, performs local read/write/edit
  operations against a configured workspace, forwards MCP Proxy calls to another
  loopback MCP server, writes a local JSONL audit log, heartbeats, reconnects,
  and limits concurrent tool execution.
- `tools/mcp_bridge_smoke.mjs`: the older sibling QidianBrowser mock smoke,
  still useful for read-only timeout/refund paths that were built before the
  local daemon existed.

The local daemon path is expected to verify:

- Bridge WebSocket registration, heartbeat, session state, and audit logs.
- MCP `tools/call` execution through `RemoteBridgeExecutor`.
- MCP Proxy transports `bridge` and `qidian_browser` through `mcp_proxy.*`
  bridge tool calls.
- Local read, write, edit, tree, glob, grep, and environment info tools.
- Loopback MCP downstream discovery and `tools/call` through Bridge.
- Concurrent `/mcp/v1` calls and persistence in `mcp_tool_calls`,
  `bridge_audit_logs`, and the daemon JSONL audit log.
- Debit/refund consistency for success, `tool_error`, timeout, and
  client-not-found paths through targeted Go tests plus smoke failures.

Write/edit is opt-in with `--enable-write` and remains restricted to the
configured workspace by default. Shell/git/install style tools are not part of
the local daemon.

## Local Prerequisites

Use the local Docker MySQL 8 instance:

```bash
SQL_DSN='root:my-secret-pw@tcp(127.0.0.1:3306)/data-proxy?charset=utf8mb4&parseTime=true&loc=Local'
```

Use the shared Go cache location:

```bash
export GOPATH=/Volumes/fushilu/.caches/gocache/gopath
export GOMODCACHE=/Volumes/fushilu/.caches/gocache/pkg/mod
export GOCACHE=/Volumes/fushilu/.caches/gocache/build
export GOTMPDIR=/Volumes/fushilu/.caches/gocache/tmp
export GOTOOLCHAIN=auto
```

## Migration Smoke

The migration smoke is opt-in for real databases because it touches the
configured SQL database. The SQLite target is safe for local/CI use because it
creates a temporary database file. These targets verify MCP, MCP Proxy,
OpenAPI binary object, Bridge, billing event, and billing relation tables plus
built-in MCP tool seeding.

```bash
make mcp-migration-sqlite
```

Run MySQL or PostgreSQL explicitly when a local database is available:

```bash
make mcp-migration-mysql \
  MCP_MIGRATION_MYSQL_DSN='root:my-secret-pw@tcp(127.0.0.1:3306)/data-proxy?charset=utf8mb4&parseTime=true&loc=Local'
```

```bash
make mcp-migration-postgres \
  MCP_MIGRATION_POSTGRES_DSN='postgres://root:password@127.0.0.1:5432/new-api?sslmode=disable'
```

## Service Smoke

The service smoke covers:

- Bridge client registration/listing/audit log listing.
- MCP tool admin listing/detail/update.
- MCP call persistence on bridge unavailable.
- MCP call success with mock executor.
- MCP remote bridge executor success.
- MCP remote bridge `tool_error` refund path.
- MCP remote bridge timeout refund path.
- Billing precheck failure.
- Settlement idempotency.

```bash
MCP_MIGRATION_TEST=1 \
SQL_DSN="$SQL_DSN" \
go test ./service -run 'TestMCP|TestBridge' -count=1 -v
```

## MCP Proxy OAuth Auth

MCP Proxy supports `auth_type=oauth` for HTTP, SSE, and streamable HTTP
transports. `auth_ref` still uses the existing secret-reference format and
must point to an environment variable:

```bash
export MCP_PROXY_OAUTH_CONFIG='{
  "access_token": "initial-access-token",
  "token_type": "Bearer",
  "refresh_token": "refresh-token",
  "token_url": "https://auth.example.com/oauth/token",
  "client_id": "client-id",
  "client_secret": "client-secret",
  "expires_at": 1760000000
}'
```

Admin server config:

```json
{
  "auth_type": "oauth",
  "auth_ref": "env:MCP_PROXY_OAUTH_CONFIG"
}
```

If the cached access token is expired, data-proxy refreshes it in memory with
the refresh token and reuses the refreshed token for later proxy calls. Refresh
failures are recorded in proxy discovery/health errors without echoing token
values or the secret reference.

## End-to-End Smoke

The E2E smoke starts `new-api`, starts the QidianBrowser mock bridge client,
calls `/mcp/v1` with `tools/call`, then verifies:

- The mock receives `tool_call` and returns `tool_result`.
- `mcp_tool_calls` records success.
- `bridge_audit_logs` records success.
- User and token quota are settled exactly once.
- The mock also advertises and successfully executes `remote_tree`,
  `remote_glob`, `remote_grep`, and `remote_env_info`; these extra calls verify
  call-log/audit-log observability but do not participate in the main quota
  assertion.
- Failure scenarios are covered end to end:
  - no online client -> `BRIDGE_CLIENT_NOT_FOUND`, no bridge audit log, debit + refund
  - mock `tool_error` -> bridge audit `error`, MCP call `error`, debit + refund
  - mock delay past bridge timeout -> bridge audit `timeout`, MCP call `timeout`, debit + refund

```bash
SQL_DSN="$SQL_DSN" \
MCP_GO_CACHE_ROOT=/Volumes/fushilu/.caches/gocache \
node tools/mcp_bridge_smoke.mjs \
  --workspace=/Users/fushilu/workspace/revocloud/data-proxy \
  --file=README.md \
  --timeout=240000
```

The script cleans its smoke user, token, calls, bridge client, sessions, and
audit logs by default. Use `--keep-data` only when manually inspecting rows.
It also temporarily raises `performance_setting.monitor_disk_threshold` to 100
for the local smoke process, then restores the original value during cleanup;
this avoids false failures on developer machines with nearly full system disks.

`MCP_REMOTE_BRIDGE_TIMEOUT_MS` defaults to 500 ms inside the smoke-started
`new-api` process so timeout scenarios complete quickly. Override with
`--bridge-timeout-ms=<milliseconds>` only when debugging timing-sensitive
behavior.

## Local Bridge Daemon

The real local daemon lives in this repository:

```text
tools/bridge_client_daemon.mjs
```

Standalone usage:

```bash
node tools/bridge_client_daemon.mjs \
  --server=ws://127.0.0.1:3000/bridge/ws \
  --token="$DATA_PROXY_API_TOKEN" \
  --workspace=/tmp/data-proxy-bridge-workspace \
  --enable-write \
  --max-results=200 \
  --tree-depth=3 \
  --walk-depth=8 \
  --audit-log=/tmp/data-proxy-bridge-workspace/bridge-daemon-audit.jsonl
```

Offline guard self-test:

```bash
node tools/bridge_client_daemon.mjs --self-test --workspace=/tmp/data-proxy-bridge-self-test
```

The self-test does not require a token or data-proxy connection. It verifies a
local read, write-disabled rejection, and outside-workspace write rejection.

Supported capabilities:

- Always advertised: `remote_read`, `remote_tree`, `remote_glob`,
  `remote_grep`, `remote_env_info`, `mcp_proxy`.
- Advertised with `--enable-write`: `remote_write`, `remote_edit`.
- Smoke-only policy verification can pass `--advertise-disabled-write-tools`
  without `--enable-write` so data-proxy can exercise the daemon's
  `REMOTE_WRITE_DISABLED` error and billing refund path.
- MCP Proxy bridge tools: `mcp_proxy.test`, `mcp_proxy.tools_list`,
  `mcp_proxy.tools_call`.

Default safety boundaries:

- File operations stay inside `--workspace`; `--allow-absolute-path` is needed
  for absolute paths outside the workspace.
- MCP Proxy targets must be `localhost`, `127.0.0.1`, or `::1`;
  `--allow-non-loopback-mcp` is needed for other hosts.
- data-proxy also enforces per-client server policy before forwarding Bridge
  tool calls. The policy can restrict allowed tools, keep write tools disabled
  even when a daemon advertises them, cap result/scan limits, and allowlist MCP
  Proxy targets. Empty policy allows advertised read/proxy tools, denies
  `remote_write`/`remote_edit`, and allows only loopback MCP targets.
- Tool execution is limited by `--max-concurrency`.
- Scan and result sizes are bounded by `--max-results`, `--tree-depth`,
  `--walk-depth`, `--max-result-bytes`, and `--max-scan-file-bytes`; these
  values are returned in `remote_env_info.metadata.limits`.
- The daemon heartbeats and reconnects with exponential backoff unless
  `--no-reconnect` is set.

Admin policy update example:

```bash
curl -X PATCH "$DATA_PROXY_BASE_URL/api/bridge/clients/$BRIDGE_CLIENT_ID?scope=all" \
  -H "Authorization: Bearer $DATA_PROXY_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "policy": {
      "allowed_tools": ["remote_read", "remote_tree", "remote_glob", "remote_grep", "remote_env_info", "mcp_proxy"],
      "allow_write": false,
      "max_result_bytes": 524288,
      "max_scan_file_bytes": 2097152,
      "max_results": 200,
      "tree_depth": 3,
      "walk_depth": 8,
      "mcp_allowed_targets": ["http://127.0.0.1:3001/mcp"]
    }
  }'
```

## Local Daemon Concurrency Smoke

The concurrency smoke starts a local `new-api` process unless `--base-url` is
provided, creates a smoke user/token, starts a loopback MCP HTTP server, starts
the local Bridge daemon, configures an MCP Proxy server with
`transport=qidian_browser`, explicitly opens the smoke client's server policy
for write/proxy target tests, actively closes the first Bridge session to
verify daemon reconnect, verifies that a write-disabled daemon rejects
`remote_write` with `REMOTE_WRITE_DISABLED` and refunds billing, verifies that
non-loopback MCP Proxy targets are rejected by the daemon with
`MCP_PROXY_FORBIDDEN_TARGET`, then
concurrently calls:

- `remote_write`
- `remote_edit`
- `remote_read`
- `remote_glob`
- `remote_grep`
- `remote_tree`
- `<namespace>.echo` through MCP Proxy over Bridge

It also verifies expected failures for an outside-workspace write and a
downstream MCP JSON-RPC error.

```bash
SQL_DSN="$SQL_DSN" \
MCP_GO_CACHE_ROOT=/Volumes/fushilu/.caches/gocache \
make mcp-bridge-smoke
```

Override the defaults with `MCP_BRIDGE_SMOKE_CONCURRENCY`,
`MCP_BRIDGE_SMOKE_ITERATIONS`, `MCP_BRIDGE_SMOKE_TIMEOUT`, or
`MCP_BRIDGE_SMOKE_ARGS`.

For a heavier local pressure run:

```bash
SQL_DSN="$SQL_DSN" \
MCP_GO_CACHE_ROOT=/Volumes/fushilu/.caches/gocache \
make mcp-bridge-stress
```

Override the stress defaults with `MCP_BRIDGE_STRESS_CONCURRENCY`,
`MCP_BRIDGE_STRESS_ITERATIONS`, `MCP_BRIDGE_STRESS_TIMEOUT`, or
`MCP_BRIDGE_STRESS_ARGS`.

The smoke validates the reconnect call and every successful concurrent request
in `mcp_tool_calls` and `bridge_audit_logs`, verifies the negative-path records,
checks the daemon JSONL audit file, verifies structured reconnect events
(`server_close`, `connection_close`, and `reconnect_scheduled`), and cleans its
user, token, proxy server, bridge client, sessions, calls, audit logs, and
temporary workspace by default.

Use `--base-url=http://127.0.0.1:<port>` to run against an already running
data-proxy process. Use `--keep-data` only when manually inspecting rows or the
temporary workspace.

## Lightweight Bridge Check

Use this command for local and CI-safe verification that does not require
MySQL:

```bash
make mcp-bridge-check
```

It runs:

- `node --check` for `tools/bridge_client_daemon.mjs`.
- `node --check` for `tools/bridge_daemon_concurrency_smoke.mjs`.
- The daemon offline self-test against a temporary workspace.
- Targeted Go tests for Bridge, MCP Proxy over Bridge, and remote Bridge
  executor paths.

`.github/workflows/mcp-bridge-check.yml` runs the same lightweight check on PRs
that touch Bridge/MCP-related files and can also be started manually. The real
daemon concurrency smoke remains opt-in because it needs a configured SQL
database.

## QidianBrowser Mock

The temporary client lives in the sibling repository:

```text
/Users/fushilu/workspace/revocloud/QidianBrowser/tools/mock-bridge-client.mjs
```

Legacy mock-backed read-only capabilities:

- `remote_read`
- `remote_tree`
- `remote_glob`
- `remote_grep`
- `remote_env_info`

Intentionally unsupported in the mock:

- `remote_write`
- `remote_edit`
- `remote_exec`
- `remote_git`
- `remote_run_tests`
- `remote_install`

## MCP Proxy Over Bridge

`data-proxy` also supports MCP Proxy servers whose transport is `bridge` or
`qidian_browser`. These transports do not execute local commands on the
data-proxy host. They select an online Bridge client by endpoint and forward
MCP proxy operations through normal `tool_call` messages.

Endpoint forms:

```text
bridge://<client_id>?target=http://127.0.0.1:8765/mcp
<client_id>
```

Use `transport=qidian_browser` with a `bridge://...` endpoint for local smoke
and new configurations. Legacy `qidian_browser://...` endpoints are normalized
by the parser, but `bridge://...` avoids URI scheme compatibility issues with
the underscore.

The client must advertise capability `mcp_proxy` and handle:

- `mcp_proxy.test`
- `mcp_proxy.tools_list`
- `mcp_proxy.tools_call`

`tools_list` should return a Bridge tool result whose metadata contains
`tools`, matching MCP `tools/list` definitions. `tools_call` should return
normal MCP content/metadata/summary fields. Bridge tool errors are preserved in
`bridge_audit_logs`, `mcp_tool_calls.error_code`, and billing refund records.

Write/edit/shell/install tools intentionally remain unsupported in the legacy
mock client until QidianBrowser has a real permission and confirmation model.

Targeted service verification for this bridge-proxy path:

```bash
GOPATH=/Volumes/fushilu/.caches/gocache/gopath \
GOMODCACHE=/Volumes/fushilu/.caches/gocache/pkg/mod \
GOCACHE=/Volumes/fushilu/.caches/gocache/build \
GOTMPDIR=/Volumes/fushilu/.caches/gocache/tmp \
GOTOOLCHAIN=auto \
go test ./service ./pkg/mcp/proxy ./pkg/mcp/executor \
  -run 'TestMCPProxy.*Bridge|TestBridge|TestRemoteBridge|TestMCP.*Bridge' \
  -count=1 \
  -timeout=120s
```

The protocol and product implementation notes are documented in:

```text
/Users/fushilu/workspace/revocloud/QidianBrowser/docs/REMOTE_BRIDGE_CLIENT.md
```

## Frontend Build Note

In the Codex desktop environment, the default `node` can be the bundled Codex
App Node binary. That binary may fail to load Rspack native bindings on macOS
because of code signing Team ID checks. Use the local runtime Node when building
the default frontend:

```bash
/Users/fushilu/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node \
  ./node_modules/.bin/rsbuild build
```
