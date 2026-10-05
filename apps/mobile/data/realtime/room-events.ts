import type { WSClient } from "@/data/realtime/ws-client";

export interface RoomRealtimeSignal {
  readonly roomId: string;
  readonly listChanged: boolean;
}

/** Keep all Room event registration in one typed adapter. Payloads are
 * intentionally bounded invalidation signals rather than alternate HTTP DTOs. */
export function subscribeRoomEvents(
  ws: WSClient,
  onSignal: (signal: RoomRealtimeSignal) => void,
): (() => void)[] {
  const signal = (roomId: string, listChanged = false) =>
    onSignal({ roomId, listChanged });

  return [
    ws.on("room:created", (payload) => signal(payload.room_id, true)),
    ws.on("room:updated", (payload) => signal(payload.room_id, true)),
    ws.on("room:entry", (payload) => signal(payload.room_id)),
    ws.on("room:cycle", (payload) => signal(payload.room_id)),
    ws.on("room:turn", (payload) => signal(payload.room_id)),
    ws.on("room:memory_revision", (payload) => signal(payload.room_id)),
    ws.on("room:review", (payload) => signal(payload.room_id, true)),
    ws.on("room:recommendation_review", (payload) => signal(payload.room_id)),
    ws.on("room:artifact", (payload) => signal(payload.room_id)),
  ];
}
