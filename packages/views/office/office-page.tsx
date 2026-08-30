"use client";

import { useCallback, useEffect, useRef, useState } from "react";
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
  const requestedWorldRef = useRef(persistedWorld);
  useEffect(() => {
    requestedWorldRef.current = persistedWorld;
    setWorld(persistedWorld);
  }, [persistedWorld, wsId]);
  const handleWorldChange = useCallback((nextWorld: OfficeWorldId) => {
    requestedWorldRef.current = nextWorld;
    setWorld(nextWorld);
  }, []);
  const persistLoadedWorld = useCallback((readyWorld: OfficeWorldId) => {
    if (requestedWorldRef.current !== readyWorld) return;
    setWorld(readyWorld);
    const state = useOfficeViewStore.getState();
    if (state.world !== readyWorld) state.setWorld(readyWorld);
  }, []);
  const handleWorldReady = useCallback(
    (readyWorld: OfficeWorldId) => persistLoadedWorld(readyWorld),
    [persistLoadedWorld],
  );
  const handlePosterReady = useCallback(
    (readyWorld: OfficeWorldId) => persistLoadedWorld(readyWorld),
    [persistLoadedWorld],
  );
  const handleWorldSwitchFailure = useCallback(
    (retainedWorld: OfficeWorldId) => {
      requestedWorldRef.current = retainedWorld;
      setWorld(retainedWorld);
    },
    [],
  );
  const handlePosterError = useCallback((failedWorld: OfficeWorldId) => {
    if (requestedWorldRef.current !== failedWorld) return;
    const retainedWorld = useOfficeViewStore.getState().world;
    requestedWorldRef.current = retainedWorld;
    setWorld(retainedWorld);
  }, []);

  return (
    <OfficeSurface
      model={model}
      world={world}
      selected={selected}
      onSelect={setSelected}
      onWorldChange={handleWorldChange}
      onWorldReady={handleWorldReady}
      onWorldSwitchFailure={handleWorldSwitchFailure}
      onPosterReady={handlePosterReady}
      onPosterError={handlePosterError}
      SceneSlot={SceneSlot ?? OfficeSceneBridge}
    />
  );
}
