"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@multica/ui/components/ui/button";
import { copyText } from "@multica/ui/lib/clipboard";
import { useT } from "../../i18n";
import type { ProjectedItem } from "./twin-workspace-types";

interface TwinSummaryInput {
  readonly heading: string;
  readonly digestLabel: string;
  readonly digest: string;
  readonly items: readonly ProjectedItem[];
}

function singleLine(value: string): string {
  return value.trim().replace(/\s+/g, " ");
}

export function formatTwinSummary({
  heading,
  digestLabel,
  digest,
  items,
}: TwinSummaryInput): string {
  const assertions = items
    .map((item) => singleLine(item.title || item.id))
    .filter(Boolean)
    .map((assertion) => `- ${assertion}`);
  const escapedDigest = digest.trim().replace(/`/g, "\\`");

  return [
    `## ${singleLine(heading)}`,
    ...(assertions.length > 0 ? ["", ...assertions] : []),
    "",
    `${singleLine(digestLabel)}: \`${escapedDigest}\``,
  ].join("\n");
}

export function TwinSummaryCopyButton({
  heading,
  digest,
  items,
}: {
  readonly heading: string;
  readonly digest: string;
  readonly items: readonly ProjectedItem[];
}) {
  const { t } = useT("twins");
  const label = t(($) => $.actions.copy_summary);

  const copySummary = () => {
    void copyText(formatTwinSummary({
      heading,
      digestLabel: t(($) => $.task_context.version_digest),
      digest,
      items,
    })).then((ok) => {
      if (ok) {
        toast.success(t(($) => $.actions.summary_copied));
      } else {
        toast.error(t(($) => $.actions.summary_copy_failed));
      }
    });
  };

  return (
    <Button
      type="button"
      size="sm"
      variant="outline"
      className="h-11 shrink-0 sm:h-8"
      onClick={copySummary}
    >
      <Copy data-icon="inline-start" aria-hidden="true" />
      {label}
    </Button>
  );
}
