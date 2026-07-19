#!/usr/bin/env bash
# One-command local start. User picks lite (default) or pg-redis.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/quickstart.sh              # lite: SQLite + in-process cache
  scripts/quickstart.sh lite
  scripts/quickstart.sh pg-redis     # PostgreSQL + Redis stack
  scripts/quickstart.sh standard     # alias of pg-redis

Env:
  DATA_PROXY_HOST_PORT=3000
  SESSION_SECRET=...                 # required for pg-redis if not in .env.pg-redis
EOF
}

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker is required" >&2
  exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
  echo "error: docker compose is required" >&2
  exit 1
fi

MODE="${1:-${DATA_PROXY_QUICKSTART_MODE:-lite}}"
case "$MODE" in
  -h|--help|help) usage; exit 0 ;;
  lite|self|self-use) MODE=lite ;;
  pg-redis|standard|pg|postgres) MODE=pg-redis ;;
  *)
    echo "error: unknown mode '$MODE' (use lite or pg-redis)" >&2
    usage
    exit 2
    ;;
esac

HOST_PORT="${DATA_PROXY_HOST_PORT:-3000}"

if [[ "$MODE" == "lite" ]]; then
  export DATA_PROXY_PROFILE=lite
  COMPOSE_FILE=docker-compose.lite.yml
  echo "starting Data Proxy [lite] (SQLite + process-local cache)..."
  docker compose -f "$COMPOSE_FILE" up -d --build
else
  if [[ ! -f .env.pg-redis ]]; then
    if [[ -f .env.example.pg-redis ]]; then
      cp .env.example.pg-redis .env.pg-redis
      echo "created .env.pg-redis from .env.example.pg-redis — edit SESSION_SECRET and passwords"
    else
      echo "error: missing .env.pg-redis and .env.example.pg-redis" >&2
      exit 1
    fi
  fi
  export DATA_PROXY_PROFILE=standard
  COMPOSE_FILE=docker-compose.pg-redis.yml
  echo "starting Data Proxy [pg-redis] (PostgreSQL + Redis)..."
  docker compose -f "$COMPOSE_FILE" --env-file .env.pg-redis up -d --build
fi

echo "waiting for http://127.0.0.1:${HOST_PORT}/api/status ..."
ok=0
for _ in $(seq 1 90); do
  if curl -fsS "http://127.0.0.1:${HOST_PORT}/api/status" 2>/dev/null | grep -q '"success"[[:space:]]*:[[:space:]]*true'; then
    ok=1
    break
  fi
  sleep 2
done

if [[ "$ok" -ne 1 ]]; then
  echo "error: health check failed; check: docker compose -f ${COMPOSE_FILE} logs --tail=100" >&2
  exit 2
fi

echo "Data Proxy is up: http://127.0.0.1:${HOST_PORT}"
echo "Mode: ${MODE} (DATA_PROXY_PROFILE=${DATA_PROXY_PROFILE}) — see docs/deploy-profiles.md"
echo "Next: setup wizard → admin → channel → docs/user-quickstart.md"
