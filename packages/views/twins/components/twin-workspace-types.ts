import type { TwinReviewStepId, TwinTopicState } from "@multica/core/twins";

export type TwinViewState = "ready" | "loading" | "empty" | "error";

export interface TwinCopy {
  eyebrow: string;
  title: string;
  description: string;
  previewBadge: string;
  actions: {
    openIssues: string;
    openAgents: string;
    reviewProfile: string;
    tryAgain: string;
    connectEvidence: string;
  };
  status: Record<
    "pending" | "signedOff" | "invalid",
    { label: string; title: string; description: string }
  >;
  summary: {
    sources: string;
    assertions: string;
    skills: string;
    rules: string;
    lastReviewed: string;
  };
  review: {
    title: string;
    description: string;
    steps: Record<TwinReviewStepId, { label: string; description: string }>;
  };
  tabs: {
    overview: string;
    evidence: string;
    topics: string;
  };
  evidence: {
    title: string;
    description: string;
    sourceCount: string;
    viewDetail: string;
  };
  topics: {
    title: string;
    description: string;
    openIssue: string;
    empty: string;
  };
  states: {
    loading: string;
    emptyTitle: string;
    emptyDescription: string;
    errorTitle: string;
    errorDescription: string;
  };
  stateLabels: Record<"complete" | "current" | "upcoming", string>;
  topicStates: Record<TwinTopicState, string>;
}
