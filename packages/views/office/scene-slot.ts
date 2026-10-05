import type { ComponentType } from "react";
import type {
  OfficeAgent,
  OfficeSnapshot,
  OfficeSubjectRef,
  OfficeWorldId,
} from "@multica/core/office";

export interface OfficeCameraControls {
  readonly fit: () => void;
  readonly zoomIn: () => void;
  readonly zoomOut: () => void;
}

export interface OfficeSceneSlotProps {
  readonly snapshot: OfficeSnapshot;
  readonly world: OfficeWorldId;
  readonly selected: OfficeSubjectRef | null;
  readonly selectedSquadAgentIds: readonly OfficeAgent["id"][];
  readonly reducedMotion: boolean;
  readonly motionFrozen: boolean;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly onCameraControlsChange: (
    controls: OfficeCameraControls | null,
  ) => void;
  readonly onRendererFallback: () => void;
  readonly onWorldReady: (world: OfficeWorldId) => void;
  readonly onWorldSwitchFailure: (retainedWorld: OfficeWorldId) => void;
}

export type OfficeSceneSlot = ComponentType<OfficeSceneSlotProps>;
