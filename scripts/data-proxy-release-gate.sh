#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_TESTS=0
SCAN_ALL=0
WITH_DOCKER_CONFIG=0
STRICT_SECRETS="${DATA_PROXY_RELEASE_GATE_STRICT_SECRETS:-0}"

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-release-gate.sh [--with-tests] [--with-docker-config] [--scan-all]

Default checks:
  - release artifact path hygiene for changed and untracked files;
  - hard failure for private-key material in changed files;
  - warning for token-like strings in changed files;
  - new-api license/NOTICE attribution presence;
  - production deploy/rollback script presence and executable bits;
  - tracked-change whitespace check.

Options:
  --with-tests          Run focused backend tests and frontend typecheck.
  --with-docker-config  Run production compose config validation.
  --scan-all           Scan all tracked files instead of only changed/untracked files.

Environment:
  DATA_PROXY_RELEASE_GATE_STRICT_SECRETS=1  Treat token-like secret warnings as failures.
  GO_TEST_ENV="GOTOOLCHAIN=auto"            Prefix for Go test commands.
EOF
}

log() {
  echo "[data-proxy-release-gate] $*" >&2
}

die() {
  log "$*"
  exit 1
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "missing required command: $1"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --with-tests)
    RUN_TESTS=1
    ;;
  --with-docker-config)
    WITH_DOCKER_CONFIG=1
    ;;
  --scan-all)
    SCAN_ALL=1
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    usage
    die "unknown argument: $1"
    ;;
  esac
  shift
done

need git

TMP_FILES="$(mktemp "${TMPDIR:-/tmp}/data-proxy-release-gate.XXXXXX")"
trap 'rm -f "$TMP_FILES"' EXIT

if [[ "$SCAN_ALL" == "1" ]]; then
  git ls-files -z >"$TMP_FILES"
else
  git diff --name-only -z --diff-filter=ACMRTUXB HEAD -- >>"$TMP_FILES" || true
  git ls-files --others --exclude-standard -z >>"$TMP_FILES"
fi

is_text_file() {
  local file="$1"
  [[ -f "$file" ]] || return 1
  [[ "$(wc -c <"$file" | tr -d ' ')" -le 2097152 ]] || return 1
  LC_ALL=C grep -Iq . "$file"
}

failures=()
warnings=()

add_failure() {
  failures+=("$1")
}

add_warning() {
  warnings+=("$1")
}

check_release_path() {
  local file="$1"
  case "$file" in
  .env | .env.*)
    [[ "$file" == ".env.example" || "$file" == ".env.example.minimal" ]] ||
      add_failure "secret env file must not be tracked or staged: $file"
    ;;
  secrets/* | ssl/* | logs/* | data/* | storage/* | image-archive/* | output/*)
    add_failure "local runtime/storage path must not be tracked or staged: $file"
    ;;
  *.pem | *.key | *.crt | *.p12 | *.pfx)
    add_failure "certificate or private-key file must not be tracked or staged: $file"
    ;;
  *.tar | *.tar.gz | *.tgz | *.tar.zst)
    add_failure "image/archive artifact must not be tracked or staged: $file"
    ;;
  *diagnostic*bundle* | *request*capture*bundle* | *raw*capture*)
    add_failure "diagnostic or raw-capture bundle must not be tracked or staged: $file"
    ;;
  esac
}

scan_pattern_locations() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  local severity="$4"
  local lines

  if [[ "$severity" == "warn" ]]; then
    lines="$(
      LC_ALL=C grep -nE "$pattern" "$file" 2>/dev/null |
        LC_ALL=C grep -Evi 'e\.g\.|example|placeholder|dummy|redacted|masked|YOUR_|xxx' |
        cut -d: -f1 |
        tr '\n' ' ' |
        sed 's/[[:space:]]*$//' ||
        true
    )"
  else
    lines="$(
      LC_ALL=C grep -nE "$pattern" "$file" 2>/dev/null |
        cut -d: -f1 |
        tr '\n' ' ' |
        sed 's/[[:space:]]*$//' ||
        true
    )"
  fi
  [[ -n "$lines" ]] || return 0

  if [[ "$severity" == "fail" ]]; then
    add_failure "$label in $file at line(s): $lines"
  else
    add_warning "$label in $file at line(s): $lines"
  fi
}

log "checking changed file paths and secret patterns"
while IFS= read -r -d '' file; do
  [[ -n "$file" ]] || continue
  check_release_path "$file"
  if ! is_text_file "$file"; then
    continue
  fi

  scan_pattern_locations "$file" '-----BEGIN (RSA |DSA |EC |OPENSSH |ENCRYPTED )?PRIVATE KEY-----|-----BEGIN PRIVATE KEY-----' "private key material" fail
  scan_pattern_locations "$file" 'sk-[A-Za-z0-9]{32,}|gh[pousr]_[A-Za-z0-9_]{30,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|xox[baprs]-[0-9A-Za-z-]+' "token-like secret" warn
done <"$TMP_FILES"

if [[ "${#warnings[@]}" -gt 0 ]]; then
  log "warnings:"
  printf '  - %s\n' "${warnings[@]}" >&2
  if [[ "$STRICT_SECRETS" == "1" ]]; then
    add_failure "DATA_PROXY_RELEASE_GATE_STRICT_SECRETS=1 converts token-like warnings to failures"
  fi
fi

log "checking new-api license and attribution files"
[[ -f LICENSE ]] || add_failure "missing LICENSE"
[[ -f NOTICE ]] || add_failure "missing NOTICE"
[[ -f THIRD-PARTY-LICENSES.md ]] || add_failure "missing THIRD-PARTY-LICENSES.md"
if ! grep -Fq "AGPLv3" README.md; then
  add_failure "README.md must mention AGPLv3"
fi
if ! grep -Fq "new-api" README.md; then
  add_failure "README.md must mention new-api"
fi
if ! grep -Fq "Frontend design and development by New API contributors." NOTICE; then
  add_failure "NOTICE must keep new-api attribution text"
fi
if ! grep -Fq "https://github.com/QuantumNous/new-api" NOTICE; then
  add_failure "NOTICE must keep upstream new-api link"
fi
if ! grep -Fq "COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/" Dockerfile; then
  add_failure "Dockerfile must copy license files into image"
fi

log "checking production release scripts"
for script in \
  scripts/prod-compose.sh \
  scripts/prod-deploy.sh \
  scripts/prod-rollback.sh \
  scripts/prod-ops-lib.sh
do
  [[ -f "$script" ]] || add_failure "missing production script: $script"
  [[ -x "$script" ]] || add_failure "production script is not executable: $script"
done

[[ -f docker-compose.prod.yml ]] || add_failure "missing docker-compose.prod.yml"
[[ -f docker-compose.wechat-pay.yml ]] || add_failure "missing docker-compose.wechat-pay.yml"
[[ -f .github/workflows/ci.yml ]] || add_failure "missing CI workflow"
[[ -f .github/workflows/data-proxy-docker.yml ]] || add_failure "missing Data Proxy Docker workflow"

log "checking whitespace for tracked changes"
git diff --check

if [[ "$WITH_DOCKER_CONFIG" == "1" ]]; then
  need docker
  log "checking production compose config"
  scripts/prod-compose.sh config >/dev/null
fi

if [[ "$RUN_TESTS" == "1" ]]; then
  need go
  log "running focused backend tests"
  go_env_value="${GO_TEST_ENV:-GOTOOLCHAIN=auto}"
  # shellcheck disable=SC2206
  go_env_parts=($go_env_value)
  env "${go_env_parts[@]}" go test ./model ./service ./controller ./router ./pkg/dpagent ./pkg/bridgepolicy ./service/openaicompat ./relay/channel/openai ./relay

  log "running frontend typecheck"
  frontend_path="$PATH"
  if ! command -v node >/dev/null 2>&1; then
    codex_node_dir="$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin"
    if [[ -x "$codex_node_dir/node" ]]; then
      frontend_path="$codex_node_dir:$frontend_path"
    fi
  fi
  if [[ -x web/default/node_modules/.bin/tsc ]]; then
    (cd web/default && PATH="$frontend_path" NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=4096}" ./node_modules/.bin/tsc -b)
  elif PATH="$frontend_path" command -v bun >/dev/null 2>&1; then
    (cd web/default && PATH="$frontend_path" NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=4096}" bun run typecheck)
  else
    die "frontend typecheck requires web/default/node_modules/.bin/tsc or bun"
  fi
fi

if [[ "${#failures[@]}" -gt 0 ]]; then
  log "failed:"
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

log "release gate passed"
