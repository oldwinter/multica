import {
  Focus,
  ListTree,
  Minus,
  PanelRightClose,
  Plus,
} from "lucide-react";
import { useRef } from "react";
import type { OfficeSnapshot, OfficeWorldId } from "@multica/core/office";
import { Button } from "@multica/ui/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { officeSnapshotCounts } from "./office-counts";
import type { OfficeCameraControls } from "./scene-slot";

interface OfficeToolbarProps {
  readonly snapshot: OfficeSnapshot | null;
  readonly world: OfficeWorldId;
  readonly railVisible: boolean;
  readonly cameraControls: OfficeCameraControls | null;
  readonly onWorldChange: (world: OfficeWorldId) => void;
  readonly onRailToggle: () => void;
}

function WorldOption({
  checked,
  label,
  swatchClassName,
  buttonRef,
  onClick,
}: {
  readonly checked: boolean;
  readonly label: string;
  readonly swatchClassName: string;
  readonly buttonRef: (element: HTMLButtonElement | null) => void;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      ref={buttonRef}
      role="radio"
      aria-checked={checked}
      tabIndex={checked ? 0 : -1}
      className={cn(
        "inline-flex h-8 min-w-0 items-center gap-1.5 rounded-md px-2 text-label font-medium text-muted-foreground outline-none transition-colors hover:bg-surface-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring max-md:h-11",
        checked &&
          "bg-surface-selected text-foreground hover:bg-surface-selected hover:text-foreground",
      )}
      onClick={onClick}
    >
      <span
        aria-hidden="true"
        className={cn("size-2.5 shrink-0 rounded-sm", swatchClassName)}
      />
      <span className="truncate">{label}</span>
    </button>
  );
}

function CameraButton({
  label,
  icon,
  disabled,
  onClick,
}: {
  readonly label: string;
  readonly icon: React.ReactNode;
  readonly disabled: boolean;
  readonly onClick?: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="max-md:size-11"
            aria-label={label}
            disabled={disabled}
            onClick={onClick}
          />
        }
      >
        {icon}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

export function OfficeToolbar({
  snapshot,
  world,
  railVisible,
  cameraControls,
  onWorldChange,
  onRailToggle,
}: OfficeToolbarProps) {
  const { t } = useT("office");
  const worldRefs = useRef<Record<OfficeWorldId, HTMLButtonElement | null>>({
    studio: null,
    expedition: null,
  });
  const counts = snapshot
    ? officeSnapshotCounts(snapshot)
    : {
        agents: { shown: 0, total: 0 },
        squads: { shown: 0, total: 0 },
        issues: { shown: 0, total: 0 },
      };
  const moveWorldFocus = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const worlds: readonly OfficeWorldId[] = ["studio", "expedition"];
    const currentIndex = worlds.indexOf(world);
    let targetIndex: number | null = null;
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      targetIndex = (currentIndex + 1) % worlds.length;
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      targetIndex = (currentIndex - 1 + worlds.length) % worlds.length;
    } else if (event.key === "Home") {
      targetIndex = 0;
    } else if (event.key === "End") {
      targetIndex = worlds.length - 1;
    }
    if (targetIndex === null) return;
    const target = worlds[targetIndex];
    if (!target) return;
    event.preventDefault();
    onWorldChange(target);
    worldRefs.current[target]?.focus();
  };

  return (
    <header className="flex min-h-11 shrink-0 flex-wrap items-center gap-2 border-b border-surface-border bg-background px-3 md:h-11 md:flex-nowrap">
      <h1 className="mr-1 shrink-0 text-title-sm font-semibold text-foreground">
        {t(($) => $.toolbar.title)}
      </h1>

      <div
        role="radiogroup"
        aria-label={t(($) => $.toolbar.world_control)}
        className="flex min-w-0 items-center rounded-md bg-surface-raised p-0.5"
        onKeyDown={moveWorldFocus}
      >
        <WorldOption
          checked={world === "studio"}
          label={t(($) => $.toolbar.worlds.studio)}
          swatchClassName="bg-success"
          buttonRef={(element) => {
            worldRefs.current.studio = element;
          }}
          onClick={() => onWorldChange("studio")}
        />
        <WorldOption
          checked={world === "expedition"}
          label={t(($) => $.toolbar.worlds.expedition)}
          swatchClassName="bg-warning"
          buttonRef={(element) => {
            worldRefs.current.expedition = element;
          }}
          onClick={() => onWorldChange("expedition")}
        />
      </div>

      <div
        aria-label={t(($) => $.toolbar.scene_counts)}
        className="hidden min-w-0 items-center gap-2 font-mono text-caption tabular-nums text-muted-foreground lg:flex"
      >
        <span>
          {t(($) => $.toolbar.counts.agents, {
            shown: counts.agents.shown,
            total: counts.agents.total,
          })}
        </span>
        <span aria-hidden="true">/</span>
        <span>
          {t(($) => $.toolbar.counts.squads, {
            shown: counts.squads.shown,
            total: counts.squads.total,
          })}
        </span>
        <span aria-hidden="true">/</span>
        <span>
          {t(($) => $.toolbar.counts.issues, {
            shown: counts.issues.shown,
            total: counts.issues.total,
          })}
        </span>
      </div>

      <div className="ml-auto flex shrink-0 items-center gap-0.5 max-md:basis-full max-md:justify-end">
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                type="button"
                variant={railVisible ? "brandSubtle" : "ghost"}
                size="icon"
                className="max-md:size-11"
                aria-label={t(($) => $.toolbar.roster_toggle)}
                aria-pressed={railVisible}
                onClick={onRailToggle}
              />
            }
          >
            {railVisible ? <PanelRightClose /> : <ListTree />}
          </TooltipTrigger>
          <TooltipContent>{t(($) => $.toolbar.roster_toggle)}</TooltipContent>
        </Tooltip>
        <CameraButton
          label={t(($) => $.toolbar.fit)}
          icon={<Focus />}
          disabled={!cameraControls}
          onClick={cameraControls?.fit}
        />
        <CameraButton
          label={t(($) => $.toolbar.zoom_out)}
          icon={<Minus />}
          disabled={!cameraControls}
          onClick={cameraControls?.zoomOut}
        />
        <CameraButton
          label={t(($) => $.toolbar.zoom_in)}
          icon={<Plus />}
          disabled={!cameraControls}
          onClick={cameraControls?.zoomIn}
        />
      </div>
    </header>
  );
}
