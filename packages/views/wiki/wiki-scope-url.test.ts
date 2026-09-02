// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  buildWikiScopePath,
  parseWikiScopeSelection,
  type WikiScopeSelection,
} from "./wiki-scope-url";

function location(search: string, hash = "#wiki-note") {
  return {
    pathname: "/acme/wiki",
    searchParams: new URLSearchParams(search),
    hash,
  };
}

describe("Wiki collection scope URL state", () => {
  it.each([
    ["", { scope: "workspace", projectId: null }],
    ["scope=workspace&project_id=stale", { scope: "workspace", projectId: null }],
    ["scope=unknown", { scope: "workspace", projectId: null }],
    ["scope=user", { scope: "workspace", projectId: null }],
    ["scope=project&project_id=project-1", { scope: "project", projectId: "project-1" }],
    ["scope=project&project_id=%20", { scope: "project", projectId: " " }],
    ["scope=project", { scope: "project", projectId: null }],
  ])("parses %s", (search, expected) => {
    expect(parseWikiScopeSelection(new URLSearchParams(search))).toEqual(expected);
  });

  it("removes collection keys for Workspace without mutating the live location", () => {
    const source = location("filter=recent&scope=project&project_id=p1&filter=all");

    expect(buildWikiScopePath(source, { scope: "workspace", projectId: null })).toBe(
      "/acme/wiki?filter=recent&filter=all#wiki-note",
    );
    expect(source.searchParams.toString()).toBe(
      "filter=recent&scope=project&project_id=p1&filter=all",
    );
  });

  it("serializes a Project with an encoded opaque id and preserves unrelated state", () => {
    const source = location("filter=recent&scope=workspace", "#project-note");
    const selection: WikiScopeSelection = { scope: "project", projectId: "project/1" };

    expect(buildWikiScopePath(source, selection)).toBe(
      "/acme/wiki?filter=recent&scope=project&project_id=project%2F1#project-note",
    );
  });

  it("keeps Project scope when its id is not selected yet", () => {
    expect(buildWikiScopePath(location("view=grid"), { scope: "project", projectId: null })).toBe(
      "/acme/wiki?view=grid&scope=project#wiki-note",
    );
  });

  it("collapses duplicate target keys while retaining duplicate unrelated keys", () => {
    expect(buildWikiScopePath(
      location("scope=workspace&scope=project&project_id=old&tag=a&tag=b"),
      { scope: "project", projectId: "new" },
    )).toBe("/acme/wiki?scope=project&project_id=new&tag=a&tag=b#wiki-note");
  });
});
