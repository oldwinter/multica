import type { ReactNode } from "react";
import { cn } from "@multica/ui/lib/utils";

export type WikiNarrowDetailRole = "collection-echo" | "required";

interface WikiMasterDetailProps {
  detailRole: WikiNarrowDetailRole;
  collection: ReactNode;
  detail: ReactNode;
}

export const wikiInteractionRegionClassName = [
  "max-lg:[&_button]:min-h-11",
  "max-lg:[&_button]:min-w-11",
  "max-lg:[&_[data-slot=input]]:min-h-11",
  "max-lg:[&_[data-slot=tabs-list]]:min-h-11",
  "max-lg:[&_[data-slot=select-trigger]]:min-h-11",
  "max-lg:[&_[data-slot=dialog-header]]:pr-12",
].join(" ");

export const wikiSelectContentClassName = cn(
  wikiInteractionRegionClassName,
  "max-lg:[&_[data-slot=select-item]]:min-h-11",
);

export const wikiDestructiveActionClassName =
  "bg-destructive text-destructive-foreground hover:bg-destructive/90 dark:bg-destructive dark:text-destructive-foreground dark:hover:bg-destructive/90";

export function WikiMasterDetail({
  detailRole,
  collection,
  detail,
}: WikiMasterDetailProps) {
  const hidesNarrowDetail = detailRole === "collection-echo";

  return (
    <div
      className={cn(
        "grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(14rem,18rem)_minmax(0,1fr)] lg:grid-rows-1",
        hidesNarrowDetail
          ? "grid-rows-[minmax(10rem,1fr)]"
          : "grid-rows-[minmax(10rem,35dvh)_minmax(20rem,1fr)]",
      )}
      data-testid="wiki-master-detail"
      data-narrow-detail-role={detailRole}
    >
      <aside
        className="min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-3 shadow-[var(--surface-shadow)]"
        data-testid="wiki-collection-pane"
      >
        {collection}
      </aside>
      <section
        className={cn(
          "min-h-0 overflow-y-auto rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]",
          hidesNarrowDetail && "max-lg:hidden",
        )}
        data-testid="wiki-detail-pane"
      >
        {detail}
      </section>
    </div>
  );
}
