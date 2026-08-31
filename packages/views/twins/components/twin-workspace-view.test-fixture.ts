import type {
  LMWikiDetail,
  LMWikiOverview,
  TwinOverview,
  TwinProposalDetail,
  TwinReviewStep,
  TwinVersionDetail,
} from "@multica/core/twins";

const revision = {
  id: "wiki-2",
  revision_number: 2,
  schema_version: 1,
  source_digest: "sha256:wiki-2",
  content: {
    schema_version: 1,
    issues: [{
      citation_key: "issue:42",
      id: "issue-42",
      number: 42,
      title: "Review the source model",
      description: "A long but safe source summary.",
      status: "in_review",
      priority: "high",
    }],
  },
  trigger_kind: "manual",
  requested_by_id: "member-1",
  created_at: "2026-08-11T08:00:00Z",
  review: null,
};

const proposal = {
  id: "proposal-2",
  kind: "evolution",
  source_wiki_revision_id: "wiki-2",
  base_twin_version_id: "version-1",
  schema_version: 1,
  content: {
    schema_version: 1,
    name: "Workspace Twin",
    assertions: [{
      id: "assertion-new",
      text: "Prefer explicit review decisions.",
      source_summary: "Review source",
      source_status: "in_review",
      citation_keys: ["issue:42"],
    }],
    topics: [{
      id: "topic-42",
      issue_id: "issue-42",
      issue_number: 42,
      title: "Review the source model",
      status: "in_review",
      citation_keys: ["issue:42"],
    }],
    diff: { added: ["assertion-new"], removed: ["assertion-old"], unchanged: [] },
  },
  content_digest: "sha256:proposal-2",
  requested_by_id: "member-1",
  created_at: "2026-08-11T08:05:00Z",
  review: null,
  signed_version: null,
};

const version = {
  id: "version-1",
  version_number: 1,
  proposal_id: "proposal-1",
  source_wiki_revision_id: "wiki-1",
  prior_version_id: null,
  schema_version: 1,
  content: { schema_version: 1, name: "Workspace Twin", assertions: [] },
  content_digest: "sha256:version-1",
  signed_off_by_id: "member-1",
  signed_off_at: "2026-08-10T08:00:00Z",
  created_at: "2026-08-10T08:00:00Z",
};

const citation = {
  id: "citation-1",
  ordinal: 0,
  citation_key: "issue:42",
  source_type: "issue",
  source_id: "issue-42",
  source_updated_at: "2026-08-11T07:00:00Z",
  locator: "issues/issue-42",
  label: "Issue #42: Review the source model",
  safe_metadata: { status: "in_review" },
  source_digest: "sha256:source-42",
};

export function lifecycleFixture() {
  const reviewSteps: readonly TwinReviewStep[] = [
    { id: "import", state: "complete" },
    { id: "generate", state: "complete" },
    { id: "topic", state: "complete" },
    { id: "coordinate", state: "current" },
    { id: "accept", state: "upcoming" },
    { id: "deposition", state: "upcoming" },
  ];
  const wiki: LMWikiOverview = {
    latest_revision: revision,
    accepted_revision: { ...revision, id: "wiki-1", revision_number: 1 },
    pending_revision: revision,
    revisions: [revision, { ...revision, id: "wiki-1", revision_number: 1 }],
    can_manage: true,
  };
  const wikiDetail: LMWikiDetail = { revision, citations: [citation] };
  const twin: TwinOverview = {
    current_version: version,
    pending_proposal: proposal,
    proposals: [proposal],
    versions: [version],
    can_manage: true,
  };
  const proposalDetail: TwinProposalDetail = {
    proposal,
    source_revision: revision,
    citations: [citation],
  };
  const versionDetail: TwinVersionDetail = {
    version,
    proposal: { ...proposal, id: "proposal-1", kind: "initial" },
    source_revision: { ...revision, id: "wiki-1", revision_number: 1 },
    citations: [citation],
  };
  return {
    wsId: "00000000-0000-4000-8000-000000000001",
    state: "ready" as const,
    overviewStale: false,
    wiki,
    wikiDetail,
    wikiDetailState: { kind: "ready" as const },
    twin,
    proposalDetail,
    proposalDetailState: { kind: "ready" as const },
    versionDetail,
    versionDetailState: { kind: "ready" as const },
    reviewSteps,
    selectedRevisionId: "wiki-2",
    selectedProposalId: "proposal-2",
    selectedVersionId: "version-1",
    canManageWiki: true,
    canManageTwin: true,
    wikiMutationPending: false,
    twinMutationPending: false,
    detailLoading: false,
    actionError: null,
    onSelectRevision: () => undefined,
    onSelectProposal: () => undefined,
    onSelectVersion: () => undefined,
    onRetryWikiDetail: () => undefined,
    onRetryProposalDetail: () => undefined,
    onRetryVersionDetail: () => undefined,
    onRefreshWiki: () => undefined,
    onAcceptWiki: async () => undefined,
    onRejectWiki: async () => undefined,
    onEnsureTwin: () => undefined,
    onAcceptTwin: async () => undefined,
    onRejectTwin: async () => undefined,
    onCorrectTwin: async () => undefined,
    onEditDeposition: async () => undefined,
    onRetry: () => undefined,
  };
}
