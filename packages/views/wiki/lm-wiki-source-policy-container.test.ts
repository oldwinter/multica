// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { WikiPageSummary } from "@multica/core/wiki";
import { mergeWikiPageLists } from "./lm-wiki-source-policy-container";

function page(id: string, title: string): WikiPageSummary {
  return {
    id,
    workspaceId: "workspace-1",
    scope: "workspace",
    projectId: null,
    ownerUserId: null,
    path: `${id}.md`,
    title,
    createdBy: "member-1",
    currentRevisionNumber: 1,
    currentRevisionId: `${id}-revision-1`,
    contentDigest: `sha256:${id}`,
    lastSourceKind: "human",
    lastActorType: "member",
    lastActorId: "member-1",
    createdAt: "2026-08-23T10:00:00Z",
    updatedAt: "2026-08-23T10:00:00Z",
  };
}

describe("mergeWikiPageLists", () => {
  it("deduplicates pages returned through multiple source catalog queries", () => {
    expect(mergeWikiPageLists([
      [page("page-1", "Old title")],
      [page("page-1", "Current title"), page("page-2", "Second page")],
    ])).toEqual([
      page("page-1", "Current title"),
      page("page-2", "Second page"),
    ]);
  });
});
