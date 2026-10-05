import { z } from "zod";
import type {
  LMWikiSourcePolicy,
  LMWikiSourcePolicyStaleConflict,
  WikiActorType,
  WikiKnowledgeMaintenanceItem,
  WikiKnowledgeNextAction,
  WikiKnowledgeReadiness,
  WikiKnowledgeSourceReadiness,
  WikiPage,
  WikiPageSummary,
  WikiProposal,
  WikiProposalSourceKind,
  WikiProposalStatus,
  WikiRevision,
  WikiRevisionConflict,
  WikiSourceKind,
} from "./types";

const WIKI_ACTOR_TYPES = new Set(["member", "agent", "system"]);
const WIKI_SOURCE_KINDS = new Set(["human", "room_promotion", "agent_proposal", "restore", "system"]);
const WIKI_PROPOSAL_STATUSES = new Set(["pending", "accepted", "rejected"]);
const WIKI_PROPOSAL_SOURCE_KINDS = new Set(["agent", "room"]);

export const WikiActorTypeSchema = z.string().transform((value): WikiActorType =>
  WIKI_ACTOR_TYPES.has(value) ? value as WikiActorType : "unknown");
export const WikiSourceKindSchema = z.string().transform((value): WikiSourceKind =>
  WIKI_SOURCE_KINDS.has(value) ? value as WikiSourceKind : "unknown");
export const WikiProposalStatusSchema = z.string().transform((value): WikiProposalStatus =>
  WIKI_PROPOSAL_STATUSES.has(value) ? value as WikiProposalStatus : "unknown");
export const WikiProposalSourceKindSchema = z.string().transform((value): WikiProposalSourceKind =>
  WIKI_PROPOSAL_SOURCE_KINDS.has(value) ? value as WikiProposalSourceKind : "unknown");
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
  agent_id: z.string().min(1).nullable().default(null),
  source_kind: WikiProposalSourceKindSchema.optional().default("agent"),
  source_ref_id: z.string().nullable().default(null),
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
  sourceKind: wire.source_kind,
  sourceRefId: wire.source_ref_id,
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

export const WikiKnowledgeSourceStateSchema = z.enum([
  "eligible_unpinned",
  "pinned_current",
  "newer_revision_available",
  "source_deleted",
  "excluded",
  "policy_stale",
]);

const WikiKnowledgeNextActionWireSchema = z.object({
  kind: z.enum(["none", "pin_revision", "remove_source", "refresh_lm_wiki", "review_lm_wiki"]),
  page_id: z.string().min(1).optional(),
  revision_id: z.string().min(1).optional(),
  revision_number: z.number().int().positive().optional(),
  lm_wiki_revision_id: z.string().min(1).optional(),
}).loose();

const WikiKnowledgeNextActionSchema = WikiKnowledgeNextActionWireSchema.transform(
  (wire): WikiKnowledgeNextAction => ({
    kind: wire.kind,
    pageId: wire.page_id,
    revisionId: wire.revision_id,
    revisionNumber: wire.revision_number,
    lmWikiRevisionId: wire.lm_wiki_revision_id,
  }),
);

const WikiKnowledgeSourceReadinessWireSchema = z.object({
  page_id: z.string().min(1),
  scope: z.enum(["workspace", "project"]).optional(),
  project_id: z.string().min(1).optional(),
  state: WikiKnowledgeSourceStateSchema,
  reason_code: z.string().min(1),
  responsible_role: z.literal("owner_admin"),
  selected_revision_id: z.string().min(1).optional(),
  selected_revision_number: z.number().int().positive().optional(),
  current_revision_id: z.string().min(1).optional(),
  current_revision_number: z.number().int().positive().optional(),
  policy_version: z.number().int().nonnegative(),
  next_action: WikiKnowledgeNextActionSchema,
}).loose();

const WikiKnowledgeSourceReadinessSchema = WikiKnowledgeSourceReadinessWireSchema.transform(
  (wire): WikiKnowledgeSourceReadiness => ({
    pageId: wire.page_id,
    scope: wire.scope,
    projectId: wire.project_id,
    state: wire.state,
    reasonCode: wire.reason_code,
    responsibleRole: wire.responsible_role,
    selectedRevisionId: wire.selected_revision_id,
    selectedRevisionNumber: wire.selected_revision_number,
    currentRevisionId: wire.current_revision_id,
    currentRevisionNumber: wire.current_revision_number,
    policyVersion: wire.policy_version,
    nextAction: wire.next_action,
  }),
);

const WikiKnowledgeMaintenanceItemWireSchema = z.object({
  id: z.string().min(1),
  kind: z.enum([
    "source_newer_revision", "source_deleted", "source_excluded",
    "policy_stale", "lm_wiki_review_pending",
  ]),
  severity: z.enum(["warning", "high"]),
  reason_code: z.string().min(1),
  responsible_role: z.literal("owner_admin"),
  page_id: z.string().min(1).optional(),
  selected_revision_number: z.number().int().positive().optional(),
  policy_version: z.number().int().nonnegative(),
  next_action: WikiKnowledgeNextActionSchema,
}).loose();

const WikiKnowledgeMaintenanceItemSchema = WikiKnowledgeMaintenanceItemWireSchema.transform(
  (wire): WikiKnowledgeMaintenanceItem => ({
    id: wire.id,
    kind: wire.kind,
    severity: wire.severity,
    reasonCode: wire.reason_code,
    responsibleRole: wire.responsible_role,
    pageId: wire.page_id,
    selectedRevisionNumber: wire.selected_revision_number,
    policyVersion: wire.policy_version,
    nextAction: wire.next_action,
  }),
);

const WikiKnowledgeReadinessWireSchema = z.object({
  schema_version: z.literal(1),
  policy: LMWikiSourcePolicySchema,
  sources: z.array(WikiKnowledgeSourceReadinessSchema),
  maintenance_items: z.array(WikiKnowledgeMaintenanceItemSchema),
  truncated: z.boolean(),
  can_manage: z.boolean(),
}).loose();

export const WikiKnowledgeReadinessSchema = WikiKnowledgeReadinessWireSchema.transform(
  (wire): WikiKnowledgeReadiness => ({
    schemaVersion: wire.schema_version,
    policy: wire.policy,
    sources: wire.sources,
    maintenanceItems: wire.maintenance_items,
    truncated: wire.truncated,
    canManage: wire.can_manage,
  }),
);

const LMWikiSourcePolicyStaleWireSchema = z.object({
  code: z.literal("wiki_source_policy_stale"),
  current_policy: LMWikiSourcePolicySchema,
}).loose();

export function parseLMWikiSourcePolicyStale(
  value: unknown,
): LMWikiSourcePolicyStaleConflict | null {
  const result = LMWikiSourcePolicyStaleWireSchema.safeParse(value);
  return result.success
    ? { code: result.data.code, currentPolicy: result.data.current_policy }
    : null;
}

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
export const EMPTY_WIKI_KNOWLEDGE_READINESS: WikiKnowledgeReadiness = {
  schemaVersion: 1,
  policy: EMPTY_LM_WIKI_SOURCE_POLICY,
  sources: [],
  maintenanceItems: [],
  truncated: false,
  canManage: false,
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
