#!/usr/bin/env bash
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[data-proxy-agent-status-smoke] missing required command: $1" >&2
    exit 1
  fi
}

log() {
  echo "[data-proxy-agent-status-smoke] $*" >&2
}

die() {
  log "$*"
  exit 1
}

need jq

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-agent-status-smoke.sh

Purpose:
  Validate dpa status --json, optional doctor --json, and optional route test
  with a temporary token-safe config. The script does not read or modify the
  user's real data-proxy-agent config.

Optional:
  DATA_PROXY_AGENT_BIN=/path/to/dpa                 dpa binary. Default: dpa from PATH.
  DATA_PROXY_AGENT_SMOKE_BASE_URL=https://dp...     Base URL written to temporary config.
  DATA_PROXY_AGENT_SMOKE_BRIDGE_WS_URL=wss://...    Bridge URL written to temporary config.
  DATA_PROXY_AGENT_SMOKE_TOKEN=...                  Token written only to temporary config.
  DATA_PROXY_AGENT_SMOKE_DOCTOR=1                   Also run doctor --json. Default: 0.
  DATA_PROXY_AGENT_SMOKE_ROUTE_TEST=1               Also test local HTTP route. Default: 0.
  DATA_PROXY_AGENT_SMOKE_HTTP_ROUTE_TARGET=http://127.0.0.1:9
                                                       HTTP route target used by route test.
  DATA_PROXY_AGENT_SMOKE_ROUTE_EXPECT_SUCCESS=1      Fail if route test does not pass. Default: 0.
  DATA_PROXY_AGENT_SMOKE_TIMEOUT=2s                 dpa timeout for health/route checks.
  DATA_PROXY_AGENT_SMOKE_OUTPUT=/path/summary.md    Write markdown summary to file.

The script never prints the token and fails if dpa JSON output leaks it.
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

AGENT_BIN="${DATA_PROXY_AGENT_BIN:-dpa}"
BASE_URL="${DATA_PROXY_AGENT_SMOKE_BASE_URL:-https://dp.app.mbu.ltd}"
BRIDGE_WS_URL="${DATA_PROXY_AGENT_SMOKE_BRIDGE_WS_URL:-}"
TOKEN="${DATA_PROXY_AGENT_SMOKE_TOKEN:-sk-data-proxy-agent-status-smoke-secret}"
RUN_DOCTOR="${DATA_PROXY_AGENT_SMOKE_DOCTOR:-0}"
RUN_ROUTE_TEST="${DATA_PROXY_AGENT_SMOKE_ROUTE_TEST:-0}"
HTTP_ROUTE_TARGET="${DATA_PROXY_AGENT_SMOKE_HTTP_ROUTE_TARGET:-http://127.0.0.1:9}"
ROUTE_EXPECT_SUCCESS="${DATA_PROXY_AGENT_SMOKE_ROUTE_EXPECT_SUCCESS:-0}"
TIMEOUT="${DATA_PROXY_AGENT_SMOKE_TIMEOUT:-2s}"
OUTPUT="${DATA_PROXY_AGENT_SMOKE_OUTPUT:-}"

if ! command -v "$AGENT_BIN" >/dev/null 2>&1 && [[ ! -x "$AGENT_BIN" ]]; then
  die "dpa binary not found: $AGENT_BIN"
fi
for boolean in RUN_DOCTOR RUN_ROUTE_TEST ROUTE_EXPECT_SUCCESS; do
  value="${!boolean}"
  if [[ "$value" != "0" && "$value" != "1" ]]; then
    die "invalid $boolean=$value; expected 0 or 1"
  fi
done

TMPDIR_SMOKE="$(mktemp -d "${TMPDIR:-/tmp}/data-proxy-agent-status-smoke.XXXXXX")"
trap 'rm -rf "$TMPDIR_SMOKE"' EXIT

CONFIG="$TMPDIR_SMOKE/config.yaml"
WORKSPACE="$TMPDIR_SMOKE/workspace"
AUDIT="$TMPDIR_SMOKE/audit.jsonl"
SUMMARY="${OUTPUT:-$TMPDIR_SMOKE/summary.md}"
mkdir -p "$WORKSPACE"

if [[ -z "$BRIDGE_WS_URL" ]]; then
  if [[ "$BASE_URL" == https://* ]]; then
    BRIDGE_WS_URL="wss://${BASE_URL#https://}/bridge/ws"
  elif [[ "$BASE_URL" == http://* ]]; then
    BRIDGE_WS_URL="ws://${BASE_URL#http://}/bridge/ws"
  else
    die "DATA_PROXY_AGENT_SMOKE_BASE_URL must start with http:// or https://"
  fi
fi

cat >"$CONFIG" <<EOF
server:
  base_url: "$BASE_URL"
  bridge_ws_url: "$BRIDGE_WS_URL"
agent:
  client_id: "smoke-client"
  name: "smoke-agent"
  token: "$TOKEN"
  workspace: "$WORKSPACE"
logging:
  local_audit_jsonl: "$AUDIT"
http_routes:
  - name: "local-web"
    target: "$HTTP_ROUTE_TARGET"
    allow_websocket: true
    allow_sse: true
tcp_routes:
  - name: "local-tcp"
    target_host: "127.0.0.1"
    target_port: 9
mcp_servers:
  - name: "coding"
    transport: "streamable_http"
    endpoint: "http://127.0.0.1:9/mcp"
EOF
chmod 600 "$CONFIG"

{
  echo "# Data Proxy Agent Status Smoke"
  echo
  echo "| Field | Value |"
  echo "| --- | --- |"
  printf '| base_url | `%s` |\n' "$BASE_URL"
  printf '| bridge_ws_url | `%s` |\n' "$BRIDGE_WS_URL"
  printf '| http_route_target | `%s` |\n' "$HTTP_ROUTE_TARGET"
} >"$SUMMARY"

summary_row() {
  local value="${2//$'\n'/ }"
  value="${value//|/\\|}"
  printf '| %s | `%s` |\n' "$1" "$value" >>"$SUMMARY"
}

run_agent_json() {
  local output="$1"
  shift
  if ! "$AGENT_BIN" "$@" >"$output" 2>"$output.stderr"; then
    cat "$output.stderr" >&2
    return 1
  fi
  if grep -F "$TOKEN" "$output" >/dev/null; then
    die "dpa output leaked token"
  fi
  jq -e type "$output" >/dev/null
}

STATUS_JSON="$TMPDIR_SMOKE/status.json"
log "dpa status --json"
run_agent_json "$STATUS_JSON" status --json --config "$CONFIG"

jq -e '
  .config_loaded == true
  and .client_id == "smoke-client"
  and .name == "smoke-agent"
  and .token_configured == true
  and .token_source == "agent.token"
  and .bridge_ws_url == $bridge
  and .mcp_servers == 1
  and .http_routes == 1
  and .tcp_routes == 1
  and ((.routes // []) | length == 3)
  and ((.capabilities // []) | index("mcp_proxy") != null)
  and ((.capabilities // []) | index("http_tunnel") != null)
  and ((.capabilities // []) | index("tcp_tunnel") != null)
' --arg bridge "$BRIDGE_WS_URL" "$STATUS_JSON" >/dev/null || die "status --json did not include expected summary"
summary_row "status_json" "passed"

if [[ "$RUN_DOCTOR" == "1" ]]; then
  DOCTOR_JSON="$TMPDIR_SMOKE/doctor.json"
  log "dpa doctor --json"
  run_agent_json "$DOCTOR_JSON" doctor --json --config "$CONFIG" --timeout "$TIMEOUT"
  jq -e '
    .config_loaded == true
    and .token_configured == true
    and (.checks // [] | any(.name == "workspace" and .status == "ok"))
    and (.checks // [] | any(.name == "local_audit"))
  ' "$DOCTOR_JSON" >/dev/null || die "doctor --json did not include expected diagnostics"
  summary_row "doctor_json" "passed"
else
  summary_row "doctor_json" "skipped"
fi

if [[ "$RUN_ROUTE_TEST" == "1" ]]; then
  ROUTE_JSON="$TMPDIR_SMOKE/route-test.json"
  log "dpa tunnel route test local-web --json"
  if "$AGENT_BIN" tunnel route test local-web --json --config "$CONFIG" --timeout "$TIMEOUT" >"$ROUTE_JSON" 2>"$ROUTE_JSON.stderr"; then
    route_status="success"
  else
    route_status="failed_expected"
  fi
  if [[ "$ROUTE_EXPECT_SUCCESS" == "1" && "$route_status" != "success" ]]; then
    cat "$ROUTE_JSON.stderr" >&2
    die "route test was required to pass but failed"
  fi
  if grep -F "$TOKEN" "$ROUTE_JSON" "$ROUTE_JSON.stderr" >/dev/null; then
    die "route test output leaked token"
  fi
  jq -e type "$ROUTE_JSON" >/dev/null || {
    cat "$ROUTE_JSON.stderr" >&2
    die "route test did not produce JSON"
  }
  if [[ "$ROUTE_EXPECT_SUCCESS" == "1" ]]; then
    jq -e '.status == "ok"' "$ROUTE_JSON" >/dev/null || die "route test JSON status was not ok"
  fi
  summary_row "route_test_json" "$route_status"
else
  summary_row "route_test_json" "skipped"
fi

summary_row "completed_at_utc" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

if [[ -n "$OUTPUT" ]]; then
  log "summary written to $OUTPUT"
else
  cat "$SUMMARY"
fi
