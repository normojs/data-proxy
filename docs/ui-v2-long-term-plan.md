# UI V2 Long-Term Plan

Status: Activated as an incremental pilot. The user reactivated this plan after
the backend and migration tasks were completed.

## Decision

The current `web/default` frontend already uses the shadcn ecosystem:

- `web/default/components.json` configures `base-nova`, Tailwind CSS v4,
  Hugeicons, and the `@/components/ui` alias.
- `web/default/src/styles/index.css` imports `shadcn/tailwind.css`.
- `web/default/src/components/ui` contains the existing shadcn-style primitive
  components.

The long-term UI work should therefore be treated as a product UI redesign on
top of the existing shadcn frontend, not as a new shadcn migration.

## Scope

Keep both existing frontend generations:

- `web/classic`: legacy UI, preserved for compatibility.
- `web/default`: current primary UI and the future home of UI v2 work.

Do not create a third frontend app unless a later architecture review proves it
is cheaper than evolving `web/default`.

## Product Direction

UI v2 should optimize for an operations-heavy product surface:

- Primary users: admins, operators, and developers managing model routing,
  channels, MCP, Bridge, OpenAPI tools, billing events, and audits.
- Context: repeated work, incident triage, configuration review, and data-heavy
  comparison.
- Design register: product UI, restrained, dense, predictable, and trustworthy.
- Anti-goal: a decorative marketing-style dashboard with large hero panels,
  card-heavy composition, or novelty controls.

## First Candidate Surface

When this plan is activated, start with the MCP / Bridge / OpenAPI operations
area because it exercises the hardest UI needs:

- overview metrics and trends
- review queue and health signals
- Bridge client policy and session detail
- OpenAPI binary object management
- proxy servers, tools, discovery, and heartbeat states
- tool calls, audit logs, billing events, refunds, and relation repair

A successful pilot here gives the design system enough evidence before wider
rollout.

## Recommended Architecture

Prefer an incremental v2 shell inside `web/default`:

- Add a UI version switcher in a low-risk location such as the profile menu or
  settings area.
- Persist the choice in local storage or a user preference when the backend
  supports it.
- Mount pilot routes under `/ui-lab/*` or `/v2/*` until the design is stable.
- Reuse existing auth, API clients, React Query keys, TanStack Router patterns,
  i18n, and shadcn components.
- Keep the current UI as the default until v2 passes usability and regression
  checks.

Avoid mixing v1 and v2 component vocabulary inside a single production page
unless the page is explicitly part of the pilot.

## Design Prerequisites

Before implementation starts:

- Create `PRODUCT.md` for strategic design context.
- Create `DESIGN.md` documenting current tokens, typography, component
  vocabulary, and UI pain points.
- Write a UI v2 design brief that names the pilot surface, visual direction,
  information architecture, density, key states, and interaction model.
- Use shadcn CLI / registry tooling for component inspection and additions, but
  do not assume generated components solve the product design problem by
  themselves.

## Phases

1. Design context
   - Produce `PRODUCT.md`, `DESIGN.md`, and a confirmed UI v2 design brief.
   - Identify what the current UI gets wrong in concrete terms.

2. Pilot shell
   - Add the UI version switcher and route guard.
   - Add a small v2 layout shell with navigation, page header, breadcrumbs, and
     density rules.

3. MCP operations pilot
   - Rebuild the MCP overview and one detail-heavy table/detail flow.
   - Cover loading, empty, partial, error, and permission states.
   - Validate responsive behavior and keyboard navigation.

4. Rollout decision
   - Compare v2 against current UI using screenshots, operator workflows, and
     regression checks.
   - Either expand to channels/models/keys or keep the pilot internal.

5. Migration or retirement
   - Promote stable v2 routes to the default UI only after parity checks pass.
   - Keep `web/classic` untouched unless a separate decision retires it.

## Activation Gate

Activated on 2026-06-13 after the Docker-backed migration gates and near-term
backend hardening work were completed. Implementation should continue through
the task list in `todo.md`, keeping the current UI available until the v2 pilot
passes validation.
