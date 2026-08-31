"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";
import type { SkillSummary } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { DropdownMenuItem } from "@multica/ui/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { copyText } from "@multica/ui/lib/clipboard";
import { formatSlashCommandLabel } from "../../editor/extensions/slash-command-utils";
import { escapeMarkdownLabel } from "../../editor/utils/escape-markdown-label";
import { useT } from "../../i18n";

type SkillReference = Pick<SkillSummary, "id" | "name">;

export function formatSkillReference(skill: SkillReference): string {
  const label = escapeMarkdownLabel(formatSlashCommandLabel(skill.name));
  return `[/${label}](slash://skill/${skill.id})`;
}

function useCopySkillReference(skill: SkillReference) {
  const { t } = useT("skills");

  return () => {
    void copyText(formatSkillReference(skill)).then((ok) => {
      if (ok) {
        toast.success(t(($) => $.actions.reference_copied_toast));
      } else {
        toast.error(t(($) => $.actions.reference_copy_failed_toast));
      }
    });
  };
}

export function SkillReferenceMenuItem({
  skill,
}: {
  readonly skill: SkillReference;
}) {
  const { t } = useT("skills");
  const handleCopy = useCopySkillReference(skill);

  return (
    <DropdownMenuItem onClick={handleCopy}>
      <Copy className="size-3.5" aria-hidden="true" />
      {t(($) => $.actions.copy_reference)}
    </DropdownMenuItem>
  );
}

export function SkillReferenceButton({
  skill,
}: {
  readonly skill: SkillReference;
}) {
  const { t } = useT("skills");
  const handleCopy = useCopySkillReference(skill);
  const label = t(($) => $.actions.copy_reference);

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            className="max-md:size-11"
            aria-label={label}
            onClick={handleCopy}
          />
        }
      >
        <Copy className="size-3.5" aria-hidden="true" />
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
