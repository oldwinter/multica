# Spec: Loosely Coupled Self-Improving Skill Evolution Loop

Triage: `ready-for-agent`

Ownership: `downstream-leaf`

Depends on: the shipped Wiki knowledge-to-execution, Room outcome, and Twin execution-feedback lifecycles.

## Problem Statement

Multica already captures several forms of durable learning. Workspace Wiki stores revisioned and reviewable knowledge, Rooms turn asynchronous Agent work into cited outcomes, and Twin records signed working assertions plus attributed run feedback and deposition proposals. These lifecycles provide most of the ingredients for self-improvement, but workspace Skills still behave as mutable bundles rather than reviewable, attributable versions.

A useful correction can therefore remain trapped in one task, Room, or feedback note. A human may manually edit a Skill after noticing the same failure repeatedly, but Multica cannot safely collect those signals, explain a minimal candidate change, evaluate it against the current Skill, or publish it through an auditable human gate. Because the Skill used by a run is not always durably reported as an immutable execution manifest, the product also cannot make a defensible claim that a later outcome improved because of a particular Skill change.

This repository is a long-lived downstream fork that regularly merges `upstream/main`. Task dispatch, daemon claim and completion, Skill CRUD, realtime synchronization, workspace deletion, shared navigation, and generated database output are active upstream integration hubs. Implementing self-improvement by embedding downstream policy throughout those hubs would turn an otherwise local product capability into recurring merge work and make upstream lifecycle changes difficult to adopt safely.

The feature must therefore add a governed self-improvement loop while preserving upstream ownership. Most implementation must live in a removable downstream leaf. Shared code may expose or register only narrow, additive interfaces. The feature must fail closed when exact evidence, authorization, compatibility, or publication state is unavailable, without changing ordinary task execution.

## Solution

Add an opt-in `SkillEvolution` module for workspace-owned Skills. The module observes explicit, authorized signals from existing task, Wiki, Room, and Twin lifecycles; asks an improver to propose the smallest evidence-backed Skill bundle change; evaluates the candidate with deterministic gates and bounded replay; and presents the result for human review. Acceptance publishes through a narrow Skill publisher adapter and records an immutable downstream-owned release. Rejection, supersession, and rollback remain append-only decisions.

The loop uses existing product concepts instead of introducing another knowledge store:

- Wiki remains the authoritative home for durable facts and documented procedures.
- Room acts as the visible outer improver that can synthesize multiple signals and produce a cited recommendation.
- Twin remains the signed home for human or team preferences, constraints, procedures, and quality bars.
- Skill remains the executable task procedure delivered to an Agent runtime.
- Issue remains the destination for product or code defects that should not be repaired through instructions.

The improver classifies each recommendation by target. Knowledge changes use the existing Wiki proposal lifecycle, preference or constraint changes use the existing Twin deposition lifecycle, executable procedure changes create a Skill evolution proposal, and implementation defects create an Issue. No recommendation directly mutates its target.

The baseline is deliberately sidecar-based. It does not replace the upstream Skill table, Skill CRUD contract, task queue, or daemon claim algorithm. When evolution is enabled for a workspace Skill, the downstream module snapshots its current canonical bundle and hash as the base revision. Manual or upstream-originated Skill changes appear as base drift and supersede stale proposals. Publication uses the existing Skill mutation behavior through one adapter, then verifies the resulting canonical bundle hash before recording the release.

Exact effectiveness claims require an execution manifest containing the Skill source, identity, canonical bundle hash, and, when available, evolution revision. A backward-compatible daemon capability may report that manifest through one optional completion-contract field. Older daemons continue to execute tasks normally but their runs are ineligible for causal comparison or automatic signal batching. Missing evolution data must never fail task dispatch or completion.

The baseline does not require new global realtime or Inbox behavior. The evolution surface uses React Query invalidation after direct mutations and bounded polling while scheduled work is pending. Realtime and Inbox projection may be added later only through existing contributor interfaces or one-line additive registrations.

## User Stories

1. As a workspace owner, I want to enable Skill evolution for one workspace-owned Skill, so that I can try the loop without changing every Agent.
2. As a workspace owner, I want evolution disabled by default, so that ordinary Skill behavior does not change after upgrading Multica.
3. As a Skill author, I want the current canonical bundle captured as an immutable base revision, so that every proposal has a reproducible starting point.
4. As a Skill author, I want supporting files included in the canonical revision, so that an improvement cannot silently evaluate only `SKILL.md` while changing other instructions.
5. As a task reviewer, I want to leave a specific correction where I already review the result, so that useful feedback does not require a separate reporting workflow.
6. As a task reviewer, I want to identify whether feedback concerns knowledge, a Twin assertion, a Skill procedure, or a product defect, so that the correction reaches the right lifecycle.
7. As a task reviewer, I want free-text reasoning to accompany a positive or negative rating, so that the improver learns why the result was useful or wrong.
8. As a workspace member, I want task reruns and explicit revisions to be usable as bounded outcome signals, so that repeated corrective work is visible without copying task output.
9. As a Room reviewer, I want an accepted Room outcome to cite the signals it synthesized, so that a later Skill proposal has inspectable provenance.
10. As a Room reviewer, I want a Room recommendation classified before promotion, so that a Skill concern is not accidentally written into Wiki or Twin.
11. As a Wiki reviewer, I want accepted and rejected Agent proposals to remain Wiki-owned signals, so that the evolution module does not duplicate Wiki content or review state.
12. As a Twin reviewer, I want helped, irrelevant, mismatch, and detailed feedback to remain Twin-owned signals, so that the evolution module does not replace execution attribution or deposition.
13. As a Skill owner, I want the improver to aggregate several compatible signals, so that one ambiguous comment does not cause a broad instruction change.
14. As a Skill owner, I want one detailed, high-quality correction to remain eligible when policy allows it, so that signal quality matters more than raw volume.
15. As a Skill owner, I want the evidence window, minimum signal policy, schedule, and cooldown to be visible, so that proposal timing is predictable.
16. As a Skill owner, I want every proposal to explain the observed pattern, proposed change, expected benefit, and possible regression, so that review is based on reasoning rather than trust.
17. As a Skill owner, I want the candidate shown as a minimal bundle diff against the exact base hash, so that unrelated edits are easy to detect.
18. As a Skill owner, I want proposals to preserve progressive disclosure and supporting-file structure, so that self-improvement does not inflate the primary Skill instructions indefinitely.
19. As a Skill owner, I want a stale proposal rejected when the live Skill bundle has changed, so that approval never overwrites newer human or upstream work.
20. As a Skill owner, I want a stale proposal to retain its audit record, so that the reason it was superseded remains inspectable.
21. As a security reviewer, I want candidate bundles checked for secret-like content, path traversal, unsafe file names, excessive size, and forbidden capability claims, so that an improver cannot expand authority through text.
22. As a security reviewer, I want Skill changes unable to grant tools, credentials, connected apps, runtime permissions, or workspace access, so that instructions remain subordinate to platform policy.
23. As a privacy reviewer, I want proposal evidence stored as scoped references and digests rather than copied prompts and outputs, so that the new loop does not become a shadow transcript store.
24. As a privacy reviewer, I want personal Wiki, local-only evidence, credentials, raw local paths, and cross-workspace records excluded, so that existing custody rules remain authoritative.
25. As a reviewer, I want deterministic validation results separated from model judgments, so that hard failures cannot be overruled by an improver.
26. As a reviewer, I want candidate evaluation to disclose test cases, sample size, failures, cost, and latency, so that an attractive diff is not mistaken for demonstrated improvement.
27. As a reviewer, I want low-sample comparisons labeled inconclusive, so that Multica does not imply causality from a few runs.
28. As a reviewer, I want to preview or reject a proposal without publishing it, so that evaluation does not automatically affect production work.
29. As a workspace owner, I want publication to require an explicit human decision, so that scheduled Agents cannot rewrite active Skills autonomously.
30. As a workspace owner, I want the publisher to verify the resulting bundle hash, so that partial file updates or concurrent changes cannot be recorded as a successful release.
31. As a workspace owner, I want publication to preserve existing Agent-to-Skill assignments, so that improving a Skill does not change who receives it.
32. As a workspace owner, I want the previous immutable bundle retained after publication, so that I can inspect and restore it.
33. As a workspace owner, I want rollback to create a new append-only release decision, so that history is not rewritten.
34. As a workspace owner, I want an immediate pause control for future proposal generation and publication, so that unexpected behavior can be contained without deleting history.
35. As an Agent user, I want a task to continue when evolution attribution is unavailable, so that the optional learning loop cannot reduce runtime reliability.
36. As an Agent user, I want older daemons to remain compatible, so that installed clients do not require a synchronized upgrade.
37. As a product operator, I want only runs with exact execution manifests included in effectiveness comparisons, so that metrics are attributable to the bundle that actually ran.
38. As a product operator, I want proposal acceptance, publication, rollback, feedback coverage, revision rate, cost, and latency measured without content payloads, so that loop quality is observable without leaking workspace data.
39. As a workspace owner, I want imported or externally managed Skills identified clearly, so that I do not mistake an upstream refresh for a local evolution release.
40. As a workspace owner, I want an externally managed Skill to be explicitly forked into a workspace-owned Skill before evolution, so that local changes do not fight its source of truth.
41. As a repository maintainer, I want the evolution implementation concentrated in downstream-owned modules, so that upstream merges rarely touch its implementation.
42. As a repository maintainer, I want each unavoidable upstream hub change limited to an additive registration or optional field, so that conflict resolution preserves upstream lifecycle intent.
43. As a repository maintainer, I want generated outputs changed only by their generators, so that sync conflicts are resolved at their source.
44. As a repository maintainer, I want evolution schema cleanup owned behind one workspace cleanup contributor, so that adding several sidecar tables does not spread deletion logic through shared code.
45. As a repository maintainer, I want the feature removable by unregistering its adapters and deleting its leaf modules, so that downstream ownership remains real rather than nominal.
46. As a repository maintainer, I want feature-disabled behavior proven equivalent to upstream task and Skill behavior, so that the downstream loop cannot become a hidden runtime dependency.
47. As a repository maintainer, I want an upstream merge preview and targeted compatibility suite before each evolution milestone lands, so that local work is shaped around current upstream reality.
48. As a mobile user, I want tasks to remain usable when the evolution management surface is unavailable on Mobile, so that management scope does not fragment execution semantics.

## Implementation Decisions

- Build one deep downstream `SkillEvolution` module. Its external interface exposes proposal generation and human decision operations; signal collection, Room orchestration, candidate validation, evaluation, persistence, publication verification, and idempotency remain implementation details.
- Use the module interface as the primary seam for callers and tests. Do not expose separate public collect, synthesize, validate, evaluate, and persist modules that force callers to reproduce ordering rules.
- Keep the baseline scoped to workspace-owned Skills. Built-in, plugin-owned, and runtime-local Skills are not mutated. An externally sourced Skill must be explicitly forked into a workspace-owned copy before evolution is enabled.
- Preserve the existing Skill entity and CRUD contract as the runtime source of truth. Do not replace or broadly refactor upstream Skill handlers, queries, client contracts, or views in the baseline.
- Store evolution state in downstream-owned sidecar tables with a distinct naming prefix. The sidecar owns loop configuration, immutable bundle revisions, proposals, evidence references, evaluations, reviews, publication records, and rollback records.
- Snapshot a canonical bundle using the existing manifest semantics: Skill identity, source, name, description, primary content, sorted supporting-file paths, supporting-file content digests, total size, and bundle hash.
- Treat the canonical bundle hash as the concurrency and publication identity. A proposal is valid only while its base hash matches the current live workspace Skill bundle.
- Lazily create the first evolution revision when a loop is enabled. Existing Skills do not require an eager repository-wide backfill.
- Manual edits, imports, refreshes, and any upstream-owned Skill mutation remain allowed. When they change the live bundle hash, pending proposals become stale and the next evolution run snapshots the new base.
- Do not dual-write every upstream Skill mutation into evolution tables. Drift detection at the module interface keeps ownership local and avoids coupling every Skill write path to the downstream lifecycle.
- Store candidate bundles immutably and bound their total bytes, file count, path length, primary content, and supporting-file content. Candidate files use the same safe path and reserved-content rules as ordinary Skills.
- Store evidence as workspace-scoped typed references plus canonical digests. Load content through the source domain's existing authorization interface only when generating or reviewing a proposal.
- Reject proposal generation if referenced evidence was deleted, changed digest, lost authorization, crossed workspace scope, or became ineligible under custody policy.
- Define internal signal adapters for explicit task feedback, task rerun/revision outcomes, accepted Room memory, Wiki proposal reviews, and Twin run feedback/deposition. These adapters translate owned records into one bounded signal read model without moving source lifecycle ownership.
- Do not create a universal feedback table or copy every task, Room, Wiki, and Twin event into the evolution schema.
- Use a visible, template-backed Improvement Room as the outer improver for multi-signal synthesis. The Room consumes bounded signal summaries and citations, not unrestricted workspace history.
- Extend Room recommendation routing through a narrow downstream adapter. Recommendations may target Wiki proposal, Twin deposition, Skill evolution proposal, Issue, or Decision; none directly mutates the target.
- Keep Room lifecycle, budget, review, retry, synthesis validation, and artifact provenance authoritative. Skill evolution must not fork or duplicate Room orchestration.
- Introduce an internal improver port with a production model adapter and deterministic test adapter. Candidate output must pass deterministic validation before it can be persisted as reviewable.
- Ask the improver for principles and concise rationale rather than exhaustive micro-rules. Require it to prefer editing an existing instruction, moving detail into a supporting file, or deleting a misleading instruction over unbounded accumulation.
- Candidate validation rejects unsupported evidence claims, unrelated bundle changes, duplicate or conflicting instructions, secret-like values, absolute or traversing paths, reserved file collisions, oversize bundles, invalid frontmatter, and attempts to grant capabilities or override higher-authority policy.
- Evaluation has two layers: deterministic bundle/security/contract gates and bounded behavioral replay. A deterministic failure blocks review publication regardless of model judgment.
- Behavioral replay compares the current base and candidate against the same held-out cases using identical declared runtime/model settings where feasible. Results disclose nondeterminism and never label a small replay as causal proof.
- The initial rollout supports observe, propose, and paused loop states. Observe collects eligible signal counts without model generation; propose may create reviewable candidates; paused stops new scheduled work while preserving history.
- Publishing remains a human-only operation. Machine credentials and Agent actors may create bounded proposals through authorized workflows but cannot approve, publish, or roll back.
- Define one narrow `SkillPublisher` seam. A workspace adapter applies a candidate through existing Skill mutation semantics inside an authorized transaction and returns the resulting canonical bundle for hash verification.
- Publication checks the proposal base hash immediately before mutation and the candidate hash immediately afterward. Any mismatch rolls back or records an unknown publication outcome that requires inspection; it never retries blindly.
- Preserve Agent-Skill assignments, creator identity, workspace identity, and unrelated Skill configuration during publication. Only the reviewed bundle fields and explicitly reviewed safe configuration fields may change.
- A rollback republishes an earlier immutable bundle through the same publisher seam and appends a new publication record. It does not change or delete prior revisions, proposals, reviews, or releases.
- Do not change normal task dispatch to depend on Skill evolution. Evolution unavailable, disabled, slow, or failed must leave upstream task selection, claim, execution, retry, and completion behavior unchanged.
- Add exact execution attribution through one optional, versioned execution-manifest contract when a stable integration point is available. The manifest reports the actual materialized Skill bundle identities and hashes after daemon resolution.
- Gate the optional manifest by daemon capability. Older daemons omit it; the server accepts completion normally and marks the run ineligible for exact effectiveness analysis.
- Do not add evolution-specific branching to task claim selection, task promotion, retry transactions, or runtime authorization. If an execution-manifest hook requires editing those algorithms rather than registering an optional contributor, defer the hook and keep proposal generation limited to explicit evidence.
- Keep manifest recording best-effort with respect to task success but strict with respect to metric eligibility. A missing or malformed manifest excludes the run rather than guessing the active Skill.
- Add a downstream management surface in its own shared view module and platform route adapters. Avoid restructuring the upstream Skills page, dashboard shell, navigation implementation, or existing Skill editor.
- The management surface shows loop state, current base hash, pending proposal, minimal diff, signal provenance, validation, evaluation, publication history, and rollback controls.
- React Query owns all loop, proposal, evaluation, and publication server state. Zustand may hold only local filters, selected proposal, dialog state, and unfinished review input.
- The baseline uses mutation invalidation and bounded polling for scheduled proposals. Do not require a new global realtime event switch or Inbox projection to ship the first safe loop.
- If realtime is added later, publish content-free typed events and register one additive invalidation adapter. Payloads contain only workspace, Skill, loop, proposal or release identity, state, and timestamps.
- If Inbox projection is added later, use one content-free attention adapter and existing deduplication semantics. Do not add a second notification center.
- Register scheduled improvement work as one leaf-owned scheduler job. The shared server bootstrap receives only one additive registration call; scheduling rules and job implementation remain in the leaf.
- Register HTTP routes as one grouped downstream route module. Shared router code should contain only the group registration, not lifecycle logic.
- Register workspace cleanup through one contributor that explicitly deletes every evolution-owned table in dependency order inside the workspace deletion transaction. Do not add foreign keys or cascading actions.
- Give every sidecar row a workspace identity and enforce workspace filtering in every query. Relationship validation and cleanup remain explicit application behavior.
- Follow repository migration rules: no foreign keys or cascades; each index uses a concurrent build in its own single-statement migration; published migration identities are never renamed or reused.
- Choose new migration identities from the current repository maximum at implementation time. Do not reserve numeric prefixes in this specification because upstream may advance before delivery.
- Resolve sqlc sources and regenerate generated Go output. Never hand-edit generated files during implementation or an upstream merge.
- Keep additive registrations one symbol per line and isolate them from formatting or unrelated refactors. A shared hub edit and the leaf implementation should be separate commits when practical.
- Maintain a downstream ownership note listing the feature-owned module prefixes and the small allowlist of shared registration points. Any implementation change outside that inventory requires explicit justification in review.
- Before each delivery phase, preview the merge against current `upstream/main`. After implementation, compare the downstream diff to upstream and verify that upstream-owned hubs differ only by intended registrations or optional compatibility fields.
- Product metrics include eligible signal count, proposal generation, validation failure, review decision, publication, rollback, exact-manifest coverage, feedback coverage, rerun/revision rate, cost, and latency. Metrics never include Skill content, feedback notes, task prompts/outputs, Wiki bodies, Room synthesis bodies, Twin assertions, citations, credentials, or paths.
- Preserve system safety, workspace membership, runtime authorization, tool policy, custody policy, current user request, and external-effect safeguards as higher authority than any evolved Skill.

## Testing Decisions

- Treat the `SkillEvolution` interface as the canonical test seam. Tests exercise proposal generation and human decisions through the same interface used by the scheduler and handlers, asserting observable revisions, proposals, reviews, releases, and errors rather than internal stages.
- Use a real PostgreSQL test database with deterministic improver, evaluator, signal-source, and publisher adapters for the canonical lifecycle. This verifies transactions, stale-base detection, idempotency, explicit cleanup, and immutable history without invoking a real Agent CLI or external model.
- The canonical acceptance scenario enables evolution for a workspace Skill, snapshots its exact bundle, records several authorized signals across existing domains, creates and accepts an Improvement Room outcome, generates a minimal candidate, passes deterministic evaluation, previews it, publishes it after human approval, verifies the resulting hash and assignments, attributes an eligible run, and rolls back by publishing the prior bundle as a new release.
- Add an equally important rejection scenario in which the live Skill changes after proposal generation. Review and publication must fail stale, preserve the human edit, supersede the proposal, and leave ordinary Agent assignments and execution unchanged.
- Test feature-disabled equivalence at the highest practical seam: with no active loop and no manifest capability, Skill CRUD, task claim, daemon resolution, task completion, Room execution, Wiki review, and Twin attribution retain their existing behavior and response compatibility.
- Test optional daemon compatibility with capability absent, capability present, malformed manifest, unknown manifest version, incomplete bundle list, duplicate Skill identity, and reported hash mismatch. None may turn a successful task into a failed task; only eligibility changes.
- Test canonical bundle hashing against file reordering, content changes, renames, description changes, missing supporting files, reserved content paths, duplicate paths, Unicode content, empty files, and maximum-size boundaries.
- Test proposal generation against single rich feedback, repeated weak feedback, conflicting signals, cross-skill signals, cross-workspace references, deleted evidence, edited feedback digest, unauthorized evidence, personal Wiki, local-only evidence, and stale accepted Room or Twin state.
- Test recommendation routing as a closed matrix: knowledge to Wiki proposal, preference/constraint to Twin deposition, executable procedure to Skill proposal, implementation defect to Issue, and unsupported target to a reviewable refusal.
- Test deterministic validation with secret-like values, credentials, local paths, path traversal, absolute paths, invalid frontmatter, unsupported capability grants, prompt-injection instructions, excessive size, excessive files, unrelated changes, duplicated rules, and attempts to weaken higher-authority policy.
- Test evaluation reporting with zero cases, insufficient sample, mixed results, deterministic failure, model timeout, cost limit, latency limit, nondeterministic output, and evaluator disagreement. Only declared passing states may proceed to publication review.
- Test human authorization for member, owner, admin, Agent actor, machine credential, removed member, and cross-workspace caller. Only authorized humans may publish, pause, or roll back.
- Test idempotency for scheduled job retries, duplicate Room promotion, repeated proposal request, repeated review, duplicate publication callback, retry after unknown outcome, and concurrent proposals for the same base hash.
- Test publication with live base drift before transaction, drift during publication, partial supporting-file failure, publisher timeout before commit, timeout after commit, post-write hash mismatch, and duplicate request. Unknown outcomes require inspection and are never blindly retried.
- Test that publication preserves Agent-Skill assignments, creator identity, unrelated configuration, workspace identity, and external origin metadata unless the reviewed decision explicitly forks ownership.
- Test rollback after one and several releases, rollback to a stale external base, concurrent rollback and publication, and rollback publisher failure. History remains append-only in every case.
- Test scheduler behavior for disabled, observe, propose, and paused loops; minimum signal policy; cooldown; overlapping ticks; lease loss; cost budget; and a workspace deleted while a job is pending.
- Test workspace cleanup through the single cleanup contributor and verify every sidecar table is empty while unrelated workspace and user data remains untouched.
- Test privacy at response and analytics interfaces. Content-free list, readiness, event, metric, and notification shapes must make it impossible to pass prompts, outputs, notes, Wiki bodies, Room bodies, Twin assertions, raw citations, credentials, or local paths.
- Test shared clients through schemas that tolerate absent optional evolution fields and unknown future enum values. Installed clients must continue to parse upstream task and Skill responses.
- Keep shared view tests focused on diff rendering, provenance links, stale state, authorization, keyboard access, focus, loading, empty, evaluation failure, publication unknown outcome, pause, and rollback confirmation. Do not replay the service state matrix through DOM tests.
- Add one end-to-end Web/Desktop workflow after the server contract is stable. Mobile management is not required, but Mobile task execution must remain unaffected by enabled workspace loops.
- Use existing Wiki proposal, Room outcome/review, Twin deposition/attribution, Skill bundle hashing, scheduler, and daemon capability tests as prior art. Extend their public contracts only where the narrow adapter requires it; do not copy their internal test matrices into the evolution suite.
- Run an upstream merge preview before implementation and before final integration. The delivery gate includes a downstream-versus-upstream path audit, conflict-marker search, migration upgrade against both fresh and existing downstream ledgers, sqlc regeneration with no residual diff, targeted Go and TypeScript tests, and typecheck.
- Apply the deletion test in review: removing the leaf modules and their registrations should remove the feature while leaving upstream Skill, task, daemon, Wiki, Room, and Twin behavior coherent. If complexity leaks into those callers, move it back behind the module interface.

## Out of Scope

- Updating model weights, fine-tuning, reinforcement learning, or autonomous model selection.
- Automatic acceptance, publication, rollback, Twin sign-off, Wiki acceptance, Room outcome acceptance, or Issue closure.
- Direct mutation of built-in Skills, plugin Skills, runtime-local Skills, externally managed Skills, system prompts, Agent instructions, or runtime configuration.
- Automatically opening GitHub or GitLab pull requests for product-owned built-in Skills. That requires a separate repository-change adapter and code-review policy.
- Replacing upstream Skill CRUD, the Skill table, Agent-Skill assignment, daemon claim, task queue, task retry, runtime authorization, or current bundle delivery.
- Guaranteeing exact attribution for older daemons that do not report an execution manifest.
- Treating runs without an exact execution manifest as effectiveness evidence.
- Hidden dual execution of production tasks, unbounded A/B testing, or automatic canary assignment.
- Copying raw task prompts, outputs, Wiki bodies, Room transcripts, Twin assertions, or personal/local-only material into a new evolution data lake.
- A universal self-evolution framework for every Multica domain. The Skill lifecycle is the first concrete downstream module; shared abstractions may be extracted only after another real adapter demonstrates the same contract.
- A second scheduler, task system, Inbox, notification center, realtime state store, knowledge store, or vector database.
- Mobile management UI in the baseline.
- Broad refactors of upstream navigation, dashboard, Skills UI, daemon protocol, task lifecycle, realtime synchronization, or workspace deletion infrastructure.
- Resolving unrelated upstream/downstream migration identity debt or refactoring generated database output.

## Further Notes

- The design adapts Warp's base-skill, human-feedback, scheduled-improver, and reviewed-small-edit pattern to Multica's existing domain lifecycles. The important invariant is reviewable compounding knowledge, not autonomous mutation.
- Loose coupling takes priority over collecting every possible signal in the first release. Explicit, high-quality signals with exact provenance are preferable to invasive instrumentation of upstream execution paths.
- The baseline should ship in phases: sidecar revision/proposal lifecycle; explicit feedback and Improvement Room routing; deterministic evaluation and human publication; optional execution manifest; attributable effectiveness reporting.
- The optional execution-manifest phase must be deferred if the current upstream version offers no narrow compatibility seam. Proposal-only learning remains useful and safer than embedding downstream policy in task claim or completion algorithms.
- Shared registrations are an integration budget, not an invitation to reorganize the hub. Each one should preserve the complete upstream lifecycle and add only the downstream leaf entry.
- The authoritative conflict-history and ownership rules remain the repository's upstream-sync documentation. Implementation should update that ledger after the feature lands and after any sync that changes its integration points.
- Reference: [How Warp builds self-improving agents on Claude](https://claude.com/blog/how-warp-builds-self-improving-agents-on-claude).
