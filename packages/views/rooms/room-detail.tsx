"use client";

import { Loader2, Pause, Play, RotateCw } from "lucide-react";
import type { Agent } from "@multica/core/types";
import type {
  RoomComposerDraft,
  RoomDetail as RoomDetailModel,
} from "@multica/core/rooms";
import { createSafeId } from "@multica/core/utils";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { countTodayTurns, roomStatusClass } from "./room-display";
import { RoomTranscript } from "./room-transcript";
import { RoomInspector } from "./room-inspector";
import type { PromotionSource } from "./promote-room-dialog";

interface RoomDetailProps {
  readonly detail: RoomDetailModel;
  readonly agents: readonly Agent[];
  readonly draft: RoomComposerDraft;
  readonly onDraftBodyChange: (body: string) => void;
  readonly onDraftMentionChange: (agentId: string, selected: boolean) => void;
  readonly waking: boolean;
  readonly statusPending: boolean;
  readonly onPost: React.ComponentProps<typeof RoomTranscript>["onPost"];
  readonly onWake: (input: { readonly idempotency_key: string }) => void;
  readonly onStatus: (status: "active" | "paused") => void;
  readonly onPromote: (source: PromotionSource) => void;
}

export function RoomDetail({
  detail,
  agents,
  draft,
  onDraftBodyChange,
  onDraftMentionChange,
  waking,
  statusPending,
  onPost,
  onWake,
  onStatus,
  onPromote,
}: RoomDetailProps) {
  const { t } = useT("rooms");
  const room = detail.room;
  const activeCycle = room.active_cycle_id !== null;
  const budgetExhausted =
    room.daily_turn_limit !== null &&
    countTodayTurns(detail, new Date()) >= room.daily_turn_limit;
  const canWake =
    room.status === "active" && !activeCycle && !budgetExhausted && !waking;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex min-h-12 shrink-0 flex-wrap items-center gap-2 border-b border-surface-border bg-surface px-3 py-2 sm:px-4">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h1 className="truncate text-body font-medium text-foreground">{room.title}</h1>
            <Badge variant="secondary" className={cn("border-0", roomStatusClass(room.status))}>
              {t(($) => $.status[room.status])}
            </Badge>
          </div>
          {room.instructions ? (
            <p className="mt-0.5 truncate text-caption text-muted-foreground">{room.instructions}</p>
          ) : null}
        </div>
        {room.status === "active" || room.status === "paused" ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={statusPending}
            aria-label={room.status === "paused" ? t(($) => $.actions.resume) : t(($) => $.actions.pause)}
            data-testid="room-status-toggle"
            onClick={() => onStatus(room.status === "paused" ? "active" : "paused")}
          >
            {statusPending ? (
              <Loader2 className="animate-spin" aria-hidden="true" />
            ) : room.status === "paused" ? (
              <Play aria-hidden="true" />
            ) : (
              <Pause aria-hidden="true" />
            )}
            <span className="hidden sm:inline">
              {room.status === "paused" ? t(($) => $.actions.resume) : t(($) => $.actions.pause)}
            </span>
          </Button>
        ) : null}
        <Tooltip>
          <TooltipTrigger
            render={
              <span className="inline-flex">
                <Button
                  type="button"
                  size="sm"
                  disabled={!canWake}
                  aria-label={waking ? t(($) => $.actions.waking) : t(($) => $.actions.wake)}
                  data-testid="room-wake"
                  onClick={() => onWake({ idempotency_key: createSafeId() })}
                >
                  {waking ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCw aria-hidden="true" />}
                  <span className="hidden sm:inline">{t(($) => $.actions.wake)}</span>
                </Button>
              </span>
            }
          />
          {!canWake && !waking ? (
            <TooltipContent>
              {room.status === "paused"
                ? t(($) => $.refusal.room_paused)
                : room.status === "archived"
                  ? t(($) => $.refusal.room_archived)
                  : budgetExhausted
                    ? t(($) => $.refusal.budget_exhausted)
                    : t(($) => $.refusal.cycle_active)}
            </TooltipContent>
          ) : null}
        </Tooltip>
      </header>

      <div className="grid min-h-0 flex-1 grid-rows-[minmax(28rem,1fr)_auto] overflow-y-auto lg:grid-cols-[minmax(0,1fr)_18rem] lg:grid-rows-1 lg:overflow-hidden">
        <RoomTranscript
          detail={detail}
          agents={agents}
          draft={draft}
          onBodyChange={onDraftBodyChange}
          onMentionChange={onDraftMentionChange}
          onPost={onPost}
          onPromoteEntry={(entryId, title) =>
            onPromote({ entryId, suggestedTitle: title })
          }
        />
        <RoomInspector
          detail={detail}
          onPromoteCycle={(cycleId, title) =>
            onPromote({ cycleId, suggestedTitle: title })
          }
        />
      </div>
    </div>
  );
}
