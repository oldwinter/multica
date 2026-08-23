import type { LifecycleContent } from "@multica/core/twins";
import type {
  ProjectedApplicability,
  ProjectedDiff,
  ProjectedItem,
  ProjectedProvenance,
  ProjectedTopic,
} from "./twin-workspace-types";

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(record: Readonly<Record<string, unknown>>, key: string): string {
  const value = record[key];
  return typeof value === "string" ? value : "";
}

function numberValue(record: Readonly<Record<string, unknown>>, key: string): number | null {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function stringArray(record: Readonly<Record<string, unknown>>, key: string): readonly string[] {
  const value = record[key];
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function records(value: unknown): readonly Readonly<Record<string, unknown>>[] {
  return Array.isArray(value) ? value.filter(isRecord) : [];
}

function wikiTitle(item: Readonly<Record<string, unknown>>, kind: string): string {
  const title = stringValue(item, "title") || stringValue(item, "label") || stringValue(item, "autopilot_title");
  const number = numberValue(item, "number");
  if (kind === "issues" && number !== null) return `Issue ${number}: ${title}`;
  return title;
}

export function projectWikiContent(content: LifecycleContent): readonly ProjectedItem[] {
  const groups = ["issues", "projects", "project_resources", "autopilot_runs"];
  return groups.flatMap((kind) => records(content[kind]).map((item) => ({
    id: stringValue(item, "id") || stringValue(item, "citation_key"),
    title: wikiTitle(item, kind),
    summary: stringValue(item, "description") || stringValue(item, "label"),
    status: stringValue(item, "status") || stringValue(item, "acceptance_state"),
    citationKeys: stringValue(item, "citation_key") ? [stringValue(item, "citation_key")] : [],
    kind,
    applicability: null,
    confidence: null,
    provenance: null,
  })).filter((item) => item.id.length > 0 || item.title.length > 0));
}

function projectApplicability(value: unknown): ProjectedApplicability | null {
  if (typeof value === "string") {
    return value.trim().length > 0 ? {
      taskId: "",
      workspaceId: "",
      agentId: "",
      projectId: "",
      issueId: "",
      keywords: [],
      legacyText: value.trim(),
    } : null;
  }
  if (!isRecord(value)) return null;
  const projected = {
    taskId: stringValue(value, "task_id"),
    workspaceId: stringValue(value, "workspace_id"),
    agentId: stringValue(value, "agent_id"),
    projectId: stringValue(value, "project_id"),
    issueId: stringValue(value, "issue_id"),
    keywords: stringArray(value, "keywords"),
    legacyText: "",
  };
  return Object.values(projected).some((item) => Array.isArray(item) ? item.length > 0 : item.length > 0)
    ? projected
    : null;
}

function projectProvenance(value: unknown): ProjectedProvenance | null {
  if (!isRecord(value)) return null;
  const kind = stringValue(value, "kind");
  const generator = stringValue(value, "generator");
  return kind || generator ? { kind, generator } : null;
}

export function projectTwinAssertions(content: LifecycleContent): readonly ProjectedItem[] {
  return records(content.assertions).map((item) => ({
    id: stringValue(item, "id"),
    title: stringValue(item, "text"),
    summary: stringValue(item, "source_summary"),
    status: stringValue(item, "source_status"),
    citationKeys: stringArray(item, "evidence_citations").length > 0
      ? stringArray(item, "evidence_citations")
      : stringArray(item, "citation_keys"),
    kind: stringValue(item, "type") || "assertion",
    applicability: projectApplicability(item.applicability),
    confidence: numberValue(item, "confidence"),
    provenance: projectProvenance(item.provenance),
  })).filter((item) => item.id.length > 0 || item.title.length > 0);
}

export function projectTwinTopics(content: LifecycleContent): readonly ProjectedTopic[] {
  return records(content.topics).map((item) => ({
    id: stringValue(item, "id"),
    issueId: stringValue(item, "issue_id"),
    issueNumber: numberValue(item, "issue_number"),
    title: stringValue(item, "title"),
    status: stringValue(item, "status"),
  })).filter((item) => item.issueId.length > 0);
}

export function projectTwinDiff(content: LifecycleContent): ProjectedDiff {
  const diff = content.diff;
  if (!isRecord(diff)) return { added: [], removed: [], unchanged: [], changed: [] };
  return {
    added: stringArray(diff, "added"),
    removed: stringArray(diff, "removed"),
    unchanged: stringArray(diff, "unchanged"),
    changed: stringArray(diff, "changed"),
  };
}

export function diffWikiContent(
  current: LifecycleContent,
  accepted: LifecycleContent | null,
): ProjectedDiff {
  const currentItems = projectWikiContent(current);
  const acceptedItems = accepted ? projectWikiContent(accepted) : [];
  const currentById = new Map(currentItems.map((item) => [item.id, item]));
  const acceptedById = new Map(acceptedItems.map((item) => [item.id, item]));
  const changed = (id: string): boolean => {
    const currentItem = currentById.get(id);
    const acceptedItem = acceptedById.get(id);
    if (!currentItem || !acceptedItem) return true;
    return currentItem.title !== acceptedItem.title ||
      currentItem.summary !== acceptedItem.summary ||
      currentItem.status !== acceptedItem.status ||
      currentItem.citationKeys.join("|") !== acceptedItem.citationKeys.join("|");
  };
  return {
    added: [...currentById.keys()].filter((id) => !acceptedById.has(id) || changed(id)),
    removed: [...acceptedById.keys()].filter((id) => !currentById.has(id) || changed(id)),
    unchanged: [...currentById.keys()].filter((id) => acceptedById.has(id) && !changed(id)),
    changed: [],
  };
}
