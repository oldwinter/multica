import type {
  RoomCyclePhase,
  RoomReviewStatus,
  RoomStatus,
} from "@/data/rooms-types";

export function roomStatusLabel(status: RoomStatus): string {
  switch (status) {
    case "active":
      return "Active";
    case "paused":
      return "Paused";
    case "archived":
      return "Archived";
    default:
      return "Unknown status";
  }
}

export function roomPhaseLabel(phase: RoomCyclePhase): string {
  switch (phase) {
    case "gathering":
      return "Gathering";
    case "synthesizing":
      return "Synthesizing";
    case "awaiting_review":
      return "Awaiting review";
    case "completed":
      return "Completed";
    case "failed":
      return "Failed";
    case "cancelled":
      return "Cancelled";
    case "refused":
      return "Refused";
    default:
      return "Unknown phase";
  }
}

export function roomReviewLabel(status: RoomReviewStatus): string {
  switch (status) {
    case "pending":
      return "Pending review";
    case "accepted":
      return "Accepted";
    case "rejected":
      return "Changes requested";
    case "corrected":
      return "Corrected";
    default:
      return "Unknown review";
  }
}

export function confidenceLabel(confidence: number | null): string | null {
  if (confidence === null) return null;
  const normalized = confidence <= 1 ? confidence * 100 : confidence;
  return `${Math.round(normalized)}% confidence`;
}
