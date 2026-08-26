// @vitest-environment node

import { describe, expect, it } from "vitest";
import { parseWithFallback } from "../api/schema";
import {
  EMPTY_WIKI_PAGE,
  EMPTY_LM_WIKI_SOURCE_POLICY,
  EMPTY_WIKI_KNOWLEDGE_READINESS,
  EMPTY_WIKI_PAGE_LIST,
  EMPTY_WIKI_PROPOSAL_LIST,
  EMPTY_WIKI_REVISION_LIST,
  WikiPageListSchema,
  WikiPageSchema,
  WikiProposalListSchema,
  WikiRevisionListSchema,
  LMWikiSourcePolicySchema,
  WikiKnowledgeReadinessSchema,
  parseLMWikiSourcePolicyStale,
  parseWikiRevisionConflict,
} from "./schemas";

const summary = {
  id: "page-1",
  workspace_id: "ws-1",
  scope: "workspace",
  project_id: null,
  owner_user_id: null,
  path: "guide.md",
  title: "Guide",
  created_by: "user-1",
  current_revision_number: 3,
  current_revision_id: "revision-3",
  content_digest: "sha256:page",
  last_source_kind: "human",
  last_actor_type: "member",
  last_actor_id: "user-1",
  created_at: "2026-08-23T10:00:00Z",
  updated_at: "2026-08-23T11:00:00Z",
};

describe("Wiki API schemas", () => {
  it("parses positive revision and provenance fields", () => {
    const parsed = WikiPageSchema.parse({
      ...summary,
      content: "# Guide",
    });
    expect(parsed.currentRevisionNumber).toBe(3);
    expect(parsed.lastSourceKind).toBe("human");
    expect(parsed).toMatchObject({
      workspaceId: "ws-1",
      currentRevisionId: "revision-3",
      contentDigest: "sha256:page",
    });

  });

  it("degrades malformed page lists instead of leaking partial wire data", () => {
    expect(parseWithFallback(
      [{ ...summary, current_revision_number: "three" }],
      WikiPageListSchema,
      EMPTY_WIKI_PAGE_LIST,
      { endpoint: "GET /api/wiki/pages" },
    )).toEqual([]);
    expect(parseWithFallback(
      { ...summary, scope: "public", content: "" },
      WikiPageSchema,
      EMPTY_WIKI_PAGE,
      { endpoint: "GET /api/wiki/pages/:id" },
    )).toEqual(EMPTY_WIKI_PAGE);
  });

  it("rejects malformed revisions and proposals", () => {
    expect(parseWithFallback(
      [{ id: "revision-1", revision_number: "1" }],
      WikiRevisionListSchema,
      EMPTY_WIKI_REVISION_LIST,
      { endpoint: "GET /api/wiki/pages/:id/revisions" },
    )).toEqual([]);
    expect(parseWithFallback(
      [{ id: "proposal-1", evidence_refs: "run-1" }],
      WikiProposalListSchema,
      EMPTY_WIKI_PROPOSAL_LIST,
      { endpoint: "GET /api/wiki/pages/:id/proposals" },
    )).toEqual([]);
  });

  it("rejects missing or invalid identity and digest fields", () => {
    const { content_digest: _digest, ...withoutDigest } = summary;
    expect(parseWithFallback(
      [withoutDigest],
      WikiPageListSchema,
      EMPTY_WIKI_PAGE_LIST,
      { endpoint: "GET /api/wiki/pages" },
    )).toEqual([]);
    const { current_revision_id: _revisionId, ...withoutRevisionId } = summary;
    expect(parseWithFallback(
      [withoutRevisionId],
      WikiPageListSchema,
      EMPTY_WIKI_PAGE_LIST,
      { endpoint: "GET /api/wiki/pages" },
    )).toEqual([]);
    expect(parseWithFallback(
      [{ ...summary, id: "", current_revision_number: 0 }],
      WikiPageListSchema,
      EMPTY_WIKI_PAGE_LIST,
      { endpoint: "GET /api/wiki/pages" },
    )).toEqual([]);
  });

  it("maps future provenance enums to unknown but rejects unknown LM source classes", () => {
    const parsed = WikiPageSchema.parse({
      ...summary,
      last_source_kind: "generated",
      last_actor_type: "service",
      content: "",
    });
    expect(parsed.lastSourceKind).toBe("unknown");
    expect(parsed.lastActorType).toBe("unknown");

    expect(parseWithFallback(
      { source_classes: ["personal_wiki"], wiki_pages: [] },
      LMWikiSourcePolicySchema,
      EMPTY_LM_WIKI_SOURCE_POLICY,
      { endpoint: "GET /api/lm-wiki/source-policy" },
    )).toEqual(EMPTY_LM_WIKI_SOURCE_POLICY);
  });

  it("defaults remote generation to off for older source-policy responses", () => {
    expect(LMWikiSourcePolicySchema.parse({
      source_classes: ["issue"],
      wiki_pages: [],
    })).toEqual({
      sourceClasses: ["issue"],
      wikiPages: [],
      remoteGenerationEnabled: false,
      policyVersion: 0,
      policyDigest: "",
      exclusions: [],
    });
  });

  it("maps revision conflict wire bodies to the public camelCase model", () => {
    expect(parseWikiRevisionConflict({
      code: "wiki_revision_conflict",
      current_revision_number: 4,
    })).toEqual({ currentRevisionNumber: 4 });
    expect(parseWikiRevisionConflict({ code: "other", current_revision_number: 4 })).toBeNull();
  });

  it("parses bounded knowledge readiness and stale-policy conflicts", () => {
    const policy = {
      source_classes: ["issue", "wiki_page"],
      wiki_pages: [{ page_id: "page-1", revision_number: 2 }],
      remote_generation_enabled: false,
      policy_version: 3,
      policy_digest: "sha256:policy",
      exclusions: [],
    };
    const readiness = WikiKnowledgeReadinessSchema.parse({
      schema_version: 1,
      policy,
      sources: [{
        page_id: "page-1",
        scope: "workspace",
        state: "newer_revision_available",
        reason_code: "newer_revision_available",
        responsible_role: "owner_admin",
        selected_revision_id: "revision-2",
        selected_revision_number: 2,
        current_revision_id: "revision-3",
        current_revision_number: 3,
        policy_version: 3,
        next_action: {
          kind: "pin_revision",
          page_id: "page-1",
          revision_id: "revision-3",
          revision_number: 3,
        },
      }],
      maintenance_items: [{
        id: "queue-1",
        kind: "source_newer_revision",
        severity: "warning",
        reason_code: "newer_revision_available",
        responsible_role: "owner_admin",
        page_id: "page-1",
        selected_revision_number: 2,
        policy_version: 3,
        next_action: {
          kind: "pin_revision",
          page_id: "page-1",
          revision_id: "revision-3",
          revision_number: 3,
        },
      }],
      truncated: false,
      can_manage: true,
    });
    expect(readiness).toMatchObject({
      schemaVersion: 1,
      canManage: true,
      sources: [{ pageId: "page-1", state: "newer_revision_available" }],
      maintenanceItems: [{ kind: "source_newer_revision" }],
    });
    expect(parseLMWikiSourcePolicyStale({
      code: "wiki_source_policy_stale",
      current_policy: policy,
    })).toMatchObject({ currentPolicy: { policyVersion: 3, policyDigest: "sha256:policy" } });
  });

  it("fails malformed readiness responses closed", () => {
    expect(parseWithFallback(
      {
        schema_version: 1,
        policy: { source_classes: [], wiki_pages: [] },
        sources: [{ page_id: "page-1", state: "current" }],
        maintenance_items: [],
        truncated: false,
        can_manage: true,
      },
      WikiKnowledgeReadinessSchema,
      EMPTY_WIKI_KNOWLEDGE_READINESS,
      { endpoint: "GET /api/wiki/knowledge-readiness" },
    )).toEqual(EMPTY_WIKI_KNOWLEDGE_READINESS);
    expect(parseLMWikiSourcePolicyStale({ code: "wiki_source_policy_stale" })).toBeNull();
  });
});
