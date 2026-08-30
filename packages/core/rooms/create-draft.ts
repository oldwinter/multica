import type {
  CreateRoomInput,
  Room,
  RoomDetail,
  RoomParticipantInput,
  RoomTemplateId,
} from "./types";

export interface RoomTemplateDraftFields {
  readonly objective: string;
  readonly successCriteria: string;
  readonly stopConditions: string;
  readonly instructions: string;
  readonly dailyTurnLimit: string;
  readonly maxCostTicks: string;
  readonly scheduleMinutes: string;
}

export type RoomTemplateTouchedFields = Partial<
  Record<keyof RoomTemplateDraftFields, boolean>
>;

const ROOM_TEMPLATE_DRAFT_FIELDS: readonly (keyof RoomTemplateDraftFields)[] = [
  "objective",
  "successCriteria",
  "stopConditions",
  "instructions",
  "dailyTurnLimit",
  "maxCostTicks",
  "scheduleMinutes",
];

export function applyRoomTemplateDefaults(
  current: RoomTemplateDraftFields,
  nextDefaults: RoomTemplateDraftFields,
  touched: RoomTemplateTouchedFields,
): RoomTemplateDraftFields {
  const next = { ...current };
  for (const field of ROOM_TEMPLATE_DRAFT_FIELDS) {
    if (!touched[field]) next[field] = nextDefaults[field];
  }
  return next;
}

export function duplicateRoomConfiguration(detail: RoomDetail): CreateRoomInput {
  const room = detail.room;
  const participants = explicitDuplicateParticipants(detail);
  const shared = {
    title: room.title,
    instructions: room.instructions || undefined,
    objective: room.objective,
    success_criteria: [...room.success_criteria],
    stop_conditions: [...room.stop_conditions],
    ...(isKnownTemplate(room.template_id)
      ? { template_id: room.template_id }
      : {}),
    ...(participants.length > 0 ? { participants } : {}),
    ...(room.daily_turn_limit !== null
      ? { daily_turn_limit: room.daily_turn_limit }
      : {}),
    ...(room.max_cost_ticks !== null
      ? { max_cost_ticks: room.max_cost_ticks }
      : {}),
    ...(room.schedule_interval_minutes !== null
      ? {
          schedule_interval_minutes: room.schedule_interval_minutes,
          start_paused: true,
        }
      : {}),
  };

  return room.facilitator_squad_id
    ? { ...shared, facilitator_squad_id: room.facilitator_squad_id }
    : { ...shared, facilitator_agent_id: room.facilitator_agent_id };
}

export function rankRoomsForValueReview(
  rooms: readonly Room[],
  now: number = Date.now(),
  recentDays = 30,
  limit = 5,
): Room[] {
  const recentThreshold = now - recentDays * 24 * 60 * 60 * 1_000;
  return rooms
    .filter((room) => {
      const value = room.value;
      if (!value) return false;
      const lastRunAt = Date.parse(value.last_run_at ?? "");
      return (
        value.repeat_run_count > 0 ||
        (Number.isFinite(lastRunAt) && lastRunAt >= recentThreshold)
      );
    })
    .sort((left, right) => {
      const leftValue = left.value!;
      const rightValue = right.value!;
      return (
        rightValue.accepted_outcomes - leftValue.accepted_outcomes ||
        rightValue.promotion_rate - leftValue.promotion_rate ||
        Date.parse(rightValue.last_run_at ?? "") -
          Date.parse(leftValue.last_run_at ?? "") ||
        left.title.localeCompare(right.title)
      );
    })
    .slice(0, Math.max(0, limit));
}

function explicitDuplicateParticipants(
  detail: RoomDetail,
): RoomParticipantInput[] {
  return detail.participants.flatMap((participant) => {
    if (
      participant.type === "unknown" ||
      participant.role === "facilitator" ||
      (participant.source_squad_id !== null &&
        participant.source_squad_id === detail.room.facilitator_squad_id)
    ) {
      return [];
    }
    return [
      {
        type: participant.type,
        id: participant.participant_id,
        role: participant.role,
      },
    ];
  });
}

function isKnownTemplate(
  template: RoomTemplateId | null,
): template is Exclude<RoomTemplateId, "unknown"> {
  return template !== null && template !== "unknown";
}
