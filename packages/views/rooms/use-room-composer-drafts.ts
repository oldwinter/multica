"use client";

import { useCallback, useEffect, useState } from "react";
import { createSafeId } from "@multica/core/utils";
import {
  EMPTY_ROOM_COMPOSER_DRAFTS,
  completeRoomComposerDraft,
  ensureRoomComposerDraft,
  markRoomComposerFailed,
  markRoomComposerPending,
  updateRoomComposerBody,
  updateRoomComposerMention,
  type RoomComposerDrafts,
} from "./room-composer-draft";

export function useRoomComposerDrafts(activeRoomId: string) {
  const [drafts, setDrafts] = useState<RoomComposerDrafts>(
    EMPTY_ROOM_COMPOSER_DRAFTS,
  );

  useEffect(() => {
    if (!activeRoomId) return;
    const idempotencyKey = createSafeId();
    setDrafts((current) =>
      ensureRoomComposerDraft(current, activeRoomId, idempotencyKey),
    );
  }, [activeRoomId]);

  const updateBody = useCallback((roomId: string, body: string) => {
    const idempotencyKey = createSafeId();
    setDrafts((current) =>
      updateRoomComposerBody(current, roomId, body, idempotencyKey),
    );
  }, []);

  const updateMention = useCallback(
    (roomId: string, agentId: string, selected: boolean) => {
      const idempotencyKey = createSafeId();
      setDrafts((current) =>
        updateRoomComposerMention(
          current,
          roomId,
          agentId,
          selected,
          idempotencyKey,
        ),
      );
    },
    [],
  );

  const markPending = useCallback((roomId: string) => {
    setDrafts((current) => markRoomComposerPending(current, roomId));
  }, []);

  const markFailed = useCallback((roomId: string) => {
    setDrafts((current) => markRoomComposerFailed(current, roomId));
  }, []);

  const complete = useCallback((roomId: string) => {
    const idempotencyKey = createSafeId();
    setDrafts((current) =>
      completeRoomComposerDraft(current, roomId, idempotencyKey),
    );
  }, []);

  return {
    draft: drafts[activeRoomId],
    updateBody,
    updateMention,
    markPending,
    markFailed,
    complete,
  };
}
