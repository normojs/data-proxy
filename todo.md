# data-proxy MCP / Bridge TODO

## P0 - Bridge daemon verification

- [x] Expand the real Bridge daemon concurrency smoke to cover `remote_edit` and `remote_glob`.
  - Acceptance: `make mcp-bridge-smoke` exercises write/read/edit/glob/grep/tree/MCP proxy calls through `/mcp/v1`.
  - Acceptance: every added call is persisted in `mcp_tool_calls`, `bridge_audit_logs`, and the local daemon JSONL audit.
- [x] Add daemon negative-path smoke coverage for write-disabled clients.
  - Acceptance: a daemon started without `--enable-write` rejects `remote_write` with `REMOTE_WRITE_DISABLED`.
  - Acceptance: server records error status and refund path for the failed MCP call.
- [x] Add daemon MCP target policy smoke coverage.
  - Acceptance: non-loopback MCP targets are rejected by default with `MCP_PROXY_FORBIDDEN_TARGET`.
  - Acceptance: policy relaxation through `--allow-non-loopback-mcp` is documented but not enabled by default in smoke.

## P1 - Bridge daemon hardening

- [x] Add a lightweight daemon self-test mode that exercises file guards without connecting to data-proxy.
  - Acceptance: `node tools/bridge_client_daemon.mjs --self-test --workspace=<tmp>` exits 0 and covers path traversal/write-disabled checks.
- [x] Add structured reconnect counters to daemon local audit events.
  - Acceptance: audit JSONL includes reconnect attempt, delay, open/clean-close status, and server-close reason.
- [x] Add configurable scan/result limits to daemon CLI.
  - Acceptance: limits can be set by flags and are reflected in `remote_env_info` metadata.

## P1 - MCP Proxy / OpenAPI robustness

- [x] Add MCP Proxy bridge tests for session replacement while a call is pending.
  - Acceptance: pending calls fail with `BRIDGE_CLIENT_DISCONNECTED` or timeout consistently and refund billing.
- [x] Add OpenAPI binary object authorization regression tests for expired/foreign download links.
  - Acceptance: owner/admin can download valid links; other users and expired links are rejected.
- [x] Add schema de-duplication metrics to OpenAPI import preview.
  - Acceptance: preview shows reused schema count and imported tool count.

## P2 - Operations Dashboard polish

- [x] Add a compact dashboard smoke route test for MCP navigation sections.
  - Acceptance: generated route tree includes MCP index and section routes.
- [x] Add trend panel empty-state and partial-data handling tests.
  - Acceptance: no runtime error when trend endpoints return empty arrays.

## P0 - Regression and release guardrails

- [x] Add a unified MCP regression Make target.
  - Acceptance: `make mcp-regression` runs OpenAPI, MCP Proxy, Bridge daemon, and MCP Dashboard checks.
  - Acceptance: failing sections are split into reusable targets for quick diagnosis.
- [ ] Add cross-database MCP migration regression documentation and opt-in targets.
  - Acceptance: SQLite default, MySQL opt-in, and PostgreSQL opt-in commands cover MCP, Bridge, billing event, and OpenAPI binary object tables.

## P1 - Core capability expansion

- [ ] Add server-side Bridge client policy controls.
  - Acceptance: admins can configure allowed tools, write permission, max result size, scan limits, and MCP target allowlist per Bridge client.
  - Acceptance: daemon defaults remain conservative and server-side policy is enforced before tool forwarding.
- [ ] Implement MCP Proxy OAuth authentication support.
  - Acceptance: MCP Proxy supports OAuth token resolve/cache/refresh without regressing none/bearer/basic/header auth.
  - Acceptance: auth failures are observable in discovery events and health checks without leaking secrets.
- [ ] Surface OpenAPI import schema metrics and diff summary in the MCP Dashboard.
  - Acceptance: import preview displays importable tool count, schema count, reused schema count, skipped reasons, and diff summary.
- [ ] Add OpenAPI binary object management to the MCP Dashboard.
  - Acceptance: admins can view binary object counts, bytes, expiry state, cleanup dry-run, cleanup execute, and download audit context.

## P1 - Reliability and observability

- [ ] Add MCP tool call idempotency and replay protection.
  - Acceptance: repeated client request IDs do not double-charge or double-settle a tool call.
- [ ] Add Bridge multi-client selection and failover.
  - Acceptance: Bridge MCP Proxy can choose by latest activity/capability and fail over to another eligible online client.
- [ ] Add MCP operations review queue.
  - Acceptance: health check, heartbeat, stale Bridge clients, and high-error tools produce actionable review reasons in dashboard summaries.

## P2 - Operations polish

- [ ] Expand MCP Overview with Bridge and OpenAPI storage trends.
  - Acceptance: overview shows Bridge online trend, Proxy error topN, OpenAPI binary storage trend, and refund/settlement anomaly summary.
- [ ] Improve billing relation repair UX.
  - Acceptance: admins can preview relation diffs and repair selected items without running a broad backfill.
- [ ] Publish MCP/Bridge/OpenAPI runbook.
  - Acceptance: docs cover local daemon, production policies, common error codes, smoke commands, and rollback/cleanup guidance.

## Done

- [x] Commit MCP/Bridge, OpenAPI, billing events, wallet ledger, and operations dashboard checkpoint.
- [x] Add real local Bridge daemon for data-proxy integration testing.
- [x] Add concurrent daemon smoke and stress targets.
- [x] Verify daemon reconnect after server-initiated session close.
