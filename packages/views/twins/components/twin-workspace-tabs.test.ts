import { describe, expect, it } from "vitest";
import {
  DEFAULT_TWIN_WORKSPACE_TAB,
  buildTwinWorkspaceTabPath,
  isTwinWorkspaceTab,
  parseTwinWorkspaceTab,
  type TwinWorkspaceTab,
} from "./twin-workspace-tabs";

function location(search: string, hash = "#evidence-1") {
  return {
    pathname: "/acme/twins",
    searchParams: new URLSearchParams(search),
    hash,
  };
}

describe("Twin workspace tab URL state", () => {
  it.each([
    ["wiki", "wiki"],
    ["twin", "twin"],
    ["use", "use"],
  ])("accepts %s", (value, expected) => {
    expect(isTwinWorkspaceTab(value)).toBe(true);
    expect(parseTwinWorkspaceTab(value)).toBe(expected);
  });

  it.each([null, "", "future", "WIKI"]) (
    "fails closed for %s",
    (value) => {
      expect(isTwinWorkspaceTab(value)).toBe(false);
      expect(parseTwinWorkspaceTab(value)).toBe(DEFAULT_TWIN_WORKSPACE_TAB);
    },
  );

  it("removes the default tab while preserving unrelated query parameters and hash", () => {
    const source = location("view=activity&tab=wiki&filter=open", "#comment-7");
    const result = buildTwinWorkspaceTabPath(source, "wiki");

    expect(result).toBe("/acme/twins?view=activity&filter=open#comment-7");
    expect(source.searchParams.toString()).toBe("view=activity&tab=wiki&filter=open");
  });

  it("sets a non-default tab without mutating the adapter search params", () => {
    const source = location("view=activity&tab=wiki&filter=open");
    const result = buildTwinWorkspaceTabPath(source, "use");

    expect(result).toBe("/acme/twins?view=activity&tab=use&filter=open#evidence-1");
    expect(source.searchParams.toString()).toBe("view=activity&tab=wiki&filter=open");
  });

  it("collapses duplicate tab values and keeps duplicate unrelated values", () => {
    const result = buildTwinWorkspaceTabPath(
      location("tag=one&tab=twin&tag=two&tab=wiki"),
      "twin",
    );

    expect(result).toBe("/acme/twins?tag=one&tab=twin&tag=two#evidence-1");
  });

  it("accepts the tab union as the path builder input", () => {
    const tabs: TwinWorkspaceTab[] = ["wiki", "twin", "use"];
    expect(tabs.map((tab) => buildTwinWorkspaceTabPath(location(""), tab))).toEqual([
      "/acme/twins#evidence-1",
      "/acme/twins?tab=twin#evidence-1",
      "/acme/twins?tab=use#evidence-1",
    ]);
  });
});
