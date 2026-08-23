import { useQueryClient } from "@tanstack/react-query";
import { useWSSubscriptions } from "@/lib/use-ws-subscriptions";
import {
  invalidateWikiCollections,
  invalidateWikiTreeForUnknownEvent,
} from "./wiki-ws-updaters";

/** Workspace-session Wiki subscription. Partial events cannot safely patch a
 * page summary, so only list/search projections are invalidated. Personal
 * events use the same contract but reach only the owning user's socket. */
export function useWikiRealtime() {
  const qc = useQueryClient();

  useWSSubscriptions(
    (ws, wsId) => {
      const invalidate = () => void invalidateWikiCollections(qc, wsId);
      return [
        ws.on("wiki:page_created", invalidate),
        ws.on("wiki:page_updated", invalidate),
        ws.on("wiki:page_deleted", invalidate),
        ws.on("wiki:revision_created", invalidate),
        ws.on("wiki:revision_restored", invalidate),
        ws.onAny((message) => {
          void invalidateWikiTreeForUnknownEvent(qc, wsId, message.type);
        }),
        ws.onReconnect(invalidate),
      ];
    },
    [qc],
  );
}
