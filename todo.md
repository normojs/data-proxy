# data-proxy MCP / Bridge TODO

## P0 - Deployment readiness preflight

- [ ] Run full backend test and build preflight.
  - Acceptance: `go test ./...` passes or any intentional skips/failures are documented with exact commands and reasons.
  - Acceptance: the backend binary can be built with the current frontend assets expectation satisfied or explicitly covered by Docker/frontend build checks.
- [ ] Run production frontend build preflight for both UI versions.
  - Acceptance: `make build-all-frontends` passes for `web/default` and `web/classic`.
  - Acceptance: generated artifacts remain uncommitted unless the repository already tracks them.
- [ ] Validate Docker deployment configuration.
  - Acceptance: `docker compose config` passes for production and dev compose files.
  - Acceptance: Docker build prerequisites are verified without committing local build artifacts.
- [ ] Record deployment readiness result.
  - Acceptance: `todo.md` records exact commands run, pass/fail result, and remaining deployment caveats.
  - Acceptance: final worktree is clean after any generated artifacts are cleaned or intentionally ignored.

## P1 - Fresh audit follow-up batch

- [x] Clarify Claude extended-thinking sampling compatibility.
  - Acceptance: duplicated temporary handling for Claude thinking sampling parameters is replaced with explicit helper logic or documented guardrails.
  - Acceptance: tests cover non-Opus extended thinking and Opus 4.7/4.8 adaptive thinking across the OpenAI-to-Claude conversion path.
  - Done: added explicit `dto.ClaudeRequest` helpers for adaptive thinking effort, sampling clearing, default extended-thinking temperature, and OpenAI conversion sampling compatibility; replaced duplicated temporary handling in native Claude and OpenAI-to-Claude paths. Tests now cover Opus 4.8 adaptive high-effort and non-Opus enabled-budget compatibility. Validation: `go test ./dto ./relay ./relay/channel/...`.
- [x] Classify non-JSON request model extraction behavior.
  - Acceptance: `common.UnmarshalBodyReusable` behavior for unknown content types is covered by tests or documented as intentional no-op behavior.
  - Acceptance: distributor model extraction remains predictable for JSON, form, multipart, and unknown content types.
  - Done: replaced the vague non-JSON TODO with an explicit no-op contract for unknown content types; added `common` tests for JSON, form, multipart, body reset, repeated reads, and unknown content type no-op; added distributor model extraction tests for JSON, form, multipart, and unknown content type. Validation: `go test ./common ./middleware`.
- [x] Clarify Jimeng task result code normalization.
  - Acceptance: Jimeng task result parsing has tests for success, provider failure, queue, and done statuses.
  - Acceptance: provider codes are either normalized through a named helper or documented as intentionally preserved upstream codes.
  - Done: added named Jimeng success-code helpers; success code `10000` maps to local `0`, provider failures preserve upstream code, and failure status can no longer be overwritten by `data.status=done`. Tests cover done, queue, provider failure, failure-with-done, and invalid JSON. Validation: `go test ./relay/channel/task/jimeng ./relay/channel/task/...`.
- [x] Refresh release regression after fresh audit batch.
  - Acceptance: selected Go package checks, `git diff --check`, and the relevant MCP regression target pass after the fresh audit fixes.
  - Acceptance: `todo.md` records the verification result and any intentionally skipped external dependency gates.
  - Done: `go test ./common ./middleware ./dto ./relay ./relay/channel/... ./relay/channel/task/...`, `git diff --check`, and `make mcp-regression` passed after the fresh audit batch. External MySQL/PostgreSQL migration gates were not rerun in this step because this batch did not touch migrations and the MCP regression target was the relevant release guard.

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
- [x] Audit non-MCP backlog and pick the next backend batch.
  - Acceptance: remaining `TODO` / `unsupported` / `not implemented` findings outside the completed MCP scope are classified as product backlog, intentional unsupported behavior, or bug-fix candidates.
  - Done: classified non-MCP findings in `docs/non-mcp-backlog-audit.md`; selected asynchronous channel balance refresh, conservative image URL sensitive checks, and SSRF-safe Gemini/Veo HTTP image input support as the next backend batch.

## P1 - Selected next backend batch

- [x] Make admin-triggered all-channel balance refresh asynchronous.
  - Acceptance: `UpdateAllChannelsBalance` returns promptly, prevents overlapping runs, and exposes enough status/log context to know whether a refresh is already running or has started.
  - Acceptance: the existing single-channel balance query remains synchronous.
  - Done: admin all-channel balance refresh now starts a background job, returns `started` and `refresh` state, rejects overlapping refreshes with the current running snapshot, and the automatic refresher shares the same lock.
- [x] Add conservative sensitive checks for image URL payloads.
  - Acceptance: `CheckSensitiveMessages` scans text-bearing image URL fields already present in request payloads without fetching remote images.
  - Acceptance: tests cover text-only, image-only, mixed content, and no-sensitive-word paths.
  - Done: `CheckSensitiveMessages` now scans text, image URL strings, `MessageImageUrl` fields, and nested textual image metadata without fetching remote content; service tests cover text, image URL string/object, direct metadata, and clean mixed content.
- [x] Add SSRF-safe HTTP image input support for Gemini/Veo task images.
  - Acceptance: HTTP image URLs are downloaded only through safe request helpers with size and MIME limits, then converted to existing base64 image input.
  - Acceptance: tests cover allowed URLs, rejected hosts, oversized content, unsupported MIME, data URI, and raw base64 behavior.
  - Done: Gemini/Veo task image parsing now downloads HTTP(S) inputs through `service.DoDownloadRequest`, preserves SSRF redirect checks, enforces 20 MB and supported image MIME limits, and keeps data URI/raw base64 support covered by tests.

## P2 - Realtime compatibility hardening

- [x] Make Realtime WebSocket subprotocol compatibility explicit.
  - Acceptance: the local WebSocket upgrader supports stable Realtime protocol names expected by known clients.
  - Acceptance: credential-bearing `openai-insecure-api-key.*` values are not selected or echoed as WebSocket subprotocols.
  - Acceptance: tests cover accepted protocol negotiation and unsupported protocol behavior without requiring upstream connectivity.
  - Done: Realtime WebSocket negotiation now explicitly supports `realtime` and `openai-beta.realtime-v1`, keeps `openai-insecure-api-key.*` out of the selectable subprotocol list, and has handshake-level tests for accepted and unsupported protocols.

## P2 - Task lifecycle hygiene

- [x] Make task polling context and resource handling explicit.
  - Acceptance: long-running task pollers no longer use `context.TODO()` for production loops.
  - Acceptance: Midjourney polling HTTP requests cancel their timeout context and close response bodies on every response/error path.
  - Acceptance: touched JSON serialization paths use the project `common` wrappers and tests cover the extracted polling request helper.
  - Done: task polling loops now use explicit background contexts; Midjourney polling uses an extracted timeout-aware request helper with deferred cancel/body close, project JSON wrappers, and tests for success, status errors, parse errors, and missing base URL.

## P2 - Veo advanced image inputs

- [x] Add Veo last-frame and reference-image request support.
  - Acceptance: Gemini and Vertex Veo request builders can map metadata `lastFrame` / `last_frame` into the request instance when a primary image is present.
  - Acceptance: metadata `referenceImages` / `reference_images` supports up to 3 safely parsed image inputs with optional `referenceType` / `reference_type`, defaults to `asset`, and is limited to Veo 3.1 models.
  - Acceptance: reference-image requests default duration to 8 seconds when not provided and reject non-8 second durations before calling upstream.
  - Acceptance: tests cover payload shape, validation failures, and Gemini/Vertex shared behavior.
  - Done: Gemini/Vertex Veo builders now share advanced image parsing for `lastFrame` and `referenceImages`, reuse safe image parsing for base64/data URI/HTTP inputs, enforce Veo 3.1 and reference duration constraints, and cover helper plus both request builders in tests.

## P2 - Channel balance capability clarity

- [x] Make Azure channel balance behavior explicit.
  - Acceptance: Azure channels are explicitly classified as unsupported for API-key balance queries instead of being left as a generic TODO.
  - Acceptance: all-channel background balance refresh skips Azure before invoking provider balance requests.
  - Acceptance: single-channel Azure balance queries return a typed unsupported error and tests cover the helper/error behavior.
  - Done: Azure balance querying now returns a typed unsupported error, all-channel refresh skips Azure before provider requests, and controller tests cover the capability helper and unsupported error.

## P2 - UI v2 pilot

- [x] Activate UI v2 design context.
  - Acceptance: `PRODUCT.md`, `DESIGN.md`, a UI v2 design brief, and Impeccable live/design config exist before implementation.
  - Acceptance: the design direction keeps `web/classic` unchanged and evolves `web/default` as the shadcn-based pilot home.
  - Done: added product/design context, `.impeccable` live/design files, and `docs/ui-v2-design-brief.md` for the active pilot.
- [x] Add a UI version switcher and persisted preference.
  - Acceptance: operators can switch between current UI and v2 pilot without losing the existing routes.
  - Acceptance: the choice is stored locally first and can be promoted to backend preference later.
  - Done: added a localStorage-backed UI version store and a profile-menu switch that preserves the current UI while enabling the v2 pilot preference.
- [x] Add the v2 authenticated pilot shell.
  - Acceptance: the v2 shell is mounted under a low-risk authenticated route and uses existing auth, router, i18n, and shadcn primitives.
  - Acceptance: the shell includes navigation, page header, breadcrumbs or context, density rules, and mobile-safe layout.
  - Done: added the admin-guarded `/ui-lab/*` shell, profile switch navigation, route tree entries, compact v2 navigation, breadcrumb context, and a placeholder MCP pilot route ready for data wiring.
- [x] Build the MCP operations v2 pilot surface.
  - Acceptance: the pilot reuses existing MCP APIs/query keys and covers overview, health/risk state, recent activity, and navigation into existing detailed sections.
  - Acceptance: loading, empty, partial, error, and permission states are represented.
  - Done: `/ui-lab/mcp` now uses the existing `getMCPSummary` API and `mcpQueryKeys.summary`, with KPI tiles, risk strip, operations trends, review queue, top tools, recent errors, detailed drill-ins, and explicit loading/empty/partial/error/admin states.
- [x] Validate UI v2 pilot and decide rollout status.
  - Acceptance: route smoke, typecheck/build, and browser screenshots pass.
  - Acceptance: TODO records whether v2 remains a pilot or is ready for a broader rollout.
  - Done: `bun run smoke:mcp-routes`, `bun run smoke:mcp-trends`, `bun run typecheck`, and `bun run build` passed on 2026-06-13.
  - Done: desktop (1440x1000) and mobile (390x844) browser screenshot validation passed against a mock-auth local preview with `/api/user/self`, `/api/status`, and `/api/mcp/summary`; no console errors were captured.
  - Rollout: keep UI v2 as a pilot behind `/ui-lab/*` and the persisted version switcher; it is deploy-safe because the current UI remains available, but broader rollout should wait for real backend/auth staging validation.

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
- [x] Make subscription post-consume settlement idempotent by request.
  - Acceptance: subscription settlement/finalize retries for the same `request_id` do not apply the final delta more than once.
  - Acceptance: conflicting replay deltas for an already-settled request return a typed error instead of silently mutating subscription usage.
  - Done: subscription pre-consume records now persist `post_consumed_delta` and `settled_at`; subscription funding settlement uses a request-scoped idempotent model path, zero-delta settlements are marked final, and service tests cover replay plus conflict behavior.

## P1 - Relay regression cleanup

- [x] Fix Claude relay OpenAI file-content conversion.
  - Acceptance: unsupported OpenAI `file` content is skipped instead of being sent as an image.
  - Acceptance: PDF files become Claude `document` blocks and text files become Claude `text` blocks.
  - Acceptance: `go test ./relay/channel/claude` and `go test ./relay/channel/...` pass.
  - Done: Claude relay now infers OpenAI file mime type from filename when needed, accepts map payload `filename`, emits PDF/document and text blocks explicitly, and ignores unsupported files.
- [x] Cover StreamScannerHandler per-call StreamStatus reset semantics.
  - Acceptance: pre-existing stream status state from a reused relay info object is not carried into the next stream scan.
  - Done: added a regression test asserting StreamScannerHandler replaces any pre-initialized StreamStatus and starts the new scan with a clean error count.
- [x] Align Coze relay JSON conversion with project wrappers.
  - Acceptance: Coze response decode and OpenAI response encode paths use `common.Unmarshal` / `common.Marshal` instead of direct `encoding/json` conversion calls.
  - Done: Coze adaptor and relay handlers now keep `encoding/json` only for `json.RawMessage` type references while routing runtime JSON conversion through project wrappers.
- [x] Align relay model mapping JSON parsing with project wrappers.
  - Acceptance: model mapping helper no longer imports `encoding/json` only to parse the channel mapping string.
  - Done: `ModelMappedHelper` now uses `common.Unmarshal` for model mapping parsing while preserving the existing cycle-detection behavior.
- [x] Stabilize StreamScannerHandler tests that mutate global streaming settings.
  - Acceptance: parallel stream scanner tests cannot restore `constant.StreamingTimeout` or ping interval settings while another stream scanner test is still running.
  - Done: stream scanner tests now serialize global streaming setting mutations through a package-level test lock while keeping the existing behavior coverage.
- [x] Align simple rerank provider JSON conversions with project wrappers.
  - Acceptance: Ali and SiliconFlow rerank response decode/encode paths no longer call `encoding/json` directly.
  - Done: Ali and SiliconFlow rerank handlers now use `common.Unmarshal` / `common.Marshal` for runtime JSON conversion.
- [x] Align Palm and Jimeng provider JSON conversions with project wrappers.
  - Acceptance: Palm response conversion and Jimeng image/signature helper JSON conversion no longer call `encoding/json` directly.
  - Done: Palm stream/non-stream response paths and Jimeng image response, payload hash, and image extra-field parsing now use project JSON wrappers.
- [x] Align Cloudflare and Cohere provider JSON conversions with project wrappers.
  - Acceptance: Cloudflare and Cohere stream/non-stream response decode/encode paths no longer call `encoding/json` directly.
  - Done: Cloudflare completions/STT and Cohere chat/rerank handlers now use `common.Unmarshal` / `common.Marshal`.
- [x] Align Baidu and Dify provider JSON conversions with project wrappers.
  - Acceptance: Baidu response/token decode and Dify upload/chat response conversion paths use project JSON wrappers.
  - Done: Baidu chat/embedding/token handling and Dify upload/user/stream/blocking response handling now use `common.DecodeJson`, `common.Unmarshal`, or `common.Marshal` as appropriate.
- [x] Align MokaAI, Tencent, Xunfei, and Zhipu provider JSON conversions with project wrappers.
  - Acceptance: simple response/signature JSON conversion paths in these providers no longer call `encoding/json` directly.
  - Done: MokaAI embedding, Tencent chat/signature payload, Xunfei websocket response, and Zhipu stream/non-stream response paths now use project JSON wrappers.
- [x] Align Minimax and Volcengine TTS JSON conversions with project wrappers.
  - Acceptance: TTS metadata request construction and provider response parsing no longer call `encoding/json` directly in Minimax and Volcengine paths.
  - Done: Minimax and Volcengine TTS request/response conversion now uses `common.Unmarshal` / `common.Marshal` while preserving existing request payload shapes.
- [x] Align Vertex, Replicate, OpenAI, and AWS focused JSON conversions with project wrappers.
  - Acceptance: single-purpose provider JSON conversion points use project wrappers while keeping `json.RawMessage` type references intact where needed.
  - Done: Vertex extra-body/token decode, Replicate output format, OpenAI THINKING parsing, AWS anthropic beta/Nova response handling now use `common` JSON helpers.
- [x] Align Claude relay runtime JSON conversions with project wrappers.
  - Acceptance: Claude tool-call argument parsing and OpenAI-format response encoding use project JSON wrappers while preserving `json.RawMessage` type references.
  - Done: Claude relay now uses `common.Unmarshal` / `common.Marshal` for runtime conversion paths covered by the provider package tests.
- [x] Align Gemini relay runtime JSON conversions with project wrappers.
  - Acceptance: Gemini function response/tool call parsing and OpenAI image response encoding use project JSON wrappers while preserving `json.RawMessage` thought-signature fields.
  - Done: Gemini relay now uses `common.Unmarshal` / `common.Marshal` for runtime conversion paths covered by provider package tests.
- [x] Align Ollama relay runtime JSON conversions with project wrappers.
  - Acceptance: Ollama request schema/tool argument parsing, version decode, stream chunk parsing, thinking decode, and tool-call argument encoding use project JSON wrappers while preserving `json.RawMessage` DTO fields.
  - Done: Ollama relay and stream handlers now use `common.Unmarshal` / `common.Marshal` for runtime conversion paths.
- [x] Align OAuth and external status controller JSON conversions with project wrappers.
  - Acceptance: GitHub, Discord, OIDC, LinuxDO, WeChat, and Uptime Kuma controller JSON encode/decode paths no longer call `encoding/json` directly.
  - Done: those controllers now use `common.Marshal` / `common.DecodeJson` for request payloads and HTTP JSON response decoding.
- [x] Align channel balance controller JSON response parsing with project wrappers.
  - Acceptance: provider balance response parsing in `controller/channel-billing.go` no longer calls `encoding/json` directly.
  - Done: OpenAI-compatible, AIProxy, API2GPT, AIGC2D, SiliconFlow, DeepSeek, OpenRouter, and Moonshot balance parsing now uses `common.Unmarshal`.
- [x] Align user controller runtime JSON conversions with project wrappers.
  - Acceptance: user login/register/update/admin request decode and sidebar/default-config JSON conversion use project wrappers while preserving `json.Number` type handling.
  - Done: `controller/user.go` now uses `common.DecodeJson`, `common.Marshal`, and `common.Unmarshal` for runtime conversions.
- [x] Align remaining small controller JSON conversions with project wrappers.
  - Acceptance: deployment, misc, Creem top-up, and model metadata controller JSON conversions use project wrappers.
  - Done: `controller/deployment.go`, `controller/misc.go`, `controller/topup_creem.go`, and `controller/model_meta.go` now route runtime JSON conversion through `common` helpers.
- [x] Finish controller runtime JSON conversion cleanup.
  - Acceptance: controller runtime JSON conversion scan has no direct `json.Marshal`, `json.Unmarshal`, `json.NewEncoder`, or `json.NewDecoder` calls outside intentional `json.RawMessage` type references.
  - Done: console migration, model sync, and channel controller JSON paths now use `common` helpers; `controller` package tests pass.
- [x] Align service notification/session JSON conversions with project wrappers.
  - Acceptance: user notification, webhook, worker download payload, Codex OAuth claim decode, and passkey session JSON conversion paths use project wrappers.
  - Done: those service paths now use `common.Marshal` / `common.Unmarshal` while preserving `json.RawMessage` DTO fields where required.
- [x] Align Midjourney service JSON conversions with project wrappers.
  - Acceptance: Midjourney request body rewrite and response parsing no longer call `encoding/json` directly.
  - Done: `service/midjourney.go` now uses `common.DecodeJson`, `common.Marshal`, and `common.Unmarshal` for runtime JSON conversion.
- [x] Align service conversion JSON helpers with project wrappers.
  - Acceptance: Claude/OpenRouter and Gemini conversion helpers in `service/convert.go` no longer call `encoding/json` directly for runtime marshal/unmarshal paths.
  - Done: `service/convert.go` now uses `common.Marshal` and `common.Unmarshal` for reasoning metadata, tool arguments, and generic helper serialization.
- [x] Align model runtime JSON conversions with project wrappers.
  - Acceptance: model-layer setting, pricing endpoint, passkey transport, channel metadata, and prefill JSON serialization paths use project wrappers while preserving `json.RawMessage` type references.
  - Done: `model/pricing.go`, `model/user.go`, `model/passkey.go`, `model/channel.go`, and `model/prefill_group.go` now route runtime conversion through `common` helpers where applicable.
- [x] Align common utility JSON conversions with project wrappers.
  - Acceptance: common utility helpers use the package JSON wrapper functions instead of direct standard-library calls, while `common/json.go` remains the wrapper implementation boundary.
  - Done: `common/utils.go`, `common/topup-ratio.go`, and `common/str.go` now call `Marshal` / `Unmarshal` directly.
- [x] Align cache codec and Midjourney proxy JSON conversions with project wrappers.
  - Acceptance: Redis JSON codec and Midjourney proxy task/response conversion paths no longer call `encoding/json` directly.
  - Done: `pkg/cachex/codec.go` and `relay/mjproxy_handler.go` now use `common.Marshal` / `common.Unmarshal` for runtime JSON conversion.
- [x] Align io.net client JSON conversions with project wrappers.
  - Acceptance: io.net request encoding, response/error decoding, flexible-time normalization, and query array encoding no longer call `encoding/json` directly.
  - Done: `pkg/ionet` now uses `common.Marshal` / `common.Unmarshal` across client, deployment, hardware, container, and JSON utility paths.
- [x] Finish cross-module runtime JSON conversion cleanup.
  - Acceptance: runtime `json.Marshal`, `json.Unmarshal`, `json.NewEncoder`, and `json.NewDecoder` scan across controller/service/model/pkg/relay/common is clean outside the `common/json.go` wrapper boundary and non-runtime comments.
  - Done: global scan now reports only `common/json.go` wrapper implementation calls and one explanatory comment in `relay/channel/task/taskcommon/helpers.go`.

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
- [x] Add shared unsupported adaptor error semantics.
  - Acceptance: provider adaptor stubs can return a typed unsupported-feature error with provider and feature context while preserving existing user-facing `not implemented` wording.
  - Acceptance: tests can assert typed unsupported errors without brittle string-only checks.
  - Done: added `channel.UnsupportedFeatureError` / `NewUnsupportedFeatureError` and typed unsupported adaptor tests.
- [x] Migrate generic provider adaptor stubs in small batches.
  - Acceptance: each batch removes generic `errors.New("not implemented")` returns from touched provider adaptors without implementing unsupported capabilities.
  - Acceptance: provider/channel tests pass after each batch.
  - Done first batch: Ali `ConvertGeminiRequest` / `ConvertAudioRequest` and XAI `ConvertGeminiRequest` now return typed unsupported-feature errors.
  - Done second batch: Baidu, Cloudflare, Cohere, Dify, Jina, Mistral, MokaAI, Palm, Tencent, Xunfei, and Zhipu generic unsupported adaptor methods now use typed unsupported-feature errors.
  - Done third batch: DeepSeek, Gemini, Minimax, Moonshot, Ollama, Perplexity, and SiliconFlow unsupported adaptor methods now use typed unsupported-feature errors.
  - Done fourth batch: AWS, Baidu v2, Claude, Coze, Jimeng, Replicate, Vertex, Volcengine, and Zhipu 4V unsupported adaptor methods now use typed unsupported-feature errors; old provider `TODO implement me` / `errors.New("not implemented")` scan is clean.
- [x] Refresh non-MCP backlog audit after adaptor error hygiene.
  - Acceptance: remaining provider `TODO` / `not implemented` scan findings are either migrated, intentionally deferred, or documented with explicit rationale.
  - Done: `docs/non-mcp-backlog-audit.md` now reflects completed provider unsupported migration and identifies Coze content handling, Cohere stream usage, API version normalization, and DTO shape tests as the next non-MCP backlog.
- [x] Audit Coze non-text content handling.
  - Acceptance: Coze response conversion has tests for known text, image/file or unknown content blocks, and behavior is deterministic for unsupported content.
  - Done: Coze request conversion now maps OpenAI string/text content to Coze text, image/file/video content to `object_string` payloads, and unsupported media to deterministic text placeholders; tests cover text, image, file, video, and unsupported mixed-content paths.
- [x] Verify Cohere streaming usage behavior.
  - Acceptance: Cohere stream usage fallback is tested and documented, or fixed if stream chunks expose usage metadata.
  - Done: Cohere stream final responses now use upstream `response.meta.billed_units` when present, always compute `total_tokens`, fill missing prompt/completion fields from local estimates only when needed, and emit a final usage chunk before `[DONE]` when usage output is enabled; tests cover upstream usage, fallback usage, partial metadata, and emitted stream usage.
- [x] Review API version normalization.
  - Acceptance: `middleware/distributor.go` API version behavior is traced with tests before any normalization logic changes.
  - Done: provider metadata setup is now isolated in `setupProviderMetadataContext`; tests lock the current Azure/Xunfei/Gemini/Cloudflare/MokaAI `api_version`, Vertex `region`, Ali `plugin`, Coze `bot_id`, and OpenAI no-op behavior, plus `GetAPIVersion` query-over-context precedence. No provider behavior was changed.
- [x] Add DTO shape compatibility tests.
  - Acceptance: audio stream lifecycle and Gemini thinking-budget conflict TODOs are covered by focused DTO/provider compatibility tests.
  - Done: audio request tests lock `stream_format=sse` as the only stream trigger and document that boolean `stream` is ignored; Gemini generation config tests lock budget+level coexistence, snake_case parsing, and snake-over-camel precedence. The DTO TODO comments were replaced with explicit compatibility notes.

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
