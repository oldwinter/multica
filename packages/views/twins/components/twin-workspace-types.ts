import type { ReactNode } from "react";
import type {
  LMWikiDetail,
  LMWikiOverview,
  TwinOverview,
  TwinProposalDetail,
  TwinReviewStep,
  TwinVersionDetail,
  LifecycleContent,
} from "@multica/core/twins";

export type TwinViewState = "ready" | "loading" | "error";

export type TwinDetailState =
  | { readonly kind: "none" }
  | { readonly kind: "loading" }
  | { readonly kind: "ready" }
  | { readonly kind: "stale" }
  | { readonly kind: "error" };

export interface TwinWorkspaceProps {
  wsId: string;
  state: TwinViewState;
  overviewStale: boolean;
  wiki: LMWikiOverview;
  wikiDetail: LMWikiDetail | null;
  wikiDetailState: TwinDetailState;
  twin: TwinOverview;
  proposalDetail: TwinProposalDetail | null;
  proposalDetailState: TwinDetailState;
  versionDetail: TwinVersionDetail | null;
  versionDetailState: TwinDetailState;
  reviewSteps: readonly TwinReviewStep[];
  selectedRevisionId: string;
  selectedProposalId: string;
  selectedVersionId: string;
  canManageWiki: boolean;
  canManageTwin: boolean;
  wikiMutationPending: boolean;
  twinMutationPending: boolean;
  detailLoading: boolean;
  actionError: string | null;
  sourcePolicyPanel?: ReactNode;
  onSelectRevision: (id: string) => void;
  onSelectProposal: (id: string) => void;
  onSelectVersion: (id: string) => void;
  onRetryWikiDetail: () => void;
  onRetryProposalDetail: () => void;
  onRetryVersionDetail: () => void;
  onRefreshWiki: () => void;
  onAcceptWiki: (id: string) => Promise<void>;
  onRejectWiki: (id: string, reason: string) => Promise<void>;
  onEnsureTwin: (wikiRevisionId: string) => void;
  onAcceptTwin: (id: string) => Promise<void>;
  onRejectTwin: (id: string, reason: string) => Promise<void>;
  onCorrectTwin: (proposalId: string, assertions: readonly LifecycleContent[]) => Promise<void>;
  onEditDeposition: (proposalId: string, assertions: readonly LifecycleContent[]) => Promise<void>;
  onRetry: () => void;
}

export interface ProjectedItem {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  readonly status: string;
  readonly citationKeys: readonly string[];
  readonly kind: string;
  readonly applicability: ProjectedApplicability | null;
  readonly confidence: number | null;
  readonly provenance: ProjectedProvenance | null;
}

export interface ProjectedApplicability {
  readonly taskId: string;
  readonly workspaceId: string;
  readonly agentId: string;
  readonly projectId: string;
  readonly issueId: string;
  readonly keywords: readonly string[];
  readonly legacyText: string;
}

export interface ProjectedProvenance {
  readonly kind: string;
  readonly generator: string;
}

export interface ProjectedDiff {
  readonly added: readonly string[];
  readonly removed: readonly string[];
  readonly unchanged: readonly string[];
  readonly changed?: readonly string[];
}

export interface ProjectedTopic {
  readonly id: string;
  readonly issueId: string;
  readonly issueNumber: number | null;
  readonly title: string;
  readonly status: string;
}
