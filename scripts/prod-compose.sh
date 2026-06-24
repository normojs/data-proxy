#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

compose_files=(
  docker-compose.prod.yml
  docker-compose.wechat-pay.yml
)

if [[ -n "${DATA_PROXY_EXTRA_COMPOSE_FILES:-}" ]]; then
  IFS=':' read -r -a extra_files <<<"${DATA_PROXY_EXTRA_COMPOSE_FILES}"
  for file in "${extra_files[@]}"; do
    [[ -n "$file" ]] && compose_files+=("$file")
  done
fi

compose_args=()
for file in "${compose_files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[data-proxy-prod] compose file not found: $file" >&2
    exit 1
  fi
  compose_args+=("-f" "$file")
done

exec docker compose "${compose_args[@]}" "$@"
