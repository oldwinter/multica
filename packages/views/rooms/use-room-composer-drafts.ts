"use client";

import { useCallback, useEffect } from "react";
import { createSafeId } from "@multica/core/utils";
import {
  completeRoomComposerDraft,
  ensureRoomComposerDraft,
  markRoomComposerFailed,
  markRoomComposerPending,
  updateRoomComposerBody,
  updateRoomComposerMention,
  useRoomComposerDraftStore,
  type RoomComposerDrafts,
} from "@multica/core/rooms";

function updateDrafts(update: (drafts: RoomComposerDrafts) => RoomComposerDrafts) {
  const store = useRoomComposerDraftStore.getState();
  const next = update(store.draft);
  if (next !== store.draft) store.setDraft(next);
}

export function useRoomComposerDrafts(activeRoomId: string) {
  const draft = useRoomComposerDraftStore((state) => state.draft[activeRoomId]);

  useEffect(() => {
    if (!activeRoomId) return;
    const ensureDraft = () => {
      updateDrafts((current) =>
        ensureRoomComposerDraft(current, activeRoomId, createSafeId()),
      );
    };
    ensureDraft();
    return useRoomComposerDraftStore.persist.onFinishHydration(ensureDraft);
  }, [activeRoomId]);

  const updateBody = useCallback((roomId: string, body: string) => {
    updateDrafts((current) =>
      updateRoomComposerBody(current, roomId, body, createSafeId()),
    );
  }, []);

  const updateMention = useCallback(
    (roomId: string, agentId: string, selected: boolean) => {
      updateDrafts((current) =>
        updateRoomComposerMention(
          current,
          roomId,
          agentId,
          selected,
          createSafeId(),
        ),
      );
    },
    [],
  );

  const markPending = useCallback((roomId: string) => {
    updateDrafts((current) => markRoomComposerPending(current, roomId));
  }, []);

  const markFailed = useCallback((roomId: string) => {
    updateDrafts((current) => markRoomComposerFailed(current, roomId));
  }, []);

  const complete = useCallback((roomId: string) => {
    updateDrafts((current) =>
      completeRoomComposerDraft(current, roomId, createSafeId()),
    );
  }, []);

  return {
    draft,
    updateBody,
    updateMention,
    markPending,
    markFailed,
    complete,
  };
}
