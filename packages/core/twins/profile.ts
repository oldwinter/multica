export type TwinProfileState = "invalid" | "pending-signoff" | "signed-off";

export type TwinProfileTopicState = "active" | "waiting" | "accepted";

export type TwinReviewStepState = "complete" | "current" | "upcoming";

export type TwinReviewStepId = "import" | "generate" | "topic" | "coordinate" | "accept" | "deposition";

export interface TwinProfileAssertion {
  id: string;
  text: string;
  sourceCount: number;
  sourceRefs: readonly string[];
  reviewed: boolean;
}

export interface TwinProfileTopic {
  id: string;
  issueIdentifier: string;
  title: string;
  state: TwinProfileTopicState;
  owner: string;
  updatedAt: string;
}

export interface TwinReviewStep {
  id: TwinReviewStepId;
  state: TwinReviewStepState;
}

export interface TwinProfileOverview {
  id: string;
  name: string;
  state: TwinProfileState;
  reviewDigest: string;
  updatedAt: string;
  sourceCount: number;
  assertionCount: number;
  skillCount: number;
  ruleCount: number;
  assertions: readonly TwinProfileAssertion[];
  topics: readonly TwinProfileTopic[];
  reviewSteps: readonly TwinReviewStep[];
}

export interface TwinOverviewResponse {
  readonly twin: TwinProfileOverview | null;
}
