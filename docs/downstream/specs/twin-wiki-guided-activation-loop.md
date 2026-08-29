# Spec: Twin And Wiki Guided Activation Loop

Status: `split-into-executable-specs`

Depends on: [Twin And Wiki Knowledge-To-Execution Loop](./twin-wiki-knowledge-execution-loop.md)

Executable specs: [Wiki Guided Knowledge Activation](./wiki-guided-knowledge-activation-loop.md) and [Twin Guided Execution Effectiveness](./twin-guided-execution-effectiveness-loop.md)

## Problem Statement

The knowledge-to-execution baseline is now real: Workspace Wiki has revision history, search, conflict protection, stable citations, and reviewed Agent proposals; LM Wiki freezes explicit source policy and immutable evidence; signed Twin assertions can preview or influence eligible runs; tasks retain exact attribution; feedback and deposition close the learning loop.

The remaining weakness is activation. Users must understand two Wiki concepts, move between Wiki and Twin surfaces, complete several gated states, and manually enter Agent, Project, Issue, and scope UUIDs in the Twin execution panel. A six-step progress display explains lifecycle state but does not consistently identify the next safe action. Exact evidence can also become stale without a single maintenance queue explaining which source changed and what must be reviewed again.

The next iteration will preserve every custody and human-review boundary while making the safe path obvious: capture useful knowledge, pin the exact evidence, sign a bounded Twin, enable it through entity pickers, and see whether it improved real work.

## Goals

- Let a workspace owner reach the first previewed Twin briefing without knowing internal IDs or lifecycle terminology.
- Connect Workspace Wiki pages to LM Wiki policy through explicit, revision-pinned actions at the point of context.
- Turn the lifecycle spine into an actionable readiness model with one clear next safe action.
- Make stale evidence, pending review, policy exclusions, and weak run feedback visible as a maintenance queue.
- Prove Twin value through attributed comparison and feedback without weakening privacy or authorization.

## Non-Goals

- Automatic source inclusion, LM Wiki acceptance, Twin sign-off, execution enablement, or deposition acceptance.
- Personal Wiki inclusion in shared evidence or remote generation.
- Vector search, model training, public profiles, cross-workspace Twins, or a second execution system.
- Letting Twin policy grant tools, credentials, permissions, or external-effect authority.
- Hiding exact versions, citations, policy decisions, or briefing bytes in the name of simplicity.

## User Stories

1. As a workspace owner, I want one next-action card that explains the current blocker and opens the correct control, so that I can progress safely without memorizing the lifecycle.
2. As a Wiki author, I want to propose the current immutable revision for LM Wiki inclusion from the page itself, so that useful knowledge has an obvious governed path into execution.
3. As an owner, I want inclusion to show the exact revision, digest, privacy boundary, and remote-generation consequence before saving.
4. As an owner, I want to choose Agents, Projects, and Issues by name or identifier rather than paste UUIDs, so that execution policy is usable.
5. As an owner, I want a guided preview generated from a selected real Agent and optional work item, so that the preview matches a plausible run.
6. As a reviewer, I want one queue of stale sources, pending evidence, pending Twin proposals, and pending depositions, so that the signed profile remains current.
7. As a user reviewing a run, I want to see which assertions helped or mismatched and what change a deposition proposes, so that feedback can improve the next version.
8. As a workspace owner, I want to compare enabled Twin runs with preview/off runs on bounded quality and cost signals, so that continued use is an evidence-based choice.

## Product Requirements

### Actionable Readiness

- Derive a versioned readiness state from persisted server data: source policy configured, evidence revision available, evidence accepted, Twin proposal available, Twin signed, preview compiled, binding configured, attributed run completed, feedback available, and deposition pending.
- Show exactly one primary next safe action plus secondary inspection links. The action must never skip a required human review or change policy implicitly.
- Every blocked state explains the responsible role, missing capability, excluded source class, stale version, or disabled kill switch in plain language.
- The current six-step history remains available as audit context, but activation is driven by actionable state rather than a decorative progress display.

### Wiki-To-Evidence Path

- Add Use as LM Wiki evidence to eligible workspace and project Wiki revisions. Personal pages and local-only sources show why the action is unavailable.
- The confirmation shows page title/path, exact revision number, content digest, source scope, current policy version, remote-generation state, and permanent exclusions.
- Confirmation updates the explicit source policy by pinning the selected immutable revision. It does not refresh or accept LM Wiki automatically.
- When the live Wiki page changes after pinning, show `current`, `newer revision available`, or `source deleted` health without mutating the frozen selection.
- Source policy remains centrally reviewable in Twin. Contextual actions are narrow entry points into the same contract, not a second policy store.

### Human Entity Selection

- Replace raw UUID entry for Agent, Project, and Issue scopes with accessible searchable selectors using existing workspace query contracts. Display stable human names/identifiers while submitting canonical IDs.
- Workspace scope remains automatic. Agent and Project selectors use current workspace membership. Issue selection supports identifier search and clearly identifies archived or ineligible work.
- The briefing preview begins with a real Agent selector, optional Project/Issue selector, and request. Advanced tags and one-off run identity remain available only when needed.
- Existing bindings render entity names and identifiers, with a safe fallback to the raw ID when the entity was deleted or is no longer visible.
- Authorization is revalidated on submit and preview; selector visibility is not an authorization decision.

### Knowledge Maintenance Queue

- Present one bounded queue for newer pinned Wiki revisions, deleted/excluded sources, unreviewed LM Wiki revisions, unsigned or stale Twin proposals, low-confidence assertions, repeated mismatch feedback, and pending depositions.
- Each item has a typed reason, severity, owning role, exact affected version, and one valid next action. It contains safe metadata, never Wiki or briefing bodies.
- Resolving an item creates or reviews a new immutable artifact. It never mutates an accepted LM Wiki revision or signed Twin version.
- Queue events are deduplicated by workspace, item kind, source/version identity, and current lifecycle state. Terminal or superseded items leave the active queue but remain auditable.

### Effectiveness Evidence

- Report attributed runs, feedback coverage, helped/irrelevant/mismatch distribution, instruction-related revision rate, deposition acceptance, briefing token overhead, and latency by policy state.
- Compare `enabled` against eligible `preview` or `off` runs only when the cohort definition, minimum sample, and time window are visible. Do not imply causality from raw counts.
- Provide assertion-level feedback summaries only after a privacy-preserving minimum count. Never expose a user's request, output, citation body, or path in analytics.
- Offer Pause Twin execution from the effectiveness view. Pausing writes an explicit `off` binding or uses the existing kill-switch path; it never deletes attribution history.

## Technical Decisions

- Add a deep `TwinActivationService` or equivalent read model that composes existing Wiki, LM Wiki, Twin, binding, attribution, feedback, and deposition state. It returns typed readiness and maintenance items; clients do not reproduce lifecycle precedence.
- Keep `WikiKnowledge`, source policy, `TwinProposalGenerator`, `TwinBriefingCompiler`, execution attribution, and deposition as the authoritative mutation paths.
- Reuse `agentListOptions`, `projectListOptions`, Issue identifier search, existing selectors, and canonical IDs. Do not create an untyped generic entity picker.
- Store only lifecycle identity and bounded reason codes for maintenance items. Content remains fetched through its existing authorized detail endpoint.
- Preserve default-off remote generation and Twin execution, fail-closed policy parsing, immutable accepted/signed artifacts, and explicit authorization checks.
- React Query owns readiness and maintenance server state. Zustand may hold only local filter, selected tab, and unfinished form input.

## Acceptance Criteria

- An owner can move from an eligible Wiki page to a compiled Twin briefing without copying or typing a UUID.
- Use as LM Wiki evidence always pins an exact immutable revision and requires explicit confirmation; it never refreshes, accepts, signs, or enables execution.
- The primary next action is deterministic for the same persisted lifecycle state and cannot bypass owner/admin gates.
- A newer or deleted pinned Wiki source appears in the maintenance queue while the accepted LM Wiki revision and signed Twin remain immutable and auditable.
- Searchable entity selectors submit canonical IDs, retain keyboard and screen-reader behavior, and handle deleted/ineligible entities without crashing or silently changing scope.
- Effectiveness comparisons disclose cohort, window, sample size, policy state, and feedback coverage; low-sample assertion detail is suppressed.
- Pausing future Twin use leaves prior task attribution, exact briefing, citations, feedback, and deposition history readable.
- Personal content, raw Wiki content, prompts, outputs, credentials, paths, and raw citations never enter readiness, queue, or analytics payloads.

## Metrics

- North star: percentage of eligible workspaces reaching a second accepted Twin-attributed run with positive feedback within 28 days of first Wiki activation.
- Activation: time and completion rate from first Wiki page to pinned evidence, accepted LM Wiki, signed Twin, first preview, first enabled run, and first feedback.
- Maintenance: stale-source age, pending-review age, mismatch recurrence, deposition acceptance rate, and percentage of signed Twins with no unresolved high-severity item.
- Effectiveness: instruction-related revision rate and helped rate for enabled versus matched preview/off cohorts, with briefing token and latency overhead.
- Guardrails: authorization failures, source-policy exclusions, unsupported assertions, privacy violations, stale binding attempts, and kill-switch usage.

## Delivery Plan

1. Replace raw IDs with real entity selectors and add a deterministic next-action card.
2. Add contextual Wiki revision pinning through the existing source-policy mutation.
3. Add the server-owned readiness and maintenance read model with actionable queue UI.
4. Add cohort-aware effectiveness reporting only after attribution and feedback coverage meet a declared minimum sample.
