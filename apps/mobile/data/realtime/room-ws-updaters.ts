import type { QueryClient } from "@tanstack/react-query";
import { roomKeys } from "@/data/queries/rooms";

export function invalidateRoomDetail(
  qc: QueryClient,
  wsId: string,
  roomId: string,
) {
  qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
  qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
  qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
}
