// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  WikiPageSchema,
  WikiKnowledgeReadinessSchema,
  WikiPageSummaryListSchema,
  WikiProposalSchema,
  WikiRevisionSchema,
  buildAcceptWikiProposalBody,
  buildRejectWikiProposalBody,
  buildPinWikiRevisionBody,
  buildUpdateWikiPageBody,
  getWikiRevisionConflict,
  getLMWikiSourcePolicyStaleConflict,
} from "./wiki-schema";

const page = {
  id: "page-1",
  workspace_id: "ws-1",
  scope: "workspace",
  project_id: null,
  owner_user_id: null,
  path: "playbook/on-call.md",
  title: "值班交接清单与异常处理说明",
  content: "# On-call\n\nEscalate after 15 minutes.",
  current_revision_id: "revision-3",
  current_revision_number: 3,
  content_digest: "sha256:page-3",
  last_source_kind: "human",
  last_actor_type: "member",
  last_actor_id: "user-1",
  created_by: "user-1",
  created_at: "2026-08-23T10:00:00Z",
  updated_at: "2026-08-23T11:00:00Z",
};

describe("mobile Wiki API schemas", () => {
  it("preserves canonical page identity and revision metadata", () => {
    expect(WikiPageSchema.parse(page)).toMatchObject({
      id: "page-1",
      scope: "workspace",
      currentRevisionId: "revision-3",
      currentRevisionNumber: 3,
      contentDigest: "sha256:page-3",
      lastSourceKind: "human",
      lastActorType: "member",
    });
  });

  it("keeps future enum values visible through an unknown fallback", () => {
    const parsed = WikiPageSchema.parse({
      ...page,
      scope: "organization",
      last_source_kind: "repository_import",
      last_actor_type: "service_account",
    });

    expect(parsed.scope).toBe("unknown");
    expect(parsed.lastSourceKind).toBe("unknown");
    expect(parsed.lastActorType).toBe("unknown");
  });

  it("parses direct array search responses", () => {
    expect(
      WikiPageSummaryListSchema.parse([
        (({ content: _content, ...summary }) => summary)(page),
      ]),
    ).toHaveLength(1);
  });

  it("parses append-only revisions and reviewable proposals", () => {
    expect(
      WikiRevisionSchema.parse({
        id: "revision-3",
        page_id: "page-1",
        revision_number: 3,
        path: page.path,
        title: page.title,
        content: page.content,
        content_digest: page.content_digest,
        source_kind: "restore",
        source_ref_id: "revision-1",
        actor_type: "member",
        actor_id: "user-1",
        created_at: page.updated_at,
      }),
    ).toMatchObject({ revisionNumber: 3, sourceKind: "restore" });

    expect(
      WikiProposalSchema.parse({
        id: "proposal-1",
        page_id: "page-1",
        base_revision_number: 3,
        proposed_path: page.path,
        proposed_title: "Updated checklist",
        proposed_content: "# Updated",
        rationale: "The completed run found a missing escalation step.",
        status: "pending",
        content_digest: "sha256:proposal-1",
        evidence_refs: [],
        agent_id: "agent-1",
        idempotency_key: "proposal-key-1",
        reviewed_by_id: null,
        review_reason: null,
        reviewed_at: null,
        accepted_revision_id: null,
        created_at: page.updated_at,
      }),
    ).toMatchObject({ status: "pending", baseRevisionNumber: 3 });
  });

  it("recognizes structured stale-edit conflicts", () => {
    expect(
      getWikiRevisionConflict({
        code: "wiki_revision_conflict",
        current_revision_number: 4,
      }),
    ).toEqual({ currentRevisionNumber: 4 });
    expect(getWikiRevisionConflict({ code: "other" })).toBeNull();
  });

  it("rejects non-positive server revision numbers", () => {
    expect(
      WikiPageSchema.safeParse({ ...page, current_revision_number: 0 }).success,
    ).toBe(false);
  });

  it("serializes accept edits and reject reasons as separate contracts", () => {
    expect(
      buildAcceptWikiProposalBody({
        expectedRevisionNumber: 3,
        path: "playbook/reviewed.md",
        content: "# Reviewed",
      }),
    ).toEqual({
      expected_revision_number: 3,
      path: "playbook/reviewed.md",
      content: "# Reviewed",
    });
    expect(buildRejectWikiProposalBody({})).toEqual({});
    expect(buildRejectWikiProposalBody({ reason: "Insufficient evidence" })).toEqual({
      reason: "Insufficient evidence",
    });
    expect(
      buildUpdateWikiPageBody({
        expectedRevisionNumber: 4,
        title: "Reviewed title",
      }),
    ).toEqual({
      expected_revision_number: 4,
      title: "Reviewed title",
    });
  });

  it("parses the bounded readiness model without introducing Wiki content", () => {
    const parsed = WikiKnowledgeReadinessSchema.parse({
      schema_version: 1,
      policy: {
        source_classes: ["wiki_page"],
        wiki_pages: [{ page_id: "page-1", revision_number: 2 }],
        remote_generation_enabled: false,
        policy_version: 5,
        policy_digest: "sha256:policy-5",
        exclusions: [],
      },
      sources: [{
        page_id: "page-1",
        scope: "workspace",
        state: "newer_revision_available",
        reason_code: "wiki_source_newer_revision_available",
        responsible_role: "owner_admin",
        selected_revision_id: "revision-2",
        selected_revision_number: 2,
        current_revision_id: "revision-3",
        current_revision_number: 3,
        policy_version: 5,
        next_action: {
          kind: "pin_revision",
          page_id: "page-1",
          revision_id: "revision-3",
          revision_number: 3,
        },
        content: "must not cross the readiness boundary",
      }],
      maintenance_items: [],
      truncated: false,
      can_manage: true,
    });

    expect(parsed.sources[0]).toMatchObject({
      state: "newer_revision_available",
      selectedRevisionNumber: 2,
      currentRevisionNumber: 3,
      nextAction: { kind: "pin_revision", revisionId: "revision-3" },
    });
    expect(JSON.stringify(parsed)).not.toContain("must not cross");
  });

  it("serializes pin CAS identity and parses the structured stale result", () => {
    expect(buildPinWikiRevisionBody({
      pageId: "page-1",
      revisionId: "revision-3",
      expectedPolicyVersion: 5,
      expectedPolicyDigest: "sha256:policy-5",
    })).toEqual({
      expected_policy_version: 5,
      expected_policy_digest: "sha256:policy-5",
    });
    expect(getLMWikiSourcePolicyStaleConflict({
      code: "wiki_source_policy_stale",
      current_policy: {
        source_classes: [],
        wiki_pages: [],
        remote_generation_enabled: false,
        policy_version: 6,
        policy_digest: "sha256:policy-6",
        exclusions: [],
      },
    })?.currentPolicy.policyVersion).toBe(6);
  });
});
