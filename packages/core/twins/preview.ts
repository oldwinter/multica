/**
 * UI contract for the first Twin vertical slice.
 *
 * This is intentionally a preview source, not a server cache. The backend
 * contract is not present in Multica yet, so the view can be reviewed without
 * inventing a second persistence topology. The shape mirrors the adopted
 * product semantics and can be replaced by a React Query adapter later.
 */

export type TwinState = "invalid" | "pending-signoff" | "signed-off";

export type TwinTopicState = "active" | "waiting" | "accepted";

export type TwinReviewStepState = "complete" | "current" | "upcoming";

export type TwinReviewStepId = "import" | "generate" | "topic" | "coordinate" | "accept" | "deposition";

export interface TwinAssertion {
  id: string;
  text: string;
  sourceCount: number;
  sourceRefs: readonly string[];
  reviewed: boolean;
}

export interface TwinTopic {
  id: string;
  issueIdentifier: string;
  title: string;
  state: TwinTopicState;
  owner: string;
  updatedAt: string;
}

export interface TwinReviewStep {
  id: TwinReviewStepId;
  state: TwinReviewStepState;
}

export interface TwinOverview {
  id: string;
  name: string;
  state: TwinState;
  reviewDigest: string;
  updatedAt: string;
  sourceCount: number;
  assertionCount: number;
  skillCount: number;
  ruleCount: number;
  assertions: readonly TwinAssertion[];
  topics: readonly TwinTopic[];
  reviewSteps: readonly TwinReviewStep[];
}

/**
 * Stable, clearly labelled data for the inspectable first UI slice.
 * Replace this source with a parsed API response once the Twin endpoint lands.
 */
export const previewTwinOverview: TwinOverview = {
  id: "twin-preview",
  name: "Product partner",
  state: "pending-signoff",
  reviewDigest: "sha256:preview-twin-20260802",
  updatedAt: "2026-08-02T14:30:00.000Z",
  sourceCount: 6,
  assertionCount: 3,
  skillCount: 4,
  ruleCount: 6,
  assertions: [
    {
      id: "assertion-1",
      text: "Prefer small, reviewable changes that keep existing workspace boundaries intact.",
      sourceCount: 3,
      sourceRefs: ["workspace-boundaries", "reviewable-change", "scope-discipline"],
      reviewed: true,
    },
    {
      id: "assertion-2",
      text: "Treat human acceptance as a separate decision from a completed runtime.",
      sourceCount: 2,
      sourceRefs: ["human-acceptance", "runtime-completion"],
      reviewed: false,
    },
    {
      id: "assertion-3",
      text: "Keep evidence attached when a bounded topic moves into execution.",
      sourceCount: 1,
      sourceRefs: ["execution-attachment"],
      reviewed: false,
    },
  ],
  topics: [
    {
      id: "topic-1",
      issueIdentifier: "MUL-42",
      title: "Define the Twin review boundary",
      state: "active",
      owner: "You",
      updatedAt: "Today",
    },
    {
      id: "topic-2",
      issueIdentifier: "MUL-37",
      title: "Attach deposition notes to completed runs",
      state: "waiting",
      owner: "Aster",
      updatedAt: "Yesterday",
    },
    {
      id: "topic-3",
      issueIdentifier: "MUL-31",
      title: "Review the first evidence import",
      state: "accepted",
      owner: "You",
      updatedAt: "2 days ago",
    },
  ],
  reviewSteps: [
    { id: "import", state: "complete" },
    { id: "generate", state: "current" },
    { id: "topic", state: "upcoming" },
    { id: "coordinate", state: "upcoming" },
    { id: "accept", state: "upcoming" },
    { id: "deposition", state: "upcoming" },
  ],
};
