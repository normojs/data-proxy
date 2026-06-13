# UI V2 Design Brief

Status: Active pilot.

## Pilot Goal

Build UI v2 as an incremental operations shell inside `web/default`, not as a
third frontend. The first pilot focuses on MCP / Bridge / OpenAPI operations
because that area exercises dense status, policy, audit, billing, and tool-call
workflows.

## Scene

An admin is using the console during normal operations or incident triage on a
laptop. They need to understand health, risk, throughput, and failed calls
quickly, then move into tables or detail panels without losing context.

## UX Direction

- Keep the current shadcn/base-ui primitives.
- Add a v2 entry point without replacing existing v1 routes.
- Prefer dense but calm surfaces: metrics, status strips, compact filters,
  tables, trend cells, and detail panels.
- Make risk and partial data visible.
- Avoid marketing heroes and decorative dashboards.

## Architecture Direction

- Persist UI version choice in local storage first.
- Add a low-risk v2 pilot route under authenticated routes.
- Add a switcher from an existing account/profile or app shell location.
- Keep current `/mcp/*` behavior intact.
- Reuse existing MCP API clients, React Query keys, i18n, route patterns, and
  shadcn UI components.

## Pilot Acceptance

- The current UI remains available.
- A user can switch into the v2 pilot intentionally.
- The v2 pilot has a product shell, not a landing page.
- MCP pilot content covers loading, empty, partial, and error states where the
  existing data hooks expose them.
- TypeScript build and route smoke checks pass.
