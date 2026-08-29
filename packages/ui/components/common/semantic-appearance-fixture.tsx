import type { ComponentProps } from "react";
import { AlertTriangle, Check, CircleCheck, Code2 } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";

type SemanticAppearanceFixtureProps = Omit<
  ComponentProps<"div">,
  "children"
> & {
  skin: "tension" | "relay" | "field";
  mode: "light" | "dark";
  compact?: boolean;
};

export function SemanticAppearanceFixture({
  skin,
  mode,
  compact = false,
  className,
  ...props
}: SemanticAppearanceFixtureProps) {
  return (
    <div
      data-appearance-fixture=""
      data-skin={skin}
      data-appearance-mode={mode}
      className={cn(
        "relative isolate overflow-hidden bg-background text-foreground",
        compact ? "min-h-32 p-2" : "min-h-64 p-4",
        className,
      )}
      {...props}
    >
      <div className={cn("grid", compact ? "gap-1.5" : "gap-3")}>
        <div data-fixture-role="primary-text" className="min-w-0">
          <div className="truncate text-caption font-semibold text-foreground">
            Review ready
          </div>
          <div
            data-fixture-role="muted-text"
            className="truncate text-caption text-muted-foreground"
          >
            Updated moments ago
          </div>
        </div>

        <div
          data-fixture-role="selection"
          className="flex min-w-0 items-center gap-1.5 bg-surface-selected px-2 py-1 text-caption font-medium text-surface-selected-foreground"
        >
          <Check className="size-3.5 shrink-0" aria-hidden="true" />
          <span className="truncate">Selected task</span>
        </div>

        <div className="grid grid-cols-[1fr_auto] items-center gap-2">
          <div
            data-fixture-role="form-control"
            className="h-7 min-w-0 truncate border border-control-border bg-surface px-2 py-1 text-caption text-foreground"
          >
            Assignee
          </div>
          <div
            data-fixture-role="focus"
            className="flex size-7 items-center justify-center bg-surface text-primary ring-2 ring-ring ring-offset-1 ring-offset-background"
          >
            <Check className="size-3.5" aria-hidden="true" />
          </div>
        </div>

        <div className="flex items-center justify-between gap-2 text-caption">
          <span
            data-fixture-role="success"
            className="flex items-center gap-1 text-foreground"
          >
            <CircleCheck className="size-3.5 text-success" aria-hidden="true" />
            Done
          </span>
          <span
            data-fixture-role="warning"
            className="flex items-center gap-1 text-foreground"
          >
            <AlertTriangle className="size-3.5 text-warning" aria-hidden="true" />
            Watch
          </span>
          <span
            data-fixture-role="destructive"
            className="bg-destructive px-1.5 py-0.5 font-medium text-destructive-foreground"
          >
            Remove
          </span>
        </div>

        <div
          data-fixture-role="charts"
          className="flex h-3 items-end gap-1"
          aria-hidden="true"
        >
          <span className="h-2 w-1/3 bg-[var(--chart-1)]" />
          <span className="h-3 w-1/3 bg-[var(--chart-2)]" />
          <span className="h-1.5 w-1/3 bg-[var(--chart-3)]" />
        </div>

        <div
          data-fixture-role="markdown"
          className={cn(
            "text-caption text-foreground",
            compact && "sr-only",
          )}
        >
          <strong>Summary</strong> with a <span className="text-primary underline">linked task</span>.
        </div>

        <div
          data-fixture-role="code-editor"
          className="flex min-w-0 items-center gap-1.5 bg-[var(--code-background)] px-2 py-1 font-mono text-caption text-[var(--code-foreground)] ring-1 ring-[var(--editor-selection)]"
        >
          <Code2 className="size-3 shrink-0" aria-hidden="true" />
          <code className="truncate">status = &quot;ready&quot;</code>
        </div>

        <div
          data-fixture-role="overlay"
          className={cn(
            "absolute right-2 top-2 bg-popover px-2 py-1 text-caption text-popover-foreground shadow-[var(--menu-shadow)] ring-1 ring-surface-border",
            compact ? "sr-only" : "block",
          )}
        >
          Command menu
        </div>
      </div>
    </div>
  );
}
