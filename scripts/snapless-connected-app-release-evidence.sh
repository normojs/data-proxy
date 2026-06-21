#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="${REPO:-normojs/data-proxy}"
REMOTE="${REMOTE:-normojs}"
WORKFLOW="${WORKFLOW:-CI}"
DOCKER_WORKFLOW="${DOCKER_WORKFLOW:-Publish Data Proxy image}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[snapless-release-evidence] missing required command: $1" >&2
    exit 1
  fi
}

optional() {
  command -v "$1" >/dev/null 2>&1
}

need git
cd "$ROOT"

COMMIT="${COMMIT:-$(git rev-parse HEAD)}"
SHORT_COMMIT="$(git rev-parse --short=12 "$COMMIT")"
BRANCH="${BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
TAG="${TAG:-}"
if [[ -z "$TAG" ]]; then
  TAG="$(git describe --tags --exact-match "$COMMIT" 2>/dev/null || true)"
fi

REMOTE_URL="$(git remote get-url "$REMOTE" 2>/dev/null || true)"
RUN_JSON=""
RUN_ID=""
RUN_URL=""
RUN_STATUS=""
RUN_CONCLUSION=""
DOCKER_RUN_JSON=""
DOCKER_RUN_ID=""
DOCKER_RUN_URL=""
DOCKER_RUN_STATUS=""
DOCKER_RUN_CONCLUSION=""
IMAGE_DIGEST=""
IMAGE_REPO="$(printf '%s' "$REPO" | tr '[:upper:]' '[:lower:]')"
IMAGE="ghcr.io/${IMAGE_REPO}"

if optional gh; then
  RUN_JSON="$(gh run list \
    --repo "$REPO" \
    --workflow "$WORKFLOW" \
    --branch "$BRANCH" \
    --limit 30 \
    --json databaseId,headSha,status,conclusion,url,createdAt,displayTitle \
    --jq "map(select(.headSha == \"$COMMIT\")) | first | [.databaseId, .url, .status, (.conclusion // \"\")] | @tsv" 2>/dev/null || true)"
  if [[ -n "$RUN_JSON" ]]; then
    IFS=$'\t' read -r RUN_ID RUN_URL RUN_STATUS RUN_CONCLUSION <<< "$RUN_JSON"
  fi

  if [[ -n "$TAG" ]]; then
    DOCKER_RUN_JSON="$(gh run list \
      --repo "$REPO" \
      --workflow "$DOCKER_WORKFLOW" \
      --limit 30 \
      --json databaseId,headSha,status,conclusion,url,createdAt,displayTitle \
      --jq "map(select(.headSha == \"$COMMIT\")) | first | [.databaseId, .url, .status, (.conclusion // \"\")] | @tsv" 2>/dev/null || true)"
    if [[ -n "$DOCKER_RUN_JSON" ]]; then
      IFS=$'\t' read -r DOCKER_RUN_ID DOCKER_RUN_URL DOCKER_RUN_STATUS DOCKER_RUN_CONCLUSION <<< "$DOCKER_RUN_JSON"
    fi
  fi
fi

if [[ -n "$TAG" ]] && optional docker; then
  IMAGE_DIGEST="$(docker buildx imagetools inspect "${IMAGE}:${TAG}" --format '{{ .Manifest.Digest }}' 2>/dev/null || true)"
fi

echo "# Snapless Connected App Release Evidence Snapshot"
echo
echo "| Field | Value |"
echo "| --- | --- |"
echo "| Repository | \`${REPO}\` |"
echo "| Remote | \`${REMOTE_URL:-unknown}\` |"
echo "| Branch | \`${BRANCH}\` |"
echo "| Commit | \`${COMMIT}\` |"
echo "| Short commit | \`${SHORT_COMMIT}\` |"
echo "| Git tag | \`${TAG:-not tagged}\` |"
echo "| CI workflow | \`${WORKFLOW}\` |"
echo "| CI run | ${RUN_URL:-not found for commit} |"
echo "| CI status | \`${RUN_STATUS:-unknown}\` |"
echo "| CI conclusion | \`${RUN_CONCLUSION:-unknown}\` |"
echo "| Docker workflow | ${DOCKER_RUN_URL:-not found or not tagged} |"
echo "| Docker workflow status | \`${DOCKER_RUN_STATUS:-unknown}\` |"
echo "| Docker workflow conclusion | \`${DOCKER_RUN_CONCLUSION:-unknown}\` |"
echo "| Docker image | \`${IMAGE}${TAG:+:${TAG}}\` |"
echo "| Docker digest | \`${IMAGE_DIGEST:-not resolved}\` |"
echo

if [[ -n "$RUN_ID" ]]; then
  echo "## CI Jobs"
  echo
  echo "| Job | Status | Conclusion | URL |"
  echo "| --- | --- | --- | --- |"
  gh run view "$RUN_ID" \
    --repo "$REPO" \
    --json jobs \
    --jq '.jobs[] | {name, status, conclusion: (.conclusion // "unknown"), url} | "| \(.name) | \(.status) | \(.conclusion) | \(.url) |"' 2>/dev/null || true
  echo
fi

cat <<EOF
## Manual Evidence Still Required

- Preprod request ID, outbox ID, token ID, screenshots or change ticket.
- Preprod approval/device flow/developer key/usage/outbox verification results.
- Release tag and Docker digest after the tag publish workflow completes.
- Rollback image digest and rollback owner.
EOF
