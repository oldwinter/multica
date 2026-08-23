// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { AgentTaskSchema } from "../api/schemas";
import {
  TwinBindingsResponseSchema,
  TwinDepositionWireSchema,
  TwinTaskContextWireSchema,
} from "./execution-schemas";

const IDS = {
  workspace: "00000000-0000-4000-8000-000000000001",
  binding: "00000000-0000-4000-8000-000000000002",
  version: "00000000-0000-4000-8000-000000000003",
  task: "00000000-0000-4000-8000-000000000004",
  feedback: "00000000-0000-4000-8000-000000000005",
};
const DIGEST_A = `sha256:${"a".repeat(64)}`;
const DIGEST_B = `sha256:${"b".repeat(64)}`;

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Twin execution response schemas", () => {
  it("converts valid binding wire fields and drops only malformed rows", () => {
    const parsed = TwinBindingsResponseSchema.parse({
      bindings: [
        {
          id: IDS.binding,
          scope_type: "workspace",
          scope_id: IDS.workspace,
          state: "preview",
          twin_version_id: IDS.version,
          created_at: "2026-08-23T00:00:00Z",
          updated_at: "2026-08-23T00:00:00Z",
        },
        { id: "not-a-uuid", state: "enabled" },
      ],
      can_manage: true,
      kill_switch: { enabled: true },
    });

    expect(parsed).toMatchObject({
      canManage: true,
      killSwitch: { enabled: true, reason: null },
      bindings: [{
        id: IDS.binding,
        scopeType: "workspace",
        scopeId: IDS.workspace,
        state: "preview",
        twinVersionId: IDS.version,
      }],
    });
  });

  it("drops a malformed attribution without inventing audit evidence", () => {
    const parsed = TwinTaskContextWireSchema.parse({
      task_id: IDS.task,
      attribution: {
        twin_version_id: IDS.version,
        twin_version_number: 2,
        twin_version_digest: DIGEST_A,
        briefing: "Bounded briefing",
        briefing_digest: "sha256:not-a-digest",
        assertion_ids: ["quality.review"],
        citation_keys: ["issue:42"],
        policy_scope_type: "workspace",
        policy_scope_id: IDS.workspace,
        policy_state: "enabled",
        compiler_version: "twin-briefing/v1",
        byte_count: 16,
        token_count: 4,
      },
      feedback: {
        id: IDS.feedback,
        task_id: IDS.task,
        rating: "helped",
      },
      depositions: [],
    });

    expect(parsed.attribution).toBeUndefined();
    expect(parsed.feedback?.rating).toBe("helped");
  });

  it("keeps valid deposition replacement lineage and drops malformed optional lineage", () => {
    const base = {
      id: IDS.feedback,
      task_id: IDS.task,
      base_twin_version_id: IDS.version,
      proposal_id: IDS.binding,
      evidence_digest: DIGEST_A,
    };

    expect(TwinDepositionWireSchema.parse({
      ...base,
      replaces_proposal_id: IDS.workspace,
    }).replacesProposalId).toBe(IDS.workspace);
    expect(TwinDepositionWireSchema.parse({
      ...base,
      replaces_proposal_id: "not-a-uuid",
    }).replacesProposalId).toBeUndefined();
  });

  it("keeps an AgentTask when its optional Twin extension is malformed", () => {
    const parsed = AgentTaskSchema.parse({
      id: IDS.task,
      twin_context: {
        task_id: IDS.task,
        attribution: {
          twin_version_id: IDS.version,
          twin_version_number: 1,
          twin_version_digest: "bad",
        },
      },
    });

    expect(parsed.id).toBe(IDS.task);
    expect(parsed.twin_context?.taskId).toBe(IDS.task);
    expect(parsed.twin_context?.attribution).toBeUndefined();
  });
});

describe("Twin execution ApiClient", () => {
  it("serializes the frozen binding and preview request contracts", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        id: IDS.binding,
        scope_type: "workspace",
        scope_id: IDS.workspace,
        state: "enabled",
        twin_version_id: IDS.version,
      }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        policy: {
          state: "enabled",
          scope_type: "workspace",
          scope_id: IDS.workspace,
          binding_id: IDS.binding,
          explicit: true,
          reason: "explicit_binding",
        },
        twin_version: {
          id: IDS.version,
          version_number: 2,
          content_digest: DIGEST_A,
        },
        briefing: "Bounded briefing",
        briefing_digest: DIGEST_B,
        compiler_version: "twin-briefing/v1",
        byte_count: 16,
        token_count: 4,
        inject: true,
      }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.upsertTwinBinding({
      scopeType: "workspace",
      scopeId: IDS.workspace,
      state: "enabled",
      twinVersionId: IDS.version,
    });
    const preview = await client.previewTwinBriefing({
      agentId: IDS.binding,
      issueId: IDS.task,
      request: "Review this change",
      tags: ["review"],
      oneOffState: "preview",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://api.example.test/api/twins/bindings");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      scope_type: "workspace",
      scope_id: IDS.workspace,
      state: "enabled",
      twin_version_id: IDS.version,
    });
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({
      agent_id: IDS.binding,
      issue_id: IDS.task,
      request: "Review this change",
      tags: ["review"],
      one_off_state: "preview",
    });
    expect(preview).toMatchObject({ tokenCount: 4, briefingDigest: DIGEST_B, inject: true });
  });
});
