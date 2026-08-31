import { describe, expect, it } from "vitest";
import type { RoomArtifact } from "@multica/core/rooms";
import {
  artifactHref,
  countTodayTurns,
  latestRefusedCycle,
  roomStatusClass,
} from "./room-display";

describe("room display helpers", () => {
  it("keeps active badge text readable on the success tint", () => {
    expect(roomStatusClass("active")).toContain("text-foreground");
    expect(roomStatusClass("active")).not.toContain("text-success");
  });

  it("counts only turns created during the current UTC day", () => {
    const detail = {
      turns: [
        { created_at: "2026-08-13T00:00:00.000Z", status: "completed" },
        { created_at: "2026-08-13T23:59:59.000Z", status: "running" },
        { created_at: "2026-08-13T12:00:00.000Z", status: "refused" },
        { created_at: "2026-08-12T23:59:59.000Z", status: "completed" },
      ],
    };

    expect(countTodayTurns(detail, new Date("2026-08-13T10:30:00.000Z"))).toBe(2);
  });

  it("returns the most recent refused cycle", () => {
    const cycles = [
      { id: "first", status: "refused" },
      { id: "middle", status: "completed" },
      { id: "latest", status: "refused" },
    ] as const;

    expect(latestRefusedCycle(cycles)?.id).toBe("latest");
  });

  it("routes linked artifact kinds while keeping decisions in the room", () => {
    const paths = {
      issueDetail: (id: string) => `/issues/${id}`,
      wikiPage: (id: string) => `/wiki/${id}`,
    };
    const artifact = (
      kind: RoomArtifact["kind"],
    ): Pick<RoomArtifact, "kind" | "target_id"> => ({
      kind,
      target_id: "target",
    });

    expect(artifactHref(artifact("issue"), paths)).toBe("/issues/target");
    expect(artifactHref(artifact("wiki"), paths)).toBe("/wiki/target");
    expect(artifactHref(artifact("decision"), paths)).toBeNull();
  });
});
