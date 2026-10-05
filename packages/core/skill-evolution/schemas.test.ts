// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  SkillEvolutionOverviewSchema,
  SkillEvolutionProposalDetailSchema,
  SkillEvolutionProposalRequestSchema,
} from "./schemas";

describe("Skill evolution response schemas", () => {
  it("maps the complete review contract without exposing wire casing", () => {
    const overview = SkillEvolutionOverviewSchema.parse({
      skill: {
        id: "skill-1",
        name: "Triage",
        bundle_hash: "sha256:base",
        ownership: "workspace",
        ownership_reason: "manual_or_unattributed",
        fork_required: false,
      },
      loop: {
        id: "loop-1",
        enabled: true,
        mode: "propose",
        cooldown_seconds: 3600,
        minimum_signals: 3,
        max_evidence_refs: 10,
        max_replay_samples: 4,
        max_cost_usd_ticks: 50,
        policy_version: "v1",
        next_eligible_at: "2026-08-28T12:00:00Z",
        updated_at: "2026-08-28T11:00:00Z",
      },
      revisions: [{
        id: "revision-1",
        kind: "base",
        bundle_hash: "sha256:base",
        byte_count: 128,
        support_file_count: 1,
        created_at: "2026-08-28T10:00:00Z",
      }],
      proposals: [{
        id: "proposal-1",
        skill_id: "skill-1",
        state: "stale",
        base_revision_id: "revision-1",
        base_hash: "sha256:base",
        stale_reason: "live_bundle_changed",
        created_at: "2026-08-28T10:30:00Z",
        updated_at: "2026-08-28T11:30:00Z",
      }],
      releases: [{
        id: "release-1",
        skill_id: "skill-1",
        proposal_id: "proposal-1",
        revision_id: "revision-2",
        kind: "publish",
        expected_base_hash: "sha256:base",
        pre_hash: "sha256:base",
        post_hash: "sha256:candidate",
        outcome: "publication_unknown",
        error_code: "post_write_hash_mismatch",
        created_at: "2026-08-28T11:00:00Z",
      }],
      permissions: { can_configure: true, can_publish: false, can_fork: false },
    });

    expect(overview).toMatchObject({
      skill: { id: "skill-1", bundleHash: "sha256:base", ownership: "workspace" },
      loop: { cooldownSeconds: 3600, nextEligibleAt: "2026-08-28T12:00:00Z" },
      revisions: [{ bundleHash: "sha256:base", supportFileCount: 1 }],
      proposals: [{ state: "stale", staleReason: "live_bundle_changed" }],
      releases: [{ outcome: "publication_unknown", errorCode: "post_write_hash_mismatch" }],
      permissions: { canConfigure: true, canPublish: false, canFork: false },
    });
  });

  it("defaults omitted optional fields and tolerates future enum values", () => {
    const overview = SkillEvolutionOverviewSchema.parse({
      skill: { id: "skill-1", ownership: "federated" },
      loop: { mode: "continuous" },
      proposals: [{ state: "awaiting_policy", id: "proposal-1" }],
      releases: [{ kind: "canary", outcome: "reconciled" }],
      permissions: { can_configure: true, can_publish: true, can_fork: true },
    });

    expect(overview.skill).toMatchObject({ ownership: "unknown", forkRequired: true });
    expect(overview.loop).toMatchObject({ mode: "unknown", lastObservedAt: null });
    expect(overview.proposals[0]).toMatchObject({
      state: "unknown",
      candidateHash: null,
      failureReason: null,
    });
    expect(overview.releases[0]).toMatchObject({ kind: "unknown", outcome: "unknown" });
    expect(overview.permissions).toEqual({
      canConfigure: false,
      canPublish: false,
      canFork: false,
    });
  });

  it("parses rationale, structured diff, provenance, safe metrics, and reviews defensively", () => {
    const detail = SkillEvolutionProposalDetailSchema.parse({
      proposal: { id: "proposal-1", state: "ready" },
      rationale: {
        observed_pattern: "Repeated correction",
        expected_benefit: "Fewer retries",
        regression_risk: "May reject terse requests",
      },
      diff: {
        truncated: true,
        omitted_rows: 42,
        metadata: [{ field: "description", before: "old", after: "new" }],
        files: [{
          path: "SKILL.md",
          change: "modified",
          truncated: true,
          omitted_rows: 12,
          rows: [{ kind: "add", new_line: 2, text: "Be explicit." }],
        }],
      },
      evidence: [{
        kind: "task_feedback",
        source_id: "task-1",
        source_state: "accepted",
        digest: "sha256:evidence",
        observed_at: "2026-08-28T10:00:00Z",
      }],
      evaluations: [{
        id: "evaluation-1",
        kind: "behavioral_replay",
        result: "future_verdict",
        safe_metrics: { sample_count: 4, nondeterministic: true },
        cost_usd_ticks: 8,
      }],
      reviews: [{ id: "review-1", decision: "publish", actor_id: "user-1" }],
    });

    expect(detail.rationale?.observedPattern).toBe("Repeated correction");
    expect(detail.diff).toMatchObject({ truncated: true, omittedRows: 42 });
    expect(detail.diff.files[0]).toMatchObject({ truncated: true, omittedRows: 12 });
    expect(detail.diff.files[0]?.rows[0]).toEqual({
      kind: "add",
      oldLine: null,
      newLine: 2,
      text: "Be explicit.",
    });
    expect(detail.evidence[0]).toMatchObject({
      sourceId: "task-1",
      sourceRevisionId: null,
      sourceState: "accepted",
      digest: "sha256:evidence",
    });
    expect(detail.evaluations[0]).toMatchObject({
      result: "unknown",
      safeMetrics: { sample_count: 4, nondeterministic: true },
      costUsdTicks: 8,
    });
    expect(detail.reviews[0]).toMatchObject({ actorId: "user-1", reason: null });
  });

  it("parses visible Room scheduling and recovered proposal responses as one safe union", () => {
    expect(SkillEvolutionProposalRequestSchema.parse({
      state: "improvement_room_queued",
      room_id: "room-1",
    })).toEqual({
      state: "improvement_room_queued",
      roomId: "room-1",
      proposal: null,
    });
    expect(SkillEvolutionProposalRequestSchema.parse({
      state: "proposal_ready",
      room_id: "room-1",
      proposal: { id: "proposal-1", state: "ready" },
    })).toMatchObject({
      state: "proposal_ready",
      roomId: "room-1",
      proposal: { id: "proposal-1", state: "ready" },
    });
    expect(SkillEvolutionProposalRequestSchema.parse({ state: "future_workflow" }))
      .toEqual({ state: "unknown", roomId: null, proposal: null });
    expect(SkillEvolutionProposalRequestSchema.parse({ state: "proposal_publication_unknown" }))
      .toEqual({ state: "proposal_publication_unknown", roomId: null, proposal: null });
  });
});
