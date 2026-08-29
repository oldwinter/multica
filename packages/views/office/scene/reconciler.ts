import type { OfficeIssue } from "@multica/core/office";
import { loadOfficeWorldPack } from "../worlds/world-packs";
import type { OfficeWorldPack } from "../worlds/types";
import type {
  OfficeRendererStatus,
  OfficeSceneCommit,
  OfficeScenePort,
} from "./contracts";
import { PlacementRegistry } from "./placement";
import { buildOfficeScenePlan } from "./scene-plan";

function issueFingerprint(issue: OfficeIssue) {
  return issue.kind === "resolved"
    ? {
        kind: issue.kind,
        id: issue.id,
        statusCategory: issue.statusCategory,
        assignedSquadId: issue.assignedSquadId,
        executingAgentIds: [...issue.executingAgentIds].sort(),
      }
    : {
        kind: issue.kind,
        id: issue.id,
        reason: issue.reason,
        executingAgentIds: [...issue.executingAgentIds].sort(),
      };
}

function commitFingerprint(commit: OfficeSceneCommit): string {
  return JSON.stringify({
    world: commit.world,
    agents: [...commit.snapshot.agents]
      .sort((left, right) => left.id.localeCompare(right.id))
      .map((agent) => ({
        id: agent.id,
        availability: agent.availability,
        workload: agent.workload,
      })),
    squads: [...commit.snapshot.squads]
      .sort((left, right) => left.id.localeCompare(right.id))
      .map((squad) => ({
        id: squad.id,
        memberCount: squad.memberCount,
        previewCount: Math.min(3, squad.memberPreview.length),
      })),
    issues: [...commit.snapshot.activeIssues]
      .sort((left, right) => left.id.localeCompare(right.id))
      .map(issueFingerprint),
    overflow: commit.snapshot.overflow,
    selected: commit.selected,
    selectedSquadAgentIds: [...commit.selectedSquadAgentIds].sort(),
    mode: commit.mode,
    effects: [...commit.effects]
      .sort((left, right) => left.taskId.localeCompare(right.taskId))
      .map((effect) => ({ ...effect })),
    reducedMotion: commit.reducedMotion,
  });
}

export class OfficeSceneController {
  readonly #port: OfficeScenePort;
  readonly #onStatus: (status: OfficeRendererStatus) => void;
  #pack: OfficeWorldPack | null = null;
  #placements: PlacementRegistry | null = null;
  #lastRequestedFingerprint: string | null = null;
  #appliedFingerprint: string | null = null;
  #latestCommit: OfficeSceneCommit | null = null;
  #generation = 0;
  #pending: Promise<void> = Promise.resolve();
  #destroyed = false;

  constructor(input: {
    readonly port: OfficeScenePort;
    readonly onStatus: (status: OfficeRendererStatus) => void;
  }) {
    this.#port = input.port;
    this.#onStatus = input.onStatus;
  }

  get currentWorld() {
    return this.#pack?.id ?? null;
  }

  reconcile(commit: OfficeSceneCommit): void {
    if (this.#destroyed) return;
    const fingerprint = commitFingerprint(commit);
    if (fingerprint === this.#lastRequestedFingerprint) return;
    this.#lastRequestedFingerprint = fingerprint;
    this.#latestCommit = commit;
    const generation = ++this.#generation;
    this.#pending = this.#apply(commit, fingerprint, generation);
  }

  async #apply(
    commit: OfficeSceneCommit,
    fingerprint: string,
    generation: number,
  ): Promise<void> {
    try {
      const worldChanged = this.#pack?.id !== commit.world;
      if (worldChanged) {
        const target = await loadOfficeWorldPack(commit.world);
        if (this.#destroyed || generation !== this.#generation) return;
        await this.#port.installWorld(target);
        if (this.#destroyed || generation !== this.#generation) return;
        this.#pack = target;
        this.#placements = new PlacementRegistry({
          world: target.id,
          layoutVersion: target.layoutVersion,
        });
      }
      const pack = this.#pack;
      const placements = this.#placements;
      if (!pack || !placements) return;

      const plan = buildOfficeScenePlan({ commit, pack, placements });
      const replace =
        worldChanged || commit.mode === "replace" || commit.reducedMotion;
      if (replace) this.#port.cancelEffects();
      this.#port.apply(plan);
      if (!replace && commit.effects.length > 0) {
        this.#port.playEffects(commit.effects.slice(0, 16));
      }
      this.#appliedFingerprint = fingerprint;
      this.#onStatus({ kind: "ready", world: pack.id });
    } catch {
      this.#lastRequestedFingerprint = this.#appliedFingerprint;
      if (this.#pack) {
        this.#onStatus({
          kind: "world-switch-failed",
          attemptedWorld: commit.world,
          retainedWorld: this.#pack.id,
        });
      } else {
        this.#onStatus({ kind: "fallback", reason: "asset" });
      }
    }
  }

  reapplyLatestAsReplace(): void {
    const latest = this.#latestCommit;
    if (!latest || this.#destroyed) return;
    this.#lastRequestedFingerprint = null;
    this.reconcile({ ...latest, mode: "replace", effects: [] });
  }

  whenIdle(): Promise<void> {
    return this.#pending;
  }

  destroy(): void {
    if (this.#destroyed) return;
    this.#destroyed = true;
    this.#generation += 1;
    this.#port.destroy();
  }
}
