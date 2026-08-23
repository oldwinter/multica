export type RoomStatus = "active" | "paused" | "archived" | "unknown";

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

export type RoomReviewStatus =
  | "pending"
  | "accepted"
  | "rejected"
  | "corrected"
  | "unknown";

export type RoomPromotionKind = "issue" | "wiki" | "decision";
export type RoomRecommendationReviewAction = "approved" | "rejected" | "unknown";

export interface RoomMemory {
  readonly summary: string;
  readonly facts: readonly string[];
  readonly decisions: readonly string[];
  readonly open_questions: readonly string[];
}

export interface RoomSynthesisItem {
  readonly text: string;
  readonly citation_entry_ids: readonly string[];
  readonly confidence: number | null;
}

export interface RoomRecommendation {
  readonly key: string;
  readonly kind: RoomPromotionKind | "unknown";
  readonly title: string;
  readonly body: string;
  readonly rationale: string;
  readonly citation_entry_ids: readonly string[];
  readonly confidence: number | null;
}

export interface RoomSynthesis {
  readonly schema_version: number;
  readonly summary: string;
  readonly facts: readonly RoomSynthesisItem[];
  readonly decisions: readonly RoomSynthesisItem[];
  readonly open_questions: readonly RoomSynthesisItem[];
  readonly disagreements: readonly RoomSynthesisItem[];
  readonly action_items: readonly RoomSynthesisItem[];
  readonly recommendations: readonly RoomRecommendation[];
  readonly confidence: number | null;
}

export interface Room {
  readonly id: string;
  readonly workspace_id: string;
  readonly title: string;
  readonly instructions: string;
  readonly objective: string;
  readonly success_criteria: readonly string[];
  readonly stop_conditions: readonly string[];
  readonly template_id: string | null;
  readonly created_by_user_id: string;
  readonly facilitator_agent_id: string;
  readonly facilitator_squad_id: string | null;
  readonly status: RoomStatus;
  readonly daily_turn_limit: number | null;
  readonly max_cost_ticks: number | null;
  readonly schedule_interval_minutes: number | null;
  readonly next_wake_at: string | null;
  readonly active_cycle_id: string | null;
  readonly accepted_memory_revision_id: string | null;
  readonly capability_version: number;
  readonly memory: RoomMemory;
  readonly memory_version: number;
  readonly revision: number | null;
  readonly created_at: string;
  readonly updated_at: string;
}

export interface RoomParticipant {
  readonly id: string;
  readonly type: "agent" | "member" | "unknown";
  readonly participant_id: string;
  readonly role: "facilitator" | "participant" | "observer" | "unknown";
  readonly source_squad_id: string | null;
  readonly joined_at: string;
}

export interface RoomEntry {
  readonly id: string;
  readonly cycle_id: string | null;
  readonly turn_id: string | null;
  readonly ordinal: number;
  readonly type: "message" | "result" | "system" | "unknown";
  readonly author_type: "member" | "agent" | "system" | "unknown";
  readonly author_id: string | null;
  readonly body: string;
  readonly mentions: readonly string[];
  readonly created_at: string;
}

export interface RoomCycle {
  readonly id: string;
  readonly sequence: number;
  readonly source: "message" | "mention" | "manual" | "schedule" | "agent" | "unknown";
  readonly status: RoomCycleStatus;
  readonly phase: RoomCyclePhase;
  readonly refusal_reason: string | null;
  readonly synthesis_error: {
    readonly code: string;
    readonly message: string;
    readonly retryable: boolean;
  } | null;
  readonly synthesis_turn_id: string | null;
  readonly memory_revision_id: string | null;
  readonly expected_max_turns: number | null;
	readonly cost_limit_ticks: number | null;
  readonly planned_at: string | null;
  readonly created_at: string;
  readonly started_at: string | null;
  readonly completed_at: string | null;
  readonly revision: number | null;
}

export interface RoomTurn {
  readonly id: string;
  readonly cycle_id: string;
  readonly agent_id: string;
  readonly squad_id: string | null;
  readonly turn_kind: "participant" | "synthesis" | "unknown";
  readonly attempt: number;
  readonly status: RoomCycleStatus | "dispatched";
  readonly refusal_reason: string | null;
  readonly created_at: string;
  readonly started_at: string | null;
  readonly completed_at: string | null;
}

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
  readonly review_status: RoomReviewStatus;
  readonly reviewed_by_user_id: string | null;
  readonly reviewed_at: string | null;
  readonly corrected_from_revision_id: string | null;
  readonly created_at: string;
}

export interface RoomRecommendationReview {
  readonly id: string;
  readonly memory_revision_id: string;
  readonly recommendation_key: string;
  readonly status: RoomRecommendationReviewAction;
  readonly reviewed_by_user_id: string | null;
  readonly artifact_id: string | null;
  readonly reviewed_at: string;
}

export interface RoomArtifact {
  readonly id: string;
  readonly cycle_id: string | null;
  readonly turn_id: string | null;
  readonly entry_id: string | null;
  readonly memory_revision_id: string | null;
  readonly recommendation_key: string | null;
  readonly kind: RoomPromotionKind | "unknown";
  readonly target_id: string | null;
  readonly title: string;
  readonly body: string;
  readonly rationale: string | null;
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

export interface RoomWakeResult {
  readonly cycle: RoomCycle;
  readonly turns: readonly RoomTurn[];
  readonly tasks: readonly string[];
}

export interface RoomMessageResult extends RoomWakeResult {
  readonly entry: RoomEntry;
}

export interface RoomRetrySynthesisResult {
  readonly cycle: RoomCycle;
  readonly turn: RoomTurn;
  readonly task_id: string;
}

export interface RoomPreflightAgent {
  readonly agent_id: string;
  readonly ready: boolean;
  readonly invocation_allowed: boolean;
  readonly reason: string | null;
}

export interface RoomPreflightBudget {
  readonly daily_turn_limit: number | null;
  readonly used_turns: number;
  readonly max_cost_ticks: number | null;
  readonly used_cost_ticks: number;
  readonly remaining_cost_ticks: number | null;
	readonly reserved_cost_ticks: number;
  readonly uncosted_turns: number;
}

export interface RoomPreflight {
  readonly room_id: string;
  readonly source: "manual" | "schedule" | "unknown";
  readonly target_agents: readonly RoomPreflightAgent[];
  readonly expected_max_turns: number;
  readonly synthesis_required: boolean;
  readonly capability_version: number;
  readonly capability_ready: boolean;
  readonly required_daemon_capability: string;
	readonly spend_limit_supported: boolean;
	readonly required_cost_capability: string;
  readonly allowed: boolean;
  readonly refusal_reason: string | null;
  readonly budget: RoomPreflightBudget;
}

export interface RoomUsage {
  readonly room_id: string;
  readonly turns_total: number;
  readonly cost_ticks: number;
  readonly uncosted_turns: number;
  readonly failures: number;
  readonly accepted_syntheses: number;
  readonly promoted_artifacts: number;
}

export interface PostRoomMessageInput {
  readonly body: string;
  readonly idempotency_key: string;
}

export interface ReviewRoomSynthesisInput {
  readonly action: "accept" | "reject" | "correct";
  readonly expected_memory_version: number;
  readonly correction?: RoomSynthesis;
  readonly idempotency_key: string;
}

export interface PromoteRoomRecommendationInput {
  readonly kind: RoomPromotionKind;
  readonly memory_revision_id: string;
  readonly recommendation_key: string;
  readonly idempotency_key: string;
  readonly title: string;
  readonly body: string;
  readonly rationale?: string;
  readonly citation_entry_ids: readonly string[];
}

export type RoomConflictCode =
  | "room_paused"
  | "room_archived"
  | "active_cycle"
  | "budget_exhausted"
  | "agent_unavailable"
  | "invocation_not_allowed"
  | "stale_review"
  | "idempotency_conflict"
  | "synthesis_not_retryable"
  | "promotion_source_mismatch"
  | "recommendation_already_reviewed";
