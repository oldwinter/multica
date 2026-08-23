export type WikiScope = "workspace" | "project" | "user";
export type WikiActorType = "member" | "agent" | "system" | "unknown";
export type WikiSourceKind = "human" | "room_promotion" | "agent_proposal" | "restore" | "system" | "unknown";
export type WikiProposalStatus = "pending" | "accepted" | "rejected" | "unknown";

export interface WikiPageSummary {
  id: string;
  /** Null for cross-workspace personal pages (scope=user). */
  workspaceId: string | null;
  scope: WikiScope;
  projectId: string | null;
  ownerUserId: string | null;
  path: string;
  title: string;
  createdBy: string | null;
  currentRevisionNumber: number;
  currentRevisionId: string;
  contentDigest: string;
  lastSourceKind: WikiSourceKind;
  lastActorType: WikiActorType;
  lastActorId: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface WikiPage extends WikiPageSummary {
  content: string;
}

export interface ListWikiPagesParams {
  scope?: WikiScope;
  projectId?: string;
}

export interface SearchWikiPagesParams extends ListWikiPagesParams {
  q: string;
}

export interface CreateWikiPageInput {
  scope: WikiScope;
  projectId?: string;
  path: string;
  title?: string;
  content?: string;
}

export interface CreatePersonalWikiPageInput {
  path: string;
  title?: string;
  content?: string;
}

export interface SearchPersonalWikiPagesParams {
  q: string;
}

export interface UpdateWikiPageInput {
  expectedRevisionNumber: number;
  path?: string;
  title?: string;
  content?: string;
}

export interface WikiRevision {
  id: string;
  pageId: string;
  revisionNumber: number;
  path: string;
  title: string;
  content: string;
  contentDigest: string;
  actorType: WikiActorType;
  actorId: string | null;
  sourceKind: WikiSourceKind;
  sourceRefId: string | null;
  createdAt: string;
}

export interface RestoreWikiRevisionInput {
  revisionId: string;
  expectedRevisionNumber: number;
}

export interface WikiProposal {
  id: string;
  pageId: string;
  baseRevisionNumber: number;
  proposedPath: string;
  proposedTitle: string;
  proposedContent: string;
  contentDigest: string;
  rationale: string;
  evidenceRefs: readonly string[];
  agentId: string;
  idempotencyKey: string;
  status: WikiProposalStatus;
  reviewedById: string | null;
  reviewReason: string | null;
  reviewedAt: string | null;
  acceptedRevisionId: string | null;
  createdAt: string;
}

export interface CreateWikiProposalInput {
  baseRevisionNumber: number;
  proposedPath: string;
  proposedTitle: string;
  proposedContent: string;
  rationale?: string;
  evidenceRefs?: readonly string[];
  agentId: string;
  idempotencyKey: string;
}

export interface AcceptWikiProposalInput {
  proposalId: string;
  expectedRevisionNumber: number;
  path?: string;
  title?: string;
  content?: string;
}

export interface RejectWikiProposalInput {
  proposalId: string;
  reason?: string;
}

export type LMWikiSourceClass = "issue" | "project" | "project_resource" | "autopilot_run" | "wiki_page";

export interface LMWikiSourceWikiPage {
  pageId: string;
  revisionNumber: number;
}

export interface LMWikiSourceExclusion {
  sourceClass: string;
  state: string;
  reason: string;
}

export interface LMWikiSourcePolicy {
  sourceClasses: readonly LMWikiSourceClass[];
  wikiPages: readonly LMWikiSourceWikiPage[];
  remoteGenerationEnabled: boolean;
  policyVersion: number;
  policyDigest: string;
  exclusions: readonly LMWikiSourceExclusion[];
}

export interface UpdateLMWikiSourcePolicyInput {
  sourceClasses: readonly LMWikiSourceClass[];
  wikiPages: readonly LMWikiSourceWikiPage[];
  remoteGenerationEnabled: boolean;
}

export interface WikiRevisionConflict {
  currentRevisionNumber: number;
}
