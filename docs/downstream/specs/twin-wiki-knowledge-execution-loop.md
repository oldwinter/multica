# Spec: Twin And Wiki Knowledge-To-Execution Loop

Triage: `ready-for-agent`

## Problem Statement

Multica currently has two Wiki concepts and a Twin lifecycle, but they do not yet form a user-value loop. Workspace Wiki provides mutable Markdown pages for workspace, project, and personal knowledge. LM Wiki creates immutable, cited snapshots from allowlisted workspace records. Twin turns an accepted LM Wiki revision into a signed artifact. These technical lifecycles are durable and reviewable, yet the accepted Twin does not affect ordinary Agent execution, mutable Wiki pages do not participate in LM Wiki, and the current Twin builder largely restates work inventory rather than encoding how the user or team wants work performed.

The result is high review effort with little observable benefit. A user can refresh evidence, approve a Wiki revision, build and sign a Twin, then receive the same Agent behavior as before. Workspace Wiki is useful as a basic Markdown store, but lacks search, history, conflict protection, trustworthy Agent contribution, and an explicit path into reviewed execution context.

## Solution

Create a trusted knowledge-to-execution loop while preserving the existing custody and authorization ADRs:

1. Humans and Agents capture durable knowledge in Workspace Wiki, with version history, search, citations, and reviewable Agent edit proposals.
2. Owners and admins define an explicit LM Wiki source policy and create immutable, cited revisions from eligible workspace evidence, including selected Workspace Wiki revisions.
3. A Twin proposal derives bounded preference, constraint, procedure, and quality-bar assertions from accepted evidence; every assertion is cited and requires sign-off.
4. A signed Twin version may be explicitly bound to eligible Agent work. Before execution, the user can preview the exact bounded briefing that will be sent.
5. The task records which Twin version and assertions were used. Completed work can propose deposition changes backed by execution evidence, but only human acceptance creates a new signed version.

This makes Twin value observable: signed knowledge changes a bounded task briefing, the run records its provenance, the user reviews the result, and execution evidence can improve the next signed version.

## User Stories

1. As a workspace member, I want to create and edit Markdown knowledge at workspace scope, so that durable team knowledge lives beside the work it supports.
2. As a project member, I want project-scoped pages, so that project conventions do not pollute the whole workspace.
3. As an individual user, I want a private cross-workspace personal library, so that my notes follow me without becoming team evidence.
4. As a Wiki author, I want full-text search by title, path, and content, so that stored knowledge is retrievable.
5. As a Wiki reader, I want results grouped by workspace, project, and personal scope, so that provenance is immediately clear.
6. As a Wiki reader, I want links to Issues, Projects, Rooms, and other Wiki pages to resolve consistently, so that knowledge connects to operational work.
7. As a Wiki author, I want every save to create a recoverable revision, so that accidental edits are reversible.
8. As a Wiki author, I want to compare two revisions, so that I can understand how knowledge changed.
9. As a Wiki author, I want stale edits to be rejected with a clear merge-or-reload choice, so that concurrent changes are not silently overwritten.
10. As a Wiki author, I want to restore an older revision as a new revision, so that history remains append-only.
11. As a workspace member, I want to see who created each Wiki revision and whether it came from a human, Room promotion, or Agent proposal, so that authorship is trustworthy.
12. As an Agent, I want typed list, get, and search operations for eligible Wiki pages, so that I can retrieve knowledge without scraping the UI.
13. As an Agent, I want to cite the exact Wiki page revision used in a result, so that users can verify my context.
14. As a workspace member, I want Agent-authored Wiki changes to arrive as reviewable proposals, so that an Agent cannot silently rewrite shared knowledge.
15. As a Wiki editor, I want to preview, edit, accept, or reject an Agent proposal, so that human intent remains authoritative.
16. As a workspace owner, I want personal pages excluded from LM Wiki by default, so that private notes never become shared evidence accidentally.
17. As a workspace owner, I want to select which workspace and project Wiki pages are eligible LM Wiki sources, so that evidence collection is explicit.
18. As a workspace owner, I want local-only and egress-restricted sources to be visibly excluded from remote generation, so that source policy is enforceable.
19. As a workspace owner, I want LM Wiki refresh to snapshot exact source revisions and digests, so that later mutable edits cannot change reviewed evidence.
20. As a reviewer, I want every LM Wiki item and diff to link to its immutable source citation, so that acceptance is evidence-based.
21. As a reviewer, I want stale LM Wiki reviews rejected, so that I cannot approve an obsolete evidence set.
22. As a reviewer, I want Twin assertions classified as preference, constraint, procedure, or quality bar, so that the artifact describes how work should be performed rather than merely listing work.
23. As a reviewer, I want every Twin assertion to cite accepted evidence, so that unsupported persona or identity claims cannot enter a signed version.
24. As a reviewer, I want to compare a proposed Twin with the current signed version, so that additions, removals, and changed meaning are visible.
25. As a reviewer, I want to edit or reject an unsupported assertion before sign-off, so that the Twin remains accurate.
26. As a workspace owner, I want initial Twin use disabled until explicitly enabled, so that signed artifacts do not silently change Agent behavior.
27. As a workspace owner, I want to enable a signed Twin by Agent, Issue, Project, or one-off run, so that deployment can be gradual and contextual.
28. As a task initiator, I want to see which Twin version will be used before execution, so that context is predictable.
29. As a task initiator, I want to preview the exact bounded Twin briefing, so that I know what leaves storage and reaches the Agent.
30. As a task initiator, I want to disable or override Twin use for one run without mutating the signed Twin, so that exceptional work remains possible.
31. As an Agent, I want a concise task-relevant briefing instead of the entire Twin, so that context remains useful and token-bounded.
32. As an Agent, I want Twin assertions to be subordinate to system safety, workspace permissions, and the current user request, so that the Twin cannot override higher-authority instructions.
33. As a result reviewer, I want to see the Twin version, briefing digest, and assertion citations used by the run, so that behavior is auditable.
34. As a result reviewer, I want to report that the Twin helped, was irrelevant, or caused a mismatch with one action, so that value can be measured without a survey.
35. As a result reviewer, I want to see whether a task required revision after using Twin context, so that quality improvement is observable.
36. As a Twin owner, I want completed execution to propose small deposition changes backed by run evidence, so that the Twin can evolve from real work.
37. As a Twin owner, I want deposition proposals isolated from the current signed version until review, so that execution cannot self-modify trusted context.
38. As a Twin owner, I want to accept, edit, or reject a deposition proposal, so that learning remains a human decision.
39. As a Twin owner, I want old signed versions to remain immutable and inspectable, so that past runs remain reproducible.
40. As a workspace member, I want read-only access to accepted evidence and signed versions while only authorized owners/admins manage policy and sign-off, so that governance is clear.
41. As a user leaving a workspace, I want personal knowledge preserved while workspace evidence, Twin bindings, and shared artifacts follow explicit cleanup policy, so that tenancy remains correct.
42. As a security reviewer, I want raw local paths, credentials, private profile names, and unapproved source bytes excluded from shared records and briefings, so that Twin preserves the existing custody contract.
43. As a product operator, I want to compare eligible work with and without Twin context, so that the feature survives only if it improves accepted outcomes.

## Implementation Decisions

- Preserve three distinct domain concepts: Workspace Wiki is mutable authored knowledge; LM Wiki is an immutable cited evidence snapshot; Twin is an immutable signed set of evidence-backed working assertions. Do not collapse them into one table or one ambiguous UI object.
- Preserve the accepted Twin ADRs: PostgreSQL is the shared lifecycle ledger; the immutable owner daemon retains raw local materials; shared records never expose paths, basenames, local profile names, credentials, or unapproved bytes; existing workspace membership, Agent, Issue, runtime, and task concepts remain authoritative.
- Deepen Workspace Wiki behind a `WikiKnowledge` interface for page lifecycle, revision history, search, links, and edit proposals. Web, Desktop, Agent tools, Room promotion, and future adapters call the same interface.
- Give personal Wiki a user-rooted API and discoverable personal-library route that do not require a current workspace membership. These routes accept only human user authentication, force personal scope, and hide shared/project identifiers.
- Add append-only Wiki page revisions with revision number, content digest, actor provenance, source kind, and timestamp. Page updates require the expected current revision and return a structured conflict when stale. Restoring history creates a new revision.
- Add PostgreSQL full-text search over eligible title, path, and Markdown content. Search is workspace-tenanted and permission-filtered before ranking. Personal results are visible only to their owner. Every new search index is built concurrently in its own migration.
- Add stable Wiki links for page revisions and operational references. Backlinks may be derived asynchronously, but link resolution must never bypass scope or membership checks.
- Deleting a live Wiki page does not delete its immutable revision records. Exact revision citations remain authorization-checked and inspectable until explicit tenant/user cleanup removes their owning data.
- Replace direct Agent mutation of shared Wiki pages with Agent edit proposals. Human-authored edits remain direct. A human-confirmed Room promotion may create a page directly because the confirmation is the review gate.
- Agent Wiki proposals contain a base revision, proposed Markdown, diff, rationale, cited run/Room evidence, and idempotency key. Acceptance rechecks the base revision and produces one new page revision transactionally.
- Define an explicit LM Wiki source policy owned by workspace owners/admins. Eligible source classes include allowlisted Issues, Projects, repository resources, completed Autopilot runs, and specifically selected workspace/project Wiki page revisions.
- Personal Wiki pages are never eligible LM Wiki sources in this spec. Local-only evidence follows the owner-daemon policy and cannot be copied to shared storage or a remote generator.
- Make remote generation an explicit, default-off source-policy decision. Every policy response exposes permanent exclusions plus a monotonic policy version and canonical policy digest; every LM Wiki revision freezes that version, digest, and decision in both revision columns and canonical schema v2 content. Downstream providers fail closed when the decision is disabled, missing, malformed, or inconsistent.
- LM Wiki refresh snapshots exact source versions, safe metadata, canonical content, citations, and digests. A later source edit creates a new candidate revision; it never mutates an existing revision.
- Commit LM Wiki acceptance independently from Twin generation. Acceptance persists the human review first; an explicit idempotent Twin proposal request consumes the accepted immutable revision afterward. Model or provider failure cannot roll back accepted evidence and can be retried without duplicating proposals.
- Reclassify the existing deterministic inventory projection as Workspace evidence presentation, not the final Twin generator.
- Introduce a `TwinProposalGenerator` seam with at least a production model adapter and deterministic test adapter. The production adapter generates candidate working assertions from egress-eligible accepted evidence; a deterministic validator canonicalizes output, verifies citations, enforces allowed assertion types, redacts forbidden fields, applies size limits, and rejects unsupported claims.
- Twin assertions use the types `preference`, `constraint`, `procedure`, and `quality_bar`. An assertion includes stable ID, concise text, applicability, evidence citations, confidence, and provenance. It must not claim a personality or identity unsupported by evidence.
- Sign-off remains the only operation that creates a current Twin version. Signed versions are immutable and use freshness checks against the latest accepted evidence and current base version.
- Introduce a deep `TwinBriefingCompiler` interface that accepts an eligible task, signed Twin version, and effective source/use policy, then returns a bounded briefing plus version ID, content digest, selected assertion IDs, citation IDs, policy decision, and exclusion reasons.
- The compiler selects only task-relevant signed assertions, never mutable proposals or raw evidence. It applies a fixed token/byte budget and deterministic ordering so the same inputs produce the same briefing digest.
- Add explicit Twin-use policy with `off`, `preview`, and `enabled` states plus scoped bindings for workspace default, Agent, Project, Issue, and one-off run. The most specific explicit setting wins; every decision is inspectable.
- Inject the compiled briefing through the existing task/runtime pipeline. It is subordinate to system safety, runtime policy, workspace permissions, and the current user request. It cannot grant tools, permissions, connected apps, or external effects.
- Persist Twin execution attribution on the task record or a task-linked append-only record: signed version ID, briefing digest, selected assertion IDs, citations, policy scope, and compiler version. Do not add foreign keys.
- Show the exact briefing preview before the user confirms an eligible run. For background automation, owners approve the binding policy once and can inspect the compiled briefing after dispatch.
- Add an optional, low-friction result review of `helped`, `irrelevant`, or `mismatch`, plus existing revision/acceptance signals. Do not block ordinary task completion on feedback.
- Deposition is an append-only proposal derived from completed execution evidence. It can add, change, or remove assertions, but cannot mutate the signed version. Acceptance creates the next signed version through the existing review lifecycle.
- Preserve unknown-outcome and no-blind-retry rules for external effects. Twin context changes instructions only; it does not create a second effect or credential path.
- Emit typed realtime events for Wiki revisions/proposals, source-policy changes, LM Wiki revisions/reviews, Twin proposals/versions/bindings, and deposition reviews. React Query remains the owner of server state.
- Keep Twin management and evidence review on Web/Desktop in accordance with the existing ADR. Mobile Twin management and runtime custody are not added by this spec.
- Add privacy-safe analytics for Wiki search success, Wiki proposal acceptance, LM Wiki acceptance, Twin sign-off, briefing compilation/use, run feedback, task revision, and deposition acceptance. Never capture page content, assertions, prompts, paths, credentials, or raw citations in analytics.
- Roll out in stages: Wiki trust primitives; explicit LM Wiki source policy; Twin assertion generation and review; preview-only briefing; opt-in execution; deposition; controlled product experiment.
- Keep downstream modules in local leaf ownership and register only narrow path, permission, task-context, realtime, workspace-delete, CLI-skill, and navigation entries into upstream-owned hubs.
- Every schema change follows repository migration rules: no foreign keys or cascading actions; explicit transactional cleanup; each concurrent index in its own single-statement migration.

## Testing Decisions

- The canonical acceptance seam is one live end-to-end knowledge-to-execution scenario with real server/database and deterministic fake model/runtime adapters: create and revise a Wiki page, reject a stale concurrent edit, search it, accept an Agent proposal, include the exact revision in LM Wiki, approve the evidence, generate and sign a cited Twin, preview and run a compiled briefing, verify task attribution, review the result, and accept a deposition into the next signed version.
- Extend the existing live LM Wiki/Twin E2E rather than creating overlapping lifecycle suites. Existing stale-review, read-only member, evolution, persistence, responsive, and cleanup coverage remains prior art.
- Treat the full E2E as the product truth. Lower tests cover only contracts and failure matrices that are impractical to isolate through the browser.
- Test Wiki page and proposal behavior at the handler/domain interface: scope ACLs, revision conflicts, history restore, unique paths, search tenancy, personal privacy, Room provenance, and idempotency.
- Test source-policy canonicalization and LM Wiki snapshot building as node/pure Go matrices: allowed source classes, explicit Wiki revision selection, deleted sources, stale revisions, local-only exclusion, credential redaction, deterministic order, size bounds, and citation integrity.
- Test `TwinProposalGenerator` with a deterministic adapter and adversarial candidate outputs: missing citations, invented evidence, forbidden assertion types, identity claims, credentials, raw paths, oversized output, duplicate IDs, stale base, and nondeterministic ordering.
- Test `TwinBriefingCompiler` as a pure contract for policy precedence, task relevance, deterministic digest, token budget, exclusion reasons, version pinning, no mutable proposal use, and instruction precedence.
- Test runtime integration through task claim and prompt assembly, asserting that the signed briefing and attribution are present for eligible tasks and byte-absent for ineligible, disabled, stale, unauthorized, or local-only cases.
- Add cross-workspace and cross-profile security tests that fail before body decode or content loading where required by the accepted authorization contract.
- Test deposition for one-use review, stale evidence, duplicate callbacks, concurrent sign-off, rejection, immutable old versions, and workspace/user departure cleanup.
- Test shared views for search, revision history, conflict resolution, source policy, exact briefing preview, review dialogs, keyboard access, focus, loading, empty, stale, offline, and structured error states.
- Record screen-based acceptance evidence across representative desktop and narrow widths in light and dark appearances, including long CJK text and dense citations.
- Test analytics at its interface and assert that sensitive content fields are impossible to pass. Product metrics tests validate attribution and event completeness, not PostHog implementation details.
- Run Twin contract/provenance checkers, migration lint, focused Go and TypeScript tests, package typechecks, canonical E2E, and deletion/retention verification before completion.

## Out of Scope

- Training or fine-tuning a model, cloning a person's identity, or making an unreviewed persona claim.
- Automatically signing a Twin, accepting a deposition, or updating shared Wiki knowledge without a human gate.
- Vector embeddings or a second general-purpose RAG stack; PostgreSQL full-text search is sufficient for this spec.
- Including personal Wiki pages in LM Wiki or sending local-only evidence to shared storage or remote generation.
- Reproducing the legacy Harness runtime, Brain, installer, SQLite topology, or compatibility interface.
- Replacing the existing Agent task queue, runtime authorization, effect proxy, workspace tenancy, or global search.
- Mobile Twin management, mobile local-artifact custody, or mobile sign-off.
- Letting Twin assertions grant permissions, tools, connected apps, credentials, or external-effect authority.
- Automatically writing generated summaries into Issue descriptions or comments.
- Cross-workspace shared Twins or public Twin profiles.

## Further Notes

- North-star metric: percentage of eligible Agent runs using an explicitly enabled signed Twin that are accepted without instruction-related revision, compared with a matched preview/off baseline.
- Wiki activation metric: workspaces where a page is created, successfully retrieved by search or an Agent, cited in operational work, and revisited within 28 days.
- Guardrails: briefing token overhead, mismatch feedback rate, stale-review conflicts, unsupported-assertion rejection rate, source-policy exclusion rate, Wiki proposal rejection rate, task latency, and privacy/security violations.
- Product naming must stay literal. Until opt-in execution ships, the surface should describe itself as evidence review and Twin building rather than claiming that the Twin already guides Agents.
- The product experiment must have a kill switch and retain version attribution so disabling Twin never makes past runs unauditable.
- Delivery order is intentionally dependency-driven: trustworthy Wiki revisions precede LM Wiki source expansion; accepted evidence precedes assertion generation; signed assertions precede preview; preview precedes runtime enablement; attributed runs precede deposition.
