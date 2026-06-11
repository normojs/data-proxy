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
- [x] Add cross-database MCP migration regression documentation and opt-in targets.
  - Acceptance: SQLite default, MySQL opt-in, and PostgreSQL opt-in commands cover MCP, Bridge, billing event, and OpenAPI binary object tables.

## P1 - Core capability expansion

- [x] Add server-side Bridge client policy controls.
  - Acceptance: admins can configure allowed tools, write permission, max result size, scan limits, and MCP target allowlist per Bridge client.
  - Acceptance: daemon defaults remain conservative and server-side policy is enforced before tool forwarding.
- [x] Implement MCP Proxy OAuth authentication support.
  - Acceptance: MCP Proxy supports OAuth token resolve/cache/refresh without regressing none/bearer/basic/header auth.
  - Acceptance: auth failures are observable in discovery events and health checks without leaking secrets.
- [x] Surface OpenAPI import schema metrics and diff summary in the MCP Dashboard.
  - Acceptance: import preview displays importable tool count, schema count, reused schema count, skipped reasons, and diff summary.
- [x] Add OpenAPI binary object management to the MCP Dashboard.
  - Acceptance: admins can view binary object counts, bytes, expiry state, cleanup dry-run, cleanup execute, and download audit context.

## P1 - Reliability and observability

- [x] Add MCP tool call idempotency and replay protection.
  - Acceptance: repeated client request IDs do not double-charge or double-settle a tool call.
- [x] Add Bridge multi-client selection and failover.
  - Acceptance: Bridge MCP Proxy can choose by latest activity/capability and fail over to another eligible online client.
- [x] Add MCP operations review queue.
  - Acceptance: health check, heartbeat, stale Bridge clients, and high-error tools produce actionable review reasons in dashboard summaries.
  - Done: `service/mcp_review.go` aggregates proxy-server review state, stale bridge clients (online in DB but no live hub session), failed health-check/heartbeat runs, and high-error-rate tools into `MCPSummary.review_queue` (admin-wide scope only).
  - Done: MCP Overview shows a Review Queue panel with critical/warning counts and per-item drilldown to proxy servers, bridge clients, and tool calls.
  - Done: severity (`critical`/`warning`), reason codes (`bridge_stale`, `health_check_failed`, `heartbeat_failed`, `high_error_rate_tool`, plus existing proxy reasons) and tests in `service/mcp_review_test.go`.

## P2 - Operations polish

- [x] Expand MCP Overview with Bridge and OpenAPI storage trends.
  - Acceptance: overview shows Bridge online trend, Proxy error topN, OpenAPI binary storage trend, and refund/settlement anomaly summary.
  - Backend: add summary DTO/model/service helpers for Bridge online buckets, Proxy error TopN, OpenAPI binary object storage buckets, and MCP billing anomaly counters.
  - Frontend: add compact Overview panels that tolerate empty/partial trend payloads and drill down to existing sections where possible.
  - Validation: cover backend aggregation with service tests and dashboard normalization/smoke tests.
  - Done: `/api/mcp/summary` includes `operations_trends` with Bridge online/session buckets, OpenAPI binary object storage buckets, Proxy error TopN, and MCP billing anomaly counters.
  - Done: MCP Overview shows storage/Bridge mini trends plus Proxy error and billing anomaly panels with drilldown links.
  - Done: `service/mcp_overview_trends_test.go` and `scripts/check-mcp-trends.mjs` cover empty/partial payloads and aggregate signals.
- [ ] Improve billing relation repair UX.
  - Acceptance: admins can preview relation diffs and repair selected items without running a broad backfill.
  - Backend: expose selected-item repair payloads for missing/orphan MCP billing relations and keep broad backfill as a fallback.
  - Frontend: add preview rows with per-item selection, summary counters, and repair action feedback.
  - Validation: cover dry-run, selected repair, idempotent repair, and no-op repair paths.
- [ ] Publish MCP/Bridge/OpenAPI runbook.
  - Acceptance: docs cover local daemon, production policies, common error codes, smoke commands, and rollback/cleanup guidance.
  - Docs: include local daemon setup, policy defaults, Bridge failover, OpenAPI binary storage, review queue, billing repair, smoke/regression commands, and rollback cleanup.
  - Validation: link every documented command to an existing Make target or script.

## Done

- [x] Commit MCP/Bridge, OpenAPI, billing events, wallet ledger, and operations dashboard checkpoint.
- [x] Add real local Bridge daemon for data-proxy integration testing.
- [x] Add concurrent daemon smoke and stress targets.
- [x] Verify daemon reconnect after server-initiated session close.
