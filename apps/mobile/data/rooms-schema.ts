import { z } from "zod";
import type {
  Room,
  RoomArtifact,
  RoomDetail,
  RoomMemory,
  RoomMessageResult,
  RoomPreflight,
  RoomRecommendationReview,
  RoomRetrySynthesisResult,
  RoomUsage,
  RoomWakeResult,
} from "./rooms-types";

function unknownEnum<const T extends string>(
  values: readonly [T, ...T[]],
): z.ZodType<T | "unknown"> {
  const allowed = new Set<string>(values);
  return z.string().transform((value): T | "unknown" =>
    allowed.has(value) ? (value as T) : "unknown",
  );
}

const nullableString = z.string().nullable().optional().default(null);
const nullableNumber = z.number().nullable().optional().default(null);

export const RoomPreflightBudgetSchema = z.object({
  daily_turn_limit: nullableNumber,
  used_turns: z.number().optional().default(0),
  max_cost_ticks: nullableNumber,
  used_cost_ticks: z.number().optional().default(0),
  remaining_cost_ticks: nullableNumber,
	reserved_cost_ticks: z.number().optional().default(0),
  uncosted_turns: z.number().optional().default(0),
}).loose();

export const RoomMemorySchema = z.object({
  summary: z.string().optional().default(""),
  facts: z.array(z.string()).optional().default([]),
  decisions: z.array(z.string()).optional().default([]),
  open_questions: z.array(z.string()).optional().default([]),
}).loose();

export const EMPTY_ROOM_MEMORY: RoomMemory = {
  summary: "",
  facts: [],
  decisions: [],
  open_questions: [],
};

export const RoomSynthesisItemSchema = z.object({
  text: z.string().optional().default(""),
  citation_entry_ids: z.array(z.string()).optional().default([]),
  confidence: nullableNumber,
}).loose();

export const RoomRecommendationSchema = z.object({
  key: z.string().optional().default(""),
  kind: unknownEnum(["issue", "wiki", "decision"]),
  title: z.string().optional().default(""),
  body: z.string().optional().default(""),
  rationale: z.string().optional().default(""),
  citation_entry_ids: z.array(z.string()).optional().default([]),
  confidence: nullableNumber,
}).loose();

export const RoomSynthesisSchema = z.object({
  schema_version: z.number().optional().default(1),
  summary: z.string().optional().default(""),
  facts: z.array(RoomSynthesisItemSchema).optional().default([]),
  decisions: z.array(RoomSynthesisItemSchema).optional().default([]),
  open_questions: z.array(RoomSynthesisItemSchema).optional().default([]),
  disagreements: z.array(RoomSynthesisItemSchema).optional().default([]),
  action_items: z.array(RoomSynthesisItemSchema).optional().default([]),
  recommendations: z.array(RoomRecommendationSchema).optional().default([]),
  confidence: nullableNumber,
}).loose();

export const RoomValueSignalSchema = z.object({
  last_accepted_revision_id: nullableString,
  last_accepted_at: nullableString,
  last_cycle_id: nullableString,
  last_run_status: unknownEnum([
    "refused",
    "queued",
    "running",
    "completed",
    "failed",
    "cancelled",
  ]).nullable().optional().default(null),
  last_run_phase: unknownEnum([
    "gathering",
    "synthesizing",
    "awaiting_review",
    "completed",
    "failed",
    "cancelled",
    "refused",
  ]).nullable().optional().default(null),
  last_run_reason: nullableString,
  last_run_at: nullableString,
  last_run_cost_ticks: z.number().optional().default(0),
  repeat_run_count: z.number().optional().default(0),
  accepted_outcomes: z.number().optional().default(0),
  active_weeks: z.number().optional().default(0),
  accepted_outcomes_per_active_week: z.number().optional().default(0),
  median_review_latency_seconds: z.number().optional().default(0),
  promotion_rate: z.number().optional().default(0),
  failed_cycles: z.number().optional().default(0),
  refused_cycles: z.number().optional().default(0),
}).loose();

export const RoomSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  title: z.string().optional().default(""),
  instructions: z.string().optional().default(""),
  objective: z.string().optional().default(""),
  success_criteria: z.array(z.string()).optional().default([]),
  stop_conditions: z.array(z.string()).optional().default([]),
  template_id: nullableString,
  created_by_user_id: z.string().optional().default(""),
  facilitator_agent_id: z.string().optional().default(""),
  facilitator_squad_id: nullableString,
  status: unknownEnum(["active", "paused", "archived"]),
  daily_turn_limit: nullableNumber,
  max_cost_ticks: nullableNumber,
  schedule_interval_minutes: nullableNumber,
  next_wake_at: nullableString,
  active_cycle_id: nullableString,
  accepted_memory_revision_id: nullableString,
  capability_version: z.number().optional().default(1),
  memory: RoomMemorySchema.optional().default({
    summary: "",
    facts: [],
    decisions: [],
    open_questions: [],
  }),
  memory_version: z.number().optional().default(0),
  revision: nullableNumber,
  created_at: z.string().optional().default(""),
  updated_at: z.string().optional().default(""),
  value: RoomValueSignalSchema.nullable().optional().default(null),
}).loose();

export const RoomParticipantSchema = z.object({
  id: z.string(),
  type: unknownEnum(["agent", "member"]),
  participant_id: z.string().optional().default(""),
  role: unknownEnum(["facilitator", "participant", "observer"]),
  source_squad_id: nullableString,
  joined_at: z.string().optional().default(""),
}).loose();

export const RoomEntrySchema = z.object({
  id: z.string(),
  cycle_id: nullableString,
  turn_id: nullableString,
  ordinal: z.number().optional().default(0),
  type: unknownEnum(["message", "result", "system"]),
  author_type: unknownEnum(["member", "agent", "system"]),
  author_id: nullableString,
  body: z.string().optional().default(""),
  mentions: z.array(z.string()).optional().default([]),
  created_at: z.string().optional().default(""),
}).loose();

export const RoomCycleSchema = z.object({
  id: z.string(),
  sequence: z.number().optional().default(0),
  source: unknownEnum(["message", "mention", "manual", "schedule", "agent"]),
  status: unknownEnum(["refused", "queued", "running", "completed", "failed", "cancelled"]),
  phase: unknownEnum(["gathering", "synthesizing", "awaiting_review", "completed", "failed", "cancelled", "refused"]),
  refusal_reason: nullableString,
  synthesis_error: z.object({
    code: z.string().optional().default("unknown"),
    message: z.string().optional().default(""),
    retryable: z.boolean().optional().default(false),
  }).loose().nullable().optional().default(null),
  synthesis_turn_id: nullableString,
  memory_revision_id: nullableString,
  expected_max_turns: nullableNumber,
	cost_limit_ticks: nullableNumber,
  planned_at: nullableString,
  created_at: z.string().optional().default(""),
  started_at: nullableString,
  completed_at: nullableString,
  revision: nullableNumber,
}).loose();

export const RoomTurnSchema = z.object({
  id: z.string(),
  cycle_id: z.string().optional().default(""),
  agent_id: z.string().optional().default(""),
  squad_id: nullableString,
  turn_kind: unknownEnum(["participant", "synthesis"]),
  attempt: z.number().int().positive().optional().default(1),
  status: unknownEnum(["refused", "queued", "dispatched", "running", "completed", "failed", "cancelled"]),
  refusal_reason: nullableString,
  created_at: z.string().optional().default(""),
  started_at: nullableString,
  completed_at: nullableString,
}).loose();

export const RoomMemoryRevisionSchema = z.object({
  id: z.string(),
  room_id: z.string().optional().default(""),
  cycle_id: z.string().optional().default(""),
  synthesis_turn_id: z.string().optional().default(""),
  version: z.number().optional().default(0),
  schema_version: z.number().optional().default(1),
  synthesis: RoomSynthesisSchema.optional().default({
    schema_version: 1,
    summary: "",
    facts: [],
    decisions: [],
    open_questions: [],
    disagreements: [],
    action_items: [],
    recommendations: [],
    confidence: null,
  }),
  digest: z.string().optional().default(""),
  creator_type: z.enum(["member", "agent"]),
  creator_id: z.string(),
  review_status: unknownEnum(["pending", "accepted", "rejected", "corrected"]),
  reviewed_by_user_id: nullableString,
  reviewed_at: nullableString,
  corrected_from_revision_id: nullableString,
  created_at: z.string().optional().default(""),
}).loose();

export const RoomRecommendationReviewSchema = z.object({
  id: z.string().optional().default(""),
  memory_revision_id: z.string(),
  recommendation_key: z.string(),
  status: unknownEnum(["approved", "rejected"]),
  reviewed_by_user_id: nullableString,
  artifact_id: nullableString,
  reviewed_at: z.string().optional().default(""),
}).loose();

export const RoomArtifactSchema = z.object({
  id: z.string(),
  cycle_id: nullableString,
  turn_id: nullableString,
  entry_id: nullableString,
  memory_revision_id: nullableString,
  recommendation_key: nullableString,
  kind: unknownEnum(["issue", "wiki", "decision"]),
  target_id: nullableString,
  title: z.string().optional().default(""),
  body: z.string().optional().default(""),
  rationale: nullableString,
  citation_entry_ids: z.array(z.string()).optional().default([]),
  created_by_user_id: z.string().optional().default(""),
  created_at: z.string().optional().default(""),
}).loose();

export const RoomDetailSchema = z.object({
  room: RoomSchema,
  participants: z.array(RoomParticipantSchema).optional().default([]),
  entries: z.array(RoomEntrySchema).optional().default([]),
  cycles: z.array(RoomCycleSchema).optional().default([]),
  turns: z.array(RoomTurnSchema).optional().default([]),
  artifacts: z.array(RoomArtifactSchema).optional().default([]),
  memory_revisions: z.array(RoomMemoryRevisionSchema).optional().default([]),
  recommendation_reviews: z.array(RoomRecommendationReviewSchema).optional().default([]),
}).loose();

export const RoomWakeResultSchema = z.object({
  cycle: RoomCycleSchema,
  turns: z.array(RoomTurnSchema).optional().default([]),
  tasks: z.array(z.string()).optional().default([]),
}).loose();

export const RoomMessageResultSchema = RoomWakeResultSchema.extend({
  entry: RoomEntrySchema,
}).loose();

export const RoomRetrySynthesisResultSchema = z.object({
  cycle: RoomCycleSchema,
  turn: RoomTurnSchema,
  task_id: z.string(),
}).loose();

export const RoomPreflightSchema = z.object({
  room_id: z.string().optional().default(""),
  source: unknownEnum(["manual", "schedule"]).optional().default("unknown"),
  target_agents: z.array(z.object({
    agent_id: z.string(),
    ready: z.boolean().optional().default(false),
    invocation_allowed: z.boolean().optional().default(false),
    reason: nullableString,
  }).loose()).optional().default([]),
  expected_max_turns: z.number().optional().default(0),
  synthesis_required: z.boolean().optional().default(false),
  capability_version: z.number().int().nonnegative().optional().default(0),
  capability_ready: z.boolean().optional().default(true),
  required_daemon_capability: z.string().optional().default(""),
	spend_limit_supported: z.boolean().optional().default(true),
	required_cost_capability: z.string().optional().default(""),
  allowed: z.boolean().optional().default(false),
  refusal_reason: nullableString,
  budget: RoomPreflightBudgetSchema.optional().default({
    daily_turn_limit: null,
    used_turns: 0,
    max_cost_ticks: null,
    used_cost_ticks: 0,
    remaining_cost_ticks: null,
		reserved_cost_ticks: 0,
    uncosted_turns: 0,
  }),
}).loose();

export const RoomUsageSchema = z.object({
  room_id: z.string().optional().default(""),
  turns_total: z.number().optional().default(0),
  cost_ticks: z.number().optional().default(0),
  uncosted_turns: z.number().optional().default(0),
  failures: z.number().optional().default(0),
  accepted_syntheses: z.number().optional().default(0),
  promoted_artifacts: z.number().optional().default(0),
  repeat_run_count: z.number().optional().default(0),
  active_weeks: z.number().optional().default(0),
  median_review_latency_seconds: z.number().optional().default(0),
  accepted_outcomes_per_active_week: z.number().optional().default(0),
  promotion_rate: z.number().optional().default(0),
  failed_cycles: z.number().optional().default(0),
  refused_cycles: z.number().optional().default(0),
  cost_ticks_per_accepted_outcome: z.number().optional().default(0),
}).loose();

export const RecommendationReviewResponseSchema = z.object({
  recommendation_review: RoomRecommendationReviewSchema,
}).loose();

export const RoomListSchema = z.array(RoomSchema).default([]);

export const EMPTY_ROOM: Room = RoomSchema.parse({ id: "", workspace_id: "", status: "unknown" });
export const EMPTY_ROOM_DETAIL: RoomDetail = RoomDetailSchema.parse({ room: EMPTY_ROOM });
export const EMPTY_ROOM_LIST: readonly Room[] = [];
export const EMPTY_ROOM_WAKE_RESULT: RoomWakeResult = RoomWakeResultSchema.parse({
  cycle: { id: "", status: "unknown", phase: "unknown", source: "unknown" },
});
export const EMPTY_ROOM_MESSAGE_RESULT: RoomMessageResult = RoomMessageResultSchema.parse({
  ...EMPTY_ROOM_WAKE_RESULT,
  entry: { id: "", type: "unknown", author_type: "unknown" },
});
export const EMPTY_ROOM_RETRY_SYNTHESIS_RESULT: RoomRetrySynthesisResult =
  RoomRetrySynthesisResultSchema.parse({
    cycle: { id: "", status: "unknown", phase: "unknown", source: "unknown" },
    turn: { id: "", cycle_id: "", turn_kind: "unknown", status: "unknown" },
    task_id: "",
  });
export const EMPTY_ROOM_ARTIFACT: RoomArtifact = RoomArtifactSchema.parse({ id: "", kind: "unknown" });
export const EMPTY_ROOM_PREFLIGHT: RoomPreflight = RoomPreflightSchema.parse({});
export const EMPTY_ROOM_USAGE: RoomUsage = RoomUsageSchema.parse({});

export type { RoomRecommendationReview };
