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

export interface TwinWorkspaceProps {
  wsId: string;
  state: TwinViewState;
  overviewStale: boolean;
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
  sourcePolicyPanel?: ReactNode;
  onSelectRevision: (id: string) => void;
  onSelectProposal: (id: string) => void;
  onSelectVersion: (id: string) => void;
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
