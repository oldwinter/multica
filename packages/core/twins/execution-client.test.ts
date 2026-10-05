// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Twin execution HTTP contract", () => {
  it("sends proposal corrections to the append-only review boundary", async () => {
    const proposal = {
      id: "proposal-2",
      kind: "correction",
      source_wiki_revision_id: "revision-1",
      base_twin_version_id: null,
      schema_version: 2,
      content: { schema_version: 2, assertions: [] },
      content_digest: "sha256:proposal-2",
      requested_by_id: "member-1",
      replaces_proposal_id: "proposal-1",
      created_at: "2026-08-23T12:00:00Z",
      review: null,
      signed_version: null,
    };
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ created: true, proposal }), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    const result = await client.correctTwinProposal("proposal-1", []);

    expect(result.proposal.replaces_proposal_id).toBe("proposal-1");
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.example.test/api/twins/proposals/proposal-1/correct",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ edited_assertions: [] }) }),
    );
  });

  it("keeps the initial deposition request bodyless", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.createTwinDeposition("00000000-0000-4000-8000-000000000008", {});

    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.example.test/api/twins/tasks/00000000-0000-4000-8000-000000000008/depositions",
      expect.objectContaining({ method: "POST" }),
    );
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBeUndefined();
  });

  it("uses the frozen endpoint paths and wire request bodies", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => new Response("{}", {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.getTwinBindings();
    await client.getTwinActivationReadiness();
    await client.upsertTwinBinding({
      scopeType: "project",
      scopeId: "00000000-0000-4000-8000-000000000002",
      state: "preview",
      twinVersionId: "00000000-0000-4000-8000-000000000003",
    });
    await client.deleteTwinBinding("00000000-0000-4000-8000-000000000004");
    await client.pauseTwinExecution();
    await client.previewTwinBriefing({
      agentId: "00000000-0000-4000-8000-000000000005",
      projectId: "00000000-0000-4000-8000-000000000002",
      issueId: "00000000-0000-4000-8000-000000000006",
      runId: "00000000-0000-4000-8000-000000000007",
      request: "Review authentication",
      tags: ["security", "auth"],
      oneOffState: "enabled",
      twinVersionId: "00000000-0000-4000-8000-000000000003",
    });
    await client.getTwinTaskContext("00000000-0000-4000-8000-000000000008");
    await client.submitTwinTaskFeedback(
      "00000000-0000-4000-8000-000000000008",
      { rating: "helped", note: "kept scope precise" },
    );
    await client.createTwinDeposition("00000000-0000-4000-8000-000000000008", {
      replacesProposalId: "00000000-0000-4000-8000-000000000009",
      editedAssertions: [{
        id: "review.explicit",
        type: "quality_bar",
        text: "Keep review decisions explicit.",
        applicability: { keywords: ["review"] },
        evidence_citations: ["issue:42"],
      }],
    });
    await client.getTwinExecutionMetrics();

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "https://api.example.test/api/twins/bindings",
      "https://api.example.test/api/twins/activation",
      "https://api.example.test/api/twins/bindings",
      "https://api.example.test/api/twins/bindings/00000000-0000-4000-8000-000000000004",
      "https://api.example.test/api/twins/pause",
      "https://api.example.test/api/twins/briefings/preview",
      "https://api.example.test/api/twins/tasks/00000000-0000-4000-8000-000000000008/context",
      "https://api.example.test/api/twins/tasks/00000000-0000-4000-8000-000000000008/feedback",
      "https://api.example.test/api/twins/tasks/00000000-0000-4000-8000-000000000008/depositions",
      "https://api.example.test/api/twins/metrics",
    ]);
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body))).toEqual({
      scope_type: "project",
      scope_id: "00000000-0000-4000-8000-000000000002",
      state: "preview",
      twin_version_id: "00000000-0000-4000-8000-000000000003",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[5]?.[1]?.body))).toEqual({
      agent_id: "00000000-0000-4000-8000-000000000005",
      project_id: "00000000-0000-4000-8000-000000000002",
      issue_id: "00000000-0000-4000-8000-000000000006",
      run_id: "00000000-0000-4000-8000-000000000007",
      request: "Review authentication",
      tags: ["security", "auth"],
      one_off_state: "enabled",
      twin_version_id: "00000000-0000-4000-8000-000000000003",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[7]?.[1]?.body))).toEqual({
      rating: "helped",
      note: "kept scope precise",
    });
    expect(JSON.parse(String(fetchMock.mock.calls[8]?.[1]?.body))).toEqual({
      replaces_proposal_id: "00000000-0000-4000-8000-000000000009",
      edited_assertions: [{
        id: "review.explicit",
        type: "quality_bar",
        text: "Keep review decisions explicit.",
        applicability: { keywords: ["review"] },
        evidence_citations: ["issue:42"],
      }],
    });
    expect(fetchMock.mock.calls.map(([, init]) => init?.method ?? "GET")).toEqual([
      "GET", "GET", "POST", "DELETE", "POST", "POST", "GET", "PUT", "POST", "GET",
    ]);
  });
});
