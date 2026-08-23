import type {
  RoomCycle,
  RoomCyclePhase,
  RoomDetail,
  RoomMemoryRevision,
  RoomPreflight,
  RoomSynthesis,
  RoomUsage,
} from "./types";

export type RoomOutcomeNextAction =
  | "resume"
  | "run_preflight"
  | "run_cycle"
  | "wait"
  | "review"
  | "retry_synthesis"
  | "resolve_blocker"
  | "none";

export interface RoomMemoryDiff {
  readonly summaryChanged: boolean;
  readonly added: readonly string[];
  readonly removed: readonly string[];
}

export interface RoomOutcomeState {
  readonly objective: string;
  readonly activeCycle: RoomCycle | null;
  readonly latestCycle: RoomCycle | null;
  readonly phase: RoomCyclePhase;
  readonly latestOutcome: RoomMemoryRevision | null;
  readonly acceptedOutcome: RoomMemoryRevision | null;
  readonly memoryDiff: RoomMemoryDiff;
  readonly nextAction: RoomOutcomeNextAction;
  readonly blocker: string | null;
  readonly canCancel: boolean;
  readonly usage: RoomUsage;
}

const EMPTY_USAGE: RoomUsage = {
  turns_total: 0,
  cost_ticks: 0,
  uncosted_turns: 0,
  failures: 0,
  accepted_syntheses: 0,
  promoted_artifacts: 0,
};

export function deriveRoomOutcomeState(
  detail: RoomDetail,
  options: {
    readonly preflight?: RoomPreflight | null;
    readonly preflightPending?: boolean;
    readonly usage?: RoomUsage | null;
  } = {},
): RoomOutcomeState {
  const latestCycle = selectLatestRoomCycle(detail.cycles);
  const lifecycleCycle = selectRoomLifecycleCycle(detail);
  const activeCycle = lifecycleCycle && isOpenPhase(deriveCyclePhase(lifecycleCycle))
    ? lifecycleCycle
    : null;
  const latestOutcome = maxBy(detail.memory_revisions, (revision) => revision.version);
  const acceptedOutcome =
    detail.memory_revisions.find(
      (revision) => revision.id === detail.room.accepted_memory_revision_id,
    ) ??
    maxBy(
      detail.memory_revisions.filter((revision) => revision.review_status === "accepted"),
      (revision) => revision.version,
    ) ??
    null;
  const previousOutcome = latestOutcome
    ? maxBy(
      detail.memory_revisions.filter(
        (revision) => revision.version < latestOutcome.version,
      ),
      (revision) => revision.version,
    )
    : null;
  const phase = deriveCyclePhase(activeCycle ?? latestCycle);
  const blocker = deriveBlocker(detail, activeCycle ?? latestCycle, options.preflight);

  return {
    objective: detail.room.objective.trim() || detail.room.instructions.trim(),
    activeCycle,
    latestCycle,
    phase,
    latestOutcome,
    acceptedOutcome,
    memoryDiff: deriveMemoryDiff(previousOutcome?.synthesis, latestOutcome?.synthesis),
    nextAction: deriveNextAction({
      roomStatus: detail.room.status,
      phase,
      cycle: activeCycle ?? latestCycle,
      latestOutcome,
      blocker,
      preflight: options.preflight,
      preflightPending: options.preflightPending === true,
    }),
    blocker,
    canCancel: activeCycle !== null && isActivePhase(phase),
    usage: options.usage ?? EMPTY_USAGE,
  };
}

function deriveCyclePhase(cycle: RoomCycle | null | undefined): RoomCyclePhase {
  if (!cycle) return "unknown";
  if (cycle.phase !== "unknown") return cycle.phase;
  switch (cycle.status) {
    case "queued":
    case "running":
      return "gathering";
    case "completed":
      return "completed";
    case "failed":
    case "cancelled":
    case "refused":
      return cycle.status;
    case "unknown":
    default:
      return "unknown";
  }
}

function deriveBlocker(
  detail: RoomDetail,
  cycle: RoomCycle | null,
  preflight: RoomPreflight | null | undefined,
): string | null {
  if (detail.room.status === "paused") return "room_paused";
  if (detail.room.status === "archived") return "room_archived";
  if (cycle?.synthesis_error) return cycle.synthesis_error.code || "synthesis_failed";
  if (cycle?.phase === "refused" || cycle?.status === "refused") {
    return cycle.refusal_reason ?? "unknown";
  }
  if (preflight?.capability_ready === false) {
    return "daemon_capability_unavailable";
  }
  if (preflight?.allowed === false) {
    return (
      preflight.refusal_reason ??
      preflight.target_agents.find(
        (agent) => agent.ready === false || agent.invocation_allowed === false,
      )?.reason ??
      "unknown"
    );
  }
  return null;
}

function deriveNextAction(input: {
  readonly roomStatus: RoomDetail["room"]["status"];
  readonly phase: RoomCyclePhase;
  readonly cycle: RoomCycle | null;
  readonly latestOutcome: RoomMemoryRevision | null;
  readonly blocker: string | null;
  readonly preflight: RoomPreflight | null | undefined;
  readonly preflightPending: boolean;
}): RoomOutcomeNextAction {
  if (input.roomStatus === "archived") return "none";
  if (input.roomStatus === "paused") return "resume";
  if (
    input.cycle?.synthesis_error?.retryable === true &&
    (input.phase === "failed" || input.phase === "awaiting_review")
  ) {
    return "retry_synthesis";
  }
  if (input.phase === "awaiting_review" && input.latestOutcome?.review_status === "pending") {
    return "review";
  }
  if (isActivePhase(input.phase)) return "wait";
  if (input.blocker) return "resolve_blocker";
  if (input.preflightPending || input.preflight === undefined || input.preflight === null) {
    return "run_preflight";
  }
  return input.preflight.allowed ? "run_cycle" : "resolve_blocker";
}

function isActivePhase(phase: RoomCyclePhase): boolean {
  return phase === "gathering" || phase === "synthesizing";
}

function isOpenPhase(phase: RoomCyclePhase): boolean {
  return isActivePhase(phase) || phase === "awaiting_review";
}

export function deriveMemoryDiff(
  accepted: RoomSynthesis | null | undefined,
  candidate: RoomSynthesis | null | undefined,
): RoomMemoryDiff {
  if (!candidate) return { summaryChanged: false, added: [], removed: [] };
  const before = synthesisTexts(accepted);
  const after = synthesisTexts(candidate);
  return {
    summaryChanged: (accepted?.summary ?? "") !== candidate.summary,
    added: after.filter((text) => !before.includes(text)),
    removed: before.filter((text) => !after.includes(text)),
  };
}

function synthesisTexts(synthesis: RoomSynthesis | null | undefined): string[] {
  if (!synthesis) return [];
  return [
    ...synthesis.facts,
    ...synthesis.decisions,
    ...synthesis.open_questions,
    ...synthesis.disagreements,
    ...synthesis.action_items,
  ].map((item) => item.text);
}

export function selectLatestRoomCycle(cycles: readonly RoomCycle[]): RoomCycle | null {
  return maxBy(cycles, (cycle) => cycle.sequence);
}

export function selectRoomLifecycleCycle(detail: RoomDetail): RoomCycle | null {
  const pointedCycle = detail.cycles.find(
    (cycle) => cycle.id === detail.room.active_cycle_id,
  );
  if (pointedCycle && isOpenPhase(deriveCyclePhase(pointedCycle))) {
    return pointedCycle;
  }
  return maxBy(
    detail.cycles.filter((cycle) => isOpenPhase(deriveCyclePhase(cycle))),
    (cycle) => cycle.sequence,
  ) ?? selectLatestRoomCycle(detail.cycles);
}

function maxBy<T>(items: readonly T[], value: (item: T) => number): T | null {
  let maximum: T | null = null;
  for (const item of items) {
    if (maximum === null || value(item) > value(maximum)) maximum = item;
  }
  return maximum;
}
