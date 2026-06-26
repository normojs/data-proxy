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

Status: implemented for the current single-node P0 scope.

- Add capture policy matcher by time window, user, token, channel, model,
  request path, protocol conversion, connected app, severity flag, and sample
  rate. Environment-based foundation implemented in
  `service/request_capture_policy.go`:
  - `CAPTURE_LEVEL`
  - `CAPTURE_SAMPLE_RATE`
  - `CAPTURE_MODEL_PATTERNS`
  - `CAPTURE_PATH_PREFIXES`
  - `CAPTURE_PROTOCOL_CHAINS`
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
- Stream responses use buffered async artifact writers for upstream and
  downstream bodies. If the writer buffer cannot keep up, the artifact is
  marked `truncated=true` and the user response continues.
- Ensure capture is fail-open and never breaks normal relay traffic.
- Add restart recovery for old `active`, `finalize`, and `failed` spool
  directories. Implemented in `RecoverStaleRequestCaptureSpool`.

### Phase 4: Finalizer Worker

Status: implemented for the current single-node P0 scope.

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
  wired through `StartRequestCaptureFinalizerTask`. Redis/distributed queue is
  intentionally out of scope for the single-node plan.
- Retry failed uploads with DB-backed exponential backoff. Failed finalizer
  attempts keep the spool directory in `finalize/`, update
  `finalize_attempts`, `next_finalize_at`, and `last_error`, then retry later.
- Clean old spool files after successful upload. Implemented for the scanner
  when `RemoveOnSuccess=true`.
- Add single-node retention cleanup. Implemented as
  `CleanupExpiredRequestCaptureData` and `StartRequestCaptureCleanupTask`:
  - `CAPTURE_RETENTION_DAYS` controls DB record and object artifact retention;
    default is 30 days, and `0` disables this cleanup dimension.
  - `CAPTURE_SPOOL_RETENTION_DAYS` controls stale local `failed/` spool cleanup
    and uploaded/expired `finalize/` spool cleanup; default is 7 days, and `0`
    disables this cleanup dimension.
  - Available object artifacts are deleted from SeaweedFS/S3-compatible storage
    before being marked `deleted`; capture records are marked `expired`.
  - The background worker is guarded by a single-process atomic lock and only
    runs on the master/single-node process.
  - Admins can manually run or preview cleanup through
    `POST /api/log/request-capture/cleanup?dry_run=true`.

### Phase 5: Online Diagnostics

Status: implemented for the current single-node P0 scope.

- Add request id based diagnostic candidates list. Implemented via
  `GET /api/log/request-diagnostic-candidates`; candidates are collected from
  explicit error logs, suspicious capture records, and abnormal
  `request_conversion_meta` on consume logs, such as non-completed Responses
  terminal status, hosted-tool policy rejection, or hosted web-search executor
  errors.
- Generate diagnostic reports from normal logs and trace metadata. Implemented
  via `POST /api/log/request/:request_id/diagnostic`.
- Enrich reports with raw capture bundle metadata when available. Implemented
  for capture records and object artifact metadata.
- Add diagnostic bundle download for authorized administrators. Implemented via
  `GET /api/log/request/:request_id/diagnostic/bundle`; the zip includes
  report JSON, trace JSON, findings, capture summary, and decoded capture bundle
  files when an encrypted raw bundle is available. Raw bundle expansion is
  guarded by `DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES`; when exceeded, the zip keeps
  report files and writes a skipped marker instead of loading the full raw tar.
- Add UI shortcuts from usage log request id to trace and diagnostics.
  Implemented in the usage log detail dialog, including generate/regenerate and
  download bundle actions.

### Phase 6: Training Corpus Builder

Status: partially implemented for a single-node MVP.

- Read encrypted raw bundles in a worker. Implemented in
  `service/request_training_corpus.go` as `BuildTrainingCorpusDataset`.
- Decrypt in memory only. Implemented by reusing the capture bundle decoder
  with `MaxDecodedBundleBytes`, defaulting to 64 MiB per raw bundle.
- Redact secrets and PII. Basic recursive JSON key redaction is implemented
  for common secret fields such as `api_key`, `authorization`, `token`,
  `password`, `secret`, `cookie`, and `session`. Broader PII detection remains
  pending.
- Normalize Responses and Chat Completions variants into training schemas.
  Implemented for raw JSON responses and SSE text extraction, including Chat
  delta content and Responses `response.output_text.delta` events.
- Produce versioned JSONL.zst shards in SeaweedFS/S3-compatible storage.
  Implemented as one shard per build at
  `training/dataset={name}/version={version}/date={yyyy-mm-dd}/part-0001.jsonl.zst`.
- Record dataset version and sample lineage. Implemented through
  `training_dataset_versions` and `training_samples`.
- Add review and approval workflows before using samples for model training.
  Sample `pending` / `approved` / `rejected` status and approve/reject admin
  APIs are implemented. Dataset export filters the generated shard to approved
  `source_hash` rows before returning it. Sample preview API is implemented by
  loading the matching generated JSONL line by `source_hash`. The admin
  training-data UI is wired into the authenticated console with dataset build,
  sample preview, approve/reject, and approved-only export actions. Richer
  export policies remain pending.

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

Raw capture remains disabled by default. For production, first enable capture
for one scoped token, user, model, or channel, then check generated reports and
downloaded diagnostic bundles before broadening the policy.

## Current Safe Rollout

The current safe rollout is:

1. Deploy database migration and SeaweedFS volume wiring.
2. Keep `CAPTURE_ENABLED=false`.
3. Verify Data Proxy starts normally.
4. Verify SeaweedFS container and volume persistence.
5. Enable capture for one scoped test token/channel.
6. Generate one request, then confirm:
   - `GET /api/log/request-diagnostic-candidates` lists suspicious failures.
   - `POST /api/log/request/:request_id/diagnostic` generates a report.
   - `GET /api/log/request/:request_id/diagnostic/bundle` downloads a zip.
   - The usage log detail dialog can generate and download the same bundle.
