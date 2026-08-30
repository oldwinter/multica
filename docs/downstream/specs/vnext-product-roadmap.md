# Downstream Product Roadmap: From Shipped Loops To Repeated Value

Date: 2026-08-25

Status: `proposed`

## Scope

This roadmap treats the current downstream work as three product initiatives:

1. Named Skins as a reliable personalization and quality system.
2. Rooms as an asynchronous outcome workflow.
3. Twin + Wiki as one governed knowledge-to-execution loop, delivered through separate Wiki and Twin implementation specs.

The baseline contracts are shipped. The next investment is deliberately product-facing: reduce activation friction, route unfinished human decisions, support repeat use, and prove value without weakening the existing safety boundaries.

## Current Diagnosis

| Initiative | What is already real | Highest-leverage remaining gap | Product judgment |
| --- | --- | --- | --- |
| Named Skins | Semantic tokens, account sync, offline reconciliation, startup correctness, accessibility contracts, diagnostics | Abstract previews, weak Undo/recovery surface, broad visual maintenance cost | Useful but bounded. Improve confidence and recovery, then keep in maintenance mode. |
| Rooms | Explicit outcome lifecycle, synthesis, cited memory, review, promotion, budgets, preflight, Mobile support, metrics | Creation is configuration-heavy; asynchronous gates do not enter Inbox; successful setups are not easy to repeat | High user value if it becomes a recurring workflow instead of a place users must remember to revisit. |
| Twin + Wiki | Revisioned Wiki, source policy, immutable evidence, signed assertions, briefing injection, attribution, feedback, deposition | Multi-surface activation, raw UUID entry, no unified stale/pending queue, weak comparison evidence | Highest strategic differentiation, but current activation cost can hide the value of the shipped execution loop. |

## Prioritization Method

Score each proposal from 1 to 5. Higher effort score means easier delivery; higher dependency score means lower dependency risk.

| Proposal | User impact 30% | Business value 25% | Strategic fit 20% | Delivery leverage 15% | Dependency confidence 10% | Weighted score |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Wiki guided knowledge activation | 5 | 5 | 5 | 3 | 4 | 92 |
| Twin guided execution effectiveness | 5 | 5 | 5 | 3 | 4 | 92 |
| Rooms attention and reuse | 5 | 4 | 5 | 3 | 4 | 87 |
| Named Skins confidence and recovery | 3 | 2 | 3 | 4 | 5 | 62 |

The scores prioritize two adoption blockers while explicitly capping investment in appearance expansion.

## Evidence Snapshot

- `packages/views/settings/components/preferences-tab.tsx` exposes the three synchronized skins and recovery state, but its picker preview is abstract and successful writes have no bounded Undo action.
- `packages/views/rooms/create-room-dialog.tsx` implements the complete outcome contract in one creation surface, while `packages/core/types/inbox.ts` has no Room action types. The execution loop is strong, but first use and asynchronous attention remain disconnected.
- `packages/views/twins/components/twin-use-panel.tsx` requires raw scope and preview IDs even though workspace Agent, Project, and Issue query contracts already exist. Wiki source policy is governed but centralized under the Twin flow, so an eligible page has no contextual path into that policy.
- Existing E2E coverage proves the baseline flows for account appearance, durable Room outcomes, Wiki evidence, signed Twins, execution attribution, and edge states. The vNext work should extend those scenarios instead of replacing them.

## Recommended Sequence

### Phase 1: Remove Activation Blockers

- Replace Twin policy and preview UUID inputs with real Agent, Project, and Issue selectors.
- Make Twin readiness expose one deterministic next action.
- Simplify Rooms creation around templates and a concise preflight summary.
- Align empty-state copy with outcome-oriented positioning.

Exit condition: a new owner can start a Room outcome and compile a Twin preview without reading documentation or handling internal identifiers.

### Phase 2: Close Human Attention Loops

- Project Room review, recommendation, failure, and blocked states into the existing Inbox.
- Add explicit notification preferences and stable Room deep links.
- Add Wiki-to-LM-Wiki revision pinning at the Wiki page.
- Add Twin/Wiki stale and pending maintenance items from a server-owned read model.

Exit condition: every state requiring human action has one durable owner-facing entry, deduplicates under replay, and clears when resolved or superseded.

### Phase 3: Create Repeat Use

- Add fresh-preflight Run again and configuration-only Duplicate Room.
- Add appearance preview, bounded Undo, reset confirmation, and user-copyable diagnostics.
- Surface recent outcome/cost state in the Room list and keep named skins in maintenance mode after its confidence work ships.

Exit condition: successful Room configuration is reusable, appearance experimentation is reversible, and neither flow requires support intervention for normal recovery.

### Phase 4: Prove Value

- Add recurring-Room value review after Inbox metrics are trustworthy.
- Add Twin effectiveness comparisons only after a minimum attributed sample and feedback coverage are available.
- Decide further investment from accepted outcomes, repeat use, instruction-related revisions, cost, and guardrails rather than raw activity.

Exit condition: product owners can decide to expand, change, or stop each initiative from bounded outcome metrics.

## Shared Engineering Rules

- Reuse existing lifecycle, Inbox, notification, query, route, analytics, and authorization contracts. New read models may compose them but may not replace their mutation authority.
- Keep human review gates explicit for Room promotion, LM Wiki acceptance, Twin sign-off, binding enablement, and deposition acceptance.
- Do not put generated content, Wiki content, prompts, raw citations, routes, paths, or credentials into analytics or system-notification payloads. Do not add identifiers beyond the existing bounded tenancy and attribution contracts.
- Use additive migrations, no foreign keys or cascades, explicit transactional cleanup, and one concurrent index per single-statement migration.
- Keep downstream features in leaf modules with narrow registrations into upstream-owned hubs.
- Each phase must include focused contract tests, one canonical live E2E extension, and screen-recorded acceptance evidence for changed UI flows.

## Decision Gates

- Do not start effectiveness dashboards until attribution and feedback coverage meet a declared minimum sample; empty percentages are not evidence.
- Do not add a fourth skin until the existing three show durable adoption and the maintenance burden remains within the guardrails.
- Do not add Room template marketplaces or arbitrary workflows until repeat-run behavior proves that built-in outcome templates retain users.
- Do not automate knowledge acceptance or Twin sign-off. If review latency is high, improve attention and comparison before weakening custody.

## Definition Of Done

- All three vNext specs have a named owner, implementation issue, and rollout flag where needed.
- Baseline and vNext specs are no longer both marked executable for the same scope.
- The activation and attention funnels can be computed from privacy-safe, typed events.
- Web/Desktop and Mobile preserve their platform interaction models while sharing lifecycle meaning.
- Documentation, release checks, and acceptance evidence describe what is shipped rather than planned behavior.
