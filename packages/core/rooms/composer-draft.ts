export type RoomComposerDraftStatus = "idle" | "pending" | "failed";

export interface RoomComposerDraft {
  readonly body: string;
  readonly mentionAgentIds: readonly string[];
  readonly idempotencyKey: string;
  readonly status: RoomComposerDraftStatus;
}

export type RoomComposerDrafts = Readonly<Record<string, RoomComposerDraft>>;

export const EMPTY_ROOM_COMPOSER_DRAFTS: RoomComposerDrafts = {};

export function createRoomComposerDraft(idempotencyKey: string): RoomComposerDraft {
  return {
    body: "",
    mentionAgentIds: [],
    idempotencyKey,
    status: "idle",
  };
}

export function ensureRoomComposerDraft(
  drafts: RoomComposerDrafts,
  roomId: string,
  idempotencyKey: string,
): RoomComposerDrafts {
  if (drafts[roomId]) return drafts;
  return { ...drafts, [roomId]: createRoomComposerDraft(idempotencyKey) };
}

export function updateRoomComposerBody(
  drafts: RoomComposerDrafts,
  roomId: string,
  body: string,
  nextIdempotencyKey: string,
): RoomComposerDrafts {
  const current = drafts[roomId];
  if (!current || current.status === "pending") return drafts;
  const startsNewDraft = current.status === "failed" && body !== current.body;
  return {
    ...drafts,
    [roomId]: {
      ...current,
      body,
      idempotencyKey: startsNewDraft
        ? nextIdempotencyKey
        : current.idempotencyKey,
      status: startsNewDraft ? "idle" : current.status,
    },
  };
}

export function updateRoomComposerMention(
  drafts: RoomComposerDrafts,
  roomId: string,
  agentId: string,
  selected: boolean,
  nextIdempotencyKey: string,
): RoomComposerDrafts {
  const current = drafts[roomId];
  if (!current || current.status === "pending") return drafts;
  const hasMention = current.mentionAgentIds.includes(agentId);
  if (hasMention === selected) return drafts;
  const mentionAgentIds = selected
    ? [...current.mentionAgentIds, agentId]
    : current.mentionAgentIds.filter((id) => id !== agentId);
  return {
    ...drafts,
    [roomId]: {
      ...current,
      mentionAgentIds,
      idempotencyKey:
        current.status === "failed"
          ? nextIdempotencyKey
          : current.idempotencyKey,
      status: "idle",
    },
  };
}

export function markRoomComposerPending(
  drafts: RoomComposerDrafts,
  roomId: string,
): RoomComposerDrafts {
  const current = drafts[roomId];
  if (!current || current.status === "pending") return drafts;
  return {
    ...drafts,
    [roomId]: { ...current, status: "pending" },
  };
}

export function markRoomComposerFailed(
  drafts: RoomComposerDrafts,
  roomId: string,
): RoomComposerDrafts {
  const current = drafts[roomId];
  if (!current) return drafts;
  return {
    ...drafts,
    [roomId]: { ...current, status: "failed" },
  };
}

export function completeRoomComposerDraft(
  drafts: RoomComposerDrafts,
  roomId: string,
  nextIdempotencyKey: string,
): RoomComposerDrafts {
  if (!drafts[roomId]) return drafts;
  return {
    ...drafts,
    [roomId]: createRoomComposerDraft(nextIdempotencyKey),
  };
}
