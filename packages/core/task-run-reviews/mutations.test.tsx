// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance, type ApiClient } from "../api";
import { useCreateTaskRunReview } from "./mutations";
import { taskRunReviewKeys } from "./queries";

afterEach(() => vi.restoreAllMocks());

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe("useCreateTaskRunReview", () => {
  it("keeps request state in a workspace-scoped React Query mutation", async () => {
    const createTaskRunReview = vi.fn().mockResolvedValue({ id: "review-1" });
    setApiInstance({ createTaskRunReview } as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const hook = renderHook(
      () => useCreateTaskRunReview("workspace-1", "task-1"),
      { wrapper: wrapper(client) },
    );
    const input = {
      outcome: "helpful" as const,
      target: "knowledge" as const,
      reason: "The cited source made the answer verifiable.",
      idempotencyKey: "review-key-1",
    };

    await act(async () => {
      await hook.result.current.mutateAsync(input);
    });

    expect(createTaskRunReview).toHaveBeenCalledWith("task-1", input);
    expect(client.getMutationCache().find({
      mutationKey: [...taskRunReviewKeys.all("workspace-1"), "create", "task-1"],
      exact: true,
    })?.state.status).toBe("success");
    client.clear();
  });

  it("exposes server errors without converting them to local success", async () => {
    setApiInstance({
      createTaskRunReview: vi.fn().mockRejectedValue(new Error("review unavailable")),
    } as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    const hook = renderHook(
      () => useCreateTaskRunReview("workspace-1", "task-1"),
      { wrapper: wrapper(client) },
    );

    await act(async () => {
      await expect(hook.result.current.mutateAsync({
        outcome: "helpful",
        target: "knowledge",
        reason: "The cited source made the answer verifiable.",
        idempotencyKey: "review-key-1",
      })).rejects.toThrow("review unavailable");
    });

    expect(hook.result.current.isError).toBe(true);
    client.clear();
  });
});
