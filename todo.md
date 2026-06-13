# data-proxy MCP / Bridge TODO

## P0 - Current development plan

- [x] Add a repeatable Docker-backed PostgreSQL migration gate.
  - Acceptance: local developers can run a documented command or Make target that starts/uses project-owned PostgreSQL and executes `make mcp-migration-postgres` with a known test DSN.
  - Acceptance: the gate does not depend on unrelated host containers or production databases.
  - Done: added `docker-compose.migration.yml` and `make mcp-migration-postgres-docker`; the target starts disposable PostgreSQL on `127.0.0.1:15432`, runs `make mcp-migration-postgres`, and cleans up by default.
- [x] Add a repeatable Docker-backed MySQL migration gate.
  - Acceptance: local developers can run a documented command or Make target that starts/uses project-owned MySQL and executes `make mcp-migration-mysql` with a known test DSN.
  - Acceptance: the gate handles existing host port conflicts such as a global `mysql8` container on port `3306`.
  - Done: added disposable MySQL to `docker-compose.migration.yml` and `make mcp-migration-mysql-docker`; the target uses `127.0.0.1:13306` by default, runs `make mcp-migration-mysql`, and cleans up by default.
- [x] Run and record external database migration gates.
  - Acceptance: PostgreSQL and MySQL migration smoke results are recorded in `todo.md` after the Docker-backed gates run.
  - Acceptance: failures are fixed or explicitly documented with reproduction commands.
  - Done: `make mcp-migration-docker` passed, running PostgreSQL on `127.0.0.1:15432` and MySQL on `127.0.0.1:13306` through disposable Docker services.
- [x] Harden external migration gate documentation and cleanup.
  - Acceptance: docs explain startup, DSN, reset, cleanup, and how the gates relate to `make mcp-regression`.
  - Acceptance: no secrets or host-specific credentials are committed beyond disposable local test defaults.
  - Done: documented Docker-backed startup, default DSNs, port overrides, debug retention, cleanup, and the relationship between `make mcp-migration-docker` and `make mcp-regression`.
- [ ] Audit non-MCP backlog and pick the next backend batch.
  - Acceptance: remaining `TODO` / `unsupported` / `not implemented` findings outside the completed MCP scope are classified as product backlog, intentional unsupported behavior, or bug-fix candidates.

## Deferred - Long-term UI v2 plan

- [ ] Revisit shadcn-based UI v2 redesign after current backend and migration work.
  - Status: deferred; do not start implementation now.
  - Plan: `docs/ui-v2-long-term-plan.md`.
  - Direction: keep `web/classic` as legacy, keep `web/default` as the current shadcn frontend, and later evolve `web/default` with a v2 product UI shell/pilot instead of creating a third frontend app.

## P0 - Release readiness

- [x] Run unified MCP regression after final architecture cleanup.
  - Acceptance: `make mcp-regression` passes after the latest MCP/OpenAPI/Proxy cleanup commits.
  - Done: `make mcp-regression` passed, covering OpenAPI, Proxy, Bridge lightweight checks, Dashboard smoke, and TypeScript build.
- [x] Run SQLite MCP migration smoke.
  - Acceptance: `make mcp-migration-sqlite` passes against a temporary SQLite database.
  - Done: `make mcp-migration-sqlite` passed against the temporary SQLite migration smoke, including repeated `InitDB` / `InitLogDB` startup on the same database.
- [x] Run real Bridge daemon concurrency smoke.
  - Acceptance: `make mcp-bridge-smoke` passes with local daemon read/write/edit/glob/proxy coverage.
  - Done: `make mcp-bridge-smoke` passed with `MCP_BRIDGE_SMOKE_CONCURRENCY=4`, `MCP_BRIDGE_SMOKE_ITERATIONS=1`, `MCP_BRIDGE_SMOKE_TIMEOUT=120000`, temp SQLite WAL/busy-timeout pragmas, and single SQLite connection.
- [x] Fix SQLite repeated migration for Bridge daemon smoke.
  - Acceptance: SQLite startup migration can run twice against the same database without `invalid DDL, unbalanced brackets`.
  - Acceptance: `make mcp-migration-sqlite` covers the repeated-start regression.
  - Done: SQLite migration now normalizes legacy `decimal(p,s)` table DDL to `numeric` before repeated AutoMigrate parsing.
- [x] Harden Bridge daemon smoke against performance monitor guard.
  - Acceptance: smoke setup avoids CPU/memory/disk monitor 503 responses caused by local test machine load.
  - Done: Bridge smoke prepare raises CPU, memory, and disk monitor thresholds to 100 for local release-gate runs.
- [x] Align Bridge daemon smoke with server-side write policy.
  - Acceptance: write/edit smoke calls explicitly enable server-side Bridge policy allowlist before expecting success.
  - Done: smoke policy setup now sets `allowed_tools: ["*"]`, `allow_write: true`, and `mcp_allowed_targets: ["*"]` for the writable daemon client.
- [x] Record external database migration gate status.
  - Acceptance: MySQL/PostgreSQL migration commands are either executed with DSNs or documented as DSN-gated.
  - Done: MySQL/PostgreSQL migration gates are DSN-gated on this machine because `MCP_MIGRATION_MYSQL_DSN` and `MCP_MIGRATION_POSTGRES_DSN` are unset; run `make mcp-migration-mysql MCP_MIGRATION_MYSQL_DSN='...'` and `make mcp-migration-postgres MCP_MIGRATION_POSTGRES_DSN='...'` when external test databases are available.
- [x] Complete final release hygiene audit.
  - Acceptance: `git diff --check` passes, worktree is clean, and remaining TODO/unsupported scan findings are classified.
  - Done: `git status --short --branch` showed a clean worktree, `git diff --check` passed, `make mcp-regression` passed, and the remaining scan hits are limited to `todo.md` documentation plus smoke-script fail-fast helper `panic` calls.

## P1 - MCP Proxy HTTP client architecture cleanup

- [x] Align MCP Proxy HTTP JSON-RPC serialization with project wrappers.
  - Acceptance: `pkg/mcp/proxy/http_client.go` no longer calls `json.Marshal` or `json.Unmarshal` directly.
  - Acceptance: `encoding/json` remains only for JSON-RPC `json.RawMessage` DTO types.
  - Acceptance: MCP Proxy HTTP/SSE/Streamable tests still pass.
  - Done: MCP Proxy HTTP client marshal/unmarshal paths now use `common.Marshal/common.Unmarshal`; proxy package tests and `make mcp-proxy-check` passed.

## P1 - MCP controller architecture cleanup

- [x] Remove unnecessary direct JSON dependency from MCP controller.
  - Acceptance: `controller/mcp.go` no longer imports `encoding/json`.
  - Acceptance: MCP initialize and invalid request controller tests still pass.
  - Done: MCP controller now passes `req.ID` directly to `common.JsonRawMessageToString`; MCP controller tests passed.

## P1 - OpenAPI parser architecture cleanup

- [x] Align OpenAPI parser JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/openapi/parser.go` no longer imports `encoding/json`.
  - Acceptance: OpenAPI parser tests still cover refs, schema merges, form/multipart, and binary request bodies.
  - Done: OpenAPI parser now uses `common.Unmarshal/common.Marshal` for JSON parsing and schema cloning; parser and OpenAPI package tests passed.

## P1 - OpenAPI binary storage architecture cleanup

- [x] Align OpenAPI binary object metadata JSON conversion with project wrappers.
  - Acceptance: local and S3 binary object stores no longer import `encoding/json` for metadata serialization.
  - Acceptance: binary object save/load/cleanup tests still pass for local and S3-compatible stores.
  - Done: local and S3 metadata save/load/cleanup now use `common.Marshal/common.Unmarshal`; OpenAPI package binary object tests passed.

## P1 - MCP executor architecture cleanup

- [x] Align built-in MCP executor JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/executor/builtin.go` no longer imports `encoding/json`.
  - Acceptance: built-in executor server time and JSON pretty tests still pass.
  - Done: built-in executor server time and JSON pretty now use `common.Marshal/common.UnmarshalJsonStr/common.MarshalIndent`; executor tests passed.

## P1 - OpenAPI executor architecture cleanup

- [x] Align OpenAPI MCP executor JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/executor/openapi.go` no longer imports `encoding/json`.
  - Acceptance: OpenAPI executor request/response formatting tests still pass.
  - Done: OpenAPI executor now uses `common.Marshal/common.Unmarshal/common.MarshalIndent`; MCP OpenAPI regression passed.

## P1 - MCP Proxy architecture cleanup

- [x] Align MCP Proxy Bridge client JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/proxy/bridge_client.go` no longer imports `encoding/json`.
  - Acceptance: Bridge proxy list/call/test behavior remains covered by existing tests.
  - Done: Bridge client result/object helpers now use `common.Marshal/common.Unmarshal`; MCP Proxy regression passed.

## P1 - Billing architecture cleanup

- [x] Replace subscription pre-consume string matching with sentinel errors.
  - Acceptance: model subscription pre-consume errors support `errors.Is`.
  - Acceptance: billing session maps no-active/insufficient subscription errors to insufficient quota without string matching.
  - Done: model now exposes subscription pre-consume sentinel errors; billing session uses `errors.Is` and tests cover model/service behavior.

## P1 - Relay regression cleanup

- [x] Fix Claude relay OpenAI file-content conversion.
  - Acceptance: unsupported OpenAI `file` content is skipped instead of being sent as an image.
  - Acceptance: PDF files become Claude `document` blocks and text files become Claude `text` blocks.
  - Acceptance: `go test ./relay/channel/claude` and `go test ./relay/channel/...` pass.
  - Done: Claude relay now infers OpenAI file mime type from filename when needed, accepts map payload `filename`, emits PDF/document and text blocks explicitly, and ignores unsupported files.

## P1 - Audit remediation

- [x] Fix Bridge reconnect/offline race so an old session close cannot mark a replaced live client offline.
  - Acceptance: closing an old session while a replacement session is online keeps `bridge_clients.status=online`.
  - Acceptance: normal last-session close still marks the client offline.
- [x] Make MCP Overview Bridge online trends accurate beyond the fixed 10k session cap.
  - Acceptance: recent buckets are not undercounted when many sessions overlap the window.
  - Acceptance: tests cover overflow-like session counts without relying on production-size fixtures.
- [x] Make MCP Review Queue large-installation behavior explicit and harder to miss.
  - Acceptance: queue summaries expose capped scan/visible counts or overflow metadata.
  - Acceptance: dashboard can distinguish “no issues” from “scan capped”.
  - Done: Review Queue responses now expose `visible_count`, `max_items`, `truncated`, and per-source `scan_limits`; MCP Overview shows visible/total counts and capped scan state.
- [x] Replace reachable provider adaptor `panic("implement me")` stubs with stable unsupported errors.
  - Acceptance: unsupported relay modes return errors instead of panicking.
  - Done: 11 provider `ConvertClaudeRequest` stubs now return stable `not implemented` errors; regression test covers no-panic behavior.
- [x] Align Bridge controller JSON decoding with project JSON wrapper rules.
  - Acceptance: Bridge controller no longer calls `encoding/json` marshal/unmarshal directly.
  - Done: `controller/bridge.go` now uses `common.Marshal/common.Unmarshal`; controller tests cover bridge result/error payload decoding.

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
- [x] Improve billing relation repair UX.
  - Acceptance: admins can preview relation diffs and repair selected items without running a broad backfill.
  - Backend: expose selected-item repair payloads for missing/orphan MCP billing relations and keep broad backfill as a fallback.
  - Frontend: add preview rows with per-item selection, summary counters, and repair action feedback.
  - Validation: cover dry-run, selected repair, idempotent repair, and no-op repair paths.
  - Done: added `POST /api/billing/events/relation-repair` for selected audit relation repair with dry-run, stale payload validation, idempotent existing-link handling, and no-op empty selection behavior.
  - Done: Billing Events Audit Relations panel now stores preview rows, supports per-row/select-all repair selection, and shows created/skipped/invalid repair feedback while keeping broad backfill as fallback.
  - Validation: targeted service/controller/router tests, `npm run typecheck --silent`, `make mcp-dashboard-check`, and `make mcp-regression`.
- [x] Publish MCP/Bridge/OpenAPI runbook.
  - Acceptance: docs cover local daemon, production policies, common error codes, smoke commands, and rollback/cleanup guidance.
  - Docs: include local daemon setup, policy defaults, Bridge failover, OpenAPI binary storage, review queue, billing repair, smoke/regression commands, and rollback cleanup.
  - Validation: link every documented command to an existing Make target or script.
  - Done: added `docs/mcp-bridge-openapi-runbook.md` with operations map, command index, local daemon flow, production policy defaults, OpenAPI binary object handling, review queue triage, billing repair guidance, common error codes, rollback, and cleanup guidance.
  - Done: linked `docs/mcp-bridge-smoke.md` to the new operations runbook.
  - Validation: verified every runbook command maps to an existing Make target or Node script.

## Done

- [x] Commit MCP/Bridge, OpenAPI, billing events, wallet ledger, and operations dashboard checkpoint.
- [x] Add real local Bridge daemon for data-proxy integration testing.
- [x] Add concurrent daemon smoke and stress targets.
- [x] Verify daemon reconnect after server-initiated session close.
