import type { OfficeAgent } from "@multica/core/office";
import type { OfficeAgentSceneState } from "./contracts";

export function mapAgentSceneState(
  agent: OfficeAgent,
  reducedMotion: boolean,
): OfficeAgentSceneState {
  const availability =
    agent.availability.kind === "known"
      ? agent.availability.value
      : "unknown";
  const workload =
    agent.workload.kind === "known" ? agent.workload.value : "unknown";
  const runningCount =
    agent.workload.kind === "known" ? agent.workload.runningCount : 0;
  const queuedCount =
    agent.workload.kind === "known" ? agent.workload.queuedCount : 0;

  const clip =
    availability === "unknown" || workload === "unknown"
      ? "wait"
      : availability === "offline"
        ? "offline"
        : workload === "working"
          ? "work"
          : workload === "queued"
            ? "wait"
            : "idle";

  return {
    availability,
    workload,
    runningCount,
    queuedCount,
    clip,
    stationLit: workload === "working",
    ambientMotion:
      !reducedMotion && availability === "online" && workload === "idle",
    pulse: !reducedMotion && availability === "unstable",
    flicker: !reducedMotion && availability === "unstable",
  };
}
