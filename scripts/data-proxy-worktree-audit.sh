#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MODE="worktree"

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/data-proxy-worktree-audit.sh [--staged]

Default:
  Group tracked working-tree changes and untracked files by release feature line.

Options:
  --staged  Group staged changes only.

This script is read-only. It helps split the current large Data Proxy worktree
into RC0/P1/P2/P3/P4-safe commits and highlights mixed shared files that should
be staged by hunk instead of as whole files.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --staged)
    MODE="staged"
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    usage
    echo "[data-proxy-worktree-audit] unknown argument: $1" >&2
    exit 1
    ;;
  esac
  shift
done

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/data-proxy-worktree-audit.XXXXXX")"
tmp_files="$tmp_dir/files"
trap 'rm -rf "$tmp_dir"' EXIT

if [[ "$MODE" == "staged" ]]; then
  git diff --cached --name-only -z --diff-filter=ACMRTUXB HEAD -- >"$tmp_files" || true
else
  git diff --name-only -z --diff-filter=ACMRTUXB HEAD -- >"$tmp_files" || true
  git ls-files --others --exclude-standard -z >>"$tmp_files"
fi

group_for_file() {
  local file="$1"
  case "$file" in
  web/default/src/i18n/locales/en.json | \
    web/default/src/i18n/locales/zh.json | \
    web/default/src/routeTree.gen.ts | \
    web/default/src/hooks/use-sidebar-config.ts | \
    web/default/src/hooks/use-sidebar-data.ts)
    echo "MIXED_SHARED"
    ;;

  .env.example | README.md | makefile | \
    docker-compose.prod.yml | \
    docker-compose.wechat-pay.yml | \
    docker-compose.capture-storage.yml | \
    .github/workflows/* | \
    docs/data-proxy-current-release-execution-plan.md | \
    docs/data-proxy-follow-up-task-board.md | \
    docs/data-proxy-next-iteration-task-plan.md | \
    docs/data-proxy-current-development-tasks.md | \
    docs/data-proxy-near-term-development-plan.md | \
    docs/data-proxy-post-v1.3-todo.md | \
    docs/data-proxy-single-node-development-roadmap.md | \
    docs/data-proxy-vnext-stabilization-task-plan.md | \
    scripts/data-proxy-worktree-audit.sh)
    echo "RC0_RELEASE_BASELINE"
    ;;

  docs/request-capture-* | \
    web/default/src/features/training-data/* | \
    web/default/src/routes/_authenticated/training-data/*)
    echo "P2_P4_DIAGNOSTICS_CAPTURE_TRAINING"
    ;;

  controller/tunnel_* | dto/bridge.go | dto/mcp.go | model/tunnel.go | \
    pkg/bridgepolicy/* | pkg/dpagent/* | pkg/mcpgateway/* | router/tunnel-router.go | \
    service/tunnel* | service/bridge* | docs/tunnel-apps-architecture.md | \
    docs/data-proxy-agent-cli-design.md | scripts/install-data-proxy-agent.sh | \
    scripts/generate-data-proxy-agent-manifest.sh | \
    web/default/src/features/mcp/*)
    echo "P3_TUNNEL_MCP_DPA"
    ;;

  controller/pricing.go | model/pricing.go | model/*billing_event* | \
    service/pricing_actual* | service/billing_event_source_matrix* | \
    web/default/src/features/pricing/*)
    echo "PRICING_BILLING"
    ;;

  relay/* | service/openaicompat/* | service/hosted_tool_executor* | \
    dto/openai_response* | dto/channel_settings.go | \
    docs/23-responses-chat-compatibility.md | \
    docs/openai-hosted-tools-support-plan.md | \
    docs/responses-chat-completions-conversion-plan.md | \
    web/default/src/features/channels/*)
    echo "PROTOCOL_REGRESSION_GUARD_ONLY"
    ;;

  tools/fusion-benchmark*)
    echo "BENCHMARK_TOOLS"
    ;;

  *)
    echo "UNCLASSIFIED"
    ;;
  esac
}

secret_path_warning() {
  local file="$1"
  case "$file" in
  .env | .env.*)
    [[ "$file" == ".env.example" ]] || return 0
    return 1
    ;;
  secrets/* | ssl/* | logs/* | data/* | storage/* | image-archive/* | output/* | \
    *.pem | *.key | *.crt | *.p12 | *.pfx | *.tar | *.tar.gz | *.tgz | *.tar.zst | \
    *diagnostic*bundle* | *request*capture*bundle* | *raw*capture*)
    return 0
    ;;
  *)
    return 1
    ;;
  esac
}

groups=(
  RC0_RELEASE_BASELINE
  P2_P4_DIAGNOSTICS_CAPTURE_TRAINING
  P3_TUNNEL_MCP_DPA
  PRICING_BILLING
  PROTOCOL_REGRESSION_GUARD_ONLY
  BENCHMARK_TOOLS
  MIXED_SHARED
  UNCLASSIFIED
)

declare -a warnings=()

for group in "${groups[@]}"; do
  : >"$tmp_dir/$group"
done

while IFS= read -r -d '' file; do
  [[ -n "$file" ]] || continue
  group="$(group_for_file "$file")"
  printf '%s\n' "$file" >>"$tmp_dir/$group"
  if secret_path_warning "$file"; then
    warnings+=("$file")
  fi
done <"$tmp_files"

echo "# Data Proxy Worktree Audit"
echo
echo "Mode: $MODE"
echo

for group in "${groups[@]}"; do
  [[ -s "$tmp_dir/$group" ]] || continue
  count="$(sed '/^$/d' "$tmp_dir/$group" | wc -l | tr -d ' ')"
  echo "## $group ($count)"
  sed '/^$/d; s/^/- /' "$tmp_dir/$group"
  echo
done

if [[ -s "$tmp_dir/MIXED_SHARED" ]]; then
  cat <<'EOF'
## Mixed File Guidance

Stage these files by hunk. They commonly contain translations, generated route
metadata, or sidebar entries for multiple feature lines:

- web/default/src/i18n/locales/en.json
- web/default/src/i18n/locales/zh.json
- web/default/src/routeTree.gen.ts
- web/default/src/hooks/use-sidebar-config.ts
- web/default/src/hooks/use-sidebar-data.ts
EOF
  echo
fi

if [[ "${#warnings[@]}" -gt 0 ]]; then
  echo "## Path Warnings"
  printf '%s\n' "${warnings[@]}" | sed 's/^/- Sensitive or generated path pattern: /'
  echo
  exit 2
fi
