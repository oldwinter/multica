// @vitest-environment node

import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { wikiKeys as workspaceWikiKeys } from "../wiki/queries";
import { wikiKeys as lmWikiKeys } from "../twins/queries";
import {
  applyLMWikiRealtimeEvent,
  applyWikiRealtimeEvent,
} from "./use-realtime-sync";

const wsId = "ws-1";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
}

function seedWikiCollections(qc: QueryClient) {
  qc.setQueryData(workspaceWikiKeys.list(wsId, { scope: "workspace" }), []);
  qc.setQueryData(workspaceWikiKeys.search(wsId, { q: "guide" }), []);
}

describe("applyWikiRealtimeEvent", () => {
  it("invalidates readiness for every Wiki event that can change source health", () => {
    const events = [
      ["wiki:page_created", {}],
      ["wiki:page_updated", {}],
      ["wiki:page_deleted", {}],
      ["wiki:revision_created", {}],
      ["wiki:revision_restored", {}],
      ["wiki:proposal_reviewed", { status: "accepted" }],
    ] as const;

    for (const [eventType, extra] of events) {
      const qc = createQueryClient();
      const readinessKey = workspaceWikiKeys.readiness(wsId);
      qc.setQueryData(readinessKey, {});
      applyWikiRealtimeEvent(qc, wsId, eventType, { page_id: "page-1", ...extra });
      expect(qc.getQueryState(readinessKey)?.isInvalidated, eventType).toBe(true);
    }
  });

  it("invalidates collections for page creation without exposing page content", () => {
    const qc = createQueryClient();
    seedWikiCollections(qc);

    applyWikiRealtimeEvent(qc, wsId, "wiki:page_created", {
      page_id: "page-1",
      scope: "workspace",
      revision_id: "revision-1",
      revision_number: 1,
    });

    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(workspaceWikiKeys.search(wsId, { q: "guide" }))?.isInvalidated).toBe(true);
  });

  it("targets page, revision, and collection caches for a new revision", () => {
    const qc = createQueryClient();
    const detailKey = workspaceWikiKeys.detail(wsId, "page-1");
    const revisionsKey = workspaceWikiKeys.revisions(wsId, "page-1");
    const proposalsKey = workspaceWikiKeys.proposals(wsId, "page-1");
    seedWikiCollections(qc);
    qc.setQueryData(detailKey, {});
    qc.setQueryData(revisionsKey, []);
    qc.setQueryData(proposalsKey, []);

    applyWikiRealtimeEvent(qc, wsId, "wiki:revision_created", {
      page_id: "page-1",
      scope: "project",
      project_id: "project-1",
      revision_id: "revision-2",
      revision_number: 2,
    });

    expect(qc.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(revisionsKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(proposalsKey)?.isInvalidated).toBe(false);
    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
  });

  it("removes a deleted page subtree and invalidates collections", () => {
    const qc = createQueryClient();
    seedWikiCollections(qc);
    qc.setQueryData(workspaceWikiKeys.detail(wsId, "page-1"), {});
    qc.setQueryData(workspaceWikiKeys.revisions(wsId, "page-1"), []);
    qc.setQueryData(workspaceWikiKeys.proposals(wsId, "page-1"), []);

    applyWikiRealtimeEvent(qc, wsId, "wiki:page_deleted", {
      page_id: "page-1",
      scope: "workspace",
    });

    expect(qc.getQueryData(workspaceWikiKeys.detail(wsId, "page-1"))).toBeUndefined();
    expect(qc.getQueryData(workspaceWikiKeys.revisions(wsId, "page-1"))).toBeUndefined();
    expect(qc.getQueryData(workspaceWikiKeys.proposals(wsId, "page-1"))).toBeUndefined();
    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
  });

  it("keeps rejected proposal invalidation narrower than accepted review", () => {
    const rejected = createQueryClient();
    const accepted = createQueryClient();
    for (const qc of [rejected, accepted]) {
      seedWikiCollections(qc);
      qc.setQueryData(workspaceWikiKeys.detail(wsId, "page-1"), {});
      qc.setQueryData(workspaceWikiKeys.revisions(wsId, "page-1"), []);
      qc.setQueryData(workspaceWikiKeys.proposals(wsId, "page-1"), []);
    }

    applyWikiRealtimeEvent(rejected, wsId, "wiki:proposal_reviewed", {
      page_id: "page-1",
      scope: "workspace",
      proposal_id: "proposal-1",
      status: "rejected",
    });
    applyWikiRealtimeEvent(accepted, wsId, "wiki:proposal_reviewed", {
      page_id: "page-1",
      scope: "workspace",
      proposal_id: "proposal-1",
      status: "accepted",
      accepted_revision_id: "revision-2",
      accepted_revision_number: 2,
    });

    expect(rejected.getQueryState(workspaceWikiKeys.proposals(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(rejected.getQueryState(workspaceWikiKeys.detail(wsId, "page-1"))?.isInvalidated).toBe(false);
    expect(accepted.getQueryState(workspaceWikiKeys.proposals(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(accepted.getQueryState(workspaceWikiKeys.detail(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(accepted.getQueryState(workspaceWikiKeys.revisions(wsId, "page-1"))?.isInvalidated).toBe(true);
  });

  it("invalidates the Wiki tree for an unknown proposal review state", () => {
    const qc = createQueryClient();
    seedWikiCollections(qc);
    qc.setQueryData(workspaceWikiKeys.detail(wsId, "page-1"), {});
    qc.setQueryData(workspaceWikiKeys.revisions(wsId, "page-1"), []);
    qc.setQueryData(workspaceWikiKeys.proposals(wsId, "page-1"), []);

    applyWikiRealtimeEvent(qc, wsId, "wiki:proposal_reviewed", {
      page_id: "page-1",
      scope: "workspace",
      proposal_id: "proposal-1",
      status: "superseded",
    });

    expect(qc.getQueryState(workspaceWikiKeys.detail(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(workspaceWikiKeys.revisions(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(workspaceWikiKeys.proposals(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
  });

  it("invalidates the Wiki tree for an unknown lifecycle event", () => {
    const qc = createQueryClient();
    seedWikiCollections(qc);
    qc.setQueryData(workspaceWikiKeys.detail(wsId, "page-1"), {});

    applyWikiRealtimeEvent(
      qc,
      wsId,
      "wiki:page_published" as never,
      { page_id: "page-1", scope: "workspace" },
    );

    expect(qc.getQueryState(workspaceWikiKeys.detail(wsId, "page-1"))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
  });

  it("falls back to invalidating the Wiki tree for a malformed known event", () => {
    const qc = createQueryClient();
    seedWikiCollections(qc);

    applyWikiRealtimeEvent(qc, wsId, "wiki:page_updated", {});

    expect(qc.getQueryState(workspaceWikiKeys.list(wsId, { scope: "workspace" }))?.isInvalidated).toBe(true);
  });
});

describe("applyLMWikiRealtimeEvent", () => {
  it("targets the overview and changed revision without invalidating Twin caches", () => {
    const qc = createQueryClient();
    const overviewKey = lmWikiKeys.overview(wsId);
    const revisionKey = lmWikiKeys.revision(wsId, "revision-2");
    const readinessKey = workspaceWikiKeys.readiness(wsId);
    qc.setQueryData(overviewKey, {});
    qc.setQueryData(revisionKey, {});
    qc.setQueryData(readinessKey, {});

    applyLMWikiRealtimeEvent(qc, wsId, "lm_wiki:review_changed", {
      revision_id: "revision-2",
      decision: "accepted",
    });

    expect(qc.getQueryState(overviewKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(revisionKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(readinessKey)?.isInvalidated).toBe(true);
  });

  it("invalidates the LM Wiki tree when source policy changes", () => {
    const qc = createQueryClient();
    const sourcePolicyKey = workspaceWikiKeys.sourcePolicy(wsId);
    const overviewKey = lmWikiKeys.overview(wsId);
    const signedRevisionKey = lmWikiKeys.revision(wsId, "signed-revision");
    const readinessKey = workspaceWikiKeys.readiness(wsId);
    qc.setQueryData(sourcePolicyKey, {});
    qc.setQueryData(overviewKey, {});
    qc.setQueryData(signedRevisionKey, {});
    qc.setQueryData(readinessKey, {});

    applyLMWikiRealtimeEvent(qc, wsId, "lm_wiki:source_policy_changed", {
      policy_version: 2,
    });

    expect(qc.getQueryState(sourcePolicyKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(overviewKey)?.isInvalidated).toBe(true);
    expect(qc.getQueryState(signedRevisionKey)?.isInvalidated).toBe(false);
    expect(qc.getQueryState(readinessKey)?.isInvalidated).toBe(true);
  });
});
