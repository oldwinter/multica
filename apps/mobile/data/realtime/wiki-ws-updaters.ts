import type { QueryClient } from "@tanstack/react-query";
import { wikiKeys } from "@/data/queries/wiki";

const knownWikiRealtimeEvents = new Set([
  "wiki:page_created",
  "wiki:page_updated",
  "wiki:page_deleted",
  "wiki:revision_created",
  "wiki:revision_restored",
  "wiki:proposal_created",
  "wiki:proposal_reviewed",
]);

export interface WikiPageInvalidationTargets {
  detail: boolean;
  revisions: boolean;
  proposals: boolean;
}

export function invalidateWikiTreeForUnknownEvent(
  qc: QueryClient,
  wsId: string,
  eventType: string,
) {
  if (!eventType.startsWith("wiki:") || knownWikiRealtimeEvents.has(eventType)) {
    return;
  }
  return qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
}

export function wikiProposalReviewTargets(
  status: unknown,
): WikiPageInvalidationTargets {
  switch (status) {
    case "accepted":
      return { detail: true, revisions: true, proposals: true };
    case "rejected":
      return { detail: false, revisions: false, proposals: true };
    default:
      // Unknown outcomes may carry the same page/revision effects as an
      // accepted proposal. Refetch all page projections until this client
      // understands the new server enum.
      return { detail: true, revisions: true, proposals: true };
  }
}

export function invalidateWikiCollections(qc: QueryClient, wsId: string) {
  return qc.invalidateQueries({
    predicate: (query) => {
      const key = query.queryKey;
      return (
        key[0] === "wiki" &&
        key[1] === wsId &&
        (key[2] === "list" || key[2] === "search")
      );
    },
  });
}

export function invalidateWikiPage(
  qc: QueryClient,
  wsId: string,
  pageId: string,
  targets: WikiPageInvalidationTargets,
) {
  if (targets.detail) {
    qc.invalidateQueries({
      queryKey: wikiKeys.detail(wsId, pageId),
      exact: true,
    });
  }
  if (targets.revisions) {
    qc.invalidateQueries({
      queryKey: wikiKeys.revisions(wsId, pageId),
      exact: true,
    });
  }
  if (targets.proposals) {
    qc.invalidateQueries({
      queryKey: wikiKeys.proposals(wsId, pageId),
      exact: true,
    });
  }
}
