import type { OfficeWorldId } from "@multica/core/office";

export const OFFICE_WORLD_CONTRACT_VERSION = 1 as const;

export const REQUIRED_MOTION_CLIPS = [
  "idle",
  "wait",
  "work",
  "walk",
  "unstable",
  "offline",
  "completion",
  "failure",
] as const;

export type OfficeMotionClipId = (typeof REQUIRED_MOTION_CLIPS)[number];

export interface OfficePoint {
  readonly id: string;
  readonly x: number;
  readonly y: number;
}

export interface OfficeFrame {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface OfficeMotionClip {
  readonly frames: readonly string[];
  readonly fps: number;
  readonly loop: boolean;
}

export interface OfficeWorldPack {
  readonly id: OfficeWorldId;
  readonly contractVersion: typeof OFFICE_WORLD_CONTRACT_VERSION;
  readonly layoutVersion: number;
  readonly map: {
    readonly asset: string;
    readonly width: number;
    readonly height: number;
    readonly tileSize: number;
    readonly layers: readonly {
      readonly name: string;
      readonly type: string;
    }[];
  };
  readonly assets: {
    readonly atlas: string;
    readonly poster: string;
    readonly atlasSize: { readonly width: number; readonly height: number };
    readonly posterSize: { readonly width: number; readonly height: number };
    readonly frames: Readonly<Record<string, OfficeFrame>>;
  };
  readonly anchors: {
    readonly agentStations: readonly OfficePoint[];
    readonly squadBoards: readonly OfficePoint[];
    readonly activeIssues: readonly OfficePoint[];
    readonly dispatch: readonly OfficePoint[];
    readonly overflow: readonly OfficePoint[];
    readonly camera: readonly OfficePoint[];
  };
  readonly clips: Readonly<Record<OfficeMotionClipId, OfficeMotionClip>>;
  readonly palette: readonly string[];
  readonly lighting: {
    readonly light: { readonly ambient: string; readonly overlayAlpha: number };
    readonly dark: { readonly ambient: string; readonly overlayAlpha: number };
  };
  readonly visuals: {
    readonly actorSilhouette: string;
    readonly stationStyle: string;
    readonly props: readonly string[];
  };
  readonly hitRegions: readonly {
    readonly role: "agent" | "squad" | "issue";
    readonly polygon: readonly number[];
  }[];
  readonly provenance: string;
}
