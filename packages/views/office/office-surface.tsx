import { useEffect, useState } from "react";
import type {
  OfficeModel,
  OfficeSubjectRef,
  OfficeWorldId,
} from "@multica/core/office";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from "@multica/ui/components/ui/sheet";
import { useIsCompact, useIsMobile } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { OfficeDomFallback } from "./office-fallback";
import {
  OfficeLoadingState,
  OfficeQualityNotice,
  OfficeUnavailableState,
} from "./office-state";
import { OfficeRail } from "./office-rail";
import { OfficeToolbar } from "./office-toolbar";
import type {
  OfficeCameraControls,
  OfficeSceneSlot,
} from "./scene-slot";
import { useReducedMotion } from "./use-reduced-motion";

export interface OfficeSurfaceProps {
  readonly model: OfficeModel;
  readonly world: OfficeWorldId;
  readonly selected: OfficeSubjectRef | null;
  readonly onSelect: (subject: OfficeSubjectRef | null) => void;
  readonly onWorldChange: (world: OfficeWorldId) => void;
  readonly onWorldReady: (world: OfficeWorldId) => void;
  readonly onWorldSwitchFailure: (retainedWorld: OfficeWorldId) => void;
  readonly SceneSlot?: OfficeSceneSlot;
}

export function OfficeSurface({
  model,
  world,
  selected,
  onSelect,
  onWorldChange,
  onWorldReady,
  onWorldSwitchFailure,
  SceneSlot,
}: OfficeSurfaceProps) {
  const { t } = useT("office");
  const isNarrow = useIsMobile();
  const isCompact = useIsCompact();
  const reducedMotion = useReducedMotion();
  const [railVisible, setRailVisible] = useState(true);
  const [cameraControls, setCameraControls] =
    useState<OfficeCameraControls | null>(null);
  const [rendererFallback, setRendererFallback] = useState(false);

  useEffect(() => {
    setRendererFallback(false);
  }, [SceneSlot, world]);

  useEffect(() => {
    if (selected) setRailVisible(true);
  }, [selected]);

  const snapshot = model.kind === "ready" ? model.snapshot : null;
  const selectedSquadAgentIds =
    model.kind === "ready" &&
    model.inspector.kind === "squad" &&
    model.inspector.members.kind === "ready"
      ? model.inspector.members.members.flatMap((member) =>
          member.kind === "agent" ? [member.id] : [],
        )
      : [];

  return (
    <main
      data-reduced-motion={reducedMotion}
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-background text-foreground"
    >
      <OfficeToolbar
        snapshot={snapshot}
        world={world}
        railVisible={railVisible}
        cameraControls={cameraControls}
        onWorldChange={onWorldChange}
        onRailToggle={() => setRailVisible((visible) => !visible)}
      />

      {model.kind === "loading" ? <OfficeLoadingState /> : null}
      {model.kind === "unavailable" ? (
        <OfficeUnavailableState retry={model.retry} />
      ) : null}
      {model.kind === "ready" ? (
        <OfficeQualityNotice quality={model.quality} />
      ) : null}
      {model.kind === "ready" ? (
        <div className="flex min-h-0 flex-1 flex-col md:flex-row">
          <div className="flex min-h-0 min-w-0 flex-1 flex-col">
            {SceneSlot && !isNarrow && !rendererFallback ? (
              <div
                aria-hidden="true"
                className="min-h-0 flex-1 overflow-hidden bg-surface"
                data-testid="office-scene-slot"
              >
                <SceneSlot
                  snapshot={model.snapshot}
                  world={world}
                  selected={selected}
                  selectedSquadAgentIds={selectedSquadAgentIds}
                  reducedMotion={reducedMotion}
                  motionFrozen={model.quality.kind !== "current"}
                  onSelect={(subject) => onSelect(subject)}
                  onCameraControlsChange={setCameraControls}
                  onRendererFallback={() => {
                    setCameraControls(null);
                    setRendererFallback(true);
                  }}
                  onWorldReady={onWorldReady}
                  onWorldSwitchFailure={onWorldSwitchFailure}
                />
              </div>
            ) : (
              <OfficeDomFallback
                snapshot={model.snapshot}
                world={world}
                reason={
                  isNarrow ? "narrow" : rendererFallback ? "renderer" : "slot"
                }
              />
            )}
          </div>

          {isCompact && !isNarrow ? (
            <Sheet open={railVisible} onOpenChange={setRailVisible}>
              <SheetContent
                side="right"
                showCloseButton={false}
                className="w-[300px] max-w-[90vw] gap-0 p-0"
              >
                <SheetTitle className="sr-only">
                  {t(($) => $.roster.title)}
                </SheetTitle>
                <SheetDescription className="sr-only">
                  {t(($) => $.toolbar.roster_toggle)}
                </SheetDescription>
                <aside
                  aria-label={t(($) => $.roster.title)}
                  className="flex h-full min-h-0 flex-col bg-surface"
                >
                  <OfficeRail
                    snapshot={model.snapshot}
                    inspector={model.inspector}
                    selected={selected}
                    narrow={false}
                    onSelect={onSelect}
                  />
                </aside>
              </SheetContent>
            </Sheet>
          ) : (
            <aside
              aria-label={t(($) => $.roster.title)}
              className={cn(
                "min-h-0 border-t border-surface-border bg-surface md:w-[300px] md:shrink-0 md:border-l md:border-t-0",
                railVisible ? "block" : "hidden",
              )}
            >
              <OfficeRail
                snapshot={model.snapshot}
                inspector={model.inspector}
                selected={selected}
                narrow={isNarrow}
                onSelect={onSelect}
              />
            </aside>
          )}
        </div>
      ) : null}
    </main>
  );
}
