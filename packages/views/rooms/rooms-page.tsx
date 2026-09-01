"use client";

import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, Loader2, MessageSquareText } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError, errorCode } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import { agentListOptions, memberListOptions, squadListOptions } from "@multica/core/workspace/queries";
import {
  roomDetailOptions,
  roomListOptions,
  roomPreflightOptions,
  roomUsageOptions,
  duplicateRoomConfiguration,
  deriveRoomOutcomeState,
  useCancelRoomCycle,
  useCreateRoom,
  usePostRoomMessage,
  usePromoteRoomArtifact,
  useRetryRoomSynthesis,
  useReviewRoomCycle,
  useReviewRoomRecommendation,
  useRoomViewStore,
  useSetRoomStatus,
  useUpdateRoomBudget,
  useWakeRoom,
  type CreateRoomInput,
  type PromoteRoomArtifactInput,
  type Room,
  type RoomArtifact,
  type RoomDetailTab,
  type RoomDetail,
  type RoomSynthesis,
} from "@multica/core/rooms";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { CreateRoomDialog } from "./create-room-dialog";
import { PromoteRoomDialog, type PromotionSource } from "./promote-room-dialog";
import { RoomBudgetDialog } from "./room-budget-dialog";
import { RoomDetail as RoomDetailView } from "./room-detail";
import { RoomList } from "./room-list";
import { useRoomComposerDrafts } from "./use-room-composer-drafts";
import { operationFingerprint, useIdempotencyRegistry } from "./idempotency";
import { selectRoomLifecycleCycleId } from "./room-controller";

const EMPTY_ROOMS: readonly Room[] = [];
const EMPTY_AGENTS: readonly Agent[] = [];
const EMPTY_MEMBERS: readonly MemberWithUser[] = [];
const EMPTY_SQUADS: readonly Squad[] = [];

export function isLinkedRoomMissing(
  linkedRoomId: string,
  rooms: readonly Pick<Room, "id">[],
  listLoaded: boolean,
): boolean {
  return Boolean(linkedRoomId) && listLoaded && !rooms.some((room) => room.id === linkedRoomId);
}

export function detailTabAfterRoomSelection(
  currentTab: RoomDetailTab,
  linkedRoomMissing: boolean,
): RoomDetailTab {
  return linkedRoomMissing ? "transcript" : currentTab;
}

export function RoomsPage({ rootElement = "main" }: { rootElement?: "main" | "div" } = {}) {
  const { t } = useT("rooms");
  const workspaceId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const nav = useNavigation();
  const linkedRoomId = nav.searchParams.get("room") ?? "";
  const [selectedRoomId, setSelectedRoomId] = useState(linkedRoomId);
  const [createOpen, setCreateOpen] = useState(false);
  const [duplicateInput, setDuplicateInput] = useState<CreateRoomInput | null>(null);
  const [promotionSource, setPromotionSource] = useState<PromotionSource | null>(null);
  const [budgetOpen, setBudgetOpen] = useState(false);
  const currentUser = useAuthStore((state) => state.user);
  const detailTab = useRoomViewStore((state) => state.detailTab);
  const setDetailTab = useRoomViewStore((state) => state.setDetailTab);
  const lifecycleIdempotency = useIdempotencyRegistry();

  const roomsQuery = useQuery(roomListOptions(workspaceId));
  const agentsQuery = useQuery(agentListOptions(workspaceId));
  const membersQuery = useQuery(memberListOptions(workspaceId));
  const squadsQuery = useQuery(squadListOptions(workspaceId));
  const rooms = roomsQuery.data ?? EMPTY_ROOMS;
  const roomsLoaded = roomsQuery.data !== undefined
    && !roomsQuery.isPending
    && !roomsQuery.isFetching
    && !roomsQuery.isError;
  const linkedRoomMissing = isLinkedRoomMissing(linkedRoomId, rooms, roomsLoaded);
  const activeRoomId = linkedRoomMissing ? "" : selectedRoomId || rooms[0]?.id || "";
  const mobileStandaloneList = roomsLoaded && rooms.length === 0;
  const detailQuery = useQuery(roomDetailOptions(workspaceId, activeRoomId));
  const preflightQuery = useQuery(roomPreflightOptions(workspaceId, activeRoomId));
  const scheduledPreflightQuery = useQuery({
    ...roomPreflightOptions(workspaceId, activeRoomId, undefined, "schedule"),
    enabled: Boolean(
      workspaceId &&
      activeRoomId &&
      detailQuery.data?.room.schedule_interval_minutes,
    ),
  });
  const usageQuery = useQuery(roomUsageOptions(workspaceId, activeRoomId));
  const detail = detailQuery.data;
  const lifecycleCycleId = selectRoomLifecycleCycleId(detail);
  const agents = agentsQuery.data ?? EMPTY_AGENTS;
  const members = membersQuery.data ?? EMPTY_MEMBERS;
  const squads = squadsQuery.data ?? EMPTY_SQUADS;
  const canManageBudget = useMemo(() => {
    const member = members.find((candidate) => candidate.user_id === currentUser?.id);
    return member?.role === "owner" || member?.role === "admin";
  }, [currentUser?.id, members]);

  useEffect(() => {
    if (linkedRoomId) {
      if (!roomsLoaded) return;
      if (rooms.some((room) => room.id === linkedRoomId)) setSelectedRoomId(linkedRoomId);
      return;
    }
    if (selectedRoomId && rooms.some((room) => room.id === selectedRoomId)) return;
    setSelectedRoomId(rooms[0]?.id ?? "");
  }, [linkedRoomId, rooms, roomsLoaded, selectedRoomId]);

  useEffect(() => {
    if (linkedRoomId && nav.searchParams.get("tab") === "outcome") {
      setDetailTab("outcome");
    }
  }, [linkedRoomId, nav.searchParams, setDetailTab]);

  const openCreate = () => {
    setDuplicateInput(null);
    setCreateOpen(true);
  };

  const selectRoom = (roomId: string) => {
    const nextDetailTab = detailTabAfterRoomSelection(detailTab, linkedRoomMissing);
    if (nextDetailTab !== detailTab) setDetailTab(nextDetailTab);
    setSelectedRoomId(roomId);
    nav.replace(paths.roomDetail(roomId));
  };

  const createRoom = useCreateRoom();
  const postMessage = usePostRoomMessage(activeRoomId);
  const wakeRoom = useWakeRoom(activeRoomId);
  const setStatus = useSetRoomStatus(activeRoomId);
  const updateBudget = useUpdateRoomBudget(activeRoomId);
  const promote = usePromoteRoomArtifact(activeRoomId);
  const retrySynthesis = useRetryRoomSynthesis(activeRoomId, lifecycleCycleId);
  const reviewCycle = useReviewRoomCycle(activeRoomId, lifecycleCycleId);
  const cancelCycle = useCancelRoomCycle(activeRoomId, lifecycleCycleId);
  const reviewRecommendation = useReviewRoomRecommendation(activeRoomId);

  const directoryLoading =
    agentsQuery.isPending || membersQuery.isPending || squadsQuery.isPending;
  const composer = useRoomComposerDrafts(activeRoomId);
  const latestError = useMemo(
    () => [roomsQuery.error, detailQuery.error].find((error) => error instanceof Error),
    [detailQuery.error, roomsQuery.error],
  );
  const outcomeState = detail
    ? deriveRoomOutcomeState(detail, {
        preflight: preflightQuery.data,
        preflightPending: preflightQuery.isPending,
        usage: usageQuery.data,
      })
    : null;

  const reportFailure = (fallback: string, error: Error | null) => {
    toast.error(error?.message || fallback);
  };

  const Root = rootElement;

  const create = (
    input: CreateRoomInput,
    onSuccess: (detail: RoomDetail) => void,
  ) => {
    createRoom.mutate(input, {
      onSuccess: (created) => {
        selectRoom(created.room.id);
        onSuccess(created);
        toast.success(t(($) => $.toast.created));
      },
      onError: (error) => reportFailure(t(($) => $.toast.create_failed), error),
    });
  };

  const promoteArtifact = (
    input: PromoteRoomArtifactInput,
    onSuccess: (artifact: RoomArtifact) => void,
  ) => {
    promote.mutate(input, {
      onSuccess: (artifact) => {
        onSuccess(artifact);
        toast.success(t(($) => $.toast.promoted));
      },
      onError: (error) => reportFailure(t(($) => $.toast.promote_failed), error),
    });
  };

  return (
    <Root
      data-room-workspace
      className="pe-chat-launcher flex min-h-0 flex-1 overflow-hidden bg-page-canvas"
    >
      <div className="grid min-h-0 w-full grid-rows-[auto_minmax(0,1fr)] lg:grid-cols-[16rem_minmax(0,1fr)] lg:grid-rows-1">
        <RoomList
          rooms={rooms}
          selectedId={activeRoomId}
          loading={roomsQuery.isPending}
          showValueReview={canManageBudget}
          mobileStandalone={mobileStandaloneList}
          onSelect={selectRoom}
          onCreate={openCreate}
        />

        {roomsQuery.isError ? (
          <WorkspaceState
            icon={AlertTriangle}
            title={t(($) => $.states.error_title)}
            description={latestError?.message || t(($) => $.states.error_description)}
            actionLabel={t(($) => $.actions.retry)}
            onAction={() => roomsQuery.refetch()}
          />
        ) : linkedRoomMissing ? (
          <WorkspaceState
            icon={AlertTriangle}
            title={t(($) => $.states.no_room_title)}
            description={t(($) => $.states.no_room_description)}
            actionLabel={t(($) => $.actions.new_room)}
            onAction={() => {
              setSelectedRoomId("");
              nav.replace(paths.rooms());
              openCreate();
            }}
          />
        ) : !activeRoomId ? (
          <WorkspaceState
            className={mobileStandaloneList ? "max-lg:hidden" : undefined}
            icon={MessageSquareText}
            title={t(($) => $.states.no_room_title)}
            description={t(($) => $.states.no_room_description)}
            actionLabel={t(($) => $.actions.new_room)}
            onAction={openCreate}
          />
        ) : detailQuery.isPending || !composer.draft ? (
          <WorkspaceState
            icon={Loader2}
            iconClassName="animate-spin"
            title={t(($) => $.states.detail_loading)}
          />
        ) : detailQuery.isError || !detail ? (
          <WorkspaceState
            icon={AlertTriangle}
            title={t(($) => $.states.error_title)}
            description={latestError?.message || t(($) => $.states.error_description)}
            actionLabel={t(($) => $.actions.retry)}
            onAction={() => detailQuery.refetch()}
          />
        ) : outcomeState ? (
          <div
            data-testid="room-detail"
            data-room-id={detail.room.id}
            className="flex min-h-0 min-w-0"
          >
            <RoomDetailView
              detail={detail}
              detailTab={detailTab}
              outcomeState={outcomeState}
              preflight={preflightQuery.data}
              scheduledPreflight={scheduledPreflightQuery.data}
              agents={agents}
              draft={composer.draft}
              onDraftBodyChange={(body) => composer.updateBody(activeRoomId, body)}
              onDraftMentionChange={(agentId, selected) =>
                composer.updateMention(activeRoomId, agentId, selected)
              }
              onDetailTabChange={setDetailTab}
              waking={wakeRoom.isPending}
              preflightPending={preflightQuery.isFetching}
              preflightError={preflightQuery.isError}
              statusPending={setStatus.isPending}
              reviewPending={reviewCycle.isPending}
              retryPending={retrySynthesis.isPending}
              cancelPending={cancelCycle.isPending}
              recommendationPending={reviewRecommendation.isPending || promote.isPending}
              attentionTarget={{
                focus: nav.searchParams.get("focus"),
                cycleId: nav.searchParams.get("cycle_id"),
                memoryRevisionId: nav.searchParams.get("memory_revision_id"),
                recommendationKey: nav.searchParams.get("recommendation_key"),
              }}
              canManageBudget={canManageBudget}
              onPost={(input) => {
                const submittedRoomId = activeRoomId;
                const submittedIdempotencyKey = input.idempotency_key;
                composer.markPending(submittedRoomId, submittedIdempotencyKey);
                void postMessage
                  .mutateAsync(input)
                  .then(() => {
                    composer.complete(submittedRoomId, submittedIdempotencyKey);
                    toast.success(t(($) => $.toast.message_posted));
                  })
                  .catch((error: Error) => {
                    if (
                      error instanceof ApiError &&
                      error.status === 409 &&
                      roomMessageWasPersisted(error)
                    ) {
                      composer.complete(submittedRoomId, submittedIdempotencyKey);
                      toast.warning(t(($) => $.toast.message_saved_no_execution));
                    } else {
                      composer.markFailed(submittedRoomId, submittedIdempotencyKey);
                      reportFailure(t(($) => $.toast.message_failed), error);
                    }
                  });
              }}
              onWake={(input) => {
                wakeRoom.mutate(input, {
                  onSuccess: () => toast.success(t(($) => $.toast.wake_queued)),
                  onError: (error) => reportFailure(t(($) => $.toast.wake_failed), error),
                });
              }}
              onRetryPreflight={() => {
                void preflightQuery.refetch();
              }}
              onReview={(action, correction?: RoomSynthesis) => {
                const fingerprint = operationFingerprint("review", {
                  roomId: activeRoomId,
                  cycleId: lifecycleCycleId,
                  action,
                  expectedMemoryVersion: detail.room.memory_version,
                  correction,
                });
                reviewCycle.mutate(
                  {
                    action,
                    expected_memory_version: detail.room.memory_version,
                    correction,
                    idempotency_key: lifecycleIdempotency.keyFor(fingerprint),
                  },
                  {
                    onSuccess: () => {
                      lifecycleIdempotency.complete(fingerprint);
                      toast.success(t(($) => $.toast.reviewed));
                    },
                    onError: (error) => reportRoomFailure(t(($) => $.toast.review_failed), error, t),
                  },
                );
              }}
              onRetrySynthesis={() => {
                const fingerprint = operationFingerprint("retry-synthesis", {
                  roomId: activeRoomId,
                  cycleId: lifecycleCycleId,
                });
                retrySynthesis.mutate(
                  { idempotency_key: lifecycleIdempotency.keyFor(fingerprint) },
                  {
                    onSuccess: () => {
                      lifecycleIdempotency.complete(fingerprint);
                      toast.success(t(($) => $.toast.synthesis_retry_queued));
                    },
                    onError: (error) => reportRoomFailure(t(($) => $.toast.synthesis_retry_failed), error, t),
                  },
                );
              }}
              onCancelCycle={() => {
                const fingerprint = operationFingerprint("cancel-cycle", {
                  roomId: activeRoomId,
                  cycleId: lifecycleCycleId,
                });
                cancelCycle.mutate(
                  { idempotency_key: lifecycleIdempotency.keyFor(fingerprint) },
                  {
                    onSuccess: () => {
                      lifecycleIdempotency.complete(fingerprint);
                      toast.success(t(($) => $.toast.cycle_cancelled));
                    },
                    onError: (error) => reportRoomFailure(t(($) => $.toast.cycle_cancel_failed), error, t),
                  },
                );
              }}
              onRejectRecommendation={(revisionId, recommendationKey) => {
                const fingerprint = operationFingerprint("reject-recommendation", {
                  roomId: activeRoomId,
                  revisionId,
                  recommendationKey,
                });
                reviewRecommendation.mutate(
                  {
                    revisionId,
                    recommendationKey,
                    input: {
                      action: "reject",
                      idempotency_key: lifecycleIdempotency.keyFor(fingerprint),
                    },
                  },
                  {
                    onSuccess: () => {
                      lifecycleIdempotency.complete(fingerprint);
                      toast.success(t(($) => $.toast.recommendation_rejected));
                    },
                    onError: (error) => reportRoomFailure(t(($) => $.toast.recommendation_review_failed), error, t),
                  },
                );
              }}
              onStatus={(status) => {
                setStatus.mutate(
                  { status },
                  {
                    onSuccess: () => toast.success(t(($) => $.toast.status_updated)),
                    onError: (error) => reportFailure(t(($) => $.toast.status_failed), error),
                  },
                );
              }}
              onPromote={setPromotionSource}
				onDuplicate={() => {
					setDuplicateInput(duplicateRoomConfiguration(detail));
					setCreateOpen(true);
				}}
              onManageBudget={() => setBudgetOpen(true)}
            />
          </div>
        ) : null}
      </div>

      <CreateRoomDialog
        open={createOpen}
        agents={agents}
        squads={squads}
        members={members}
        pending={createRoom.isPending || directoryLoading}
        initialInput={duplicateInput}
        mode={duplicateInput ? "duplicate" : "create"}
        onOpenChange={(open) => {
          setCreateOpen(open);
          if (!open) setDuplicateInput(null);
        }}
        onCreate={create}
      />
      <PromoteRoomDialog
        source={promotionSource}
        pending={promote.isPending}
        onOpenChange={(open) => !open && setPromotionSource(null)}
        onPromote={promoteArtifact}
      />
      {detail ? (
        <RoomBudgetDialog
          open={budgetOpen}
          room={detail.room}
          pending={updateBudget.isPending}
          onOpenChange={setBudgetOpen}
          onSave={(input) => {
            updateBudget.mutate(input, {
              onSuccess: () => {
                setBudgetOpen(false);
                toast.success(t(($) => $.toast.budget_updated));
              },
              onError: (error) => reportFailure(t(($) => $.toast.budget_failed), error),
            });
          }}
        />
      ) : null}
    </Root>
  );
}

type RoomsT = ReturnType<typeof useT<"rooms">>["t"];

function reportRoomFailure(fallback: string, error: Error | null, t: RoomsT): void {
  switch (errorCode(error)) {
    case "stale_review":
      toast.error(t(($) => $.errors.stale_review));
      return;
    case "idempotency_conflict":
      toast.error(t(($) => $.errors.idempotency_conflict));
      return;
    case "synthesis_not_retryable":
      toast.error(t(($) => $.errors.synthesis_not_retryable));
      return;
    case "recommendation_already_reviewed":
      toast.error(t(($) => $.errors.recommendation_already_reviewed));
      return;
    default:
      toast.error(error?.message || fallback);
  }
}

export function roomMessageWasPersisted(error: unknown): boolean {
  switch (errorCode(error)) {
    case "room_paused":
    case "room_archived":
    case "budget_exhausted":
    case "spend_limit_unsupported":
    case "active_cycle":
    case "agent_unavailable":
      return true;
    default:
      return false;
  }
}

function WorkspaceState({
  className,
  icon: Icon,
  iconClassName,
  title,
  description,
  actionLabel,
  onAction,
}: {
  readonly className?: string;
  readonly icon: typeof AlertTriangle;
  readonly iconClassName?: string;
  readonly title: string;
  readonly description?: string;
  readonly actionLabel?: string;
  readonly onAction?: () => void;
}) {
  return (
    <section className={`flex min-h-0 items-center justify-center px-6 py-12 text-center ${className ?? ""}`}>
      <div className="max-w-sm">
        <Icon className={`mx-auto size-6 text-muted-foreground ${iconClassName ?? ""}`} aria-hidden="true" />
        <h1 className="mt-3 text-title font-medium text-foreground">{title}</h1>
        {description ? <p className="mt-1 text-body text-muted-foreground">{description}</p> : null}
        {actionLabel && onAction ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="mt-4 max-md:min-h-11"
            onClick={onAction}
          >
            {actionLabel}
          </Button>
        ) : null}
      </div>
    </section>
  );
}
