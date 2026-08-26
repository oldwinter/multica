# Spec: Rooms Attention And Reuse Loop

Triage: `ready-for-agent`

Depends on: [Rooms Outcome Loop](./rooms-outcome-loop.md)

## Problem Statement

Rooms now has the hard execution contract: objectives, templates, budgets, preflight, participant and synthesis phases, cited memory, human review, artifact promotion, realtime updates, Mobile gates, and value metrics. The remaining product problem is not orchestration. It is getting a user from intent to a reviewed outcome repeatedly.

Creation currently exposes most configuration at once, even when a built-in template supplies reasonable defaults. Once work becomes asynchronous, a Room can reach `awaiting_review`, fail synthesis, or stop on budget without appearing in the existing Inbox. Users must remember to revisit individual Rooms. After a successful outcome, there is no first-class way to reuse its configuration for the next recurring question.

Rooms will become valuable when they behave like accountable recurring workflows: quick to start, impossible to forget at a human gate, and easy to repeat with the same operating contract.

## Goals

- Reduce time and decisions required to create the first outcome-oriented Room.
- Route every actionable Room state to an existing user attention surface with stable deep links and deduplication.
- Let users repeat a successful Room without rebuilding participants, objective, policy, and budget from scratch.
- Measure accepted outcomes, review latency, repeat use, and cost per outcome rather than transcript volume.

## Non-Goals

- A second Inbox, notification center, task queue, or scheduler.
- Autonomous acceptance or promotion of Room output.
- A general workflow builder, arbitrary DAG, or marketplace of community templates.
- Token-streaming debate, cross-workspace Rooms, or public Room sharing.
- Automatic changes to promoted Issues, Wiki pages, or Decisions after promotion.

## User Stories

1. As a new user, I want to choose an outcome template, participants, and budget in a short guided flow, so that I can start without understanding every Room field.
2. As an advanced user, I want to review and edit the full objective, success criteria, stop conditions, instructions, schedule, and limits before creation.
3. As a Room owner, I want an Inbox item when a cycle needs review, fails, or is blocked by budget/readiness, so that asynchronous work does not disappear.
4. As a reviewer, I want a notification to open the exact outcome, failure, or recommendation that needs action, so that I do not reconstruct context.
5. As a reviewer, I want duplicate realtime and retry events to produce one current Inbox item, so that the queue stays trustworthy.
6. As a user, I want to rerun a completed Room with the same contract or clone it into a new Room, so that recurring work is cheap to repeat.
7. As a Room owner, I want to see which recurring Rooms produce accepted outcomes for their cost, so that I can pause low-value schedules.

## Product Requirements

### Template-First Creation

- Make template selection the first decision. Each built-in template shows the outcome it produces, its default success criteria, and a concise example.
- The default path asks only for name, objective refinement, facilitator/participants, and a bounded execution choice. Advanced fields remain available before submission without being required for ordinary creation.
- Changing templates updates only untouched defaults. User-edited objective, criteria, stop conditions, instructions, schedule, or budget values are never silently overwritten.
- Before creation, show one summary of who will run, whether synthesis is required, maximum turns/cost, and whether execution is manual or scheduled.
- Empty-state and navigation copy position Rooms as outcome workflows, not generic durable conversation.

### Actionable Attention

- Extend the existing Inbox with Room-specific item types for `room_outcome_review_required`, `room_recommendation_review_required`, `room_cycle_failed`, and `room_cycle_blocked`.
- Create action-required items for human gates and attention items for failures or refusals. Ordinary transcript messages and successful intermediate turns do not create Inbox noise.
- The recipient set is deterministic: Room creator plus explicit human participants for outcome and recommendation review; workspace owners/admins for policy or budget blocks.
- Each item stores bounded details: Room ID, cycle ID, memory revision or recommendation key, phase, reason code, and stable route. It never stores transcript, synthesis, or artifact bodies.
- Deduplicate by recipient, Room, cycle, action kind, and current review identity. Retries update or replace the current item; accepted, rejected, corrected, cancelled, archived, or superseded work archives it transactionally.
- Respect a new `rooms` notification preference group and the existing system-notification channel. Muting delivery does not remove the durable in-product gate.
- Opening an item navigates to the exact Room and outcome/recommendation tab. Stale deep links explain the terminal state and offer the next valid action.

### Repeat And Clone

- Add Run again for a terminal Room. It performs current preflight and starts a new cycle against the same Room configuration; it never copies an old readiness result.
- Add Duplicate Room for users who need a separate objective or participant history. The draft copies template, configuration, participants, schedule, and budget but excludes transcript, memory, cycles, reviews, artifacts, usage, and idempotency keys.
- A duplicated scheduled Room starts paused until the creator explicitly confirms the schedule, preventing accidental duplicate automation.
- Surface the last accepted outcome, last run cost, failure state, and next scheduled run in the Room list so users can choose what to revisit.

### Value Review

- Extend Room usage with accepted outcomes per active week, median review latency, repeat-run count, promotion rate, failed/refused cycles, and cost ticks per accepted outcome.
- Provide a compact owner view that ranks only the workspace's recurring or recently active Rooms. It recommends no autonomous action; owners decide whether to pause or change a Room.
- Analytics correlate lifecycle events through opaque Room and cycle IDs already permitted by the bounded contract and never include generated content.

## Technical Decisions

- Keep `RoomLifecycle` authoritative for state changes. Inbox projection is an adapter invoked from committed lifecycle transitions, not from UI observation.
- Reuse `inbox_item`, notification preferences, realtime invalidation, system notification adapters, and route handling. Add the minimum Room identity fields needed for stable non-Issue items.
- Notification creation and lifecycle mutation must share a transaction or durable outbox boundary so a committed human gate cannot be lost. Duplicate delivery remains idempotent.
- Keep React Query authoritative for Rooms and Inbox. Zustand stores only local creation drafts, filters, and selected tabs.
- Reuse current preflight, budget, permission, and capability checks for creation summaries and repeat runs.

## Acceptance Criteria

- A first-time user can create a valid template-backed Room without opening advanced settings, and can inspect all derived defaults before submission.
- User-edited creation fields survive template switching unless the user explicitly resets them.
- Every review-required, failed, or blocked terminal transition produces at most one current Inbox item per intended recipient.
- Resolving, superseding, cancelling, or archiving the referenced work removes its action-required state without deleting audit history.
- An Inbox item opens the exact Room context on Web and Desktop and the equivalent native route on Mobile.
- Run again performs fresh authorization, capability, readiness, and budget checks and preserves the single-active-cycle invariant.
- Duplicate Room copies configuration only, starts scheduled automation paused, and cannot copy transcript or accepted memory.
- No Room content appears in Inbox details, system-notification payloads, or analytics.

## Metrics

- North star: weekly workspaces with a Room that produces a second accepted or promoted outcome in a later week.
- Activation: median time from opening Rooms to first cycle start and first accepted outcome.
- Attention: median time in `awaiting_review`, percentage of actionable states opened from Inbox, and unresolved gates older than 24 hours.
- Value: accepted outcomes per 100 turns, cost ticks per accepted outcome, repeat-run rate, and promotion rate.
- Guardrails: notification fan-out, duplicate-item rate, muted-delivery leakage, cycle failure rate, unsupported-claim rate, and accidental duplicate schedules.

## Delivery Plan

1. Simplify template-first creation and align empty-state positioning.
2. Add Room Inbox contracts, durable projection, deep links, lifecycle cleanup, and preferences.
3. Add Run again and safe Duplicate Room.
4. Add compact recurring-Room value review after attention metrics are trustworthy.
