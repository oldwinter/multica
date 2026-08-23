import type {
  RoomConflictCode,
  RoomCycle,
  RoomRecommendationReview,
  RoomReviewStatus,
  RoomSynthesis,
} from "@/data/rooms-types";

interface ApiErrorShape extends Error {
  readonly status: number;
  readonly body?: unknown;
}

function apiError(error: unknown): ApiErrorShape | null {
  if (!(error instanceof Error)) return null;
  const candidate = error as Partial<ApiErrorShape>;
  return typeof candidate.status === "number"
    ? (candidate as ApiErrorShape)
    : null;
}

export function createRoomIdempotencyKey(action: string): string {
  return `mobile:${action}:${Date.now().toString(36)}:${Math.random()
    .toString(36)
    .slice(2, 10)}`;
}

/** Keeps an operation key stable across transport failures. Payload changes
 * select a new fingerprint; confirmed success explicitly retires the key. */
export class RoomIdempotencyKeys {
  private readonly keys = new Map<string, string>();

  constructor(
    private readonly createKey: (action: string) => string =
      createRoomIdempotencyKey,
  ) {}

  keyFor(action: string, fingerprint: string): string {
    const operation = `${action}\u0000${fingerprint}`;
    const existing = this.keys.get(operation);
    if (existing) return existing;
    const created = this.createKey(action);
    this.keys.set(operation, created);
    return created;
  }

  complete(action: string, fingerprint: string): void {
    this.keys.delete(`${action}\u0000${fingerprint}`);
  }
}

export function roomReviewCorrection(
  action: "accept" | "reject" | "correct",
  synthesis: RoomSynthesis,
  correctedSummary: string,
): RoomSynthesis | undefined {
  return action === "correct"
    ? { ...synthesis, summary: correctedSummary.trim() }
    : undefined;
}

export function canPromoteRoomRevision(status: RoomReviewStatus): boolean {
  return status === "accepted";
}

export function roomConflictCode(error: unknown): RoomConflictCode | undefined {
  const structured = apiError(error);
  if (!structured?.body || typeof structured.body !== "object") {
    return undefined;
  }
  const candidate = (structured.body as { code?: unknown }).code;
  return typeof candidate === "string"
    ? (candidate as RoomConflictCode)
    : undefined;
}

export function roomMessageWasSaved(error: unknown): boolean {
  if (apiError(error)?.status !== 409) return false;
  const code = roomConflictCode(error);
  return code === "room_paused" || code === "room_archived" ||
    code === "budget_exhausted" || code === "active_cycle" ||
    code === "agent_unavailable";
}

export function roomErrorMessage(error: unknown): string {
  switch (roomConflictCode(error)) {
    case "room_paused":
      return "Message saved. Resume the Room when you want Agents to respond.";
    case "budget_exhausted":
      return "Message saved. The Room has reached its current budget.";
    case "room_archived":
      return "Message saved. Archived Rooms do not run Agents.";
    case "active_cycle":
      return "Message saved. A cycle is already running.";
    case "agent_unavailable":
      return "Message saved. One or more target Agents are unavailable.";
    case "invocation_not_allowed":
      return "You do not have permission to invoke one or more target Agents.";
    case "stale_review":
      return "This synthesis changed on another client. Review the latest version.";
    case "idempotency_conflict":
      return "This action conflicts with an earlier request. Refresh and try again.";
    case "synthesis_not_retryable":
      return "This synthesis cannot be retried in its current state.";
    case "promotion_source_mismatch":
      return "This recommendation no longer matches the reviewed synthesis.";
    case "recommendation_already_reviewed":
      return "This recommendation was already reviewed on another client.";
    default:
      return error instanceof Error ? error.message : "The Room action failed.";
  }
}

export function canRetrySynthesis(cycle: RoomCycle | undefined): boolean {
  return cycle?.synthesis_error?.retryable === true;
}

export function recommendationStatus(
  reviews: readonly RoomRecommendationReview[],
  memoryRevisionId: string,
  recommendationKey: string,
): RoomRecommendationReview["status"] | null {
  return (
    reviews.find(
      (review) =>
        review.memory_revision_id === memoryRevisionId &&
        review.recommendation_key === recommendationKey,
    )?.status ?? null
  );
}
