import { useQueryClient } from "@tanstack/react-query";
import { roomKeys } from "@/data/queries/rooms";
import { useWSSubscriptions } from "@/lib/use-ws-subscriptions";
import { subscribeRoomEvents } from "./room-events";

/** Workspace-lifetime subscription. HTTP remains the only Room DTO source. */
export function useRoomsRealtime() {
  const qc = useQueryClient();

  useWSSubscriptions(
    (ws, wsId) => {
      const invalidate = () =>
        qc.invalidateQueries({ queryKey: roomKeys.list(wsId) });

      return [
        ...subscribeRoomEvents(ws, ({ listChanged }) => {
          if (listChanged) invalidate();
        }),
        ws.onReconnect(invalidate),
      ];
    },
    [qc],
  );
}
