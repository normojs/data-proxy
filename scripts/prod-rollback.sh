#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/prod-ops-lib.sh
source "$ROOT/scripts/prod-ops-lib.sh"

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/prod-rollback.sh
  scripts/prod-rollback.sh /root/workspace/dataproxy/image-archive/<archive>.tar
  scripts/prod-rollback.sh ghcr.io/normojs/data-proxy:<tag-or-digest>

Without an argument, the newest local image archive is used.
EOF
}

if [[ $# -gt 1 ]]; then
  usage
  exit 2
fi

prod_need docker
cd "$ROOT"

input="${1:-}"
target_image=""
target_archive=""

if [[ -z "$input" ]]; then
  target_archive="$(prod_latest_archive)"
  [[ -n "$target_archive" ]] || prod_die "no image archive found; set DATA_PROXY_IMAGE or pass an image reference"
elif [[ -f "$input" ]]; then
  target_archive="$input"
elif [[ "$input" == *.tar ]]; then
  prod_die "image archive not found: $input"
else
  target_image="$input"
fi

prod_archive_running_image

if [[ -n "$target_archive" ]]; then
  target_image="$(prod_load_image_archive "$target_archive")"
else
  prod_ensure_image_present "$target_image"
fi

export DATA_PROXY_IMAGE="$target_image"
prod_log "rollback to $DATA_PROXY_IMAGE with docker-compose.prod.yml + docker-compose.wechat-pay.yml"
"$ROOT/scripts/prod-compose.sh" up -d data-proxy
prod_wait_for_health
prod_log "rollback completed: $DATA_PROXY_IMAGE"
