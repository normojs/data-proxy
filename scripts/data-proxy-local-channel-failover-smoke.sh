#!/usr/bin/env bash
set -euo pipefail

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[data-proxy-local-failover-smoke] missing required command: $1" >&2
    exit 1
  fi
}

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-local-channel-failover-smoke.sh

Purpose:
  Run a local-only same-model channel failover smoke. It creates a temporary
  SQLite database, one synthetic bad upstream returning 502, one synthetic
  backup upstream returning a valid Chat Completions response, then verifies
  request trace and diagnostic candidate evidence.

Optional:
  DATA_PROXY_LOCAL_FAILOVER_OUTPUT=/path/summary.md
      Write the smoke markdown summary to a file as well as stdout.

This script does not use production API keys, production Redis, or production
database connections.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
if [[ $# -gt 0 ]]; then
  usage
  echo "[data-proxy-local-failover-smoke] unknown argument: $1" >&2
  exit 1
fi

need go

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT="${DATA_PROXY_LOCAL_FAILOVER_OUTPUT:-}"

cd "${REPO_ROOT}"

if [[ -n "${OUTPUT}" ]]; then
  mkdir -p "$(dirname "${OUTPUT}")"
  go run ./tools/channel_failover_local_smoke | tee "${OUTPUT}"
else
  go run ./tools/channel_failover_local_smoke
fi
