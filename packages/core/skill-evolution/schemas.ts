import { z } from "zod";
import type {
  SkillEvolutionBundleDiff,
  SkillEvolutionDiffChange,
  SkillEvolutionDiffRowKind,
  SkillEvolutionEvaluationResult,
  SkillEvolutionLoop,
  SkillEvolutionLoopMode,
  SkillEvolutionOverview,
  SkillEvolutionOwnership,
  SkillEvolutionProposalDetail,
  SkillEvolutionProposalRequest,
  SkillEvolutionProposalRequestState,
  SkillEvolutionProposalState,
  SkillEvolutionPublication,
  SkillEvolutionPublicationOutcome,
  SkillEvolutionRelease,
  SkillEvolutionReleaseKind,
  SkillEvolutionRevision,
  SkillEvolutionSkillIdentity,
} from "./types";

function compatibleEnum<T extends string>(values: readonly string[]) {
  const known = new Set(values);
  return z.string().transform((value): T => known.has(value) ? value as T : "unknown" as T);
}

export const SkillEvolutionLoopModeSchema = compatibleEnum<SkillEvolutionLoopMode>([
  "observe", "propose", "paused",
]);
export const SkillEvolutionOwnershipSchema = compatibleEnum<SkillEvolutionOwnership>([
  "workspace", "plugin", "external", "runtime_local", "builtin",
]);
export const SkillEvolutionProposalStateSchema = compatibleEnum<SkillEvolutionProposalState>([
  "queued", "running", "ready", "failed", "stale", "rejected", "publishing",
  "published", "publication_unknown",
]);
export const SkillEvolutionProposalRequestStateSchema =
  compatibleEnum<SkillEvolutionProposalRequestState>([
    "improvement_room_queued", "proposal_queued", "proposal_running", "proposal_ready",
    "proposal_failed", "proposal_rejected", "proposal_stale", "proposal_publishing",
    "proposal_published", "proposal_publication_unknown",
  ]);
export const SkillEvolutionReleaseKindSchema = compatibleEnum<SkillEvolutionReleaseKind>([
  "publish", "rollback",
]);
export const SkillEvolutionPublicationOutcomeSchema =
  compatibleEnum<SkillEvolutionPublicationOutcome>([
    "pending", "succeeded", "failed", "publication_unknown",
  ]);
export const SkillEvolutionEvaluationResultSchema =
  compatibleEnum<SkillEvolutionEvaluationResult>([
    "passed", "failed", "inconclusive",
  ]);
export const SkillEvolutionDiffChangeSchema = compatibleEnum<SkillEvolutionDiffChange>([
  "added", "deleted", "modified",
]);
export const SkillEvolutionDiffRowKindSchema = compatibleEnum<SkillEvolutionDiffRowKind>([
  "context", "add", "delete",
]);

const nullableString = z.string().nullish().transform((value) => value ?? null);
const optionalTimestamp = z.string().nullish().transform((value) => value ?? null);
const safeMetricsSchema = z.record(z.string(), z.unknown()).catch({});

const SkillIdentityWireSchema = z.object({
  id: z.string().default(""),
  name: z.string().default(""),
  bundle_hash: z.string().default(""),
  ownership: SkillEvolutionOwnershipSchema.default("unknown"),
  ownership_reason: z.string().default(""),
  fork_required: z.boolean().default(false),
}).loose();

export const SkillEvolutionSkillIdentitySchema = SkillIdentityWireSchema.transform(
  (wire): SkillEvolutionSkillIdentity => ({
    id: wire.id,
    name: wire.name,
    bundleHash: wire.bundle_hash,
    ownership: wire.ownership,
    ownershipReason: wire.ownership_reason,
    forkRequired: wire.fork_required || wire.ownership === "unknown",
  }),
);

const LoopWireSchema = z.object({
  id: z.string().default(""),
  enabled: z.boolean().default(false),
  mode: SkillEvolutionLoopModeSchema.default("unknown"),
  cooldown_seconds: z.number().finite().default(0),
  minimum_signals: z.number().finite().default(0),
  max_evidence_refs: z.number().finite().default(0),
  max_replay_samples: z.number().finite().default(0),
  max_cost_usd_ticks: z.number().finite().default(0),
  policy_version: z.string().default(""),
  last_observed_at: optionalTimestamp.default(null),
  last_proposal_at: optionalTimestamp.default(null),
  next_eligible_at: optionalTimestamp.default(null),
  updated_at: z.string().default(""),
}).loose();

export const SkillEvolutionLoopSchema = LoopWireSchema.transform((wire): SkillEvolutionLoop => ({
  id: wire.id,
  enabled: wire.enabled,
  mode: wire.mode,
  cooldownSeconds: wire.cooldown_seconds,
  minimumSignals: wire.minimum_signals,
  maxEvidenceRefs: wire.max_evidence_refs,
  maxReplaySamples: wire.max_replay_samples,
  maxCostUsdTicks: wire.max_cost_usd_ticks,
  policyVersion: wire.policy_version,
  lastObservedAt: wire.last_observed_at,
  lastProposalAt: wire.last_proposal_at,
  nextEligibleAt: wire.next_eligible_at,
  updatedAt: wire.updated_at,
}));

const RevisionWireSchema = z.object({
  id: z.string().default(""),
  kind: z.string().default("unknown"),
  bundle_hash: z.string().default(""),
  byte_count: z.number().finite().default(0),
  support_file_count: z.number().finite().default(0),
  created_at: z.string().default(""),
}).loose();

export const SkillEvolutionRevisionSchema = RevisionWireSchema.transform(
  (wire): SkillEvolutionRevision => ({
    id: wire.id,
    kind: wire.kind,
    bundleHash: wire.bundle_hash,
    byteCount: wire.byte_count,
    supportFileCount: wire.support_file_count,
    createdAt: wire.created_at,
  }),
);

const ProposalSummaryWireSchema = z.object({
  id: z.string().default(""),
  skill_id: z.string().default(""),
  state: SkillEvolutionProposalStateSchema.default("unknown"),
  base_revision_id: z.string().default(""),
  base_hash: z.string().default(""),
  candidate_revision_id: nullableString.default(null),
  candidate_hash: nullableString.default(null),
  failure_reason: nullableString.default(null),
  stale_reason: nullableString.default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

export const SkillEvolutionProposalSummarySchema = ProposalSummaryWireSchema.transform((wire) => ({
  id: wire.id,
  skillId: wire.skill_id,
  state: wire.state,
  baseRevisionId: wire.base_revision_id,
  baseHash: wire.base_hash,
  candidateRevisionId: wire.candidate_revision_id,
  candidateHash: wire.candidate_hash,
  failureReason: wire.failure_reason,
  staleReason: wire.stale_reason,
  createdAt: wire.created_at,
  updatedAt: wire.updated_at,
}));

const ProposalRequestWireSchema = z.object({
  state: SkillEvolutionProposalRequestStateSchema.default("unknown"),
  room_id: nullableString.default(null),
  proposal: SkillEvolutionProposalSummarySchema.nullish().transform((value) => value ?? null),
}).loose();

export const SkillEvolutionProposalRequestSchema = ProposalRequestWireSchema.transform(
  (wire): SkillEvolutionProposalRequest => ({
    state: wire.state,
    roomId: wire.room_id,
    proposal: wire.proposal,
  }),
);

const ReleaseWireSchema = z.object({
  id: z.string().default(""),
  skill_id: z.string().default(""),
  proposal_id: nullableString.default(null),
  source_release_id: nullableString.default(null),
  revision_id: z.string().default(""),
  kind: SkillEvolutionReleaseKindSchema.default("unknown"),
  expected_base_hash: z.string().default(""),
  pre_hash: nullableString.default(null),
  post_hash: nullableString.default(null),
  outcome: SkillEvolutionPublicationOutcomeSchema.default("unknown"),
  error_code: nullableString.default(null),
  created_at: z.string().default(""),
  completed_at: optionalTimestamp.default(null),
}).loose();

export const SkillEvolutionReleaseSchema = ReleaseWireSchema.transform(
  (wire): SkillEvolutionRelease => ({
    id: wire.id,
    skillId: wire.skill_id,
    proposalId: wire.proposal_id,
    sourceReleaseId: wire.source_release_id,
    revisionId: wire.revision_id,
    kind: wire.kind,
    expectedBaseHash: wire.expected_base_hash,
    preHash: wire.pre_hash,
    postHash: wire.post_hash,
    outcome: wire.outcome,
    errorCode: wire.error_code,
    createdAt: wire.created_at,
    completedAt: wire.completed_at,
  }),
);

const PermissionsWireSchema = z.object({
  can_configure: z.boolean().default(false),
  can_publish: z.boolean().default(false),
  can_fork: z.boolean().default(false),
}).loose().default({ can_configure: false, can_publish: false, can_fork: false });

const OverviewWireSchema = z.object({
  skill: SkillEvolutionSkillIdentitySchema,
  loop: SkillEvolutionLoopSchema.nullish().transform((value) => value ?? null),
  revisions: z.array(SkillEvolutionRevisionSchema).catch([]).default([]),
  proposals: z.array(SkillEvolutionProposalSummarySchema).catch([]).default([]),
  releases: z.array(SkillEvolutionReleaseSchema).catch([]).default([]),
  permissions: PermissionsWireSchema,
}).loose();

export const SkillEvolutionOverviewSchema = OverviewWireSchema.transform(
  (wire): SkillEvolutionOverview => ({
    skill: wire.skill,
    loop: wire.loop,
    revisions: wire.revisions,
    proposals: wire.proposals,
    releases: wire.releases,
    permissions: wire.skill.ownership === "unknown" || wire.skill.id.trim() === ""
      ? { canConfigure: false, canPublish: false, canFork: false }
      : {
          canConfigure: wire.permissions.can_configure,
          canPublish: wire.permissions.can_publish,
          canFork: wire.permissions.can_fork,
        },
  }),
);

const RationaleWireSchema = z.object({
  observed_pattern: z.string().default(""),
  expected_benefit: z.string().default(""),
  regression_risk: z.string().default(""),
}).loose();

const DiffWireSchema = z.object({
  truncated: z.boolean().default(false),
  omitted_rows: z.number().finite().default(0),
  metadata: z.array(z.object({
    field: z.string().default(""),
    before: z.string().default(""),
    after: z.string().default(""),
  }).loose()).catch([]).default([]),
  files: z.array(z.object({
    path: z.string().default(""),
    change: SkillEvolutionDiffChangeSchema.default("unknown"),
    truncated: z.boolean().default(false),
    omitted_rows: z.number().finite().default(0),
    rows: z.array(z.object({
      kind: SkillEvolutionDiffRowKindSchema.default("unknown"),
      old_line: z.number().finite().nullish().transform((value) => value ?? null),
      new_line: z.number().finite().nullish().transform((value) => value ?? null),
      text: z.string().default(""),
    }).loose()).catch([]).default([]),
  }).loose()).catch([]).default([]),
}).loose().default({ truncated: false, omitted_rows: 0, metadata: [], files: [] });

const EvidenceWireSchema = z.object({
  kind: z.string().default("unknown"),
  source_id: z.string().default(""),
  source_revision_id: nullableString.default(null),
  source_state: z.string().default("unknown"),
  digest: z.string().default(""),
  observed_at: z.string().default(""),
}).loose();

const EvaluationWireSchema = z.object({
  id: z.string().default(""),
  kind: z.string().default("unknown"),
  result: SkillEvolutionEvaluationResultSchema.default("unknown"),
  adapter: z.string().default(""),
  adapter_version: z.string().default(""),
  policy_version: z.string().default(""),
  result_digest: z.string().default(""),
  safe_metrics: safeMetricsSchema.default({}),
  cost_usd_ticks: z.number().finite().nullish().transform((value) => value ?? null),
  duration_ms: z.number().finite().default(0),
  created_at: z.string().default(""),
}).loose();

const ReviewWireSchema = z.object({
  id: z.string().default(""),
  decision: z.string().default("unknown"),
  actor_id: z.string().default(""),
  reason: nullableString.default(null),
  created_at: z.string().default(""),
}).loose();

const ProposalDetailWireSchema = z.object({
  proposal: SkillEvolutionProposalSummarySchema,
  rationale: RationaleWireSchema.nullish().catch(null).transform((value) => value === null || value === undefined
    ? null
    : {
        observedPattern: value.observed_pattern,
        expectedBenefit: value.expected_benefit,
        regressionRisk: value.regression_risk,
      }),
  diff: DiffWireSchema,
  evidence: z.array(EvidenceWireSchema).catch([]).default([]),
  evaluations: z.array(EvaluationWireSchema).catch([]).default([]),
  reviews: z.array(ReviewWireSchema).catch([]).default([]),
}).loose();

export const SkillEvolutionProposalDetailSchema = ProposalDetailWireSchema.transform(
  (wire): SkillEvolutionProposalDetail => ({
    proposal: wire.proposal,
    rationale: wire.rationale,
    diff: {
      truncated: wire.diff.truncated,
      omittedRows: wire.diff.omitted_rows,
      metadata: wire.diff.metadata,
      files: wire.diff.files.map((file) => ({
        path: file.path,
        change: file.change,
        truncated: file.truncated,
        omittedRows: file.omitted_rows,
        rows: file.rows.map((row) => ({
          kind: row.kind,
          oldLine: row.old_line,
          newLine: row.new_line,
          text: row.text,
        })),
      })),
    } satisfies SkillEvolutionBundleDiff,
    evidence: wire.evidence.map((item) => ({
      kind: item.kind,
      sourceId: item.source_id,
      sourceRevisionId: item.source_revision_id,
      sourceState: item.source_state,
      digest: item.digest,
      observedAt: item.observed_at,
    })),
    evaluations: wire.evaluations.map((item) => ({
      id: item.id,
      kind: item.kind,
      result: item.result,
      adapter: item.adapter,
      adapterVersion: item.adapter_version,
      policyVersion: item.policy_version,
      resultDigest: item.result_digest,
      safeMetrics: item.safe_metrics,
      costUsdTicks: item.cost_usd_ticks,
      durationMs: item.duration_ms,
      createdAt: item.created_at,
    })),
    reviews: wire.reviews.map((item) => ({
      id: item.id,
      decision: item.decision,
      actorId: item.actor_id,
      reason: item.reason,
      createdAt: item.created_at,
    })),
  }),
);

const PublicationWireSchema = z.object({
  proposal: SkillEvolutionProposalSummarySchema.nullish().transform((value) => value ?? null),
  release: SkillEvolutionReleaseSchema,
}).loose();

export const SkillEvolutionPublicationSchema = PublicationWireSchema.transform(
  (wire): SkillEvolutionPublication => ({ proposal: wire.proposal, release: wire.release }),
);

export const EMPTY_SKILL_EVOLUTION_IDENTITY: SkillEvolutionSkillIdentity = {
  id: "",
  name: "",
  bundleHash: "",
  ownership: "unknown",
  ownershipReason: "",
  forkRequired: false,
};

export const EMPTY_SKILL_EVOLUTION_LOOP: SkillEvolutionLoop = {
  id: "",
  enabled: false,
  mode: "unknown",
  cooldownSeconds: 0,
  minimumSignals: 0,
  maxEvidenceRefs: 0,
  maxReplaySamples: 0,
  maxCostUsdTicks: 0,
  policyVersion: "",
  lastObservedAt: null,
  lastProposalAt: null,
  nextEligibleAt: null,
  updatedAt: "",
};

export const EMPTY_SKILL_EVOLUTION_OVERVIEW: SkillEvolutionOverview = {
  skill: EMPTY_SKILL_EVOLUTION_IDENTITY,
  loop: null,
  revisions: [],
  proposals: [],
  releases: [],
  permissions: { canConfigure: false, canPublish: false, canFork: false },
};

export const EMPTY_SKILL_EVOLUTION_PROPOSAL_SUMMARY =
  SkillEvolutionProposalSummarySchema.parse({});
export const EMPTY_SKILL_EVOLUTION_PROPOSAL_REQUEST: SkillEvolutionProposalRequest = {
  state: "unknown",
  roomId: null,
  proposal: null,
};
export const EMPTY_SKILL_EVOLUTION_RELEASE = SkillEvolutionReleaseSchema.parse({});

export const EMPTY_SKILL_EVOLUTION_PROPOSAL_DETAIL: SkillEvolutionProposalDetail = {
  proposal: EMPTY_SKILL_EVOLUTION_PROPOSAL_SUMMARY,
  rationale: null,
  diff: { truncated: false, omittedRows: 0, metadata: [], files: [] },
  evidence: [],
  evaluations: [],
  reviews: [],
};

export const EMPTY_SKILL_EVOLUTION_PUBLICATION: SkillEvolutionPublication = {
  proposal: null,
  release: EMPTY_SKILL_EVOLUTION_RELEASE,
};
