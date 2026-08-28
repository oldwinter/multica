// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  isProposalActionable,
  isProposalPending,
  normalizeLoopMode,
  proposalStatusTone,
  releaseStatusTone,
} from "./status";

describe("skill evolution status presentation", () => {
  it("fails unknown loop modes closed", () => {
    expect(normalizeLoopMode("observe")).toBe("observe");
    expect(normalizeLoopMode("future-mode")).toBe("unknown");
  });

  it("keeps unknown proposal and release states neutral", () => {
    expect(proposalStatusTone("future-state")).toBe("neutral");
    expect(releaseStatusTone("future-outcome")).toBe("neutral");
  });

  it("polls only states with server work still in flight", () => {
    expect(isProposalPending("queued")).toBe(true);
    expect(isProposalPending("running")).toBe(true);
    expect(isProposalPending("publishing")).toBe(true);
    expect(isProposalPending("publication_unknown")).toBe(false);
  });

  it("allows human decisions only for ready proposals", () => {
    expect(isProposalActionable("ready")).toBe(true);
    expect(isProposalActionable("stale")).toBe(false);
    expect(isProposalActionable("unknown")).toBe(false);
  });
});
