// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  canReviewWikiProposal,
  wikiProjectPickerScreenOptions,
  wikiProposalReviewRoute,
} from "./wiki-navigation";

describe("Mobile Wiki human review navigation", () => {
  it("opens the exact proposal review route", () => {
    expect(wikiProposalReviewRoute("acme", "page-1", "proposal-1")).toEqual({
      pathname: "/[workspace]/wiki/[id]/proposal/[proposalId]",
      params: {
        workspace: "acme",
        id: "page-1",
        proposalId: "proposal-1",
      },
    });
  });

  it("keeps completed and unknown proposals read-only", () => {
    expect(canReviewWikiProposal("pending")).toBe(true);
    expect(canReviewWikiProposal("accepted")).toBe(false);
    expect(canReviewWikiProposal("rejected")).toBe(false);
    expect(canReviewWikiProposal("unknown")).toBe(false);
  });

  it("keeps shared formSheet behavior while showing the native search header", () => {
    expect(
      wikiProjectPickerScreenOptions({
        presentation: "formSheet",
        sheetGrabberVisible: true,
        sheetAllowedDetents: [0.6, 0.95],
        headerShown: false,
      }),
    ).toEqual({
      presentation: "formSheet",
      sheetGrabberVisible: true,
      sheetAllowedDetents: [0.6, 0.95],
      headerShown: true,
      title: "Project",
    });
  });
});
