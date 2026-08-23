import type { LifecycleContent } from "./types";

export type TwinBindingState = "off" | "preview" | "enabled";
export type TwinBindingScope = "workspace" | "agent" | "project" | "issue" | "one_off";
export type TwinFeedbackRating = "helped" | "irrelevant" | "mismatch";

export interface TwinKillSwitch {
  readonly enabled: boolean;
  readonly reason: string | null;
}

export interface TwinBinding {
  readonly id: string;
  readonly scopeType: TwinBindingScope;
  readonly scopeId: string;
  readonly state: TwinBindingState;
  readonly twinVersionId: string;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface TwinBindingsResponse {
  readonly bindings: readonly TwinBinding[];
  readonly canManage: boolean;
  readonly killSwitch: TwinKillSwitch;
}

export interface UpsertTwinBindingInput {
  readonly scopeType: TwinBindingScope;
  readonly scopeId: string;
  readonly state: TwinBindingState;
  readonly twinVersionId: string;
}

export interface TwinPolicyExclusion {
  readonly bindingId: string;
  readonly scopeType: TwinBindingScope;
  readonly code: string;
}

export interface TwinEffectivePolicy {
  readonly state: TwinBindingState;
  readonly scopeType: TwinBindingScope | null;
  readonly scopeId: string | null;
  readonly bindingId: string | null;
  readonly explicit: boolean;
  readonly reason: string;
  readonly exclusions: readonly TwinPolicyExclusion[];
}

export interface TwinVersionReference {
  readonly id: string;
  readonly versionNumber: number;
  readonly contentDigest: string;
}

export interface TwinBriefingPreviewInput {
  readonly agentId: string;
  readonly projectId?: string;
  readonly issueId?: string;
  readonly runId?: string;
  readonly request: string;
  readonly tags?: readonly string[];
  readonly oneOffState?: TwinBindingState;
  readonly twinVersionId?: string;
}

export interface TwinBriefingPreview {
  readonly policy: TwinEffectivePolicy;
  readonly twinVersion: TwinVersionReference | null;
  readonly briefing: string;
  readonly briefingDigest: string;
  readonly assertionIds: readonly string[];
  readonly citationKeys: readonly string[];
  readonly compilerVersion: string;
  readonly byteCount: number;
  readonly tokenCount: number;
  readonly inject: boolean;
  readonly exclusionReasons: readonly string[];
}

export interface TwinTaskAttribution {
  readonly twinVersionId: string;
  readonly twinVersionNumber: number;
  readonly twinVersionDigest: string;
  readonly briefing: string;
  readonly briefingDigest: string;
  readonly assertionIds: readonly string[];
  readonly citationKeys: readonly string[];
  readonly policyScopeType: TwinBindingScope;
  readonly policyScopeId: string;
  readonly policyState: TwinBindingState;
  readonly compilerVersion: string;
  readonly byteCount: number;
  readonly tokenCount: number;
}

export interface TwinTaskFeedback {
  readonly id: string;
  readonly taskId: string;
  readonly rating: TwinFeedbackRating;
  readonly note: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface TwinDeposition {
  readonly id: string;
  readonly taskId: string;
  readonly baseTwinVersionId: string;
  readonly proposalId: string;
  readonly replacesProposalId?: string;
  readonly evidenceDigest: string;
  readonly state: string;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface TwinRunAssertion {
  readonly id: string;
  readonly type: string;
  readonly text: string;
  readonly citationKeys: readonly string[];
}

export interface TwinRunCitation {
  readonly key: string;
  readonly label: string;
  readonly sourceType: string;
  readonly locator: string;
}

export interface TwinTaskContext {
  readonly taskId: string;
  readonly attribution?: TwinTaskAttribution;
  readonly feedback?: TwinTaskFeedback;
  readonly depositions: readonly TwinDeposition[];
  readonly assertions: readonly TwinRunAssertion[];
  readonly citations: readonly TwinRunCitation[];
}

export interface TwinFeedbackInput {
  readonly rating: TwinFeedbackRating;
  readonly note?: string;
}

export interface CreateTwinDepositionInput {
  readonly replacesProposalId?: string;
  readonly editedAssertions?: readonly LifecycleContent[];
}

export interface TwinDepositionProposal {
  readonly id: string;
  readonly kind: string;
  readonly schemaVersion: number;
  readonly contentDigest: string;
  readonly createdAt: string;
}

export interface TwinDepositionResponse {
  readonly deposition: TwinDeposition | null;
  readonly proposal: TwinDepositionProposal | null;
}

export interface TwinMetricsFeedback {
  readonly total: number;
  readonly helped: number;
  readonly irrelevant: number;
  readonly mismatch: number;
}

export interface TwinMetricsDepositions {
  readonly total: number;
  readonly pending: number;
  readonly accepted: number;
  readonly rejected: number;
}

export interface TwinMetricsBindings {
  readonly off: number;
  readonly preview: number;
  readonly enabled: number;
}

export interface TwinExecutionMetrics {
  readonly attributedRuns: number;
  readonly feedback: TwinMetricsFeedback;
  readonly depositions: TwinMetricsDepositions;
  readonly bindings: TwinMetricsBindings;
  readonly helpfulnessRate: number | null;
  readonly killSwitch: TwinKillSwitch;
}

export interface TwinDepositionEvidence {
  readonly taskId: string;
  readonly baseTwinVersionId: string;
  readonly evidenceDigest: string;
  readonly taskStatus: string;
  readonly completedAt: string | null;
  readonly feedbackRating: TwinFeedbackRating | null;
  readonly safeMetadata: LifecycleContent;
}
