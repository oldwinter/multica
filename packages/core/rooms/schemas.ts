import { z } from "zod";
import type {
  Room,
  RoomArtifact,
  RoomCycle,
  RoomDetail,
  RoomEntry,
  RoomMessageResult,
  RoomMemoryRevision,
  RoomRecommendationReview,
  RoomMemory,
  RoomParticipant,
  RoomPreflight,
  RoomSynthesis,
  RoomTurn,
  RoomUsage,
  RetryRoomSynthesisResult,
  ReviewRoomCycleResult,
  CancelRoomCycleResult,
  ReviewRoomRecommendationResult,
  RoomWakeResult,
} from "./types";

const RoomStatusSchema = z
  .enum(["active", "paused", "archived"])
  .or(z.string().transform(() => "unknown" as const));
const RoomParticipantTypeSchema = z
  .enum(["agent", "member"])
  .or(z.string().transform(() => "unknown" as const));
const RoomParticipantRoleSchema = z
  .enum(["facilitator", "participant", "observer"])
  .or(z.string().transform(() => "participant" as const));
const RoomEntryTypeSchema = z
  .enum(["message", "result", "system"])
  .or(z.string().transform(() => "unknown" as const));
const RoomAuthorTypeSchema = z
  .enum(["member", "agent", "system"])
  .or(z.string().transform(() => "unknown" as const));
const RoomCycleSourceSchema = z
  .enum(["message", "mention", "manual", "schedule", "agent"])
  .or(z.string().transform(() => "unknown" as const));
const RoomCycleStatusSchema = z
  .enum(["refused", "queued", "running", "completed", "failed", "cancelled"])
  .or(z.string().transform(() => "unknown" as const));
const RoomCyclePhaseSchema = z
  .enum(["gathering", "synthesizing", "awaiting_review", "completed", "failed", "cancelled", "refused"])
  .or(z.string().transform(() => "unknown" as const));
const RoomTurnStatusSchema = z
  .enum(["refused", "queued", "dispatched", "running", "completed", "failed", "cancelled"])
  .or(z.string().transform(() => "unknown" as const));
const RoomTurnKindSchema = z
  .enum(["participant", "synthesis"])
  .or(z.string().transform(() => "unknown" as const));
const RoomRefusalReasonSchema = z
  .enum([
    "room_paused",
    "room_archived",
    "budget_exhausted",
    "active_cycle",
    "agent_unavailable",
    "invocation_not_allowed",
  ])
  .or(z.string().transform(() => "unknown" as const));
const RoomTemplateIdSchema = z
  .enum(["research", "planning", "risk", "incident", "decision", "improvement"])
  .or(z.string().transform(() => "unknown" as const));
const RoomRecommendationKindSchema = z
  .enum([
    "knowledge",
    "preference",
    "constraint",
    "executable_procedure",
    "implementation_defect",
    "decision",
    "unsupported",
  ])
  .or(z.string().transform(() => "unknown" as const));
const RoomArtifactKindSchema = z
  .enum([
    "issue",
    "wiki",
    "knowledge",
    "preference",
    "constraint",
    "executable_procedure",
    "implementation_defect",
    "decision",
    "unsupported",
  ])
  .or(z.string().transform(() => "unknown" as const));
const RoomMemoryReviewStatusSchema = z
  .enum(["pending", "accepted", "rejected", "corrected"])
  .or(z.string().transform(() => "unknown" as const));

export const RoomSynthesisItemSchema = z.object({
  text: z.string(),
  citation_entry_ids: z.array(z.string()).optional().default([]),
  confidence: z.number().min(0).max(1).catch(0),
}).loose();

export const RoomRecommendationSchema = z.object({
  key: z.string(),
  kind: RoomRecommendationKindSchema,
  title: z.string(),
  body: z.string().optional().default(""),
  rationale: z.string().optional().default(""),
  citation_entry_ids: z.array(z.string()).optional().default([]),
  confidence: z.number().min(0).max(1).catch(0),
}).loose();

export const RoomSynthesisSchema = z.object({
  schema_version: z.literal(1),
  summary: z.string(),
  facts: z.array(RoomSynthesisItemSchema).optional().default([]),
  decisions: z.array(RoomSynthesisItemSchema).optional().default([]),
  open_questions: z.array(RoomSynthesisItemSchema).optional().default([]),
  disagreements: z.array(RoomSynthesisItemSchema).optional().default([]),
  action_items: z.array(RoomSynthesisItemSchema).optional().default([]),
  recommendations: z.array(RoomRecommendationSchema).optional().default([]),
  confidence: z.number().min(0).max(1).catch(0),
}).loose();

export const RoomMemorySchema = z.object({
  summary: z.string().optional().default(""),
  facts: z.array(z.string()).optional().default([]),
  decisions: z.array(z.string()).optional().default([]),
  open_questions: z.array(z.string()).optional().default([]),
  recent_contributions: z.array(z.object({
    agent_id: z.string(),
    turn_id: z.string(),
    body: z.string(),
    at: z.string(),
  }).loose()).optional().default([]),
}).loose();

export const RoomValueSignalSchema = z.object({
	last_accepted_revision_id: z.string().nullable().optional().default(null),
	last_accepted_at: z.string().nullable().optional().default(null),
	last_cycle_id: z.string().nullable().optional().default(null),
	last_run_status: RoomCycleStatusSchema.nullable().optional().default(null),
	last_run_phase: RoomCyclePhaseSchema.nullable().optional().default(null),
	last_run_reason: z.string().nullable().optional().default(null),
	last_run_at: z.string().nullable().optional().default(null),
	last_run_cost_ticks: z.number().int().nonnegative().optional().default(0),
	repeat_run_count: z.number().int().nonnegative().optional().default(0),
	accepted_outcomes: z.number().int().nonnegative().optional().default(0),
	active_weeks: z.number().int().nonnegative().optional().default(0),
	accepted_outcomes_per_active_week: z.number().nonnegative().optional().default(0),
	median_review_latency_seconds: z.number().nonnegative().optional().default(0),
	promotion_rate: z.number().nonnegative().optional().default(0),
	failed_cycles: z.number().int().nonnegative().optional().default(0),
	refused_cycles: z.number().int().nonnegative().optional().default(0),
}).loose();

export const RoomSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  title: z.string(),
  instructions: z.string().optional().default(""),
  objective: z.string().optional().default(""),
  success_criteria: z.array(z.string()).optional().default([]),
  stop_conditions: z.array(z.string()).optional().default([]),
  template_id: RoomTemplateIdSchema.nullable().optional().default(null),
  created_by_user_id: z.string(),
  facilitator_agent_id: z.string(),
  facilitator_squad_id: z.string().nullable().optional().default(null),
  status: RoomStatusSchema,
  daily_turn_limit: z.number().nullable().optional().default(null),
  max_cost_ticks: z.number().nullable().optional().default(null),
  schedule_interval_minutes: z.number().nullable().optional().default(null),
  next_wake_at: z.string().nullable().optional().default(null),
  active_cycle_id: z.string().nullable().optional().default(null),
  memory: RoomMemorySchema.optional().default({
    summary: "",
    facts: [],
    decisions: [],
    open_questions: [],
    recent_contributions: [],
  }),
  memory_version: z.number().optional().default(0),
  accepted_memory_revision_id: z.string().nullable().optional().default(null),
  capability_version: z.number().int().nonnegative().optional().default(0),
  created_at: z.string(),
  updated_at: z.string(),
	value: RoomValueSignalSchema.nullable().optional().default(null),
}).loose();

export const RoomParticipantSchema = z.object({
  id: z.string(),
  type: RoomParticipantTypeSchema,
  participant_id: z.string(),
  role: RoomParticipantRoleSchema.optional().default("participant"),
  source_squad_id: z.string().nullable().optional().default(null),
  joined_at: z.string(),
}).loose();

export const RoomEntrySchema = z.object({
  id: z.string(),
  cycle_id: z.string().nullable().optional().default(null),
  turn_id: z.string().nullable().optional().default(null),
  ordinal: z.number(),
  type: RoomEntryTypeSchema,
  author_type: RoomAuthorTypeSchema,
  author_id: z.string().nullable().optional().default(null),
  body: z.string(),
  mentions: z.array(z.string()).optional().default([]),
  created_at: z.string(),
}).loose();

export const RoomCycleSchema = z.object({
  id: z.string(),
  sequence: z.number(),
  source: RoomCycleSourceSchema,
  wake_key: z.string(),
  triggering_entry_id: z.string().nullable().optional().default(null),
  status: RoomCycleStatusSchema,
  phase: RoomCyclePhaseSchema.optional().default("unknown"),
  refusal_reason: RoomRefusalReasonSchema.nullable().optional().default(null),
  synthesis_error: z.object({
    code: z.string(),
    message: z.string(),
    retryable: z.boolean(),
  }).loose().nullable().optional().default(null),
  synthesis_turn_id: z.string().nullable().optional().default(null),
  memory_revision_id: z.string().nullable().optional().default(null),
  expected_max_turns: z.number().int().nonnegative().optional().default(0),
  cost_limit_ticks: z.number().int().positive().nullable().optional().default(null),
  planned_at: z.string().nullable().optional().default(null),
  created_at: z.string(),
  started_at: z.string().nullable().optional().default(null),
  completed_at: z.string().nullable().optional().default(null),
}).loose();

export const RoomTurnSchema = z.object({
  id: z.string(),
  cycle_id: z.string(),
  agent_id: z.string(),
  squad_id: z.string().nullable().optional().default(null),
  status: RoomTurnStatusSchema,
  turn_kind: RoomTurnKindSchema.optional().default("participant"),
  attempt: z.number().int().positive().optional().default(1),
  refusal_reason: RoomRefusalReasonSchema.nullable().optional().default(null),
  created_at: z.string(),
  started_at: z.string().nullable().optional().default(null),
  completed_at: z.string().nullable().optional().default(null),
}).loose();

export const RoomArtifactSchema = z.object({
  id: z.string(),
  cycle_id: z.string().nullable().optional().default(null),
  turn_id: z.string().nullable().optional().default(null),
  entry_id: z.string().nullable().optional().default(null),
  kind: RoomArtifactKindSchema,
  target_id: z.string().nullable().optional().default(null),
  title: z.string(),
  body: z.string(),
  rationale: z.string().nullable().optional().default(null),
  memory_revision_id: z.string().nullable().optional().default(null),
  recommendation_key: z.string().nullable().optional().default(null),
  citation_entry_ids: z.array(z.string()).optional().default([]),
  created_by_user_id: z.string(),
  created_at: z.string(),
}).loose();

export const RoomMemoryRevisionSchema = z.object({
  id: z.string(),
  room_id: z.string(),
  cycle_id: z.string(),
  synthesis_turn_id: z.string(),
  version: z.number().int().nonnegative(),
  schema_version: z.number().int().positive(),
  synthesis: RoomSynthesisSchema,
  digest: z.string(),
  creator_type: z.enum(["member", "agent"]),
  creator_id: z.string(),
  review_status: RoomMemoryReviewStatusSchema,
  reviewed_by_user_id: z.string().nullable().optional().default(null),
  reviewed_at: z.string().nullable().optional().default(null),
  corrected_from_revision_id: z.string().nullable().optional().default(null),
  created_at: z.string(),
}).loose();

export const RoomRecommendationReviewSchema = z.object({
  id: z.string(),
  room_id: z.string(),
  memory_revision_id: z.string(),
  recommendation_key: z.string(),
  status: z.enum(["approved", "rejected"]).or(
    z.string().transform(() => "unknown" as const),
  ),
  artifact_id: z.string().nullable().optional().default(null),
  reviewed_by_user_id: z.string(),
  reviewed_at: z.string(),
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

export const RoomPreflightSchema = z.object({
  source: z
    .enum(["manual", "schedule"])
    .or(z.string().transform(() => "unknown" as const))
    .optional()
    .default("unknown"),
  allowed: z.boolean(),
  refusal_reason: z.string().nullable().optional().default(null),
  capability_version: z.number().int().nonnegative().optional().default(0),
  capability_ready: z.boolean().optional().default(true),
  spend_limit_supported: z.boolean().optional().default(true),
  required_daemon_capability: z.string().optional().default(""),
  required_cost_capability: z.string().optional().default(""),
  target_agents: z.array(z.object({
    agent_id: z.string(),
    ready: z.boolean(),
    invocation_allowed: z.boolean(),
    reason: z.string().nullable().optional().default(null),
  }).loose()).optional().default([]),
  expected_max_turns: z.number().int().nonnegative(),
  synthesis_required: z.boolean(),
  budget: z.object({
    daily_turn_limit: z.number().nullable().optional().default(null),
    used_turns: z.number().int().nonnegative(),
    max_cost_ticks: z.number().nullable().optional().default(null),
    used_cost_ticks: z.number().int().nonnegative(),
    remaining_cost_ticks: z.number().nullable().optional().default(null),
    reserved_cost_ticks: z.number().int().nonnegative().optional().default(0),
    uncosted_turns: z.number().int().nonnegative(),
  }).loose(),
}).loose();

export const RoomUsageSchema = z.object({
  turns_total: z.number().int().nonnegative(),
  cost_ticks: z.number().int().nonnegative(),
  uncosted_turns: z.number().int().nonnegative(),
  failures: z.number().int().nonnegative(),
  accepted_syntheses: z.number().int().nonnegative(),
  promoted_artifacts: z.number().int().nonnegative(),
	repeat_run_count: z.number().int().nonnegative().optional().default(0),
	active_weeks: z.number().int().nonnegative().optional().default(0),
	median_review_latency_seconds: z.number().nonnegative().optional().default(0),
	accepted_outcomes_per_active_week: z.number().nonnegative().optional().default(0),
	promotion_rate: z.number().nonnegative().optional().default(0),
	failed_cycles: z.number().int().nonnegative().optional().default(0),
	refused_cycles: z.number().int().nonnegative().optional().default(0),
	cost_ticks_per_accepted_outcome: z.number().nonnegative().optional().default(0),
}).loose();

export const RetryRoomSynthesisResultSchema = z.object({
  cycle: RoomCycleSchema,
  turn: RoomTurnSchema,
  task_id: z.string(),
}).loose();

export const ReviewRoomCycleResultSchema = z.object({
  room: RoomSchema,
  memory_revision: RoomMemoryRevisionSchema,
}).loose();

export const CancelRoomCycleResultSchema = z.object({
  cycle: RoomCycleSchema,
}).loose();

export const ReviewRoomRecommendationResultSchema = z.object({
  recommendation_review: RoomRecommendationReviewSchema,
}).loose();

export const RoomWakeResultSchema = z.object({
  cycle: RoomCycleSchema,
  turns: z.array(RoomTurnSchema).optional().default([]),
  tasks: z.array(z.string()).optional().default([]),
}).loose();

export const RoomMessageResultSchema = RoomWakeResultSchema.extend({
  entry: RoomEntrySchema,
}).loose();

export const RoomListSchema = z.array(RoomSchema);

export const EMPTY_ROOM_MEMORY: RoomMemory = {
  summary: "",
  facts: [],
  decisions: [],
  open_questions: [],
  recent_contributions: [],
};

export const EMPTY_ROOM: Room = {
  id: "",
  workspace_id: "",
  title: "",
  instructions: "",
  objective: "",
  success_criteria: [],
  stop_conditions: [],
  template_id: null,
  created_by_user_id: "",
  facilitator_agent_id: "",
  facilitator_squad_id: null,
  status: "unknown",
  daily_turn_limit: null,
  max_cost_ticks: null,
  schedule_interval_minutes: null,
  next_wake_at: null,
  active_cycle_id: null,
  memory: EMPTY_ROOM_MEMORY,
  memory_version: 0,
  accepted_memory_revision_id: null,
  capability_version: 0,
  created_at: "",
  updated_at: "",
	value: null,
};

export const EMPTY_ROOM_LIST: readonly Room[] = [];

export const EMPTY_ROOM_DETAIL: RoomDetail = {
  room: EMPTY_ROOM,
  participants: [],
  entries: [],
  cycles: [],
  turns: [],
  artifacts: [],
  memory_revisions: [],
  recommendation_reviews: [],
};

const EMPTY_ROOM_CYCLE: RoomCycle = {
  id: "",
  sequence: 0,
  source: "unknown",
  wake_key: "",
  triggering_entry_id: null,
  status: "unknown",
  phase: "unknown",
  refusal_reason: null,
  synthesis_error: null,
  synthesis_turn_id: null,
  memory_revision_id: null,
  expected_max_turns: 0,
  cost_limit_ticks: null,
  planned_at: null,
  created_at: "",
  started_at: null,
  completed_at: null,
};

const EMPTY_ROOM_TURN: RoomTurn = {
  id: "",
  cycle_id: "",
  agent_id: "",
  squad_id: null,
  status: "unknown",
  turn_kind: "participant",
  attempt: 1,
  refusal_reason: null,
  created_at: "",
  started_at: null,
  completed_at: null,
};

export const EMPTY_ROOM_SYNTHESIS: RoomSynthesis = {
  schema_version: 1,
  summary: "",
  facts: [],
  decisions: [],
  open_questions: [],
  disagreements: [],
  action_items: [],
  recommendations: [],
  confidence: 0,
};

export const EMPTY_ROOM_PREFLIGHT: RoomPreflight = {
  source: "unknown",
  allowed: false,
  refusal_reason: null,
  capability_version: 0,
  capability_ready: true,
  spend_limit_supported: true,
  required_daemon_capability: "",
  required_cost_capability: "",
  target_agents: [],
  expected_max_turns: 0,
  synthesis_required: false,
  budget: {
    daily_turn_limit: null,
    used_turns: 0,
    max_cost_ticks: null,
    used_cost_ticks: 0,
    remaining_cost_ticks: null,
    reserved_cost_ticks: 0,
    uncosted_turns: 0,
  },
};

export const EMPTY_ROOM_USAGE: RoomUsage = {
  turns_total: 0,
  cost_ticks: 0,
  uncosted_turns: 0,
  failures: 0,
  accepted_syntheses: 0,
  promoted_artifacts: 0,
	repeat_run_count: 0,
	active_weeks: 0,
	median_review_latency_seconds: 0,
	accepted_outcomes_per_active_week: 0,
	promotion_rate: 0,
	failed_cycles: 0,
	refused_cycles: 0,
	cost_ticks_per_accepted_outcome: 0,
};

export const EMPTY_ROOM_WAKE_RESULT: RoomWakeResult = {
  cycle: EMPTY_ROOM_CYCLE,
  turns: [],
  tasks: [],
};

const EMPTY_ROOM_ENTRY: RoomEntry = {
  id: "",
  cycle_id: null,
  turn_id: null,
  ordinal: 0,
  type: "unknown",
  author_type: "unknown",
  author_id: null,
  body: "",
  mentions: [],
  created_at: "",
};

export const EMPTY_ROOM_MESSAGE_RESULT: RoomMessageResult = {
  ...EMPTY_ROOM_WAKE_RESULT,
  entry: EMPTY_ROOM_ENTRY,
};

export const EMPTY_ROOM_ARTIFACT: RoomArtifact = {
  id: "",
  cycle_id: null,
  turn_id: null,
  entry_id: null,
  kind: "decision",
  target_id: null,
  title: "",
  body: "",
  rationale: null,
  memory_revision_id: null,
  recommendation_key: null,
  citation_entry_ids: [],
  created_by_user_id: "",
  created_at: "",
};

const EMPTY_ROOM_MEMORY_REVISION: RoomMemoryRevision = {
  id: "",
  room_id: "",
  cycle_id: "",
  synthesis_turn_id: "",
  version: 0,
  schema_version: 1,
  synthesis: EMPTY_ROOM_SYNTHESIS,
  digest: "",
  creator_type: "agent",
  creator_id: "",
  review_status: "pending",
  reviewed_by_user_id: null,
  reviewed_at: null,
  corrected_from_revision_id: null,
  created_at: "",
};

const EMPTY_ROOM_RECOMMENDATION_REVIEW: RoomRecommendationReview = {
  id: "",
  room_id: "",
  memory_revision_id: "",
  recommendation_key: "",
  status: "unknown",
  artifact_id: null,
  reviewed_by_user_id: "",
  reviewed_at: "",
};

export const EMPTY_RETRY_ROOM_SYNTHESIS_RESULT: RetryRoomSynthesisResult = {
  cycle: EMPTY_ROOM_CYCLE,
  turn: EMPTY_ROOM_TURN,
  task_id: "",
};

export const EMPTY_REVIEW_ROOM_CYCLE_RESULT: ReviewRoomCycleResult = {
  room: EMPTY_ROOM,
  memory_revision: EMPTY_ROOM_MEMORY_REVISION,
};

export const EMPTY_CANCEL_ROOM_CYCLE_RESULT: CancelRoomCycleResult = {
  cycle: EMPTY_ROOM_CYCLE,
};

export const EMPTY_REVIEW_ROOM_RECOMMENDATION_RESULT: ReviewRoomRecommendationResult = {
  recommendation_review: EMPTY_ROOM_RECOMMENDATION_REVIEW,
};

export type {
  Room,
  RoomArtifact,
  RoomCycle,
  RoomDetail,
  RoomEntry,
  RoomMessageResult,
  RoomMemoryRevision,
  RoomRecommendationReview,
  RoomParticipant,
  RoomPreflight,
  RoomSynthesis,
  RoomTurn,
  RoomUsage,
  RetryRoomSynthesisResult,
  ReviewRoomCycleResult,
  CancelRoomCycleResult,
  ReviewRoomRecommendationResult,
  RoomWakeResult,
};
