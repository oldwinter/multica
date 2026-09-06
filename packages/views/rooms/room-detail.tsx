"use client";

import { Archive, Copy, Gauge, Link2, Loader2, Pause, Play, RotateCw, Square } from "lucide-react";
import { toast } from "sonner";
import type { Agent } from "@multica/core/types";
import type {
  RoomComposerDraft,
  RoomDetail as RoomDetailModel,
  RoomDetailTab,
  RoomOutcomeState,
  RoomPreflight,
  RoomRecommendation,
  RoomSynthesis,
} from "@multica/core/rooms";
import { createSafeId } from "@multica/core/utils";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { Tabs, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { copyText } from "@multica/ui/lib/clipboard";
import { currentPath, useOptionalNavigation } from "../navigation";
import { roomStatusClass } from "./room-display";
import { RoomTranscript } from "./room-transcript";
import { RoomInspector } from "./room-inspector";
import { RoomOutcome, type RoomAttentionTarget } from "./room-outcome";
import type { PromotionSource } from "./promote-room-dialog";

interface RoomDetailProps {
  readonly detail: RoomDetailModel;
  readonly detailTab: RoomDetailTab;
  readonly agents: readonly Agent[];
  readonly draft: RoomComposerDraft;
  readonly outcomeState: RoomOutcomeState;
  readonly preflight: RoomPreflight | undefined;
  readonly scheduledPreflight: RoomPreflight | undefined;
  readonly onDraftBodyChange: (body: string) => void;
  readonly onDraftMentionChange: (agentId: string, selected: boolean) => void;
  readonly onDetailTabChange: (tab: RoomDetailTab) => void;
  readonly waking: boolean;
  readonly preflightPending: boolean;
  readonly preflightError: boolean;
  readonly statusPending: boolean;
  readonly reviewPending: boolean;
  readonly retryPending: boolean;
  readonly cancelPending: boolean;
  readonly recommendationPending: boolean;
	readonly attentionTarget?: RoomAttentionTarget;
	readonly canManageBudget: boolean;
  readonly onPost: React.ComponentProps<typeof RoomTranscript>["onPost"];
  readonly onWake: (input: { readonly idempotency_key: string }) => void;
  readonly onRetryPreflight: () => void;
  readonly onStatus: (status: "active" | "paused" | "archived") => void;
  readonly onReview: (action: "accept" | "reject" | "correct", correction?: RoomSynthesis) => void;
  readonly onRetrySynthesis: () => void;
  readonly onCancelCycle: () => void;
  readonly onRejectRecommendation: (revisionId: string, recommendationKey: string) => void;
  readonly onPromote: (source: PromotionSource) => void;
	readonly onDuplicate: () => void;
	readonly onManageBudget: () => void;
}

export function RoomDetail({
  detail,
  detailTab,
  agents,
  draft,
  outcomeState,
  preflight,
  scheduledPreflight,
  onDraftBodyChange,
  onDraftMentionChange,
  onDetailTabChange,
  waking,
  preflightPending,
  preflightError,
  statusPending,
  reviewPending,
  retryPending,
  cancelPending,
  recommendationPending,
	attentionTarget,
	canManageBudget,
  onPost,
  onWake,
  onRetryPreflight,
  onStatus,
  onReview,
  onRetrySynthesis,
  onCancelCycle,
  onRejectRecommendation,
  onPromote,
	onDuplicate,
	onManageBudget,
}: RoomDetailProps) {
  const { t } = useT("rooms");
  const navigation = useOptionalNavigation();
  const room = detail.room;
  const canWake = outcomeState.nextAction === "run_cycle" && !waking;
  const repeatEligible =
    room.status === "active" &&
    outcomeState.activeCycle === null &&
    outcomeState.latestCycle !== null &&
    ["completed", "failed", "cancelled", "refused"].includes(outcomeState.phase);
  const canRunAgain = repeatEligible && !waking;
  const canRetryPreflight = preflightError && !preflightPending;
  const wakeBusy = waking || (!repeatEligible && preflightPending);

  const copyRoomLink = () => {
    if (!navigation) return;
    void copyText(navigation.getShareableUrl(currentPath(navigation))).then((ok) => {
      toast[ok ? "success" : "error"](t(($) => ok ? $.toast.link_copied : $.toast.link_copy_failed));
    });
  };

  const jumpToCitation = (entryId: string) => {
    onDetailTabChange("transcript");
    requestAnimationFrame(() => {
      const target = document.getElementById(`room-entry-${entryId}`);
      const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      target?.scrollIntoView({ block: "center", behavior: reduceMotion ? "auto" : "smooth" });
      target?.focus({ preventScroll: true });
    });
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex min-h-12 shrink-0 flex-wrap items-center gap-2 border-b border-surface-border bg-surface px-3 py-2 sm:px-4">
        <div className="min-w-0 flex-1 max-lg:basis-full">
          <div className="flex items-center gap-2">
            <h1 className="truncate text-body font-medium text-foreground">{room.title}</h1>
            <Badge variant="secondary" className={cn("border-0", roomStatusClass(room.status))}>
              {t(($) => $.status[room.status])}
            </Badge>
          </div>
          {outcomeState.objective ? (
            <p className="mt-0.5 truncate text-caption text-muted-foreground">{outcomeState.objective}</p>
          ) : null}
        </div>
        {room.status === "active" || room.status === "paused" ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="max-lg:min-h-11 max-lg:min-w-11"
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
        {outcomeState.canCancel ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="max-lg:min-h-11 max-lg:min-w-11"
            disabled={cancelPending}
            aria-label={t(($) => $.actions.cancel_cycle)}
            data-testid="room-cancel-cycle"
            onClick={onCancelCycle}
          >
            {cancelPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Square aria-hidden="true" />}
            <span className="hidden sm:inline">{t(($) => $.actions.cancel_cycle)}</span>
          </Button>
        ) : null}
		<Tooltip>
			<TooltipTrigger render={
				<Button
					type="button"
					size="icon-sm"
					variant="ghost"
					className="max-lg:size-11"
					aria-label={t(($) => $.actions.duplicate)}
					data-testid="room-duplicate"
					onClick={onDuplicate}
				>
					<Copy aria-hidden="true" />
				</Button>
			} />
			<TooltipContent>{t(($) => $.actions.duplicate)}</TooltipContent>
		</Tooltip>
		<Tooltip>
			<TooltipTrigger render={
				<Button
					type="button"
					size="icon-sm"
					variant="ghost"
					className="max-lg:size-11"
					disabled={!navigation}
					aria-label={t(($) => $.actions.copy_link)}
					data-testid="room-copy-link"
					onClick={copyRoomLink}
				>
					<Link2 aria-hidden="true" />
				</Button>
			} />
			<TooltipContent>{t(($) => $.actions.copy_link)}</TooltipContent>
		</Tooltip>
		{canManageBudget ? (
			<Tooltip>
				<TooltipTrigger render={
					<Button
						type="button"
						size="icon-sm"
						variant="ghost"
						className="max-lg:size-11"
						aria-label={t(($) => $.budget.manage)}
						data-testid="room-manage-budget"
						onClick={onManageBudget}
					>
						<Gauge aria-hidden="true" />
					</Button>
				} />
				<TooltipContent>{t(($) => $.budget.manage)}</TooltipContent>
			</Tooltip>
		) : null}
        <Tooltip>
          <TooltipTrigger
            render={
              <span className="inline-flex">
                <Button
                  type="button"
                  size="sm"
                  className="max-lg:min-h-11 max-lg:min-w-11"
                  disabled={!canRunAgain && !canWake && !canRetryPreflight}
                  aria-label={repeatEligible
                    ? t(($) => $.actions.run_again)
                    : canRetryPreflight
                      ? t(($) => $.actions.retry_preflight)
                      : wakeBusy
                      ? t(($) => $.actions.waking)
                      : t(($) => $.actions.wake)}
                  data-testid="room-wake"
                  onClick={() => {
                    if (canRunAgain) onWake({ idempotency_key: createSafeId() });
                    else if (canRetryPreflight) onRetryPreflight();
                    else onWake({ idempotency_key: createSafeId() });
                  }}
                >
                  {wakeBusy ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCw aria-hidden="true" />}
                  <span className="hidden sm:inline">
                    {repeatEligible
                      ? t(($) => $.actions.run_again)
                      : canRetryPreflight
                        ? t(($) => $.actions.retry_preflight)
                        : t(($) => $.actions.wake)}
                  </span>
                </Button>
              </span>
            }
          />
          {!canRunAgain && !canWake && !waking ? (
            <TooltipContent>
              {preflightError
                ? t(($) => $.refusal.preflight_failed)
                : room.status === "paused"
                ? t(($) => $.refusal.room_paused)
                : blockerCopy(t, outcomeState.blocker)}
            </TooltipContent>
          ) : null}
        </Tooltip>
        {room.status !== "archived" ? (
          <Tooltip>
            <TooltipTrigger render={
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                className="max-lg:size-11"
                disabled={statusPending || outcomeState.canCancel}
                aria-label={t(($) => $.actions.archive)}
                data-testid="room-archive"
                onClick={() => onStatus("archived")}
              >
                <Archive aria-hidden="true" />
              </Button>
            } />
            <TooltipContent>{t(($) => $.actions.archive)}</TooltipContent>
          </Tooltip>
        ) : null}
      </header>

      <Tabs
        value={detailTab}
        onValueChange={(value) => {
          if (value === "transcript" || value === "outcome" || value === "activity") {
            onDetailTabChange(value);
          }
        }}
        className="shrink-0 gap-0 border-b border-surface-border bg-surface px-3 py-1 2xl:hidden"
      >
        <TabsList
          variant="line"
          className="grid h-9 w-full grid-cols-3 max-lg:p-0 max-lg:group-data-horizontal/tabs:h-11"
        >
          <TabsTrigger className="max-lg:min-h-11" value="transcript">
            {t(($) => $.detail.transcript)}
          </TabsTrigger>
          <TabsTrigger className="max-lg:min-h-11" value="outcome">
            {t(($) => $.outcome.title)}
          </TabsTrigger>
          <TabsTrigger className="max-lg:min-h-11" value="activity">
            {t(($) => $.detail.activity)}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden 2xl:grid-cols-[minmax(22rem,1fr)_minmax(20rem,0.8fr)_18rem]">
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
          className={detailTab === "transcript" ? "flex" : "hidden 2xl:flex"}
        />
        <RoomOutcome
          detail={detail}
          state={outcomeState}
          className={cn("2xl:border-l", detailTab === "outcome" ? "block" : "hidden 2xl:block")}
          reviewPending={reviewPending}
          retryPending={retryPending}
          recommendationPending={recommendationPending}
          attentionTarget={attentionTarget}
          onReview={onReview}
          onRetry={onRetrySynthesis}
          onCitation={jumpToCitation}
          onPromoteRecommendation={(revisionId, recommendation: RoomRecommendation) =>
            onPromote({
              memoryRevisionId: revisionId,
              recommendationKey: recommendation.key,
              suggestedTitle: recommendation.title,
              suggestedBody: recommendation.body,
              suggestedRationale: recommendation.rationale,
              citationEntryIds: recommendation.citation_entry_ids,
              suggestedKind: recommendation.kind,
            })
          }
          onRejectRecommendation={onRejectRecommendation}
        />
        <RoomInspector
          detail={detail}
          usage={outcomeState.usage}
          preflight={preflight}
          scheduledPreflight={scheduledPreflight}
          className={cn("2xl:border-l", detailTab === "activity" ? "block" : "hidden 2xl:block")}
          onPromoteCycle={(cycleId, title) =>
            onPromote({ cycleId, suggestedTitle: title })
          }
        />
      </div>
    </div>
  );
}

function blockerCopy(t: ReturnType<typeof useT<"rooms">>["t"], blocker: string | null): string {
  switch (blocker) {
    case "room_paused": return t(($) => $.refusal.room_paused);
    case "room_archived": return t(($) => $.refusal.room_archived);
    case "budget_exhausted": return t(($) => $.refusal.budget_exhausted);
    case "active_cycle":
    case "cycle_active": return t(($) => $.refusal.cycle_active);
    case "agent_unavailable": return t(($) => $.refusal.agent_unavailable);
    case "daemon_capability_unavailable": return t(($) => $.refusal.daemon_capability_unavailable);
		case "spend_limit_unsupported": return t(($) => $.refusal.spend_limit_unsupported);
    case "invocation_not_allowed": return t(($) => $.refusal.invocation_not_allowed);
    default: return t(($) => $.refusal.preflight_required);
  }
}
