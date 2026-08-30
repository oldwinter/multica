# Spec: Twin Guided Execution Effectiveness Loop

Triage: `ready-for-agent`

Depends on: [Twin And Wiki Guided Activation Loop](./twin-wiki-guided-activation-loop.md)

## Problem Statement

Signed Twin versions can already preview or influence eligible Agent runs, and completed tasks retain exact briefing attribution, feedback, and deposition history. The remaining activation path is too technical: the execution-policy panel requires raw Agent, Project, Issue, and scope UUIDs, while the lifecycle progress display does not consistently identify the next safe action.

Even after activation, aggregate counts do not yet give owners a defensible decision about continued use. Stale versions, pending proposals, mismatch feedback, and depositions need a single maintenance path, and effectiveness comparisons must disclose cohort, sample, feedback coverage, cost, and latency instead of implying causality from raw activity.

## Goals

- Let an owner sign, preview, and explicitly bind a Twin without handling internal identifiers.
- Turn persisted lifecycle state into one deterministic next safe action without skipping review or policy gates.
- Provide a bounded Twin maintenance queue for stale versions, pending proposals/depositions, and repeated mismatch signals.
- Show whether Twin-enabled work is helping through attributable, cohort-aware evidence and an immediate pause path.

## Non-Goals

- Automatic Twin sign-off, binding enablement, deposition acceptance, or autonomous policy changes.
- Granting tools, permissions, credentials, connected apps, or external-effect authority through Twin context.
- Public profiles, cross-workspace Twins, model training, or a second task/runtime system.
- Hiding exact versions, citations, briefing bytes, exclusions, or policy decisions to simplify the UI.

## User Stories

1. As an owner, I want one next-action card that explains the current blocker and opens the correct control, so that I can progress safely.
2. As an owner, I want to choose Agents, Projects, and Issues by human name or identifier, so that binding and preview do not require UUIDs.
3. As an owner, I want a preview based on a real Agent and optional work item, so that it represents a plausible run.
4. As a reviewer, I want stale versions, pending proposals, mismatch feedback, and pending depositions in one queue, so that the signed Twin remains current.
5. As a workspace owner, I want enabled, preview, and off cohorts compared with visible sample and feedback coverage, so that I can decide whether Twin is useful.
6. As an owner, I want to pause future Twin execution without deleting history, so that a poor result has an immediate safe response.

## Product Requirements

### Actionable Activation

- Add a versioned `TwinActivationReadiness` read model derived from accepted evidence, proposals, signed versions, feature flags, bindings, attributed runs, feedback, and depositions.
- Return exactly one primary next action plus secondary inspection links. Typed blockers identify responsible role, missing capability, stale version, exclusion, or kill-switch state.
- The next action never signs, enables, accepts, or deposits automatically. Every existing human gate remains explicit.
- Retain the six-step lifecycle history for audit context, but drive activation from the actionable read model.

### Human Entity Selection

- Replace raw UUID entry with accessible searchable Agent, Project, and Issue selectors built from existing domain query contracts. Display names/identifiers while submitting canonical IDs.
- Workspace scope remains automatic. Deleted or no-longer-visible binding targets fall back to their stored ID with a clear unavailable state.
- Preview starts with a real Agent selector, optional Project/Issue selectors, and the run request. Tags and one-off run identity remain advanced inputs.
- Selection is not authorization. Preview and mutation handlers revalidate tenancy, eligibility, permission, signed-version freshness, and effective policy.
- Existing binding precedence and default-off behavior remain unchanged.

### Twin Maintenance Queue

- Surface unsigned or stale proposals, signed versions superseded by accepted evidence, repeated mismatch feedback, low-confidence assertions, and pending depositions.
- Each item carries bounded identity, reason/severity, owning role, exact affected version, and one valid next action; content is fetched only from existing authorized detail endpoints.
- Deduplicate by workspace, item kind, version/proposal/task identity, and current lifecycle state. Resolved and superseded items leave the active queue without deleting audit history.
- The queue may project high-severity owner actions into the existing Inbox through a narrow adapter. Inbox items never carry assertions, briefings, prompts, outputs, or raw citations.

### Effectiveness Evidence

- Report attributed runs, feedback coverage, helped/irrelevant/mismatch distribution, instruction-related revision rate, deposition acceptance, briefing overhead, task latency, and bounded execution cost by policy state.
- Compare `enabled` with eligible `preview` or `off` runs only when cohort definition, time window, minimum sample, and feedback coverage are visible.
- Suppress assertion-level summaries below a privacy-preserving minimum sample and never expose request, output, citation body, path, or workspace content in analytics.
- Add `Pause future Twin use` beside the effectiveness summary. It writes an explicit `off` binding or uses the existing operator kill-switch path and retains prior attribution.

## Technical Ownership

- Primary modules: Twin services/handlers/queries, `packages/core/twins`, `packages/views/twins`, task run-confirm integration, and task-transcript Twin context.
- Reuse `agentListOptions`, `projectListOptions`, Issue identifier search, and established selectors. Do not create an untyped generic entity-picker service.
- The Twin implementation consumes accepted Wiki/LM Wiki contracts and may not edit Wiki page/proposal/source-policy mutation ownership except through a separately reviewed shared type change.
- React Query owns readiness, maintenance, binding, attribution, and metric server state. Zustand may hold only local filters, selected tabs, and unfinished form inputs.
- Preserve fail-closed policy parsing, immutable versions, exact task attribution, and unknown-outcome/no-blind-retry rules.

## Acceptance Criteria

- An owner can compile a real briefing preview and save a scoped binding without copying or typing a UUID.
- The same persisted lifecycle state always produces the same primary next action, and no action bypasses an owner/admin or review gate.
- Entity selectors are keyboard/screen-reader operable, submit canonical IDs, and handle deleted or ineligible entities explicitly.
- Stale evidence/version, disabled feature, unauthorized scope, local-only input, and over-budget briefing states fail closed and remain inspectable.
- Effectiveness comparisons disclose policy state, cohort, window, sample size, and feedback coverage; low-sample detail is suppressed.
- Pausing future execution leaves historical briefing, version, citation, feedback, and deposition attribution readable.
- Readiness, maintenance, Inbox, notification, and analytics payloads contain no assertions, prompts, outputs, raw citations, credentials, or local paths.

## Metrics

- North star: percentage of eligible workspaces reaching a second accepted Twin-attributed run with positive feedback within 28 days.
- Activation: time from accepted LM Wiki evidence to signed Twin, first preview, first enabled run, and first feedback.
- Maintenance: stale-version age, pending-proposal/deposition age, mismatch recurrence, and deposition acceptance rate.
- Effectiveness: instruction-related revision and helped rates for enabled versus matched preview/off cohorts, with briefing token, latency, and cost overhead.
- Guardrails: authorization failures, unsupported assertions, stale binding attempts, privacy violations, and kill-switch usage.

## Delivery Plan

1. Replace raw IDs with typed entity selectors and extend focused UI tests.
2. Add server-owned activation readiness and deterministic next-action UI.
3. Add Twin maintenance queue with optional high-severity Inbox projection.
4. Add cohort-aware effectiveness evidence only after minimum sample and feedback coverage are defined and tested.
