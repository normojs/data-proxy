# Request Capture and Diagnostics Implementation Plan

Date: 2026-06-23

This project is based on `new-api` and keeps the upstream AGPL attribution and
NOTICE requirements.

## Development Order

### Phase 1: Metadata and Migration

Status: implemented in code.

- Add request capture metadata tables.
- Add capture artifact metadata tables.
- Add request diagnostic report metadata tables.
- Add training dataset and sample lineage tables.
- Register the tables in both normal migration and fast migration paths.

This phase does not capture raw traffic yet. It only prepares the database
schema.

### Phase 2: SeaweedFS Object Store Foundation

Status: implemented in code.

- Add S3-compatible object store adapter for capture bundles.
- Use SeaweedFS through the S3 API in production.
- Keep capture disabled by default until an explicit policy and finalizer are
  enabled.
- Add production Docker Compose service and volume mappings for SeaweedFS and
  local capture spool directories.
- Add `.env.example` capture storage variables.

This phase provides `Save`, `Load`, and `Delete` primitives for encrypted
capture bundle objects. It does not upload stream chunks directly.

### Phase 3: Capture Session and Spool Writer

Status: foundation implemented, minimal relay integration implemented.

- Add capture policy matcher by time window, user, token, channel, model,
  request path, protocol conversion, connected app, severity flag, and sample
  rate. Environment-based foundation implemented in
  `service/request_capture_policy.go`:
  - `CAPTURE_LEVEL`
  - `CAPTURE_SAMPLE_RATE`
  - `CAPTURE_MODEL_PATTERNS`
  - `CAPTURE_PATH_PREFIXES`
  - `CAPTURE_USER_IDS`
  - `CAPTURE_TOKEN_IDS`
  - `CAPTURE_CHANNEL_IDS`
  - `CAPTURE_CONNECTED_APP_IDS`
  - `CAPTURE_MAX_ARTIFACT_BYTES`
  Time-window and severity-flag policies are pending.
- Add local capture session lifecycle. Implemented as
  `service/request_capture_spool.go`:
  - create `active/{request_id}` spool directory
  - write request/upstream/downstream artifacts under `artifacts/`
  - append stream chunks to local artifact files
  - truncate oversized artifacts and mark `truncated=true`
  - maintain `manifest.json`
  - move completed sessions to `finalize/`
  - move failed sessions to `failed/`
- Minimal relay request-path integration is implemented in `controller/relay.go`
  and `service/request_capture_relay.go`:
  - create metadata record at request start
  - capture client request body for `sanitized_bundle` / `full_bundle`
  - capture raw upstream response body before provider-specific conversion
  - capture downstream response bytes through a temporary Gin response writer
    wrapper
  - move successful sessions to `finalize/`
  - move failed sessions to `failed/`
- Remaining response capture improvements:
  - replace synchronous artifact appends with buffered async writes for high
    token-rate streams
- Ensure capture is fail-open and never breaks normal relay traffic.
- Add restart recovery for old `active`, `finalize`, and `failed` spool
  directories. Pending.

### Phase 4: Finalizer Worker

Status: single-session finalizer foundation implemented, background worker
pending.

- Build `manifest.json`. Implemented by the spool writer.
- Archive spool files into tar. Implemented in
  `service/request_capture_finalizer.go`.
- Compress with zstd. Implemented.
- Encrypt with AES-256-GCM before upload. Implemented; requires
  `CAPTURE_BUNDLE_MASTER_KEY`.
- Upload the encrypted bundle to SeaweedFS/S3-compatible storage. Implemented.
- Return artifact metadata for persistence. Implemented.
- Persist artifact metadata and capture status in MySQL. Implemented as
  `PersistRequestCaptureFinalizeResult`.
- Add background worker and Redis/job scheduling. Core scanner implemented as
  `FinalizePendingRequestCaptureSpool`; periodic master-node scheduler is
  wired through `StartRequestCaptureFinalizerTask`; Redis job queue is pending.
- Retry failed uploads with backoff. Pending; failed finalizer attempts keep
  the spool directory in `finalize/` for retry.
- Clean old spool files after successful upload. Implemented for the scanner
  when `RemoveOnSuccess=true`.

### Phase 5: Online Diagnostics

Status: minimal request-id report loop and candidate API implemented; raw
bundle download pending.

- Add request id based diagnostic candidates list. Implemented via
  `GET /api/log/request-diagnostic-candidates`.
- Generate diagnostic reports from normal logs and trace metadata. Implemented
  via `POST /api/log/request/:request_id/diagnostic`.
- Enrich reports with raw capture bundle metadata when available. Implemented
  for capture records and object artifact metadata; raw payload is not exposed.
- Add diagnostic bundle download for authorized administrators.
- Add UI shortcuts from usage log request id to trace and diagnostics.
  Implemented in the usage log detail dialog.

### Phase 6: Training Corpus Builder

Status: planned.

- Read encrypted raw bundles in a worker.
- Decrypt in memory only.
- Redact secrets and PII.
- Normalize Responses and Chat Completions variants into training schemas.
- Produce versioned JSONL.zst or Parquet shards in SeaweedFS.
- Record dataset version and sample lineage.
- Add review and approval workflows before using samples for model training.

## Deployment Notes

Production storage root:

```text
/root/workspace/dataproxy/storage
```

Required persistent paths:

```text
/root/workspace/dataproxy/storage/seaweedfs
/root/workspace/dataproxy/storage/capture/spool
/root/workspace/dataproxy/storage/capture/tmp
```

Tracked Compose overlay:

```text
docker-compose.capture-storage.yml
```

Production start command when capture storage is needed:

```bash
docker compose -f docker-compose.prod.yml -f docker-compose.capture-storage.yml up -d
```

SeaweedFS is only attached to the Docker internal network. Data Proxy talks to
it through:

```text
http://seaweedfs:8333
```

Raw capture should remain disabled until Phase 3 and Phase 4 are complete and
tested on a small scoped policy.

## Current Safe Rollout

The current safe rollout is:

1. Deploy database migration and SeaweedFS volume wiring.
2. Keep `CAPTURE_ENABLED=false`.
3. Verify Data Proxy starts normally.
4. Verify SeaweedFS container and volume persistence.
5. Enable capture only after capture session, finalizer, encryption, and admin
   access controls are implemented.
