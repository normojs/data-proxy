# Request ID Trace Troubleshooting

Data Proxy records a local `request_id` for relay requests and, when available,
an `upstream_request_id` returned by the upstream provider. Use request trace
when a request looks successful at HTTP level but the client sees an empty
answer, a protocol conversion mismatch, a stream interruption, or a billing
discrepancy.

This project is based on `new-api`; keep request trace output free of API keys,
Authorization headers, raw secrets, and private user payloads when sharing logs.

## Quick Lookup

Admin users can query any request:

```bash
curl -sS 'https://YOUR_DOMAIN/api/log/request?request_id=REQ_ID' \
  -H 'Cookie: YOUR_ADMIN_SESSION_COOKIE'
```

Signed-in users can query only their own request logs:

```bash
curl -sS 'https://YOUR_DOMAIN/api/log/self/request?request_id=REQ_ID' \
  -H 'Cookie: YOUR_USER_SESSION_COOKIE'
```

Path style is also supported:

```text
GET /api/log/request/:request_id
GET /api/log/self/request/:request_id
```

The backend matches both `request_id` and `upstream_request_id`, so either value
can be pasted into the same field.

## Find Problematic Request IDs

If you do not know which request id failed, administrators can list recent
diagnostic candidates. The API combines error logs and suspicious capture
records, such as failed captures or stream requests with no downstream body.

```bash
curl -sS \
  'https://YOUR_DOMAIN/api/log/request-diagnostic-candidates?limit=50&start_timestamp=START&end_timestamp=END' \
  -H 'Cookie: YOUR_ADMIN_SESSION_COOKIE'
```

Each item contains `request_id`, `severity`, `source`, `summary`,
`last_seen_at`, log counters, and the latest diagnostic report status when one
exists. Use the returned `request_id` with the trace or diagnostic report APIs.

## Console Lookup

Open **Usage Logs -> Common**, then:

1. Paste the request id into the **Request ID** filter, or click the filter
   button next to a request id in the table.
2. Click the trace button next to a request id to open the log detail dialog
   directly, or open the row details from the Details column.
3. Read the **Request Trace** section.

The trace section shows:

- request status and matched log count;
- local and upstream request ids;
- request path and model mapping;
- max response time and total quota;
- protocol conversion metadata;
- stream status and errors when visible;
- a short list of related consume/error logs.

The copy button in this section copies the sanitized trace JSON for support or
incident notes.

## Diagnostic Report

Administrators can generate a request diagnostic report after finding a
problematic request id. The report combines the sanitized request trace,
request-capture metadata, object artifact metadata, and automatic findings.

```bash
curl -sS -X POST \
  'https://YOUR_DOMAIN/api/log/request/REQ_ID/diagnostic' \
  -H 'Cookie: YOUR_ADMIN_SESSION_COOKIE'
```

The latest generated report can be fetched without regenerating it:

```bash
curl -sS 'https://YOUR_DOMAIN/api/log/request/REQ_ID/diagnostic' \
  -H 'Cookie: YOUR_ADMIN_SESSION_COOKIE'
```

Query style is also supported:

```text
GET  /api/log/request-diagnostic?request_id=REQ_ID
POST /api/log/request-diagnostic?request_id=REQ_ID
```

In the console, open **Usage Logs -> Common**, open a log detail dialog, then
use the **Diagnostic Report** section. The generate button stores a report in
`request_diagnostic_reports`; the copy button copies the report JSON for local
analysis.

The diagnostic report intentionally does not include raw user request bodies or
raw model response bodies. It only includes metadata. Use the private capture
bundle workflow for raw data once the authorized download API is enabled.

## Key Fields

| Field | Meaning |
| --- | --- |
| `summary.status` | `completed`, `error`, `logged`, or `not_found`. |
| `summary.type_counts` | Matched log types, such as consume and error. |
| `diagnostics.request_path` | Relay path, for example `/v1/responses`. |
| `diagnostics.request_conversion` | Conversion chain, such as Responses -> Chat Completions. |
| `diagnostics.request_conversion_meta` | Structured protocol metadata. |
| `diagnostics.stream_status` | Stream ending state and soft errors. Admin view only. |
| `diagnostics.request_conversion_meta.hosted_tools_filtered` | Hosted tools filtered from a Chat-only conversion path. |
| `diagnostics.request_conversion_meta.hosted_tools_direct_answer_hint` | Whether Data Proxy injected the direct-answer hint after filtering hosted tools. |
| `report.findings` | Automatic diagnostic findings generated from trace and capture metadata. |
| `report.capture.capture_status` | Capture lifecycle status, such as spooling, finalizing, uploaded, or failed. |
| `report.capture.artifacts` | Stored object metadata for encrypted capture bundles. Raw payload is not included. |

## Common Diagnosis Patterns

### HTTP 200 but client shows blank

Check:

- `summary.status`;
- `diagnostics.request_conversion_meta.responses_terminal_status`;
- `diagnostics.stream_status`;
- whether `hosted_tools_filtered` contains `web_search`,
  `file_search`, `computer`, `code_interpreter`, or hosted `mcp`;
- whether the related logs include an error row for the same request id.

For Chat-only domestic channels, OpenAI hosted tools are filtered by default.
The conversion path does not execute external web search or file search unless
a future executor bridge is explicitly configured.

### Upstream returned Chat SSE on a non-stream route

Check:

- `diagnostics.request_conversion_meta.chat_sse_fallback`;
- final `summary.status`;
- `summary.completion_tokens`.

If `chat_sse_fallback=true`, Data Proxy aggregated the mislabeled Chat SSE body
before converting it back to a Responses JSON result.

### Responses request converted to Chat-only upstream

Check:

- `diagnostics.request_conversion_meta.responses_protocol`;
- `diagnostics.request_conversion_meta.upstream_protocol`;
- `diagnostics.request_conversion_meta.responses_protocol_decision`;
- `diagnostics.request_conversion_meta.responses_reasoning_adapter`;
- `diagnostics.request_conversion_meta.reasoning_params`.

These fields explain why the request stayed native Responses or was converted
to Chat Completions, and which reasoning adapter was applied.

### Tool-call follow-up did not continue

Check:

- `diagnostics.request_conversion_meta.history_restored_count`;
- `diagnostics.request_conversion_meta.history_restore_sources`;
- related logs with the same request id or upstream request id.

`history_restore_sources` can include `previous_response_id` or
`unique_call_id` when Data Proxy restored missing function call context before
converting a follow-up turn to Chat Completions.

## Privacy Notes

- Admin trace can include routing details such as `admin_info` and
  `stream_status`.
- User self trace removes admin-only diagnostic fields.
- Do not paste full trace JSON into public issues without reviewing it for
  user prompts, business identifiers, or internal channel names.
