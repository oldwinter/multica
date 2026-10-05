# Dynamic pixel office

Triage: ready-for-human

Design date: 2026-08-29

## Decision

Multica will add a workspace page at /{slug}/office. Web and Desktop use the
same page from packages/views. The normal workspace shell, sidebar, Desktop tab
model, search, and NavigationAdapter remain in place.

The Office is a read-only spatial view of existing Agents, Squads, active
Issues, runtimes, and Agent tasks. It owns no business mutations. Selecting an
Agent, Squad, or active Issue opens a compact inspector. The inspector links to
the existing detail page.

V1 has two scene worlds:

- studio is an original modern pixel workplace.
- expedition is an original fantasy-creature field base. It uses no Palworld
  names, characters, silhouettes, maps, UI, audio, or assets.

The scene uses a lazy-loaded PixiJS renderer over WebGL. A synchronized DOM
roster and inspector remain the complete accessible and failure-safe
experience. A renderer failure must never leave a blank page.

The client composes existing bounded reads. V1 adds no Office API, database
table, migration, WebSocket event, terminal, chat system, or task state store.

## Product contract

The page is for a workspace member who wants to answer four questions without
opening several operational pages:

1. Which Agents can work now?
2. Which Agents are working or waiting?
3. Which active Issues are involved?
4. Which Squads are relevant to the current work?

Success is a page that makes those answers visible within a few seconds of the
existing Query caches updating. The scene may be playful, but every status,
relationship, count, and one-shot effect must have an authoritative source.

The visitor mode is Operate. Office is an overview for repeated use, not a
landing page, game, or decorative screen saver.

### V1 boundaries

V1 includes:

- one workspace route shared by Web and Desktop;
- two original scene worlds;
- real Agent availability and workload;
- active task-linked Issues;
- Squad boards, selected-Squad membership highlighting, and Issue assignment
  links;
- a compact toolbar, one roster or inspector rail, and camera controls;
- workspace-aware, device-local world persistence;
- responsive DOM fallback, reduced motion, strict-CSP support, and WebGL
  recovery.

V1 excludes:

- create, edit, assign, archive, cancel, run, chat, or command controls;
- raw task messages, prompts, tool calls, transcripts, progress text, paths,
  results, errors, and runtime configuration;
- permanent Squad rooms or a primary Squad for each Agent;
- human presence animation, because Multica has no authoritative human presence
  signal;
- audio, multiplayer cursors, shared cameras, physics, collisions between
  actors, inventory, scores, or game rules;
- native Mobile rendering and a third scene world;
- server-side Office preferences or an Office projection endpoint.

## Experience

### Page composition

The Office fills the existing page canvas. It does not add another application
shell or floating page card.

At wide widths, the page has:

1. A 44px toolbar with the page title, Studio and Expedition segmented control,
   exact scene counts, roster toggle, Zoom Out, Zoom In, and Fit controls.
2. One full-bleed scene region.
3. One 300px right rail. The rail shows the roster by default. Selection
   replaces the roster with the inspector. A Back control returns to the same
   roster tab and row.

At medium widths, the right rail becomes a Sheet over the scene. Below 768px,
Pixi does not mount. The selected world's poster, complete roster, health
notice, and inspector remain usable.

The toolbar changes only view presentation. It contains no filters that change
server state and no text that teaches the feature.

### The authored moments

Motion uses a small semantic vocabulary:

- A newly observed queued task creates one work token at the dispatch point.
- A continuously observed transition from queued-like to running moves the
  token to the Agent station and lights the workstation.
- A continuously observed terminal row with the same task ID may play one
  short completion or failure effect.
- Selecting a Squad loads its full member status and highlights the visible
  member stations without moving them.
- Selecting an active Issue highlights the executing Agents and, when the Issue
  is assigned to a Squad, its Squad board.
- Switching worlds rebuilds the same facts through different art and motion.
  It never replays task history.

Studio represents work as dispatch packets, desk lights, and project-board
signals. Expedition represents the same facts as field dispatches, workbench
activity, route maps, and signal beacons.

Ambient movement is bounded to an Agent's local station zone. It carries no
business meaning. The scene has no simulated conversations, prompt bubbles, or
tool-specific travel. Multica does not expose a safe, authoritative current-tool
signal without reading high-frequency task messages.

### Subject behavior

Agents have stable home stations independent of Squad membership. Availability
and workload stay visually separate. An offline Agent with queued work shows
both facts.

Squads own boards or expedition standards, not seats. The default board shows
the leader, complete member count, and an explicitly labeled preview of up to
three members. It never treats the preview as complete membership.

Active Issues are Issues linked from nonterminal Agent tasks. One Issue marker
may connect to several executing Agents. A line to a Squad board appears only
when the resolved Issue assignee is that Squad. The line means assignment, not
membership.

Tasks without an Issue use a generic source token. Office does not invent an
Issue link or reveal prompt text.

### Visual direction

Studio uses a top-down 16-bit visual language with concrete, graphite, green,
coral, amber, and cool metal. It borrows Multica's structural restraint without
turning the global Tension, Relay, or Field skin into pixel art.

Expedition uses moss, basalt, cobalt canvas, coral markers, stone, field tables,
and original geometric companion creatures. In this world the creatures
represent Agents. They are not an additional domain entity.

The worlds differ in map composition, actor silhouettes, props, palettes,
stations, and motion clips. A palette swap does not satisfy the second-world
requirement.

Product chrome, text, focus, selection, errors, and controls always use Multica
semantic tokens. Scene palettes stay inside the canvas art.

## Data truth

The scene translates existing product facts. It does not interpret names,
descriptions, instructions, or messages to guess behavior.

| Visible concept | Authoritative source | Scene use | Inspector use |
| --- | --- | --- | --- |
| Agent identity | agentListOptions | Stable actor and station | Name, avatar, description, detail link |
| Availability | Agent plus runtime list through existing availability derivation | Online, unstable, offline, or unknown marker | Text and icon |
| Workload | Agent task snapshot through existing workload derivation | Idle, queued, working, or unknown pose | Running and queued counts |
| Active task | Agent task snapshot | Work token and transition proof | Task-linked Issue references only |
| Active Issue detail | One restricted listIssues ids query | Identifier/status only when resolved | Title, status, assignee, detail link |
| Squad summary | squadListOptions | Board, leader, complete count, preview | Summary and detail link |
| Full Squad membership | Selected Squad member-status query | Temporary exact highlights | Full selected-Squad member state |
| Human member | Workspace member list when selected Squad needs names | No presence animation | Name only, no status |
| Realtime freshness | Existing useRealtimeSync invalidation and Query state | Reconcile after settled cache updates | Refreshing, stale, or partial notice |

AgentTask.issue_id is a required string in the current TypeScript type. An empty
string means no linked Issue. The Office model converts that sentinel to no
Issue before it reaches the view.

Archived Agents are excluded from the live scene. A stale runtime row or task
cannot make an archived Agent look active.

## Caller usage

The apps register one page. They do not know about queries, Pixi, packs, or
Office preference storage.

~~~tsx
// apps/web/app/[workspaceSlug]/(dashboard)/office/page.tsx
export { OfficePage as default } from "@multica/views/office";
~~~

~~~tsx
// apps/desktop/src/renderer/src/routes.tsx
import { OfficePage } from "@multica/views/office";

{
  path: "office",
  element: <OfficePage />,
  handle: { title: "Office" },
}
~~~

The shared page resolves the workspace under the existing provider. Its hooks
still receive wsId explicitly.

~~~tsx
// packages/views/office/office-page.tsx
export function OfficePage() {
  const wsId = useWorkspaceId();
  const [selected, setSelected] = useState<OfficeSubjectRef | null>(null);
  const model = useOfficeModel({ wsId, selected });
  const world = useOfficeViewStore((state) => state.world);

  return (
    <OfficeSurface
      model={model}
      world={world}
      selected={selected}
      onSelect={setSelected}
    />
  );
}
~~~

The inspector uses AppLink and useWorkspacePaths. It does not receive a Next.js
router, React Router object, or app-supplied navigation callback.

## Type sketch

The public model uses existing domain ID types. It does not introduce
Office-only branded IDs that require casts at every boundary.

~~~ts
export const OFFICE_WORLD_IDS = ["studio", "expedition"] as const;
export type OfficeWorldId = (typeof OFFICE_WORLD_IDS)[number];

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

export interface OfficeSquad {
  readonly id: Squad["id"];
  readonly name: Squad["name"];
  readonly description: Squad["description"];
  readonly avatarUrl: Squad["avatar_url"];
  readonly leaderAgentId: Agent["id"];
  readonly memberCount: number;
  readonly memberPreview: readonly {
    readonly kind: "agent" | "member" | "unknown";
    readonly id: string;
    readonly role: string;
  }[];
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

type OfficeSource<T> =
  | { readonly kind: "available"; readonly value: T }
  | { readonly kind: "unavailable" };

interface DeriveOfficeSnapshotInput {
  readonly nowMs: number;
  readonly agents: readonly Agent[];
  readonly runtimes: OfficeSource<readonly AgentRuntime[]>;
  readonly tasks: OfficeSource<readonly AgentTask[]>;
  readonly squads: OfficeSource<readonly Squad[]>;
  readonly issueBriefs: OfficeSource<readonly Issue[]>;
  readonly limits: {
    readonly agents: number;
    readonly squads: number;
    readonly activeIssues: number;
  };
}

export function useOfficeModel(input: {
  readonly wsId: string;
  readonly selected: OfficeSubjectRef | null;
}): OfficeModel {
  throw new Error("not implemented");
}

export function buildOfficeSnapshot(
  input: DeriveOfficeSnapshotInput,
): OfficeSnapshot {
  throw new Error("not implemented");
}
~~~

Continuity and visual effects stay private to the Office leaf. They are not part
of OfficeSnapshot.

~~~ts
type OfficeEffect =
  | {
      readonly kind: "task-queued";
      readonly taskId: AgentTask["id"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
    }
  | {
      readonly kind: "task-started";
      readonly taskId: AgentTask["id"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
    }
  | {
      readonly kind: "task-finished";
      readonly taskId: AgentTask["id"];
      readonly agentId: Agent["id"];
      readonly issueId: Issue["id"] | null;
      readonly outcome: "completed" | "failed";
    };

type OfficeSceneCommit = {
  readonly world: OfficeWorldId;
  readonly snapshot: OfficeSnapshot;
  readonly selected: OfficeSubjectRef | null;
  readonly mode: "replace" | "transition";
  readonly effects: readonly OfficeEffect[];
  readonly reducedMotion: boolean;
};

type OfficeRendererStatus =
  | { readonly kind: "ready" }
  | { readonly kind: "recovering" }
  | {
      readonly kind: "fallback";
      readonly reason: "unsupported" | "asset" | "context";
    };

interface OfficeSceneHandle {
  reconcile(commit: OfficeSceneCommit): void;
  destroy(): void;
}

async function createOfficeScene(input: {
  host: HTMLElement;
  onSelect(subject: OfficeSubjectRef): void;
  onStatus(status: OfficeRendererStatus): void;
}): Promise<OfficeSceneHandle> {
  throw new Error("not implemented");
}
~~~

This interface is deep. The caller does not coordinate camera movement,
selection reveal, pack loading, context recovery, sprite allocation, or Pixi
lifecycle. Reconcile owns all consequences of one semantic commit.

## Query plan

The workspace shell already warms the four primary caches through
WorkspacePresencePrefetch:

- Agents;
- runtimes;
- Agent task snapshot;
- Squads.

Office subscribes to those same keys. It does not add four duplicate network
requests.

After the task snapshot settles, Office extracts unique nonempty Issue IDs from
queued, dispatched, waiting_local_directory, and running tasks. It performs one
restricted listIssues request for up to 100 Issue briefs. The request uses the
existing POST query path when the IDs do not fit safely in a URL.

The returned list is filtered again against the requested ID set. This protects
an installed client talking to an older backend that ignores an unknown ids
filter. A missing row remains an unresolved active Issue. It never borrows the
title or status of an unrelated row.

The active-Issue query has an Office-owned key:

~~~ts
officeKeys.issueBriefsAll(wsId)
officeKeys.issueBriefs(wsId, sortedIssueIds)
~~~

Implementation adds targeted Office-brief invalidation to the existing
issue:created, issue:updated, and issue:deleted handlers. Reconnect also
invalidates the Office query tree. Task lifecycle events already invalidate the
Agent task snapshot, so a changed active ID set naturally creates a new brief
query.

Before Office ships, getAgentTaskSnapshot must parse its network response with
the existing AgentTaskListSchema and parseWithFallback. This is required by the
repository's installed-Desktop compatibility rule. Unknown task statuses
normalize to unknown workload rather than crashing or pretending to be idle.

Selection adds bounded reads:

- Agent selection uses the current Office projection.
- Resolved Issue selection uses the current brief.
- Unresolved Issue selection enables the existing Issue detail query.
- Squad selection enables that Squad's existing member-status query. The
  workspace member list loads only when human names are needed.

No query runs per Agent. No Squad query runs until selection.

## Realtime and continuity

React Query remains the only server-state owner. Office never mirrors Agent,
runtime, task, Issue, or Squad rows into Zustand.

A private continuity state owns effects:

~~~text
cold -> rebasing -> observing
~~~

The state returns to rebasing on:

- workspace change;
- WebSocket reconnect;
- page foreground resume;
- world switch;
- renderer recovery;
- reduced-motion change that cancels motion.

Cold and rebasing commits use replace mode and contain no effects. The latest
settled snapshot becomes the baseline. Only later snapshots observed
continuously while foregrounded use transition mode.

Effect proof is strict:

- A new queued-like task ID may emit task-queued.
- The same task ID changing from queued-like to running may emit task-started.
- A task that was active may emit task-finished only when the next snapshot
  contains the same task ID as completed or failed.
- A task that disappears without a matching terminal row emits no success or
  failure story.

Stale cached data freezes ambient and transition motion. Static poses, glyphs,
counts, selection, roster, inspector, and links remain available. The page does
not show a Live badge because the current WS context has no authoritative
connection-state signal.

Office never subscribes to task:message or task:progress.

## Renderer choice

V1 uses PixiJS v8 through a dynamic import in packages/views/office. It does
not use @pixi/react. React owns data and DOM structure. Pixi owns only pixels in
one host element.

The renderer explicitly prefers WebGL. Pixi's current documentation recommends
WebGL for production and describes WebGPU as still maturing. V1 does not add
WebGPU-specific code.

Multica's server CSP uses script-src 'self'. The Office renderer imports the
Pixi strict-CSP compatibility module, pixi.js/unsafe-eval, and must pass a
production-build CSP test without adding 'unsafe-eval' to the CSP header.

Pixi v8 does not provide a production Canvas2D fallback. The required fallback
is the existing DOM roster, inspector, exact counts, and selected-world poster.

Official references:

- https://pixijs.com/8.x/guides/components/application
- https://pixijs.com/8.x/guides/components/renderers
- https://pixijs.com/8.x/guides/components/application/ticker-plugin
- https://pixijs.com/8.x/guides/components/accessibility
- https://doc.mapeditor.org/en/stable/reference/json-map-format/

### Placement and scale

Each pack declares stable station, Squad-board, active-Issue, dispatch,
overflow, and camera anchors. Placement uses a checked-in stable hash of world,
layoutVersion, kind, and entity ID. It does not use list order, display name, or
Squad membership. Existing allocations stay fixed during a mount.

V1 starts with separate budgets:

- 40 animated Agents;
- 12 Squad boards;
- 48 active Issue markers;
- 100 fetched Issue briefs;
- a complete virtualized DOM roster for every returned subject.

These are provisional engineering budgets, not product facts. Validate them
against real workspace percentiles before map art locks its anchor counts. The
scene shows exact overflow counts. It does not add annex rooms or try to animate
hundreds of subjects.

An active Issue is one scene object even when several Agents execute it.
Multiple tasks show a count, not cloned Agent actors.

### State mapping

| Fact | Stable scene state | Transition allowed while observing |
| --- | --- | --- |
| online and idle | Local idle pose and limited ambient motion | None |
| online and queued | Waiting pose and queued token | New queued token |
| online and working | Work pose, lit station, running count | Same task token moves to station |
| unstable | Current workload pose plus unstable geometry | No availability story |
| offline | Frozen or empty station plus offline geometry | No workload story |
| unknown availability or workload | Neutral hold pose and question geometry | None |
| proven completion | Next truthful stable pose | Short check or signal flare |
| proven failure | Next truthful stable pose | Short non-celebratory alert |

Reduced motion removes wandering, path travel, particles, pulses, flicker,
camera easing, and world crossfades. Static state, icons, text, counts, focus,
and selection remain.

## World-pack contract

World packs are private renderer data. The page knows only OfficeWorldId.

Each pack supplies:

- contractVersion and layoutVersion;
- Tiled JSON map and collision/walk layers;
- atlas URLs and frame metadata;
- station, board, Issue, dispatch, overflow, and camera anchor pools;
- all required idle, wait, work, walk, unstable, offline, completion, and
  failure clips;
- light and dark scene lighting;
- hit polygons, palette, fallback poster, and provenance records.

The two layouts may differ, but both implement the same semantic roles. A build
validator checks map bounds, unique anchors, required layers, anchor capacities,
atlas bounds, clip completeness, asset paths, hashes, licenses, and pack
budgets. layoutVersion makes an intentional geometry change explicit.

Only the selected pack loads and decodes. A world change validates and loads
the target pack before persisting the new world. A failed switch leaves the
current pack and preference in place.

The Office view store persists only world per workspace and device through
createWorkspaceAwareStorage(defaultStorage). Camera, selection, roster tab,
hover, effects, and renderer status stay local.

## Asset and IP policy

Munder Difflin was inspected at commit
b91a49fc0896cb95058ff74b7910820452b3bb42. Its useful ideas are Pixi, Tiled
maps, semantic state-to-motion mapping, data-driven packs, ticker pause, and
context recovery.

Multica will not copy Munder code, maps, tilesets, characters, or audio. Munder
ships separately licensed LimeZu art that is not covered by its source-code
license.

Every Office asset has a record in PROVENANCE.json with author, source,
creation date, license identifier, modification note, attribution requirement,
and SHA-256. ATTRIBUTION.md contains distributable notices. CI fails when a
runtime asset lacks a record or a pack lacks a required clip.

Expedition art starts from a written Multica brief and original silhouette
sheets. Palworld screenshots or extracted assets are not tracing, generation,
or layout references. A human design and IP review remains required before the
pack may ship. Hash checks prove file provenance, not visual originality.

V1 has no audio.

Reference project:

- https://github.com/chaitanyagiri/munder-difflin/tree/b91a49fc0896cb95058ff74b7910820452b3bb42

## Module map

| Owner | Add or change | Responsibility |
| --- | --- | --- |
| packages/core/office | types.ts, model.ts, queries.ts, use-office-model.ts, continuity.ts, view-store.ts, index.ts, focused tests | Query composition, safe projection, partial-data policy, selected-subject reads, private continuity, world preference |
| packages/core/api/client.ts | Parse getAgentTaskSnapshot with AgentTaskListSchema | API compatibility boundary |
| packages/core/realtime/use-realtime-sync.ts | Invalidate Office Issue briefs on Issue events and reconnect | One thin freshness registration, no new event |
| packages/core/package.json | Export ./office | Leaf registration |
| packages/views/office | OfficePage, surface, toolbar, roster, inspector, scene host, DOM fallback, tests, index | Shared Web and Desktop UX |
| packages/views/office/scene | Pixi runtime, reconciler, placement, camera, recovery, world-pack loader | Private renderer |
| packages/views/office/worlds | studio and expedition maps, atlases, posters, manifests, provenance | Exactly two scene packs |
| packages/views/package.json | Export ./office and depend on pixi.js through the workspace catalog | Direct dependency ownership |
| packages/ui | No Office directory | Reuse existing controls and semantic tokens |
| packages/core/paths | Add office path, page key, nav key, and Building2 icon name | Route and Desktop tab identity |
| packages/views/layout | Add Building2 component and Office after Rooms in workspaceNav | Thin sidebar registration |
| packages/views/search | Add PAGE_KEYWORDS.office | Existing generated page search |
| packages/views/editor/utils/link-handler.ts | Add office to WORKSPACE_ROUTE_SEGMENTS | Internal link recognition |
| packages/views/locales | Add office namespace and layout.nav.office for en, zh-Hans, ja, and ko | Shared product copy |
| apps/web | Add one-line Office route | App Router registration only |
| apps/desktop | Add OfficePage import and route | Desktop session route only |
| server/internal/handler/reserved_slugs.json | Add office and regenerate reserved-slugs.ts | Prevent route collision |
| server otherwise | No Office package, endpoint, query, migration, or event | Existing server truth is sufficient |
| apps/mobile | No Office route, renderer, or asset copy | Native Mobile is out of scope |
| scripts/check-office-assets.mjs | Validate world contracts, assets, hashes, licenses, and budgets | Re-runnable asset gate |
| docs/downstream/specs/dynamic-pixel-office.md | This contract | Product and architecture truth |

This shape owns a leaf and registers it at existing hubs. It does not rewrite
the upstream shell.

## Accessibility and responsive behavior

The Pixi canvas is aria-hidden. The DOM roster is the canonical interaction
tree.

The roster has Agent, Squad, and Active Issue tabs. Rows are buttons with
visible focus, icon plus text status, exact counts, and roving keyboard focus.
Arrow keys, Home, End, Enter, and Escape follow existing app behavior. Canvas
selection synchronizes to the roster. Roster selection centers and highlights
the scene through reconcile. Closing the inspector restores focus to the
originating row.

Routine realtime churn does not enter a global live region. Only a selected
subject disappearing or the selected subject changing state may produce one
coalesced polite announcement.

All user-provided text remains DOM text. The canvas draws generic sprites,
glyphs, and packets. This avoids CJK font loading, name-texture churn, private
content in GPU textures, and tiny unreadable labels. A DOM tooltip may show the
selected or hovered subject's safe name.

At 200% zoom, the rail and toolbar wrap or collapse without overlapping the
scene. At narrow widths, the poster, roster, and inspector are the full
experience. Touch controls meet 44px targets.

## Failure behavior

| Failure | Result |
| --- | --- |
| Agent list pending | Page skeleton, not an empty office |
| Agent list unavailable with no cache | Page-level retry |
| Runtime unavailable | Agent identity remains, availability is unknown |
| Task snapshot unavailable | Agent identity remains, workload is unknown, no active Issue effects |
| Squad list unavailable | Agent scene remains, Squad boards are absent with one notice |
| Issue briefs unavailable or omitted | Generic active Issue remains selectable and linkable |
| Selected Squad status unavailable | Summary remains, member section has a local retry |
| Cached refresh failure | Last-known static state remains, motion freezes |
| Target world load failure | Current world remains selected and persisted |
| Initial WebGL failure | Poster, roster, inspector, and links remain |
| First context loss | Rebuild once from the newest snapshot in replace mode |
| Repeated context loss | Retire Pixi for the mount and keep DOM fallback |

## Performance and recovery

Pixi uses WebGL, nearest-neighbor textures, antialiasing off, roundPixels,
resolution capped at min(devicePixelRatio, 2), one private ticker, keyed sprite
reuse, effect pooling, and viewport culling.

The ticker runs at no more than 30 fps. Sprite clips may animate at 8 to 16 fps.
At most 16 one-shot effects run together. Later effects coalesce to final
truthful poses.

document.visibilitychange and IntersectionObserver stop the ticker when the
page is hidden or offscreen. Query state may continue to update, but returning
uses replace mode and does not replay changes.

Initial acceptance budgets are:

- each selected pack at or below 2 MiB compressed transfer;
- decoded textures at or below 16 MiB per pack;
- p95 projection and scene reconciliation below 8 ms at 40 Agents, 12 Squads,
  and 48 active Issues on the agreed reference laptop;
- no main-thread task above 50 ms for one task-state change;
- zero animation frames while hidden;
- no growing sprite, listener, texture, or ticker count after 20 world switches
  and 10 context recovery cycles;
- no Pixi or world-pack chunk in a non-Office route's initial production graph.

Treat these as test targets, not current performance claims.

## Privacy and telemetry

The Office projection admits only display identity, availability, workload
counts, active Issue briefs, Squad summary, and selected-Squad member status.

It drops task result, error, failure text, trigger summary, handoff note,
attribution, work and durable paths, branch, prompt, message, tool, token,
runtime configuration, Agent instructions, and Squad instructions.

V1 adds no selection or world-change analytics. Existing route pageview policy
may record the Office route. Renderer failure reporting may contain only world,
client type, fixed failure class, and recovery attempt. It contains no
workspace, entity, title, count, path, message, or browser content.

Screenshots and recordings use seeded synthetic workspaces. Production browser
content, credentials, and private responses never enter evidence artifacts.

## Delivery plan

### Phase 0: renderer gate

Before production art starts, build a small disposable renderer spike in both
production Web and Desktop builds. Prove:

- dynamic import does not enter non-Office routes;
- strict CSP works without widening script-src;
- WebGL initializes and renders nonblank pixels;
- hidden-page pause, teardown, and one context recovery work;
- the DOM fallback remains usable.

If this gate fails, keep the same Office model and DOM experience and revisit
the renderer choice before art production.

### Phase 1: truthful headless model

Add the Office types, safe task-snapshot parser, pure projection, active-Issue
query, targeted invalidation, selected-subject reads, world store, and Node
tests. No route or Pixi lands in this phase.

### Phase 2: accessible shared page

Add route registrations, sidebar/search/link handling, locales, toolbar,
virtualized roster, read-only inspector, responsive poster fallback, and
NavigationAdapter links. Both Web and Desktop become usable before animation
lands.

### Phase 3: Studio

Add the private renderer, deterministic placement, camera, continuity gate,
context recovery, Studio pack, asset validator, and renderer tests. Prove real
Query snapshot changes in a running app.

### Phase 4: Expedition and closure

Add the independently art-directed Expedition pack through the same contract.
Complete design and IP review, performance tuning, CJK, reduced-motion,
context-loss, responsive, Web/Desktop, screenshot, recording, and license
evidence.

Each phase ends in a runnable and reviewable state. A green build does not
replace user-flow or visual proof.

## Test ownership

| Behavior | Canonical test layer |
| --- | --- |
| Status mapping, unknown sources, archived exclusion, Issue sentinel, active task classification, dedupe, multi-Squad truth, caps, and exact task-ID effects | packages/core/office Node tests |
| Task snapshot response parsing and malformed fallback | packages/core/api client/schema tests |
| Four warm base subscriptions, one Issue batch, no per-Agent read, selected-Squad-only read, unresolved selected-Issue read, query keys, retry, and rebase behavior | packages/core/office hook/query tests |
| World persistence and invalid stored value recovery per workspace | packages/core/office view-store tests |
| Roster keyboard behavior, focus return, inspector links, no CRUD, partial states, CJK, reduced motion, and narrow fallback | packages/views/office component tests |
| Deterministic placement, idempotent reconcile, no effects on replace, effect proof, visibility pause, pack swap, disposal, context restore, and fallback | packages/views/office scene tests |
| Paths, route icon, tab subject/presentation, sidebar, search keywords, internal links, locales, Web route, Desktop route, and reserved slug | Existing focused registration tests |
| Map layers, anchor capacities, clips, hashes, licenses, atlas bounds, and budgets | check-office-assets tests |
| Real WebGL, task transition, selection, navigation, reconnect, theme switch, reduced motion, context loss, and no mutation requests | Playwright and Desktop smoke tests |

Do not repeat the pure status matrix through mounted DOM tests.

## Acceptance evidence

Implementation is complete only after fresh evidence shows:

- both worlds in Web at 1440x900 and 1024x768;
- both worlds in Desktop at the corresponding app sizes;
- poster and roster fallback at 390x844;
- light and dark appearance, plus representative Tension, Relay, and Field
  chrome;
- English and Simplified Chinese with long names and titles;
- empty, partial, stale, overflow, and renderer-failure states;
- nonblank canvas pixel checks and a material image difference between worlds;
- a Web and Desktop interaction recording covering queued to running to
  completed, Agent/Squad/Issue inspection, world switch, and detail navigation;
- a recovery recording covering reconnect without replay, reduced motion, and
  forced context loss;
- a network trace proving no task-message subscription, no per-Agent query, one
  active-Issue batch, and one selected-Squad read;
- bundle, frame-time, hidden-ticker, heap, listener, asset, provenance, and
  accessibility reports;
- focused tests, pnpm typecheck, pnpm lint, pnpm test, production Web/Desktop
  builds, focused Playwright, and the reserved-slug Go test.

Synthetic proof, build success, real browser behavior, real Desktop behavior,
and the human Expedition design/IP gate are separate claims.

## Interface depth and red-flag screen

The apps learn one page. The page learns one selected-aware model hook and one
scene host. The model hook hides Query fan-in, unknown and stale policy, Issue
batching, selected-Squad loading, privacy filtering, and continuity. The scene
handle hides Pixi, Tiled, placement, packs, camera, ticker, and recovery.

No public Office type contains wire payloads, Query results, Tiled objects,
Pixi classes, router objects, source timestamps, layout seeds, task effects, or
continuity epochs. The view store persists only OfficeWorldId.

The modules are grouped by knowledge, not execution order. Data normalization,
query composition, and continuity stay in the core Office leaf. World parsing,
scene reconciliation, and recovery stay in the private scene leaf. The page
does not coordinate load, validate, transform, and render stages.

OfficeSceneHandle has no pass-through reveal or focus method. Reconcile already
owns selection framing. The design adds no Office service, repository wrapper,
event bus, scene Zustand store, or second renderer. These decisions clear the
shallow-module, information-leakage, temporal-decomposition, and pass-through
red flags.

## Tradeoffs accepted

- Client composition uses four warm reads plus one bounded Issue read. This
  avoids a cross-domain Office API and a second realtime contract.
- Full Squad membership loads only on selection. This avoids startup fanout and
  keeps the three-row preview honest.
- The scene has explicit caps. The complete roster and exact overflow counts
  preserve access without adding room navigation.
- Unresolved active Issues stay generic but selectable and linkable. This keeps
  the page useful through API drift and partial failure.
- One DOM roster duplicates the scene's semantics. It provides keyboard,
  screen-reader, overflow, narrow-screen, and renderer-failure behavior.
- World choice is per workspace and device in V1. This avoids a new account
  preference API.
- Observed effects require exact task-ID proof. Missed effects after reconnect
  are preferable to fabricated live activity.
- Original art production is real project work. Two worlds cannot be delivered
  as one map with a palette change.

## Alternatives considered

### Server Office projection

A new GET /api/office could reduce frontend reads, but it would create a wide
Agent/runtime/task/Issue/Squad compatibility contract and another projection
that can disagree with existing pages. The four primary caches are already
warmed. The endpoint does not justify its maintenance cost in V1.

### DOM-only primary renderer

A DOM sprite scene removes WebGL recovery, but camera transforms, many moving
actors, tile occlusion, pathing, and two atlas-driven worlds would spread scene
geometry through React components. The DOM path remains the fallback, where its
simplicity is useful.

### React-Pixi

React-Pixi would make the first implementation familiar, but it would expose
scene nodes to React and turn context recovery into component lifecycle. One
imperative reconcile boundary is smaller.

### Direct WebSocket scene state

Direct task events could animate earlier, but they would create a second client
state database and a replay problem. Query snapshots remain authoritative.

### Permanent Squad rooms

Permanent rooms require an exclusive primary Squad that the domain does not
have. Boards, selected membership highlights, and Issue assignment links show
the real many-to-many shape.

### Unbounded maps or annex rooms

Procedural room navigation adds another product model and hides scale problems.
One art-directed cohort plus the full roster is smaller and more predictable.

## Synthesis decision

Five native gpt-5.6-sol@max candidates completed. Candidate B is the base because
it had the strongest exact task-ID transition proof and one selected-aware model
hook. Candidate A contributed separate brief and scene budgets plus the
single-rail interaction. Candidate C contributed the private continuity rebase
state machine. Candidate D contributed the rule that stale cached data freezes
motion. Candidate E contributed layoutVersion for explicit geometry changes.

The synthesis rejects Candidate B's Office-only branded IDs, redundant reveal
renderer command, public workspace layout key, and effects inside the stable
snapshot. It also rejects triple permanent columns, annex rooms, unbounded
actors, app-supplied navigation adapters, transport-shaped public types, and
blanket removal of continuously observed effects.

The configured Claude Fable, Grok, and Claude Opus lanes did not produce
candidates. Grok failed authentication preflight. Both Claude lanes passed the
local login check but the service returned HTTP 401 for the subscription. The
cross-judge used the same Sol provider as the surviving candidates, so this
design has no provider-diversity confirmation.

## Assumptions to confirm before art production

- The initial 40-Agent, 12-Squad, and 48-Issue scene cohort is large enough for
  the intended launch workspaces. Real percentiles should replace this
  assumption before anchor counts lock.
- Expedition represents Agents as original companion creatures rather than
  humans with separate pets.
- World choice remains per workspace and device in V1.
- The Office sidebar entry is visible to every workspace member and appears
  after Rooms and before Agents.
- V1 ships with no audio and no behavioral analytics beyond existing pageview
  policy.

## Next implementation step

Start with failing Node tests for the Office projection. Cover independent
availability and workload, missing sources, empty Issue linkage, active task
classification, exact terminal-task proof, archived exclusion, multiple Squad
membership, unresolved Issue handling, and scene-cap overflow before adding a
route or renderer.
