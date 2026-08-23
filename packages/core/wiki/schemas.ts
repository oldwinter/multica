import { z } from "zod";
import type {
  LMWikiSourcePolicy,
  WikiActorType,
  WikiPage,
  WikiPageSummary,
  WikiProposal,
  WikiProposalStatus,
  WikiRevision,
  WikiRevisionConflict,
  WikiSourceKind,
} from "./types";

const WIKI_ACTOR_TYPES = new Set(["member", "agent", "system"]);
const WIKI_SOURCE_KINDS = new Set(["human", "room_promotion", "agent_proposal", "restore", "system"]);
const WIKI_PROPOSAL_STATUSES = new Set(["pending", "accepted", "rejected"]);

export const WikiActorTypeSchema = z.string().transform((value): WikiActorType =>
  WIKI_ACTOR_TYPES.has(value) ? value as WikiActorType : "unknown");
export const WikiSourceKindSchema = z.string().transform((value): WikiSourceKind =>
  WIKI_SOURCE_KINDS.has(value) ? value as WikiSourceKind : "unknown");
export const WikiProposalStatusSchema = z.string().transform((value): WikiProposalStatus =>
  WIKI_PROPOSAL_STATUSES.has(value) ? value as WikiProposalStatus : "unknown");
export const LMWikiSourceClassSchema = z.enum([
  "issue", "project", "project_resource", "autopilot_run", "wiki_page",
]);

const WikiPageSummaryWireSchema = z.object({
  id: z.string().min(1),
  workspace_id: z.string().nullable().default(null),
  scope: z.enum(["workspace", "project", "user"]),
  project_id: z.string().nullable().default(null),
  owner_user_id: z.string().nullable().default(null),
  path: z.string(),
  title: z.string().default(""),
  created_by: z.string().nullable().default(null),
  current_revision_number: z.number().int().positive(),
  current_revision_id: z.string().min(1),
  content_digest: z.string().min(1),
  last_source_kind: WikiSourceKindSchema,
  last_actor_type: WikiActorTypeSchema,
  last_actor_id: z.string().nullable().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose();

function wikiPageSummaryFromWire(
  wire: z.infer<typeof WikiPageSummaryWireSchema>,
): WikiPageSummary {
  return {
    id: wire.id,
    workspaceId: wire.workspace_id,
    scope: wire.scope,
    projectId: wire.project_id,
    ownerUserId: wire.owner_user_id,
    path: wire.path,
    title: wire.title,
    createdBy: wire.created_by,
    currentRevisionNumber: wire.current_revision_number,
    currentRevisionId: wire.current_revision_id,
    contentDigest: wire.content_digest,
    lastSourceKind: wire.last_source_kind,
    lastActorType: wire.last_actor_type,
    lastActorId: wire.last_actor_id,
    createdAt: wire.created_at,
    updatedAt: wire.updated_at,
  };
}

export const WikiPageSummarySchema = WikiPageSummaryWireSchema.transform(wikiPageSummaryFromWire);

const WikiPageWireSchema = WikiPageSummaryWireSchema.extend({
  content: z.string().default(""),
}).loose();

export const WikiPageSchema = WikiPageWireSchema.transform((wire): WikiPage => ({
  ...wikiPageSummaryFromWire(wire),
  content: wire.content,
}));

export const WikiPageListSchema = z.array(WikiPageSummarySchema);

const WikiRevisionWireSchema = z.object({
  id: z.string().min(1),
  page_id: z.string().min(1),
  revision_number: z.number().int().positive(),
  path: z.string(),
  title: z.string().default(""),
  content: z.string().default(""),
  content_digest: z.string().min(1),
  actor_type: WikiActorTypeSchema,
  actor_id: z.string().nullable().default(null),
  source_kind: WikiSourceKindSchema,
  source_ref_id: z.string().nullable().default(null),
  created_at: z.string().default(""),
}).loose();

export const WikiRevisionSchema = WikiRevisionWireSchema.transform((wire): WikiRevision => ({
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

const WikiProposalWireSchema = z.object({
  id: z.string().min(1),
  page_id: z.string().min(1),
  base_revision_number: z.number().int().positive(),
  proposed_path: z.string(),
  proposed_title: z.string().default(""),
  proposed_content: z.string().default(""),
  content_digest: z.string().min(1),
  rationale: z.string().default(""),
  evidence_refs: z.array(z.string()).default([]),
  agent_id: z.string().min(1),
  idempotency_key: z.string().min(1),
  status: WikiProposalStatusSchema,
  reviewed_by_id: z.string().nullable().default(null),
  review_reason: z.string().nullable().default(null),
  reviewed_at: z.string().nullable().default(null),
  accepted_revision_id: z.string().nullable().default(null),
  created_at: z.string().default(""),
}).loose();

export const WikiProposalSchema = WikiProposalWireSchema.transform((wire): WikiProposal => ({
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

const LMWikiSourcePolicyWireSchema = z.object({
  source_classes: z.array(LMWikiSourceClassSchema),
  wiki_pages: z.array(z.object({
    page_id: z.string().min(1),
    revision_number: z.number().int().positive(),
  }).loose()),
  remote_generation_enabled: z.boolean().optional().default(false),
  policy_version: z.number().int().nonnegative().optional().default(0),
  policy_digest: z.string().optional().default(""),
  exclusions: z.array(z.object({
    source_class: z.string(),
    state: z.string(),
    reason: z.string(),
  }).loose()).optional().default([]),
}).loose();

export const LMWikiSourcePolicySchema = LMWikiSourcePolicyWireSchema.transform(
  (wire): LMWikiSourcePolicy => ({
    sourceClasses: wire.source_classes,
    wikiPages: wire.wiki_pages.map((page) => ({
      pageId: page.page_id,
      revisionNumber: page.revision_number,
    })),
    remoteGenerationEnabled: wire.remote_generation_enabled,
    policyVersion: wire.policy_version,
    policyDigest: wire.policy_digest,
    exclusions: wire.exclusions.map((exclusion) => ({
      sourceClass: exclusion.source_class,
      state: exclusion.state,
      reason: exclusion.reason,
    })),
  }),
);

const WikiRevisionConflictWireSchema = z.object({
  code: z.literal("wiki_revision_conflict"),
  current_revision_number: z.number().int().positive(),
}).loose();

export function parseWikiRevisionConflict(value: unknown): WikiRevisionConflict | null {
  const result = WikiRevisionConflictWireSchema.safeParse(value);
  return result.success
    ? { currentRevisionNumber: result.data.current_revision_number }
    : null;
}

export const EMPTY_WIKI_PAGE_LIST: WikiPageSummary[] = [];
export const EMPTY_WIKI_REVISION_LIST: WikiRevision[] = [];
export const EMPTY_WIKI_PROPOSAL_LIST: WikiProposal[] = [];
export const EMPTY_WIKI_REVISION: WikiRevision = {
  id: "",
  pageId: "",
  revisionNumber: 0,
  path: "",
  title: "",
  content: "",
  contentDigest: "",
  actorType: "unknown",
  actorId: null,
  sourceKind: "unknown",
  sourceRefId: null,
  createdAt: "",
};
export const EMPTY_LM_WIKI_SOURCE_POLICY: LMWikiSourcePolicy = {
  sourceClasses: [],
  wikiPages: [],
  remoteGenerationEnabled: false,
  policyVersion: 0,
  policyDigest: "",
  exclusions: [],
};

export const EMPTY_WIKI_PAGE: WikiPage = {
  id: "",
  workspaceId: null,
  scope: "workspace",
  projectId: null,
  ownerUserId: null,
  path: "",
  title: "",
  content: "",
  createdBy: null,
  currentRevisionNumber: 0,
  currentRevisionId: "",
  contentDigest: "",
  lastSourceKind: "unknown",
  lastActorType: "unknown",
  lastActorId: null,
  createdAt: "",
  updatedAt: "",
};
