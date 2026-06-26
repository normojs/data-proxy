# Data Proxy Current Release Execution Plan

Date: 2026-06-26

This release is a single-node production release. The goal is to ship a
deployable, rollbackable, diagnosable Data Proxy build first, then continue
feature hardening in a controlled order.

The project is based on `new-api`; keep AGPLv3, `NOTICE`, attribution, and
third-party license obligations visible in every release.

## Scope Freeze

Protocol-conversion long-tail work is deferred until after the next vNext
release is stable and must not drive the current release.

Current release may keep or fix only the existing conversion behavior:

- native Responses channels stay native;
- Chat-only channels use the current Responses/Chat compatibility path;
- existing hosted-tool policy remains visible in request trace metadata;
- webhook-backed `web_search` / `web_search_preview` MVP may remain if already
  wired and guarded by explicit admin configuration;
- regressions that cause blank responses, broken usage accounting, broken
  request traces, or production failures may be fixed narrowly.

Current release must not add new long-tail executors or bridges for:

- `file_search`;
- `computer` / `computer_use`;
- `code_interpreter`;
- `image_generation`;
- hosted `mcp`;
- `shell` or other local runtime execution;
- new generic hosted-tool executor framework expansion.

These items go to a post-vNext backlog after the next vNext release is stable.

## Priority Order

### P0: Release Baseline

Goal: turn the current worktree into a clean release candidate.

Tasks:

- Split the dirty worktree by feature line instead of one large commit.
- Before each commit, check staged files only and avoid unrelated file churn.
- Keep secrets out of Git:
  `.env.production`, certificates, WeChat merchant keys, API keys, diagnostic
  bundles, raw capture bundles, and object-store data.
- Run the release gate and focused tests for the staged feature.
- Confirm GitHub Actions can build the production Docker image.
- Record image tag, digest, previous image tag, and rollback command.
- Deploy to the single server and run production smoke.

Acceptance:

- Production runs the latest image.
- A previous image can be restored with a documented command.
- Smoke evidence includes request ids and the image digest.

### P1: Channel Failover And User Group Restrictions

Goal: if two channels serve the same model, temporary upstream failure can
retry the backup channel; user-bound model groups prevent accidental overuse.

Tasks:

- Verify retry count, temporary fault rules, hard fault rules, thresholds,
  time window, cooldown, max cooldown, and admin reset UI.
- Validate retryable 5xx/429 style failures switch to another enabled channel.
- Validate hard failures such as credential/auth errors are not retried
  endlessly.
- Show selected channel, failed channel, retry decision, and circuit action in
  request trace.
- Finish user-bound model group behavior:
  empty binding means unrestricted, one or many groups restrict access, key
  create/update/use all respect the binding, and legacy keys outside allowed
  groups show "group unavailable".

Acceptance:

- Same-model backup failover is reproducible in tests and smoke.
- A group-bound user cannot create or use keys outside assigned groups.

### P2: Request Diagnostics And Capture Safety

Goal: production failures can be diagnosed from the console by time range or
request id, without SSH-only workflows.

Tasks:

- Document request trace API/UI in README or troubleshooting docs.
- Add shortcuts from common logs request id to trace detail or filtered logs.
- Keep diagnostic candidate search by time range, model, channel, abnormal
  status, conversion/capture/failover flags.
- Keep diagnostic bundles access-controlled and separate from raw capture.
- Verify capture spool restart recovery, finalizer single-process retry
  backoff, cleanup, fail-open stream capture, and artifact byte caps.
- Capture should remain disabled by default and enabled only by narrow policy:
  user, token, model, channel, path, time window, or abnormal request type.

Acceptance:

- Admin can find suspicious request ids, generate a report, download a bundle,
  and analyze locally.
- Capture failure never interrupts user requests or streaming responses.

### P3: Tunnel, MCP Gateway, And `dpa`

Goal: an early user can create a Tunnel App, install `dpa`, expose local
MCP/HTTP/TCP routes, and see audit plus billing behavior on one server.

Tasks:

- Finish Tunnel App UI for per-user connection key, random public prefix,
  delete/revoke, route visibility, and status.
- Improve MCP Gateway visibility for allowed/denied tools, resources, prompts,
  session id, connection id, and permission-deny audit.
- Keep HTTP Tunnel hardening focused on WebSocket, SSE, large upload/download,
  client disconnect, byte counters, close reasons, and orphaned call cleanup.
- Keep TCP Tunnel as TCP-over-WebSocket MVP; defer raw TCP public listener,
  port pool, and connection reuse.
- Finish single-node Tunnel billing risk controls:
  postpaid idempotent settlement by default, optional positive/sufficient
  balance check, optional concurrency limit, denied/failed/charged audit.
- Productize `dpa` enough for one-server use:
  install command, enroll command, route add/test command, `status --json`,
  `doctor --json`, update manifest, local MCP process health summary.

Acceptance:

- A user can complete create app -> enroll `dpa` -> add route -> test route ->
  inspect audit/billing without editing JSON by hand.

### P4: Training Data Review

Goal: captured traffic can become reviewable training samples, but raw data
remains private and exports require approval.

Tasks:

- Finish the training-data admin UI:
  dataset versions, build form, sample filters, redacted preview,
  approve/reject, approved-only export.
- Keep raw bundle storage separate from sanitized diagnostic bundles.
- Track request id, model, channel, redaction version, source hash, review
  status, reviewer, and dataset version.
- Keep export format versioned, preferably `jsonl.zst` first.
- Add retention and permission language before broad production use.

Acceptance:

- Exported training data contains approved samples only.
- Every exported sample traces back to source request id and redaction version.

### P5: Packaging And Release Polish

Goal: once core single-node features are stable, improve installation and
rollback experience.

Tasks:

- Keep amd64 Docker image as the primary server deployment path.
- Keep historical image tags and rollback notes.
- Keep `dpa` release manifest and sha256 checksums generated by CI.
- Continue Homebrew/deb/rpm/MSI/macOS notarization and keychain/secret-store
  work after the server release is stable.

Acceptance:

- Server release and rollback are repeatable.
- `dpa` install/update artifacts can be verified.

## Commit Split

Use this split while cleaning the current worktree:

1. P1 failover, circuit breaker, user group restrictions, and related UI.
2. P2 request trace, diagnostic bundle, capture safety, and diagnostics UI.
3. P4 training data backend/UI review flow.
4. P3 Tunnel, MCP Gateway, HTTP/TCP Tunnel, `dpa`.
5. Protocol conversion regression guard only, if there are narrow production
   fixes already in the worktree.
6. Packaging, README, `.env.example`, CI, and release docs.

Do not create a protocol-conversion feature commit for the current release.

## Validation Commands

Release gate:

```bash
scripts/data-proxy-release-gate.sh
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
```

Focused regressions:

```bash
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
scripts/data-proxy-focused-regression.sh --all --frontend
```

Protocol regression guard only when conversion files are touched:

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```

## Production Smoke

After deployment, verify:

- `/api/status`;
- `/v1/chat/completions`;
- `/v1/responses` native path;
- `/v1/responses` Chat-only compatibility path;
- common logs show request id;
- request trace and diagnostic bundle work;
- same-model channel failover works when retry settings allow it;
- Tunnel connection list shows the enrolled agent;
- `dpa status --json` returns a healthy report;
- payment/top-up happy path if online recharge is enabled.

Record the image digest, request ids, and rollback image in release notes.

## Post-vNext Parking Lot

Reopen only after this release and the next vNext release are deployed and
stable:

- full hosted-tool capability model;
- `file_search` executor or retrieval bridge;
- `computer_use` / browser or desktop automation bridge;
- `code_interpreter` or sandboxed code execution bridge;
- `image_generation` bridge;
- hosted MCP to local MCP Gateway conversion;
- generic executor policy, permission prompts, audit, and billing;
- multi-node Data Proxy;
- distributed Tunnel routing, rate limits, and settlement;
- raw public TCP listener and port pool.
