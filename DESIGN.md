# Multica Twin Design System

## 0. Research Log

- Embedded refs: extracted the existing Multica system from `docs/design.md` and `packages/ui/styles/tokens.css`; compared Linear, Notion, and Sentry; picked the Multica token system with Linear's precise review-console grammar because Twin is an operational workspace surface.
- Legacy product intent: used `docs/downstream/twin/product-goals.md` and the frozen Harness Studio `DESIGN.md` only for evidence-first review, explicit sign-off, and list/detail behavior. Legacy CSS and data topology are not copied.
- Skipped lanes: lazyweb and image drafts were skipped because this is an existing app-shell surface with no image-first requirement.

## 1. Atmosphere & Identity

Twin should feel like a quiet review room inside the Multica workspace: evidence is easy to inspect, status is explicit, and execution links back to the existing issue and agent model. The signature is a thin review spine that makes the six-step loop visible without turning the page into a marketing dashboard.

Design read: operational app shell for technical workspace users, with restrained Linear-like precision and Multica's tonal surface hierarchy. `DESIGN_VARIANCE=3`, `MOTION_INTENSITY=2`, `VISUAL_DENSITY=6`.

## 2. Color

### Palette

| Role | Token | Light | Dark | Usage |
|---|---|---|---|---|
| App shell | `bg-app-shell` | Multica app-shell token | Multica app-shell token | Outer frame and sidebar relationship |
| Page canvas | `bg-page-canvas` | Multica page-canvas token | Multica page-canvas token | Twin page scroll owner |
| Bounded surface | `bg-surface` + `border-surface-border` | Multica surface token | Multica surface token | Review groups and summary panels |
| Selected surface | `bg-surface-selected` | Multica selected token | Multica selected token | Active topic/evidence row |
| Primary text | `text-foreground` | Multica foreground token | Multica foreground token | Titles and decisions |
| Secondary text | `text-muted-foreground` | Multica muted token | Multica muted token | Descriptions and metadata |
| Brand action | `bg-brand` / `text-brand-foreground` | Multica brand token | Multica brand token | One primary review action |
| Success | `text-success` / `bg-success/10` | Multica success token | Multica success token | Signed-off and connected states |
| Warning | `text-warning` / `bg-warning/10` | Multica warning token | Multica warning token | Pending sign-off and review needed |
| Error | `text-destructive` / `bg-destructive/10` | Multica destructive token | Multica destructive token | Invalid package and load errors |

Rules: use semantic Tailwind tokens only. Accent colors carry state or action; they never decorate a whole panel. Keep one primary action per view and no more than three simultaneous semantic colors.

## 3. Typography

| Level | Token | Weight | Usage |
|---|---|---|---|
| Page title | `text-display-sm` | `font-medium` | Twin workspace heading |
| Section title | `text-title` | `font-medium` | Evidence, topics, review path |
| Body | `text-body` | `font-normal` | Descriptions and row content |
| Compact body | `text-label` | `font-medium` or `font-normal` | Navigation, statuses, actions |
| Caption | `text-caption` | `font-normal` | Timestamps, source counts, metadata |
| Technical value | `font-mono text-caption` | `font-normal` | Digests and IDs only |

Font families remain the existing `--font-sans` and `--font-mono` declarations. Do not add a display face or arbitrary sizes.

## 4. Spacing & Layout

- Base unit: 4px, using the existing Tailwind spacing scale (`gap-1` through `gap-6`).
- Page shell: one scrolling `main` with `min-h-0`, `overflow-y-auto`, and a constrained inner column (`max-w-6xl`).
- Desktop: 12-column-feeling composition expressed as a responsive two-column grid; evidence and topics collapse to one column below `lg`.
- Mobile: strict one-column flow, `px-4`, no horizontal scrolling, and actions wrap into a cluster.
- Bounded surfaces use `rounded-lg`; do not nest cards. List rows rely on spacing and one sparse separator rather than a border on every edge.

## 5. Components

### TwinPageShell
- **Structure**: scroll owner `main` -> constrained `div` -> header, status panel, tabs/content.
- **Variants**: ready, loading, empty, error.
- **Spacing**: page `gap-6`, section `gap-4`, compact group `gap-2`.
- **States**: loading preserves shell geometry; empty and error provide one next action; ready exposes review and execution links.
- **Accessibility**: one `h1`, landmark `main`, visible focus, action labels that describe destination.
- **Motion**: no route transition; color-only hover and pressed feedback.
- **Layout**: shell with the page as scroll owner.

### TwinStatusBanner
- **Structure**: status icon, state label, explanation, one primary or outline action.
- **Variants**: `pending-signoff`, `signed-off`, `invalid`, `preview`.
- **States**: default, hover/focus on action, disabled, loading action, error copy.
- **Accessibility**: state is text, not color alone; action is keyboard reachable.
- **Motion**: 150ms color transition only.
- **Layout**: bounded surface, content wraps at mobile.

### EvidenceList
- **Structure**: section heading, assertion rows, source count, expandable evidence detail.
- **Variants**: compact summary and expanded review.
- **States**: loading skeleton, empty prompt, invalid warning, populated rows, selected row.
- **Accessibility**: rows expose names, source counts, and `aria-expanded` when disclosure is used.
- **Motion**: 200ms opacity/height disclosure; disabled under reduced motion.
- **Layout**: one column; row content may wrap long evidence excerpts.

### TopicList
- **Structure**: issue-backed topic rows with state, owner, and destination action.
- **Variants**: active, waiting, accepted.
- **States**: loading, empty, error, populated, hover, active, focus.
- **Accessibility**: status has visible text; issue links are ordinary links/buttons with focus rings.
- **Motion**: color-only row hover.
- **Layout**: responsive list; no fixed-width columns on mobile.

## 6. Motion & Interaction

| Type | Duration | Easing | Usage |
|---|---:|---|---|
| Hover/focus color | 150ms | ease-out | Actions and rows |
| Disclosure | 200ms | ease-in-out | Evidence detail |
| Route change | none | none | Preserve shell stability |

All motion uses existing component transitions; disclosure may animate opacity/height. Every interactive state has hover, pressed, focus, and disabled behavior. `prefers-reduced-motion` keeps content immediately visible.

## 7. Depth & Surface

Strategy: mixed, constrained by the existing Multica system. The page canvas is unframed; bounded groups use `bg-surface border-surface-border` and the existing `--surface-shadow`; ephemeral overlays use the existing raised/floating tokens. No gradients, decorative glows, or per-section theme changes.

## 8. Accessibility Constraints & Accepted Debt

### Constraints

- WCAG 2.2 AA target: 4.5:1 body text and 3:1 large text.
- All actions are keyboard reachable with `focus-visible` rings.
- Status is conveyed by text and icon, never color alone.
- Primary content reflows to one column at 375px without horizontal scroll.
- Reduced motion is respected.

### Accepted Debt

The preview-data debt is resolved. The shared web/desktop surface now reads the parsed, workspace-scoped LM Wiki and Twin lifecycle contracts through React Query; no Twin-specific design debt is accepted.
