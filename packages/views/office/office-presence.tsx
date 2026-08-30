import {
  Activity,
  CircleCheck,
  CircleHelp,
  CircleOff,
  Clock3,
  Gauge,
  Pause,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import type {
  OfficeAvailability,
  OfficeWorkload,
} from "@multica/core/office";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

type PresenceLabelKey =
  | "online"
  | "unstable"
  | "offline"
  | "idle"
  | "queued"
  | "working"
  | "loading"
  | "unavailable";

interface PresencePresentation {
  readonly icon: LucideIcon;
  readonly tone: string;
  readonly labelKey: PresenceLabelKey;
}

function availabilityPresentation(
  availability: OfficeAvailability,
): PresencePresentation {
  if (availability.kind === "unknown") {
    return {
      icon: CircleHelp,
      tone: "text-muted-foreground",
      labelKey: availability.reason,
    };
  }
  switch (availability.value) {
    case "online":
      return { icon: CircleCheck, tone: "text-success", labelKey: "online" };
    case "unstable":
      return {
        icon: TriangleAlert,
        tone: "text-warning",
        labelKey: "unstable",
      };
    case "offline":
      return {
        icon: CircleOff,
        tone: "text-muted-foreground",
        labelKey: "offline",
      };
    default: {
      const exhaustive: never = availability.value;
      return exhaustive;
    }
  }
}

function workloadPresentation(workload: OfficeWorkload): PresencePresentation {
  if (workload.kind === "unknown") {
    return {
      icon: CircleHelp,
      tone: "text-muted-foreground",
      labelKey: workload.reason,
    };
  }
  switch (workload.value) {
    case "idle":
      return { icon: Pause, tone: "text-muted-foreground", labelKey: "idle" };
    case "queued":
      return { icon: Clock3, tone: "text-warning", labelKey: "queued" };
    case "working":
      return { icon: Activity, tone: "text-success", labelKey: "working" };
    default: {
      const exhaustive: never = workload.value;
      return exhaustive;
    }
  }
}

function presenceLabel(
  key: PresenceLabelKey,
  t: ReturnType<typeof useT<"office">>["t"],
) {
  switch (key) {
    case "online":
      return t(($) => $.presence.online);
    case "unstable":
      return t(($) => $.presence.unstable);
    case "offline":
      return t(($) => $.presence.offline);
    case "idle":
      return t(($) => $.presence.idle);
    case "queued":
      return t(($) => $.presence.queued);
    case "working":
      return t(($) => $.presence.working);
    case "loading":
      return t(($) => $.presence.loading);
    case "unavailable":
      return t(($) => $.presence.unavailable);
    default: {
      const exhaustive: never = key;
      return exhaustive;
    }
  }
}

export function OfficePresence({
  availability,
  workload,
  compact = false,
}: {
  readonly availability: OfficeAvailability;
  readonly workload: OfficeWorkload;
  readonly compact?: boolean;
}) {
  const { t } = useT("office");
  const availabilityView = availabilityPresentation(availability);
  const workloadView = workloadPresentation(workload);
  const AvailabilityIcon = availabilityView.icon;
  const WorkloadIcon = workloadView.icon;

  return (
    <div
      className={cn(
        "flex min-w-0 text-caption",
        compact
          ? "flex-wrap gap-x-3 gap-y-1"
          : "flex-col gap-2 border-y border-surface-border py-3",
      )}
    >
      <div className="flex min-w-0 items-center gap-1.5">
        <AvailabilityIcon
          className={cn("size-3.5 shrink-0", availabilityView.tone)}
          aria-hidden="true"
        />
        {!compact ? (
          <span className="text-muted-foreground">
            {t(($) => $.presence.availability)}
          </span>
        ) : null}
        <span className={cn("font-medium", availabilityView.tone)}>
          {presenceLabel(availabilityView.labelKey, t)}
        </span>
      </div>

      <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5">
        <WorkloadIcon
          className={cn("size-3.5 shrink-0", workloadView.tone)}
          aria-hidden="true"
        />
        {!compact ? (
          <span className="text-muted-foreground">
            {t(($) => $.presence.workload)}
          </span>
        ) : null}
        <span className={cn("font-medium", workloadView.tone)}>
          {presenceLabel(workloadView.labelKey, t)}
        </span>
        {workload.kind === "known" ? (
          <span className="inline-flex flex-wrap gap-x-1.5 font-mono tabular-nums text-muted-foreground">
            {workload.runningCount > 0 ? (
              <span>
                {t(($) => $.presence.running_count, {
                  count: workload.runningCount,
                })}
              </span>
            ) : null}
            {workload.queuedCount > 0 ? (
              <span>
                {t(($) => $.presence.queued_count, {
                  count: workload.queuedCount,
                })}
              </span>
            ) : null}
            {!compact ? (
              <span className="inline-flex items-center gap-1">
                <Gauge className="size-3" aria-hidden="true" />
                {t(($) => $.presence.capacity, {
                  count: workload.capacity,
                })}
              </span>
            ) : null}
          </span>
        ) : null}
      </div>
    </div>
  );
}
