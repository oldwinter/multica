import type {
  OfficeAgent,
  OfficeIssue,
  OfficeSnapshot,
  OfficeSubjectRef,
  OfficeWorldId,
} from "@multica/core/office";
import type {
  OfficeMotionClipId,
  OfficePoint,
  OfficeWorldPack,
} from "../worlds/types";

export type OfficeEffect =
  | {
      readonly kind: "task-queued";
      readonly taskId: string;
      readonly agentId: OfficeAgent["id"];
      readonly issueId: OfficeIssue["id"] | null;
    }
  | {
      readonly kind: "task-started";
      readonly taskId: string;
      readonly agentId: OfficeAgent["id"];
      readonly issueId: OfficeIssue["id"] | null;
    }
  | {
      readonly kind: "task-finished";
      readonly taskId: string;
      readonly agentId: OfficeAgent["id"];
      readonly issueId: OfficeIssue["id"] | null;
      readonly outcome: "completed" | "failed";
    };

export interface OfficeSceneCommit {
  readonly world: OfficeWorldId;
  readonly snapshot: OfficeSnapshot;
  readonly selected: OfficeSubjectRef | null;
  readonly selectedSquadAgentIds: readonly OfficeAgent["id"][];
  readonly mode: "replace" | "transition";
  readonly effects: readonly OfficeEffect[];
  readonly reducedMotion: boolean;
}

export type OfficeRendererStatus =
  | { readonly kind: "ready"; readonly world: OfficeWorldId }
  | { readonly kind: "recovering" }
  | {
      readonly kind: "world-switch-failed";
      readonly attemptedWorld: OfficeWorldId;
      readonly retainedWorld: OfficeWorldId;
    }
  | {
      readonly kind: "fallback";
      readonly reason: "unsupported" | "asset" | "context";
    };

export interface OfficeAgentSceneState {
  readonly availability: "online" | "unstable" | "offline" | "unknown";
  readonly workload: "idle" | "queued" | "working" | "unknown";
  readonly runningCount: number;
  readonly queuedCount: number;
  readonly clip: OfficeMotionClipId;
  readonly stationLit: boolean;
  readonly ambientMotion: boolean;
  readonly pulse: boolean;
  readonly flicker: boolean;
}

export interface OfficeAgentVisualVariant {
  readonly accent: number;
  readonly body: number;
  readonly silhouette: number;
}

interface OfficeSceneEntityBase {
  readonly key: string;
  readonly id: string;
  readonly anchor: OfficePoint;
  readonly highlighted: boolean;
}

export interface OfficeAgentSceneEntity extends OfficeSceneEntityBase {
  readonly kind: "agent";
  readonly state: OfficeAgentSceneState;
  readonly visualVariant: OfficeAgentVisualVariant;
}

export interface OfficeSquadSceneEntity extends OfficeSceneEntityBase {
  readonly kind: "squad";
  readonly memberCount: number;
  readonly previewCount: number;
}

export interface OfficeIssueSceneEntity extends OfficeSceneEntityBase {
  readonly kind: "issue";
  readonly resolved: boolean;
}

export interface OfficeOverflowSceneEntity extends OfficeSceneEntityBase {
  readonly kind: "overflow";
  readonly subjectKind: "agent" | "squad" | "issue";
  readonly count: number;
}

export type OfficeSceneEntity =
  | OfficeAgentSceneEntity
  | OfficeSquadSceneEntity
  | OfficeIssueSceneEntity
  | OfficeOverflowSceneEntity;

export interface OfficeSceneLink {
  readonly from: string;
  readonly to: string;
  readonly kind: "execution" | "assignment";
}

export interface OfficeScenePlan {
  readonly world: OfficeWorldId;
  readonly layoutVersion: number;
  readonly width: number;
  readonly height: number;
  readonly reducedMotion: boolean;
  readonly entities: readonly OfficeSceneEntity[];
  readonly links: readonly OfficeSceneLink[];
}

export interface OfficeScenePort {
  installWorld(pack: OfficeWorldPack): Promise<void>;
  apply(plan: OfficeScenePlan): void;
  cancelEffects(): void;
  playEffects(effects: readonly OfficeEffect[]): void;
  pause(): void;
  resume(): void;
  fit(): void;
  zoomIn(): void;
  zoomOut(): void;
  rebuild(): Promise<void>;
  onContextLoss(handler: () => void): () => void;
  destroy(): void;
}

export interface OfficeSceneHandle {
  reconcile(commit: OfficeSceneCommit): void;
  fit(): void;
  zoomIn(): void;
  zoomOut(): void;
  destroy(): void;
}
