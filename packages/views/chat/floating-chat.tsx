"use client";

import { useEffect, useRef } from "react";
import { useChatStore } from "@multica/core/chat";
import { useWorkspacePaths } from "@multica/core/paths";
import { useNavigation } from "../navigation";
import { ChatFab } from "./components/chat-fab";
import { ChatWindow } from "./components/chat-window";
import { isFloatingChatRouteSuppressed } from "./floating-chat-visibility";

/**
 * Mount point for the floating chat overlay (FAB + window). Rendered once in
 * each app shell's dashboard layout; owns the two gates that decide whether the
 * overlay exists at all:
 *
 *  1. The Settings → Chat preference (`floatingChatEnabled`). When a user turns
 *     the floating window off, Chat lives only in its dedicated tab.
 *  2. The Chat tab route itself. On `/:slug/chat` the full-page surface already
 *     owns the conversation, so a floating copy of the same `activeSessionId`
 *     would be pure duplication — hide it there.
 */
export function FloatingChat() {
  const enabled = useChatStore((s) => s.floatingChatEnabled);
  const isOpen = useChatStore((s) => s.isOpen);
  const setOpen = useChatStore((s) => s.setOpen);
  const { pathname } = useNavigation();
  const wsPaths = useWorkspacePaths();
  const fabRef = useRef<HTMLButtonElement>(null);
  const restoreFabFocus = useRef(false);

  useEffect(() => {
    if (isOpen || !restoreFabFocus.current || !fabRef.current) return;
    fabRef.current.focus();
    restoreFabFocus.current = false;
  }, [isOpen]);

  if (!enabled) return null;
  // Suppress on the Chat tab — it renders the same conversation full-page.
  if (isFloatingChatRouteSuppressed(pathname, wsPaths.chat())) return null;

  return (
    <>
      <ChatWindow
        onMinimize={() => {
          restoreFabFocus.current = true;
          setOpen(false);
        }}
      />
      <ChatFab triggerRef={fabRef} />
    </>
  );
}
