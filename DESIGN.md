---
name: Data Proxy
description: Operations console for model routing, MCP, Bridge, billing, and provider control.
colors:
  background: "oklch(1 0 0)"
  foreground: "oklch(0.145 0 0)"
  card: "oklch(1 0 0)"
  muted: "oklch(0.97 0 0)"
  muted-foreground: "oklch(0.49 0 0)"
  primary: "oklch(0.13 0 0)"
  border: "oklch(0.93 0 0)"
  success: "oklch(0.596 0.145 163.225)"
  warning: "oklch(0.681 0.162 75.834)"
  destructive: "oklch(0.577 0.245 27.325)"
  info: "oklch(0.588 0.158 241.966)"
typography:
  headline:
    fontFamily: "Public Sans Variable, Public Sans, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: "0"
  title:
    fontFamily: "Public Sans Variable, Public Sans, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 600
    lineHeight: 1.35
    letterSpacing: "0"
  body:
    fontFamily: "Public Sans Variable, Public Sans, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0"
  label:
    fontFamily: "Public Sans Variable, Public Sans, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.25
    letterSpacing: "0"
rounded:
  sm: "0.375rem"
  md: "0.5rem"
  lg: "0.75rem"
  xl: "1rem"
spacing:
  xs: "0.25rem"
  sm: "0.5rem"
  md: "1rem"
  lg: "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.background}"
    rounded: "{rounded.lg}"
    height: "2rem"
    padding: "0 0.625rem"
  input-default:
    backgroundColor: "transparent"
    textColor: "{colors.foreground}"
    rounded: "{rounded.lg}"
    height: "2rem"
  card-default:
    backgroundColor: "{colors.card}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.xl}"
    padding: "1rem"
---

# Design System: Data Proxy

## 1. Overview

**Creative North Star: "The Operations Ledger"**

Data Proxy should look like a precise operations ledger for model and tool
infrastructure. The current visual system is restrained: neutral surfaces,
Public Sans typography, compact controls, shadcn/base-ui primitives, and state
colors reserved for actual system meaning. UI v2 should keep that trust while
making high-density operations areas easier to scan.

The system rejects decorative dashboards and marketing-style layouts. Pages
should open directly into work: filters, status, metrics, tables, detail panels,
and actions. When a screen needs personality, it should come from clarity,
rhythm, and useful status hierarchy, not from novelty.

**Key Characteristics:**
- Neutral, high-contrast surfaces with semantic status color.
- Compact controls with stable heights and predictable focus states.
- Tables, tabs, filters, menus, and detail panels as first-class patterns.
- Motion that explains navigation or state only.

## 2. Colors

The palette is a restrained neutral system with semantic accents for status.

### Primary
- **Ledger Ink** (`oklch(0.13 0 0)`): Primary actions, active navigation, and
  selected states. Use sparingly so the operator can find the next action.

### Neutral
- **Console White** (`oklch(1 0 0)`): Default content background and card
  surface in light mode.
- **Working Gray** (`oklch(0.97 0 0)`): Muted panels, grouped rows, and quiet
  secondary surfaces.
- **Readable Muted Text** (`oklch(0.49 0 0)`): Secondary labels and metadata.
- **Divider Line** (`oklch(0.93 0 0)`): Structural borders and table dividers.

### State
- **Healthy Green** (`oklch(0.596 0.145 163.225)`): Success, online, settled.
- **Operator Amber** (`oklch(0.681 0.162 75.834)`): Warning, partial data,
  unsettled work.
- **Failure Red** (`oklch(0.577 0.245 27.325)`): Destructive and failed state.
- **Signal Blue** (`oklch(0.588 0.158 241.966)`): Informational state.

### Named Rules

**The Semantic Color Rule.** Status color is for system meaning only. Do not use
green, amber, red, or blue as decoration.

## 3. Typography

**Display Font:** Public Sans Variable, Public Sans, system-ui, sans-serif
**Body Font:** Public Sans Variable, Public Sans, system-ui, sans-serif
**Label/Mono Font:** Public Sans with tabular numbers where data comparison
requires it.

**Character:** The type system is familiar and compact. It should read as a
serious operator tool rather than a branded editorial surface.

### Hierarchy
- **Headline** (700, 1.125rem, 1.25): Page and section titles in app chrome.
- **Title** (600, 1rem, 1.35): Cards, panels, table group titles.
- **Body** (400, 0.875rem, 1.5): Operational copy, descriptions, row metadata.
- **Label** (500, 0.75rem, 1.25): Badges, filter labels, compact metadata.

### Named Rules

**The Fixed Scale Rule.** Product UI uses fixed rem sizes, not viewport-fluid
type. Large display type belongs to public pages only.

## 4. Elevation

Depth is mostly conveyed through tonal layering and 1px rings, not heavy
shadows. Product surfaces should be flat at rest. Hover, focus, and active
states can use background shifts and focus rings.

### Shadow Vocabulary
- **Focus Ring** (`ring-3 ring-ring/50`): Keyboard focus and invalid controls.
- **Surface Ring** (`ring-1 ring-foreground/10`): Card/container separation.

### Named Rules

**The Flat-By-Default Rule.** Do not pair broad soft shadows with 1px borders on
ordinary cards. If a surface needs separation, choose a ring or a tonal layer.

## 5. Components

### Buttons
- **Shape:** Compact rounded rectangles (`rounded-lg`, 0.5rem).
- **Primary:** Ledger Ink background with Console White text, 2rem default
  height, icon-capable spacing.
- **Hover / Focus:** Background opacity shift, visible 3px focus ring.
- **Secondary / Ghost:** Muted hover states, no decorative fills at rest.

### Chips
- **Style:** Use badges/status components for compact state. Text must not rely
  on color alone.
- **State:** Selected filters should be visually distinct from passive badges.

### Cards / Containers
- **Corner Style:** 0.75rem to 1rem; never oversized.
- **Background:** Card or muted surface depending on hierarchy.
- **Shadow Strategy:** Ring or border, not decorative shadow.
- **Internal Padding:** 0.75rem to 1rem for dense product panels.

### Inputs / Fields
- **Style:** 2rem height, 0.5rem radius, transparent or muted background.
- **Focus:** Border/ring shift with clear keyboard visibility.
- **Error / Disabled:** Error ring and disabled opacity/background states.

### Navigation
- **Style:** App header plus sidebar. Nested workspaces use drill-in sidebar
  views. Current route state must be obvious, compact, and keyboard reachable.

### Operations Panels
- **Style:** Prefer tables, split panes, compact trend strips, and detail
  panels. Avoid opening a modal for every inspection task.

## 6. Do's and Don'ts

### Do:
- **Do** keep operator pages dense, scannable, and aligned to standard product
  affordances.
- **Do** reserve semantic colors for health, risk, and state.
- **Do** provide loading, empty, partial, error, and permission states for v2
  pilot surfaces.
- **Do** evolve `web/default` incrementally, with route-level rollback plans for
  risky UI v2 pilot surfaces.

### Don't:
- **Don't** use giant hero panels, marketing dashboard composition, or novelty
  AI cockpit visuals in authenticated product routes.
- **Don't** use purple-blue gradient branding, gradient text, glassmorphism, or
  decorative card grids.
- **Don't** invent custom controls where standard tabs, tables, switches, menus,
  and detail panels fit.
- **Don't** hide risk or billing impact away from the action that creates it.
