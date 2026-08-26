// @vitest-environment node

import { describe, expect, it, vi } from "vitest";

vi.mock("@/data/api", () => ({ api: {} }));

import { wikiKeys } from "./wiki";

describe("mobile Wiki query identity", () => {
  it("tenants every cache by workspace and scope", () => {
    expect(wikiKeys.list("ws-1", { scope: "workspace" })).toEqual([
      "wiki",
      "ws-1",
      "list",
      "workspace",
      "",
    ]);
    expect(
      wikiKeys.list("ws-1", { scope: "project", projectId: "project-1" }),
    ).toEqual(["wiki", "ws-1", "list", "project", "project-1"]);
  });

  it("separates search terms and page-owned records", () => {
    expect(
      wikiKeys.search("ws-1", "交接", { scope: "workspace" }),
    ).toEqual(["wiki", "ws-1", "search", "workspace", "", "交接"]);
    expect(wikiKeys.detail("ws-1", "page-1")).toEqual([
      "wiki",
      "ws-1",
      "detail",
      "page-1",
    ]);
    expect(wikiKeys.revisions("ws-1", "page-1")).toEqual([
      "wiki",
      "ws-1",
      "detail",
      "page-1",
      "revisions",
    ]);
    expect(wikiKeys.proposals("ws-1", "page-1")).toEqual([
      "wiki",
      "ws-1",
      "detail",
      "page-1",
      "proposals",
    ]);
    expect(wikiKeys.readiness("ws-1")).toEqual([
      "wiki",
      "ws-1",
      "knowledge-readiness",
    ]);
  });
});
