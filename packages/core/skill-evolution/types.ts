export type SkillEvolutionLoopMode = "observe" | "propose" | "paused" | "unknown";

export type SkillEvolutionOwnership =
  | "workspace"
  | "plugin"
  | "external"
  | "runtime_local"
  | "builtin"
  | "unknown";

export type SkillEvolutionProposalState =
  | "queued"
  | "running"
  | "ready"
  | "failed"
  | "stale"
  | "rejected"
  | "publishing"
  | "published"
  | "publication_unknown"
  | "unknown";

export type SkillEvolutionProposalRequestState =
  | "improvement_room_queued"
  | "proposal_queued"
  | "proposal_running"
  | "proposal_ready"
  | "proposal_failed"
  | "proposal_rejected"
  | "proposal_stale"
  | "proposal_publishing"
  | "proposal_published"
  | "unknown";

export type SkillEvolutionReleaseKind = "publish" | "rollback" | "unknown";

export type SkillEvolutionPublicationOutcome =
  | "pending"
  | "succeeded"
  | "failed"
  | "publication_unknown"
  | "unknown";

export type SkillEvolutionEvaluationResult =
  | "passed"
  | "failed"
  | "inconclusive"
  | "unknown";

export interface SkillEvolutionSkillIdentity {
  readonly id: string;
  readonly name: string;
  readonly bundleHash: string;
  readonly ownership: SkillEvolutionOwnership;
  readonly ownershipReason: string;
  readonly forkRequired: boolean;
}

export interface SkillEvolutionPermissions {
  readonly canConfigure: boolean;
  readonly canPublish: boolean;
  readonly canFork: boolean;
}

export interface SkillEvolutionLoop {
  readonly id: string;
  readonly enabled: boolean;
  readonly mode: SkillEvolutionLoopMode;
  readonly cooldownSeconds: number;
  readonly minimumSignals: number;
  readonly maxEvidenceRefs: number;
  readonly maxReplaySamples: number;
  readonly maxCostUsdTicks: number;
  readonly policyVersion: string;
  readonly lastObservedAt: string | null;
  readonly lastProposalAt: string | null;
  readonly nextEligibleAt: string | null;
  readonly updatedAt: string;
}

export interface SkillEvolutionRevision {
  readonly id: string;
  readonly kind: string;
  readonly bundleHash: string;
  readonly byteCount: number;
  readonly supportFileCount: number;
  readonly createdAt: string;
}

export interface SkillEvolutionProposalSummary {
  readonly id: string;
  readonly skillId: string;
  readonly state: SkillEvolutionProposalState;
  readonly baseRevisionId: string;
  readonly baseHash: string;
  readonly candidateRevisionId: string | null;
  readonly candidateHash: string | null;
  readonly failureReason: string | null;
  readonly staleReason: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface SkillEvolutionRelease {
  readonly id: string;
  readonly skillId: string;
  readonly proposalId: string | null;
  readonly sourceReleaseId: string | null;
  readonly revisionId: string;
  readonly kind: SkillEvolutionReleaseKind;
  readonly expectedBaseHash: string;
  readonly preHash: string | null;
  readonly postHash: string | null;
  readonly outcome: SkillEvolutionPublicationOutcome;
  readonly errorCode: string | null;
  readonly createdAt: string;
  readonly completedAt: string | null;
}

export interface SkillEvolutionOverview {
  readonly skill: SkillEvolutionSkillIdentity;
  readonly loop: SkillEvolutionLoop | null;
  readonly revisions: readonly SkillEvolutionRevision[];
  readonly proposals: readonly SkillEvolutionProposalSummary[];
  readonly releases: readonly SkillEvolutionRelease[];
  readonly permissions: SkillEvolutionPermissions;
}

export interface SkillEvolutionRationale {
  readonly observedPattern: string;
  readonly expectedBenefit: string;
  readonly regressionRisk: string;
}

export type SkillEvolutionDiffChange = "added" | "deleted" | "modified" | "unknown";
export type SkillEvolutionDiffRowKind = "context" | "add" | "delete" | "unknown";

export interface SkillEvolutionMetadataDiff {
  readonly field: string;
  readonly before: string;
  readonly after: string;
}

export interface SkillEvolutionDiffRow {
  readonly kind: SkillEvolutionDiffRowKind;
  readonly oldLine: number | null;
  readonly newLine: number | null;
  readonly text: string;
}

export interface SkillEvolutionFileDiff {
  readonly path: string;
  readonly change: SkillEvolutionDiffChange;
  readonly truncated: boolean;
  readonly omittedRows: number;
  readonly rows: readonly SkillEvolutionDiffRow[];
}

export interface SkillEvolutionBundleDiff {
  readonly truncated: boolean;
  readonly omittedRows: number;
  readonly metadata: readonly SkillEvolutionMetadataDiff[];
  readonly files: readonly SkillEvolutionFileDiff[];
}

export interface SkillEvolutionEvidence {
  readonly kind: string;
  readonly sourceId: string;
  readonly sourceRevisionId: string | null;
  readonly sourceState: string;
  readonly digest: string;
  readonly observedAt: string;
}

export interface SkillEvolutionEvaluation {
  readonly id: string;
  readonly kind: string;
  readonly result: SkillEvolutionEvaluationResult;
  readonly adapter: string;
  readonly adapterVersion: string;
  readonly policyVersion: string;
  readonly resultDigest: string;
  readonly safeMetrics: Readonly<Record<string, unknown>>;
  readonly costUsdTicks: number | null;
  readonly durationMs: number;
  readonly createdAt: string;
}

export interface SkillEvolutionReview {
  readonly id: string;
  readonly decision: string;
  readonly actorId: string;
  readonly reason: string | null;
  readonly createdAt: string;
}

export interface SkillEvolutionProposalDetail {
  readonly proposal: SkillEvolutionProposalSummary;
  readonly rationale: SkillEvolutionRationale | null;
  readonly diff: SkillEvolutionBundleDiff;
  readonly evidence: readonly SkillEvolutionEvidence[];
  readonly evaluations: readonly SkillEvolutionEvaluation[];
  readonly reviews: readonly SkillEvolutionReview[];
}

export interface SkillEvolutionProposalRequest {
  readonly state: SkillEvolutionProposalRequestState;
  readonly roomId: string | null;
  readonly proposal: SkillEvolutionProposalSummary | null;
}

export interface SkillEvolutionPublication {
  readonly proposal: SkillEvolutionProposalSummary | null;
  readonly release: SkillEvolutionRelease;
}

export interface ConfigureSkillEvolutionInput {
  readonly enabled: boolean;
  readonly mode: Exclude<SkillEvolutionLoopMode, "unknown">;
  readonly cooldownSeconds: number;
  readonly minimumSignals: number;
  readonly maxEvidenceRefs: number;
  readonly maxReplaySamples: number;
  readonly maxCostUsdTicks: number;
  readonly policyVersion: string;
}

export type SkillEvolutionLoopConfigInput = ConfigureSkillEvolutionInput;

export interface SkillEvolutionIdempotencyInput {
  readonly idempotencyKey: string;
}

export interface RollbackSkillEvolutionReleaseInput extends SkillEvolutionIdempotencyInput {
  readonly releaseId: string;
}

export interface DecideSkillEvolutionProposalInput extends SkillEvolutionIdempotencyInput {
  readonly reason?: string;
}

export interface ForkSkillForEvolutionInput extends SkillEvolutionIdempotencyInput {
  readonly name: string;
}
