import type { ReactNode } from "react";
import { Pressable, View } from "react-native";
import type {
  RoomArtifact,
  RoomCycle,
  RoomDetail,
  RoomMemoryRevision,
  RoomPreflight,
  RoomRecommendation,
  RoomSynthesisItem,
  RoomUsage,
} from "@/data/rooms-types";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Text } from "@/components/ui/text";
import {
  confidenceLabel,
  roomPhaseLabel,
  roomReviewLabel,
  roomStatusLabel,
} from "@/lib/room-display";
import {
  canPromoteRoomRevision,
  canRetrySynthesis,
  recommendationStatus,
} from "@/lib/room-interactions";
import { timeAgo } from "@/lib/time-ago";
import { useActorLookup } from "@/data/use-actor-name";
import {
  latestRoomCycle,
  latestRoomMemoryRevision,
} from "@/lib/room-selectors";

interface Props {
  readonly detail: RoomDetail;
  readonly preflight: RoomPreflight | undefined;
  readonly scheduledPreflight: RoomPreflight | undefined;
  readonly usage: RoomUsage | undefined;
  readonly onReview: (cycleId: string, memoryRevisionId: string) => void;
  readonly onRecommendation: (
    memoryRevisionId: string,
    recommendation: RoomRecommendation,
  ) => void;
  readonly onRetrySynthesis: (cycleId: string) => void;
  readonly onCancelCycle: (cycleId: string) => void;
  readonly onOpenArtifact: (artifact: RoomArtifact) => void;
}

export function RoomDetailSections({
  detail,
  preflight,
  scheduledPreflight,
  usage,
  onReview,
  onRecommendation,
  onRetrySynthesis,
  onCancelCycle,
  onOpenArtifact,
}: Props) {
  const latestCycle = latestRoomCycle(detail.cycles);
  const latestRevision = latestRoomMemoryRevision(detail.memory_revisions);

  return (
    <View>
      <RoomOverview
        detail={detail}
        preflight={preflight}
        scheduledPreflight={scheduledPreflight}
      />
      <Separator />
      <OutcomeSection
        revision={latestRevision}
        reviews={detail.recommendation_reviews}
        onReview={onReview}
        onRecommendation={onRecommendation}
      />
      <Separator />
      <CycleSection
        cycle={latestCycle}
        onRetry={onRetrySynthesis}
        onCancel={onCancelCycle}
      />
      <Separator />
      <MemorySection detail={detail} />
      <Separator />
      <UsageSection usage={usage} />
      <Separator />
      <ArtifactsSection
        artifacts={detail.artifacts}
        onOpenArtifact={onOpenArtifact}
      />
      <Separator />
      <TranscriptSection detail={detail} />
    </View>
  );
}

function Section({
  title,
  trailing,
  children,
}: {
  title: string;
  trailing?: ReactNode;
  children: ReactNode;
}) {
  return (
    <View className="px-4 py-4 gap-3">
      <View className="flex-row items-center justify-between gap-3">
        <Text className="text-xs uppercase text-muted-foreground font-semibold">
          {title}
        </Text>
        {trailing}
      </View>
      {children}
    </View>
  );
}

function RoomOverview({
  detail,
  preflight,
  scheduledPreflight,
}: {
  detail: RoomDetail;
  preflight: RoomPreflight | undefined;
  scheduledPreflight: RoomPreflight | undefined;
}) {
  const room = detail.room;
  const blockedAgents =
    preflight?.target_agents.filter(
      (agent) => !agent.ready || !agent.invocation_allowed,
    ) ?? [];
  const daemonUpgradeRequired =
    preflight &&
    (!preflight.capability_ready ||
      blockedAgents.some(
        (agent) => agent.reason === "daemon_capability_unavailable",
      ));
	const spendLimitUnsupported =
		preflight &&
		(!preflight.spend_limit_supported ||
			blockedAgents.some((agent) => agent.reason === "spend_limit_unsupported"));
  return (
    <View className="px-4 py-4 gap-3">
      <View className="flex-row items-center justify-between gap-3">
        <Text className="text-xs font-semibold text-muted-foreground uppercase">
          {roomStatusLabel(room.status)}
        </Text>
        <Text className="text-xs text-muted-foreground">
          {detail.participants.length} participants
        </Text>
      </View>
      <Text className="text-lg font-semibold text-foreground" selectable>
        {room.objective || room.title}
      </Text>
      {room.instructions ? (
        <Text className="text-sm text-muted-foreground" selectable>
          {room.instructions}
        </Text>
      ) : null}
      {room.success_criteria.length > 0 ? (
        <LabeledStrings label="Success criteria" items={room.success_criteria} />
      ) : null}
      {room.stop_conditions.length > 0 ? (
        <LabeledStrings label="Stop conditions" items={room.stop_conditions} />
      ) : null}
      {preflight ? <PreflightEstimate preflight={preflight} /> : null}
      {scheduledPreflight ? (
        <PreflightEstimate preflight={scheduledPreflight} />
      ) : null}
      {blockedAgents.length > 0 ? (
        <View className="rounded-md bg-warning/10 border border-warning/30 px-3 py-2 gap-1">
          <Text className="text-sm font-medium text-foreground">
			{spendLimitUnsupported
				? "Cost-bound execution unavailable"
				: daemonUpgradeRequired
              ? "Daemon upgrade required"
              : `${blockedAgents.length} participant${
                  blockedAgents.length === 1 ? " is" : "s are"
                } not ready`}
          </Text>
			{spendLimitUnsupported ? (
				<Text className="text-xs text-muted-foreground">
					This Room will not queue work until its Agents use an execution backend that enforces the assigned cost quota.
				</Text>
			) : daemonUpgradeRequired ? (
            <Text className="text-xs text-muted-foreground">
              Upgrade and restart the affected Agent daemon for
              {preflight.required_daemon_capability
                ? ` ${preflight.required_daemon_capability}`
                : " the required Room capability"}
              .
            </Text>
          ) : null}
          {blockedAgents.map((agent) => (
            <Text key={agent.agent_id} className="text-xs text-muted-foreground">
				{agent.reason === "spend_limit_unsupported"
					? "Agent backend cannot enforce this Room's cost quota"
					: agent.reason === "daemon_capability_unavailable"
                ? "Agent daemon does not advertise the required capability"
                : agent.reason || "Agent unavailable"}
            </Text>
          ))}
        </View>
      ) : null}
    </View>
  );
}

function PreflightEstimate({ preflight }: { preflight: RoomPreflight }) {
  const ready = preflight.target_agents.filter(
    (agent) => agent.ready && agent.invocation_allowed,
  ).length;
  const dailyLimit = preflight.budget.daily_turn_limit;
  const costLimit = preflight.budget.max_cost_ticks;
  return (
    <View className="border-t border-border pt-3 gap-2">
      <View className="flex-row items-center justify-between gap-3">
        <Text className="text-sm font-medium text-foreground">
          {preflight.source === "schedule"
            ? "Scheduled estimate"
            : "Manual estimate"}
        </Text>
        <Text
          className={
            preflight.allowed
              ? "text-xs text-foreground"
              : "text-xs text-warning"
          }
        >
          {preflight.allowed ? "Ready" : "Blocked"}
        </Text>
      </View>
      <Text className="text-xs text-muted-foreground">
        {ready} / {preflight.target_agents.length} Agents ready /{" "}
        {preflight.expected_max_turns} max turns
      </Text>
      <Text className="text-xs text-muted-foreground">
        {preflight.synthesis_required ? "Facilitator synthesis" : "Direct result"}
      </Text>
      {preflight.target_agents.map((agent) => (
        <View key={agent.agent_id} className="flex-row items-center justify-between gap-3">
          <Text className="text-xs text-foreground" numberOfLines={1}>
            {agent.agent_id}
          </Text>
          <Text
            className={
              agent.ready && agent.invocation_allowed
                ? "text-xs text-muted-foreground"
                : "text-xs text-warning"
            }
          >
            {agent.ready && agent.invocation_allowed
              ? "Ready"
              : agent.reason || "Blocked"}
          </Text>
        </View>
      ))}
      <Text className="text-xs text-muted-foreground">
        {dailyLimit === null
          ? "Daily turns unlimited"
          : `${preflight.budget.used_turns} / ${dailyLimit} daily turns`}
      </Text>
      <Text className="text-xs text-muted-foreground">
        {costLimit === null
          ? "Cost unlimited"
          : `${preflight.budget.used_cost_ticks} / ${costLimit} cost ticks`}
        {preflight.budget.remaining_cost_ticks === null
          ? ""
          : ` / ${preflight.budget.remaining_cost_ticks} remaining`}
      </Text>
      {preflight.budget.uncosted_turns > 0 ? (
        <Text className="text-xs text-warning">
          {preflight.budget.uncosted_turns} uncosted turns
        </Text>
      ) : null}
		{preflight.budget.reserved_cost_ticks > 0 ? (
			<Text className="text-xs text-muted-foreground">
				{preflight.budget.reserved_cost_ticks} cost ticks reserved for active work
			</Text>
		) : null}
    </View>
  );
}

function OutcomeSection({
  revision,
  reviews,
  onReview,
  onRecommendation,
}: {
  revision: RoomMemoryRevision | undefined;
  reviews: RoomDetail["recommendation_reviews"];
  onReview: (cycleId: string, revisionId: string) => void;
  onRecommendation: (revisionId: string, recommendation: RoomRecommendation) => void;
}) {
  const { getName } = useActorLookup();
  if (!revision) {
    return (
      <Section title="Outcome">
        <Text className="text-sm text-muted-foreground">
          No synthesis yet. Replies and Agent contributions will appear in the
          transcript until the facilitator produces an outcome.
        </Text>
      </Section>
    );
  }

  const synthesis = revision.synthesis;
  return (
    <Section
      title="Outcome"
      trailing={
        <Text className="text-xs text-muted-foreground">
          {roomReviewLabel(revision.review_status)}
        </Text>
      }
    >
      <Text className="text-base font-medium text-foreground" selectable>
        {synthesis.summary || "No summary provided"}
      </Text>
      <Text className="text-xs text-muted-foreground">
        Created by {getName(revision.creator_type, revision.creator_id)}
      </Text>
      {confidenceLabel(synthesis.confidence) ? (
        <Text className="text-xs text-muted-foreground">
          {confidenceLabel(synthesis.confidence)}
        </Text>
      ) : null}
      <SynthesisGroup label="Facts" items={synthesis.facts} />
      <SynthesisGroup label="Decisions" items={synthesis.decisions} />
      <SynthesisGroup label="Open questions" items={synthesis.open_questions} />
      <SynthesisGroup label="Disagreements" items={synthesis.disagreements} />
      <SynthesisGroup label="Action items" items={synthesis.action_items} />

      {revision.review_status === "pending" ? (
        <Button
          variant="default"
          onPress={() => onReview(revision.cycle_id, revision.id)}
          accessibilityLabel="Review synthesis"
        >
          <Text>Review synthesis</Text>
        </Button>
      ) : null}

      {synthesis.recommendations.length > 0 ? (
        <View className="gap-2 pt-1">
          <Text className="text-xs font-semibold text-muted-foreground uppercase">
            Recommendations
          </Text>
          {synthesis.recommendations.map((recommendation) => {
            const status = recommendationStatus(
              reviews,
              revision.id,
              recommendation.key,
            );
            const canPromote = canPromoteRoomRevision(revision.review_status);
            return (
              <Pressable
                key={recommendation.key || `${recommendation.kind}:${recommendation.title}`}
                onPress={() => onRecommendation(revision.id, recommendation)}
                disabled={
                  !canPromote || status === "approved" || status === "rejected"
                }
                className="rounded-md border border-border px-3 py-3 active:bg-secondary disabled:opacity-60"
                accessibilityRole="button"
                accessibilityLabel={`${recommendation.kind} recommendation: ${recommendation.title}`}
              >
                <View className="flex-row items-start justify-between gap-3">
                  <View className="flex-1 gap-1">
                    <Text className="text-sm font-medium text-foreground">
                      {recommendation.title || `New ${recommendation.kind}`}
                    </Text>
                    {recommendation.rationale ? (
                      <Text className="text-xs text-muted-foreground" numberOfLines={3}>
                        {recommendation.rationale}
                      </Text>
                    ) : null}
                  </View>
                  <Text className="text-xs text-muted-foreground capitalize">
                    {status ?? (canPromote ? recommendation.kind : "Accept first")}
                  </Text>
                </View>
              </Pressable>
            );
          })}
        </View>
      ) : null}
    </Section>
  );
}

function SynthesisGroup({
  label,
  items,
}: {
  label: string;
  items: readonly RoomSynthesisItem[];
}) {
  if (items.length === 0) return null;
  return (
    <View className="gap-2">
      <Text className="text-xs font-semibold text-muted-foreground uppercase">
        {label}
      </Text>
      {items.map((item, index) => (
        <View key={`${label}:${index}`} className="pl-1 gap-1">
          <Text className="text-sm text-foreground" selectable>
            • {item.text}
          </Text>
          {item.citation_entry_ids.length > 0 ? (
            <Text className="text-[11px] text-muted-foreground" selectable>
              Sources: {item.citation_entry_ids.map(shortId).join(", ")}
            </Text>
          ) : (
            <Text className="text-[11px] text-warning">No cited source</Text>
          )}
        </View>
      ))}
    </View>
  );
}

function CycleSection({
  cycle,
  onRetry,
  onCancel,
}: {
  cycle: RoomCycle | undefined;
  onRetry: (cycleId: string) => void;
  onCancel: (cycleId: string) => void;
}) {
  if (!cycle) {
    return (
      <Section title="Cycle">
        <Text className="text-sm text-muted-foreground">No cycle has run yet.</Text>
      </Section>
    );
  }
  const active = cycle.phase === "gathering" || cycle.phase === "synthesizing";
  return (
    <Section
      title={`Cycle ${cycle.sequence}`}
      trailing={
        <Text className="text-xs font-medium text-muted-foreground">
          {roomPhaseLabel(cycle.phase)}
        </Text>
      }
    >
      {cycle.synthesis_error ? (
        <View className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 gap-1">
          <Text className="text-sm font-medium text-foreground">
            Synthesis failed
          </Text>
          <Text className="text-xs text-muted-foreground" selectable>
            {cycle.synthesis_error.message || cycle.synthesis_error.code}
          </Text>
        </View>
      ) : (
        <Text className="text-sm text-muted-foreground">
          {cycle.expected_max_turns !== null
            ? `Up to ${cycle.expected_max_turns} turns`
            : `Started ${timeAgo(cycle.created_at)}`}
        </Text>
      )}
      <View className="flex-row gap-2">
        {canRetrySynthesis(cycle) ? (
          <Button variant="outline" size="sm" onPress={() => onRetry(cycle.id)}>
            <Text>Retry synthesis</Text>
          </Button>
        ) : null}
        {active ? (
          <Button variant="outline" size="sm" onPress={() => onCancel(cycle.id)}>
            <Text>Cancel cycle</Text>
          </Button>
        ) : null}
      </View>
    </Section>
  );
}

function MemorySection({ detail }: { detail: RoomDetail }) {
  const room = detail.room;
  return (
    <Section
      title="Accepted memory"
      trailing={
        <Text className="text-xs text-muted-foreground">
          v{room.memory_version}
        </Text>
      }
    >
      {room.memory.summary ? (
        <Text className="text-sm text-foreground" selectable>
          {room.memory.summary}
        </Text>
      ) : (
        <Text className="text-sm text-muted-foreground">
          No accepted memory yet.
        </Text>
      )}
      <LabeledStrings label="Facts" items={room.memory.facts} />
      <LabeledStrings label="Decisions" items={room.memory.decisions} />
      <LabeledStrings label="Open questions" items={room.memory.open_questions} />
    </Section>
  );
}

function UsageSection({ usage }: { usage: RoomUsage | undefined }) {
  return (
    <Section title="Usage">
      {!usage ? (
        <Text className="text-sm text-muted-foreground">Usage unavailable.</Text>
      ) : (
        <View className="flex-row flex-wrap gap-x-5 gap-y-3">
          <Metric value={usage.turns_total} label="Turns" />
          <Metric value={usage.cost_ticks} label="Cost ticks" />
          <Metric value={usage.accepted_syntheses} label="Accepted" />
          <Metric value={usage.promoted_artifacts} label="Promoted" />
          <Metric value={usage.failures} label="Failures" />
          <Metric value={usage.repeat_run_count} label="Repeat runs" />
          <Metric
            value={formatMetric(usage.accepted_outcomes_per_active_week)}
            label="Accepted / week"
          />
          <Metric
            value={`${Math.round(usage.promotion_rate * 100)}%`}
            label="Promotion rate"
          />
          <Metric
            value={formatDuration(usage.median_review_latency_seconds)}
            label="Median review"
          />
          <Metric
            value={`${usage.failed_cycles} / ${usage.refused_cycles}`}
            label="Failed / refused"
          />
          <Metric
            value={formatMetric(usage.cost_ticks_per_accepted_outcome)}
            label="Cost / accepted"
          />
          {usage.uncosted_turns > 0 ? (
            <Metric value={usage.uncosted_turns} label="Uncosted" />
          ) : null}
        </View>
      )}
    </Section>
  );
}

function Metric({ value, label }: { value: number | string; label: string }) {
  return (
    <View className="min-w-16 gap-0.5">
      <Text className="text-lg font-semibold text-foreground tabular-nums">
        {value}
      </Text>
      <Text className="text-xs text-muted-foreground">{label}</Text>
    </View>
  );
}

function formatMetric(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds / 3600)}h`;
}

function ArtifactsSection({
  artifacts,
  onOpenArtifact,
}: {
  artifacts: readonly RoomArtifact[];
  onOpenArtifact: (artifact: RoomArtifact) => void;
}) {
  return (
    <Section title="Promoted results">
      {artifacts.length === 0 ? (
        <Text className="text-sm text-muted-foreground">
          No Issue, Wiki page, or Decision has been promoted yet.
        </Text>
      ) : (
        artifacts.map((artifact) => (
          <Pressable
            key={artifact.id}
            onPress={() => onOpenArtifact(artifact)}
            disabled={!artifact.target_id}
            className="flex-row items-start justify-between gap-3 py-1 active:opacity-60"
            accessibilityRole="link"
          >
            <View className="flex-1 gap-0.5">
              <Text className="text-sm font-medium text-foreground">
                {artifact.title}
              </Text>
              <Text className="text-xs text-muted-foreground capitalize">
                {artifact.kind} · {timeAgo(artifact.created_at)}
              </Text>
            </View>
            <Text className="text-xs text-primary">
              {artifact.target_id ? "Open" : "Pending"}
            </Text>
          </Pressable>
        ))
      )}
    </Section>
  );
}

function TranscriptSection({ detail }: { detail: RoomDetail }) {
  const entries = [...detail.entries].sort((a, b) => a.ordinal - b.ordinal);
  return (
    <Section title="Transcript">
      {entries.length === 0 ? (
        <Text className="text-sm text-muted-foreground">
          No messages or Agent results yet.
        </Text>
      ) : (
        entries.map((entry) => (
          <View
            key={entry.id}
            nativeID={`room-entry-${entry.id}`}
            className="border-l-2 border-border pl-3 py-1 gap-1"
          >
            <View className="flex-row items-center justify-between gap-3">
              <Text className="text-xs font-medium text-muted-foreground capitalize">
                {entry.author_type === "unknown" ? "Unknown author" : entry.author_type}
              </Text>
              <Text className="text-[11px] text-muted-foreground">
                #{entry.ordinal} · {timeAgo(entry.created_at)}
              </Text>
            </View>
            <Text className="text-sm text-foreground" selectable>
              {entry.body}
            </Text>
          </View>
        ))
      )}
    </Section>
  );
}

function LabeledStrings({
  label,
  items,
}: {
  label: string;
  items: readonly string[];
}) {
  if (items.length === 0) return null;
  return (
    <View className="gap-1">
      <Text className="text-xs font-semibold text-muted-foreground uppercase">
        {label}
      </Text>
      {items.map((item, index) => (
        <Text key={`${label}:${index}`} className="text-sm text-foreground" selectable>
          • {item}
        </Text>
      ))}
    </View>
  );
}

function shortId(value: string): string {
  return value.length > 8 ? value.slice(0, 8) : value;
}
