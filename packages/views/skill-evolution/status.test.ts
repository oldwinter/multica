// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  canRequestSkillEvolutionProposal,
  isProposalActionable,
  isProposalPending,
  isProposalPublicationUnknown,
  isPublicationUnknown,
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

  it("only enables proposal requests for enabled propose loops", () => {
    expect(canRequestSkillEvolutionProposal(null)).toBe(false);
    expect(canRequestSkillEvolutionProposal({ enabled: false, mode: "propose" })).toBe(false);
    expect(canRequestSkillEvolutionProposal({ enabled: true, mode: "observe" })).toBe(false);
    expect(canRequestSkillEvolutionProposal({ enabled: true, mode: "propose" })).toBe(true);
    expect(canRequestSkillEvolutionProposal({ enabled: true, mode: "unknown" })).toBe(false);
  });

  it("identifies publication outcomes that require inspection", () => {
    expect(isPublicationUnknown({ outcome: "publication_unknown" })).toBe(true);
    expect(isPublicationUnknown({ outcome: "unknown" })).toBe(true);
    expect(isPublicationUnknown({ outcome: "succeeded" })).toBe(false);
  });

  it("keeps proposal publication uncertainty explicit", () => {
    expect(isProposalPublicationUnknown("publication_unknown")).toBe(true);
    expect(isProposalPublicationUnknown("proposal_publication_unknown")).toBe(true);
    expect(isProposalPublicationUnknown("proposal_published")).toBe(false);
  });
});
