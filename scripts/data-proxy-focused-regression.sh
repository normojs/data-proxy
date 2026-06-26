#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_P1=1
RUN_P2=1
RUN_P3=1
RUN_FRONTEND=0

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-focused-regression.sh [--p1] [--p2] [--p3] [--all] [--frontend]

Default checks:
  - P1 channel failover, circuit breaker, user-bound groups, and token group limits;
  - P2 request trace, diagnostic bundle, capture spool/finalizer/cleanup, and training APIs;
  - P3 Tunnel, MCP Gateway, dpa, Bridge policy, and HTTP/TCP tunnel regressions.

Options:
  --p1        Run only P1 checks.
  --p2        Run only P2 checks.
  --p3        Run only P3 checks.
  --all       Run P1, P2, and P3 checks. This is also the default.
  --frontend  Also run frontend TypeScript build mode.

Environment:
  GO_TEST_ENV="GOTOOLCHAIN=auto"  Prefix for Go test commands.
EOF
}

log() {
  echo "[data-proxy-focused-regression] $*" >&2
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

if [[ $# -gt 0 ]]; then
  RUN_P1=0
  RUN_P2=0
  RUN_P3=0
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
  --p1)
    RUN_P1=1
    ;;
  --p2)
    RUN_P2=1
    ;;
  --p3)
    RUN_P3=1
    ;;
  --all)
    RUN_P1=1
    RUN_P2=1
    RUN_P3=1
    ;;
  --frontend)
    RUN_FRONTEND=1
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

need go

go_env_value="${GO_TEST_ENV:-GOTOOLCHAIN=auto}"
# shellcheck disable=SC2206
go_env_parts=($go_env_value)

run_go_test() {
  log "go test $*"
  env "${go_env_parts[@]}" go test "$@"
}

run_frontend_typecheck() {
  log "running frontend TypeScript build mode"
  local frontend_path="$PATH"
  if ! command -v node >/dev/null 2>&1; then
    local codex_node_dir="$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin"
    if [[ -x "$codex_node_dir/node" ]]; then
      frontend_path="$codex_node_dir:$frontend_path"
    fi
  fi

  if [[ -x web/default/node_modules/.bin/tsc ]]; then
    (
      cd web/default
      PATH="$frontend_path" NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=4096}" ./node_modules/.bin/tsc -b
    )
  elif PATH="$frontend_path" command -v bun >/dev/null 2>&1; then
    (
      cd web/default
      PATH="$frontend_path" NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=4096}" bun run typecheck
    )
  else
    die "frontend typecheck requires web/default/node_modules/.bin/tsc or bun"
  fi
}

if [[ "$RUN_P1" == "1" ]]; then
  log "P1: channel failover, circuit breaker, and user group restrictions"
  run_go_test ./service -run 'Test(Channel|Select|Group|UserToken|Circuit|Failover|FilterUserUsableGroups|GetUserAutoGroup)' -count=1
  run_go_test ./controller -run 'Test(Channel|Token|UserToken|Group|Failover|ListModelsHonorsBoundTokenGroups|GetModelListGroups)' -count=1
  run_go_test ./model -run 'Test(Channel|UserToken|Group|Failover)' -count=1
fi

if [[ "$RUN_P2" == "1" ]]; then
  log "P2: request trace, diagnostics, capture, and training data review APIs"
  run_go_test ./service -run 'Test(RequestCapture|BuildTrainingCorpus|Training|CleanupExpiredRequestCapture)' -count=1
  run_go_test ./controller -run 'Test(GetRequestLogTrace|Generate.*RequestDiagnostic|ListRequestDiagnosticCandidates|DownloadRequestDiagnosticBundle|RequestDiagnosticBundle|Training)' -count=1
  run_go_test ./router -run 'TestRequestDiagnosticRoutes' -count=1
fi

if [[ "$RUN_P3" == "1" ]]; then
  log "P3: Tunnel, MCP Gateway, dpa, Bridge policy, and HTTP/TCP tunnel"
  run_go_test ./service -run 'Test(Tunnel|Bridge|MCP|ForwardTunnel|CallTunnel|ListTunnel|CreateAndApprove|TunnelBilling|EnsureTunnel)' -count=1
  run_go_test ./controller -run 'Test(Bridge|MCP|Tunnel)' -count=1
  run_go_test ./router -run 'Test(SetTunnelRouter|RequestDiagnosticRoutes)' -count=1
  run_go_test ./pkg/dpagent -count=1
  run_go_test ./pkg/bridgepolicy ./pkg/mcp/executor ./pkg/mcp/proxy -run 'Test(Bridge|MCP|Remote|Validate|HTTPClient)' -count=1
fi

if [[ "$RUN_FRONTEND" == "1" ]]; then
  run_frontend_typecheck
fi

log "focused regression checks passed"
