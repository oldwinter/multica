"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowDown, Check, CircleX, Copy, Sparkles } from "lucide-react";
import { toast } from "sonner";
import type { Agent } from "@multica/core/types";
import type {
  PostRoomMessageInput,
  RoomComposerDraft,
  RoomDetail,
} from "@multica/core/rooms";
import { useActorName } from "@multica/core/workspace/hooks";
import { Markdown } from "@multica/ui/markdown";
import { Button } from "@multica/ui/components/ui/button";
import { copyText } from "@multica/ui/lib/clipboard";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../i18n";
import { ActorAvatar } from "../common/actor-avatar";
import { RoomComposer } from "./room-composer";
import { useRoomTranscriptScroll } from "./use-room-transcript-scroll";

interface RoomTranscriptProps {
  readonly detail: RoomDetail;
  readonly agents: readonly Agent[];
  readonly draft: RoomComposerDraft;
  readonly onBodyChange: (body: string) => void;
  readonly onMentionChange: (agentId: string, selected: boolean) => void;
  readonly onPost: (input: PostRoomMessageInput) => void;
  readonly onPromoteEntry: (entryId: string, title: string) => void;
  readonly className?: string;
}

type CopyStatus = "idle" | "copied" | "failed";

interface RoomEntryCopyButtonProps {
  readonly entryId: string;
  readonly body: string;
  readonly copyLabel: string;
  readonly copiedLabel: string;
  readonly failedLabel: string;
}

function RoomEntryCopyButton({
  entryId,
  body,
  copyLabel,
  copiedLabel,
  failedLabel,
}: RoomEntryCopyButtonProps) {
  const [status, setStatus] = useState<CopyStatus>("idle");
  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (resetTimer.current) clearTimeout(resetTimer.current);
    },
    [],
  );

  const copyEntry = async () => {
    const ok = await copyText(body);

    const nextStatus = ok ? "copied" : "failed";
    setStatus(nextStatus);
    if (ok) {
      toast.success(copiedLabel);
    } else {
      toast.error(failedLabel);
    }

    if (resetTimer.current) clearTimeout(resetTimer.current);
    resetTimer.current = setTimeout(() => setStatus("idle"), 2_000);
  };

  const statusLabel =
    status === "copied" ? copiedLabel : status === "failed" ? failedLabel : copyLabel;

  return (
    <Button
      type="button"
      size="icon-xs"
      variant="ghost"
      className="size-11 sm:size-6"
      aria-label={statusLabel}
      title={statusLabel}
      data-testid={`room-copy-entry-${entryId}`}
      data-copy-status={status}
      onClick={() => void copyEntry()}
    >
      {status === "copied" ? (
        <Check className="text-success" aria-hidden="true" />
      ) : status === "failed" ? (
        <CircleX className="text-destructive" aria-hidden="true" />
      ) : (
        <Copy aria-hidden="true" />
      )}
    </Button>
  );
}

export function RoomTranscript({
  detail,
  agents,
  draft,
  onBodyChange,
  onMentionChange,
  onPost,
  onPromoteEntry,
  className,
}: RoomTranscriptProps) {
  const { t } = useT("rooms");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const { scrollRef, onScroll, unseenEntryCount, scrollToLatest } =
    useRoomTranscriptScroll(detail.room.id, detail.entries.length);

  return (
    <section className={cn("min-h-0 flex-1 flex-col bg-page-canvas", className)} aria-labelledby="room-transcript-heading">
      <div className="flex h-11 shrink-0 items-center justify-between border-b border-surface-border px-4">
        <h2 id="room-transcript-heading" className="text-body font-medium text-foreground">
          {t(($) => $.detail.transcript)}
        </h2>
        <span className="sr-only" aria-live="polite" aria-atomic="true">
          {unseenEntryCount > 0
            ? t(($) => $.actions.new_activity_announcement, { count: unseenEntryCount })
            : ""}
        </span>
        {unseenEntryCount > 0 ? (
          <Button
            type="button"
            size="xs"
            variant="brandSubtle"
            aria-label={t(($) => $.actions.show_new_activity, { count: unseenEntryCount })}
            data-testid="room-transcript-new-entries"
            onClick={scrollToLatest}
          >
            <ArrowDown data-icon="inline-start" aria-hidden="true" />
            {t(($) => $.actions.new_activity, { count: unseenEntryCount })}
          </Button>
        ) : (
          <span className="font-mono text-caption tabular-nums text-muted-foreground">
            {detail.entries.length}
          </span>
        )}
      </div>

      <div
        ref={scrollRef}
        className="min-h-0 flex-1 overflow-y-auto px-4 py-3"
        data-testid="room-transcript"
        onScroll={onScroll}
        tabIndex={-1}
      >
        {detail.entries.length === 0 ? (
          <div className="flex h-full min-h-40 items-center justify-center text-center">
            <p className="text-body text-muted-foreground">
              {t(($) => $.detail.empty_transcript)}
            </p>
          </div>
        ) : (
          <ol className="mx-auto w-full max-w-3xl space-y-1">
            {detail.entries.map((entry) => {
                const actorType = entry.author_type === "unknown" ? "system" : entry.author_type;
                const actorId = entry.author_id ?? "system";
                const actorName =
                  actorType === "system"
                    ? t(($) => $.detail.system)
                    : getActorName(actorType, actorId);
                const promotable = entry.type === "result";
                return (
                  <li
                    key={entry.id}
                    id={`room-entry-${entry.id}`}
                    tabIndex={-1}
                    data-testid={`room-entry-${entry.id}`}
                    className={cn(
                      "group flex gap-3 rounded-lg px-2 py-2.5 outline-none focus-visible:ring-2 focus-visible:ring-ring",
                      entry.type === "system" && "bg-muted/40",
                    )}
                  >
                    <ActorAvatar
                      actorType={actorType}
                      actorId={actorId}
                      size="md"
                      enableHoverCard={actorType === "agent" || actorType === "member"}
                      profileLink={actorType !== "system"}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex min-h-5 flex-wrap items-center gap-x-2 gap-y-1">
                        <span className="text-body font-medium text-foreground">{actorName}</span>
                        {entry.type === "result" ? (
                          <span className="inline-flex items-center gap-1 text-caption text-brand">
                            <Sparkles className="size-3" aria-hidden="true" />
                            {t(($) => $.cycle.source.agent)}
                          </span>
                        ) : null}
                        <time
                          dateTime={entry.created_at}
                          className="font-mono text-caption tabular-nums text-muted-foreground"
                        >
                          {timeAgo(entry.created_at)}
                        </time>
                        <span className="ml-auto flex shrink-0 items-center gap-1 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100">
                          <RoomEntryCopyButton
                            entryId={entry.id}
                            body={entry.body}
                            copyLabel={t(($) => $.actions.copy_entry)}
                            copiedLabel={t(($) => $.toast.entry_copied)}
                            failedLabel={t(($) => $.toast.copy_entry_failed)}
                          />
                          {promotable ? (
                            <Button
                              type="button"
                              size="xs"
                              variant="ghost"
                              data-testid={`room-promote-entry-${entry.id}`}
                              onClick={() => onPromoteEntry(entry.id, detail.room.title)}
                            >
                              <Sparkles data-icon="inline-start" aria-hidden="true" />
                              {t(($) => $.actions.promote)}
                            </Button>
                          ) : null}
                        </span>
                      </div>
                      <Markdown mode="minimal" className="mt-0.5 text-body leading-6 text-foreground">
                        {entry.body}
                      </Markdown>
                    </div>
                  </li>
                );
            })}
          </ol>
        )}
      </div>

      <RoomComposer
        roomStatus={detail.room.status}
        participants={detail.participants}
        agents={agents}
        draft={draft}
        showStarters={detail.entries.length === 0 && draft.body.length === 0}
        onBodyChange={onBodyChange}
        onMentionChange={onMentionChange}
        onPost={onPost}
      />
    </section>
  );
}
