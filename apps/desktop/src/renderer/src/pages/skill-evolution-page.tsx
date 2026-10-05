import { useParams } from "react-router-dom";
import { useWorkspaceId } from "@multica/core/hooks";
import { SkillEvolutionPage as SharedSkillEvolutionPage } from "@multica/views/skill-evolution";
import { useDocumentTitle } from "@/hooks/use-document-title";

export function SkillEvolutionPage() {
  const { workspaceSlug, id } = useParams<{
    workspaceSlug: string;
    id: string;
  }>();
  const workspaceId = useWorkspaceId();

  useDocumentTitle("Evolution");

  if (!workspaceSlug || !id) return null;
  return (
    <SharedSkillEvolutionPage
      workspaceId={workspaceId}
      workspaceSlug={workspaceSlug}
      skillId={id}
    />
  );
}
