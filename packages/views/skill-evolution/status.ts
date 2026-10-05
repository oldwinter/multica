import type {
  SkillEvolutionLoop,
  SkillEvolutionRelease,
} from "@multica/core/skill-evolution";

export type EvolutionStatusTone =
  | "neutral"
  | "info"
  | "success"
  | "warning"
  | "danger";

export type KnownLoopMode = "observe" | "propose" | "paused" | "unknown";

export function normalizeLoopMode(value: string): KnownLoopMode {
  switch (value) {
    case "observe":
    case "propose":
    case "paused":
      return value;
    default:
      return "unknown";
  }
}

export function proposalStatusTone(value: string): EvolutionStatusTone {
  switch (value) {
    case "ready":
    case "published":
      return "success";
    case "queued":
    case "running":
    case "publishing":
      return "info";
    case "stale":
    case "publication_unknown":
      return "warning";
    case "failed":
    case "rejected":
      return "danger";
    default:
      return "neutral";
  }
}

export function releaseStatusTone(value: string): EvolutionStatusTone {
  switch (value) {
    case "succeeded":
      return "success";
    case "pending":
      return "info";
    case "publication_unknown":
      return "warning";
    case "failed":
      return "danger";
    default:
      return "neutral";
  }
}

export function isProposalPending(value: string): boolean {
  return value === "queued" || value === "running" || value === "publishing";
}

export function isProposalActionable(value: string): boolean {
  return value === "ready";
}

export function canRequestSkillEvolutionProposal(
  loop: Pick<SkillEvolutionLoop, "enabled" | "mode"> | null | undefined,
): boolean {
  return loop?.enabled === true && loop.mode === "propose";
}

export function isPublicationUnknown(
  release: Pick<SkillEvolutionRelease, "outcome">,
): boolean {
  return release.outcome === "publication_unknown" || release.outcome === "unknown";
}

export function isProposalPublicationUnknown(value: string): boolean {
  return value === "publication_unknown" || value === "proposal_publication_unknown";
}
