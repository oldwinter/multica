import { useQueryClient } from "@tanstack/react-query";
import { wikiKeys } from "@/data/queries/wiki";
import { useWSSubscriptions } from "@/lib/use-ws-subscriptions";
import {
  invalidateWikiPage,
  wikiProposalReviewTargets,
  type WikiPageInvalidationTargets,
} from "./wiki-ws-updaters";

export function useWikiPageRealtime(
  pageId: string | undefined,
  onDeleted?: () => void,
) {
  const qc = useQueryClient();

  useWSSubscriptions(
    (ws, wsId) => {
      if (!pageId) return;
      const invalidate = (targets: WikiPageInvalidationTargets) =>
        invalidateWikiPage(qc, wsId, pageId, targets);
      return [
        ws.on("wiki:page_updated", (payload) => {
          if (payload.page_id === pageId) {
            invalidate({ detail: true, revisions: false, proposals: false });
          }
        }),
        ws.on("wiki:page_deleted", (payload) => {
          if (payload.page_id !== pageId) return;
          qc.removeQueries({ queryKey: wikiKeys.detail(wsId, pageId) });
          onDeleted?.();
        }),
        ws.on("wiki:revision_created", (payload) => {
          if (payload.page_id === pageId) {
            invalidate({ detail: true, revisions: true, proposals: false });
          }
        }),
        ws.on("wiki:revision_restored", (payload) => {
          if (payload.page_id === pageId) {
            invalidate({ detail: true, revisions: true, proposals: false });
          }
        }),
        ws.on("wiki:proposal_created", (payload) => {
          if (payload.page_id === pageId) {
            invalidate({ detail: false, revisions: false, proposals: true });
          }
        }),
        ws.on("wiki:proposal_reviewed", (payload) => {
          if (payload.page_id !== pageId) return;
          invalidate(wikiProposalReviewTargets(payload.status));
        }),
        ws.onReconnect(() =>
          invalidate({ detail: true, revisions: true, proposals: true }),
        ),
      ];
    },
    [pageId, qc, onDeleted],
  );
}
