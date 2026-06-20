#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
export GOTMPDIR="${GOTMPDIR:-$ROOT/.cache/go-tmp}"

ARTIFACT_PATHS=(
  .playwright-cli
  web/default/dist
  web/classic/dist
  logs
  data
  output
  .cache
)

ARTIFACT_PATTERNS=(
  '*.db'
  '*.sqlite'
  '*.sqlite3'
  '*.log'
  '*.tmp'
)

check_release_worktree() {
  local forbidden
  forbidden="$({
    git status --short --untracked-files=all -- "${ARTIFACT_PATHS[@]}" || true
    git status --short --untracked-files=all -- "${ARTIFACT_PATTERNS[@]}" || true
    git ls-files -- "${ARTIFACT_PATHS[@]}" "${ARTIFACT_PATTERNS[@]}" | sed 's/^/tracked artifact: /' || true
  } | sed '/^$/d')"

  if [[ -n "$forbidden" ]]; then
    echo "[enterprise-governance] release worktree contains local artifacts that must not be committed:" >&2
    echo "$forbidden" >&2
    echo "[enterprise-governance] remove or ignore Playwright output, logs, local DBs, caches, and build artifacts before release." >&2
    return 1
  fi
}

mkdir -p "$GOCACHE" "$GOTMPDIR"
cd "$ROOT"

echo "[enterprise-governance] release worktree artifact check"
check_release_worktree

if [[ "${1:-}" == "--artifact-check-only" ]]; then
  exit 0
fi

echo "[enterprise-governance] go test ./model ./controller ./service ./router"
go test ./model ./controller ./service ./router

echo "[enterprise-governance] go test ./service -run TestEnterpriseGovernanceRolloutRunbookR0ToR3 -count=1"
go test ./service -run TestEnterpriseGovernanceRolloutRunbookR0ToR3 -count=1

echo "[enterprise-governance] go test ./controller -run 'TestEnterprise(QuotaPolicyCreateWritesAuditLog|AuditLogFilters|UsageSummaryAndBreakdown)' -count=1"
go test ./controller -run 'TestEnterprise(QuotaPolicyCreateWritesAuditLog|AuditLogFilters|UsageSummaryAndBreakdown)' -count=1

echo "[enterprise-governance] cd web/default && pnpm typecheck"
(cd web/default && pnpm typecheck)

echo "[enterprise-governance] cd web/default && pnpm build"
(cd web/default && pnpm build)

echo "[enterprise-governance] git diff --check"
git diff --check

echo "[enterprise-governance] release worktree artifact check"
check_release_worktree
