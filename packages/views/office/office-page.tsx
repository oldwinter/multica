"use client";

import { useCallback, useState } from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  useOfficeModel,
  useOfficeViewStore,
  type OfficeSubjectRef,
  type OfficeWorldId,
} from "@multica/core/office";
import { OfficeSurface } from "./office-surface";
import type { OfficeSceneSlot } from "./scene-slot";

export function OfficePage({ SceneSlot }: { readonly SceneSlot?: OfficeSceneSlot }) {
  const wsId = useWorkspaceId();
  const [selected, setSelected] = useState<OfficeSubjectRef | null>(null);
  const model = useOfficeModel({ wsId, selected });
  const world = useOfficeViewStore((state) => state.world);
  const handleWorldChange = useCallback((nextWorld: OfficeWorldId) => {
    useOfficeViewStore.getState().setWorld(nextWorld);
  }, []);

  return (
    <OfficeSurface
      model={model}
      world={world}
      selected={selected}
      onSelect={setSelected}
      onWorldChange={handleWorldChange}
      SceneSlot={SceneSlot}
    />
  );
}
