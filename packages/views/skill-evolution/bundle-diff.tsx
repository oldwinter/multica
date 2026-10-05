"use client";

import type { SkillEvolutionProposalDetail } from "@multica/core/skill-evolution";
import { AlertTriangle, FileCode2 } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

type Diff = SkillEvolutionProposalDetail["diff"];
type DiffRow = Diff["files"][number]["rows"][number];

function DiffLine({ row }: { row: DiffRow }) {
  const kind = row.kind === "add" || row.kind === "delete" ? row.kind : "context";
  const marker = kind === "add" ? "+" : kind === "delete" ? "-" : " ";
  const line = kind === "add" ? row.newLine : row.oldLine;

  return (
    <div
      data-diff-kind={kind}
      className={cn(
        "grid min-w-max grid-cols-[3rem_1.25rem_minmax(20rem,1fr)] px-2 font-mono text-micro leading-6",
        kind === "add" && "bg-success/10",
        kind === "delete" && "bg-destructive/10",
        kind === "context" && "text-muted-foreground",
      )}
    >
      <span className="select-none pe-2 text-end text-faint-foreground">
        {line ?? ""}
      </span>
      <span
        aria-hidden="true"
        className={cn(
          "select-none text-center",
          kind === "add" && "text-success",
          kind === "delete" && "text-destructive",
        )}
      >
        {marker}
      </span>
      <span className="min-w-0 whitespace-pre-wrap break-words pe-3">
        {row.text || " "}
      </span>
    </div>
  );
}

export function BundleDiff({ diff }: { diff: Diff }) {
  const { t } = useT("skill-evolution");
  const changeLabel = (change: string): string => {
    switch (change) {
      case "added":
        return t(($) => $.diff.added);
      case "modified":
        return t(($) => $.diff.modified);
      case "deleted":
        return t(($) => $.diff.deleted);
      default:
        return t(($) => $.diff.unknown);
    }
  };
  const metadata = diff.metadata ?? [];
  const files = diff.files ?? [];

  if (metadata.length === 0 && files.length === 0) {
    return (
      <p className="py-5 text-caption text-muted-foreground">
        {t(($) => $.diff.empty)}
      </p>
    );
  }

  return (
    <div className="space-y-6">
      {diff.truncated ? (
        <div role="status" className="flex items-center gap-2 rounded-md border border-warning/35 bg-warning/10 px-3 py-2 text-caption text-warning-foreground">
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {t(($) => $.diff.truncated, { count: diff.omittedRows })}
        </div>
      ) : null}
      {metadata.length > 0 ? (
        <section aria-labelledby="evolution-metadata-diff">
          <h4
            id="evolution-metadata-diff"
            className="mb-2 text-label font-medium text-muted-foreground"
          >
            {t(($) => $.diff.metadata)}
          </h4>
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full min-w-[34rem] table-fixed text-left text-caption">
              <thead className="border-b bg-muted/40 text-muted-foreground">
                <tr>
                  <th className="w-32 px-3 py-2 font-medium">{t(($) => $.diff.field)}</th>
                  <th className="px-3 py-2 font-medium">{t(($) => $.diff.before)}</th>
                  <th className="border-l px-3 py-2 font-medium">{t(($) => $.diff.after)}</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {metadata.map((item, index) => (
                  <tr key={`${item.field}:${index}`}>
                    <th className="px-3 py-2 align-top font-mono font-medium">
                      {item.field}
                    </th>
                    <td className="whitespace-pre-wrap break-words px-3 py-2 align-top text-muted-foreground">
                      {item.before}
                    </td>
                    <td className="border-l whitespace-pre-wrap break-words px-3 py-2 align-top">
                      {item.after}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}

      {files.length > 0 ? (
        <section aria-labelledby="evolution-file-diff">
          <h4
            id="evolution-file-diff"
            className="mb-2 text-label font-medium text-muted-foreground"
          >
            {t(($) => $.diff.files)}
          </h4>
          <div className="divide-y overflow-hidden rounded-md border">
            {files.map((file, fileIndex) => (
              <article key={`${file.path}:${fileIndex}`}>
                <header className="flex min-w-0 items-center gap-2 bg-muted/35 px-3 py-2">
                  <FileCode2 className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span className="min-w-0 flex-1 truncate font-mono text-caption" title={file.path}>
                    {file.path}
                  </span>
                  <span className="shrink-0 text-micro font-medium uppercase text-muted-foreground">
                    {changeLabel(file.change)}
                  </span>
                </header>
                <div className="max-h-[26rem] overflow-auto bg-background">
                  {(file.rows ?? []).map((row, rowIndex) => (
                    <DiffLine
                      key={`${row.oldLine ?? "x"}:${row.newLine ?? "x"}:${rowIndex}`}
                      row={row}
                    />
                  ))}
                </div>
                {file.truncated ? (
                  <div role="status" className="flex items-center gap-2 border-t border-warning/30 bg-warning/10 px-3 py-2 text-caption text-warning-foreground">
                    <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
                    {t(($) => $.diff.truncated, { count: file.omittedRows })}
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
