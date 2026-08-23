import { useQueryClient } from "@tanstack/react-query";
import { roomKeys } from "@/data/queries/rooms";
import { useWSSubscriptions } from "@/lib/use-ws-subscriptions";
import { subscribeRoomEvents } from "./room-events";
import { invalidateRoomDetail } from "./room-ws-updaters";

/** Per-Room subscription. The server sends IDs/status only, so every matching
 * event refreshes the authoritative detail and outcome queries. */
export function useRoomRealtime(roomId: string | undefined) {
  const qc = useQueryClient();

  useWSSubscriptions(
    (ws, wsId) => {
      if (!roomId) return;
      const invalidate = () => invalidateRoomDetail(qc, wsId, roomId);

      return [
        ...subscribeRoomEvents(ws, (signal) => {
          if (signal.roomId === roomId) invalidate();
        }),
        ws.onReconnect(() => {
          invalidate();
          qc.invalidateQueries({ queryKey: roomKeys.list(wsId) });
        }),
      ];
    },
    [roomId, qc],
  );
}
