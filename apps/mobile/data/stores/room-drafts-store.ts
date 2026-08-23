/**
 * Room composer drafts survive route changes for the app session. Each draft
 * carries its idempotency key so a retry after a timeout or 409 sends the same
 * logical message. This mirrors the existing chat-draft lifecycle; it avoids
 * writing every keystroke into iOS Keychain storage.
 */
import { create } from "zustand";
import { createRoomIdempotencyKey } from "@/lib/room-interactions";

export interface RoomDraft {
  readonly body: string;
  readonly idempotencyKey: string;
  readonly submittedBody: string | null;
}

interface RoomDraftsState {
  drafts: Record<string, RoomDraft>;
  setBody: (key: string, body: string) => void;
  markSubmitted: (key: string, submittedBody: string) => void;
  clear: (key: string) => void;
}

export function roomDraftKey(wsId: string | null, roomId: string): string {
  return `${wsId ?? "unknown"}:${roomId}`;
}

export const useRoomDraftsStore = create<RoomDraftsState>((set, get) => ({
  drafts: {},
  setBody: (key, body) => {
    const current = get().drafts;
    const existing = current[key];
    if (existing?.body === body) return;
    if (!body && !existing) return;
    set({
      drafts: {
        ...current,
        [key]: {
          body,
          idempotencyKey:
            existing?.submittedBody !== null && body !== existing?.submittedBody
              ? createRoomIdempotencyKey("message")
              : existing?.idempotencyKey ?? createRoomIdempotencyKey("message"),
          submittedBody:
            existing?.submittedBody !== null && body !== existing?.submittedBody
              ? null
              : existing?.submittedBody ?? null,
        },
      },
    });
  },
  markSubmitted: (key, submittedBody) => {
    const current = get().drafts;
    const existing = current[key];
    if (!existing || existing.submittedBody === submittedBody) return;
    set({ drafts: { ...current, [key]: { ...existing, submittedBody } } });
  },
  clear: (key) => {
    const current = get().drafts;
    if (!(key in current)) return;
    const next = { ...current };
    delete next[key];
    set({ drafts: next });
  },
}));
