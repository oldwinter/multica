"use client";

import {
  ArrowUpRight,
  CircleDot,
  Clock3,
  FileText,
  Gauge,
  Sparkles,
  Users,
} from "lucide-react";
import type {
  RoomCycle,
  RoomDetail,
  RoomPreflight,
  RoomUsage,
} from "@multica/core/rooms";
import { useWorkspacePaths } from "@multica/core/paths";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Progress } from "@multica/ui/components/ui/progress";
import { cn } from "@multica/ui/lib/utils";
import { useActorName } from "@multica/core/workspace/hooks";
import { useT, useTimeAgo } from "../i18n";
import { AppLink } from "../navigation";
import { ActorAvatar } from "../common/actor-avatar";
import { selectRecentRoomCycles } from "./room-controller";
import {
  artifactHref,
  countTodayTurns,
  cycleStatusClass,
} from "./room-display";

interface RoomInspectorProps {
  readonly detail: RoomDetail;
  readonly usage: RoomUsage;
  readonly preflight?: RoomPreflight;
  readonly scheduledPreflight?: RoomPreflight;
  readonly className?: string;
  readonly onPromoteCycle: (cycleId: string, title: string) => void;
}

export function RoomInspector({
  detail,
  usage,
  preflight,
  scheduledPreflight,
  className,
  onPromoteCycle,
}: RoomInspectorProps) {
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
    <aside className={cn("min-h-0 overflow-y-auto border-surface-border bg-surface", className)} data-testid="room-activity">
      <div className="sticky top-0 z-10 flex min-h-11 items-center border-b border-surface-border bg-surface px-3">
        <h2 className="text-body font-medium text-foreground">{t(($) => $.detail.activity)}</h2>
      </div>
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

      {preflight ? (
        <InspectorSection icon={Gauge} title={t(($) => $.preflight.title)}>
          <div className="space-y-3">
            <PreflightEstimate preflight={preflight} getActorName={getActorName} />
            {scheduledPreflight ? (
              <PreflightEstimate
                preflight={scheduledPreflight}
                getActorName={getActorName}
              />
            ) : null}
          </div>
        </InspectorSection>
      ) : null}

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

      <InspectorSection icon={Gauge} title={t(($) => $.usage.title)}>
        <dl className="grid grid-cols-2 gap-x-3 gap-y-2 text-caption">
          <UsageValue label={t(($) => $.usage.turns)} value={usage.turns_total} />
          <UsageValue label={t(($) => $.usage.cost)} value={usage.cost_ticks} />
          <UsageValue label={t(($) => $.usage.accepted)} value={usage.accepted_syntheses} />
          <UsageValue label={t(($) => $.usage.promoted)} value={usage.promoted_artifacts} />
          <UsageValue label={t(($) => $.usage.failures)} value={usage.failures} />
          <UsageValue label={t(($) => $.usage.uncosted)} value={usage.uncosted_turns} />
          <UsageValue label={t(($) => $.usage.repeat_runs)} value={usage.repeat_run_count} />
          <UsageValue label={t(($) => $.usage.active_weeks)} value={usage.active_weeks} />
          <UsageValue
            label={t(($) => $.usage.accepted_per_week)}
            value={formatRate(usage.accepted_outcomes_per_active_week)}
          />
          <UsageValue
            label={t(($) => $.usage.review_latency)}
            value={formatReviewLatency(usage.median_review_latency_seconds, t)}
          />
          <UsageValue
            label={t(($) => $.usage.promotion_rate)}
            value={`${Math.round(usage.promotion_rate * 100)}%`}
          />
          <UsageValue
            label={t(($) => $.usage.failed_refused)}
            value={`${usage.failed_cycles} / ${usage.refused_cycles}`}
          />
          <UsageValue
            label={t(($) => $.usage.cost_per_outcome)}
            value={formatRate(usage.cost_ticks_per_accepted_outcome)}
          />
        </dl>
      </InspectorSection>

      <InspectorSection icon={CircleDot} title={t(($) => $.detail.cycles)}>
        {detail.cycles.length === 0 ? (
          <p className="text-caption text-muted-foreground">{t(($) => $.detail.empty_transcript)}</p>
        ) : (
          <ol className="space-y-1">
            {selectRecentRoomCycles(detail.cycles).map((cycle) => (
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

function PreflightEstimate({
  preflight,
  getActorName,
}: {
  readonly preflight: RoomPreflight;
  readonly getActorName: (actorType: "agent", actorId: string) => string;
}) {
  const { t } = useT("rooms");
  const ready = preflight.target_agents.filter(
    (agent) => agent.ready && agent.invocation_allowed,
  ).length;
  const limit = preflight.budget.daily_turn_limit;
  const costLimit = preflight.budget.max_cost_ticks;

  return (
    <div className="space-y-1.5 border-b border-surface-border pb-3 last:border-b-0 last:pb-0">
      <div className="flex items-center justify-between gap-2 text-caption">
        <span className="font-medium text-foreground">
          {preflight.source === "schedule"
            ? t(($) => $.preflight.schedule)
            : t(($) => $.preflight.manual)}
        </span>
        <Badge variant={preflight.allowed ? "secondary" : "outline"}>
          {t(($) => preflight.allowed ? $.preflight.ready : $.preflight.blocked)}
        </Badge>
      </div>
      <p className="text-caption text-muted-foreground">
        {t(($) => $.preflight.agents_ready, {
          ready,
          count: preflight.target_agents.length,
        })}
      </p>
      <div className="flex flex-wrap gap-x-2 text-caption text-muted-foreground">
        <span>{t(($) => $.preflight.turns, { count: preflight.expected_max_turns })}</span>
        <span>
          {preflight.synthesis_required
            ? t(($) => $.preflight.synthesis)
            : t(($) => $.preflight.direct)}
        </span>
      </div>
      <ul className="space-y-1">
        {preflight.target_agents.map((agent) => (
          <li key={agent.agent_id} className="flex items-center justify-between gap-2 text-caption">
            <span className="min-w-0 truncate text-foreground">
              {getActorName("agent", agent.agent_id)}
            </span>
            <span className={agent.ready && agent.invocation_allowed ? "text-foreground" : "text-warning"}>
              {t(($) => agent.ready && agent.invocation_allowed
                ? $.preflight.ready
                : $.preflight.blocked)}
            </span>
          </li>
        ))}
      </ul>
      <p className="text-caption text-muted-foreground">
        {limit === null
          ? t(($) => $.detail.unlimited)
          : t(($) => $.preflight.daily, {
              used: preflight.budget.used_turns,
              limit,
            })}
      </p>
      <p className="text-caption text-muted-foreground">
        {costLimit === null
          ? t(($) => $.detail.unlimited)
          : t(($) => $.preflight.cost, {
              used: preflight.budget.used_cost_ticks,
              limit: costLimit,
            })}
        {preflight.budget.remaining_cost_ticks !== null
          ? ` / ${t(($) => $.preflight.remaining, {
              count: preflight.budget.remaining_cost_ticks,
            })}`
          : ""}
      </p>
      {preflight.budget.uncosted_turns > 0 ? (
        <p className="text-caption text-warning">
          {t(($) => $.preflight.uncosted, {
            count: preflight.budget.uncosted_turns,
          })}
        </p>
      ) : null}
		{preflight.budget.reserved_cost_ticks > 0 ? (
			<p className="text-caption text-muted-foreground">
				{t(($) => $.preflight.reserved, {
					count: preflight.budget.reserved_cost_ticks,
				})}
			</p>
		) : null}
		{!preflight.spend_limit_supported ? (
			<p className="text-caption text-warning">
				{t(($) => $.refusal.spend_limit_unsupported)}
			</p>
		) : null}
    </div>
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

function UsageValue({ label, value }: { readonly label: string; readonly value: number | string }) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 font-mono text-body tabular-nums text-foreground">{value}</dd>
    </div>
  );
}

function formatRate(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function formatReviewLatency(value: number, t: ReturnType<typeof useT<"rooms">>["t"]): string {
  if (value < 60) return t(($) => $.usage.seconds, { count: Math.round(value) });
  if (value < 3600) return t(($) => $.usage.minutes, { count: Math.round(value / 60) });
  return t(($) => $.usage.hours, { count: Math.round(value / 3600) });
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
  const refusalReason = cycle.refusal_reason;
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
      {refusalReason ? (
        <p className="mt-1 text-caption text-warning">
          {refusalReason === "active_cycle"
            ? t(($) => $.refusal.cycle_active)
            : t(($) => $.refusal[refusalReason])}
        </p>
      ) : null}
    </li>
  );
}
