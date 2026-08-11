import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { wikiKeys, wikiPageListOptions } from "./queries";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("wikiKeys", () => {
  it("namespaces list keys by scope and project", () => {
    expect(wikiKeys.list("ws-1", { scope: "workspace" })).toEqual([
      "wiki",
      "ws-1",
      "list",
      "workspace",
      "",
    ]);
    expect(wikiKeys.list("ws-1", { scope: "project", project_id: "p1" })).toEqual([
      "wiki",
      "ws-1",
      "list",
      "project",
      "p1",
    ]);
  });
});

describe("wikiPageListOptions", () => {
  it("disables project lists until project_id is provided", () => {
    expect(wikiPageListOptions("ws-1", { scope: "project" }).enabled).toBe(false);
    expect(wikiPageListOptions("ws-1", { scope: "project", project_id: "p1" }).enabled).toBe(true);
  });

  it("queryFn calls listWikiPages on the api singleton", async () => {
    const listWikiPages = vi.fn().mockResolvedValue([]);
    setApiInstance({ listWikiPages } as unknown as ApiClient);

    const options = wikiPageListOptions("ws-1", { scope: "user" });
    await expect(options.queryFn!({} as never)).resolves.toEqual([]);
    expect(listWikiPages).toHaveBeenCalledWith({ scope: "user", project_id: undefined });
  });

  it("detail options call getWikiPage", async () => {
    const getWikiPage = vi.fn().mockResolvedValue({
      id: "p1",
      workspace_id: null,
      scope: "user",
      path: "notes.md",
      title: "Notes",
      content: "hi",
      project_id: null,
      owner_user_id: "u1",
      created_by: "u1",
      created_at: "",
      updated_at: "",
    });
    setApiInstance({ getWikiPage } as unknown as ApiClient);

    const { wikiPageDetailOptions } = await import("./queries");
    const options = wikiPageDetailOptions("ws-1", "p1");
    expect(options.enabled).toBe(true);
    await expect(options.queryFn!({} as never)).resolves.toMatchObject({ id: "p1", workspace_id: null });
    expect(getWikiPage).toHaveBeenCalledWith("p1");
  });
});
