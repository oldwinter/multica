"use client";

import { useCallback, useEffect, useMemo } from "react";
import { useAuthStore } from "@multica/core/auth";
import { useCurrentWorkspace } from "@multica/core/paths";
import { createSafeId } from "@multica/core/utils";
import {
  completeRoomComposerDraft,
  ensureRoomComposerDraft,
  markRoomComposerFailed,
  markRoomComposerPending,
  roomComposerDraftsForScope,
  updateRoomComposerBody,
  updateRoomComposerMention,
  useRoomComposerDraftStore,
  type RoomComposerDraftScope,
  type RoomComposerDrafts,
} from "@multica/core/rooms";

function initializeScopedDrafts(
  scope: RoomComposerDraftScope,
  update: (drafts: RoomComposerDrafts) => RoomComposerDrafts,
) {
  const store = useRoomComposerDraftStore.getState();
  const current = roomComposerDraftsForScope(store.draft, scope);
  const next = update(current);
  if (
    next !== current ||
    store.draft.ownerUserId !== scope.userId ||
    store.draft.ownerWorkspaceId !== scope.workspaceId
  ) {
    store.setDraft({
      ownerUserId: scope.userId,
      ownerWorkspaceId: scope.workspaceId,
      rooms: next,
    });
  }
}

function updateOwnedDrafts(
  scope: RoomComposerDraftScope,
  update: (drafts: RoomComposerDrafts) => RoomComposerDrafts,
) {
  const store = useRoomComposerDraftStore.getState();
  if (
    store.draft.ownerUserId !== scope.userId ||
    store.draft.ownerWorkspaceId !== scope.workspaceId
  ) {
    return;
  }
  const next = update(store.draft.rooms);
  if (next !== store.draft.rooms) store.setDraft({ rooms: next });
}

export function useRoomComposerDrafts(activeRoomId: string) {
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const workspaceId = useCurrentWorkspace()?.id ?? null;
  const scope = useMemo(
    () => (userId && workspaceId ? { userId, workspaceId } : null),
    [userId, workspaceId],
  );
  const draft = useRoomComposerDraftStore((state) =>
    scope
      ? roomComposerDraftsForScope(state.draft, scope)[activeRoomId]
      : undefined,
  );

  useEffect(() => {
    if (!activeRoomId || !scope) return;
    const ensureOwnedDraft = () => {
      updateOwnedDrafts(scope, (current) =>
        ensureRoomComposerDraft(current, activeRoomId, createSafeId()),
      );
    };
    initializeScopedDrafts(scope, (current) =>
      ensureRoomComposerDraft(current, activeRoomId, createSafeId()),
    );
    // A workspace switch can start rehydration before the previous Room
    // component's passive-effect cleanup runs. The old listener must never
    // reclaim a store that hydration has already assigned to the new scope.
    return useRoomComposerDraftStore.persist.onFinishHydration(ensureOwnedDraft);
  }, [activeRoomId, scope]);

  const updateBody = useCallback((roomId: string, body: string) => {
    if (!scope) return;
    updateOwnedDrafts(scope, (current) =>
      updateRoomComposerBody(current, roomId, body, createSafeId()),
    );
  }, [scope]);

  const updateMention = useCallback(
    (roomId: string, agentId: string, selected: boolean) => {
      if (!scope) return;
      updateOwnedDrafts(scope, (current) =>
        updateRoomComposerMention(
          current,
          roomId,
          agentId,
          selected,
          createSafeId(),
        ),
      );
    },
    [scope],
  );

  const markPending = useCallback(
    (roomId: string, idempotencyKey: string) => {
      if (!scope) return;
      updateOwnedDrafts(scope, (current) =>
        current[roomId]?.idempotencyKey === idempotencyKey
          ? markRoomComposerPending(current, roomId)
          : current,
      );
    },
    [scope],
  );

  const markFailed = useCallback(
    (roomId: string, idempotencyKey: string) => {
      if (!scope) return;
      updateOwnedDrafts(scope, (current) =>
        current[roomId]?.idempotencyKey === idempotencyKey
          ? markRoomComposerFailed(current, roomId)
          : current,
      );
    },
    [scope],
  );

  const complete = useCallback(
    (roomId: string, idempotencyKey: string) => {
      if (!scope) return;
      updateOwnedDrafts(scope, (current) =>
        current[roomId]?.idempotencyKey === idempotencyKey
          ? completeRoomComposerDraft(current, roomId, createSafeId())
          : current,
      );
    },
    [scope],
  );

  return {
    draft,
    updateBody,
    updateMention,
    markPending,
    markFailed,
    complete,
  };
}
