import { createDraftStore } from "../drafts/create-draft-store";
import {
  EMPTY_SCOPED_ROOM_COMPOSER_DRAFTS,
  type RoomComposerDraft,
  type RoomComposerDraftStatus,
  type ScopedRoomComposerDrafts,
} from "./composer-draft";

export const ROOM_COMPOSER_DRAFT_STORAGE_KEY = "multica_room_composer_drafts";

export const useRoomComposerDraftStore = createDraftStore<ScopedRoomComposerDrafts>({
  storageKey: ROOM_COMPOSER_DRAFT_STORAGE_KEY,
  emptyData: EMPTY_SCOPED_ROOM_COMPOSER_DRAFTS,
  hasMeaningful: (drafts) =>
    Object.values(drafts.rooms).some(
      (draft) =>
        draft.body.length > 0 ||
        draft.mentionAgentIds.length > 0 ||
        draft.status !== "idle",
    ),
  migrateData: migrateRoomComposerDrafts,
});

function migrateRoomComposerDrafts(
  raw: unknown,
): ScopedRoomComposerDrafts {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return EMPTY_SCOPED_ROOM_COMPOSER_DRAFTS;
  }

  const stored = raw as Partial<ScopedRoomComposerDrafts>;
  if (
    typeof stored.ownerUserId !== "string" ||
    !stored.ownerUserId ||
    typeof stored.ownerWorkspaceId !== "string" ||
    !stored.ownerWorkspaceId
  ) {
    // Ownerless drafts cannot be attributed safely after an auth transition.
    return EMPTY_SCOPED_ROOM_COMPOSER_DRAFTS;
  }

  const rawRooms = stored.rooms;
  if (!rawRooms || typeof rawRooms !== "object" || Array.isArray(rawRooms)) {
    return {
      ownerUserId: stored.ownerUserId,
      ownerWorkspaceId: stored.ownerWorkspaceId,
      rooms: {},
    };
  }

  const migrated: Record<string, RoomComposerDraft> = {};
  for (const [roomId, value] of Object.entries(rawRooms)) {
    if (!roomId || !value || typeof value !== "object" || Array.isArray(value)) {
      continue;
    }
    const draft = value as Partial<RoomComposerDraft>;
    if (typeof draft.idempotencyKey !== "string" || !draft.idempotencyKey) {
      continue;
    }

    const status: RoomComposerDraftStatus =
      draft.status === "pending" || draft.status === "failed" ? "failed" : "idle";
    migrated[roomId] = {
      body: typeof draft.body === "string" ? draft.body : "",
      mentionAgentIds: Array.isArray(draft.mentionAgentIds)
        ? draft.mentionAgentIds.filter((id): id is string => typeof id === "string")
        : [],
      idempotencyKey: draft.idempotencyKey,
      status,
    };
  }
  return {
    ownerUserId: stored.ownerUserId,
    ownerWorkspaceId: stored.ownerWorkspaceId,
    rooms: migrated,
  };
}
