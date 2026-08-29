import type { ComponentType } from "react";
import type {
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
  readonly reducedMotion: boolean;
  readonly motionFrozen: boolean;
  readonly onSelect: (subject: OfficeSubjectRef) => void;
  readonly onCameraControlsChange: (
    controls: OfficeCameraControls | null,
  ) => void;
  readonly onRendererFallback: () => void;
}

export type OfficeSceneSlot = ComponentType<OfficeSceneSlotProps>;
