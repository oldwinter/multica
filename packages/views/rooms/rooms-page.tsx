"use client";

import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, Loader2, MessageSquareText } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import { agentListOptions, memberListOptions, squadListOptions } from "@multica/core/workspace/queries";
import {
  roomDetailOptions,
  roomListOptions,
  useCreateRoom,
  usePostRoomMessage,
  usePromoteRoomArtifact,
  useSetRoomStatus,
  useWakeRoom,
  type CreateRoomInput,
  type PromoteRoomArtifactInput,
  type Room,
  type RoomArtifact,
  type RoomDetail,
} from "@multica/core/rooms";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { CreateRoomDialog } from "./create-room-dialog";
import { PromoteRoomDialog, type PromotionSource } from "./promote-room-dialog";
import { RoomDetail as RoomDetailView } from "./room-detail";
import { RoomList } from "./room-list";
import { useRoomComposerDrafts } from "./use-room-composer-drafts";

const EMPTY_ROOMS: readonly Room[] = [];
const EMPTY_AGENTS: readonly Agent[] = [];
const EMPTY_MEMBERS: readonly MemberWithUser[] = [];
const EMPTY_SQUADS: readonly Squad[] = [];

export function RoomsPage() {
  const { t } = useT("rooms");
  const workspaceId = useWorkspaceId();
  const [selectedRoomId, setSelectedRoomId] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [promotionSource, setPromotionSource] = useState<PromotionSource | null>(null);

  const roomsQuery = useQuery(roomListOptions(workspaceId));
  const agentsQuery = useQuery(agentListOptions(workspaceId));
  const membersQuery = useQuery(memberListOptions(workspaceId));
  const squadsQuery = useQuery(squadListOptions(workspaceId));
  const rooms = roomsQuery.data ?? EMPTY_ROOMS;
  const activeRoomId = selectedRoomId || rooms[0]?.id || "";
  const detailQuery = useQuery(roomDetailOptions(workspaceId, activeRoomId));
  const agents = agentsQuery.data ?? EMPTY_AGENTS;
  const members = membersQuery.data ?? EMPTY_MEMBERS;
  const squads = squadsQuery.data ?? EMPTY_SQUADS;

  useEffect(() => {
    if (selectedRoomId && rooms.some((room) => room.id === selectedRoomId)) return;
    setSelectedRoomId(rooms[0]?.id ?? "");
  }, [rooms, selectedRoomId]);

  const createRoom = useCreateRoom();
  const postMessage = usePostRoomMessage(activeRoomId);
  const wakeRoom = useWakeRoom(activeRoomId);
  const setStatus = useSetRoomStatus(activeRoomId);
  const promote = usePromoteRoomArtifact(activeRoomId);

  const directoryLoading =
    agentsQuery.isPending || membersQuery.isPending || squadsQuery.isPending;
  const detail = detailQuery.data;
  const composer = useRoomComposerDrafts(activeRoomId);
  const latestError = useMemo(
    () => [roomsQuery.error, detailQuery.error].find((error) => error instanceof Error),
    [detailQuery.error, roomsQuery.error],
  );

  const reportFailure = (fallback: string, error: Error | null) => {
    toast.error(error?.message || fallback);
  };

  const create = (
    input: CreateRoomInput,
    onSuccess: (detail: RoomDetail) => void,
  ) => {
    createRoom.mutate(input, {
      onSuccess: (created) => {
        setSelectedRoomId(created.room.id);
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
    <main
      data-room-workspace
      className="pe-chat-launcher flex min-h-0 flex-1 overflow-hidden bg-page-canvas"
    >
      <div className="grid min-h-0 w-full grid-rows-[auto_minmax(0,1fr)] lg:grid-cols-[16rem_minmax(0,1fr)] lg:grid-rows-1">
        <RoomList
          rooms={rooms}
          selectedId={activeRoomId}
          loading={roomsQuery.isPending}
          onSelect={setSelectedRoomId}
          onCreate={() => setCreateOpen(true)}
        />

        {roomsQuery.isError ? (
          <WorkspaceState
            icon={AlertTriangle}
            title={t(($) => $.states.error_title)}
            description={latestError?.message || t(($) => $.states.error_description)}
            actionLabel={t(($) => $.actions.retry)}
            onAction={() => roomsQuery.refetch()}
          />
        ) : !activeRoomId ? (
          <WorkspaceState
            icon={MessageSquareText}
            title={t(($) => $.states.no_room_title)}
            description={t(($) => $.states.no_room_description)}
            actionLabel={t(($) => $.actions.new_room)}
            onAction={() => setCreateOpen(true)}
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
        ) : (
          <div
            data-testid="room-detail"
            data-room-id={detail.room.id}
            className="flex min-h-0 min-w-0"
          >
            <RoomDetailView
              detail={detail}
              agents={agents}
              draft={composer.draft}
              onDraftBodyChange={(body) => composer.updateBody(activeRoomId, body)}
              onDraftMentionChange={(agentId, selected) =>
                composer.updateMention(activeRoomId, agentId, selected)
              }
              waking={wakeRoom.isPending}
              statusPending={setStatus.isPending}
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
                    if (error instanceof ApiError && error.status === 409) {
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
            />
          </div>
        )}
      </div>

      <CreateRoomDialog
        open={createOpen}
        agents={agents}
        squads={squads}
        members={members}
        pending={createRoom.isPending || directoryLoading}
        onOpenChange={setCreateOpen}
        onCreate={create}
      />
      <PromoteRoomDialog
        source={promotionSource}
        pending={promote.isPending}
        onOpenChange={(open) => !open && setPromotionSource(null)}
        onPromote={promoteArtifact}
      />
    </main>
  );
}

function WorkspaceState({
  icon: Icon,
  iconClassName,
  title,
  description,
  actionLabel,
  onAction,
}: {
  readonly icon: typeof AlertTriangle;
  readonly iconClassName?: string;
  readonly title: string;
  readonly description?: string;
  readonly actionLabel?: string;
  readonly onAction?: () => void;
}) {
  return (
    <section className="flex min-h-0 items-center justify-center px-6 py-12 text-center">
      <div className="max-w-sm">
        <Icon className={`mx-auto size-6 text-muted-foreground ${iconClassName ?? ""}`} aria-hidden="true" />
        <h1 className="mt-3 text-title font-medium text-foreground">{title}</h1>
        {description ? <p className="mt-1 text-body text-muted-foreground">{description}</p> : null}
        {actionLabel && onAction ? (
          <Button type="button" size="sm" variant="outline" className="mt-4" onClick={onAction}>
            {actionLabel}
          </Button>
        ) : null}
      </div>
    </section>
  );
}
