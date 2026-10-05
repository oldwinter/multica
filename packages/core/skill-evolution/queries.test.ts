// @vitest-environment node

import { describe, expect, it } from "vitest";
import { EMPTY_SKILL_EVOLUTION_OVERVIEW } from "./schemas";
import {
  SKILL_EVOLUTION_POLL_INTERVAL_MS,
  SKILL_EVOLUTION_POLL_WINDOW_MS,
  skillEvolutionKeys,
  skillEvolutionOverviewOptions,
  skillEvolutionOverviewPollingInterval,
  skillEvolutionProposalOptions,
  skillEvolutionProposalPollingInterval,
} from "./queries";
import type { SkillEvolutionProposalSummary } from "./types";

const NOW = Date.parse("2026-08-28T12:00:00Z");

function proposal(
  state: SkillEvolutionProposalSummary["state"],
  updatedAt = "2026-08-28T11:59:30Z",
): SkillEvolutionProposalSummary {
  return {
    id: "proposal-1",
    skillId: "skill-1",
    state,
    baseRevisionId: "revision-1",
    baseHash: "sha256:base",
    candidateRevisionId: null,
    candidateHash: null,
    failureReason: null,
    staleReason: null,
    createdAt: updatedAt,
    updatedAt,
  };
}

describe("Skill evolution query ownership", () => {
  it("scopes overview and proposal caches by workspace and identity", () => {
    expect(skillEvolutionKeys.overview("ws-a", "skill-1")).toEqual([
      "workspaces", "ws-a", "skill-evolution", "skills", "skill-1", "overview",
    ]);
    expect(skillEvolutionKeys.proposal("ws-a", "proposal-1")).toEqual([
      "workspaces", "ws-a", "skill-evolution", "proposals", "proposal-1",
    ]);
    expect(skillEvolutionKeys.overview("ws-a", "skill-1"))
      .not.toEqual(skillEvolutionKeys.overview("ws-b", "skill-1"));
  });

  it("disables requests without both workspace and resource identity", () => {
    expect(skillEvolutionOverviewOptions("", "skill-1").enabled).toBe(false);
    expect(skillEvolutionOverviewOptions("ws-a", "").enabled).toBe(false);
    expect(skillEvolutionProposalOptions("", "proposal-1").enabled).toBe(false);
    expect(skillEvolutionProposalOptions("ws-a", "").enabled).toBe(false);
  });

  it("keeps interval polling out of background tabs", () => {
    expect(skillEvolutionOverviewOptions("ws-a", "skill-1").refetchIntervalInBackground)
      .toBe(false);
    expect(skillEvolutionProposalOptions("ws-a", "proposal-1").refetchIntervalInBackground)
      .toBe(false);
  });
});

describe("Skill evolution bounded polling", () => {
  it("polls only known in-flight proposal states", () => {
    for (const state of ["queued", "running", "publishing"] as const) {
      expect(skillEvolutionProposalPollingInterval(proposal(state), NOW))
        .toBe(SKILL_EVOLUTION_POLL_INTERVAL_MS);
    }
    for (const state of ["ready", "failed", "stale", "publication_unknown", "unknown"] as const) {
      expect(skillEvolutionProposalPollingInterval(proposal(state), NOW)).toBe(false);
    }
  });

  it("stops polling a stuck proposal after the bounded window", () => {
    const staleTimestamp = new Date(NOW - SKILL_EVOLUTION_POLL_WINDOW_MS - 1).toISOString();
    expect(skillEvolutionProposalPollingInterval(proposal("queued", staleTimestamp), NOW))
      .toBe(false);
    const implausiblyFutureTimestamp = new Date(
      NOW + SKILL_EVOLUTION_POLL_WINDOW_MS + 1,
    ).toISOString();
    expect(skillEvolutionProposalPollingInterval(
      proposal("queued", implausiblyFutureTimestamp),
      NOW,
    )).toBe(false);
  });

  it("wakes for a near scheduled run but ignores distant or inactive loops", () => {
    const scheduled = {
      ...EMPTY_SKILL_EVOLUTION_OVERVIEW,
      loop: {
        id: "loop-1",
        enabled: true,
        mode: "propose" as const,
        cooldownSeconds: 3600,
        minimumSignals: 1,
        maxEvidenceRefs: 5,
        maxReplaySamples: 2,
        maxCostUsdTicks: 10,
        policyVersion: "v1",
        lastObservedAt: null,
        lastProposalAt: null,
        nextEligibleAt: new Date(NOW + 30_000).toISOString(),
        updatedAt: new Date(NOW).toISOString(),
      },
    };
    expect(skillEvolutionOverviewPollingInterval(scheduled, NOW)).toBe(30_000);
    expect(skillEvolutionOverviewPollingInterval({
      ...scheduled,
      loop: { ...scheduled.loop, nextEligibleAt: new Date(NOW + 10 * 60_000).toISOString() },
    }, NOW)).toBe(false);
    expect(skillEvolutionOverviewPollingInterval({
      ...scheduled,
      loop: { ...scheduled.loop, enabled: false },
    }, NOW)).toBe(false);
  });
});
