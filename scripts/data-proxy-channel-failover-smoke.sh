#!/usr/bin/env bash
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[data-proxy-failover-smoke] missing required command: $1" >&2
    exit 1
  fi
}

log() {
  echo "[data-proxy-failover-smoke] $*" >&2
}

die() {
  log "$*"
  exit 1
}

need curl
need jq

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-channel-failover-smoke.sh

Purpose:
  Non-destructively prove that an already configured same-model bad channel
  fails over to a backup channel. The script never creates, edits, disables,
  or deletes channels.

Required:
  DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd
  DATA_PROXY_API_KEY=sk-...
  DATA_PROXY_FAILOVER_MODEL=deepseek-ai/DeepSeek-V4-Flash

Admin auth, one of:
  DATA_PROXY_ADMIN_HEADER='Cookie: session=...'
  DATA_PROXY_ADMIN_ACCESS_TOKEN=...

Also required for admin auth:
  DATA_PROXY_ADMIN_USER_ID=1

Optional:
  DATA_PROXY_FAILOVER_REQUEST_ID=REQ_ID              Validate an existing request id instead of sending a new request.
  DATA_PROXY_FAILOVER_EXPECT_FAILED_CHANNEL_ID=123   Assert the failed channel id.
  DATA_PROXY_FAILOVER_EXPECT_BACKUP_CHANNEL_ID=456   Assert the retry-selected backup channel id.
  DATA_PROXY_FAILOVER_EXPECT_FAILED_STATUS_CODE=502  Assert the failed status code.
  DATA_PROXY_FAILOVER_CHECK_CANDIDATES=0             Skip diagnostic candidate check. Default: 1.
  DATA_PROXY_FAILOVER_TRACE_ATTEMPTS=8               Trace polling attempts. Default: 8.
  DATA_PROXY_FAILOVER_TRACE_INTERVAL_SECONDS=2       Trace polling interval. Default: 2.
  DATA_PROXY_FAILOVER_TIMEOUT_SECONDS=45             curl max-time. Default: 45.
  DATA_PROXY_FAILOVER_OUTPUT=/path/summary.md        Write markdown summary to file.

The script never prints API keys or admin headers.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
if [[ $# -gt 0 ]]; then
  usage
  die "unknown argument: $1"
fi

BASE_URL="${DATA_PROXY_BASE_URL:-http://127.0.0.1:13002}"
BASE_URL="${BASE_URL%/}"
API_KEY="${DATA_PROXY_API_KEY:-}"
ADMIN_HEADER="${DATA_PROXY_ADMIN_HEADER:-}"
ADMIN_ACCESS_TOKEN="${DATA_PROXY_ADMIN_ACCESS_TOKEN:-}"
ADMIN_USER_ID="${DATA_PROXY_ADMIN_USER_ID:-}"
MODEL="${DATA_PROXY_FAILOVER_MODEL:-${DATA_PROXY_SMOKE_MODEL:-gpt-4o-mini}}"
REQUEST_ID="${DATA_PROXY_FAILOVER_REQUEST_ID:-}"
EXPECT_FAILED_CHANNEL_ID="${DATA_PROXY_FAILOVER_EXPECT_FAILED_CHANNEL_ID:-}"
EXPECT_BACKUP_CHANNEL_ID="${DATA_PROXY_FAILOVER_EXPECT_BACKUP_CHANNEL_ID:-}"
EXPECT_FAILED_STATUS_CODE="${DATA_PROXY_FAILOVER_EXPECT_FAILED_STATUS_CODE:-}"
CHECK_CANDIDATES="${DATA_PROXY_FAILOVER_CHECK_CANDIDATES:-1}"
TRACE_ATTEMPTS="${DATA_PROXY_FAILOVER_TRACE_ATTEMPTS:-8}"
TRACE_INTERVAL_SECONDS="${DATA_PROXY_FAILOVER_TRACE_INTERVAL_SECONDS:-2}"
TIMEOUT_SECONDS="${DATA_PROXY_FAILOVER_TIMEOUT_SECONDS:-45}"
OUTPUT="${DATA_PROXY_FAILOVER_OUTPUT:-}"

for numeric in TRACE_ATTEMPTS TRACE_INTERVAL_SECONDS TIMEOUT_SECONDS; do
  value="${!numeric}"
  if [[ ! "$value" =~ ^[0-9]+$ ]]; then
    die "invalid $numeric=$value"
  fi
done
for numeric in EXPECT_FAILED_CHANNEL_ID EXPECT_BACKUP_CHANNEL_ID EXPECT_FAILED_STATUS_CODE; do
  value="${!numeric}"
  if [[ -n "$value" && ! "$value" =~ ^[0-9]+$ ]]; then
    die "invalid $numeric=$value"
  fi
done
if (( TRACE_ATTEMPTS <= 0 )); then
  die "DATA_PROXY_FAILOVER_TRACE_ATTEMPTS must be greater than 0"
fi
if (( TIMEOUT_SECONDS <= 0 )); then
  die "DATA_PROXY_FAILOVER_TIMEOUT_SECONDS must be greater than 0"
fi

if [[ -z "$REQUEST_ID" && -z "$API_KEY" ]]; then
  die "DATA_PROXY_API_KEY is required unless DATA_PROXY_FAILOVER_REQUEST_ID is set"
fi
if [[ -z "$ADMIN_HEADER" && -z "$ADMIN_ACCESS_TOKEN" ]]; then
  die "DATA_PROXY_ADMIN_HEADER or DATA_PROXY_ADMIN_ACCESS_TOKEN is required"
fi
if [[ -z "$ADMIN_USER_ID" ]]; then
  die "DATA_PROXY_ADMIN_USER_ID is required"
fi

TMPDIR_SMOKE="$(mktemp -d "${TMPDIR:-/tmp}/data-proxy-failover-smoke.XXXXXX")"
trap 'rm -rf "$TMPDIR_SMOKE"' EXIT

SUMMARY="${OUTPUT:-$TMPDIR_SMOKE/summary.md}"
{
  echo "# Data Proxy Channel Failover Smoke"
  echo
  echo "| Field | Value |"
  echo "| --- | --- |"
  printf '| base_url | `%s` |\n' "$BASE_URL"
  printf '| model | `%s` |\n' "$MODEL"
} >"$SUMMARY"

summary_row() {
  local value="${2//$'\n'/ }"
  value="${value//|/\\|}"
  printf '| %s | `%s` |\n' "$1" "$value" >>"$SUMMARY"
}

admin_headers=()
if [[ -n "$ADMIN_HEADER" ]]; then
  admin_headers+=(-H "$ADMIN_HEADER")
fi
if [[ -n "$ADMIN_ACCESS_TOKEN" ]]; then
  admin_headers+=(-H "Authorization: Bearer $ADMIN_ACCESS_TOKEN")
fi
admin_headers+=(-H "New-Api-User: $ADMIN_USER_ID")

curl_json() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth_kind="${4:-public}"
  local output="$5"
  local headers="$6"
  local status
  local args=(-sS --max-time "$TIMEOUT_SECONDS" -D "$headers" -o "$output" -w '%{http_code}' -X "$method" "$url")

  case "$auth_kind" in
    api_key)
      args+=(-H "Authorization: Bearer $API_KEY")
      ;;
    admin)
      args+=("${admin_headers[@]}")
      ;;
    public)
      ;;
    *)
      die "unknown auth kind: $auth_kind"
      ;;
  esac

  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$body")
  fi

  status="$(curl "${args[@]}" || true)"
  if [[ ! "$status" =~ ^[0-9]{3}$ ]]; then
    log "$method $url failed before receiving an HTTP status"
    cat "$output" >&2 2>/dev/null || true
    return 1
  fi
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    log "$method $url failed with HTTP $status"
    jq -c '{success: .success?, error: .error?, message: .message?}' "$output" 2>/dev/null || cat "$output" >&2
    return 1
  fi
}

json_success() {
  jq -e '.success == true' "$1" >/dev/null
}

header_request_id() {
  local headers="$1"
  awk '
    {
      line = $0
      sub(/\r$/, "", line)
      lower = tolower(line)
      if (lower ~ /^(x-oneapi-request-id|x-data-proxy-request-id|x-request-id|request-id|openai-request-id):/) {
        sub(/^[^:]+:[[:space:]]*/, "", line)
        print line
        exit
      }
    }
  ' "$headers"
}

uri_encode() {
  jq -rn --arg v "$1" '$v|@uri'
}

send_failover_probe() {
  local body="$TMPDIR_SMOKE/chat.json"
  local headers="$TMPDIR_SMOKE/chat.headers"
  local payload
  payload="$(jq -n --arg model "$MODEL" '{
    model: $model,
    messages: [{role: "user", content: "Reply with the single word failover-pong."}],
    max_tokens: 32,
    stream: false
  }')"

  log "POST /v1/chat/completions"
  curl_json POST "$BASE_URL/v1/chat/completions" "$payload" api_key "$body" "$headers"
  jq -e '(.error? // null) == null and ((.choices // []) | length > 0)' "$body" >/dev/null || die "probe request did not return a usable chat completion"

  REQUEST_ID="$(header_request_id "$headers" || true)"
  [[ -n "$REQUEST_ID" ]] || die "probe response did not include a request id header"
  summary_row "probe_request" "passed"
  summary_row "request_id" "$REQUEST_ID"
}

fetch_trace() {
  local output="$1"
  local headers="$TMPDIR_SMOKE/trace.headers"
  curl_json GET "$BASE_URL/api/log/request/$REQUEST_ID" "" admin "$output" "$headers"
  json_success "$output" || return 1
  jq -e '(.data.total // 0) > 0' "$output" >/dev/null
}

poll_trace() {
  local trace="$TMPDIR_SMOKE/trace.json"
  local attempt
  for ((attempt = 1; attempt <= TRACE_ATTEMPTS; attempt++)); do
    if fetch_trace "$trace"; then
      echo "$trace"
      return 0
    fi
    if (( attempt < TRACE_ATTEMPTS && TRACE_INTERVAL_SECONDS > 0 )); then
      sleep "$TRACE_INTERVAL_SECONDS"
    fi
  done
  die "request trace was not available for $REQUEST_ID after $TRACE_ATTEMPTS attempts"
}

events_filter='[ (.data.diagnostics.admin_info.channel_failover // [])[]?, (.data.logs[]?.other.admin_info.channel_failover // [])[]? ]'

validate_trace() {
  local trace="$1"
  local events="$TMPDIR_SMOKE/failover-events.json"
  jq -c "$events_filter" "$trace" >"$events"

  jq -e 'length > 0' "$events" >/dev/null || die "trace has no admin_info.channel_failover events"
  jq -e 'any(.[]; .event == "failed" and .retry_planned == true)' "$events" >/dev/null || die "trace has no failed event with retry_planned=true"
  jq -e 'any(.[]; .event == "selected" and ((.retry_index // 0) > 0))' "$events" >/dev/null || die "trace has no retry selected event"
  jq -e '[.[] | select(.event == "selected") | .channel_id] | unique | length >= 2' "$events" >/dev/null || die "trace does not show two distinct selected channels"
  jq -e '(.data.diagnostics.contains_consume == true) or (((.data.summary.type_counts.consume // 0) | tonumber) > 0)' "$trace" >/dev/null || die "trace does not show a successful consume log"

  if [[ -n "$EXPECT_FAILED_CHANNEL_ID" ]]; then
    jq -e --argjson id "$EXPECT_FAILED_CHANNEL_ID" 'any(.[]; .event == "failed" and .channel_id == $id)' "$events" >/dev/null || die "expected failed channel id $EXPECT_FAILED_CHANNEL_ID was not found"
  fi
  if [[ -n "$EXPECT_BACKUP_CHANNEL_ID" ]]; then
    jq -e --argjson id "$EXPECT_BACKUP_CHANNEL_ID" 'any(.[]; .event == "selected" and ((.retry_index // 0) > 0) and .channel_id == $id)' "$events" >/dev/null || die "expected backup channel id $EXPECT_BACKUP_CHANNEL_ID was not found"
  fi
  if [[ -n "$EXPECT_FAILED_STATUS_CODE" ]]; then
    jq -e --argjson status "$EXPECT_FAILED_STATUS_CODE" 'any(.[]; .event == "failed" and .status_code == $status)' "$events" >/dev/null || die "expected failed status code $EXPECT_FAILED_STATUS_CODE was not found"
  fi

  local event_summary
  event_summary="$(jq -r 'map("\(.event):channel=\(.channel_id // "-"),retry=\(.retry_index // "-"),planned=\(.retry_planned // "-"),status=\(.status_code // "-")") | join("; ")' "$events")"
  summary_row "request_trace" "passed"
  summary_row "failover_events" "$event_summary"
}

validate_candidates() {
  [[ "$CHECK_CANDIDATES" == "1" ]] || {
    summary_row "diagnostic_candidates" "skipped"
    return 0
  }

  local body="$TMPDIR_SMOKE/candidates.json"
  local headers="$TMPDIR_SMOKE/candidates.headers"
  local encoded_model
  encoded_model="$(uri_encode "$MODEL")"
  local start_timestamp=$(( request_started_at > 60 ? request_started_at - 60 : 0 ))
  local end_timestamp=$(( $(date +%s) + 60 ))
  local url="$BASE_URL/api/log/request-diagnostic-candidates?limit=100&source=failover&model_name=$encoded_model&start_timestamp=$start_timestamp&end_timestamp=$end_timestamp"

  log "GET /api/log/request-diagnostic-candidates?source=failover"
  curl_json GET "$url" "" admin "$body" "$headers"
  json_success "$body" || die "diagnostic candidates did not return success=true"
  jq -e --arg request_id "$REQUEST_ID" 'any(.data.items[]?; .request_id == $request_id and .source == "channel_failover")' "$body" >/dev/null || die "diagnostic candidates do not include the failover request id"
  summary_row "diagnostic_candidates" "passed"
}

request_started_at="$(date +%s)"
if [[ -z "$REQUEST_ID" ]]; then
  send_failover_probe
else
  summary_row "request_id" "$REQUEST_ID"
  summary_row "probe_request" "skipped_existing_request_id"
fi

trace_file="$(poll_trace)"
validate_trace "$trace_file"
validate_candidates

summary_row "started_at_unix" "$request_started_at"
summary_row "completed_at_utc" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

if [[ -n "$OUTPUT" ]]; then
  log "summary written to $OUTPUT"
else
  cat "$SUMMARY"
fi
