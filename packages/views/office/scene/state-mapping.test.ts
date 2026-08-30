// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { OfficeAgent } from "@multica/core/office";
import { mapAgentSceneState } from "./state-mapping";

function agent(
  availability: OfficeAgent["availability"],
  workload: OfficeAgent["workload"],
): OfficeAgent {
  return {
    id: "agent-1",
    name: "must-not-enter-scene",
    avatarUrl: null,
    description: "must-not-enter-scene",
    availability,
    workload,
    activeIssueIds: [],
  };
}

describe("Office scene state mapping", () => {
  it("preserves independent offline availability and working workload", () => {
    const state = mapAgentSceneState(
      agent(
        { kind: "known", value: "offline" },
        {
          kind: "known",
          value: "working",
          runningCount: 2,
          queuedCount: 1,
          capacity: 3,
        },
      ),
      false,
    );

    expect(state.availability).toBe("offline");
    expect(state.workload).toBe("working");
    expect(state.runningCount).toBe(2);
    expect(state.queuedCount).toBe(1);
    expect(state.clip).toBe("offline");
  });

  it("maps either unknown axis to a neutral hold without inventing activity", () => {
    const state = mapAgentSceneState(
      agent(
        { kind: "unknown", reason: "unavailable" },
        { kind: "unknown", reason: "loading", capacity: 1 },
      ),
      false,
    );

    expect(state.availability).toBe("unknown");
    expect(state.workload).toBe("unknown");
    expect(state.clip).toBe("wait");
    expect(state.stationLit).toBe(false);
    expect(state.ambientMotion).toBe(false);
  });

  it("reduced motion removes every moving channel but preserves static truth", () => {
    const state = mapAgentSceneState(
      agent(
        { kind: "known", value: "unstable" },
        {
          kind: "known",
          value: "working",
          runningCount: 1,
          queuedCount: 0,
          capacity: 2,
        },
      ),
      true,
    );

    expect(state.availability).toBe("unstable");
    expect(state.workload).toBe("working");
    expect(state.stationLit).toBe(true);
    expect(state.ambientMotion).toBe(false);
    expect(state.pulse).toBe(false);
    expect(state.flicker).toBe(false);
  });
});
