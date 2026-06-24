#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/prod-ops-lib.sh
source "$ROOT/scripts/prod-ops-lib.sh"

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/prod-deploy.sh data-proxy:<tag>
  scripts/prod-deploy.sh ghcr.io/normojs/data-proxy:<tag>
  scripts/prod-deploy.sh /path/to/data-proxy-<tag>.tar

Environment:
  DATA_PROXY_IMAGE_ARCHIVE_DIR=/root/workspace/dataproxy/image-archive
  DATA_PROXY_IMAGE_ARCHIVE_KEEP=10
  DATA_PROXY_HEALTH_URL=http://127.0.0.1:13002/api/status
  DATA_PROXY_HEALTH_TIMEOUT_SECONDS=90
  DATA_PROXY_SKIP_HEALTHCHECK=1
  DATA_PROXY_EXTRA_COMPOSE_FILES=docker-compose.extra.yml:docker-compose.more.yml
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

prod_need docker
cd "$ROOT"

input="$1"
new_image=""

prod_archive_running_image

if [[ -f "$input" ]]; then
  new_image="$(prod_load_image_archive "$input")"
elif [[ "$input" == *.tar ]]; then
  prod_die "image archive not found: $input"
else
  new_image="$input"
  prod_ensure_image_present "$new_image"
fi

export DATA_PROXY_IMAGE="$new_image"
prod_log "deploy $DATA_PROXY_IMAGE with docker-compose.prod.yml + docker-compose.wechat-pay.yml"
"$ROOT/scripts/prod-compose.sh" up -d data-proxy
prod_wait_for_health
prod_log "deployment completed: $DATA_PROXY_IMAGE"
