import type {
  Agent,
  AgentRuntime,
  AgentTask,
  Issue,
  IssueStatusCategory,
  Squad,
} from "../types";

export const OFFICE_WORLD_IDS = ["studio", "expedition"] as const;
export type OfficeWorldId = (typeof OFFICE_WORLD_IDS)[number];

export const OFFICE_LIMITS = {
  agents: 40,
  squads: 12,
  activeIssues: 48,
  issueBriefs: 100,
} as const;

export interface OfficeLimits {
  readonly agents: number;
  readonly squads: number;
  readonly activeIssues: number;
  readonly issueBriefs: number;
}

export type OfficeSubjectRef =
  | { readonly kind: "agent"; readonly id: Agent["id"] }
  | { readonly kind: "squad"; readonly id: Squad["id"] }
  | { readonly kind: "issue"; readonly id: Issue["id"] };

export type OfficeAvailability =
  | {
      readonly kind: "known";
      readonly value: "online" | "unstable" | "offline";
    }
  | {
      readonly kind: "unknown";
      readonly reason: "loading" | "unavailable";
    };

export type OfficeWorkload =
  | {
      readonly kind: "known";
      readonly value: "idle" | "queued" | "working";
      readonly runningCount: number;
      readonly queuedCount: number;
      readonly capacity: number;
    }
  | {
      readonly kind: "unknown";
      readonly reason: "loading" | "unavailable";
      readonly capacity: number;
    };

export interface OfficeAgent {
  readonly id: Agent["id"];
  readonly name: Agent["name"];
  readonly avatarUrl: Agent["avatar_url"];
  readonly description: Agent["description"];
  readonly availability: OfficeAvailability;
  readonly workload: OfficeWorkload;
  readonly activeIssueIds: readonly Issue["id"][];
}

export interface OfficeSquadMemberPreview {
  readonly kind: "agent" | "member" | "unknown";
  readonly id: string;
  readonly role: string;
}

export interface OfficeSquad {
  readonly id: Squad["id"];
  readonly name: Squad["name"];
  readonly description: Squad["description"];
  readonly avatarUrl: Squad["avatar_url"];
  readonly leaderAgentId: Agent["id"];
  readonly memberCount: number;
  readonly memberPreview: readonly OfficeSquadMemberPreview[];
}

export type OfficeIssue =
  | {
      readonly kind: "resolved";
      readonly id: Issue["id"];
      readonly identifier: Issue["identifier"];
      readonly title: Issue["title"];
      readonly status: Issue["status"];
      readonly statusCategory: IssueStatusCategory | null;
      readonly assignedSquadId: Squad["id"] | null;
      readonly executingAgentIds: readonly Agent["id"][];
    }
  | {
      readonly kind: "unresolved";
      readonly id: Issue["id"];
      readonly reason:
        | "loading"
        | "unavailable"
        | "not-returned"
        | "brief-limit";
      readonly executingAgentIds: readonly Agent["id"][];
    };

export interface OfficeSnapshot {
  readonly agents: readonly OfficeAgent[];
  readonly squads: readonly OfficeSquad[];
  readonly activeIssues: readonly OfficeIssue[];
  readonly overflow: {
    readonly agents: number;
    readonly squads: number;
    readonly activeIssues: number;
  };
}

export type OfficeSquadMembers =
  | { readonly kind: "loading" }
  | {
      readonly kind: "ready";
      readonly members: readonly {
        readonly kind: "agent" | "member" | "unknown";
        readonly id: string;
        readonly name: string | null;
        readonly activeIssueIds: readonly Issue["id"][];
      }[];
    }
  | {
      readonly kind: "unavailable";
      readonly retry: () => Promise<void>;
    };

export type OfficeInspector =
  | { readonly kind: "closed" }
  | { readonly kind: "agent"; readonly agent: OfficeAgent }
  | {
      readonly kind: "squad";
      readonly squad: OfficeSquad;
      readonly members: OfficeSquadMembers;
    }
  | { readonly kind: "issue"; readonly issue: OfficeIssue }
  | {
      readonly kind: "missing";
      readonly subject: OfficeSubjectRef;
    };

export type OfficeDataGap =
  | "availability"
  | "workload"
  | "squads"
  | "issue-briefs"
  | "selected-squad";

export type OfficeModel =
  | { readonly kind: "loading" }
  | {
      readonly kind: "unavailable";
      readonly retry: () => Promise<void>;
    }
  | {
      readonly kind: "ready";
      readonly snapshot: OfficeSnapshot;
      readonly quality:
        | { readonly kind: "current"; readonly refreshing: boolean }
        | {
            readonly kind: "partial" | "stale";
            readonly gaps: readonly OfficeDataGap[];
          };
      readonly inspector: OfficeInspector;
      readonly retry: () => Promise<void>;
    };

export type OfficeSource<T> =
  | { readonly kind: "available"; readonly value: T }
  | {
      readonly kind: "unavailable";
      readonly reason?: "loading" | "unavailable";
    };

export interface DeriveOfficeSnapshotInput {
  readonly nowMs: number;
  readonly agents: readonly Agent[];
  readonly runtimes: OfficeSource<readonly AgentRuntime[]>;
  readonly tasks: OfficeSource<readonly AgentTask[]>;
  readonly squads: OfficeSource<readonly Squad[]>;
  readonly issueBriefs: OfficeSource<readonly Issue[]>;
  readonly limits: Pick<
    OfficeLimits,
    "agents" | "squads" | "activeIssues"
  >;
}
