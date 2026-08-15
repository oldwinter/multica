import { z } from "zod";
import type {
  Room,
  RoomArtifact,
  RoomCycle,
  RoomDetail,
  RoomEntry,
  RoomMessageResult,
  RoomMemory,
  RoomParticipant,
  RoomTurn,
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
const RoomTurnStatusSchema = z
  .enum(["refused", "queued", "dispatched", "running", "completed", "failed", "cancelled"])
  .or(z.string().transform(() => "unknown" as const));
const RoomRefusalReasonSchema = z
  .enum(["room_paused", "room_archived", "budget_exhausted", "cycle_active", "no_targets"])
  .or(z.string().transform(() => "unknown" as const));

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

export const RoomSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  title: z.string(),
  instructions: z.string().optional().default(""),
  created_by_user_id: z.string(),
  facilitator_agent_id: z.string(),
  facilitator_squad_id: z.string().nullable().optional().default(null),
  status: RoomStatusSchema,
  daily_turn_limit: z.number().nullable().optional().default(null),
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
  created_at: z.string(),
  updated_at: z.string(),
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
  refusal_reason: RoomRefusalReasonSchema.nullable().optional().default(null),
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
  kind: z.enum(["issue", "wiki", "decision"]).or(
    z.string().transform(() => "unknown" as const),
  ),
  target_id: z.string().nullable().optional().default(null),
  title: z.string(),
  body: z.string(),
  rationale: z.string().nullable().optional().default(null),
  created_by_user_id: z.string(),
  created_at: z.string(),
}).loose();

export const RoomDetailSchema = z.object({
  room: RoomSchema,
  participants: z.array(RoomParticipantSchema).optional().default([]),
  entries: z.array(RoomEntrySchema).optional().default([]),
  cycles: z.array(RoomCycleSchema).optional().default([]),
  turns: z.array(RoomTurnSchema).optional().default([]),
  artifacts: z.array(RoomArtifactSchema).optional().default([]),
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
  created_by_user_id: "",
  facilitator_agent_id: "",
  facilitator_squad_id: null,
  status: "unknown",
  daily_turn_limit: null,
  schedule_interval_minutes: null,
  next_wake_at: null,
  active_cycle_id: null,
  memory: EMPTY_ROOM_MEMORY,
  memory_version: 0,
  created_at: "",
  updated_at: "",
};

export const EMPTY_ROOM_LIST: readonly Room[] = [];

export const EMPTY_ROOM_DETAIL: RoomDetail = {
  room: EMPTY_ROOM,
  participants: [],
  entries: [],
  cycles: [],
  turns: [],
  artifacts: [],
};

const EMPTY_ROOM_CYCLE: RoomCycle = {
  id: "",
  sequence: 0,
  source: "unknown",
  wake_key: "",
  triggering_entry_id: null,
  status: "unknown",
  refusal_reason: null,
  planned_at: null,
  created_at: "",
  started_at: null,
  completed_at: null,
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
  created_by_user_id: "",
  created_at: "",
};

export type {
  Room,
  RoomArtifact,
  RoomCycle,
  RoomDetail,
  RoomEntry,
  RoomMessageResult,
  RoomParticipant,
  RoomTurn,
  RoomWakeResult,
};
