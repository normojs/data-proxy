#!/usr/bin/env bash
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[snapless-preprod-smoke] missing required command: $1" >&2
    exit 1
  fi
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "[snapless-preprod-smoke] set $name before running this smoke." >&2
    exit 1
  fi
}

header_for_role() {
  case "$1" in
    admin) printf '%s' "$ADMIN_HEADER" ;;
    developer) printf '%s' "$DEVELOPER_HEADER" ;;
    user) printf '%s' "$AUTHORIZING_USER_HEADER" ;;
    public) printf '' ;;
    *) echo "[snapless-preprod-smoke] unknown role: $1" >&2; exit 1 ;;
  esac
}

json_field() {
  local response="$1"
  local filter="$2"
  jq -er "$filter" "$response"
}

summary_row() {
  printf '| %s | `%s` |\n' "$1" "$2"
}

api() {
  local role="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local header
  local output
  local status

  header="$(header_for_role "$role")"
  output="$(mktemp)"

  if [[ -n "$body" ]]; then
    if [[ -n "$header" ]]; then
      status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$BASE_URL$path" -H "$header" -H 'Content-Type: application/json' --data "$body")"
    else
      status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$BASE_URL$path" -H 'Content-Type: application/json' --data "$body")"
    fi
  else
    if [[ -n "$header" ]]; then
      status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$BASE_URL$path" -H "$header")"
    else
      status="$(curl -sS -o "$output" -w '%{http_code}' -X "$method" "$BASE_URL$path")"
    fi
  fi

  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "[snapless-preprod-smoke] $method $path failed with HTTP $status" >&2
    cat "$output" >&2
    exit 1
  fi
  if ! jq -e '.success == true' "$output" >/dev/null 2>&1; then
    echo "[snapless-preprod-smoke] $method $path did not return success=true" >&2
    cat "$output" >&2
    exit 1
  fi

  printf '%s' "$output"
}

cleanup_preprod_smoke() {
  if [[ "${SNAPLESS_PREPROD_CLEANUP:-}" != "1" || "${CLEANUP_DONE:-0}" == "1" ]]; then
    return
  fi
  CLEANUP_DONE=1

  set +e
  local header output status body
  header="$(header_for_role admin)"

  if [[ -n "${APP_ID:-}" ]]; then
    echo "[snapless-preprod-smoke] cleanup: disable connected app $APP_ID" >&2
    output="$(mktemp)"
    body="$(jq -n \
      --arg name "$APP_NAME" \
      '{name:$name,description:"Preprod connected app smoke",allowed_scopes:["openai.models","openai.chat","quota.read","token.manage"],default_scopes:["openai.models","openai.chat","quota.read"],authorization_flow:"device_code",trusted:true,status:2}')"
    status="$(curl -sS -o "$output" -w '%{http_code}' -X PUT "$BASE_URL/api/connected-apps/$APP_ID" -H "$header" -H 'Content-Type: application/json' --data "$body")"
    if [[ "$status" -ge 200 && "$status" -lt 300 ]] && jq -e '.success == true' "$output" >/dev/null 2>&1; then
      CLEANUP_RESULT="app_disabled"
    else
      CLEANUP_RESULT="app_disable_failed_http_$status"
      cat "$output" >&2
    fi
    set -e
    return
  fi

  if [[ -n "${REQUEST_ID:-}" ]]; then
    echo "[snapless-preprod-smoke] cleanup: reject pending connected app request $REQUEST_ID" >&2
    output="$(mktemp)"
    body='{"decision":"rejected","review_note":"preprod smoke cleanup"}'
    status="$(curl -sS -o "$output" -w '%{http_code}' -X POST "$BASE_URL/api/connected-apps/requests/$REQUEST_ID/review" -H "$header" -H 'Content-Type: application/json' --data "$body")"
    if [[ "$status" -ge 200 && "$status" -lt 300 ]] && jq -e '.success == true' "$output" >/dev/null 2>&1; then
      CLEANUP_RESULT="request_rejected"
    else
      CLEANUP_RESULT="request_reject_failed_http_$status"
      cat "$output" >&2
    fi
  fi
  set -e
}

need curl
need jq

if [[ "${SNAPLESS_PREPROD_CONFIRM:-}" != "1" ]]; then
  cat >&2 <<'EOF'
[snapless-preprod-smoke] this smoke creates a connected app request, approves it,
creates/rotates developer keys, and authorizes a device session.
Set SNAPLESS_PREPROD_CLEANUP=1 to disable the smoke app after the run.
Set SNAPLESS_PREPROD_CONFIRM=1 to run it against preprod.
EOF
  exit 1
fi

require_env DATA_PROXY_BASE_URL
require_env ADMIN_HEADER
require_env DEVELOPER_HEADER

BASE_URL="${DATA_PROXY_BASE_URL%/}"
AUTHORIZING_USER_HEADER="${AUTHORIZING_USER_HEADER:-$DEVELOPER_HEADER}"
RUN_ID="${SNAPLESS_PREPROD_RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
APP_SLUG="${SNAPLESS_PREPROD_APP_SLUG:-snapless-preprod-$RUN_ID}"
APP_NAME="${SNAPLESS_PREPROD_APP_NAME:-Snapless Preprod $RUN_ID}"
DEVICE_ID="${SNAPLESS_PREPROD_DEVICE_ID:-preprod-device-$RUN_ID}"
DEVELOPER_DEVICE_ID="${SNAPLESS_PREPROD_DEVELOPER_DEVICE_ID:-preprod-developer-$RUN_ID}"
REQUEST_ID=""
APP_ID=""
CLEANUP_DONE=0
CLEANUP_RESULT="${SNAPLESS_PREPROD_CLEANUP:-0}"
trap cleanup_preprod_smoke EXIT

SUMMARY="$(mktemp)"
{
  echo "# Snapless Connected App Preprod Smoke"
  echo
  echo "| Field | Value |"
  echo "| --- | --- |"
  summary_row "base_url" "$BASE_URL"
  summary_row "run_id" "$RUN_ID"
  summary_row "app_slug" "$APP_SLUG"
} > "$SUMMARY"

echo "[snapless-preprod-smoke] submit connected app request: $APP_SLUG"
request_body="$(jq -n \
  --arg slug "$APP_SLUG" \
  --arg name "$APP_NAME" \
  '{slug:$slug,name:$name,description:"Preprod connected app smoke",requested_scopes:["openai.models","openai.chat","quota.read","token.manage"],default_scopes:["openai.models","openai.chat","quota.read"],authorization_flow:"device_code",homepage_url:"https://snapless.example",reason:"preprod smoke"}')"
response="$(api developer POST /api/connected-app-requests "$request_body")"
REQUEST_ID="$(json_field "$response" '.data.request.id')"
summary_row "request_id" "$REQUEST_ID" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify developer self request list"
response="$(api developer GET /api/connected-app-requests/self)"
SELF_TOTAL="$(json_field "$response" '.data.total')"
summary_row "developer_self_request_total" "$SELF_TOTAL" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify admin pending request list and notification"
response="$(api admin GET '/api/connected-apps/requests?status=pending')"
ADMIN_PENDING_TOTAL="$(json_field "$response" '.data.total')"
summary_row "admin_pending_request_total" "$ADMIN_PENDING_TOTAL" >> "$SUMMARY"
response="$(api admin GET /api/notifications/connected-app-requests)"
ADMIN_NOTIFICATION_UNREAD="$(json_field "$response" '.data.unread_count')"
summary_row "admin_notification_unread" "$ADMIN_NOTIFICATION_UNREAD" >> "$SUMMARY"

echo "[snapless-preprod-smoke] approve connected app request: $REQUEST_ID"
review_body='{"decision":"approved","review_note":"preprod smoke approved","allowed_scopes":["openai.models","openai.chat","quota.read","token.manage"],"default_scopes":["openai.models","openai.chat","quota.read"]}'
response="$(api admin POST "/api/connected-apps/requests/$REQUEST_ID/review" "$review_body")"
APP_ID="$(json_field "$response" '.data.app.id')"
APP_STATUS="$(json_field "$response" '.data.request.status')"
summary_row "app_id" "$APP_ID" >> "$SUMMARY"
summary_row "request_status" "$APP_STATUS" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify audit and applicant notification"
response="$(api admin GET "/api/connected-apps/audit-logs?target_type=connected_app_request&target_id=$REQUEST_ID")"
AUDIT_TOTAL="$(json_field "$response" '.data.total')"
summary_row "audit_total" "$AUDIT_TOTAL" >> "$SUMMARY"
response="$(api developer GET /api/notifications/connected-app-requests)"
DEVELOPER_NOTIFICATION_UNREAD="$(json_field "$response" '.data.unread_count')"
summary_row "developer_notification_unread" "$DEVELOPER_NOTIFICATION_UNREAD" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify SDK/OpenAPI config"
response="$(api developer GET "/api/connected-apps/$APP_SLUG/developer/sdk-config")"
CAN_CREATE_KEY="$(json_field "$response" '.data.permissions.can_create_key')"
CAN_READ_USAGE="$(json_field "$response" '.data.permissions.can_read_usage')"
OPENAPI_URL="$(json_field "$response" '.data.openapi_url')"
summary_row "can_create_key" "$CAN_CREATE_KEY" >> "$SUMMARY"
summary_row "can_read_usage" "$CAN_READ_USAGE" >> "$SUMMARY"
summary_row "openapi_url" "$OPENAPI_URL" >> "$SUMMARY"
api developer GET "/api/connected-apps/$APP_SLUG/developer/openapi" >/dev/null

echo "[snapless-preprod-smoke] create/reuse/rotate developer key"
developer_key_body="$(jq -n \
  --arg device_id "$DEVELOPER_DEVICE_ID" \
  '{device_id:$device_id,device_name:"Preprod Developer",platform:"server",app_version:"1.0.0",client:"preprod-smoke"}')"
response="$(api developer POST "/api/connected-apps/$APP_SLUG/developer/keys" "$developer_key_body")"
FIRST_DEVELOPER_TOKEN_ID="$(json_field "$response" '.data.token.id')"
FIRST_DEVELOPER_KEY_ONCE="$(json_field "$response" '.data.api_key_once')"
summary_row "developer_token_id_first" "$FIRST_DEVELOPER_TOKEN_ID" >> "$SUMMARY"
summary_row "developer_key_once_first" "$FIRST_DEVELOPER_KEY_ONCE" >> "$SUMMARY"

response="$(api developer POST "/api/connected-apps/$APP_SLUG/developer/keys" "$developer_key_body")"
REUSED_DEVELOPER_TOKEN_ID="$(json_field "$response" '.data.token.id')"
REUSED_DEVELOPER_KEY_ONCE="$(json_field "$response" '.data.api_key_once')"
summary_row "developer_token_id_reuse" "$REUSED_DEVELOPER_TOKEN_ID" >> "$SUMMARY"
summary_row "developer_key_once_reuse" "$REUSED_DEVELOPER_KEY_ONCE" >> "$SUMMARY"

developer_rotate_body="$(jq -n \
  --arg device_id "$DEVELOPER_DEVICE_ID" \
  '{device_id:$device_id,device_name:"Preprod Developer",platform:"server",app_version:"1.0.1",client:"preprod-smoke",rotate:true}')"
response="$(api developer POST "/api/connected-apps/$APP_SLUG/developer/keys" "$developer_rotate_body")"
ROTATED_DEVELOPER_TOKEN_ID="$(json_field "$response" '.data.token.id')"
ROTATED_DEVELOPER_KEY_ONCE="$(json_field "$response" '.data.api_key_once')"
summary_row "developer_token_id_rotated" "$ROTATED_DEVELOPER_TOKEN_ID" >> "$SUMMARY"
summary_row "developer_key_once_rotated" "$ROTATED_DEVELOPER_KEY_ONCE" >> "$SUMMARY"

echo "[snapless-preprod-smoke] run connected app device flow"
device_body="$(jq -n \
  --arg device_id "$DEVICE_ID" \
  '{device_id:$device_id,device_name:"Preprod Desktop",platform:"macos",app_version:"1.0.0",client:"preprod-smoke"}')"
response="$(api public POST "/api/connected-apps/$APP_SLUG/device/start" "$device_body")"
DEVICE_CODE="$(json_field "$response" '.data.device_code')"
USER_CODE="$(json_field "$response" '.data.user_code')"
summary_row "device_code" "$DEVICE_CODE" >> "$SUMMARY"
summary_row "user_code" "$USER_CODE" >> "$SUMMARY"

poll_body="$(jq -n --arg device_code "$DEVICE_CODE" '{device_code:$device_code}')"
response="$(api public POST "/api/connected-apps/$APP_SLUG/device/poll" "$poll_body")"
PENDING_STATUS="$(json_field "$response" '.data.status')"
summary_row "device_poll_pending_status" "$PENDING_STATUS" >> "$SUMMARY"

api user GET "/api/connected-apps/$APP_SLUG/device/status?user_code=$USER_CODE" >/dev/null
authorize_body="$(jq -n --arg user_code "$USER_CODE" '{user_code:$user_code,approve:true}')"
response="$(api user POST "/api/connected-apps/$APP_SLUG/device/authorize" "$authorize_body")"
AUTHORIZED_STATUS="$(json_field "$response" '.data.status')"
AUTHORIZED_TOKEN_ID="$(json_field "$response" '.data.token.id')"
summary_row "device_authorized_status" "$AUTHORIZED_STATUS" >> "$SUMMARY"
summary_row "device_authorized_token_id" "$AUTHORIZED_TOKEN_ID" >> "$SUMMARY"

response="$(api public POST "/api/connected-apps/$APP_SLUG/device/poll" "$poll_body")"
FIRST_POLL_TOKEN_ID="$(json_field "$response" '.data.token.id')"
FIRST_POLL_KEY_ONCE="$(json_field "$response" '.data.api_key_once')"
summary_row "device_poll_token_id" "$FIRST_POLL_TOKEN_ID" >> "$SUMMARY"
summary_row "device_poll_key_once" "$FIRST_POLL_KEY_ONCE" >> "$SUMMARY"

response="$(api public POST "/api/connected-apps/$APP_SLUG/device/poll" "$poll_body")"
CONSUMED_STATUS="$(json_field "$response" '.data.status')"
summary_row "device_poll_second_status" "$CONSUMED_STATUS" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify developer diagnostics"
response="$(api developer GET "/api/connected-apps/$APP_SLUG/developer/authorizations")"
AUTHORIZATION_TOTAL="$(json_field "$response" '.data.total')"
summary_row "developer_authorization_total" "$AUTHORIZATION_TOTAL" >> "$SUMMARY"
response="$(api developer GET "/api/connected-apps/$APP_SLUG/developer/device-sessions?status=consumed")"
CONSUMED_SESSION_TOTAL="$(json_field "$response" '.data.total')"
summary_row "developer_consumed_session_total" "$CONSUMED_SESSION_TOTAL" >> "$SUMMARY"
response="$(api developer GET "/api/connected-apps/$APP_SLUG/developer/usage")"
USAGE_TOKEN_COUNT="$(json_field "$response" '.data.token_count')"
summary_row "developer_usage_token_count" "$USAGE_TOKEN_COUNT" >> "$SUMMARY"

echo "[snapless-preprod-smoke] verify notification outbox visibility"
response="$(api admin GET "/api/connected-apps/notification-outbox?app_id=$APP_ID")"
ADMIN_OUTBOX_TOTAL="$(json_field "$response" '.data.total')"
summary_row "admin_outbox_total" "$ADMIN_OUTBOX_TOTAL" >> "$SUMMARY"
response="$(api developer GET "/api/connected-apps/$APP_SLUG/developer/notification-outbox")"
DEVELOPER_OUTBOX_TOTAL="$(json_field "$response" '.data.total')"
summary_row "developer_outbox_total" "$DEVELOPER_OUTBOX_TOTAL" >> "$SUMMARY"
api admin GET /api/connected-apps/notification-outbox/worker-metrics >/dev/null

if [[ "${SNAPLESS_PREPROD_CLEANUP:-}" == "1" ]]; then
  cleanup_preprod_smoke
else
  CLEANUP_RESULT="not_requested"
fi
summary_row "cleanup" "$CLEANUP_RESULT" >> "$SUMMARY"

echo
cat "$SUMMARY"
echo
echo "[snapless-preprod-smoke] completed"
