/** @vitest-environment jsdom */
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setApiInstance } from "../api";
import {
  useAcceptLMWikiRevision,
  useAcceptTwinProposal,
  useEnsureTwinProposal,
  useRefreshLMWiki,
  useRejectLMWikiRevision,
  useRejectTwinProposal,
} from "./mutations";
import { twinKeys, twinProfileKeys, wikiKeys } from "./queries";

const WORKSPACE_A = "workspace-a";
const WORKSPACE_B = "workspace-b";

const revision = {
  id: "revision-1", revision_number: 1, schema_version: 1, source_digest: "sha256:wiki",
  content: {}, trigger_kind: "manual", requested_by_id: null, created_at: "", review: null,
};
const proposal = {
  id: "proposal-1", kind: "initial", source_wiki_revision_id: "revision-1",
  base_twin_version_id: null, schema_version: 1, content: {}, content_digest: "sha256:twin",
  requested_by_id: null, created_at: "", review: null, signed_version: null,
};
const version = {
  id: "version-1", version_number: 1, proposal_id: "proposal-1",
  source_wiki_revision_id: "revision-1", prior_version_id: null, schema_version: 1,
  content: {}, content_digest: "sha256:twin", signed_off_by_id: "member-1", signed_off_at: "", created_at: "",
};

function wrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

describe("Wiki and Twin mutations", () => {
  let queryClient: QueryClient;
  let client: ApiClient;

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client = new ApiClient("https://api.example.test");
    setApiInstance(client);
  });

  afterEach(() => {
    queryClient.clear();
    vi.restoreAllMocks();
  });

  it("settles lifecycle writes by invalidating only the initiating workspace without optimistic writes", async () => {
    vi.spyOn(client, "refreshLMWiki").mockResolvedValue({ created: true, revision });
    vi.spyOn(client, "acceptLMWikiRevision").mockResolvedValue({ revision, citations: [] });
    vi.spyOn(client, "rejectLMWikiRevision").mockResolvedValue({ revision, citations: [] });
    vi.spyOn(client, "ensureTwinProposal").mockResolvedValue({ created: true, proposal });
    vi.spyOn(client, "acceptTwinProposal").mockResolvedValue({ created: true, version });
    vi.spyOn(client, "rejectTwinProposal").mockResolvedValue({ proposal, source_revision: revision, citations: [] });
    queryClient.setQueryData(wikiKeys.overview(WORKSPACE_A), "wiki-a");
    queryClient.setQueryData(wikiKeys.overview(WORKSPACE_B), "wiki-b");
    const setQueryData = vi.spyOn(queryClient, "setQueryData");
    const cancelQueries = vi.spyOn(queryClient, "cancelQueries");
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => ({
      refresh: useRefreshLMWiki(WORKSPACE_A),
      acceptWiki: useAcceptLMWikiRevision(WORKSPACE_A),
      rejectWiki: useRejectLMWikiRevision(WORKSPACE_A),
      ensureTwin: useEnsureTwinProposal(WORKSPACE_A),
      acceptTwin: useAcceptTwinProposal(WORKSPACE_A),
      rejectTwin: useRejectTwinProposal(WORKSPACE_A),
    }), { wrapper: wrapper(queryClient) });

    await act(async () => {
      await result.current.refresh.mutateAsync();
      await result.current.acceptWiki.mutateAsync("revision-1");
      await result.current.rejectWiki.mutateAsync({ revisionId: "revision-1", reason: "not ready" });
      await result.current.ensureTwin.mutateAsync("revision-1");
      await result.current.acceptTwin.mutateAsync("proposal-1");
      await result.current.rejectTwin.mutateAsync({ proposalId: "proposal-1", reason: "not ready" });
    });

    const keys = invalidateQueries.mock.calls.map(([filters]) => filters?.queryKey);
    expect(keys).toContainEqual(wikiKeys.all(WORKSPACE_A));
    expect(keys).toContainEqual(twinKeys.all(WORKSPACE_A));
    expect(keys).toContainEqual(twinProfileKeys.all(WORKSPACE_A));
    expect(keys).not.toContainEqual(wikiKeys.all(WORKSPACE_B));
    expect(keys).not.toContainEqual(twinKeys.all(WORKSPACE_B));
    expect(keys).not.toContainEqual(twinProfileKeys.all(WORKSPACE_B));
    expect(setQueryData).not.toHaveBeenCalled();
    expect(cancelQueries).not.toHaveBeenCalled();
  });

  it("invalidates the workspace Wiki scope after an error without writing a cache artifact", async () => {
    vi.spyOn(client, "refreshLMWiki").mockRejectedValue(new Error("offline"));
    const setQueryData = vi.spyOn(queryClient, "setQueryData");
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useRefreshLMWiki(WORKSPACE_A), {
      wrapper: wrapper(queryClient),
    });

    await act(async () => {
      await expect(result.current.mutateAsync()).rejects.toThrow("offline");
    });

    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: wikiKeys.all(WORKSPACE_A) });
    expect(setQueryData).not.toHaveBeenCalled();
  });
});
