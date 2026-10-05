# Spec: Wiki Guided Knowledge Activation Loop

Triage: `ready-for-agent`

Depends on: [Twin And Wiki Guided Activation Loop](./twin-wiki-guided-activation-loop.md)

## Problem Statement

Workspace Wiki already provides revisioned Markdown, stable citations, search, conflict recovery, personal isolation, and reviewed Agent proposals. LM Wiki can pin exact eligible revisions under an explicit source policy. The remaining gap is the path between those systems: users discover useful knowledge in Wiki, but must leave that context and reconstruct the source-policy decision inside the Twin surface.

This makes governed knowledge activation feel like infrastructure administration. It also hides source freshness: a pinned revision remains correctly immutable, but reviewers do not get one clear view of newer revisions, deleted sources, permanent exclusions, or evidence awaiting review.

## Goals

- Make the governed path from a Wiki revision to LM Wiki policy discoverable at the page where the knowledge is reviewed.
- Preserve exact revision pinning, explicit human confirmation, default-off remote generation, and permanent personal/local-only exclusions.
- Provide one server-owned readiness and source-health view instead of reproducing policy precedence in clients.
- Let reviewers resolve stale or pending knowledge through one valid next action without mutating accepted evidence.

## Non-Goals

- Automatically refreshing or accepting LM Wiki, signing a Twin, or enabling execution.
- Including personal Wiki or local-only material in shared evidence.
- Vector search, embeddings, implicit mutable-page ingestion, or a second knowledge store.
- Replacing the existing source-policy mutation, Wiki authorization, revision, or proposal lifecycles.

## User Stories

1. As a Wiki author, I want to propose the current immutable revision for LM Wiki inclusion from the page itself, so that useful knowledge has an obvious governed path.
2. As an owner, I want confirmation to show exact revision, digest, scope, policy version, and egress consequences, so that inclusion remains an informed decision.
3. As a member without policy permission, I want to see why inclusion is unavailable and who can act, so that the gate is understandable.
4. As a reviewer, I want pinned sources labeled current, newer revision available, deleted, or excluded, so that evidence freshness is visible.
5. As a reviewer, I want one queue of source-policy and LM Wiki review work, so that stale knowledge does not silently remain the execution baseline.

## Product Requirements

### Contextual Revision Pinning

- Add `Use as LM Wiki evidence` to eligible workspace and project Wiki revision views. The action targets the immutable revision currently being inspected, not an implicitly changing live page.
- Personal and local-only sources render a disabled explanation and never send a policy mutation request.
- Confirmation shows page title/path, scope, exact revision number, content digest, actor provenance, current policy version/digest, remote-generation state, and permanent exclusions.
- Confirmation updates the existing source policy by pinning the exact revision. It does not refresh LM Wiki or accept any evidence.
- Repeating the same confirmed request is idempotent. Concurrent policy changes return a structured stale-policy result and preserve the user's review context.

### Knowledge Readiness And Health

- Add a `WikiKnowledgeReadiness` read model derived from authoritative Wiki revision, source policy, and LM Wiki review state.
- Each source reports a typed state: `eligible_unpinned`, `pinned_current`, `newer_revision_available`, `source_deleted`, `excluded`, or `policy_stale`.
- The read model returns bounded identity, reason code, responsible role, affected revision/policy versions, and one next action. It never returns page content through the readiness endpoint.
- Resolving freshness creates a new policy selection or LM Wiki candidate. It never changes an accepted LM Wiki revision or signed Twin version.
- The Wiki surface shows page-local state; the LM Wiki source-policy surface shows the workspace-wide queue. Both consume the same read model.

### Review Attention

- Surface unreviewed LM Wiki revisions, stale pinned sources, and policy conflicts in a compact knowledge maintenance queue.
- Deduplicate active items by workspace, kind, source identity, selected revision, and policy version. Superseded or resolved items leave the active queue while remaining reconstructable from immutable history.
- High-severity owner actions may project into the existing Inbox through a narrow adapter, but the knowledge queue remains the domain detail surface.
- Notifications contain only safe metadata and a stable route; they never contain Wiki bodies, raw citations, paths, prompts, or generated evidence.

## Technical Ownership

- Primary modules: `server/internal/service/wiki_knowledge.go`, `server/internal/service/lm_wiki_policy.go`, Wiki/LM Wiki handlers and queries, `packages/core/wiki`, `packages/views/wiki`, and Mobile Wiki adapters.
- The Wiki implementation may add typed contracts consumed by Twin, but it must not edit Twin proposal generation, briefing compilation, bindings, execution attribution, or effectiveness metrics.
- React Query owns readiness and source-health server state. Local view state may store only selected filters, dialogs, and drafts.
- Any persistence follows the repository migration rules: no foreign keys/cascades, explicit cleanup, and one concurrent index per single-statement migration.

## Acceptance Criteria

- An eligible Wiki revision can be pinned into the existing source policy without navigating away or copying an identifier.
- The confirmation always names the exact immutable revision and never refreshes or accepts LM Wiki as a side effect.
- Unauthorized, personal, local-only, deleted, stale-policy, and concurrent-update cases fail closed with an actionable explanation.
- Page-local freshness and the workspace maintenance queue agree for the same persisted state.
- A newer revision produces a new explicit selection path while accepted LM Wiki and signed Twin artifacts remain immutable.
- Readiness, Inbox, notification, and analytics payloads contain no Wiki content, prompts, raw citations, credentials, or local paths.
- Web/Desktop and Mobile retain platform-native interaction while sharing lifecycle semantics and authorization.

## Metrics

- Primary: percentage of eligible Wiki revisions intentionally pinned and subsequently included in an accepted LM Wiki revision.
- Activation: time from first shared Wiki page to first pinned revision and first accepted LM Wiki revision.
- Maintenance: stale-source age, policy-conflict rate, review latency, and percentage of accepted evidence with no unresolved high-severity source item.
- Guardrails: personal/local-only exclusion attempts, authorization failures, stale writes, notification duplication, and privacy violations.

## Delivery Plan

1. Add the bounded readiness/source-health contract and tests.
2. Add contextual exact-revision pinning with stale-policy protection.
3. Add workspace maintenance queue and optional high-severity Inbox projection.
4. Extend the canonical Wiki-to-LM-Wiki E2E and record Web/Desktop/Mobile acceptance evidence.
