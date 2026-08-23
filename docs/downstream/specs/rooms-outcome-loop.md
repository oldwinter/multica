# Spec: Rooms Outcome Loop

Triage: `ready-for-agent`

## Problem Statement

Rooms already lets people and Agents exchange durable messages, schedule cycles, limit daily turns, and promote completed output into an Issue, Wiki page, or Decision. The technical orchestration is real, but the user still has to infer whether a Room reached a useful conclusion. Participant turns in one cycle do not form a deliberate synthesis, shared memory mostly mirrors the latest response, and the product does not make quality, cost, provenance, or the next human decision obvious.

For a small technical team, this creates a trust gap. A Room can consume several Agent turns and produce a long transcript without yielding a decision, a verified fact, or executable work. Users cannot quickly distinguish discussion from outcome, cannot see which evidence supports the memory, and cannot measure whether recurring Rooms are worth their runtime cost.

## Solution

Turn Rooms into outcome-producing, asynchronous collaboration spaces. Each cycle will have an explicit objective, participant phase, facilitator synthesis phase, reviewable structured memory, and a visible completion outcome. The Room will retain the existing task queue and Agent runtime model, but place the orchestration behind one deep lifecycle module so callers do not need to coordinate turns, retries, synthesis, memory, artifacts, budgets, and realtime events themselves.

The primary user experience will answer four questions at all times: what the Room is trying to achieve, what is happening now, what changed because of the last cycle, and what the user can do next. Completed cycles will produce cited facts, decisions, open questions, action items, and optional artifact recommendations. Human approval remains required before a recommendation becomes an Issue, Wiki page, or durable Decision.

## User Stories

1. As a workspace member, I want to create a Room around a concrete objective, so that Agents collaborate toward an outcome rather than an open-ended conversation.
2. As a Room creator, I want to define success criteria and stop conditions, so that the Room knows when enough work has been done.
3. As a Room creator, I want to start from a research, planning, risk review, incident review, or decision template, so that I do not have to invent effective instructions.
4. As a Room creator, I want to select a facilitator Agent or Squad, so that one accountable participant owns synthesis.
5. As a Room creator, I want to add human and Agent participants with visible roles, so that responsibility is legible.
6. As a workspace member, I want to mention a subset of Agent participants, so that a focused question does not wake every Agent.
7. As a workspace member, I want to start a full cycle explicitly, so that all eligible participants can contribute to a planned review.
8. As a workspace member, I want scheduled cycles to show their next run and expected scope, so that recurring work is predictable.
9. As a workspace owner, I want turn and spend limits before execution, so that a Room cannot consume unbounded runtime quota.
10. As a workspace owner, I want to see a preflight estimate of participating Agents and maximum turns, so that I can understand the cost before starting a cycle.
11. As a participant, I want each cycle to show whether it is gathering, synthesizing, awaiting review, completed, refused, failed, or cancelled, so that I know what is happening.
12. As a participant, I want Agent contributions to retain author, cycle, turn, and timestamp attribution, so that conclusions remain auditable.
13. As a participant Agent, I want the Room objective, instructions, current memory, and bounded recent transcript in my task briefing, so that I can contribute without reconstructing the Room.
14. As a facilitator Agent, I want to receive the completed participant contributions in a dedicated synthesis turn, so that I can resolve overlap and disagreement.
15. As a workspace member, I want the facilitator to distinguish facts, decisions, open questions, disagreements, and action items, so that the result is scannable.
16. As a workspace member, I want every synthesized item to cite one or more Room entries, so that I can inspect the supporting discussion.
17. As a workspace member, I want disagreements to remain visible instead of being silently collapsed, so that the Room does not manufacture consensus.
18. As a workspace member, I want uncertain or unsupported claims to be marked as such, so that I do not treat speculation as fact.
19. As a workspace member, I want shared memory to evolve by reviewed revisions, so that I can understand what changed after each cycle.
20. As a workspace member, I want to compare the latest memory with the previous revision, so that material changes are easy to review.
21. As a workspace member, I want to accept a synthesis, request another synthesis, or add a correction, so that the human remains the decision maker.
22. As a workspace member, I want a failed synthesis to preserve participant results and offer a targeted retry, so that useful work is not lost.
23. As a workspace member, I want a cancelled cycle to remain auditable without updating accepted memory, so that cancellation has no hidden side effects.
24. As a workspace member, I want a completed result to recommend an Issue, Wiki page, or Decision only when there is a concrete outcome, so that promotion is relevant.
25. As a workspace member, I want to preview and edit a recommended artifact before promotion, so that generated text does not enter the workspace unchecked.
26. As a workspace member, I want promotion to retain a link to its source cycle and cited entries, so that downstream work preserves provenance.
27. As a workspace member, I want repeat promotion requests to be idempotent, so that retries cannot create duplicates.
28. As a workspace member, I want an archived Room to remain searchable and readable, so that its decisions remain useful.
29. As a workspace member, I want a paused Room to save messages without running Agents and explain that state immediately, so that I can capture context safely.
30. As a workspace member, I want stale or offline Agents to be identified before a cycle starts, so that a cycle does not fail mysteriously.
31. As an Agent owner, I want existing invocation permissions to apply inside Rooms, so that Rooms cannot bypass Agent visibility or ownership rules.
32. As a workspace owner, I want a concise Room usage summary showing turns, estimated spend, failures, accepted syntheses, and promoted artifacts, so that I can judge value.
33. As a mobile user, I want to read Room outcomes, reply, pause, resume, and approve or reject promotion, so that human gates do not require a desktop.
34. As a keyboard user, I want to create, navigate, run, review, and promote a Room without a pointer, so that the complete workflow is accessible.
35. As a screen-reader user, I want cycle state changes and new outcomes announced without losing focus, so that realtime updates remain understandable.
36. As a user with reduced motion enabled, I want new entries and state transitions to update without decorative motion, so that the Room remains comfortable to use.
37. As a product operator, I want to know whether Rooms produce accepted or promoted outcomes rather than only count messages, so that adoption reflects real value.

## Implementation Decisions

- Preserve the existing Room, Room participant, Room entry, Room cycle, Room turn, Room artifact, Agent task queue, membership, permission, and realtime concepts. Do not introduce a parallel task or Agent execution system.
- Introduce one deep `RoomLifecycle` module as the external seam for create, post, wake, task lifecycle synchronization, synthesis, review, promotion, pause, resume, archive, cancellation, and scheduled dispatch. HTTP handlers, scheduler jobs, task listeners, and realtime adapters call this interface instead of coordinating state transitions themselves.
- Model a cycle as explicit phases: `gathering`, `synthesizing`, `awaiting_review`, and terminal `completed`, `failed`, `cancelled`, or `refused`. Persist the phase separately from the existing public status where an additive migration avoids breaking current clients.
- Add a `turn_kind` distinction for participant and facilitator-synthesis turns. Participant turns may run in parallel. The facilitator synthesis turn starts only after all eligible participant turns are terminal and receives their cited outputs.
- A direct message to one Agent remains a lightweight cycle. A full facilitator synthesis is mandatory for multi-Agent and scheduled cycles, and optional for a one-Agent direct cycle when the output already satisfies the structured result contract.
- Define a versioned Room synthesis contract containing a human-readable summary plus facts, decisions, open questions, disagreements, action items, artifact recommendations, confidence, and cited entry IDs. Validate size, citation ownership, and schema before accepting it.
- Keep the Agent-facing output resilient: malformed structured output is stored as an ordinary contribution, the cycle becomes `awaiting_review` with a visible synthesis error, and the user can retry synthesis without rerunning participant turns.
- Replace mutable pseudo-memory with append-only Room memory revisions. Each revision records the source cycle, synthesis turn, schema version, cited entries, creator, timestamp, and digest. The Room retains a pointer/version number for the currently accepted revision without adding database foreign keys.
- Accepted Room memory is the only memory included in future Agent briefings. Pending or rejected synthesis never silently changes subsequent context.
- Preserve transcript history separately from memory. Briefings use a bounded recent transcript plus accepted structured memory; they never treat truncated transcript text as durable memory.
- Add explicit success criteria, stop conditions, template identity, and budget policy to Room configuration. Existing Rooms migrate to neutral defaults and continue working.
- Extend budget controls from daily turn count to an optional spend ceiling using the repository's existing integer cost-tick representation. Budget checks happen before task enqueue and are rechecked when scheduled work starts.
- Add a preflight result that reports target Agents, readiness, invocation permission, expected maximum turns, synthesis requirement, and applicable budget. The UI uses the same result for manual and scheduled runs.
- Preserve the single-active-cycle invariant and existing idempotency keys. Every phase transition, retry, task completion, memory acceptance, and artifact promotion must be safe under duplicate delivery and concurrent clients.
- Artifact recommendations are not artifacts. A human preview/edit/confirm action continues to use the existing promotion lifecycle to create an Issue, Wiki page, or Decision.
- Promoted targets store source Room, cycle, synthesis revision, and citation metadata. Dependent cleanup remains explicit in application transactions; no foreign keys or cascading actions are added.
- Emit typed realtime payloads for Room, cycle, turn, memory revision, review, and artifact changes. React Query owns all server state; client stores retain only drafts and view preferences.
- Add Mobile as a native adapter for outcome review and human gates. It may use a more compact presentation but must preserve the same lifecycle semantics and permissions.
- Instrument privacy-safe product events for Room creation, first completed cycle, synthesis accepted/rejected/retried, artifact promoted, budget refusal, and cycle failure. Do not capture transcript or memory content in analytics.
- Roll out the new lifecycle behind a workspace capability. Existing Rooms are readable throughout migration, and activation occurs only after required data migrations and daemon capability checks are satisfied.
- Every new index uses a standalone concurrent-index migration. Relationships and cleanup are enforced explicitly without foreign keys or cascading actions.
- Keep Rooms in local leaf modules with narrow registration points into task claiming, scheduler dispatch, realtime event routing, workspace deletion, navigation, and analytics to reduce future upstream merge conflicts.

## Testing Decisions

- The canonical acceptance seam is one live browser-to-database E2E scenario using a real server, real Room HTTP/realtime flow, and deterministic fake Agent runtime: create a multi-Agent Room, run participant turns, run facilitator synthesis, review cited memory, retry a failed synthesis without rerunning participants, promote an edited artifact, and verify provenance and idempotency.
- Extend the existing durable Room E2E rather than creating a second overlapping product-flow suite. The existing paused-message, draft recovery, task synchronization, promotion, responsive screenshot, and cleanup behavior remains prior art.
- Test the `RoomLifecycle` interface through externally visible state transitions and returned results. Tests should not assert internal query order or helper calls.
- Keep one node-environment matrix beside the pure lifecycle reducer for phase transitions, duplicate delivery, partial participant failure, cancellation, stale review, budget refusal, and retry. Do not repeat that matrix through DOM mounts.
- Test synthesis validation as a pure contract: valid citations, missing citations, cross-Room citations, oversized payloads, malformed output, unsupported schema versions, and deterministic digest generation.
- Test database-backed behavior for concurrent wake, duplicate task completion, memory revision acceptance, artifact promotion, workspace deletion, and scheduler replay using existing database fixtures and handler helpers.
- Test authorization at the handler seam for workspace membership, Agent invocation rights, private Agents, archived Rooms, cross-workspace IDs, and mobile clients.
- Test the shared Web/Desktop views for keyboard navigation, focus retention, live-region announcements, loading, empty, refused, partial failure, retry, review, and promotion states.
- Test Mobile through its native navigation and mutation adapters for read, reply, pause/resume, review, and promotion approval; do not duplicate the server lifecycle matrix.
- Record screen-based acceptance evidence for desktop, narrow desktop, and mobile widths in light and dark appearances, including long CJK content, disagreement, budget refusal, and failed synthesis states.
- Verify analytics only at the event interface: correct event name and bounded metadata, with an explicit assertion that transcript and memory bodies are absent.
- Run package typechecks, focused TypeScript and Go tests, the canonical E2E, migration lint, and conflict-marker checks before marking the spec complete.

## Out of Scope

- Replacing Issues, Chat, Squads, Autopilots, or the existing Agent task queue.
- Synchronous token-by-token multi-Agent conversation or Agents observing one another's in-progress streams.
- Audio/video meetings, voice transcription, collaborative whiteboards, and arbitrary file delivery inside the Room transcript.
- Autonomous promotion of Agent output without a human confirmation.
- Cross-workspace Rooms or public Room sharing.
- Unbounded transcript ingestion, vector memory, or a second general-purpose RAG system.
- Automatic consensus claims when participants disagree.
- New named participant roles beyond facilitator, participant, and observer unless validated by later research.

## Further Notes

- North-star metric: weekly active workspaces with at least one Room cycle whose synthesis is accepted or whose output is promoted into a durable artifact.
- Guardrail metrics: median cost per accepted outcome, synthesis retry rate, unsupported-claim rate, cycle failure rate, promotion reversal rate, and percentage of scheduled cycles stopped by budget.
- Activation funnel: Room created -> first message or manual wake -> first terminal participant set -> first synthesis reviewed -> first accepted synthesis -> first promoted artifact -> second active week.
- Initial rollout should target recurring research, planning, risk review, and incident review. Generic always-on chat is not the positioning.
- Delivery should be staged: lifecycle and structured memory first; review/promotion experience second; metrics and workspace rollout third; Mobile human gates after the shared contract is stable.
