import type { Agent, Issue } from "../types";
import type { OfficeWorldId } from "./types";

export interface OfficeTaskObservation {
  readonly taskId: string;
  readonly agentId: Agent["id"];
  readonly issueId: Issue["id"] | null;
  readonly status: "queued-like" | "running" | "completed" | "failed";
}

type OfficeEffect =
  | {
      readonly kind: "task-queued";
      readonly taskId: OfficeTaskObservation["taskId"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
    }
  | {
      readonly kind: "task-started";
      readonly taskId: OfficeTaskObservation["taskId"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
    }
  | {
      readonly kind: "task-finished";
      readonly taskId: OfficeTaskObservation["taskId"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
      readonly outcome: "completed" | "failed";
    };

interface OfficeContinuityContext {
  readonly workspaceId: string;
  readonly world: OfficeWorldId;
  readonly foreground: boolean;
  readonly stale: boolean;
  readonly reconnectEpoch: number;
  readonly recoveryEpoch: number;
  readonly reducedMotion: boolean;
}

export interface OfficeContinuityInput extends OfficeContinuityContext {
  readonly observations: readonly OfficeTaskObservation[];
}

export interface OfficeContinuityState {
  readonly phase: "cold" | "rebasing" | "observing";
  readonly context: OfficeContinuityContext | null;
  readonly tasksById: ReadonlyMap<
    OfficeTaskObservation["taskId"],
    OfficeTaskObservation
  >;
}

export interface OfficeContinuityResult {
  readonly state: OfficeContinuityState;
  readonly commit: {
    readonly mode: "replace" | "transition";
    readonly effects: readonly OfficeEffect[];
  };
}

export function createOfficeContinuityState(): OfficeContinuityState {
  return {
    phase: "cold",
    context: null,
    tasksById: new Map(),
  };
}

function indexTasks(
  observations: readonly OfficeTaskObservation[],
): Map<OfficeTaskObservation["taskId"], OfficeTaskObservation> {
  const tasksById = new Map<
    OfficeTaskObservation["taskId"],
    OfficeTaskObservation
  >();
  for (const observed of observations) {
    tasksById.set(observed.taskId, observed);
  }
  return tasksById;
}

function contextOf(input: OfficeContinuityInput): OfficeContinuityContext {
  return {
    workspaceId: input.workspaceId,
    world: input.world,
    foreground: input.foreground,
    stale: input.stale,
    reconnectEpoch: input.reconnectEpoch,
    recoveryEpoch: input.recoveryEpoch,
    reducedMotion: input.reducedMotion,
  };
}

function contextChanged(
  previous: OfficeContinuityContext,
  current: OfficeContinuityContext,
): boolean {
  return (
    previous.workspaceId !== current.workspaceId ||
    previous.world !== current.world ||
    previous.foreground !== current.foreground ||
    previous.stale !== current.stale ||
    previous.reconnectEpoch !== current.reconnectEpoch ||
    previous.recoveryEpoch !== current.recoveryEpoch ||
    previous.reducedMotion !== current.reducedMotion
  );
}

function effectForTransition(
  previous: OfficeTaskObservation | undefined,
  current: OfficeTaskObservation,
): OfficeEffect | null {
  if (!previous && current.status === "queued-like") {
    return {
      kind: "task-queued",
      taskId: current.taskId,
      agentId: current.agentId,
      issueId: current.issueId,
    };
  }
  if (previous?.status === "queued-like" && current.status === "running") {
    return {
      kind: "task-started",
      taskId: current.taskId,
      agentId: current.agentId,
      issueId: current.issueId,
    };
  }
  if (
    previous &&
    (previous.status === "queued-like" || previous.status === "running") &&
    (current.status === "completed" || current.status === "failed")
  ) {
    return {
      kind: "task-finished",
      taskId: current.taskId,
      agentId: current.agentId,
      issueId: current.issueId,
      outcome: current.status,
    };
  }
  return null;
}

export function advanceOfficeContinuity(
  state: OfficeContinuityState,
  input: OfficeContinuityInput,
): OfficeContinuityResult {
  const context = contextOf(input);
  const tasksById = indexTasks(input.observations);
  const rebase =
    state.phase !== "observing" ||
    state.context === null ||
    contextChanged(state.context, context) ||
    !input.foreground ||
    input.stale ||
    input.reducedMotion;

  if (rebase) {
    return {
      state: { phase: "observing", context, tasksById },
      commit: { mode: "replace", effects: [] },
    };
  }

  const effects: OfficeEffect[] = [];
  const taskIds = [...tasksById.keys()].sort();
  for (const taskId of taskIds) {
    const current = tasksById.get(taskId);
    if (!current) continue;
    const effect = effectForTransition(state.tasksById.get(taskId), current);
    if (effect) effects.push(effect);
  }

  return {
    state: { phase: "observing", context, tasksById },
    commit: { mode: "transition", effects },
  };
}
