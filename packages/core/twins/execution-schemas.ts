import { z } from "zod";
import type {
  TwinBinding,
  TwinActivationReadiness,
  TwinBindingsResponse,
  TwinBriefingPreview,
  TwinDeposition,
  TwinDepositionResponse,
  TwinExecutionMetrics,
  TwinTaskContext,
  TwinTaskFeedback,
} from "./execution-types";

const UuidSchema = z.string().uuid();
const DigestSchema = z.string().regex(/^sha256:[a-f0-9]{64}$/);
const NonEmptyStringSchema = z.string().min(1);
const BindingStateSchema = z.enum(["off", "preview", "enabled"]);
const BindingScopeSchema = z.enum(["workspace", "agent", "project", "issue", "one_off"]);
const FeedbackRatingSchema = z.enum(["helped", "irrelevant", "mismatch"]);
const ActivationActionKeySchema = z.enum([
  "inspect_disabled",
  "configure_source",
  "review_evidence",
  "refresh_evidence",
  "review_twin",
  "generate_twin",
  "compile_preview",
  "configure_binding",
  "run_with_twin",
  "review_run",
  "review_deposition",
  "monitor_effectiveness",
]);

function safeArray<T>(schema: z.ZodType<T>) {
  return z.array(z.unknown()).optional().default([]).transform((values) =>
    values.flatMap((value) => {
      const parsed = schema.safeParse(value);
      return parsed.success ? [parsed.data] : [];
    }),
  );
}

const KillSwitchSchema = z.object({
  enabled: z.boolean().default(false),
  reason: z.string().nullable().optional().default(null),
}).loose().transform((value) => ({
  enabled: value.enabled,
  reason: value.reason,
}));

export const TwinBindingWireSchema = z.object({
  id: UuidSchema,
  scope_type: BindingScopeSchema,
  scope_id: UuidSchema,
  state: BindingStateSchema,
  twin_version_id: UuidSchema,
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose().transform((value): TwinBinding => ({
  id: value.id,
  scopeType: value.scope_type,
  scopeId: value.scope_id,
  state: value.state,
  twinVersionId: value.twin_version_id,
  createdAt: value.created_at,
  updatedAt: value.updated_at,
}));

export const TwinBindingsResponseSchema = z.object({
  bindings: safeArray(TwinBindingWireSchema),
  can_manage: z.boolean().optional().default(false),
  kill_switch: KillSwitchSchema.optional().default({ enabled: false, reason: null }),
}).loose().transform((value): TwinBindingsResponse => ({
  bindings: value.bindings,
  canManage: value.can_manage,
  killSwitch: value.kill_switch,
}));

const TwinActivationActionWireSchema = z.object({
  key: ActivationActionKeySchema,
  reason: NonEmptyStringSchema,
  target: z.enum(["wiki", "twin", "use"]),
  responsible_role: z.enum(["member", "owner_admin"]),
  can_act: z.boolean().default(false),
}).loose().transform((value) => ({
  key: value.key,
  reason: value.reason,
  target: value.target,
  responsibleRole: value.responsible_role,
  canAct: value.can_act,
}));

const TwinActivationStageWireSchema = z.object({
  key: z.enum([
    "source_policy",
    "evidence",
    "signed_twin",
    "preview",
    "binding",
    "attributed_run",
    "feedback",
    "deposition",
  ]),
  complete: z.boolean().default(false),
  count: z.number().int().nonnegative().optional().default(0),
}).loose().transform((value) => ({ key: value.key, complete: value.complete, count: value.count }));

const TwinActivationBlockerWireSchema = z.object({
  kind: z.enum(["kill_switch", "missing_capability", "missing_state", "stale_version", "review_gate", "exclusion"]),
  reason: NonEmptyStringSchema,
  responsible_role: z.enum(["member", "owner_admin"]),
}).loose().transform((value) => ({
  kind: value.kind,
  reason: value.reason,
  responsibleRole: value.responsible_role,
}));

const TwinActivationInspectionLinkWireSchema = z.object({
  key: z.enum(["evidence_history", "twin_history", "execution_evidence"]),
  target: z.enum(["wiki", "twin", "use"]),
}).loose();

const TwinMaintenanceItemWireSchema = z.object({
  id: NonEmptyStringSchema,
  kind: z.enum([
    "pending_proposal",
    "stale_signed_version",
    "repeated_mismatch",
    "low_confidence",
    "pending_deposition",
  ]),
  reason: NonEmptyStringSchema,
  severity: z.enum(["high", "medium", "low"]),
  owner_role: z.literal("owner_admin"),
  subject_type: z.enum(["workspace", "twin_proposal", "twin_version", "twin_deposition"]),
  subject_id: UuidSchema.optional().catch(undefined),
  version_number: z.number().int().positive().optional().catch(undefined),
  count: z.number().int().nonnegative().optional().default(0),
  created_at: z.string().optional().catch(undefined),
  action: ActivationActionKeySchema,
}).loose().transform((value) => ({
  id: value.id,
  kind: value.kind,
  reason: value.reason,
  severity: value.severity,
  ownerRole: value.owner_role,
  subjectType: value.subject_type,
  subjectId: value.subject_id ?? null,
  versionNumber: value.version_number ?? null,
  count: value.count,
  createdAt: value.created_at ?? null,
  action: value.action,
}));

export const TwinActivationReadinessSchema = z.object({
  contract_version: z.number().int().positive(),
  ready: z.boolean().default(false),
  can_manage: z.boolean().default(false),
  stages: safeArray(TwinActivationStageWireSchema),
  next_action: TwinActivationActionWireSchema,
  blockers: safeArray(TwinActivationBlockerWireSchema),
  inspection_links: safeArray(TwinActivationInspectionLinkWireSchema),
  maintenance: safeArray(TwinMaintenanceItemWireSchema),
}).loose().transform((value): TwinActivationReadiness => ({
  contractVersion: value.contract_version,
  ready: value.ready,
  canManage: value.can_manage,
  stages: value.stages,
  nextAction: value.next_action,
  blockers: value.blockers,
  inspectionLinks: value.inspection_links,
  maintenance: value.maintenance,
}));

export const TwinPauseResponseSchema = z.object({
  binding: TwinBindingWireSchema,
}).loose().transform((value) => value.binding);

const PolicyExclusionWireSchema = z.object({
  binding_id: UuidSchema,
  scope_type: BindingScopeSchema,
  code: NonEmptyStringSchema,
}).loose().transform((value) => ({
  bindingId: value.binding_id,
  scopeType: value.scope_type,
  code: value.code,
}));

const EffectivePolicyWireSchema = z.object({
  state: BindingStateSchema.default("off"),
  scope_type: BindingScopeSchema.nullable().optional().default(null),
  scope_id: UuidSchema.nullable().optional().default(null),
  binding_id: UuidSchema.nullable().optional().default(null),
  explicit: z.boolean().optional().default(false),
  reason: z.string().optional().default("no_explicit_binding"),
  exclusions: safeArray(PolicyExclusionWireSchema),
}).loose().transform((value) => ({
  state: value.state,
  scopeType: value.scope_type,
  scopeId: value.scope_id,
  bindingId: value.binding_id,
  explicit: value.explicit,
  reason: value.reason,
  exclusions: value.exclusions,
}));

const TwinVersionReferenceWireSchema = z.object({
  id: UuidSchema,
  version_number: z.number().int().positive(),
  content_digest: DigestSchema,
}).loose().transform((value) => ({
  id: value.id,
  versionNumber: value.version_number,
  contentDigest: value.content_digest,
}));

export const TwinBriefingPreviewSchema = z.object({
  policy: EffectivePolicyWireSchema,
  twin_version: TwinVersionReferenceWireSchema.nullable().optional().default(null),
  briefing: z.string().optional().default(""),
  briefing_digest: z.union([DigestSchema, z.literal("")]).optional().default(""),
  assertion_ids: z.array(NonEmptyStringSchema).optional().default([]).catch([]),
  citation_keys: z.array(NonEmptyStringSchema).optional().default([]).catch([]),
  compiler_version: z.string().optional().default(""),
  byte_count: z.number().int().nonnegative().optional().default(0),
  token_count: z.number().int().nonnegative().optional().default(0),
  inject: z.boolean().optional().default(false),
  exclusion_reasons: z.array(z.string()).optional().default([]).catch([]),
}).loose().transform((value): TwinBriefingPreview => ({
  policy: value.policy,
  twinVersion: value.twin_version,
  briefing: value.briefing,
  briefingDigest: value.briefing_digest,
  assertionIds: value.assertion_ids,
  citationKeys: value.citation_keys,
  compilerVersion: value.compiler_version,
  byteCount: value.byte_count,
  tokenCount: value.token_count,
  inject: value.inject,
  exclusionReasons: value.exclusion_reasons,
}));

export const TwinTaskAttributionWireSchema = z.object({
  twin_version_id: UuidSchema,
  twin_version_number: z.number().int().positive(),
  twin_version_digest: DigestSchema,
  briefing: z.string(),
  briefing_digest: DigestSchema,
  assertion_ids: z.array(NonEmptyStringSchema),
  citation_keys: z.array(NonEmptyStringSchema),
  policy_scope_type: BindingScopeSchema,
  policy_scope_id: UuidSchema,
  policy_state: BindingStateSchema,
  compiler_version: NonEmptyStringSchema,
  byte_count: z.number().int().nonnegative(),
  token_count: z.number().int().nonnegative(),
}).loose().transform((value) => ({
  twinVersionId: value.twin_version_id,
  twinVersionNumber: value.twin_version_number,
  twinVersionDigest: value.twin_version_digest,
  briefing: value.briefing,
  briefingDigest: value.briefing_digest,
  assertionIds: value.assertion_ids,
  citationKeys: value.citation_keys,
  policyScopeType: value.policy_scope_type,
  policyScopeId: value.policy_scope_id,
  policyState: value.policy_state,
  compilerVersion: value.compiler_version,
  byteCount: value.byte_count,
  tokenCount: value.token_count,
}));

export const TwinTaskFeedbackWireSchema = z.object({
  id: UuidSchema,
  task_id: UuidSchema,
  rating: FeedbackRatingSchema,
  note: z.string().nullable().optional().default(null),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose().transform((value): TwinTaskFeedback => ({
  id: value.id,
  taskId: value.task_id,
  rating: value.rating,
  note: value.note,
  createdAt: value.created_at,
  updatedAt: value.updated_at,
}));

export const TwinDepositionWireSchema = z.object({
  id: UuidSchema,
  task_id: UuidSchema,
  base_twin_version_id: UuidSchema,
  proposal_id: UuidSchema,
  replaces_proposal_id: UuidSchema.optional().catch(undefined),
  evidence_digest: DigestSchema,
  state: z.string().default("pending"),
  created_at: z.string().default(""),
  updated_at: z.string().default(""),
}).loose().transform((value): TwinDeposition => ({
  id: value.id,
  taskId: value.task_id,
  baseTwinVersionId: value.base_twin_version_id,
  proposalId: value.proposal_id,
  replacesProposalId: value.replaces_proposal_id,
  evidenceDigest: value.evidence_digest,
  state: value.state,
  createdAt: value.created_at,
  updatedAt: value.updated_at,
}));

const TwinRunAssertionWireSchema = z.object({
  id: NonEmptyStringSchema,
  type: z.string().optional().default(""),
  text: z.string().optional().default(""),
  citation_keys: z.array(z.string()).optional().default([]).catch([]),
}).loose().transform((value) => ({
  id: value.id,
  type: value.type,
  text: value.text,
  citationKeys: value.citation_keys,
}));

const TwinRunCitationWireSchema = z.object({
  key: NonEmptyStringSchema,
  label: z.string().optional().default(""),
  source_type: z.string().optional().default(""),
  locator: z.string().optional().default(""),
}).loose().transform((value) => ({
  key: value.key,
  label: value.label,
  sourceType: value.source_type,
  locator: value.locator,
}));

export const TwinTaskContextWireSchema = z.object({
  task_id: UuidSchema,
  attribution: TwinTaskAttributionWireSchema.nullable().optional().catch(undefined),
  feedback: TwinTaskFeedbackWireSchema.nullable().optional().catch(undefined),
  depositions: safeArray(TwinDepositionWireSchema),
  assertions: safeArray(TwinRunAssertionWireSchema),
  citations: safeArray(TwinRunCitationWireSchema),
}).loose().transform((value): TwinTaskContext => ({
  taskId: value.task_id,
  attribution: value.attribution ?? undefined,
  feedback: value.feedback ?? undefined,
  depositions: value.depositions,
  assertions: value.assertions,
  citations: value.citations,
}));

export const TwinFeedbackResponseSchema = z.object({
  feedback: TwinTaskFeedbackWireSchema,
}).loose().transform((value) => ({ feedback: value.feedback }));

const TwinDepositionProposalWireSchema = z.object({
  id: UuidSchema,
  kind: z.string().default("deposition"),
  schema_version: z.number().int().positive().default(2),
  content_digest: DigestSchema,
  created_at: z.string().default(""),
}).loose().transform((value) => ({
  id: value.id,
  kind: value.kind,
  schemaVersion: value.schema_version,
  contentDigest: value.content_digest,
  createdAt: value.created_at,
}));

export const TwinDepositionResponseSchema = z.object({
  deposition: TwinDepositionWireSchema.nullable().optional().default(null),
  proposal: TwinDepositionProposalWireSchema.nullable().optional().default(null),
}).loose().transform((value): TwinDepositionResponse => ({
  deposition: value.deposition,
  proposal: value.proposal,
}));

const CounterSchema = z.number().int().nonnegative().catch(0).default(0);
const RatioSchema = z.number().min(0).max(1).catch(0).default(0);
const NullableCounterSchema = CounterSchema.nullable().catch(null);

const TwinEffectivenessCohortWireSchema = z.object({
  policy_state: BindingStateSchema,
  sample_size: CounterSchema,
  completed_runs: CounterSchema,
  attributed_runs: CounterSchema,
  feedback_total: CounterSchema,
  feedback_coverage: RatioSchema,
  detail_suppressed: z.boolean().default(true),
  feedback_helped: NullableCounterSchema,
  feedback_irrelevant: NullableCounterSchema,
  feedback_mismatch: NullableCounterSchema,
  helped_rate: RatioSchema.nullable().catch(null),
  revision_count: NullableCounterSchema,
  revision_rate: RatioSchema.nullable().catch(null),
  average_latency_ms: NullableCounterSchema,
  average_briefing_tokens: NullableCounterSchema,
  cost_usd_ticks: NullableCounterSchema,
  costed_runs: CounterSchema,
  uncosted_runs: CounterSchema,
  cost_coverage: RatioSchema,
  deposition_total: NullableCounterSchema,
  deposition_accepted: NullableCounterSchema,
  deposition_acceptance_rate: RatioSchema.nullable().catch(null),
}).loose().transform((value) => ({
  policyState: value.policy_state,
  sampleSize: value.sample_size,
  completedRuns: value.completed_runs,
  attributedRuns: value.attributed_runs,
  feedbackTotal: value.feedback_total,
  feedbackCoverage: value.feedback_coverage,
  detailSuppressed: value.detail_suppressed,
  feedbackHelped: value.feedback_helped,
  feedbackIrrelevant: value.feedback_irrelevant,
  feedbackMismatch: value.feedback_mismatch,
  helpedRate: value.helped_rate,
  revisionCount: value.revision_count,
  revisionRate: value.revision_rate,
  averageLatencyMs: value.average_latency_ms,
  averageBriefingTokens: value.average_briefing_tokens,
  costUsdTicks: value.cost_usd_ticks,
  costedRuns: value.costed_runs,
  uncostedRuns: value.uncosted_runs,
  costCoverage: value.cost_coverage,
  depositionTotal: value.deposition_total,
  depositionAccepted: value.deposition_accepted,
  depositionAcceptanceRate: value.deposition_acceptance_rate,
}));

const EmptyEffectiveness = {
  windowDays: 28,
  minimumSample: 5,
  cohortDefinition: "",
  revisionMeasure: "",
  costMeasure: "",
  cohorts: [],
  comparison: { eligible: false, enabledState: "enabled" as const, controlState: null, reason: "unavailable" },
};

const TwinEffectivenessWireSchema = z.object({
  window_days: z.number().int().positive().default(28),
  minimum_sample: z.number().int().positive().default(5),
  cohort_definition: z.string().default(""),
  revision_measure: z.string().default(""),
  cost_measure: z.string().default(""),
  cohorts: safeArray(TwinEffectivenessCohortWireSchema),
  comparison: z.object({
    eligible: z.boolean().default(false),
    enabled_state: z.literal("enabled").default("enabled"),
    control_state: z.enum(["off", "preview"]).nullable().optional().catch(null),
    reason: z.string().default("unavailable"),
  }).loose().optional().default({
    eligible: false,
    enabled_state: "enabled",
    control_state: null,
    reason: "unavailable",
  }),
}).loose().transform((value) => ({
  windowDays: value.window_days,
  minimumSample: value.minimum_sample,
  cohortDefinition: value.cohort_definition,
  revisionMeasure: value.revision_measure,
  costMeasure: value.cost_measure,
  cohorts: value.cohorts,
  comparison: {
    eligible: value.comparison.eligible,
    enabledState: value.comparison.enabled_state,
    controlState: value.comparison.control_state ?? null,
    reason: value.comparison.reason,
  },
}));

export const TwinExecutionMetricsSchema = z.object({
  attributed_runs: CounterSchema,
  feedback: z.object({
    total: CounterSchema,
    helped: CounterSchema,
    irrelevant: CounterSchema,
    mismatch: CounterSchema,
  }).loose().optional().default({ total: 0, helped: 0, irrelevant: 0, mismatch: 0 }),
  depositions: z.object({
    total: CounterSchema,
    pending: CounterSchema,
    accepted: CounterSchema,
    rejected: CounterSchema,
  }).loose().optional().default({ total: 0, pending: 0, accepted: 0, rejected: 0 }),
  bindings: z.object({
    off: CounterSchema,
    preview: CounterSchema,
    enabled: CounterSchema,
  }).loose().optional().default({ off: 0, preview: 0, enabled: 0 }),
  helpfulness_rate: z.number().min(0).max(1).nullable().optional().default(null).catch(null),
  kill_switch: KillSwitchSchema.optional().default({ enabled: false, reason: null }),
  effectiveness: TwinEffectivenessWireSchema.optional().default(EmptyEffectiveness),
}).loose().transform((value): TwinExecutionMetrics => ({
  attributedRuns: value.attributed_runs,
  feedback: value.feedback,
  depositions: value.depositions,
  bindings: value.bindings,
  helpfulnessRate: value.helpfulness_rate,
  killSwitch: value.kill_switch,
  effectiveness: value.effectiveness,
}));

export const EMPTY_TWIN_BINDINGS_RESPONSE: TwinBindingsResponse = {
  bindings: [],
  canManage: false,
  killSwitch: { enabled: false, reason: null },
};

export const EMPTY_TWIN_ACTIVATION_READINESS: TwinActivationReadiness = {
  contractVersion: 1,
  ready: false,
  canManage: false,
  stages: [],
  nextAction: {
    key: "inspect_disabled",
    reason: "unavailable",
    target: "use",
    responsibleRole: "owner_admin",
    canAct: false,
  },
  blockers: [],
  inspectionLinks: [],
  maintenance: [],
};

export const EMPTY_TWIN_BRIEFING_PREVIEW: TwinBriefingPreview = {
  policy: {
    state: "off",
    scopeType: null,
    scopeId: null,
    bindingId: null,
    explicit: false,
    reason: "no_explicit_binding",
    exclusions: [],
  },
  twinVersion: null,
  briefing: "",
  briefingDigest: "",
  assertionIds: [],
  citationKeys: [],
  compilerVersion: "",
  byteCount: 0,
  tokenCount: 0,
  inject: false,
  exclusionReasons: [],
};

export const EMPTY_TWIN_DEPOSITION_RESPONSE: TwinDepositionResponse = {
  deposition: null,
  proposal: null,
};

export const EMPTY_TWIN_EXECUTION_METRICS: TwinExecutionMetrics = {
  attributedRuns: 0,
  feedback: { total: 0, helped: 0, irrelevant: 0, mismatch: 0 },
  depositions: { total: 0, pending: 0, accepted: 0, rejected: 0 },
  bindings: { off: 0, preview: 0, enabled: 0 },
  helpfulnessRate: null,
  killSwitch: { enabled: false, reason: null },
  effectiveness: EmptyEffectiveness,
};
