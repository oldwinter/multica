import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type {
  SkillEvolutionOverview,
  SkillEvolutionProposalDetail,
  SkillEvolutionProposalSummary,
} from "./types";

export const SKILL_EVOLUTION_POLL_INTERVAL_MS = 2_500;
export const SKILL_EVOLUTION_POLL_WINDOW_MS = 2 * 60 * 1_000;

const POLLED_PROPOSAL_STATES = new Set(["queued", "running", "publishing"]);

export const skillEvolutionKeys = {
  all: (wsId: string) => ["workspaces", wsId, "skill-evolution"] as const,
  skill: (wsId: string, skillId: string) => [
    ...skillEvolutionKeys.all(wsId), "skills", skillId,
  ] as const,
  overview: (wsId: string, skillId: string) => [
    ...skillEvolutionKeys.skill(wsId, skillId), "overview",
  ] as const,
  proposal: (wsId: string, proposalId: string) => [
    ...skillEvolutionKeys.all(wsId), "proposals", proposalId,
  ] as const,
};

function recentTimestamp(value: string, now: number): boolean {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return false;
  return Math.abs(now - timestamp) <= SKILL_EVOLUTION_POLL_WINDOW_MS;
}

export function isSkillEvolutionProposalPending(
  proposal: Pick<SkillEvolutionProposalSummary, "state">,
): boolean {
  return POLLED_PROPOSAL_STATES.has(proposal.state);
}

export function skillEvolutionProposalPollingInterval(
  proposal: Pick<SkillEvolutionProposalSummary, "state" | "updatedAt"> | undefined,
  now = Date.now(),
): number | false {
  return proposal !== undefined &&
    isSkillEvolutionProposalPending(proposal) &&
    recentTimestamp(proposal.updatedAt, now)
    ? SKILL_EVOLUTION_POLL_INTERVAL_MS
    : false;
}

export function skillEvolutionOverviewPollingInterval(
  overview: SkillEvolutionOverview | undefined,
  now = Date.now(),
): number | false {
  if (!overview) return false;

  const pending = overview.proposals.find((proposal) =>
    skillEvolutionProposalPollingInterval(proposal, now) !== false);
  if (pending) return SKILL_EVOLUTION_POLL_INTERVAL_MS;

  const loop = overview.loop;
  if (loop?.enabled !== true || loop.mode !== "propose" || loop.nextEligibleAt === null) {
    return false;
  }
  const dueAt = Date.parse(loop.nextEligibleAt);
  if (!Number.isFinite(dueAt)) return false;
  const untilDue = dueAt - now;
  if (untilDue > 0 && untilDue <= SKILL_EVOLUTION_POLL_WINDOW_MS) {
    return Math.max(SKILL_EVOLUTION_POLL_INTERVAL_MS, untilDue);
  }
  if (untilDue <= 0 && -untilDue <= SKILL_EVOLUTION_POLL_WINDOW_MS) {
    return SKILL_EVOLUTION_POLL_INTERVAL_MS;
  }
  return false;
}

export function skillEvolutionOverviewOptions(wsId: string, skillId: string) {
  return queryOptions<SkillEvolutionOverview>({
    queryKey: skillEvolutionKeys.overview(wsId, skillId),
    queryFn: ({ signal }) => api.getSkillEvolutionOverview(skillId, { signal }),
    enabled: Boolean(wsId && skillId),
    refetchInterval: (query) => skillEvolutionOverviewPollingInterval(query.state.data),
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  });
}

export function skillEvolutionProposalOptions(wsId: string, proposalId: string) {
  return queryOptions<SkillEvolutionProposalDetail>({
    queryKey: skillEvolutionKeys.proposal(wsId, proposalId),
    queryFn: ({ signal }) => api.getSkillEvolutionProposal(proposalId, { signal }),
    enabled: Boolean(wsId && proposalId),
    refetchInterval: (query) => skillEvolutionProposalPollingInterval(query.state.data?.proposal),
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  });
}
