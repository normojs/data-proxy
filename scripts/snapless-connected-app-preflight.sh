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
    echo "[snapless-connected-app] release worktree contains local artifacts that must not be committed:" >&2
    echo "$forbidden" >&2
    echo "[snapless-connected-app] remove or ignore Playwright output, logs, local DBs, caches, and build artifacts before release." >&2
    return 1
  fi
}

require_file_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  if ! grep -Fq "$pattern" "$file"; then
    echo "[snapless-connected-app] missing ${label}: ${file} must contain ${pattern}" >&2
    return 1
  fi
}

mkdir -p "$GOCACHE" "$GOTMPDIR"
cd "$ROOT"

echo "[snapless-connected-app] release worktree artifact check"
check_release_worktree

if [[ "${1:-}" == "--artifact-check-only" ]]; then
  exit 0
fi

echo "[snapless-connected-app] connected app router preflight"
go test ./router -run 'TestConnectedApp(RequestApprovalCreatesAppAuditAndNotifications|DeveloperAPIAndDeviceFlow|DeveloperSelfService|RequestRejectsDuplicateAndInvalidReview)' -count=1

echo "[snapless-connected-app] snapless device flow preflight"
go test ./router -run 'TestSnapless(EnsureCreatesReusableConnectedAppToken|DeviceFlowAuthorizesAndReturnsKeyOnce|ConfigDeviceListRotateAndRevokeDevices|DeviceFlowRejectsWhenReadinessFails)' -count=1

echo "[snapless-connected-app] connected app notification outbox preflight"
go test ./service -run 'Test(EnqueueConnectedAppRequestReviewOutboxRespectsPreferencesAndIsIdempotent|SendConnectedAppWebhookWithSignatureAndPayloadVersion|EnqueueConnectedAppTokenLifecycleOutboxRespectsPreferencesAndIsIdempotent|ConnectedAppNotificationRejectsInvalidAppIdAndWebhookEvent)' -count=1

echo "[snapless-connected-app] MCP billing regression check"
go test ./pkg/mcp/billing ./pkg/mcp/proxy ./pkg/mcp/executor -count=1

echo "[snapless-connected-app] frontend connected app lint"
(cd web/default && bunx eslint src/features/snapless-device/developer-self-service-panel.tsx src/features/snapless-device/developer-apps-card.tsx src/features/system-settings/operations/connected-apps-api.ts src/features/system-settings/operations/connected-apps-section.tsx src/features/system-settings/operations/connected-app-notifications-section.tsx)

echo "[snapless-connected-app] frontend typecheck"
(cd web/default && bun run typecheck)

echo "[snapless-connected-app] frontend build"
(cd web/default && bun run build)

echo "[snapless-connected-app] release compliance check"
test -f LICENSE
test -f NOTICE
test -f THIRD-PARTY-LICENSES.md
require_file_contains README.md "new-api" "new-api attribution"
require_file_contains README.md "AGPLv3" "AGPLv3 notice"
require_file_contains NOTICE "Frontend design and development by New API contributors." "NOTICE Section 7 attribution text"
require_file_contains NOTICE "https://github.com/QuantumNous/new-api" "upstream project link"
require_file_contains Dockerfile "COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/" "Docker license copy"

echo "[snapless-connected-app] whitespace check"
git diff --check

echo "[snapless-connected-app] release worktree artifact check"
check_release_worktree

echo "[snapless-connected-app] preflight passed"
