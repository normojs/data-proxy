#!/usr/bin/env bash

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  echo "[data-proxy-prod] prod-ops-lib.sh must be sourced by another script" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

prod_log() {
  echo "[data-proxy-prod] $*" >&2
}

prod_die() {
  prod_log "$*"
  exit 1
}

prod_need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    prod_die "missing required command: $1"
  fi
}

prod_sanitize_ref() {
  local value="$1"
  value="$(printf '%s' "$value" | sed 's/[^A-Za-z0-9_.-]/_/g')"
  if [[ -z "$value" ]]; then
    value="image"
  fi
  printf '%s\n' "$value"
}

prod_parse_loaded_image() {
  awk -F'Loaded image: ' '/Loaded image: / { value=$2 } END { print value }'
}

prod_parse_loaded_image_id() {
  awk -F'Loaded image ID: ' '/Loaded image ID: / { value=$2 } END { print value }'
}

prod_latest_archive() {
  local archive_dir="${DATA_PROXY_IMAGE_ARCHIVE_DIR:-/root/workspace/dataproxy/image-archive}"
  find "$archive_dir" -maxdepth 1 -type f -name '*.tar' 2>/dev/null | sort | tail -n 1
}

prod_prune_archives() {
  local archive_dir="${DATA_PROXY_IMAGE_ARCHIVE_DIR:-/root/workspace/dataproxy/image-archive}"
  local keep="${DATA_PROXY_IMAGE_ARCHIVE_KEEP:-10}"
  local archives

  if [[ ! "$keep" =~ ^[0-9]+$ ]] || (( keep <= 0 )); then
    prod_log "skip image archive pruning because DATA_PROXY_IMAGE_ARCHIVE_KEEP=$keep"
    return 0
  fi

  mapfile -t archives < <(find "$archive_dir" -maxdepth 1 -type f -name '*.tar' 2>/dev/null | sort -r)
  for ((i = keep; i < ${#archives[@]}; i++)); do
    rm -f "${archives[$i]}" "${archives[$i]}.meta"
  done
}

prod_archive_running_image() {
  local container="${DATA_PROXY_CONTAINER_NAME:-data-proxy}"
  local archive_dir="${DATA_PROXY_IMAGE_ARCHIVE_DIR:-/root/workspace/dataproxy/image-archive}"
  local config_ref image_id save_ref timestamp safe_ref archive tmp_archive

  if ! docker container inspect "$container" >/dev/null 2>&1; then
    prod_log "container $container does not exist; skip current image archive"
    return 0
  fi

  config_ref="$(docker inspect -f '{{.Config.Image}}' "$container" 2>/dev/null || true)"
  image_id="$(docker inspect -f '{{.Image}}' "$container" 2>/dev/null || true)"
  save_ref="$config_ref"

  if [[ -z "$save_ref" ]] || ! docker image inspect "$save_ref" >/dev/null 2>&1; then
    save_ref="$image_id"
  fi
  if [[ -z "$save_ref" ]] || ! docker image inspect "$save_ref" >/dev/null 2>&1; then
    prod_log "cannot resolve image for container $container; skip current image archive"
    return 0
  fi

  mkdir -p "$archive_dir"
  timestamp="$(date -u '+%Y%m%dT%H%M%SZ')"
  safe_ref="$(prod_sanitize_ref "${config_ref:-$image_id}")"
  archive="${archive_dir}/${timestamp}_${safe_ref}.tar"
  tmp_archive="${archive}.tmp"

  prod_log "archive current image ${config_ref:-$image_id} -> $archive"
  docker image save -o "$tmp_archive" "$save_ref"
  mv "$tmp_archive" "$archive"
  cat >"${archive}.meta" <<EOF
image_ref=${config_ref}
image_id=${image_id}
saved_ref=${save_ref}
container=${container}
created_at=${timestamp}
host=$(hostname 2>/dev/null || echo unknown)
git_commit=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)
EOF

  prod_prune_archives
}

prod_load_image_archive() {
  local archive="$1"
  local output loaded_image loaded_image_id meta_ref target_image

  [[ -f "$archive" ]] || prod_die "image archive not found: $archive"

  output="$(docker load -i "$archive")"
  printf '%s\n' "$output" >&2
  loaded_image="$(printf '%s\n' "$output" | prod_parse_loaded_image)"
  loaded_image_id="$(printf '%s\n' "$output" | prod_parse_loaded_image_id)"
  meta_ref=""
  if [[ -f "${archive}.meta" ]]; then
    meta_ref="$(awk -F= '$1 == "image_ref" { print substr($0, length($1) + 2) }' "${archive}.meta" | tail -n 1)"
  fi

  target_image="${DATA_PROXY_IMAGE:-}"
  if [[ -z "$target_image" && -n "$meta_ref" && "$meta_ref" != *@sha256:* && "$meta_ref" != sha256:* ]]; then
    target_image="$meta_ref"
  fi
  if [[ -z "$target_image" && -n "$loaded_image" ]]; then
    target_image="$loaded_image"
  fi
  if [[ -z "$target_image" && -n "$meta_ref" ]]; then
    target_image="$meta_ref"
  fi

  if [[ -n "$target_image" ]] &&
    ! docker image inspect "$target_image" >/dev/null 2>&1 &&
    [[ -n "$loaded_image_id" ]] &&
    [[ "$target_image" != *@sha256:* ]] &&
    [[ "$target_image" != sha256:* ]]; then
    docker image tag "$loaded_image_id" "$target_image"
  fi

  [[ -n "$target_image" ]] || prod_die "cannot determine loaded image name; set DATA_PROXY_IMAGE explicitly"
  printf '%s\n' "$target_image"
}

prod_ensure_image_present() {
  local image="$1"
  if docker image inspect "$image" >/dev/null 2>&1; then
    return 0
  fi

  prod_log "image $image is not local; trying docker pull"
  docker pull "$image"
}

prod_wait_for_health() {
  local url="${DATA_PROXY_HEALTH_URL:-http://127.0.0.1:13002/api/status}"
  local timeout="${DATA_PROXY_HEALTH_TIMEOUT_SECONDS:-90}"
  local deadline response

  if [[ "${DATA_PROXY_SKIP_HEALTHCHECK:-0}" == "1" ]]; then
    prod_log "skip health check because DATA_PROXY_SKIP_HEALTHCHECK=1"
    return 0
  fi
  if [[ ! "$timeout" =~ ^[0-9]+$ ]] || (( timeout <= 0 )); then
    prod_die "invalid DATA_PROXY_HEALTH_TIMEOUT_SECONDS=$timeout"
  fi
  if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
    prod_log "curl/wget not found; skip health check"
    return 0
  fi

  prod_log "wait for health: $url"
  deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    if command -v curl >/dev/null 2>&1; then
      response="$(curl -fsS --max-time 3 "$url" 2>/dev/null || true)"
    else
      response="$(wget -q -O - "$url" 2>/dev/null || true)"
    fi
    if printf '%s\n' "$response" | grep -q '"success"[[:space:]]*:[[:space:]]*true'; then
      prod_log "health check passed"
      return 0
    fi
    sleep 3
  done

  prod_die "health check failed after ${timeout}s: $url"
}
