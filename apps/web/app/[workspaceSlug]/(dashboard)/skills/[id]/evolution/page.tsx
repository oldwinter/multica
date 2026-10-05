"use client";

import { use } from "react";
import { useWorkspaceId } from "@multica/core/hooks";
import { SkillEvolutionPage } from "@multica/views/skill-evolution";

export default function SkillEvolutionRoute({
  params,
}: {
  params: Promise<{ workspaceSlug: string; id: string }>;
}) {
  const { workspaceSlug, id } = use(params);
  const workspaceId = useWorkspaceId();

  return (
    <SkillEvolutionPage
      workspaceId={workspaceId}
      workspaceSlug={workspaceSlug}
      skillId={id}
    />
  );
}
