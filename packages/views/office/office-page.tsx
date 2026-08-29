"use client";

import { useCallback, useEffect, useState } from "react";
import type { ReactElement } from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  useOfficeModel,
  useOfficeViewStore,
  type OfficeSubjectRef,
  type OfficeWorldId,
} from "@multica/core/office";
import { OfficeSurface } from "./office-surface";
import { OfficeSceneBridge } from "./office-scene-bridge";
import type { OfficeSceneSlot } from "./scene-slot";

interface OfficePageProps {
  readonly SceneSlot?: OfficeSceneSlot;
}

export function OfficePage(props: OfficePageProps): ReactElement;
export function OfficePage(): ReactElement;
export function OfficePage({ SceneSlot }: OfficePageProps = {}) {
  const wsId = useWorkspaceId();
  const [selected, setSelected] = useState<OfficeSubjectRef | null>(null);
  const model = useOfficeModel({ wsId, selected });
  const persistedWorld = useOfficeViewStore((state) => state.world);
  const [world, setWorld] = useState(persistedWorld);
  useEffect(() => setWorld(persistedWorld), [persistedWorld, wsId]);
  const handleWorldChange = useCallback((nextWorld: OfficeWorldId) => {
    setWorld(nextWorld);
  }, []);
  const handleWorldReady = useCallback((readyWorld: OfficeWorldId) => {
    setWorld(readyWorld);
    const state = useOfficeViewStore.getState();
    if (state.world !== readyWorld) state.setWorld(readyWorld);
  }, []);
  const handleWorldSwitchFailure = useCallback(
    (retainedWorld: OfficeWorldId) => setWorld(retainedWorld),
    [],
  );

  return (
    <OfficeSurface
      model={model}
      world={world}
      selected={selected}
      onSelect={setSelected}
      onWorldChange={handleWorldChange}
      onWorldReady={handleWorldReady}
      onWorldSwitchFailure={handleWorldSwitchFailure}
      SceneSlot={SceneSlot ?? OfficeSceneBridge}
    />
  );
}
