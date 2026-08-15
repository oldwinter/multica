import { createDraftStore } from "../drafts/create-draft-store";
import {
  EMPTY_ROOM_COMPOSER_DRAFTS,
  type RoomComposerDraft,
  type RoomComposerDraftStatus,
  type RoomComposerDrafts,
} from "./composer-draft";

export const ROOM_COMPOSER_DRAFT_STORAGE_KEY = "multica_room_composer_drafts";

export const useRoomComposerDraftStore = createDraftStore<RoomComposerDrafts>({
  storageKey: ROOM_COMPOSER_DRAFT_STORAGE_KEY,
  emptyData: EMPTY_ROOM_COMPOSER_DRAFTS,
  hasMeaningful: (drafts) =>
    Object.values(drafts).some(
      (draft) =>
        draft.body.length > 0 ||
        draft.mentionAgentIds.length > 0 ||
        draft.status !== "idle",
    ),
  migrateData: migrateRoomComposerDrafts,
});

function migrateRoomComposerDrafts(raw: unknown): RoomComposerDrafts {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};

  const migrated: Record<string, RoomComposerDraft> = {};
  for (const [roomId, value] of Object.entries(raw)) {
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
  return migrated;
}
