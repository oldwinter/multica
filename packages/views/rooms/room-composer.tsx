"use client";

import { useRef } from "react";
import { AtSign, Loader2, RotateCw, Send } from "lucide-react";
import type {
  PostRoomMessageInput,
  RoomComposerDraft,
  RoomParticipant,
  RoomStatus,
} from "@multica/core/rooms";
import type { Agent } from "@multica/core/types";
import { isImeComposing } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n/use-t";

interface RoomComposerProps {
  readonly roomStatus: RoomStatus;
  readonly participants: readonly RoomParticipant[];
  readonly agents: readonly Agent[];
  readonly draft: RoomComposerDraft;
  readonly showStarters: boolean;
  readonly onBodyChange: (body: string) => void;
  readonly onMentionChange: (agentId: string, selected: boolean) => void;
  readonly onPost: (input: PostRoomMessageInput) => void;
}

export function RoomComposer({
  roomStatus,
  participants,
  agents,
  draft,
  showStarters,
  onBodyChange,
  onMentionChange,
  onPost,
}: RoomComposerProps) {
  const { t } = useT("rooms");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const pending = draft.status === "pending";
  const failed = draft.status === "failed";
  const isArchived = roomStatus === "archived";
  const participantAgentIds = new Set<string>();
  for (const participant of participants) {
    if (participant.type === "agent") {
      participantAgentIds.add(participant.participant_id);
    }
  }
  const mentionableAgents = agents.filter(
    (agent) => participantAgentIds.has(agent.id) && !agent.archived_at,
  );
  const selectedMentionAgentIds = new Set(draft.mentionAgentIds);
  const canSubmit = draft.body.trim().length > 0 && !pending && !isArchived;
  const starters = [
    ["unblock", t(($) => $.composer.starters.unblock)],
    ["plan", t(($) => $.composer.starters.plan)],
    ["challenge", t(($) => $.composer.starters.challenge)],
  ] as const;

  const submit = () => {
    if (!canSubmit) return;
    onPost({
      body: draft.body.trim(),
      mention_agent_ids:
        draft.mentionAgentIds.length > 0
          ? draft.mentionAgentIds
          : undefined,
      idempotency_key: draft.idempotencyKey,
    });
  };

  return (
    <div className="shrink-0 border-t border-surface-border bg-surface px-3 py-3">
      {roomStatus === "paused" ? (
        <p className="mb-2 text-caption text-warning" role="status">
          {t(($) => $.composer.paused_notice)}
        </p>
      ) : null}
      {isArchived ? (
        <p className="mb-2 text-caption text-muted-foreground" role="status">
          {t(($) => $.composer.archived_notice)}
        </p>
      ) : null}
      {failed ? (
        <p className="mb-2 flex items-center gap-1.5 text-caption text-destructive" role="alert">
          <RotateCw className="size-3.5" aria-hidden="true" />
          {t(($) => $.toast.message_failed)}
        </p>
      ) : null}
      {showStarters && !pending && !isArchived ? (
        <div
          className="mx-auto mb-2 w-full max-w-3xl"
          role="group"
          aria-label={t(($) => $.composer.starters_label)}
        >
          <p className="mb-1.5 text-caption text-muted-foreground">
            {t(($) => $.composer.starters_label)}
          </p>
          <div className="grid gap-2 sm:grid-cols-3">
            {starters.map(([key, text]) => (
              <Button
                key={key}
                type="button"
                size="sm"
                variant="outline"
                className="h-auto min-h-11 justify-start whitespace-normal px-3 py-2 text-left font-normal"
                data-testid={`room-starter-${key}`}
                onClick={() => {
                  onBodyChange(text);
                  textareaRef.current?.focus();
                }}
              >
                {text}
              </Button>
            ))}
          </div>
        </div>
      ) : null}
      <div className="mx-auto flex w-full max-w-3xl items-end gap-2 rounded-lg border border-input bg-page-canvas p-2 shadow-[var(--surface-shadow)] focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30">
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                type="button"
                size="icon-sm"
                variant="ghost"
                className="relative overflow-visible"
                aria-label={
                  draft.mentionAgentIds.length > 0
                    ? `${t(($) => $.composer.mentions)}, ${t(($) => $.composer.mentions_selected, {
                        count: draft.mentionAgentIds.length,
                      })}`
                    : t(($) => $.composer.mentions)
                }
                disabled={pending || isArchived}
              />
            }
          >
            <AtSign aria-hidden="true" />
            {draft.mentionAgentIds.length > 0 ? (
              <span
                data-testid="room-mention-count"
                className="absolute -end-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-brand px-1 text-caption leading-none text-brand-foreground sm:hidden"
                aria-hidden="true"
              >
                {draft.mentionAgentIds.length}
              </span>
            ) : null}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" side="top" className="w-60">
            {mentionableAgents.map((agent) => (
              <DropdownMenuCheckboxItem
                key={agent.id}
                checked={selectedMentionAgentIds.has(agent.id)}
                onCheckedChange={(checked) => {
                  onMentionChange(agent.id, checked);
                }}
              >
                <ActorAvatar actorType="agent" actorId={agent.id} size="xs" profileLink={false} />
                <span className="truncate">{agent.name}</span>
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        <Textarea
          ref={textareaRef}
          value={draft.body}
          disabled={pending || isArchived}
          rows={1}
          className="max-h-36 min-h-8 flex-1 resize-none border-0 bg-transparent px-1 py-1.5 shadow-none focus-visible:ring-0"
          placeholder={t(($) => $.composer.placeholder)}
          aria-label={t(($) => $.composer.placeholder)}
          data-testid="room-message-input"
          onChange={(event) => onBodyChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey || isImeComposing(event)) return;
            event.preventDefault();
            submit();
          }}
        />
        {draft.mentionAgentIds.length > 0 ? (
          <span className="hidden shrink-0 text-caption text-muted-foreground sm:inline">
            {t(($) => $.composer.mentions_selected, {
              count: draft.mentionAgentIds.length,
            })}
          </span>
        ) : null}
        <Button
          type="button"
          size="icon-sm"
          disabled={!canSubmit}
          aria-label={
            pending
              ? t(($) => $.actions.posting)
              : failed
                ? t(($) => $.actions.retry)
                : t(($) => $.actions.post)
          }
          data-testid="room-message-send"
          onClick={submit}
        >
          {pending ? (
            <Loader2 className="animate-spin" aria-hidden="true" />
          ) : failed ? (
            <RotateCw aria-hidden="true" />
          ) : (
            <Send aria-hidden="true" />
          )}
        </Button>
      </div>
    </div>
  );
}
