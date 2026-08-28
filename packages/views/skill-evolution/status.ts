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
