import type { OfficeIssue } from "@multica/core/office";
import type { OfficeWorldPack } from "../worlds/types";
import type {
  OfficeAgentVisualVariant,
  OfficeSceneCommit,
  OfficeSceneEntity,
  OfficeSceneLink,
  OfficeScenePlan,
} from "./contracts";
import { PlacementRegistry } from "./placement";
import { mapAgentSceneState } from "./state-mapping";

function resolvedIssue(
  issue: OfficeIssue,
): issue is Extract<OfficeIssue, { readonly kind: "resolved" }> {
  return issue.kind === "resolved";
}

function stableHash(value: string): number {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

export function agentVisualVariant(id: string): OfficeAgentVisualVariant {
  return {
    accent: stableHash(`accent|${id}`) % 4,
    body: stableHash(`body|${id}`) % 4,
    silhouette: stableHash(`silhouette|${id}`) % 4,
  };
}

export function buildOfficeScenePlan(input: {
  readonly commit: OfficeSceneCommit;
  readonly pack: OfficeWorldPack;
  readonly placements: PlacementRegistry;
}): OfficeScenePlan {
  const { commit, pack, placements } = input;
  const agentPlacements = placements.reconcile(
    "agent",
    commit.snapshot.agents.map((agent) => agent.id),
    pack.anchors.agentStations,
  ).placements;
  const squadPlacements = placements.reconcile(
    "squad",
    commit.snapshot.squads.map((squad) => squad.id),
    pack.anchors.squadBoards,
  ).placements;
  const issuePlacements = placements.reconcile(
    "issue",
    commit.snapshot.activeIssues.map((issue) => issue.id),
    pack.anchors.activeIssues,
  ).placements;

  const selectedIssue =
    commit.selected?.kind === "issue"
      ? commit.snapshot.activeIssues.find(
          (issue) => issue.id === commit.selected?.id,
        )
      : undefined;
  const highlightedAgentIds = new Set(
    commit.selected?.kind === "squad"
      ? commit.selectedSquadAgentIds
      : commit.selected?.kind === "issue" && selectedIssue
        ? selectedIssue.executingAgentIds
        : [],
  );
  if (commit.selected?.kind === "agent") {
    highlightedAgentIds.add(commit.selected.id);
  }

  const entities: OfficeSceneEntity[] = [];
  for (const agent of [...commit.snapshot.agents].sort((left, right) =>
    left.id.localeCompare(right.id),
  )) {
    const anchor = agentPlacements.get(agent.id);
    if (!anchor) continue;
    entities.push({
      kind: "agent",
      key: `agent:${agent.id}`,
      id: agent.id,
      anchor,
      highlighted: highlightedAgentIds.has(agent.id),
      state: mapAgentSceneState(
        agent,
        commit.reducedMotion || commit.motionFrozen,
      ),
      visualVariant: agentVisualVariant(agent.id),
    });
  }

  for (const squad of [...commit.snapshot.squads].sort((left, right) =>
    left.id.localeCompare(right.id),
  )) {
    const anchor = squadPlacements.get(squad.id);
    if (!anchor) continue;
    const highlightedByIssue =
      selectedIssue &&
      resolvedIssue(selectedIssue) &&
      selectedIssue.assignedSquadId === squad.id;
    entities.push({
      kind: "squad",
      key: `squad:${squad.id}`,
      id: squad.id,
      anchor,
      highlighted:
        (commit.selected?.kind === "squad" && commit.selected.id === squad.id) ||
        highlightedByIssue === true,
      memberCount: squad.memberCount,
      previewCount: Math.min(3, squad.memberPreview.length),
    });
  }

  for (const issue of [...commit.snapshot.activeIssues].sort((left, right) =>
    left.id.localeCompare(right.id),
  )) {
    const anchor = issuePlacements.get(issue.id);
    if (!anchor) continue;
    entities.push({
      kind: "issue",
      key: `issue:${issue.id}`,
      id: issue.id,
      anchor,
      highlighted:
        commit.selected?.kind === "issue" && commit.selected.id === issue.id,
      resolved: resolvedIssue(issue),
    });
  }

  const overflow = [
    ["agent", commit.snapshot.overflow.agents],
    ["squad", commit.snapshot.overflow.squads],
    ["issue", commit.snapshot.overflow.activeIssues],
  ] as const;
  overflow.forEach(([subjectKind, count], index) => {
    const anchor = pack.anchors.overflow[index];
    if (!anchor || count <= 0) return;
    entities.push({
      kind: "overflow",
      key: `overflow:${subjectKind}`,
      id: subjectKind,
      subjectKind,
      count,
      anchor,
      highlighted: false,
    });
  });

  const visibleKeys = new Set(entities.map((entity) => entity.key));
  const links: OfficeSceneLink[] = [];
  if (selectedIssue && resolvedIssue(selectedIssue) && selectedIssue.assignedSquadId) {
    const issueKey = `issue:${selectedIssue.id}`;
    const squadKey = `squad:${selectedIssue.assignedSquadId}`;
    if (visibleKeys.has(issueKey) && visibleKeys.has(squadKey)) {
      links.push({ from: issueKey, to: squadKey, kind: "assignment" });
    }
  }

  return {
    world: pack.id,
    layoutVersion: pack.layoutVersion,
    width: pack.map.width * pack.map.tileSize,
    height: pack.map.height * pack.map.tileSize,
    reducedMotion: commit.reducedMotion,
    motionFrozen: commit.motionFrozen,
    entities,
    links,
  };
}
