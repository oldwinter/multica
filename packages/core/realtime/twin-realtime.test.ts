// @vitest-environment node

import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { twinExecutionKeys } from "../twins/execution-queries";
import { twinKeys, twinProfileKeys } from "../twins/queries";
import { invalidateTwinRealtimeQueries } from "./use-realtime-sync";

const wsId = "workspace-1";

function invalidatedKeys(eventType: string, payload: unknown) {
  const queryClient = new QueryClient();
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  invalidateTwinRealtimeQueries(queryClient, wsId, eventType, payload);
  return invalidate.mock.calls.map(([options]) => options?.queryKey);
}

describe("Twin realtime query invalidation", () => {
  it("targets proposal overview, detail, and optional version", () => {
    expect(invalidatedKeys("twin:proposal_changed", {
      proposal_id: "proposal-1",
      state: "accepted",
      version_id: "version-1",
    })).toEqual([
      twinKeys.overview(wsId),
      twinKeys.proposal(wsId, "proposal-1"),
      twinKeys.version(wsId, "version-1"),
    ]);
  });

  it("targets the signed version, proposal, overview, and profile", () => {
    expect(invalidatedKeys("twin:version_changed", {
      version_id: "version-2",
      proposal_id: "proposal-2",
      version_number: 2,
    })).toEqual([
      twinKeys.overview(wsId),
      twinKeys.version(wsId, "version-2"),
      twinKeys.proposal(wsId, "proposal-2"),
      twinProfileKeys.overview(wsId),
    ]);
  });

  it("keeps binding refreshes inside the execution control plane", () => {
    expect(invalidatedKeys("twin:binding_changed", {
      binding_id: "binding-1",
      state: "enabled",
      twin_version_id: "version-1",
    })).toEqual([
      twinExecutionKeys.bindings(wsId),
      twinExecutionKeys.metrics(wsId),
    ]);
  });

  it("refreshes only the attributed task and related deposition surfaces", () => {
    expect(invalidatedKeys("twin:deposition_changed", {
      deposition_id: "deposition-1",
      proposal_id: "proposal-1",
      task_id: "task-1",
      base_twin_version_id: "version-1",
      state: "accepted",
    })).toEqual([
      twinExecutionKeys.taskContext(wsId, "task-1"),
      twinExecutionKeys.metrics(wsId),
      twinKeys.overview(wsId),
      twinKeys.proposal(wsId, "proposal-1"),
    ]);
  });

  it.each([
    ["twin:future_changed", { id: "opaque" }],
    ["twin:proposal_changed", { proposal_id: null }],
  ])("falls back safely for unknown or malformed event %s", (eventType, payload) => {
    expect(invalidatedKeys(eventType, payload)).toEqual([
      twinKeys.all(wsId),
      twinProfileKeys.all(wsId),
      twinExecutionKeys.all(wsId),
    ]);
  });
});
