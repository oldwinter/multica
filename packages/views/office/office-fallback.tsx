import { Bot, Flag, MapPinned, RadioTower, Users } from "lucide-react";
import type { OfficeSnapshot, OfficeWorldId } from "@multica/core/office";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { officeSnapshotCounts } from "./office-counts";

const POSTER_URLS = {
  studio: new URL("./worlds/studio/assets/poster.png", import.meta.url).href,
  expedition: new URL(
    "./worlds/expedition/assets/poster.png",
    import.meta.url,
  ).href,
} satisfies Record<OfficeWorldId, string>;

export function OfficeDomFallback({
  snapshot,
  world,
  reason,
  onPosterReady,
  onPosterError,
}: {
  readonly snapshot: OfficeSnapshot;
  readonly world: OfficeWorldId;
  readonly reason: "slot" | "narrow" | "renderer";
  readonly onPosterReady: (world: OfficeWorldId) => void;
  readonly onPosterError: (world: OfficeWorldId) => void;
}) {
  const { t } = useT("office");
  const isStudio = world === "studio";
  const counts = officeSnapshotCounts(snapshot);
  const subjectCount =
    counts.agents.total + counts.squads.total + counts.issues.total;

  return (
    <section
      data-testid="office-dom-fallback"
      data-world={world}
      data-fallback-reason={reason}
      aria-labelledby="office-poster-title"
      className="relative flex min-h-[30rem] flex-1 flex-col overflow-hidden bg-surface md:min-h-0"
    >
      <div
        aria-hidden="true"
        className="relative aspect-video w-full flex-none overflow-hidden bg-surface-raised md:min-h-44 md:basis-[52%] md:aspect-auto"
      >
        <img
          key={world}
          src={POSTER_URLS[world]}
          alt=""
          draggable={false}
          className="h-full w-full object-cover"
          style={{ imageRendering: "pixelated" }}
          onLoad={() => onPosterReady(world)}
          onError={() => onPosterError(world)}
        />
      </div>

      <div className="relative z-10 flex min-h-0 flex-1 flex-col justify-between gap-6 border-t border-surface-border bg-surface p-5 max-md:flex-none sm:p-7">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h2
              id="office-poster-title"
              className="text-title font-semibold text-foreground"
            >
              {isStudio
                ? t(($) => $.poster.studio_title)
                : t(($) => $.poster.expedition_title)}
            </h2>
            <p className="mt-1 max-w-[52ch] text-body text-muted-foreground">
              {isStudio
                ? t(($) => $.poster.studio_description)
                : t(($) => $.poster.expedition_description)}
            </p>
          </div>
          <span className="inline-flex size-10 shrink-0 items-center justify-center rounded-md border border-surface-border bg-surface-raised text-muted-foreground">
            {isStudio ? (
              <RadioTower className="size-5" aria-hidden="true" />
            ) : (
              <MapPinned className="size-5" aria-hidden="true" />
            )}
          </span>
        </div>

        <div className="grid grid-cols-3 gap-2 sm:max-w-lg sm:gap-3">
          <PosterMetric
            icon={<Bot aria-hidden="true" />}
            label={t(($) => $.roster.tabs.agents)}
            value={counts.agents.total}
            accentClassName="bg-success"
          />
          <PosterMetric
            icon={<Users aria-hidden="true" />}
            label={t(($) => $.roster.tabs.squads)}
            value={counts.squads.total}
            accentClassName="bg-info"
          />
          <PosterMetric
            icon={<Flag aria-hidden="true" />}
            label={t(($) => $.roster.tabs.issues)}
            value={counts.issues.total}
            accentClassName="bg-warning"
          />
        </div>

        {subjectCount === 0 ? (
          <div className="max-w-lg border-t border-surface-border pt-3">
            <h3 className="text-title-sm font-semibold text-foreground">
              {t(($) => $.states.empty_title)}
            </h3>
            <p className="mt-1 text-body text-muted-foreground">
              {t(($) => $.states.empty_body)}
            </p>
          </div>
        ) : null}

        {reason === "renderer" ? (
          <p className="max-w-lg border-t border-surface-border pt-3 text-body text-muted-foreground">
            {t(($) => $.poster.renderer_fallback)}
          </p>
        ) : null}
      </div>
    </section>
  );
}

function PosterMetric({
  icon,
  label,
  value,
  accentClassName,
}: {
  readonly icon: React.ReactNode;
  readonly label: string;
  readonly value: number;
  readonly accentClassName: string;
}) {
  return (
    <div className="min-w-0 border-t border-surface-border pt-2">
      <div className="flex items-center gap-1.5 text-muted-foreground">
        <span className={cn("h-1 w-4 shrink-0", accentClassName)} />
        <span className="[&_svg]:size-3.5">{icon}</span>
      </div>
      <div className="mt-2 font-mono text-title-sm font-semibold tabular-nums text-foreground">
        {value}
      </div>
      <div className="break-words text-caption text-muted-foreground">
        {label}
      </div>
    </div>
  );
}
