// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "../api/client";
import { parseLMWikiSourcePolicyStale } from "./schemas";

const page = {
  id: "page-1",
  workspace_id: "ws-1",
  scope: "workspace",
  project_id: null,
  owner_user_id: null,
  path: "guide.md",
  title: "Guide",
  content: "# Guide",
  created_by: "user-1",
  current_revision_number: 2,
  current_revision_id: "revision-2",
  content_digest: "sha256:page",
  last_source_kind: "human",
  last_actor_type: "member",
  last_actor_id: "user-1",
  created_at: "2026-08-23T10:00:00Z",
  updated_at: "2026-08-23T11:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(body: unknown, status = 200) {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("Wiki API client", () => {
  it("uses workspace-free Personal Wiki routes and serializes CAS writes", async () => {
    const personalPage = {
      ...page,
      workspace_id: null,
      scope: "user",
      owner_user_id: "user-1",
    };
    const revision = {
      id: "revision-2",
      page_id: "page-1",
      revision_number: 2,
      path: "guide.md",
      title: "Guide",
      content: "# Guide",
      content_digest: "sha256:page",
      actor_type: "member",
      actor_id: "user-1",
      source_kind: "human",
      source_ref_id: null,
      created_at: "2026-08-23T11:00:00Z",
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond([personalPage]))
      .mockResolvedValueOnce(respond([personalPage]))
      .mockResolvedValueOnce(respond(personalPage, 201))
      .mockResolvedValueOnce(respond({ ...personalPage, current_revision_number: 3 }))
      .mockResolvedValueOnce(respond([revision]))
      .mockResolvedValueOnce(respond({ ...personalPage, current_revision_number: 4 }))
      .mockResolvedValueOnce(respond(revision))
      .mockResolvedValueOnce(respond(revision))
      .mockResolvedValueOnce(respond(undefined, 204));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.listPersonalWikiPages();
    await client.searchPersonalWikiPages({ q: " private note " });
    await client.createPersonalWikiPage({ path: "notes.md", title: "Notes", content: "private" });
    await client.updatePersonalWikiPage("page/1", { expectedRevisionNumber: 2, content: "new" });
    await client.listPersonalWikiRevisions("page/1");
    await client.restorePersonalWikiRevision("page/1", "revision/1", 3);
    await client.getPersonalWikiPageRevision("page/1", "revision/2");
    await client.getPersonalWikiRevision("revision/2");
    await client.deletePersonalWikiPage("page/1");

    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      "https://api.example.test/api/personal-wiki/pages",
      "https://api.example.test/api/personal-wiki/search?q=+private+note+",
      "https://api.example.test/api/personal-wiki/pages",
      "https://api.example.test/api/personal-wiki/pages/page%2F1",
      "https://api.example.test/api/personal-wiki/pages/page%2F1/revisions",
      "https://api.example.test/api/personal-wiki/pages/page%2F1/revisions/revision%2F1/restore",
      "https://api.example.test/api/personal-wiki/pages/page%2F1/revisions/revision%2F2",
      "https://api.example.test/api/personal-wiki/revisions/revision%2F2",
      "https://api.example.test/api/personal-wiki/pages/page%2F1",
    ]);
    expect(JSON.parse(String((fetchMock.mock.calls[3]?.[1] as RequestInit).body))).toEqual({
      expected_revision_number: 2,
      content: "new",
    });
    expect(JSON.parse(String((fetchMock.mock.calls[5]?.[1] as RequestInit).body))).toEqual({
      expected_revision_number: 3,
    });
  });

  it("loads immutable shared revisions and rejects malformed identity with an empty fallback", async () => {
    const fetchMock = vi.fn().mockResolvedValue(respond({ revision_number: 2 }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getWikiRevision("revision/2")).resolves.toMatchObject({ id: "" });
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://api.example.test/api/wiki/revisions/revision%2F2",
    );
  });

  it("encodes search filters and parses a summary list", async () => {
    const fetchMock = vi.fn().mockResolvedValue(respond([page]));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.searchWikiPages({
      q: "quality bar",
      scope: "project",
      projectId: "project/1",
    })).resolves.toMatchObject([{ id: "page-1", currentRevisionNumber: 2 }]);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://api.example.test/api/wiki/search?q=quality+bar&scope=project&project_id=project%2F1",
    );
  });

  it("degrades malformed search responses to an empty list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(respond({ pages: [page] })));
    const client = new ApiClient("https://api.example.test");
    await expect(client.searchWikiPages({ q: "guide" })).resolves.toEqual([]);
  });

  it("sends optimistic concurrency fields for page updates and restores", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond({ ...page, current_revision_number: 3 }))
      .mockResolvedValueOnce(respond({ ...page, current_revision_number: 4 }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.updateWikiPage("page/1", {
      expectedRevisionNumber: 2,
      content: "updated",
    });
    await client.restoreWikiRevision("page/1", "revision/1", 3);

    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://api.example.test/api/wiki/pages/page%2F1");
    expect(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body))).toEqual({
      expected_revision_number: 2,
      content: "updated",
    });
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "https://api.example.test/api/wiki/pages/page%2F1/revisions/revision%2F1/restore",
    );
  });

  it("keeps proposal review edits and expected revision in the accept body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(respond({ ...page, current_revision_number: 3 }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.acceptWikiProposal("page-1", {
      proposalId: "proposal-1",
      expectedRevisionNumber: 2,
      title: "Reviewed title",
      content: "Reviewed content",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://api.example.test/api/wiki/pages/page-1/proposals/proposal-1/accept",
    );
    expect(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body))).toEqual({
      expected_revision_number: 2,
      title: "Reviewed title",
      content: "Reviewed content",
    });
  });

  it("parses and updates the minimal LM Wiki source policy contract", async () => {
    const wirePolicy = {
      source_classes: ["issue", "wiki_page"],
      wiki_pages: [{ page_id: "page-1", revision_number: 2 }],
      remote_generation_enabled: false,
      policy_version: 3,
      policy_digest: "sha256:policy",
      exclusions: [{
        source_class: "personal_wiki",
        state: "always_excluded",
        reason: "personal_scope_never_eligible",
      }],
    } as const;
    const policy = {
      sourceClasses: ["issue", "wiki_page"],
      wikiPages: [{ pageId: "page-1", revisionNumber: 2 }],
      remoteGenerationEnabled: false,
      policyVersion: 3,
      policyDigest: "sha256:policy",
      exclusions: [{
        sourceClass: "personal_wiki",
        state: "always_excluded",
        reason: "personal_scope_never_eligible",
      }],
    } as const;
    const update = {
      sourceClasses: policy.sourceClasses,
      wikiPages: policy.wikiPages,
      remoteGenerationEnabled: policy.remoteGenerationEnabled,
    } as const;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond(wirePolicy))
      .mockResolvedValueOnce(respond(wirePolicy));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getLMWikiSourcePolicy()).resolves.toEqual(policy);
    await expect(client.updateLMWikiSourcePolicy(update)).resolves.toEqual(policy);
    expect(fetchMock.mock.calls[1]?.[0]).toBe("https://api.example.test/api/lm-wiki/source-policy");
    expect((fetchMock.mock.calls[1]?.[1] as RequestInit).method).toBe("PUT");
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toEqual({
      source_classes: wirePolicy.source_classes,
      wiki_pages: wirePolicy.wiki_pages,
      remote_generation_enabled: false,
    });
  });

  it("loads readiness and pins an encoded exact revision with a policy expectation", async () => {
    const wirePolicy = {
      source_classes: ["wiki_page"],
      wiki_pages: [{ page_id: "page/1", revision_number: 2 }],
      remote_generation_enabled: false,
      policy_version: 4,
      policy_digest: "sha256:policy",
      exclusions: [],
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond({
        schema_version: 1,
        policy: wirePolicy,
        sources: [{
          page_id: "page/1",
          scope: "workspace",
          state: "pinned_current",
          reason_code: "revision_current",
          responsible_role: "owner_admin",
          selected_revision_id: "revision/2",
          selected_revision_number: 2,
          current_revision_id: "revision/2",
          current_revision_number: 2,
          policy_version: 4,
          next_action: { kind: "none" },
        }],
        maintenance_items: [],
        truncated: false,
        can_manage: true,
      }))
      .mockResolvedValueOnce(respond(wirePolicy));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getWikiKnowledgeReadiness()).resolves.toMatchObject({
      canManage: true,
      sources: [{ pageId: "page/1", state: "pinned_current" }],
    });
    await client.pinWikiRevisionAsLMWikiEvidence({
      pageId: "page/1",
      revisionId: "revision/2",
      expectedPolicyVersion: 4,
      expectedPolicyDigest: "sha256:policy",
    });

    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "https://api.example.test/api/lm-wiki/source-policy/wiki-pages/page%2F1/revisions/revision%2F2",
    );
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toEqual({
      expected_policy_version: 4,
      expected_policy_digest: "sha256:policy",
    });
  });

  it("preserves the authoritative policy from a structured stale response", async () => {
    const currentPolicy = {
      source_classes: ["wiki_page"],
      wiki_pages: [],
      remote_generation_enabled: false,
      policy_version: 9,
      policy_digest: "sha256:current",
      exclusions: [],
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(respond({
      error: "policy changed",
      code: "wiki_source_policy_stale",
      current_policy: currentPolicy,
    }, 409)));
    const client = new ApiClient("https://api.example.test");

    try {
      await client.pinWikiRevisionAsLMWikiEvidence({
        pageId: "page-1",
        revisionId: "revision-1",
        expectedPolicyVersion: 8,
        expectedPolicyDigest: "sha256:stale",
      });
      throw new Error("expected pin to fail");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      const conflict = parseLMWikiSourcePolicyStale((error as ApiError).body);
      expect(conflict?.currentPolicy).toMatchObject({
        policyVersion: 9,
        policyDigest: "sha256:current",
      });
    }
  });

  it("serializes camelCase page and proposal inputs to wire field names", async () => {
    const proposal = {
      id: "proposal-1",
      page_id: "page-1",
      base_revision_number: 2,
      proposed_path: "guide.md",
      proposed_title: "Guide",
      proposed_content: "# Guide",
      content_digest: "sha256:proposal",
      rationale: "Keep docs current",
      evidence_refs: ["run:1"],
      agent_id: "agent-1",
      idempotency_key: "proposal-key",
      status: "pending",
      reviewed_by_id: null,
      review_reason: null,
      reviewed_at: null,
      accepted_revision_id: null,
      created_at: "2026-08-23T11:00:00Z",
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(respond(page))
      .mockResolvedValueOnce(respond(proposal));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.createWikiPage({
      scope: "project",
      projectId: "project-1",
      path: "guide.md",
      title: "Guide",
    });
    await client.createWikiProposal("page-1", {
      baseRevisionNumber: 2,
      proposedPath: "guide.md",
      proposedTitle: "Guide",
      proposedContent: "# Guide",
      rationale: "Keep docs current",
      evidenceRefs: ["run:1"],
      agentId: "agent-1",
      idempotencyKey: "proposal-key",
    });

    expect(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body))).toMatchObject({
      project_id: "project-1",
    });
    expect(JSON.parse(String((fetchMock.mock.calls[1]?.[1] as RequestInit).body))).toMatchObject({
      base_revision_number: 2,
      evidence_refs: ["run:1"],
      agent_id: "agent-1",
    });
  });
});
