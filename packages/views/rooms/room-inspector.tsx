"use client";

import {
  ArrowUpRight,
  Brain,
  CheckCircle2,
  CircleDot,
  Clock3,
  FileText,
  Sparkles,
  Users,
} from "lucide-react";
import type { RoomCycle, RoomDetail } from "@multica/core/rooms";
import { useWorkspacePaths } from "@multica/core/paths";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Progress } from "@multica/ui/components/ui/progress";
import { cn } from "@multica/ui/lib/utils";
import { useActorName } from "@multica/core/workspace/hooks";
import { useT, useTimeAgo } from "../i18n";
import { AppLink } from "../navigation";
import { ActorAvatar } from "../common/actor-avatar";
import {
  artifactHref,
  countTodayTurns,
  cycleStatusClass,
} from "./room-display";

interface RoomInspectorProps {
  readonly detail: RoomDetail;
  readonly onPromoteCycle: (cycleId: string, title: string) => void;
}

export function RoomInspector({ detail, onPromoteCycle }: RoomInspectorProps) {
  const { t } = useT("rooms");
  const paths = useWorkspacePaths();
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const usedTurns = countTodayTurns(detail, new Date());
  const dailyLimit = detail.room.daily_turn_limit;
  const budgetPercent = dailyLimit
    ? Math.min(100, Math.round((usedTurns / dailyLimit) * 100))
    : 0;

  return (
    <aside className="min-h-0 overflow-y-auto border-t border-surface-border bg-surface lg:border-t-0 lg:border-l">
      <InspectorSection icon={Users} title={t(($) => $.detail.participants)}>
        <ul className="space-y-1">
          {detail.participants.map((participant) => {
            const actorType = participant.type === "unknown" ? "system" : participant.type;
            const name = getActorName(actorType, participant.participant_id);
            return (
              <li key={participant.id} className="flex items-center gap-2 py-1">
                <ActorAvatar
                  actorType={actorType}
                  actorId={participant.participant_id}
                  size="sm"
                  showStatusDot={actorType === "agent"}
                  enableHoverCard={actorType === "agent" || actorType === "member"}
                />
                <span className="min-w-0 flex-1 truncate text-body text-foreground">{name}</span>
                <span className="shrink-0 text-caption text-muted-foreground">
                  {t(($) => $.detail[participant.role])}
                </span>
              </li>
            );
          })}
        </ul>
      </InspectorSection>

      <InspectorSection icon={Clock3} title={t(($) => $.detail.budget)}>
        <div className="space-y-2">
          <div className="flex items-center justify-between text-caption">
            <span className="text-muted-foreground">
              {dailyLimit
                ? t(($) => $.detail.turns_used, { count: usedTurns, limit: dailyLimit })
                : t(($) => $.detail.unlimited)}
            </span>
            {detail.room.schedule_interval_minutes ? (
              <span className="font-mono tabular-nums text-foreground">
                {t(($) => $.detail.minutes, {
                  count: detail.room.schedule_interval_minutes,
                })}
              </span>
            ) : null}
          </div>
          {dailyLimit ? <Progress value={budgetPercent} aria-label={t(($) => $.detail.budget)} /> : null}
          <p className="text-caption text-muted-foreground">
            {detail.room.next_wake_at
              ? t(($) => $.detail.next_wake, { time: timeAgo(detail.room.next_wake_at) })
              : t(($) => $.detail.disabled)}
          </p>
        </div>
      </InspectorSection>

      <InspectorSection icon={Brain} title={t(($) => $.detail.memory)}>
        <div className="flex items-center justify-between">
          <span className="text-caption text-muted-foreground">
            {t(($) => $.detail.memory_version, { version: detail.room.memory_version })}
          </span>
        </div>
        {detail.room.memory.summary ||
        detail.room.memory.facts.length > 0 ||
        detail.room.memory.decisions.length > 0 ||
        detail.room.memory.open_questions.length > 0 ||
        detail.room.memory.recent_contributions.length > 0 ? (
          <div className="mt-2 space-y-3">
            {detail.room.memory.summary ? (
              <p className="text-body leading-5 text-foreground">{detail.room.memory.summary}</p>
            ) : null}
            <MemoryList title={t(($) => $.detail.facts)} items={detail.room.memory.facts} />
            <MemoryList title={t(($) => $.detail.decisions)} items={detail.room.memory.decisions} />
            <MemoryList
              title={t(($) => $.detail.open_questions)}
              items={detail.room.memory.open_questions}
            />
            {detail.room.memory.recent_contributions.length > 0 ? (
              <div>
                <h3 className="text-caption font-medium text-muted-foreground">
                  {t(($) => $.detail.recent_contributions)}
                </h3>
                <ul className="mt-1 space-y-2">
                  {detail.room.memory.recent_contributions.map((contribution) => (
                    <li key={contribution.turn_id} className="flex gap-2">
                      <ActorAvatar
                        actorType="agent"
                        actorId={contribution.agent_id}
                        size="xs"
                        enableHoverCard
                      />
                      <p className="min-w-0 text-caption leading-5 text-foreground">
                        {contribution.body}
                      </p>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        ) : (
          <p className="mt-2 text-caption leading-5 text-muted-foreground">
            {t(($) => $.detail.empty_memory)}
          </p>
        )}
      </InspectorSection>

      <InspectorSection icon={CircleDot} title={t(($) => $.detail.cycles)}>
        {detail.cycles.length === 0 ? (
          <p className="text-caption text-muted-foreground">{t(($) => $.detail.empty_transcript)}</p>
        ) : (
          <ol className="space-y-1">
            {detail.cycles.slice(-6).reverse().map((cycle) => (
              <CycleRow
                key={cycle.id}
                cycle={cycle}
                roomTitle={detail.room.title}
                onPromote={onPromoteCycle}
              />
            ))}
          </ol>
        )}
      </InspectorSection>

      <InspectorSection icon={FileText} title={t(($) => $.detail.artifacts)}>
        {detail.artifacts.length === 0 ? (
          <p className="text-caption text-muted-foreground">
            {t(($) => $.detail.empty_artifacts)}
          </p>
        ) : (
          <ul className="space-y-1">
            {detail.artifacts.map((artifact) => {
              const href = artifactHref(artifact, paths);
              const content = (
                <>
                  <span className="min-w-0 flex-1 truncate text-body text-foreground">
                    {artifact.title}
                  </span>
                  <Badge variant="outline">
                    {t(($) => $.artifact.kinds[artifact.kind])}
                  </Badge>
                  {href ? <ArrowUpRight className="size-3.5 text-muted-foreground" aria-hidden="true" /> : null}
                </>
              );
              return (
                <li key={artifact.id} data-testid={`room-artifact-${artifact.id}`}>
                  {href ? (
                    <AppLink
                      href={href}
                      className="flex items-center gap-2 rounded-md px-1.5 py-1.5 outline-none hover:bg-surface-hover focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      {content}
                    </AppLink>
                  ) : (
                    <div className="flex items-center gap-2 px-1.5 py-1.5">{content}</div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </InspectorSection>
    </aside>
  );
}

function InspectorSection({
  icon: Icon,
  title,
  children,
}: {
  readonly icon: typeof Users;
  readonly title: string;
  readonly children: React.ReactNode;
}) {
  return (
    <section className="border-b border-surface-border px-3 py-3 last:border-b-0">
      <h2 className="mb-2 flex items-center gap-1.5 text-caption font-medium text-muted-foreground">
        <Icon className="size-3.5" aria-hidden="true" />
        {title}
      </h2>
      {children}
    </section>
  );
}

function MemoryList({ title, items }: { readonly title: string; readonly items: readonly string[] }) {
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="text-caption font-medium text-muted-foreground">{title}</h3>
      <ul className="mt-1 space-y-1">
        {items.map((item) => (
          <li key={item} className="flex gap-2 text-caption leading-5 text-foreground">
            <CheckCircle2 className="mt-1 size-3 shrink-0 text-success" aria-hidden="true" />
            <span>{item}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function CycleRow({
  cycle,
  roomTitle,
  onPromote,
}: {
  readonly cycle: RoomCycle;
  readonly roomTitle: string;
  readonly onPromote: (cycleId: string, title: string) => void;
}) {
  const { t } = useT("rooms");
  const timeAgo = useTimeAgo();
  const promotable = cycle.status === "completed";
  return (
    <li className="group rounded-md px-1.5 py-1.5 hover:bg-surface-hover">
      <div className="flex items-center gap-2">
        <span className={cn("text-caption font-medium", cycleStatusClass(cycle.status))}>
          {t(($) => $.cycle.status[cycle.status])}
        </span>
        <span className="text-caption text-muted-foreground">
          {t(($) => $.cycle.source[cycle.source])}
        </span>
        <time className="ml-auto font-mono text-caption tabular-nums text-muted-foreground">
          {timeAgo(cycle.created_at)}
        </time>
        {promotable ? (
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            aria-label={t(($) => $.actions.promote)}
            data-testid={`room-promote-cycle-${cycle.id}`}
            onClick={() => onPromote(cycle.id, roomTitle)}
          >
            <Sparkles aria-hidden="true" />
          </Button>
        ) : null}
      </div>
      {cycle.refusal_reason ? (
        <p className="mt-1 text-caption text-warning">
          {t(($) => $.refusal[cycle.refusal_reason ?? "unknown"])}
        </p>
      ) : null}
    </li>
  );
}
