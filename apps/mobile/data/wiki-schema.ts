import { z } from "zod";

const nullableString = z.string().nullable().optional().default(null);
const futureProofEnum = <const T extends readonly [string, ...string[]]>(
  values: T,
) =>
  z.string().transform((value): T[number] | "unknown" =>
    values.includes(value as T[number]) ? (value as T[number]) : "unknown",
  );

export const WikiScopeSchema = futureProofEnum(["workspace", "project", "user"]);

export const WikiSourceKindSchema = futureProofEnum([
  "human",
  "room_promotion",
  "agent_proposal",
  "restore",
  "system",
]);

export const WikiActorTypeSchema = futureProofEnum(["member", "agent", "system"]);

export const WikiProposalStatusSchema = futureProofEnum([
  "pending",
  "accepted",
  "rejected",
]);

const WikiPageSummaryWireSchema = z
  .object({
    id: z.string(),
    workspace_id: nullableString,
    scope: WikiScopeSchema,
    project_id: nullableString,
    owner_user_id: nullableString,
    path: z.string(),
    title: z.string(),
    created_by: nullableString,
    created_at: z.string(),
    updated_at: z.string(),
    current_revision_id: z.string(),
    current_revision_number: z.number().int().positive(),
    content_digest: z.string(),
    last_source_kind: WikiSourceKindSchema,
    last_actor_type: WikiActorTypeSchema,
    last_actor_id: nullableString,
  })
  .loose();

function wikiPageSummaryFromWire(
  wire: z.infer<typeof WikiPageSummaryWireSchema>,
) {
  return {
    id: wire.id,
    workspaceId: wire.workspace_id,
    scope: wire.scope,
    projectId: wire.project_id,
    ownerUserId: wire.owner_user_id,
    path: wire.path,
    title: wire.title,
    createdBy: wire.created_by,
    createdAt: wire.created_at,
    updatedAt: wire.updated_at,
    currentRevisionId: wire.current_revision_id,
    currentRevisionNumber: wire.current_revision_number,
    contentDigest: wire.content_digest,
    lastSourceKind: wire.last_source_kind,
    lastActorType: wire.last_actor_type,
    lastActorId: wire.last_actor_id,
  };
}

export const WikiPageSummarySchema = WikiPageSummaryWireSchema.transform(
  wikiPageSummaryFromWire,
);

const WikiPageWireSchema = WikiPageSummaryWireSchema.extend({
  content: z.string(),
}).loose();

export const WikiPageSchema = WikiPageWireSchema.transform((wire) => ({
  ...wikiPageSummaryFromWire(wire),
  content: wire.content,
}));

export const WikiPageSummaryListSchema = z.array(WikiPageSummarySchema);

const WikiRevisionWireSchema = z
  .object({
    id: z.string(),
    page_id: z.string(),
    revision_number: z.number().int().positive(),
    path: z.string(),
    title: z.string(),
    content: z.string(),
    content_digest: z.string(),
    actor_type: WikiActorTypeSchema,
    actor_id: nullableString,
    source_kind: WikiSourceKindSchema,
    source_ref_id: nullableString,
    created_at: z.string(),
  })
  .loose();

export const WikiRevisionSchema = WikiRevisionWireSchema.transform((wire) => ({
  id: wire.id,
  pageId: wire.page_id,
  revisionNumber: wire.revision_number,
  path: wire.path,
  title: wire.title,
  content: wire.content,
  contentDigest: wire.content_digest,
  actorType: wire.actor_type,
  actorId: wire.actor_id,
  sourceKind: wire.source_kind,
  sourceRefId: wire.source_ref_id,
  createdAt: wire.created_at,
}));

export const WikiRevisionListSchema = z.array(WikiRevisionSchema);

const WikiProposalWireSchema = z
  .object({
    id: z.string(),
    page_id: z.string(),
    base_revision_number: z.number().int().positive(),
    proposed_path: z.string(),
    proposed_title: z.string(),
    proposed_content: z.string(),
    content_digest: z.string(),
    rationale: z.string(),
    evidence_refs: z.array(z.string()).optional().default([]),
    agent_id: z.string(),
    idempotency_key: z.string(),
    status: WikiProposalStatusSchema,
    reviewed_by_id: nullableString,
    review_reason: nullableString,
    reviewed_at: nullableString,
    accepted_revision_id: nullableString,
    created_at: z.string(),
  })
  .loose();

export const WikiProposalSchema = WikiProposalWireSchema.transform((wire) => ({
  id: wire.id,
  pageId: wire.page_id,
  baseRevisionNumber: wire.base_revision_number,
  proposedPath: wire.proposed_path,
  proposedTitle: wire.proposed_title,
  proposedContent: wire.proposed_content,
  contentDigest: wire.content_digest,
  rationale: wire.rationale,
  evidenceRefs: wire.evidence_refs,
  agentId: wire.agent_id,
  idempotencyKey: wire.idempotency_key,
  status: wire.status,
  reviewedById: wire.reviewed_by_id,
  reviewReason: wire.review_reason,
  reviewedAt: wire.reviewed_at,
  acceptedRevisionId: wire.accepted_revision_id,
  createdAt: wire.created_at,
}));

export const WikiProposalListSchema = z.array(WikiProposalSchema);

export type WikiScope = z.infer<typeof WikiScopeSchema>;
export type WikiSourceKind = z.infer<typeof WikiSourceKindSchema>;
export type WikiActorType = z.infer<typeof WikiActorTypeSchema>;
export type WikiProposalStatus = z.infer<typeof WikiProposalStatusSchema>;
export type WikiPageSummary = z.infer<typeof WikiPageSummarySchema>;
export type WikiPage = z.infer<typeof WikiPageSchema>;
export type WikiRevision = z.infer<typeof WikiRevisionSchema>;
export type WikiProposal = z.infer<typeof WikiProposalSchema>;

export interface ListWikiPagesParams {
  scope: Exclude<WikiScope, "unknown">;
  projectId?: string;
}

export interface CreateWikiPageInput extends ListWikiPagesParams {
  path: string;
  title?: string;
  content?: string;
}

export interface UpdateWikiPageInput {
  expectedRevisionNumber: number;
  path?: string;
  title?: string;
  content?: string;
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
  expectedRevisionNumber: number;
  path?: string;
  title?: string;
  content?: string;
}

export interface RejectWikiProposalInput {
  reason?: string;
}

export function buildCreateWikiPageBody(input: CreateWikiPageInput) {
  return {
    scope: input.scope,
    ...(input.projectId === undefined ? {} : { project_id: input.projectId }),
    path: input.path,
    ...(input.title === undefined ? {} : { title: input.title }),
    ...(input.content === undefined ? {} : { content: input.content }),
  };
}

export function buildUpdateWikiPageBody(input: UpdateWikiPageInput) {
  return {
    expected_revision_number: input.expectedRevisionNumber,
    ...(input.path === undefined ? {} : { path: input.path }),
    ...(input.title === undefined ? {} : { title: input.title }),
    ...(input.content === undefined ? {} : { content: input.content }),
  };
}

export function buildCreateWikiProposalBody(input: CreateWikiProposalInput) {
  return {
    base_revision_number: input.baseRevisionNumber,
    proposed_path: input.proposedPath,
    proposed_title: input.proposedTitle,
    proposed_content: input.proposedContent,
    ...(input.rationale === undefined ? {} : { rationale: input.rationale }),
    ...(input.evidenceRefs === undefined
      ? {}
      : { evidence_refs: input.evidenceRefs }),
    agent_id: input.agentId,
    idempotency_key: input.idempotencyKey,
  };
}

export function buildAcceptWikiProposalBody(input: AcceptWikiProposalInput) {
  return {
    expected_revision_number: input.expectedRevisionNumber,
    ...(input.path === undefined ? {} : { path: input.path }),
    ...(input.title === undefined ? {} : { title: input.title }),
    ...(input.content === undefined ? {} : { content: input.content }),
  };
}

export function buildRejectWikiProposalBody(input: RejectWikiProposalInput) {
  return input.reason === undefined ? {} : { reason: input.reason };
}

export const EMPTY_WIKI_PAGE_SUMMARIES: WikiPageSummary[] = [];
export const EMPTY_WIKI_REVISIONS: WikiRevision[] = [];
export const EMPTY_WIKI_PROPOSALS: WikiProposal[] = [];
export const EMPTY_WIKI_PROPOSAL: WikiProposal = {
  id: "",
  pageId: "",
  baseRevisionNumber: 1,
  proposedPath: "",
  proposedTitle: "",
  proposedContent: "",
  contentDigest: "",
  rationale: "",
  evidenceRefs: [],
  agentId: "",
  idempotencyKey: "",
  status: "unknown",
  reviewedById: null,
  reviewReason: null,
  reviewedAt: null,
  acceptedRevisionId: null,
  createdAt: "",
};
export const EMPTY_WIKI_PAGE: WikiPage = {
  id: "",
  workspaceId: null,
  scope: "unknown",
  projectId: null,
  ownerUserId: null,
  path: "",
  title: "",
  content: "",
  createdBy: null,
  createdAt: "",
  updatedAt: "",
  currentRevisionId: "",
  currentRevisionNumber: 0,
  contentDigest: "",
  lastSourceKind: "unknown",
  lastActorType: "unknown",
  lastActorId: null,
};

export function getWikiRevisionConflict(
  body: unknown,
): { currentRevisionNumber: number } | null {
  if (!body || typeof body !== "object") return null;
  const record = body as Record<string, unknown>;
  if (
    record.code !== "wiki_revision_conflict" ||
    typeof record.current_revision_number !== "number" ||
    !Number.isInteger(record.current_revision_number) ||
    record.current_revision_number < 1
  ) {
    return null;
  }
  return { currentRevisionNumber: record.current_revision_number };
}
