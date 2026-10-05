import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { TaskRunReviewSkillOption } from "./types";

export const taskRunReviewKeys = {
  all: (wsId: string) => ["task-run-reviews", wsId] as const,
  skills: (wsId: string, agentId: string) =>
    [...taskRunReviewKeys.all(wsId), "skills", agentId] as const,
};

export function taskRunReviewSkillOptions(wsId: string, agentId: string) {
  return queryOptions({
    queryKey: taskRunReviewKeys.skills(wsId, agentId),
    queryFn: async (): Promise<TaskRunReviewSkillOption[]> => {
      const workspaceSkills = await api.listSkills();
      let assignedIds = new Set<string>();
      try {
        assignedIds = new Set(
          (await api.listAgentSkills(agentId))
            .filter((skill) => skill.enabled !== false)
            .map((skill) => skill.id),
        );
      } catch {
        // The workspace list is still an authorized, server-validated source.
      }
      return workspaceSkills
        .map((skill) => ({
          id: skill.id,
          name: skill.name,
          assignedToTaskAgent: assignedIds.has(skill.id),
        }))
        .sort((left, right) =>
          Number(right.assignedToTaskAgent) - Number(left.assignedToTaskAgent) ||
          left.name.localeCompare(right.name),
        );
    },
    enabled: !!wsId && !!agentId,
    staleTime: 30_000,
  });
}
