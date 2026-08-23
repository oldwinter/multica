export type LifecycleContent = Readonly<Record<string, unknown>>;

export interface LMWikiReview {
  readonly id: string;
  readonly decision: string;
  readonly reviewer_id: string;
  readonly reason: string | null;
  readonly created_at: string;
}

export interface LMWikiRevision {
  readonly id: string;
  readonly revision_number: number;
  readonly schema_version: number;
  readonly source_digest: string;
  readonly content: LifecycleContent;
  readonly trigger_kind: string;
  readonly requested_by_id: string | null;
  readonly created_at: string;
  readonly review: LMWikiReview | null;
}

export interface LMWikiCitation {
  readonly id: string;
  readonly ordinal: number;
  readonly citation_key: string;
  readonly source_type: string;
  readonly source_id: string;
  readonly source_updated_at: string | null;
  readonly locator: string;
  readonly label: string;
  readonly safe_metadata: LifecycleContent;
  readonly source_digest: string;
}

export interface LMWikiOverview {
  readonly latest_revision: LMWikiRevision | null;
  readonly accepted_revision: LMWikiRevision | null;
  readonly pending_revision: LMWikiRevision | null;
  readonly revisions: readonly LMWikiRevision[];
  readonly can_manage: boolean;
}

export interface LMWikiDetail {
  readonly revision: LMWikiRevision;
  readonly citations: readonly LMWikiCitation[];
}

export interface LMWikiRefreshResult {
  readonly created: boolean;
  readonly revision: LMWikiRevision;
}

export interface TwinProposalReview {
  readonly id: string;
  readonly decision: string;
  readonly reviewer_id: string;
  readonly reason: string | null;
  readonly created_at: string;
}

export interface TwinVersion {
  readonly id: string;
  readonly version_number: number;
  readonly proposal_id: string;
  readonly source_wiki_revision_id: string;
  readonly prior_version_id: string | null;
  readonly schema_version: number;
  readonly content: LifecycleContent;
  readonly content_digest: string;
  readonly signed_off_by_id: string;
  readonly signed_off_at: string;
  readonly created_at: string;
}

export interface TwinProposal {
  readonly id: string;
  readonly kind: string;
  readonly source_wiki_revision_id: string;
  readonly base_twin_version_id: string | null;
  readonly schema_version: number;
  readonly content: LifecycleContent;
  readonly content_digest: string;
  readonly requested_by_id: string | null;
  readonly replaces_proposal_id?: string | null;
  readonly created_at: string;
  readonly review: TwinProposalReview | null;
  readonly signed_version: TwinVersion | null;
}

export interface TwinOverview {
  readonly current_version: TwinVersion | null;
  readonly pending_proposal: TwinProposal | null;
  readonly proposals: readonly TwinProposal[];
  readonly versions: readonly TwinVersion[];
  readonly can_manage: boolean;
}

export interface TwinProposalDetail {
  readonly proposal: TwinProposal;
  readonly source_revision: LMWikiRevision;
  readonly citations: readonly LMWikiCitation[];
  readonly run_evidence?: import("./execution-types").TwinDepositionEvidence;
}

export interface TwinVersionDetail {
  readonly version: TwinVersion;
  readonly proposal: TwinProposal;
  readonly source_revision: LMWikiRevision;
  readonly citations: readonly LMWikiCitation[];
}

export interface TwinProposalResult {
  readonly created: boolean;
  readonly proposal: TwinProposal;
}

export interface TwinVersionResult {
  readonly created: boolean;
  readonly version: TwinVersion;
}
