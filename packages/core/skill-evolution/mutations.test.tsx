/** @vitest-environment jsdom */

import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import {
  useConfigureSkillEvolution,
  useForkSkillForEvolution,
  usePauseSkillEvolution,
  usePublishSkillEvolutionProposal,
  useRejectSkillEvolutionProposal,
  useRequestSkillEvolutionProposal,
  useRollbackSkillEvolutionRelease,
} from "./mutations";
import { skillEvolutionKeys } from "./queries";

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

afterEach(() => vi.restoreAllMocks());

describe("Skill evolution mutation cache ownership", () => {
  it("invalidates only the affected overview and proposal detail", async () => {
    const fakeApi = {
      configureSkillEvolution: vi.fn().mockResolvedValue({}),
      pauseSkillEvolution: vi.fn().mockResolvedValue({}),
      requestSkillEvolutionProposal: vi.fn().mockResolvedValue({ id: "proposal-1" }),
      rejectSkillEvolutionProposal: vi.fn().mockResolvedValue({ id: "proposal-1" }),
      publishSkillEvolutionProposal: vi.fn().mockResolvedValue({}),
      rollbackSkillEvolutionRelease: vi.fn().mockResolvedValue({}),
      forkSkillForEvolution: vi.fn().mockResolvedValue({ id: "skill-2" }),
    };
    setApiInstance(fakeApi as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const renderOptions = { wrapper: wrapper(client) };
    const configure = renderHook(
      () => useConfigureSkillEvolution("ws-1", "skill-1"),
      renderOptions,
    );
    const pause = renderHook(() => usePauseSkillEvolution("ws-1", "skill-1"), renderOptions);
    const request = renderHook(
      () => useRequestSkillEvolutionProposal("ws-1", "skill-1"),
      renderOptions,
    );
    const reject = renderHook(
      () => useRejectSkillEvolutionProposal("ws-1", "skill-1", "proposal-1"),
      renderOptions,
    );
    const publish = renderHook(
      () => usePublishSkillEvolutionProposal("ws-1", "skill-1", "proposal-1"),
      renderOptions,
    );
    const rollback = renderHook(
      () => useRollbackSkillEvolutionRelease("ws-1", "skill-1"),
      renderOptions,
    );
    const fork = renderHook(
      () => useForkSkillForEvolution("ws-1", "skill-1"),
      renderOptions,
    );

    await act(async () => {
      await configure.result.current.mutateAsync({
        enabled: true,
        mode: "propose",
        cooldownSeconds: 3600,
        minimumSignals: 3,
        maxEvidenceRefs: 8,
        maxReplaySamples: 4,
        maxCostUsdTicks: 50,
        policyVersion: "v1",
      });
      await pause.result.current.mutateAsync({ idempotencyKey: "pause-1" });
      await request.result.current.mutateAsync({ idempotencyKey: "request-1" });
      await reject.result.current.mutateAsync({ reason: "No", idempotencyKey: "reject-1" });
      await publish.result.current.mutateAsync({ idempotencyKey: "publish-1" });
      await rollback.result.current.mutateAsync({
        releaseId: "release-1",
        idempotencyKey: "rollback-1",
      });
      await fork.result.current.mutateAsync({ name: "Local", idempotencyKey: "fork-1" });
    });

    const overviewInvalidation = {
      queryKey: skillEvolutionKeys.overview("ws-1", "skill-1"),
      exact: true,
    };
    const proposalInvalidation = {
      queryKey: skillEvolutionKeys.proposal("ws-1", "proposal-1"),
      exact: true,
    };
    expect(invalidate).toHaveBeenCalledTimes(9);
    expect(invalidate).toHaveBeenCalledWith(overviewInvalidation);
    expect(invalidate).toHaveBeenCalledWith(proposalInvalidation);
    const allowedKeys = new Set([
      JSON.stringify(overviewInvalidation.queryKey),
      JSON.stringify(proposalInvalidation.queryKey),
    ]);
    expect(invalidate.mock.calls.every(([filters]) =>
      filters !== undefined &&
      filters.exact === true &&
      allowedKeys.has(JSON.stringify(filters.queryKey)))).toBe(true);
    expect(fakeApi.publishSkillEvolutionProposal).toHaveBeenCalledWith("proposal-1", {
      idempotencyKey: "publish-1",
    });
    expect(fakeApi.rollbackSkillEvolutionRelease).toHaveBeenCalledWith(
      "skill-1",
      "release-1",
      { idempotencyKey: "rollback-1" },
    );
    client.clear();
  });

  it("refreshes persisted stale or unknown audit state after a failed decision", async () => {
    setApiInstance({
      publishSkillEvolutionProposal: vi.fn().mockRejectedValue(new Error("stale base")),
    } as unknown as ApiClient);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const publish = renderHook(
      () => usePublishSkillEvolutionProposal("ws-1", "skill-1", "proposal-1"),
      { wrapper: wrapper(client) },
    );

    await act(async () => {
      await expect(publish.result.current.mutateAsync({
        idempotencyKey: "publish-1",
      })).rejects.toThrow("stale base");
    });

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: skillEvolutionKeys.overview("ws-1", "skill-1"),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: skillEvolutionKeys.proposal("ws-1", "proposal-1"),
      exact: true,
    });
    expect(invalidate).toHaveBeenCalledTimes(2);
    client.clear();
  });
});
