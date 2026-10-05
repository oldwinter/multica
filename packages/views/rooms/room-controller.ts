import {
  selectRoomLifecycleCycle,
  type RoomCycle,
  type RoomDetail,
} from "@multica/core/rooms";

export function selectRoomLifecycleCycleId(
  detail: RoomDetail | null | undefined,
): string {
  if (!detail) return "";
  return selectRoomLifecycleCycle(detail)?.id ?? "";
}

export function selectRecentRoomCycles(
  cycles: readonly RoomCycle[],
  limit = 6,
): readonly RoomCycle[] {
  return [...cycles]
    .sort((left, right) => right.sequence - left.sequence)
    .slice(0, limit);
}
