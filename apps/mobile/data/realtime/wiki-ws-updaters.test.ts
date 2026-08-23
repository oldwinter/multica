// @vitest-environment node

import { describe, expect, it, vi } from "vitest";

vi.mock("@/data/queries/wiki", () => ({
  wikiKeys: {
    all: (wsId: string) => ["wiki", wsId],
    detail: (wsId: string, pageId: string) => ["wiki", wsId, "detail", pageId],
    revisions: (wsId: string, pageId: string) => [
      "wiki",
      wsId,
      "detail",
      pageId,
      "revisions",
    ],
    proposals: (wsId: string, pageId: string) => [
      "wiki",
      wsId,
      "detail",
      pageId,
      "proposals",
    ],
  },
}));

import {
  invalidateWikiCollections,
  invalidateWikiPage,
  invalidateWikiTreeForUnknownEvent,
  wikiProposalReviewTargets,
} from "./wiki-ws-updaters";

describe("Wiki realtime payload boundary", () => {
  it("invalidates only list and search projections in the active workspace", () => {
    const invalidateQueries = vi.fn();
    invalidateWikiCollections(
      { invalidateQueries } as never,
      "ws-1",
    );

    const { predicate } = invalidateQueries.mock.calls[0][0];
    expect(predicate({ queryKey: ["wiki", "ws-1", "list"] })).toBe(true);
    expect(predicate({ queryKey: ["wiki", "ws-1", "search"] })).toBe(true);
    expect(predicate({ queryKey: ["wiki", "ws-1", "detail", "page-1"] })).toBe(false);
    expect(predicate({ queryKey: ["wiki", "ws-2", "list"] })).toBe(false);
  });

  it("invalidates only the selected page projections with exact keys", () => {
    const invalidateQueries = vi.fn();
    invalidateWikiPage(
      { invalidateQueries } as never,
      "ws-1",
      "page-1",
      { detail: false, revisions: false, proposals: true },
    );

    expect(invalidateQueries).toHaveBeenCalledTimes(1);
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["wiki", "ws-1", "detail", "page-1", "proposals"],
      exact: true,
    });
  });

  it("keeps known review outcomes precise and fails safe for a new outcome", () => {
    expect(wikiProposalReviewTargets("rejected")).toEqual({
      detail: false,
      revisions: false,
      proposals: true,
    });
    expect(wikiProposalReviewTargets("accepted")).toEqual({
      detail: true,
      revisions: true,
      proposals: true,
    });
    expect(wikiProposalReviewTargets("superseded")).toEqual({
      detail: true,
      revisions: true,
      proposals: true,
    });
  });

  it("invalidates the Wiki tree only for unknown Wiki lifecycle events", () => {
    const invalidateQueries = vi.fn();
    const qc = { invalidateQueries } as never;

    invalidateWikiTreeForUnknownEvent(qc, "ws-1", "wiki:page_updated");
    invalidateWikiTreeForUnknownEvent(qc, "ws-1", "issue:updated");
    expect(invalidateQueries).not.toHaveBeenCalled();

    invalidateWikiTreeForUnknownEvent(qc, "ws-1", "wiki:page_published");
    expect(invalidateQueries).toHaveBeenCalledOnce();
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["wiki", "ws-1"],
    });
  });
});
