import type {
  RoomArtifact,
  RoomCycle,
  RoomStatus,
} from "@multica/core/rooms";

interface RoomStatusStyle {
  readonly badge: string;
  readonly dot: string;
}

function roomStatusStyle(status: RoomStatus | string): RoomStatusStyle {
  switch (status) {
    case "active":
      return { badge: "bg-success/10 text-success", dot: "bg-success" };
    case "paused":
      return { badge: "bg-warning/10 text-warning", dot: "bg-warning" };
    case "archived":
    case "unknown":
    default:
      return {
        badge: "bg-muted text-muted-foreground",
        dot: "bg-muted-foreground",
      };
  }
}

export function roomStatusClass(status: RoomStatus | string): string {
  return roomStatusStyle(status).badge;
}

export function roomStatusDotClass(status: RoomStatus | string): string {
  return roomStatusStyle(status).dot;
}

export function cycleStatusClass(status: RoomCycle["status"] | string): string {
  switch (status) {
    case "completed":
      return "text-success";
    case "failed":
    case "cancelled":
      return "text-destructive";
    case "refused":
      return "text-warning";
    case "queued":
    case "running":
      return "text-brand";
    case "unknown":
    default:
      return "text-muted-foreground";
  }
}

export function countTodayTurns(
  detail: {
    readonly turns: readonly {
      readonly created_at: string;
      readonly status: string;
    }[];
  },
  now: Date,
): number {
  const dayStart = Date.UTC(
    now.getUTCFullYear(),
    now.getUTCMonth(),
    now.getUTCDate(),
  );
  return detail.turns.filter(
    (turn) =>
      turn.status !== "refused" &&
      new Date(turn.created_at).getTime() >= dayStart,
  ).length;
}

export function latestRefusedCycle<T extends Pick<RoomCycle, "status">>(
  cycles: readonly T[],
): T | null {
  for (let index = cycles.length - 1; index >= 0; index -= 1) {
    const cycle = cycles[index];
    if (cycle?.status === "refused") return cycle;
  }
  return null;
}

export function artifactHref(
  artifact: Pick<RoomArtifact, "kind" | "target_id">,
  paths: { issueDetail: (id: string) => string; wikiPage: (id: string) => string },
): string | null {
  if (!artifact.target_id) return null;
  switch (artifact.kind) {
    case "issue":
      return paths.issueDetail(artifact.target_id);
    case "wiki":
      return paths.wikiPage(artifact.target_id);
    case "decision":
    case "unknown":
    default:
      return null;
  }
}
