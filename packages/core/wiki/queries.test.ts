import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import {
  wikiKeys,
  wikiPageDetailOptions,
  wikiPageListOptions,
  wikiProposalListOptions,
  wikiRevisionListOptions,
  wikiSearchOptions,
  lmWikiSourcePolicyOptions,
  wikiKnowledgeReadinessOptions,
  personalWikiKeys,
  personalWikiPageDetailOptions,
  personalWikiPageListOptions,
  personalWikiRevisionDetailOptions,
  personalWikiRevisionListOptions,
  personalWikiSearchOptions,
  wikiRevisionDetailOptions,
} from "./queries";

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
    expect(wikiKeys.list("ws-1", { scope: "project", projectId: "p1" })).toEqual([
      "wiki",
      "ws-1",
      "list",
      "project",
      "p1",
    ]);
  });

  it("namespaces search, revision, and proposal keys by workspace and page", () => {
    expect(wikiKeys.search("ws-1", { q: " guide " })).toEqual([
      "wiki", "ws-1", "search", "guide", "all", "",
    ]);
    expect(wikiKeys.revisions("ws-1", "page-1")).not.toEqual(
      wikiKeys.revisions("ws-2", "page-1"),
    );
    expect(wikiKeys.proposals("ws-1", "page-1")).not.toEqual(
      wikiKeys.proposals("ws-1", "page-2"),
    );
    expect(wikiKeys.sourcePolicy("ws-1")).not.toEqual(wikiKeys.sourcePolicy("ws-2"));
    expect(wikiKeys.readiness("ws-1")).not.toEqual(wikiKeys.readiness("ws-2"));
  });
});

describe("Personal Wiki queries", () => {
  it("uses a global cache namespace and does not require a workspace", async () => {
    const api = {
      listPersonalWikiPages: vi.fn().mockResolvedValue([]),
      searchPersonalWikiPages: vi.fn().mockResolvedValue([]),
      getPersonalWikiPage: vi.fn().mockResolvedValue({ id: "page-1" }),
      listPersonalWikiRevisions: vi.fn().mockResolvedValue([]),
      getPersonalWikiRevision: vi.fn().mockResolvedValue({ id: "revision-1" }),
    };
    setApiInstance(api as unknown as ApiClient);

    expect(personalWikiKeys.all).toEqual(["personal-wiki"]);
    await personalWikiPageListOptions().queryFn!({} as never);
    await personalWikiPageDetailOptions("page-1").queryFn!({} as never);
    await personalWikiRevisionListOptions("page-1").queryFn!({} as never);
    await personalWikiRevisionDetailOptions("revision-1").queryFn!({} as never);
    const search = personalWikiSearchOptions({ q: "  private  " });
    await search.queryFn!({} as never);

    expect(search.enabled).toBe(true);
    expect(personalWikiSearchOptions({ q: "x" }).enabled).toBe(false);
    expect(api.searchPersonalWikiPages).toHaveBeenCalledWith({ q: "private" });
  });

  it("loads a workspace-bound immutable revision", async () => {
    const getWikiRevision = vi.fn().mockResolvedValue({ id: "revision-1" });
    setApiInstance({ getWikiRevision } as unknown as ApiClient);
    const options = wikiRevisionDetailOptions("ws-1", "revision-1");
    await options.queryFn!({} as never);
    expect(options.enabled).toBe(true);
    expect(getWikiRevision).toHaveBeenCalledWith("revision-1");
  });
});

describe("wikiPageListOptions", () => {
  it("disables project lists until projectId is provided", () => {
    expect(wikiPageListOptions("ws-1", { scope: "project" }).enabled).toBe(false);
    expect(wikiPageListOptions("ws-1", { scope: "project", projectId: "p1" }).enabled).toBe(true);
  });

  it("queryFn calls listWikiPages on the api singleton", async () => {
    const listWikiPages = vi.fn().mockResolvedValue([]);
    setApiInstance({ listWikiPages } as unknown as ApiClient);

    const options = wikiPageListOptions("ws-1", { scope: "user" });
    await expect(options.queryFn!({} as never)).resolves.toEqual([]);
    expect(listWikiPages).toHaveBeenCalledWith({ scope: "user", projectId: undefined });
  });

  it("detail options call getWikiPage", async () => {
    const getWikiPage = vi.fn().mockResolvedValue({
      id: "p1",
      workspaceId: null,
      scope: "user",
      path: "notes.md",
      title: "Notes",
      content: "hi",
      projectId: null,
      ownerUserId: "u1",
      createdBy: "u1",
      createdAt: "",
      updatedAt: "",
    });
    setApiInstance({ getWikiPage } as unknown as ApiClient);

    const options = wikiPageDetailOptions("ws-1", "p1");
    expect(options.enabled).toBe(true);
    expect(options.retry).toBe(false);
    expect(personalWikiPageDetailOptions("p1").retry).toBe(false);
    await expect(options.queryFn!({} as never)).resolves.toMatchObject({ id: "p1", workspaceId: null });
    expect(getWikiPage).toHaveBeenCalledWith("p1");
  });

  it("only searches non-trivial queries and forwards normalized filters", async () => {
    const searchWikiPages = vi.fn().mockResolvedValue([]);
    setApiInstance({ searchWikiPages } as unknown as ApiClient);

    expect(wikiSearchOptions("ws-1", { q: "x" }).enabled).toBe(false);
    const options = wikiSearchOptions("ws-1", {
      q: "  handbook  ",
      scope: "project",
      projectId: "project-1",
    });
    expect(options.enabled).toBe(true);
    await expect(options.queryFn!({} as never)).resolves.toEqual([]);
    expect(searchWikiPages).toHaveBeenCalledWith({
      q: "handbook",
      scope: "project",
      projectId: "project-1",
    });
  });

  it("loads revision history and proposals for the selected page", async () => {
    const listWikiRevisions = vi.fn().mockResolvedValue([]);
    const listWikiProposals = vi.fn().mockResolvedValue([]);
    setApiInstance({ listWikiRevisions, listWikiProposals } as unknown as ApiClient);

    await wikiRevisionListOptions("ws-1", "page-1").queryFn!({} as never);
    await wikiProposalListOptions("ws-1", "page-1").queryFn!({} as never);
    expect(listWikiRevisions).toHaveBeenCalledWith("page-1");
    expect(listWikiProposals).toHaveBeenCalledWith("page-1");
  });

  it("loads the LM Wiki source policy in the workspace cache namespace", async () => {
    const getLMWikiSourcePolicy = vi.fn().mockResolvedValue({
      sourceClasses: [],
      wikiPages: [],
      remoteGenerationEnabled: false,
      policyVersion: 0,
      policyDigest: "",
      exclusions: [],
    });
    setApiInstance({ getLMWikiSourcePolicy } as unknown as ApiClient);
    const options = lmWikiSourcePolicyOptions("ws-1");
    await expect(options.queryFn!({} as never)).resolves.toEqual({
      sourceClasses: [],
      wikiPages: [],
      remoteGenerationEnabled: false,
      policyVersion: 0,
      policyDigest: "",
      exclusions: [],
    });
    expect(getLMWikiSourcePolicy).toHaveBeenCalledOnce();
  });

  it("loads server-derived knowledge readiness in the workspace namespace", async () => {
    const getWikiKnowledgeReadiness = vi.fn().mockResolvedValue({
      schemaVersion: 1,
      policy: { sourceClasses: [], wikiPages: [], remoteGenerationEnabled: false },
      sources: [],
      maintenanceItems: [],
      truncated: false,
      canManage: false,
    });
    setApiInstance({ getWikiKnowledgeReadiness } as unknown as ApiClient);
    const options = wikiKnowledgeReadinessOptions("ws-1");
    await options.queryFn!({} as never);
    expect(options.enabled).toBe(true);
    expect(getWikiKnowledgeReadiness).toHaveBeenCalledOnce();
  });
});
