import type {
  LMWikiDetail,
  LMWikiOverview,
  TwinOverview,
  TwinProposalDetail,
  TwinReviewStep,
  TwinVersionDetail,
} from "@multica/core/twins";

export type TwinViewState = "ready" | "loading" | "error";

export interface TwinWorkspaceProps {
  state: TwinViewState;
  wiki: LMWikiOverview;
  wikiDetail: LMWikiDetail | null;
  twin: TwinOverview;
  proposalDetail: TwinProposalDetail | null;
  versionDetail: TwinVersionDetail | null;
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
  onSelectRevision: (id: string) => void;
  onSelectProposal: (id: string) => void;
  onSelectVersion: (id: string) => void;
  onRefreshWiki: () => void;
  onAcceptWiki: (id: string) => Promise<void>;
  onRejectWiki: (id: string, reason: string) => Promise<void>;
  onEnsureTwin: (wikiRevisionId: string) => void;
  onAcceptTwin: (id: string) => Promise<void>;
  onRejectTwin: (id: string, reason: string) => Promise<void>;
  onRetry: () => void;
}

export interface ProjectedItem {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  readonly status: string;
  readonly citationKeys: readonly string[];
  readonly kind: string;
}

export interface ProjectedDiff {
  readonly added: readonly string[];
  readonly removed: readonly string[];
  readonly unchanged: readonly string[];
}

export interface ProjectedTopic {
  readonly id: string;
  readonly issueId: string;
  readonly issueNumber: number | null;
  readonly title: string;
  readonly status: string;
}
