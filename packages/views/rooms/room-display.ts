import type {
  RoomArtifact,
  RoomCycle,
  RoomCycleStatus,
  RoomStatus,
} from "@multica/core/rooms";

interface RoomStatusStyle {
  readonly badge: string;
  readonly dot: string;
}

const ROOM_STATUS_STYLES: Readonly<Record<RoomStatus, RoomStatusStyle>> = {
  active: { badge: "bg-success/10 text-foreground", dot: "bg-success" },
  paused: { badge: "bg-warning/10 text-warning", dot: "bg-warning" },
  archived: { badge: "bg-muted text-muted-foreground", dot: "bg-muted-foreground" },
  unknown: { badge: "bg-muted text-muted-foreground", dot: "bg-muted-foreground" },
};

const CYCLE_STATUS_CLASSES: Readonly<Record<RoomCycleStatus, string>> = {
  completed: "text-success",
  failed: "text-destructive",
  cancelled: "text-destructive",
  refused: "text-warning",
  queued: "text-brand",
  running: "text-brand",
  unknown: "text-muted-foreground",
};

export type RoomRefusalKey =
  | "room_paused"
  | "room_archived"
  | "budget_exhausted"
  | "cycle_active"
  | "agent_unavailable"
  | "daemon_capability_unavailable"
  | "spend_limit_unsupported"
  | "invocation_not_allowed"
  | "preflight_required";

const ROOM_REFUSAL_KEYS: Readonly<Record<string, RoomRefusalKey>> = {
  room_paused: "room_paused",
  room_archived: "room_archived",
  budget_exhausted: "budget_exhausted",
  active_cycle: "cycle_active",
  cycle_active: "cycle_active",
  agent_unavailable: "agent_unavailable",
  daemon_capability_unavailable: "daemon_capability_unavailable",
  spend_limit_unsupported: "spend_limit_unsupported",
  invocation_not_allowed: "invocation_not_allowed",
  preflight_required: "preflight_required",
};

function roomStatusStyle(status: RoomStatus | string): RoomStatusStyle {
  return Object.hasOwn(ROOM_STATUS_STYLES, status)
    ? ROOM_STATUS_STYLES[status as RoomStatus]
    : ROOM_STATUS_STYLES.unknown;
}

export function roomStatusClass(status: RoomStatus | string): string {
  return roomStatusStyle(status).badge;
}

export function roomStatusDotClass(status: RoomStatus | string): string {
  return roomStatusStyle(status).dot;
}

export function cycleStatusClass(status: RoomCycle["status"] | string): string {
  return Object.hasOwn(CYCLE_STATUS_CLASSES, status)
    ? CYCLE_STATUS_CLASSES[status as RoomCycleStatus]
    : CYCLE_STATUS_CLASSES.unknown;
}

export function roomRefusalKey(blocker: string | null | undefined): RoomRefusalKey {
  return blocker && Object.hasOwn(ROOM_REFUSAL_KEYS, blocker)
    ? ROOM_REFUSAL_KEYS[blocker]!
    : "preflight_required";
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
