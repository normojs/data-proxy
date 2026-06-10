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
- [ ] Add trend panel empty-state and partial-data handling tests.
  - Acceptance: no runtime error when trend endpoints return empty arrays.

## Done

- [x] Commit MCP/Bridge, OpenAPI, billing events, wallet ledger, and operations dashboard checkpoint.
- [x] Add real local Bridge daemon for data-proxy integration testing.
- [x] Add concurrent daemon smoke and stress targets.
- [x] Verify daemon reconnect after server-initiated session close.
