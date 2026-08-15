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

export type RoomTurnStatus = RoomCycleStatus | "dispatched";

export type RoomRefusalReason =
  | "room_paused"
  | "room_archived"
  | "budget_exhausted"
  | "cycle_active"
  | "no_targets"
  | "unknown";

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
  readonly created_by_user_id: string;
  readonly facilitator_agent_id: string;
  readonly facilitator_squad_id: string | null;
  readonly status: RoomStatus;
  readonly daily_turn_limit: number | null;
  readonly schedule_interval_minutes: number | null;
  readonly next_wake_at: string | null;
  readonly active_cycle_id: string | null;
  readonly memory: RoomMemory;
  readonly memory_version: number;
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
  readonly refusal_reason: RoomRefusalReason | null;
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
  readonly participants?: readonly RoomParticipantInput[];
  readonly daily_turn_limit?: number;
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

type RoomPromotionSource =
  | {
      readonly entry_id: string;
      readonly cycle_id?: never;
    }
  | {
      readonly entry_id?: never;
      readonly cycle_id: string;
    };

export type PromoteRoomArtifactInput = RoomPromotionSource & {
  readonly kind: RoomPromotionKind;
  readonly idempotency_key: string;
  readonly title: string;
  readonly rationale?: string;
};
