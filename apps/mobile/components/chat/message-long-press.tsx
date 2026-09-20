/**
 * Long-press handler for a chat message bubble. Exposes `onLongPress`
 * (drives a platform action sheet) and `isPressed` (drives the
 * caller's highlight ring while the sheet is on screen).
 *
 * iOS keeps its native sheet; Expo supplies Android presentation.
 *
 * Item set (v1, conditional):
 *   Copy · Select Text · Cancel
 *
 * Mirrors `useCommentLongPress` in `components/issue/comment-context-
 * menu.tsx` — kept as a sibling rather than a shared primitive because
 * we have only 2 callers (chat + comments). Below the "3 callers + no
 * native alternative" threshold in apps/mobile/CLAUDE.md.
 */
import { useCallback, useState } from "react";
import { useAppActionSheet } from "@/lib/use-action-sheet";
import * as Clipboard from "expo-clipboard";
import * as Haptics from "expo-haptics";
import type { ChatMessage } from "@multica/core/types";
import { useChatSelectStore } from "@/data/chat-select-store";

export function useChatMessageLongPress(
  message: ChatMessage,
): { onLongPress: () => void; isPressed: boolean } {
  const [isPressed, setIsPressed] = useState(false);
  const showActionSheet = useAppActionSheet();

  const onLongPress = useCallback(() => {
    const hasContent = !!message.content;

    Haptics.selectionAsync().catch(() => {});
    setIsPressed(true);

    type Action =
      | { kind: "copy" }
      | { kind: "select" }
      | { kind: "cancel" };

    const options: string[] = [];
    const actions: Action[] = [];
    const push = (label: string, action: Action) => {
      options.push(label);
      actions.push(action);
    };

    if (hasContent) {
      push("Copy", { kind: "copy" });
      push("Select Text", { kind: "select" });
    }
    push("Cancel", { kind: "cancel" });

    const cancelButtonIndex = options.length - 1;

    showActionSheet(
      { options, cancelButtonIndex },
      (i) => {
        setIsPressed(false);
        const action = actions[i];
        if (!action || action.kind === "cancel") return;

        switch (action.kind) {
          case "copy":
            if (message.content) {
              Clipboard.setStringAsync(message.content);
              Haptics.notificationAsync(
                Haptics.NotificationFeedbackType.Success,
              ).catch(() => {});
            }
            return;
          case "select":
            useChatSelectStore.getState().setSelecting(message.id);
            return;
        }
      },
    );
  }, [message, showActionSheet]);

  return { onLongPress, isPressed };
}
