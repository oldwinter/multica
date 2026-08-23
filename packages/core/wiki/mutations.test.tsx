/** @vitest-environment jsdom */
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import {
  useAcceptWikiProposal,
  useCreateWikiPage,
  useDeleteWikiPage,
  useRestoreWikiRevision,
  useUpdateWikiPage,
  useUpdateLMWikiSourcePolicy,
  useCreatePersonalWikiPage,
  useDeletePersonalWikiPage,
  useRestorePersonalWikiRevision,
  useUpdatePersonalWikiPage,
} from "./mutations";
import { personalWikiKeys, wikiKeys } from "./queries";

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

afterEach(() => vi.restoreAllMocks());

describe("Wiki mutations", () => {
  it("keeps Personal Wiki writes in a workspace-free cache namespace", async () => {
    const api = {
      createPersonalWikiPage: vi.fn().mockResolvedValue({ id: "page-1" }),
      updatePersonalWikiPage: vi.fn().mockResolvedValue({ id: "page-1" }),
      deletePersonalWikiPage: vi.fn().mockResolvedValue(undefined),
      restorePersonalWikiRevision: vi.fn().mockResolvedValue({ id: "page-1" }),
    };
    setApiInstance(api as unknown as ApiClient);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const options = { wrapper: wrapper(client) };
    const create = renderHook(() => useCreatePersonalWikiPage(), options);
    const update = renderHook(() => useUpdatePersonalWikiPage("page-1"), options);
    const remove = renderHook(() => useDeletePersonalWikiPage(), options);
    const restore = renderHook(() => useRestorePersonalWikiRevision("page-1"), options);

    await act(async () => {
      await create.result.current.mutateAsync({ path: "notes.md", content: "private" });
      await update.result.current.mutateAsync({ expectedRevisionNumber: 1, content: "new" });
      await restore.result.current.mutateAsync({ revisionId: "revision-1", expectedRevisionNumber: 2 });
      await remove.result.current.mutateAsync("page-1");
    });

    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: personalWikiKeys.all }));
    expect(invalidate).toHaveBeenCalledTimes(4);
    expect(api.updatePersonalWikiPage).toHaveBeenCalledWith("page-1", {
      expectedRevisionNumber: 1,
      content: "new",
    });
    expect(api.restorePersonalWikiRevision).toHaveBeenCalledWith("page-1", "revision-1", 2);
    client.clear();
  });

  it("keeps write logic in core and invalidates the workspace Wiki tree", async () => {
    const api = {
      createWikiPage: vi.fn().mockResolvedValue({ id: "page-1" }),
      updateWikiPage: vi.fn().mockResolvedValue({ id: "page-1" }),
      deleteWikiPage: vi.fn().mockResolvedValue(undefined),
      restoreWikiRevision: vi.fn().mockResolvedValue({ id: "page-1" }),
      acceptWikiProposal: vi.fn().mockResolvedValue({ id: "page-1" }),
      updateLMWikiSourcePolicy: vi.fn().mockResolvedValue({
        sourceClasses: [],
        wikiPages: [],
        remoteGenerationEnabled: false,
        policyVersion: 0,
        policyDigest: "",
        exclusions: [],
      }),
    };
    setApiInstance(api as unknown as ApiClient);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const options = { wrapper: wrapper(client) };
    const create = renderHook(() => useCreateWikiPage("ws-1"), options);
    const update = renderHook(() => useUpdateWikiPage("ws-1", "page-1"), options);
    const remove = renderHook(() => useDeleteWikiPage("ws-1"), options);
    const restore = renderHook(() => useRestoreWikiRevision("ws-1", "page-1"), options);
    const accept = renderHook(() => useAcceptWikiProposal("ws-1", "page-1"), options);
    const updatePolicy = renderHook(() => useUpdateLMWikiSourcePolicy("ws-1"), options);

    await act(async () => {
      await create.result.current.mutateAsync({ scope: "workspace", path: "guide.md" });
      await update.result.current.mutateAsync({ expectedRevisionNumber: 2, content: "new" });
      await remove.result.current.mutateAsync("page-1");
      await restore.result.current.mutateAsync({ revisionId: "revision-1", expectedRevisionNumber: 3 });
      await accept.result.current.mutateAsync({ proposalId: "proposal-1", expectedRevisionNumber: 4 });
      await updatePolicy.result.current.mutateAsync({
        sourceClasses: ["issue", "wiki_page"],
        wikiPages: [{ pageId: "page-1", revisionNumber: 4 }],
        remoteGenerationEnabled: false,
      });
    });

    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: wikiKeys.all("ws-1") }));
    expect(invalidate).toHaveBeenCalledTimes(6);
    expect(api.updateWikiPage).toHaveBeenCalledWith("page-1", {
      expectedRevisionNumber: 2,
      content: "new",
    });
    expect(api.restoreWikiRevision).toHaveBeenCalledWith("page-1", "revision-1", 3);
    expect(api.updateLMWikiSourcePolicy).toHaveBeenCalledWith({
      sourceClasses: ["issue", "wiki_page"],
      wikiPages: [{ pageId: "page-1", revisionNumber: 4 }],
      remoteGenerationEnabled: false,
    });
    client.clear();
  });
});
