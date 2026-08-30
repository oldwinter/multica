import { AlertTriangle, LoaderCircle, RefreshCw } from "lucide-react";
import type { OfficeDataGap, OfficeModel } from "@multica/core/office";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../i18n";

export function OfficeLoadingState() {
  const { t } = useT("office");
  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-4 p-5 motion-reduce:[&_[data-slot=skeleton]]:animate-none"
      role="status"
      aria-label={t(($) => $.states.loading_title)}
    >
      <div className="flex items-center gap-2 text-body text-muted-foreground">
        <LoaderCircle
          className="size-4 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
        <span>{t(($) => $.states.loading_title)}</span>
      </div>
      <Skeleton className="min-h-52 flex-1 rounded-md" />
    </div>
  );
}

export function OfficeUnavailableState({
  retry,
}: {
  readonly retry: () => Promise<void>;
}) {
  const { t } = useT("office");
  return (
    <div className="flex flex-1 items-center justify-center p-6">
      <div className="max-w-sm text-center">
        <AlertTriangle
          className="mx-auto size-6 text-warning"
          aria-hidden="true"
        />
        <h2 className="mt-3 text-title-sm font-semibold text-foreground">
          {t(($) => $.states.unavailable_title)}
        </h2>
        <p className="mt-1 text-body text-muted-foreground">
          {t(($) => $.states.unavailable_body)}
        </p>
        <Button
          type="button"
          variant="outline"
          className="mt-4 min-h-11"
          onClick={() => void retry()}
        >
          <RefreshCw aria-hidden="true" />
          {t(($) => $.states.retry)}
        </Button>
      </div>
    </div>
  );
}

type OfficeQuality = Extract<OfficeModel, { kind: "ready" }>["quality"];

function gapLabel(
  gap: OfficeDataGap,
  t: ReturnType<typeof useT<"office">>["t"],
): string {
  switch (gap) {
    case "availability":
      return t(($) => $.states.gaps.availability);
    case "workload":
      return t(($) => $.states.gaps.workload);
    case "squads":
      return t(($) => $.states.gaps.squads);
    case "issue-briefs":
      return t(($) => $.states.gaps.issue_briefs);
    case "selected-squad":
      return t(($) => $.states.gaps.selected_squad);
    default: {
      const exhaustive: never = gap;
      return exhaustive;
    }
  }
}

export function OfficeQualityNotice({ quality }: { readonly quality: OfficeQuality }) {
  const { t } = useT("office");
  if (quality.kind === "current") {
    if (!quality.refreshing) return null;
    return (
      <div className="flex min-h-8 shrink-0 items-center gap-2 border-b border-surface-border bg-surface px-3 text-caption text-muted-foreground">
        <RefreshCw
          className="size-3.5 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
        {t(($) => $.states.refreshing)}
      </div>
    );
  }

  const stale = quality.kind === "stale";
  return (
    <div
      data-testid="office-quality-notice"
      data-quality={quality.kind}
      className="shrink-0 border-b border-surface-border bg-surface px-3 py-2"
    >
      <div className="flex min-w-0 items-start gap-2">
        <AlertTriangle
          className="mt-0.5 size-4 shrink-0 text-warning"
          aria-hidden="true"
        />
        <div className="min-w-0">
          <h2 className="text-body font-semibold text-foreground">
            {stale
              ? t(($) => $.states.stale_title)
              : t(($) => $.states.partial_title)}
          </h2>
          <p className="mt-0.5 max-w-[75ch] text-caption text-muted-foreground">
            {stale
              ? t(($) => $.states.stale_body)
              : t(($) => $.states.partial_body)}
          </p>
          {quality.gaps.length > 0 ? (
            <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-caption text-muted-foreground">
              {quality.gaps.map((gap) => (
                <li key={gap}>{gapLabel(gap, t)}</li>
              ))}
            </ul>
          ) : null}
        </div>
      </div>
    </div>
  );
}
