// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";

function jsonResponse(value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("Skill evolution ApiClient contract", () => {
  it("uses the grouped routes and serializes only strict wire inputs", async () => {
    const summary = { id: "proposal-1", skill_id: "skill-1", state: "queued" };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ skill: { id: "skill-1" } }))
      .mockResolvedValueOnce(jsonResponse({ id: "loop-1", mode: "propose" }))
      .mockResolvedValueOnce(jsonResponse({ id: "loop-1", mode: "paused" }))
      .mockResolvedValueOnce(jsonResponse({
        state: "improvement_room_queued",
        room_id: "room-1",
      }))
      .mockResolvedValueOnce(jsonResponse({ proposal: summary }))
      .mockResolvedValueOnce(jsonResponse({ ...summary, state: "rejected" }))
      .mockResolvedValueOnce(jsonResponse({ release: { id: "release-1" } }))
      .mockResolvedValueOnce(jsonResponse({ release: { id: "release-2" } }))
      .mockResolvedValueOnce(jsonResponse({ id: "skill-2", ownership: "workspace" }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.getSkillEvolutionOverview("skill/1");
    await client.configureSkillEvolution("skill-1", {
      enabled: true,
      mode: "propose",
      cooldownSeconds: 3600,
      minimumSignals: 3,
      maxEvidenceRefs: 8,
      maxReplaySamples: 4,
      maxCostUsdTicks: 50,
      policyVersion: "v1",
    });
    await client.pauseSkillEvolution("skill-1", { idempotencyKey: "pause-1" });
    await client.requestSkillEvolutionProposal("skill-1", { idempotencyKey: "request-1" });
    await client.getSkillEvolutionProposal("proposal-1");
    await client.rejectSkillEvolutionProposal("proposal-1", {
      reason: "Not enough evidence",
      idempotencyKey: "reject-1",
    });
    await client.publishSkillEvolutionProposal("proposal-1", {
      reason: "Reviewed",
      idempotencyKey: "publish-1",
    });
    await client.rollbackSkillEvolutionRelease("skill-1", "release-1", {
      idempotencyKey: "rollback-1",
    });
    await client.forkSkillForEvolution("skill-1", {
      name: "Triage local",
      idempotencyKey: "fork-1",
    });

    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      "https://api.example.test/api/skill-evolution/skills/skill%2F1",
      "https://api.example.test/api/skill-evolution/skills/skill-1/loop",
      "https://api.example.test/api/skill-evolution/skills/skill-1/pause",
      "https://api.example.test/api/skill-evolution/skills/skill-1/proposals",
      "https://api.example.test/api/skill-evolution/proposals/proposal-1",
      "https://api.example.test/api/skill-evolution/proposals/proposal-1/reject",
      "https://api.example.test/api/skill-evolution/proposals/proposal-1/publish",
      "https://api.example.test/api/skill-evolution/skills/skill-1/releases/release-1/rollback",
      "https://api.example.test/api/skill-evolution/skills/skill-1/fork",
    ]);
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({
      enabled: true,
      mode: "propose",
      cooldown_seconds: 3600,
      minimum_signals: 3,
      max_evidence_refs: 8,
      max_replay_samples: 4,
      max_cost_usd_ticks: 50,
      policy_version: "v1",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body)))
      .toEqual({ idempotency_key: "pause-1" });
    expect(JSON.parse(String(fetchMock.mock.calls[5]?.[1]?.body))).toEqual({
      reason: "Not enough evidence",
      idempotency_key: "reject-1",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[7]?.[1]?.body)))
      .toEqual({ idempotency_key: "rollback-1" });
    expect(JSON.parse(String(fetchMock.mock.calls[8]?.[1]?.body)))
      .toEqual({ name: "Triage local", idempotency_key: "fork-1" });
  });

  it("degrades a malformed response to an explicit safe empty state", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ skill: 42 })));
    const overview = await new ApiClient("https://api.example.test")
      .getSkillEvolutionOverview("skill-1");

    expect(overview.skill.id).toBe("");
    expect(overview.skill.ownership).toBe("unknown");
    expect(overview.loop).toBeNull();
    expect(overview.proposals).toEqual([]);
  });
});
