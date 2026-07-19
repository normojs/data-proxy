#!/usr/bin/env bash
# Minimal one-command local start for Data Proxy.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker is required" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "error: docker compose is required" >&2
  exit 1
fi

if [[ ! -f .env ]] && [[ -f .env.example.minimal ]]; then
  cp .env.example.minimal .env
  echo "created .env from .env.example.minimal (edit SESSION_SECRET for production)"
fi

# Path A defaults to lite (SQLite + in-process cache). Override with DATA_PROXY_PROFILE=standard if needed.
export DATA_PROXY_PROFILE="${DATA_PROXY_PROFILE:-lite}"

echo "starting data-proxy (docker compose up -d --build, profile=${DATA_PROXY_PROFILE})..."
docker compose up -d --build

echo "waiting for /api/status ..."
ok=0
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:3000/api/status" 2>/dev/null | grep -q '"success"[[:space:]]*:[[:space:]]*true'; then
    ok=1
    break
  fi
  sleep 2
done

if [[ "$ok" -ne 1 ]]; then
  echo "error: health check failed after waiting; check: docker compose logs --tail=100" >&2
  exit 2
fi

echo "Data Proxy is up: http://127.0.0.1:3000"
echo "Deploy profile: ${DATA_PROXY_PROFILE} (see docs/deploy-profiles.md)"
echo "Next: open the setup wizard, create admin, add a channel, then see docs/user-quickstart.md"
