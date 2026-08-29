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
  readonly variants: readonly (readonly string[])[];
  readonly fps: number;
  readonly loop: boolean;
}

interface OfficeDecorBase {
  readonly color: number;
}

export type OfficeDecorElement =
  | (OfficeDecorBase & {
      readonly kind: "rect";
      readonly x: number;
      readonly y: number;
      readonly width: number;
      readonly height: number;
    })
  | (OfficeDecorBase & {
      readonly kind: "circle";
      readonly x: number;
      readonly y: number;
      readonly radius: number;
    })
  | (OfficeDecorBase & {
      readonly kind: "polygon";
      readonly points: readonly number[];
    })
  | (OfficeDecorBase & {
      readonly kind: "line";
      readonly points: readonly number[];
      readonly width: number;
    });

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
    readonly backdropColor: number;
    readonly decor: readonly OfficeDecorElement[];
  };
  readonly hitRegions: readonly {
    readonly role: "agent" | "squad" | "issue";
    readonly polygon: readonly number[];
  }[];
  readonly provenance: string;
}
