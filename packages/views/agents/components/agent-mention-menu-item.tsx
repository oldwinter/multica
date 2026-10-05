"use client";

import { AtSign } from "lucide-react";
import { toast } from "sonner";
import type { Agent } from "@multica/core/types";
import { DropdownMenuItem } from "@multica/ui/components/ui/dropdown-menu";
import { copyText } from "@multica/ui/lib/clipboard";
import { escapeMarkdownLabel } from "../../editor/utils/escape-markdown-label";
import { useT } from "../../i18n";

export function formatAgentMention(agent: Pick<Agent, "id" | "name">): string {
  return `[@${escapeMarkdownLabel(agent.name)}](mention://agent/${agent.id})`;
}

export function AgentMentionMenuItem({ agent }: { agent: Agent }) {
  const { t } = useT("agents");

  if (agent.archived_at) return null;

  const handleCopy = () => {
    void copyText(formatAgentMention(agent)).then((ok) => {
      if (ok) {
        toast.success(t(($) => $.mention.copied_toast));
      } else {
        toast.error(t(($) => $.mention.copy_failed_toast));
      }
    });
  };

  return (
    <DropdownMenuItem onClick={handleCopy}>
      <AtSign className="h-3.5 w-3.5" aria-hidden="true" />
      {t(($) => $.mention.copy)}
    </DropdownMenuItem>
  );
}
