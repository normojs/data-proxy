# Data Proxy Next Iteration Task Plan

Date: 2026-06-26

This project is based on `new-api`. Keep the upstream AGPLv3 license,
`NOTICE`, attribution text, and third-party license obligations visible in
source, docs, release artifacts, and deployment notes.

## Current Boundary

Protocol conversion long-tail work is not part of the current queue. It should
be reopened only after the next vNext release is deployed and stable.

Do not schedule these items now:

- `file_search`;
- `computer` / `computer_use`;
- `code_interpreter`;
- `image_generation`;
- hosted `mcp` auto-bridging;
- `shell` or any local runtime executor;
- generic hosted-tool executor expansion;
- provider-specific long-tail Responses event emulation.

Allowed protocol work in this release is limited to production regression
repair: blank response fixes, request id/logging fixes, usage accounting fixes,
diagnostic metadata fixes, and keeping already shipped behavior from breaking.

## Execution Order

### 1. RC0 Release Baseline

Goal: get a deployable, rollbackable, diagnosable single-node build.

Tasks:

- Split the current dirty worktree into small feature commits.
- Keep protocol long-tail files out of current feature commits unless they are
  needed for a narrow production regression fix.
- Run staged-file audit before each commit.
- Run release gate, focused regression, and frontend typecheck for changed
  areas.
- Confirm secrets are not tracked: `.env.production`, certificates, WeChat
  merchant keys, API keys, diagnostic bundles, raw capture bundles, object-store
  data, and local runtime data.
- Build through GitHub Actions or local Docker as needed.
- Deploy to the single server and record image tag, digest, previous image, and
  rollback command.

Acceptance:

- Latest image is running in production.
- Rollback command is documented and tested or at least smoke-verified.
- Smoke evidence includes request ids and image digest.

### 2. P1 Channel Failover And User Group Restrictions

Goal: a bad same-model channel can fail over automatically, and users can be
restricted to specific model groups.

Tasks:

- Finish failover/circuit-breaker UI and Chinese wording.
- Expose retry times, temporary fault rules, hard fault rules, threshold,
  window, cooldown, max cooldown, and admin reset.
- Ensure 5xx/429 style temporary failures retry another enabled same-model
  channel when retry settings allow it.
- Ensure hard failures such as auth errors do not retry forever.
- Show selected channel, failed channel, retry decision, and circuit action in
  request trace.
- Finish user-bound model groups:
  - empty binding means unrestricted;
  - one or many groups means restricted;
  - key create/update/use follows the binding;
  - legacy keys outside the binding show group unavailable.

Acceptance:

- Same-model backup failover has tests and production smoke evidence.
- Group-bound users cannot create or use keys outside their allowed groups.

### 3. P2 Request Diagnostics And Capture Safety

Goal: production issues can be diagnosed from the console by time range or
request id, without relying on SSH-only workflows.

Tasks:

- Document request trace API/UI and diagnostic bundle usage.
- Add common-log shortcuts from request id to trace detail or filtered logs.
- Add diagnostic candidate filters for time, model, channel, status, failover,
  capture, and conversion metadata.
- Keep sanitized diagnostic bundles separate from raw capture bundles.
- Keep capture disabled by default and enable it only by narrow policy: user,
  token, model, channel, path, time window, or abnormal request type.
- Verify capture spool restart recovery, finalizer retry backoff, cleanup,
  stream fail-open behavior, and byte caps.

Acceptance:

- Admins can find suspicious request ids, generate reports, and download
  diagnostic bundles.
- Capture failures do not interrupt user requests or streaming output.

### 4. P3 Tunnel, MCP Gateway, And `dpa`

Goal: early users can create a Tunnel App, enroll `dpa`, expose local
MCP/HTTP/TCP routes, and inspect audit plus basic billing.

Tasks:

- Finish Tunnel App UI for per-user connection, random public prefix,
  delete/revoke, route status, and online state.
- Improve MCP Gateway visibility for allowed/denied tools, resources, prompts,
  session id, connection id, and deny reasons.
- Harden HTTP Tunnel for WebSocket, SSE, large upload/download, client
  disconnects, byte counters, close reasons, and orphan cleanup.
- Keep TCP Tunnel as TCP-over-WebSocket MVP.
- Finish single-node billing risk controls: postpaid idempotent settlement,
  optional balance check, optional concurrency limit, and denied/failed/charged
  audit.
- Productize `dpa` enough for single-node use: install command, enroll command,
  route add/test, `status --json`, `doctor --json`, update manifest, and local
  MCP process health summary.

Acceptance:

- A user can complete create app -> enroll `dpa` -> add route -> test route ->
  inspect audit/billing without hand-editing JSON.

### 5. P4 Training Data Review

Goal: saved request/response traffic can become reviewable training data, but
raw data remains private and exports require approval.

Tasks:

- Store raw bundles in a lightweight object store, preferably SeaweedFS or an
  S3-compatible provider, not custom storage.
- Write streaming data asynchronously, in bounded chunks, with fail-open
  behavior.
- Build a unified training sample schema for Chat and Responses traffic.
- Track request id, tenant/user, model, channel, redaction version, source hash,
  review status, reviewer, and dataset version.
- Add admin review UI for dataset build, redacted preview, approve, reject, and
  approved-only export.
- Export a versioned `jsonl.zst` format first.

Acceptance:

- Exported training data contains approved samples only.
- Every sample can be traced back to request id and redaction version.

### 6. P5 Packaging And Release Polish

Goal: after the server release is stable, improve install, upgrade, and
rollback experience.

Tasks:

- Keep Docker image as the primary server deployment path.
- Keep historical image tags, digests, and rollback notes.
- Generate `dpa` release manifest and sha256 checksums in CI.
- Continue Homebrew, deb, rpm, MSI, Windows service helper, macOS
  notarization, keychain/secret-store, and PTY terminal polish.

Acceptance:

- Server release and rollback are repeatable.
- `dpa` binaries and packages can be verified before installation.

## Current Commit Split

Use this order while cleaning the dirty worktree:

1. RC0 release baseline: docs, README, CI, Docker, `.env.example`, makefile,
   audit scripts, deployment/runbook updates.
2. P1 failover and user groups.
3. P2 diagnostics and capture safety.
4. P3 Tunnel/MCP Gateway/`dpa`.
5. P4 training data review.
6. Pricing/billing polish if still separate from Tunnel billing.
7. Benchmark tooling cleanup, if still needed.

Protocol conversion long-tail should not get a feature commit in this sequence.
If there are protocol files in the worktree, either leave them for a later
post-vNext branch or include only narrow regression-guard changes with focused
tests and explicit notes.

## Validation Commands

Worktree and staging audit:

```bash
scripts/data-proxy-worktree-audit.sh
scripts/data-proxy-worktree-audit.sh --staged
git diff --check
```

Release gates:

```bash
scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-release-gate.sh --with-tests
scripts/data-proxy-release-gate.sh --with-docker-config
```

Focused regressions:

```bash
scripts/data-proxy-focused-regression.sh --p1 --frontend
scripts/data-proxy-focused-regression.sh --p2 --frontend
scripts/data-proxy-focused-regression.sh --p3
```

Protocol regression tests only when a narrow production fix touches conversion
files:

```bash
go test ./service/openaicompat ./relay/channel/openai ./relay -count=1
```
