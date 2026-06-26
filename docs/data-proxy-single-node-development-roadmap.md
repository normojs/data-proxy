# Data Proxy Single-Node Development Roadmap

Date: 2026-06-25

This project is based on `new-api`; all development must keep the upstream
AGPLv3 license, NOTICE, attribution, and third-party license requirements.

## Scope

The near-term production target is a single Data Proxy server:

- One Data Proxy application instance.
- One external Nginx entrypoint managed outside the project compose file.
- Existing MySQL and Redis on the same LAN.
- Local Docker volumes for persistent storage.
- Optional SeaweedFS single-node object storage for request capture bundles.

The goal is to make the single-server deployment stable, diagnosable, and
feature-complete enough for production use before investing in multi-node
coordination.

## Deferred Until Later

These items are intentionally not part of the current development track:

- Multi-node Data Proxy deployment.
- Cross-node Tunnel MCP SSE routing.
- Redis/shared Tunnel rate limit state for multiple app instances.
- Distributed shared bandwidth accounting.
- Sticky-session-free old SSE transport across multiple nodes.
- Multi-region or HA object storage.
- Cluster-level scheduler/job queue design.
- Protocol-conversion long-tail executors and hosted-tool bridges beyond the
  existing safe policy/webhook `web_search` MVP.

Single-node code may still use MySQL/Redis for normal application state,
existing quota counters, background tasks, and cache-like behavior. The
deferred part is cross-node coordination.

## Priority Order

Current planning override: protocol-conversion long-tail is not part of the
current single-node production queue. The active sequence is diagnostics and
release baseline first, then channel routing/access controls, request capture
and training review basics, then Tunnel/`dpa` productization. Protocol
conversion below is kept only as a regression guard for already implemented
behavior and must not expand scope before vNext.

### P0: Single-Node Observability And Diagnostics

Reason: when production traffic fails, request id based diagnosis must be fast
and self-contained.

Tasks:

1. Add authorized raw diagnostic bundle download.
   - Input: `request_id`.
   - Output: encrypted or access-controlled zip/tar bundle with metadata,
     sanitized request/response artifacts, conversion metadata, relay logs, and
     trace summary.
   - UI: from usage log/request trace detail page, add "Generate diagnostic
     bundle" and "Download bundle".
   - Current status: admin bundle download is implemented at the request trace
     diagnostic endpoint. It includes report, trace, findings, capture summary,
     and decoded raw capture files when an artifact is available.
     `DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES` bounds zip-time raw tar expansion;
     oversized raw bundles are skipped in the zip with an explanatory marker
     while the original object-store artifact remains intact. The common usage
     logs toolbar now has an admin "Diagnostic Candidates" dialog that lists
     suspicious request ids in the selected time range, including conversion
     anomalies, hosted-tool direct-answer/fallback handling, capture failures,
     and channel failover/circuit events, and can jump directly to a request-id
     filtered log view.

2. Add spool restart recovery for request capture.
   - Recover old `active/` sessions as failed or finalizable.
   - Continue finalizing old `finalize/` sessions on boot.
   - Keep old `failed/` sessions visible for diagnosis and cleanup.
   - Current status: startup recovery scans local spool before the periodic
     finalizer runs. Stale `active/` sessions move to `failed/`, existing
     `finalize/` and `failed/` directories are synced back to
     `request_capture_records`, and retry/backoff error details are preserved
     for diagnosis until a later finalize success clears them. If an
     `active/` or `finalize/` spool directory has an unreadable manifest, it is
     quarantined into `failed/` with a minimal recovered manifest and the
     related capture record is marked failed when it can be matched by request
     id or previous spool path.

3. Add finalizer retry backoff without distributed queue.
   - Use single-process background scanner plus DB state.
   - Persist attempt count, last error, and next retry time.
   - Avoid Redis job queue for now.
   - Current status: the finalizer task is master/single-process guarded and
     uses `finalize_attempts`, `last_error`, and `next_finalize_at` for
     exponential retry backoff. The scanner skips records whose next retry time
     is still in the future.

4. Improve capture safety for stream traffic.
   - Replace synchronous hot-path artifact append with buffered async writes
     where needed.
   - Keep fail-open behavior: capture failures must not break normal relay.
   - Current status: stream upstream/downstream body capture uses bounded
     async artifact writers with a pending-memory cap and close/append race
     protection. Per-artifact byte caps mark capture artifacts as truncated
     without limiting the user response, and relay tests cover fail-open
     behavior when capture append fails. Stream capture writer failures now mark
     the capture record as `failed` with `error_code=request_capture_relay_failed`
     so diagnostic candidates can be grouped by failure type.

5. Add diagnostic retention and cleanup settings.
   - Retention days for bundles.
   - Max local spool size warning.
   - Admin-visible cleanup result.
   - Current status: `CAPTURE_RETENTION_DAYS`,
     `CAPTURE_SPOOL_RETENTION_DAYS`, `CAPTURE_SPOOL_WARN_BYTES`,
     `CAPTURE_CLEANUP_INTERVAL_SECONDS`, and `CAPTURE_CLEANUP_LIMIT` are
     implemented. The cleanup task is single-node and master-process guarded;
     it marks old capture records `expired`, deletes available raw bundle
     artifacts from object storage before marking them `deleted`, removes old
     local `failed/` plus already uploaded/expired `finalize/` spool
     directories, and emits an admin-visible warning when the current spool
     usage exceeds `CAPTURE_SPOOL_WARN_BYTES`. Admins can preview or run
     cleanup with `POST /api/log/request-capture/cleanup?dry_run=true`.

Acceptance:

- Admin can find bad requests from candidate list, generate a report, download
  a full diagnostic bundle, and analyze it locally.
- Restarting the service does not strand capture sessions forever.
- Capture remains disabled by default and can be enabled for scoped users,
  models, channels, or paths.

### Regression Guard: Protocol Conversion Stabilization Only

Reason: domestic Chat-only providers must not regress when clients use the
already supported Responses API compatibility path.

Scope note:

- This is not a scheduled feature track for the current release.
- The current release track is stabilization only when an existing production
  path breaks.
- New protocol-conversion long-tail executor work is deferred to vNext.
- Do not start new executor work for `file_search`, `computer_use`,
  `code_interpreter`, `image_generation`, hosted `mcp`, `shell`, or other local
  runtime execution semantics in this track.
- Only fix regressions that break current production traffic, request
  diagnosis, or billing/usage accounting.

Tasks:

1. Add explicit hosted tools policy per channel/model.
   - `filter_and_direct_answer` as default.
   - `native_responses_required`.
   - `executor_bridge`.
   - `reject_with_clear_error`.
   - Chinese UI labels and help text.

2. Record selected policy in request conversion metadata.
   - Show requested hosted tools.
   - Show filtered/rejected/native/executor behavior.
   - Show whether direct-answer hint was injected.
   - Current status: `request_conversion_meta` includes requested/filtered
     hosted tools, selected `hosted_tools_policy`, final
     `hosted_tools_policy_effect`, executor readiness/request flags,
     rejection markers, and direct-answer hint markers.

3. Add web search executor bridge MVP.
   - Define `HostedToolExecutor` interface.
   - Support one admin-configured provider first, such as Tavily, Brave,
     SerpAPI, Bing, self-hosted search, or webhook.
   - Execute a bounded Chat tool loop server-side.
   - Convert final result back to Responses-compatible output.
   - Current status: webhook-backed `web_search` / `web_search_preview`
     executor bridge is implemented for Chat-only Responses conversion. It
     also supports client streaming requests by running the internal tool loop
     as non-streaming Chat Completions, then emitting the final result as
     Responses SSE events. The bridge only activates when all filtered hosted
     tools are web search tools. If the model repeatedly returns more
     `web_search` tool calls past the bounded executor loop, Data Proxy returns
     a clear upstream failure and records
     `hosted_web_search_executor_error=max_iterations_exceeded` rather than
     handing an unexecuted hosted call back to the client. Each executor
     webhook invocation also records lightweight
     `hosted_web_search_executor_events` metadata for request-id diagnosis
     without storing the raw search query in normal logs.

4. Add provider-specific regression fixtures.
   - DeepSeek, Qwen, Kimi/Moonshot, SiliconFlow, OpenRouter.
   - Streaming text, reasoning, tool calls, empty response, SSE error body,
     malformed usage chunks.
   - Current status: golden fixtures cover DeepSeek hosted web search fallback,
     executor-bridge readiness, mixed hosted tool fallback, Qwen namespace
     tools, Kimi/Moonshot low/high reasoning, OpenRouter reasoning details /
     reasoning strings, and
     previous-response tool output restoration. Chat SSE fixtures now also
     cover SiliconFlow-style error bodies, Moonshot-style message-only error
     events, DeepSeek reasoning-only streams that must fall back to visible
     output text, Qwen inline `<think>...</think>` reasoning content, and
     OpenRouter-style heartbeat plus DONE-only empty streams. Kimi/Moonshot
     compatible `code`/`msg` SSE error bodies are also normalized to provider
     errors, and generic `error_message` / `error_msg` provider bodies are
     covered. Qwen-style post-stop `choices=[]` usage chunks are covered so
     token usage is preserved without turning a valid answer into an empty or
     truncated response. OpenRouter-style post-stop `choices=[]` usage chunks
     with provider `cost` are covered so cost metadata survives the
     Chat->Responses conversion. OpenRouter-style structured provider errors
     keep `code`, `param`, and metadata such as provider/status/request id in
     the converted `response.failed.error`; SiliconFlow/Kimi/generic
     `error_msg` bodies also preserve diagnostic code/status/provider metadata.
     Normal chunks that include top-level provider/message routing metadata plus
     `choices` are not misclassified as errors. Qwen-style malformed streamed
     tool calls that finish with `tool_calls` but never provide a function name
     now convert to `response.failed` with `type=malformed_tool_call` instead
     of a silent empty `completed` response. DeepSeek-style streams where the tool id
     arrives before the function name are covered and still aggregate to a
     valid `function_call`. Partial-output streams that later emit a provider
     error are covered and convert to `response.failed` instead of a misleading
     `completed` response. Non-stream Chat JSON provider errors such as
     top-level `error_msg` / `message` plus `code` are now returned as upstream
     errors with metadata, and non-stream `choices=[]` converts to
     `response.failed(type=empty_response)` instead of a silent empty success.
     Native Responses -> Chat conversion also maps `status=incomplete` /
     `response.incomplete` back to Chat `finish_reason=length` or
     `finish_reason=content_filter`, including streams with no prior delta.
     Native Responses -> Chat usage conversion now preserves provider
     extensions such as OpenRouter `cost`, `usage_source`, `usage_semantic`,
     prompt cache hits, token details, and reasoning-token details in both
     non-stream results and final stream usage chunks. Direct native Responses
     handling for `/v1/responses`, streaming Responses, and compaction
     responses now uses the same usage mapper, including
     `response.completed`, `response.incomplete`, and `response.failed` stream
     terminal events, so these extensions survive even when no protocol
     conversion is needed. Non-stream Responses SSE fallback aggregation also
     recognizes native `response.incomplete` terminal events and preserves
     `incomplete_details` plus usage. Native Responses -> Chat streaming also
     backfills text/function-call chunks from terminal `response.output` when
     a provider/proxy omits incremental delta events, and maps complete
     `response.output_item.done` message/reasoning items when those are the
     only item-level events.
     These cases convert to Responses
     `response.failed` or split reasoning/text in both non-stream aggregation
     and live streaming relay paths. More production-derived malformed stream
     fixtures are still needed.

5. Keep unsupported high-risk hosted tools safe.
   - `file_search`, `computer_use`, `code_interpreter`, `image_generation`,
     hosted `mcp`, and `shell` remain filtered or rejected until their executor
     and policy are designed in vNext.

Acceptance, only when protocol conversion files are touched:

- Chat-only channels no longer silently return empty output for hosted-tool
  requests.
- Admin can see exactly why a request was filtered, rejected, answered directly,
  or executor-bridged.
- At least one real `web_search` path works on a domestic Chat-only model.

### P2: Tunnel And dpa Single-Node Productization

Reason: MCP/HTTP Tunnel MVP exists; the next goal is production usability on
one server, not horizontal scale.

Tasks:

1. Finish single-node Tunnel billing settlement.
   - `mcp_code`: settle successful `tools/call` by call count.
   - `http_tunnel`: settle by request, bytes, and duration according to a
     configurable policy.
   - Keep failed/denied events auditable; make charging behavior explicit.
   - Current status: configurable settlement is implemented behind
     `billing.settlement.enabled=true`; it writes idempotent `debit` events and
     deducts the Tunnel App owner wallet balance in the same transaction.
     `ledger_only=true` or `deduct_balance=false` keeps observation-only mode.
     `require_positive_balance=true` now rejects MCP/HTTP data-plane requests
     before forwarding when the owner balance is not positive, and records a
     `billing_deny` audit event with `reason=billing_insufficient`.
     `require_sufficient_balance=true` adds an optional single-node preflight
     check against the estimated minimum charge for the current data-plane
     action, without changing the default postpaid behavior.
     `auto_disable_on_overdue=true` / `disable_when_overdue=true` /
     `disable_on_negative_balance=true` can now automatically set the Tunnel
     App to `disabled` after an idempotent settlement debit leaves the owner
     quota non-positive, and writes a `billing_overdue_disabled` audit row.
   - Remaining settlement risk work: prepaid reservation and stricter
     concurrency-safe hard limits.

2. Harden single-node MCP Gateway.
   - Tool/prompt/resource display filtering.
   - Better audit grouping by gateway session id.
   - Clear UI for allowed/denied tools and permission mode.
   - More Streamable HTTP/SSE compatibility tests on one instance.
   - Current status: `tools/list` permission filtering is implemented;
     `resources/list`, `resources/read`, `resources/templates/list`,
     `prompts/list`, and `prompts/get` now support `policy.mcp_gateway`
     allow/deny filtering and policy-deny audit events.

3. Harden HTTP Tunnel.
   - Longer WebSocket/SSE/large body stress tests.
   - Login-state access design and implementation if needed.
   - CONNECT support only after route and security policy are clear.
   - Current status: service-side regression tests now cover multi-chunk large
     streamed responses, byte-order preservation, audit `bytes_out`, and
     streamed response overflow. Overflow, decode, rate-limit, client-writer,
     and bridge error paths cancel the pending Bridge stream so a failed
     public HTTP request does not leave an orphaned `dpa` tool call.
     Public client writer failures are recorded with
     `reason=client_write_failed`, and `dpa` stream emitter failures now have
     targeted regression coverage to ensure the local request exits cleanly.
     WebSocket coverage includes text, binary, ping/pong control frames,
     response overflow, response rate-limit, and client-writer failure paths.
     SSE-style streamed responses are covered with per-chunk `Flush=true`
     assertions so LLM token streams do not regress into buffered delivery.

4. Add TCP Tunnel MVP.
   - Single-node TCP-over-WebSocket data-plane entry.
   - Explicitly approved apps only.
   - Default deny public exposure.
   - Connection audit, byte counters, timeout, and revoke support.
   - Current status: MVP is implemented at
     `/t/:connection_key/tunnel/tcp/:slug` with Bridge `tcp_tunnel.connect`,
     local `dpa` TCP dialing, binary frame forwarding, audit logs, rate limits,
     billing event mirroring, and bridge error audit coverage. WebSocket
     ping/pong control frames are handled as transport heartbeat and are not
     forwarded into the TCP byte stream. Client-writer failures cancel the
     pending Bridge stream with `reason=client_write_failed`; request and
     response byte-limit overflows close the public WebSocket with 1009,
     cancel the pending Bridge stream, and write deny audit rows with
     `reason=request_too_large` or `reason=response_too_large`. `dpa` closes
     its TCP stream input queue when either pipe direction exits.
   - Remaining TCP work: raw TCP listener, port pool, connection reuse,
     active backend probe UI, and richer route/security policy.

5. Improve `dpa` onboarding.
   - One-command install examples in console.
   - Setup token flow polish.
   - Agent version recommendation in console.
   - Multi-agent display and management for one user.
   - Current status: Tunnel Connections now shows per-app local
     `dpa tunnel route add ...` commands for approved HTTP/TCP Tunnel Apps, and
     My Tunnel Apps can request TCP Tunnel Apps from the user console. The
     local CLI now supports `dpa status --json`, a no-network redacted summary
     for install scripts and console-side collection, including Bridge URL,
     token presence/source, effective capabilities, static MCP/HTTP/TCP route
     summaries, and route counts. `dpa status --health` can additionally probe
     local HTTP/TCP/MCP targets for a quick machine-side health snapshot.
     `dpa doctor --json` now emits token-safe structured diagnostics
     for script or console-side collection, and `dpa doctor --check-update` /
     `dpa report --check-update` can query GitHub Release or a custom manifest
     for release metadata and record newer-version availability as a warning
     without failing offline/local health checks. Local health checks now probe
     configured `tcp_routes` directly, so deployment reports can catch an
     unreachable local TCP backend before exposing the tunnel. `dpa tunnel
     route test <name>` provides a focused HTTP/TCP route probe with text or
     JSON output for install scripts and deployment checks. The CLI command
     name is standardized as `dpa`, including Windows-style `dpa.exe`
     invocation normalization.

6. Improve local MCP process health.
   - stdio MCP stderr classification.
   - Long-running subprocess watchdog.
   - UI aggregation of local process events reported by `dpa`.
   - Current status: stdio MCP stderr now has basic classification
     (`permission`, `command_not_found`, `dependency`, `port_in_use`, `auth`,
     `crash`, or generic `stderr`). The class is exposed as `stderr_class` in
     local audit metadata and `dpa doctor`/health details for exited stdio MCP
     processes. A lightweight local watchdog now reaps exited cached stdio MCP
     sessions and writes `mcp_stdio.watchdog_reap` audit events with
     `stderr_class`; it does not proactively restart local MCP processes.
     Agent health reports now include a structured `mcp_processes` summary, and
     the Bridge Client health API / console detail view can display stdio MCP
     process status, PID, initialization state, stderr class, and exit errors.
     Historical process event aggregation remains pending.

Acceptance:

- A user can create a Tunnel App, install `dpa`, connect one machine, expose
  MCP/HTTP targets, view audit, and understand billing on a single server.
- Revoking a connection stops access quickly.
- Admin can diagnose tunnel failures from request id/session id/audit logs.

### P3: Request Data Lake And Training Corpus

Reason: valuable, but only after capture safety and diagnostics are solid.

Tasks:

1. Define raw data retention and privacy policy.
   - Full bundle vs sanitized bundle.
   - Tenant/user opt-in or admin policy.
   - Secret and PII redaction rules.

2. Build training corpus worker.
   - Read encrypted bundles.
   - Decrypt in memory only.
   - Normalize Responses and Chat Completions into a stable schema.
   - Produce versioned JSONL.zst or Parquet shards.
   - Track sample lineage and dataset version.
   - Current status: a single-node service-layer MVP is implemented as
     `BuildTrainingCorpusDataset`. It reads available raw bundle artifacts,
     decodes them in memory with a per-bundle size cap, applies basic recursive
     JSON secret-key redaction, extracts Chat/Responses JSON and SSE output
     text, writes a versioned `jsonl.zst` shard to SeaweedFS/S3-compatible
     storage, and records `training_dataset_versions` plus `training_samples`
     lineage rows. Admin APIs are available at `GET /api/training/datasets`,
     `POST /api/training/datasets/build`, and
     `GET /api/training/datasets/:id/export`. Sample lineage and review APIs
     are available at `GET /api/training/samples`,
     `GET /api/training/samples/:id/preview`,
     `POST /api/training/samples/:id/approve`, and
     `POST /api/training/samples/:id/reject`. This is not wired to an admin UI
     yet.

3. Add review workflow.
   - Admin preview.
   - Approve/reject samples.
   - Export dataset version.
   - Current status: approve/reject APIs are implemented for sample lineage,
     and dataset export now filters the generated `jsonl.zst` shard down to
     approved `source_hash` rows before returning it. A sample preview API is
     implemented by looking up the matching JSONL line in the generated shard.
     Sample preview UI and broader review UX are still required before broad
     production training use.

Acceptance:

- Stored traffic can be safely transformed into reviewable training samples.
- Every sample can be traced back to source request id and redaction version.

### P4: Release And Packaging Polish

Reason: useful after core single-node production features are stable.

Tasks:

1. Keep amd64 production Docker image as the primary path.
2. Preserve release assets and rollback tags.
3. Publish a machine-readable `dpa` release manifest.
   - Done: `Data Proxy Agent` workflow now generates
     `data-proxy-agent-manifest.json` from release artifacts and sha256 files.
   - The manifest is suitable for `dpa update --manifest-url ...`, console
     proxying, domestic mirrors, and controlled rollout.
   - macOS manifest entries prefer notarized archives when notarization assets
     are present, otherwise they fall back to ordinary tar.gz archives.
   - Done: `scripts/install-data-proxy-agent.sh` can also consume
     `DATA_PROXY_AGENT_MANIFEST_URL`, so first install and later `dpa update`
     can use the same mirrored manifest and sha256 metadata.
4. Add arm64 container publishing later with a non-QEMU or matrix strategy.
5. Improve Homebrew tap automation.
6. Improve Windows service helper and ConPTY support.
7. Complete macOS notarization once Apple secrets are available.

Acceptance:

- Production can deploy and roll back predictably.
- Agent installers remain easy to verify and upgrade.
- `dpa` update metadata can be mirrored away from GitHub without changing the
  CLI updater.

## Recommended Next Sprint

Use `docs/data-proxy-near-term-development-plan.md` as the current execution
queue and `docs/data-proxy-current-development-tasks.md` as the concrete
checklist. The next sprint is release-oriented, not protocol-feature-oriented:

1. Freeze protocol conversion scope to regression fixes only.
2. Clean and split the current worktree into feature-sized commits.
3. Run focused backend and frontend validation for channel failover, user group
   binding, request diagnostics, Tunnel, and `dpa`.
4. Build or trigger the Docker image through CI, deploy to the single server,
   and record image digest plus rollback command.
5. Run production smoke for `/api/status`, Chat, Responses native,
   Responses-via-Chat, request trace, diagnostic bundle, same-model channel
   failover, payment if enabled, and one `dpa status --json` report.

After the release baseline is stable, continue with channel routing/group
restrictions, then Tunnel/`dpa` productization, then request capture and
training-data review.

## Notes

- Do not commit `.env.production`, certificates, merchant private keys, API
  keys, diagnostic bundles, raw captures, or local storage data.
- Keep single-node assumptions visible in docs and UI where they affect
  behavior.
- Any future multi-node work should start from a separate plan instead of
  silently changing the single-node roadmap.
