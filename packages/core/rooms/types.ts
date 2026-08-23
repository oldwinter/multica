export type RoomParticipantInputType = "agent" | "member";

export type RoomParticipantType = RoomParticipantInputType | "unknown";

export type RoomParticipantRole = "facilitator" | "participant" | "observer";

export type RoomPromotionKind = "issue" | "wiki" | "decision";

export type RoomArtifactKind = RoomPromotionKind | "unknown";

export type RoomStatus = "active" | "paused" | "archived" | "unknown";

export type RoomEntryType = "message" | "result" | "system" | "unknown";

export type RoomAuthorType = "member" | "agent" | "system" | "unknown";

export type RoomCycleSource = "message" | "mention" | "manual" | "schedule" | "agent" | "unknown";

export type RoomCycleStatus =
  | "refused"
  | "queued"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "unknown";

export type RoomCyclePhase =
  | "gathering"
  | "synthesizing"
  | "awaiting_review"
  | "completed"
  | "failed"
  | "cancelled"
  | "refused"
  | "unknown";

export type RoomTurnKind = "participant" | "synthesis" | "unknown";

export type RoomTemplateId =
  | "research"
  | "planning"
  | "risk"
  | "incident"
  | "decision"
  | "unknown";

export type RoomTurnStatus = RoomCycleStatus | "dispatched";

export type RoomRefusalReason =
  | "room_paused"
  | "room_archived"
  | "budget_exhausted"
  | "active_cycle"
  | "agent_unavailable"
  | "invocation_not_allowed"
  | "spend_limit_unsupported"
  | "unknown";

export interface RoomSynthesisItem {
  readonly text: string;
  readonly citation_entry_ids: readonly string[];
  readonly confidence: number;
}

export interface RoomRecommendation {
  readonly key: string;
  readonly kind: RoomPromotionKind;
  readonly title: string;
  readonly body: string;
  readonly rationale: string;
  readonly citation_entry_ids: readonly string[];
  readonly confidence: number;
}

export interface RoomSynthesis {
  readonly schema_version: 1;
  readonly summary: string;
  readonly facts: readonly RoomSynthesisItem[];
  readonly decisions: readonly RoomSynthesisItem[];
  readonly open_questions: readonly RoomSynthesisItem[];
  readonly disagreements: readonly RoomSynthesisItem[];
  readonly action_items: readonly RoomSynthesisItem[];
  readonly recommendations: readonly RoomRecommendation[];
  readonly confidence: number;
}

export type RoomMemoryReviewStatus =
  | "pending"
  | "accepted"
  | "rejected"
  | "corrected"
  | "unknown";

export interface RoomMemoryRevision {
  readonly id: string;
  readonly room_id: string;
  readonly cycle_id: string;
  readonly synthesis_turn_id: string;
  readonly version: number;
  readonly schema_version: number;
  readonly synthesis: RoomSynthesis;
  readonly digest: string;
  readonly creator_type: "member" | "agent";
  readonly creator_id: string;
  readonly review_status: RoomMemoryReviewStatus;
  readonly reviewed_by_user_id: string | null;
  readonly reviewed_at: string | null;
  readonly corrected_from_revision_id: string | null;
  readonly created_at: string;
}

export type RoomRecommendationReviewStatus = "approved" | "rejected" | "unknown";

export interface RoomRecommendationReview {
  readonly id: string;
  readonly room_id: string;
  readonly memory_revision_id: string;
  readonly recommendation_key: string;
  readonly status: RoomRecommendationReviewStatus;
  readonly artifact_id: string | null;
  readonly reviewed_by_user_id: string;
  readonly reviewed_at: string;
}

export interface RoomMemoryContribution {
  readonly agent_id: string;
  readonly turn_id: string;
  readonly body: string;
  readonly at: string;
}

export interface RoomMemory {
  readonly summary: string;
  readonly facts: readonly string[];
  readonly decisions: readonly string[];
  readonly open_questions: readonly string[];
  readonly recent_contributions: readonly RoomMemoryContribution[];
}

export interface Room {
  readonly id: string;
  readonly workspace_id: string;
  readonly title: string;
  readonly instructions: string;
  readonly objective: string;
  readonly success_criteria: readonly string[];
  readonly stop_conditions: readonly string[];
  readonly template_id: RoomTemplateId | null;
  readonly created_by_user_id: string;
  readonly facilitator_agent_id: string;
  readonly facilitator_squad_id: string | null;
  readonly status: RoomStatus;
  readonly daily_turn_limit: number | null;
  readonly max_cost_ticks: number | null;
  readonly schedule_interval_minutes: number | null;
  readonly next_wake_at: string | null;
  readonly active_cycle_id: string | null;
  readonly memory: RoomMemory;
  readonly memory_version: number;
  readonly accepted_memory_revision_id: string | null;
  readonly capability_version: number;
  readonly created_at: string;
  readonly updated_at: string;
}

export interface RoomParticipant {
  readonly id: string;
  readonly type: RoomParticipantType;
  readonly participant_id: string;
  readonly role: RoomParticipantRole;
  readonly source_squad_id: string | null;
  readonly joined_at: string;
}

export interface RoomEntry {
  readonly id: string;
  readonly cycle_id: string | null;
  readonly turn_id: string | null;
  readonly ordinal: number;
  readonly type: RoomEntryType;
  readonly author_type: RoomAuthorType;
  readonly author_id: string | null;
  readonly body: string;
  readonly mentions: readonly string[];
  readonly created_at: string;
}

export interface RoomCycle {
  readonly id: string;
  readonly sequence: number;
  readonly source: RoomCycleSource;
  readonly wake_key: string;
  readonly triggering_entry_id: string | null;
  readonly status: RoomCycleStatus;
  readonly phase: RoomCyclePhase;
  readonly refusal_reason: RoomRefusalReason | null;
  readonly synthesis_error: {
    readonly code: string;
    readonly message: string;
    readonly retryable: boolean;
  } | null;
  readonly synthesis_turn_id: string | null;
  readonly memory_revision_id: string | null;
  readonly expected_max_turns: number;
  readonly cost_limit_ticks: number | null;
  readonly planned_at: string | null;
  readonly created_at: string;
  readonly started_at: string | null;
  readonly completed_at: string | null;
}

export interface RoomTurn {
  readonly id: string;
  readonly cycle_id: string;
  readonly agent_id: string;
  readonly squad_id: string | null;
  readonly status: RoomTurnStatus;
  readonly turn_kind: RoomTurnKind;
  readonly attempt: number;
  readonly refusal_reason: RoomRefusalReason | null;
  readonly created_at: string;
  readonly started_at: string | null;
  readonly completed_at: string | null;
}

export interface RoomArtifact {
  readonly id: string;
  readonly cycle_id: string | null;
  readonly turn_id: string | null;
  readonly entry_id: string | null;
  readonly kind: RoomArtifactKind;
  readonly target_id: string | null;
  readonly title: string;
  readonly body: string;
  readonly rationale: string | null;
  readonly memory_revision_id: string | null;
  readonly recommendation_key: string | null;
  readonly citation_entry_ids: readonly string[];
  readonly created_by_user_id: string;
  readonly created_at: string;
}

export interface RoomDetail {
  readonly room: Room;
  readonly participants: readonly RoomParticipant[];
  readonly entries: readonly RoomEntry[];
  readonly cycles: readonly RoomCycle[];
  readonly turns: readonly RoomTurn[];
  readonly artifacts: readonly RoomArtifact[];
  readonly memory_revisions: readonly RoomMemoryRevision[];
  readonly recommendation_reviews: readonly RoomRecommendationReview[];
}

export interface RoomPreflightAgent {
  readonly agent_id: string;
  readonly ready: boolean;
  readonly invocation_allowed: boolean;
  readonly reason: string | null;
}

export interface RoomBudgetUsage {
  readonly daily_turn_limit: number | null;
  readonly used_turns: number;
  readonly max_cost_ticks: number | null;
  readonly used_cost_ticks: number;
  readonly remaining_cost_ticks: number | null;
  readonly reserved_cost_ticks: number;
  readonly uncosted_turns: number;
}

export interface RoomPreflight {
  readonly source: "manual" | "schedule" | "unknown";
  readonly allowed: boolean;
  readonly refusal_reason: string | null;
  readonly capability_version: number;
  readonly capability_ready: boolean;
  readonly spend_limit_supported: boolean;
  readonly required_daemon_capability: string;
  readonly required_cost_capability: string;
  readonly target_agents: readonly RoomPreflightAgent[];
  readonly expected_max_turns: number;
  readonly synthesis_required: boolean;
  readonly budget: RoomBudgetUsage;
}

export interface RoomUsage {
  readonly turns_total: number;
  readonly cost_ticks: number;
  readonly uncosted_turns: number;
  readonly failures: number;
  readonly accepted_syntheses: number;
  readonly promoted_artifacts: number;
}

export interface RoomWakeResult {
  readonly cycle: RoomCycle;
  readonly turns: readonly RoomTurn[];
  readonly tasks: readonly string[];
}

export interface RoomMessageResult extends RoomWakeResult {
  readonly entry: RoomEntry;
}

export interface RoomParticipantInput {
  readonly type: RoomParticipantInputType;
  readonly id: string;
  readonly role?: RoomParticipantRole;
}

type RoomFacilitator =
  | {
      readonly facilitator_agent_id: string;
      readonly facilitator_squad_id?: never;
    }
  | {
      readonly facilitator_agent_id?: never;
      readonly facilitator_squad_id: string;
    };

export type CreateRoomInput = RoomFacilitator & {
  readonly title: string;
  readonly instructions?: string;
  readonly objective: string;
  readonly success_criteria?: readonly string[];
  readonly stop_conditions?: readonly string[];
  readonly template_id?: Exclude<RoomTemplateId, "unknown">;
  readonly participants?: readonly RoomParticipantInput[];
  readonly daily_turn_limit?: number;
  readonly max_cost_ticks?: number;
  readonly schedule_interval_minutes?: number;
};

export interface PostRoomMessageInput {
  readonly body: string;
  readonly mention_agent_ids?: readonly string[];
  readonly idempotency_key: string;
}

export interface WakeRoomInput {
  readonly idempotency_key: string;
  readonly target_agent_ids?: readonly string[];
}

export interface SetRoomStatusInput {
  readonly status: "active" | "paused" | "archived";
}

export interface UpdateRoomBudgetInput {
  readonly daily_turn_limit: number | null;
  readonly max_cost_ticks: number | null;
}

export interface RetryRoomSynthesisInput {
  readonly idempotency_key: string;
}

export interface RetryRoomSynthesisResult {
  readonly cycle: RoomCycle;
  readonly turn: RoomTurn;
  readonly task_id: string;
}

export interface ReviewRoomCycleInput {
  readonly action: "accept" | "reject" | "correct";
  readonly expected_memory_version: number;
  readonly correction?: RoomSynthesis;
  readonly idempotency_key: string;
}

export interface ReviewRoomCycleResult {
  readonly room: Room;
  readonly memory_revision: RoomMemoryRevision;
}

export interface CancelRoomCycleInput {
  readonly idempotency_key: string;
}

export interface CancelRoomCycleResult {
  readonly cycle: RoomCycle;
}

export interface RejectRoomRecommendationInput {
  readonly action: "reject";
  readonly idempotency_key: string;
}

export interface ReviewRoomRecommendationResult {
  readonly recommendation_review: RoomRecommendationReview;
}

type RoomPromotionSource =
  | {
      readonly entry_id: string;
      readonly cycle_id?: never;
      readonly memory_revision_id?: never;
      readonly recommendation_key?: never;
    }
  | {
      readonly entry_id?: never;
      readonly cycle_id: string;
      readonly memory_revision_id?: never;
      readonly recommendation_key?: never;
    }
  | {
      readonly entry_id?: never;
      readonly cycle_id?: never;
      readonly memory_revision_id: string;
      readonly recommendation_key: string;
    };

export type PromoteRoomArtifactInput = RoomPromotionSource & {
  readonly kind: RoomPromotionKind;
  readonly idempotency_key: string;
  readonly title: string;
  readonly body?: string;
  readonly rationale?: string;
  readonly citation_entry_ids?: readonly string[];
};
