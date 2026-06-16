# Branding and Release Policy

This repository is maintained as Data Proxy on top of the upstream New API
codebase. Brand handling must separate product runtime behavior from source
attribution.

## Runtime Product Brand

Use Data Proxy for user-facing runtime surfaces:

- browser title, favicon, logo, setup wizard, settings defaults, and home page
- navigation, dashboard, notifications, API key, MCP, Bridge, OpenAPI, billing,
  and admin UI copy
- logs, startup/help output, update checks, generated examples, and default
  operator-facing placeholders
- release notes, deployment docs, and operator setup docs created for Data Proxy

Runtime copy may still mention OpenAI-compatible APIs or upstream providers
when describing protocol compatibility.

## Source Attribution

Keep upstream attribution where it is part of project history, licensing, or
source identity:

- Go module paths and imports such as `github.com/QuantumNous/new-api`
- copyright headers and contributor notices
- AGPL Section 7 attribution requirements in the README and UI legal notices
- upstream repository, issue, release, badge, and documentation links when the
  file is preserved as upstream documentation
- package names and lockfile metadata that would require a separate migration
  plan to change safely

Do not bulk-rewrite these references during runtime brand cleanup.

## README and Installation Docs

The existing top-level README files are treated as upstream attribution
documents until Data Proxy-specific release docs fully replace them. They should
not be used as the primary operator handoff for Data Proxy releases.

Data Proxy operator guidance belongs in dedicated docs, for example:

- `docs/deployment-readiness.md`
- first-run setup and dependency configuration docs
- future Data Proxy release notes and container publishing docs

When Data Proxy-specific README files are introduced, preserve required upstream
license and attribution sections instead of deleting them.

## Desktop Packaging

The `electron/` wrapper remains upstream packaging material and is not part of
the current Data Proxy release path. Do not promise Electron desktop releases
until a separate packaging plan renames runtime paths, application names,
icons, update metadata, and legal notices consistently.

## Audit Rule

For every `New API` or `new-api` finding:

1. If it is shown to an operator or end user at runtime, change it to Data
   Proxy unless it intentionally describes upstream compatibility.
2. If it is required attribution, a module/import path, or preserved upstream
   documentation, keep it and document the reason when needed.
3. If it is release packaging, decide whether that release path is active for
   Data Proxy before changing names or image tags.
