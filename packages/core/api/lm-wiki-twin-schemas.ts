import { z } from "zod";
import type {
  LMWikiOverview,
  TwinOverview,
} from "../twins/types";
import type { TwinDepositionEvidence } from "../twins/execution-types";

const NonEmptyString = z.string().min(1);
const ContentSchema = z.object({}).loose();

const LMWikiReviewSchema = z.object({
  id: NonEmptyString,
  decision: z.string(),
  reviewer_id: NonEmptyString,
  reason: z.string().nullable().optional().default(null),
  created_at: z.string(),
}).loose();

export const LMWikiRevisionSchema = z.object({
  id: NonEmptyString,
  revision_number: z.number(),
  schema_version: z.number(),
  source_digest: z.string(),
  content: ContentSchema,
  trigger_kind: z.string(),
  requested_by_id: z.string().nullable().optional().default(null),
  created_at: z.string(),
  review: LMWikiReviewSchema.nullable().optional().default(null),
}).loose();

export const LMWikiCitationSchema = z.object({
  id: NonEmptyString,
  ordinal: z.number(),
  citation_key: NonEmptyString,
  source_type: z.string(),
  source_id: NonEmptyString,
  source_updated_at: z.string().nullable().optional().default(null),
  locator: z.string(),
  label: z.string(),
  safe_metadata: ContentSchema.optional().default({}),
  source_digest: z.string(),
}).loose();

export const LMWikiOverviewSchema = z.object({
  latest_revision: LMWikiRevisionSchema.nullable().optional().default(null),
  accepted_revision: LMWikiRevisionSchema.nullable().optional().default(null),
  pending_revision: LMWikiRevisionSchema.nullable().optional().default(null),
  revisions: z.array(LMWikiRevisionSchema).optional().default([]),
  can_manage: z.boolean().optional().default(false),
}).loose();

export const LMWikiDetailSchema = z.object({
  revision: LMWikiRevisionSchema,
  citations: z.array(LMWikiCitationSchema).optional().default([]),
}).loose();

export const LMWikiRefreshResultSchema = z.object({
  created: z.boolean(),
  revision: LMWikiRevisionSchema,
}).loose();

const TwinProposalReviewSchema = z.object({
  id: NonEmptyString,
  decision: z.string(),
  reviewer_id: NonEmptyString,
  reason: z.string().nullable().optional().default(null),
  created_at: z.string(),
}).loose();

export const TwinVersionSchema = z.object({
  id: NonEmptyString,
  version_number: z.number(),
  proposal_id: NonEmptyString,
  source_wiki_revision_id: NonEmptyString,
  prior_version_id: z.string().nullable().optional().default(null),
  schema_version: z.number(),
  content: ContentSchema,
  content_digest: z.string(),
  signed_off_by_id: NonEmptyString,
  signed_off_at: z.string(),
  created_at: z.string(),
}).loose();

export const TwinProposalSchema = z.object({
  id: NonEmptyString,
  kind: z.string(),
  source_wiki_revision_id: NonEmptyString,
  base_twin_version_id: z.string().nullable().optional().default(null),
  schema_version: z.number(),
  content: ContentSchema,
  content_digest: z.string(),
  requested_by_id: z.string().nullable().optional().default(null),
  replaces_proposal_id: z.string().nullable().optional().default(null),
  created_at: z.string(),
  review: TwinProposalReviewSchema.nullable().optional().default(null),
  signed_version: TwinVersionSchema.nullable().optional().default(null),
}).loose();

export const TwinOverviewSchema = z.object({
  current_version: TwinVersionSchema.nullable().optional().default(null),
  pending_proposal: TwinProposalSchema.nullable().optional().default(null),
  proposals: z.array(TwinProposalSchema).optional().default([]),
  versions: z.array(TwinVersionSchema).optional().default([]),
  can_manage: z.boolean().optional().default(false),
}).loose();

export const TwinProposalDetailSchema = z.object({
  proposal: TwinProposalSchema,
  source_revision: LMWikiRevisionSchema,
  citations: z.array(LMWikiCitationSchema).optional().default([]),
  run_evidence: z.object({
    task_id: z.string().uuid(),
    base_twin_version_id: z.string().uuid(),
    evidence_digest: z.string().regex(/^sha256:[a-f0-9]{64}$/),
    task_status: z.string().default(""),
    completed_at: z.string().nullable().optional().default(null),
    feedback_rating: z.enum(["helped", "irrelevant", "mismatch"]).nullable().optional().default(null),
    safe_metadata: ContentSchema.optional().default({}),
  }).loose().optional().catch(undefined).transform((value): TwinDepositionEvidence | undefined => value ? ({
    taskId: value.task_id,
    baseTwinVersionId: value.base_twin_version_id,
    evidenceDigest: value.evidence_digest,
    taskStatus: value.task_status,
    completedAt: value.completed_at,
    feedbackRating: value.feedback_rating,
    safeMetadata: value.safe_metadata,
  }) : undefined),
}).loose();

export const TwinVersionDetailSchema = z.object({
  version: TwinVersionSchema,
  proposal: TwinProposalSchema,
  source_revision: LMWikiRevisionSchema,
  citations: z.array(LMWikiCitationSchema).optional().default([]),
}).loose();

export const TwinProposalResultSchema = z.object({
  created: z.boolean(),
  proposal: TwinProposalSchema,
}).loose();

export const TwinVersionResultSchema = z.object({
  created: z.boolean(),
  version: TwinVersionSchema,
}).loose();

export const EMPTY_LM_WIKI_OVERVIEW: LMWikiOverview = {
  latest_revision: null, accepted_revision: null, pending_revision: null, revisions: [], can_manage: false,
};
export const EMPTY_TWIN_OVERVIEW: TwinOverview = { current_version: null, pending_proposal: null, proposals: [], versions: [], can_manage: false };
