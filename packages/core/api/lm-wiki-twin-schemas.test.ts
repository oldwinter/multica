import { describe, expect, it } from "vitest";
import {
  LMWikiDetailSchema,
  LMWikiOverviewSchema,
  TwinOverviewSchema,
  TwinVersionDetailSchema,
} from "./schemas";

const revision = {
  id: "revision-1",
  revision_number: 1,
  schema_version: 1,
  source_digest: "sha256:wiki",
  content: { schema_version: 1, issues: [] },
  trigger_kind: "manual",
  requested_by_id: null,
  created_at: "2026-08-11T00:00:00Z",
  review: null,
};

const citation = {
  id: "citation-1",
  ordinal: 0,
  citation_key: "issue:issue-1",
  source_type: "issue",
  source_id: "issue-1",
  source_updated_at: null,
  locator: "issue:MUL-1",
  label: "MUL-1",
  safe_metadata: { title: "Issue" },
  source_digest: "sha256:source",
};

const proposal = {
  id: "proposal-1",
  kind: "initial",
  source_wiki_revision_id: "revision-1",
  base_twin_version_id: null,
  schema_version: 1,
  content: { schema_version: 1, assertions: [] },
  content_digest: "sha256:twin",
  requested_by_id: null,
  replaces_proposal_id: null,
  created_at: "2026-08-11T00:00:00Z",
  review: null,
  signed_version: null,
};

const version = {
  id: "version-1",
  version_number: 1,
  proposal_id: "proposal-1",
  source_wiki_revision_id: "revision-1",
  prior_version_id: null,
  schema_version: 1,
  content: { schema_version: 1, assertions: [] },
  content_digest: "sha256:twin",
  signed_off_by_id: "member-1",
  signed_off_at: "2026-08-11T00:00:00Z",
  created_at: "2026-08-11T00:00:00Z",
};

describe("LM Wiki wire schemas", () => {
  it("defaults optional overview fields and preserves additive fields", () => {
    const parsed = LMWikiOverviewSchema.parse({
      latest_revision: revision,
      future_server_field: true,
    });

    expect(parsed).toMatchObject({
      latest_revision: revision,
      accepted_revision: null,
      pending_revision: null,
      revisions: [],
      can_manage: false,
      future_server_field: true,
    });
  });

  it("accepts unknown review and trigger display enums", () => {
    const parsed = LMWikiDetailSchema.parse({
      revision: {
        ...revision,
        trigger_kind: "backfill",
        review: {
          id: "review-1",
          decision: "superseded",
          reviewer_id: "member-1",
          reason: null,
          created_at: "2026-08-11T00:00:00Z",
        },
      },
    });

    expect(parsed.revision.trigger_kind).toBe("backfill");
    expect(parsed.revision.review?.decision).toBe("superseded");
    expect(parsed.citations).toEqual([]);
  });

  it("rejects a detail when a critical revision id or content is structurally invalid", () => {
    for (const malformed of [
      { revision: { ...revision, id: "" }, citations: [citation] },
      { revision: { ...revision, content: [] }, citations: [citation] },
      { revision, citations: "not-an-array" },
    ]) {
      expect(LMWikiDetailSchema.safeParse(malformed).success).toBe(false);
    }
  });
});

describe("Twin wire schemas", () => {
  it("defaults missing overview arrays and can_manage to safe values", () => {
    const parsed = TwinOverviewSchema.parse({
      current_version: null,
      pending_proposal: null,
    });

    expect(parsed).toEqual({
      current_version: null,
      pending_proposal: null,
      proposals: [],
      versions: [],
      can_manage: false,
    });
  });

  it("preserves unknown proposal and review display enums", () => {
    const parsed = TwinVersionDetailSchema.parse({
      version,
      proposal: {
        ...proposal,
        kind: "reconciled",
        review: {
          id: "review-1",
          decision: "deferred",
          reviewer_id: "member-1",
          reason: null,
          created_at: "2026-08-11T00:00:00Z",
        },
      },
      source_revision: revision,
      citations: [citation],
      future_server_field: "kept",
    });

    expect(parsed.proposal.kind).toBe("reconciled");
    expect(parsed.proposal.review?.decision).toBe("deferred");
    expect(parsed.future_server_field).toBe("kept");
  });

  it("keeps append-only proposal replacement identity backward compatible", () => {
    const legacy = TwinOverviewSchema.parse({ pending_proposal: proposal });
    expect(legacy.pending_proposal?.replaces_proposal_id).toBeNull();

    const corrected = TwinOverviewSchema.parse({
      pending_proposal: { ...proposal, kind: "correction", replaces_proposal_id: "proposal-0" },
    });
    expect(corrected.pending_proposal?.replaces_proposal_id).toBe("proposal-0");
    expect(TwinOverviewSchema.safeParse({
      pending_proposal: { ...proposal, replaces_proposal_id: { invalid: true } },
    }).success).toBe(false);
  });

  it("rejects Twin details that lose a critical id, content, or citations array", () => {
    for (const malformed of [
      { version: { ...version, id: "" }, proposal, source_revision: revision },
      { version: { ...version, content: null }, proposal, source_revision: revision },
      { version, proposal, source_revision: revision, citations: {} },
    ]) {
      expect(TwinVersionDetailSchema.safeParse(malformed).success).toBe(false);
    }
  });
});
