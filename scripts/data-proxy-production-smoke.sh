#!/usr/bin/env bash
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[data-proxy-smoke] missing required command: $1" >&2
    exit 1
  fi
}

log() {
  echo "[data-proxy-smoke] $*" >&2
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
  scripts/data-proxy-production-smoke.sh

Environment:
  DATA_PROXY_BASE_URL=http://127.0.0.1:13002
  DATA_PROXY_API_KEY=sk-...                         Optional; enables /v1 smoke.
  DATA_PROXY_SMOKE_MODEL=gpt-4o-mini                Model used for Chat/Responses.
  DATA_PROXY_ADMIN_HEADER='Cookie: session=...'     Optional; enables admin trace checks.
  DATA_PROXY_SMOKE_REQUEST_ID=REQ_ID                Optional trace/diagnostic request id.
  DATA_PROXY_SMOKE_DIAGNOSTIC=1                     Generate diagnostic report.
  DATA_PROXY_SMOKE_DOWNLOAD_BUNDLE=1                Also download diagnostic zip.
  DATA_PROXY_SMOKE_CHAT=0                           Skip /v1/chat/completions.
  DATA_PROXY_SMOKE_RESPONSES=0                      Skip /v1/responses.
  DATA_PROXY_SMOKE_OUTPUT=/path/summary.md          Write markdown summary to file.

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
MODEL="${DATA_PROXY_SMOKE_MODEL:-gpt-4o-mini}"
TIMEOUT_SECONDS="${DATA_PROXY_SMOKE_TIMEOUT_SECONDS:-30}"
TRACE_WAIT_SECONDS="${DATA_PROXY_SMOKE_TRACE_WAIT_SECONDS:-2}"
RUN_CHAT="${DATA_PROXY_SMOKE_CHAT:-1}"
RUN_RESPONSES="${DATA_PROXY_SMOKE_RESPONSES:-1}"
RUN_DIAGNOSTIC="${DATA_PROXY_SMOKE_DIAGNOSTIC:-0}"
DOWNLOAD_BUNDLE="${DATA_PROXY_SMOKE_DOWNLOAD_BUNDLE:-0}"
REQUEST_ID="${DATA_PROXY_SMOKE_REQUEST_ID:-}"
OUTPUT="${DATA_PROXY_SMOKE_OUTPUT:-}"

if [[ ! "$TIMEOUT_SECONDS" =~ ^[0-9]+$ ]] || (( TIMEOUT_SECONDS <= 0 )); then
  die "invalid DATA_PROXY_SMOKE_TIMEOUT_SECONDS=$TIMEOUT_SECONDS"
fi
if [[ ! "$TRACE_WAIT_SECONDS" =~ ^[0-9]+$ ]]; then
  die "invalid DATA_PROXY_SMOKE_TRACE_WAIT_SECONDS=$TRACE_WAIT_SECONDS"
fi

TMPDIR_SMOKE="$(mktemp -d "${TMPDIR:-/tmp}/data-proxy-smoke.XXXXXX")"
trap 'rm -rf "$TMPDIR_SMOKE"' EXIT

SUMMARY="${OUTPUT:-$TMPDIR_SMOKE/summary.md}"
{
  echo "# Data Proxy Production Smoke"
  echo
  echo "| Field | Value |"
  echo "| --- | --- |"
  printf '| base_url | `%s` |\n' "$BASE_URL"
  printf '| model | `%s` |\n' "$MODEL"
} >"$SUMMARY"

summary_row() {
  printf '| %s | `%s` |\n' "$1" "$2" >>"$SUMMARY"
}

curl_json() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth_kind="${4:-public}"
  local output="$5"
  local headers="$6"
  local status
  local args=(-sS --max-time "$TIMEOUT_SECONDS" -D "$headers" -o "$output" -w '%{http_code}' -X "$method" "$url")

  if [[ "$auth_kind" == "api_key" ]]; then
    [[ -n "$API_KEY" ]] || die "DATA_PROXY_API_KEY is required for $url"
    args+=(-H "Authorization: Bearer $API_KEY")
  elif [[ "$auth_kind" == "admin" ]]; then
    [[ -n "$ADMIN_HEADER" ]] || die "DATA_PROXY_ADMIN_HEADER is required for $url"
    args+=(-H "$ADMIN_HEADER")
  fi

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
    jq -c '{error: .error?, message: .message?, success: .success?}' "$output" 2>/dev/null || cat "$output" >&2
    return 1
  fi
}

json_success() {
  local file="$1"
  jq -e '.success == true' "$file" >/dev/null
}

json_no_error() {
  local file="$1"
  jq -e '(.error? // null) == null' "$file" >/dev/null
}

header_request_id() {
  local headers="$1"
  awk 'BEGIN{IGNORECASE=1} /^X-Oneapi-Request-Id:|^X-Data-Proxy-Request-Id:/ { gsub("\r", "", $2); print $2; exit }' "$headers"
}

run_status_smoke() {
  local body="$TMPDIR_SMOKE/status.json"
  local headers="$TMPDIR_SMOKE/status.headers"
  log "GET /api/status"
  curl_json GET "$BASE_URL/api/status" "" public "$body" "$headers"
  json_success "$body" || die "/api/status did not return success=true"
  summary_row "api_status" "passed"
}

run_chat_smoke() {
  [[ "$RUN_CHAT" == "1" ]] || return 0
  [[ -n "$API_KEY" ]] || {
    summary_row "chat_completions" "skipped_no_api_key"
    return 0
  }
  local body="$TMPDIR_SMOKE/chat.json"
  local headers="$TMPDIR_SMOKE/chat.headers"
  local payload
  payload="$(jq -n --arg model "$MODEL" '{
    model: $model,
    messages: [{role: "user", content: "Reply with the single word pong."}],
    max_tokens: 16,
    stream: false
  }')"

  log "POST /v1/chat/completions"
  curl_json POST "$BASE_URL/v1/chat/completions" "$payload" api_key "$body" "$headers"
  json_no_error "$body" || die "/v1/chat/completions returned an error"
  jq -e '(.choices | length) > 0' "$body" >/dev/null || die "/v1/chat/completions returned no choices"
  local req_id
  req_id="$(header_request_id "$headers" || true)"
  [[ -n "$REQUEST_ID" || -z "$req_id" ]] || REQUEST_ID="$req_id"
  summary_row "chat_completions" "passed"
  [[ -z "$req_id" ]] || summary_row "chat_request_id" "$req_id"
}

run_responses_smoke() {
  [[ "$RUN_RESPONSES" == "1" ]] || return 0
  [[ -n "$API_KEY" ]] || {
    summary_row "responses" "skipped_no_api_key"
    return 0
  }
  local body="$TMPDIR_SMOKE/responses.json"
  local headers="$TMPDIR_SMOKE/responses.headers"
  local payload
  payload="$(jq -n --arg model "$MODEL" '{
    model: $model,
    input: "Reply with the single word pong.",
    max_output_tokens: 16,
    stream: false
  }')"

  log "POST /v1/responses"
  curl_json POST "$BASE_URL/v1/responses" "$payload" api_key "$body" "$headers"
  json_no_error "$body" || die "/v1/responses returned an error"
  jq -e '(.id? // "") != "" or (.output? | length) > 0 or (.output_text? // "") != ""' "$body" >/dev/null || die "/v1/responses returned no response id/output"
  local req_id
  req_id="$(header_request_id "$headers" || true)"
  [[ -n "$REQUEST_ID" || -z "$req_id" ]] || REQUEST_ID="$req_id"
  summary_row "responses" "passed"
  [[ -z "$req_id" ]] || summary_row "responses_request_id" "$req_id"
}

run_admin_diagnostic_smoke() {
  if [[ -z "$ADMIN_HEADER" ]]; then
    summary_row "diagnostic_candidates" "skipped_no_admin_header"
    summary_row "request_trace" "skipped_no_admin_header"
    return 0
  fi

  local candidates="$TMPDIR_SMOKE/diagnostic-candidates.json"
  local candidate_headers="$TMPDIR_SMOKE/diagnostic-candidates.headers"
  log "GET /api/log/request-diagnostic-candidates"
  curl_json GET "$BASE_URL/api/log/request-diagnostic-candidates?limit=5" "" admin "$candidates" "$candidate_headers"
  json_success "$candidates" || die "diagnostic candidates did not return success=true"
  summary_row "diagnostic_candidates" "passed"

  if [[ -z "$REQUEST_ID" ]]; then
    summary_row "request_trace" "skipped_no_request_id"
    return 0
  fi

  if (( TRACE_WAIT_SECONDS > 0 )); then
    sleep "$TRACE_WAIT_SECONDS"
  fi

  local trace="$TMPDIR_SMOKE/request-trace.json"
  local trace_headers="$TMPDIR_SMOKE/request-trace.headers"
  log "GET /api/log/request/$REQUEST_ID"
  curl_json GET "$BASE_URL/api/log/request/$REQUEST_ID" "" admin "$trace" "$trace_headers"
  json_success "$trace" || die "request trace did not return success=true"
  summary_row "request_trace" "passed"
  summary_row "trace_request_id" "$REQUEST_ID"

  if [[ "$RUN_DIAGNOSTIC" != "1" ]]; then
    summary_row "diagnostic_report" "skipped_set_DATA_PROXY_SMOKE_DIAGNOSTIC_1"
    return 0
  fi

  local diagnostic="$TMPDIR_SMOKE/request-diagnostic.json"
  local diagnostic_headers="$TMPDIR_SMOKE/request-diagnostic.headers"
  log "POST /api/log/request/$REQUEST_ID/diagnostic"
  curl_json POST "$BASE_URL/api/log/request/$REQUEST_ID/diagnostic" "" admin "$diagnostic" "$diagnostic_headers"
  json_success "$diagnostic" || die "diagnostic report did not return success=true"
  summary_row "diagnostic_report" "passed"

  if [[ "$DOWNLOAD_BUNDLE" == "1" ]]; then
    local bundle="$TMPDIR_SMOKE/request-diagnostic.zip"
    local bundle_headers="$TMPDIR_SMOKE/request-diagnostic-bundle.headers"
    local status
    log "GET /api/log/request/$REQUEST_ID/diagnostic/bundle"
    status="$(curl -sS --max-time "$TIMEOUT_SECONDS" -D "$bundle_headers" -o "$bundle" -w '%{http_code}' -X GET "$BASE_URL/api/log/request/$REQUEST_ID/diagnostic/bundle" -H "$ADMIN_HEADER")"
    if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
      die "diagnostic bundle download failed with HTTP $status"
    fi
    summary_row "diagnostic_bundle" "passed"
  else
    summary_row "diagnostic_bundle" "skipped_set_DATA_PROXY_SMOKE_DOWNLOAD_BUNDLE_1"
  fi
}

run_status_smoke
run_chat_smoke
run_responses_smoke
run_admin_diagnostic_smoke

summary_row "completed_at_utc" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

if [[ -n "$OUTPUT" ]]; then
  log "summary written to $OUTPUT"
else
  cat "$SUMMARY"
fi
