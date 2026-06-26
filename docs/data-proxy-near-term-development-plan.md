# Data Proxy Near-Term Development Plan

Date: 2026-06-26

This project is based on `new-api`; keep the upstream AGPLv3 license, NOTICE,
attribution, and third-party license requirements visible in every release.

## Product Decision

Protocol-conversion long-tail work is deferred until after the next vNext
release is stable.

For the current release window, keep only the already implemented safe behavior:

- native Responses channels pass hosted tools through unchanged;
- Chat-only channels keep the current hosted-tool policy behavior;
- `web_search` / `web_search_preview` may use the existing webhook executor
  bridge when an administrator explicitly enables it;
- unsupported hosted tools remain filtered, rejected, or routed to native
  Responses according to channel policy.

Do not start new executor work for:

- `file_search`;
- `computer` / `computer_use`;
- `code_interpreter`;
- `image_generation`;
- hosted `mcp`;
- `shell` or local runtime execution.

Only fix protocol conversion regressions that break existing production traffic
or billing/usage accounting.

## Current Goal

Ship a deployable single-node Data Proxy build that is stable, diagnosable,
and manageable on one server.

The current execution checklist is maintained in
`docs/data-proxy-follow-up-task-board.md`.
The detailed current development checklist is maintained in
`docs/data-proxy-current-development-tasks.md`.
The current release execution order is maintained in
`docs/data-proxy-current-release-execution-plan.md`.

The near-term priority is not broadening model protocol features. The priority
is making the current server usable in production:

1. clean release state;
2. predictable deploy and rollback;
3. request diagnosis by request id;
4. channel failover and user group restrictions;
5. Tunnel/dpa minimum productization;
6. safe request capture and training-data review basics.

## Execution Queue

### Sprint 0: Release Freeze

Goal: produce a deployable build without adding protocol-conversion scope.

Tasks:

- Freeze protocol conversion to regression fixes only.
- Do not schedule protocol-conversion long-tail as a current-version feature.
- Review the dirty worktree and split it into feature-sized commits.
- Run backend and frontend validation for the files included in each commit.
- Confirm secret hygiene before commit and release.
- Build or trigger the Docker image through CI and record the image digest.
- Deploy to the single server and run the production smoke runbook.

Exit criteria:

- The current production server runs the latest tagged image.
- `/api/status`, normal Chat, Responses native, Responses-via-Chat, request
  trace, diagnostic bundle, and same-model channel failover have smoke evidence.
- Rollback image and command are recorded.

### Sprint 1: Routing And Access Controls

Goal: make model access and same-model backup channel behavior predictable.

Tasks:

- Finish channel failover/circuit-breaker UI review and Chinese copy.
- Verify retry attempts, temporary fault rules, hard fault rules, thresholds,
  cooldown, and admin reset behavior.
- Verify user group binding end to end:
  - empty binding means unrestricted;
  - one or many groups means restricted;
  - key creation and key usage respect the binding;
  - legacy keys outside the binding show group unavailable.
- Add or run focused tests for user edit, token list/create/update, model list,
  relay authorization, and channel retry traces.

Exit criteria:

- A failed channel can automatically retry a backup channel for the same model
  when retry settings allow it.
- A group-bound user cannot create or use keys outside assigned groups.

### Sprint 2: Diagnostics, Capture, And Training Data

Goal: make production failures and captured training data reviewable without
SSH-only workflows.

Tasks:

- Polish request diagnostic candidate search and bundle download docs/UI.
- Verify capture spool restart recovery, finalizer retry backoff, cleanup, and
  fail-open streaming capture.
- Add capture policy selectors for time window and severity flags.
- Build the first training corpus review UI:
  - dataset versions;
  - redacted sample preview;
  - approve/reject;
  - export with request id and redaction version.

Exit criteria:

- Admins can find suspicious request ids, generate a diagnostic bundle, and
  analyze it locally.
- Captured samples cannot enter a training export without review.

### Sprint 3: Tunnel And dpa Production MVP

Goal: make Tunnel Apps and the `dpa` agent usable by early users on one server.

Tasks:

- Add console-visible install/enroll commands for `dpa`.
- Show agent version, health, routes, bridge status, and MCP process summary.
- Verify HTTP Tunnel stream behavior for WebSocket, SSE, large upload/download,
  and client disconnect.
- Keep TCP Tunnel as TCP-over-WebSocket MVP for now.
- Add billing risk controls for Tunnel:
  - postpaid idempotent settlement as default;
  - optional hard concurrency limit;
  - denied/failed/charged audit visibility.

Exit criteria:

- A user can create a Tunnel App, enroll `dpa`, expose one local route, test it,
  and see audit/billing behavior without manual JSON editing.

## P0: Release And Production Baseline

### P0-01 Worktree And Secret Hygiene

Tasks:

- Split current large worktree into reviewable feature commits.
- Keep secrets, `.env.production`, merchant keys, certificates, diagnostic
  bundles, raw capture bundles, and local storage out of Git.
- Run whitespace checks and a secret scan before every release commit.
- Preserve `new-api` license and attribution files.

Acceptance:

- `git status --short` only contains intentional files before commit.
- No private keys, API keys, certificates, local capture data, or diagnostic
  bundles are tracked.
- CI can run from a clean checkout.

### P0-02 CI And Docker Release Chain

Tasks:

- Ensure GitHub Actions build, test, and Docker image publishing still pass.
- Keep image tags, digest, and rollback image recorded in the release runbook.
- Keep `docker-compose.prod.yml` compatible with the existing external Nginx
  deployment.
- Keep SeaweedFS/capture storage as an optional compose overlay.

Acceptance:

- A GitHub tag can build a production image.
- The server can roll back to the previous image without DB or volume loss.
- Nginx SSE/WebSocket/large-streaming proxy settings remain documented.

### P0-03 Production Smoke Runbook

Tasks:

- Run a smoke set after deployment:
  - `/api/status`;
  - normal `/v1/chat/completions`;
  - `/v1/responses` through native and Chat-only channels;
  - request id visible in common logs;
  - request trace and diagnostic bundle generation;
  - channel failover between two same-model channels;
  - payment/top-up happy path if payment is enabled;
  - Tunnel App connection list and one `dpa status --json` report.
- Record request ids and image digest in the release notes.

Acceptance:

- A bad request can be found from request id and diagnosed without SSHing into
  application internals.
- A failed same-model channel can retry to a backup channel when retry settings
  permit it.

## P1: Channel Routing, Groups, And Failover

### P1-01 Channel Failover Hardening

Tasks:

- Verify retry settings UI and documentation are clear:
  - retry attempts;
  - hard fault rules;
  - temporary fault rules;
  - threshold, window, cooldown, and max cooldown.
- Add production smoke coverage for:
  - retryable 5xx/429 temporary failure;
  - hard 401/credential failure;
  - backup channel selection for the same model;
  - admin reset of temporary circuit state.
- Keep temporary circuit state single-node for now.

Acceptance:

- Operators can configure which failures are temporary and which should hard
  disable a channel.
- Request trace shows selected channel, failed channel, retry decision, and
  circuit action for admins.

### P1-02 User Group Binding And Key Restrictions

Tasks:

- Verify user-bound model groups:
  - user may have zero, one, or multiple allowed groups;
  - empty group binding means unrestricted legacy behavior;
  - user-bound groups restrict model list, key creation, and key usage;
  - keys created before binding show group unavailable instead of silently
    routing outside the user's allowed groups.
- Add regression tests for admin user edit, token create/update, model list,
  and relay authorization.
- Make the UI message clear in Chinese.

Acceptance:

- A bound user cannot create or use a key outside assigned groups.
- Existing incompatible keys fail clearly and are easy to identify.

## P2: Tunnel And dpa Productization

### P2-01 Tunnel Billing Risk Controls

Tasks:

- Keep current settlement as default postpaid/idempotent debit.
- Add stricter optional controls:
  - prepaid reservation for long-lived HTTP/TCP sessions;
  - concurrency-safe hard limit;
  - automatic pause for high error rate or overdue balance;
  - clear audit for denied, failed, and charged events.

Acceptance:

- Billing behavior is explicit per Tunnel App type.
- Failed/denied traffic is auditable even when not charged.

### P2-02 MCP Gateway Usability

Tasks:

- Improve allowed/denied tool, resource, and prompt visibility in the console.
- Group audit by gateway session id and connection id.
- Add more single-node Streamable HTTP/SSE compatibility samples.
- Keep old SSE cross-node behavior out of current scope.

Acceptance:

- A user can understand which local MCP tools are exposed and why a call was
  denied.
- An admin can trace one AI client session across initialize, list, call, and
  close events.

### P2-03 HTTP/TCP Tunnel Hardening

Tasks:

- Extend stress tests for WebSocket, SSE, large uploads/downloads, and client
  disconnects.
- Keep HTTP CONNECT and login-state access as explicit future work until route
  and security rules are clearer.
- For TCP, keep current TCP-over-WebSocket MVP; defer raw TCP listener, port
  pool, and connection reuse.

Acceptance:

- Long-lived streams do not leave orphaned Bridge calls.
- Byte counters, close reasons, and rate-limit denials are visible in audit.

### P2-04 dpa Onboarding

Tasks:

- Surface one-command install and enroll commands in the console.
- Show agent version, platform, health, local routes, and MCP process summary.
- Support multiple agents per user in the UI.
- Keep Windows ConPTY, Homebrew tap auto-submit, enterprise config push, and
  advanced terminal UX as later product polish.

Acceptance:

- A user can install `dpa`, enroll it, add a local MCP/HTTP/TCP route, test the
  route, and connect it to an approved Tunnel App without manual JSON editing.

## P3: Request Capture And Training Data

### P3-01 Capture Policy Polish

Tasks:

- Keep raw capture disabled by default.
- Add time-window and severity-flag policy selectors.
- Keep capture fail-open and bounded by byte caps.
- Improve admin wording around raw bundle sensitivity.

Acceptance:

- Admins can enable capture narrowly for a token, model, channel, path, time
  window, or suspicious failure type.
- Capture failure never breaks user traffic.

### P3-02 Training Corpus Review UI

Tasks:

- Build admin UI for dataset versions and sample review.
- Preview redacted sample content.
- Approve/reject samples before export.
- Add richer export policy later.

Acceptance:

- Training data can be reviewed before use.
- Every exported sample links back to source request id and redaction version.

## Deferred vNext Tracks

These are explicitly not part of the near-term release:

- protocol-conversion hosted tool executors beyond the current `web_search`
  webhook MVP;
- local computer-use executor bridge;
- local shell/code executor bridge through Responses hosted-tool semantics;
- file-search/retrieval bridge;
- image-generation executor bridge;
- hosted MCP conversion to local MCP gateway;
- multi-node Data Proxy deployment;
- distributed Tunnel rate limits and distributed bandwidth settlement;
- raw TCP public listener and port pool.

## Immediate Next Steps

1. Freeze protocol conversion scope for the current release.
2. Clean and split the current worktree into feature commits.
3. Run focused backend tests for channel failover, user group binding, request
   diagnostics, Tunnel, and dpa.
4. Run frontend typecheck/build after the UI-bearing commits are isolated.
5. Deploy the tagged image to the single server and record smoke evidence.
6. Leave protocol-conversion long-tail items in the post-vNext list unless a
   current production regression forces a narrow fix.

## Validation Snapshot

2026-06-26:

- Fixed the token controller test fixture so token-owner users are seeded
  idempotently before token API masking tests run. This keeps the newer
  user-bound token-group lookup from breaking older masking tests.
- Passed focused backend release baseline:

```bash
go test ./model ./service ./controller ./router ./pkg/dpagent ./pkg/bridgepolicy ./service/openaicompat ./relay/channel/openai ./relay
```

- Passed frontend typecheck by directly invoking the installed local
  TypeScript binary because `bun` is not available in the current shell and
  `pnpm` attempts to reinstall dependencies without the bun catalog context:

```bash
cd web/default
PATH="/Users/fushilu/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH" ./node_modules/.bin/tsc -b
```

- Added a reusable P0 release gate:

```bash
scripts/data-proxy-release-gate.sh
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
scripts/data-proxy-release-gate.sh --scan-all
```

- Verified the gate in the current worktree. The `--with-tests` mode passed
  focused backend tests for model/service/controller/router/dpa/bridge-policy/
  OpenAI compatibility/relay packages and frontend TypeScript build mode. The
  `--with-docker-config` mode passed production compose validation with
  `docker-compose.prod.yml` plus `docker-compose.wechat-pay.yml`. The
  `--scan-all` mode passed after filtering obvious token example/comment lines,
  and CI now runs this mode in the Backend job.
- Ran P1 focused checks for channel failover, circuit-breaker state, user group
  binding, and token group restrictions:

```bash
go test ./service -run 'Test(Channel|Select|Group|UserToken|Circuit|Failover)' -count=1
go test ./controller -run 'Test(Channel|Token|UserToken|Group|Failover)' -count=1
go test ./model -run 'Test(Channel|UserToken|Group|Failover)' -count=1
```

- Added `scripts/data-proxy-production-smoke.sh` for post-deploy smoke. It can
  run minimal `/api/status`, optional Chat/Responses relay checks with
  `DATA_PROXY_API_KEY`, and optional admin request trace/diagnostic report
  checks with `DATA_PROXY_ADMIN_HEADER` and `DATA_PROXY_SMOKE_DIAGNOSTIC=1`.
  The script was syntax-checked and exercised against a temporary local mock
  server covering status, Chat, Responses, diagnostic candidates, request trace,
  diagnostic report generation, and diagnostic bundle download.
- Added `scripts/data-proxy-focused-regression.sh` as the P1/P2/P3 focused
  validation entrypoint. It covers channel failover, circuit-breaker state,
  user group and token restrictions, request diagnostics, capture/finalizer/
  cleanup paths, training dataset APIs, Tunnel, MCP Gateway, `dpa`, Bridge
  policy, and HTTP/TCP tunnel regressions. The script syntax check, default
  P1/P2/P3 run, release gate, production compose config validation, and
  targeted `git diff --check` all passed in the current worktree.
