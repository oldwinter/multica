import { z } from "zod";
export const RoomCreatedPayloadSchema = z.object({
  room_id: z.string().min(1),
  status: z.string().min(1),
  memory_version: z.number().int().nonnegative(),
  active_cycle_id: z.string().min(1).optional(),
}).strict();

export const RoomUpdatedPayloadSchema = RoomCreatedPayloadSchema;

export const RoomEntryPayloadSchema = z.object({
  room_id: z.string().min(1),
  entry_id: z.string().min(1),
  cycle_id: z.string().min(1).optional(),
  turn_id: z.string().min(1).optional(),
}).strict();

export const RoomCyclePayloadSchema = z.object({
  room_id: z.string().min(1),
  cycle_id: z.string().min(1),
  status: z.string().min(1),
  phase: z.string().min(1),
}).strict();

export const RoomTurnPayloadSchema = z.object({
  room_id: z.string().min(1),
  cycle_id: z.string().min(1),
  turn_id: z.string().min(1),
  status: z.string().min(1),
  turn_kind: z.string().min(1),
  attempt: z.number().int().positive(),
}).strict();

export const RoomMemoryRevisionPayloadSchema = z.object({
  room_id: z.string().min(1),
  cycle_id: z.string().min(1),
  memory_revision_id: z.string().min(1),
  review_status: z.string().min(1),
  version: z.number().int().positive(),
}).strict();

export const RoomReviewPayloadSchema = z.object({
  room_id: z.string().min(1),
  cycle_id: z.string().min(1),
  memory_revision_id: z.string().min(1),
  review_status: z.string().min(1),
  action: z.string().min(1),
  memory_version: z.number().int().nonnegative(),
}).strict();

export const RoomRecommendationReviewPayloadSchema = z.object({
  room_id: z.string().min(1),
  memory_revision_id: z.string().min(1),
  recommendation_key: z.string().min(1),
  status: z.string().min(1),
  artifact_id: z.string().min(1).optional(),
}).strict();

export const RoomArtifactPayloadSchema = z.object({
  room_id: z.string().min(1),
  artifact_id: z.string().min(1),
  kind: z.string().min(1),
  target_id: z.string().min(1).optional(),
  memory_revision_id: z.string().min(1).optional(),
  recommendation_key: z.string().min(1).optional(),
}).strict();
