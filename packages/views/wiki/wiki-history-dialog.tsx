"use client";

import { useEffect, useMemo, useState } from "react";
import { History, RotateCcw } from "lucide-react";
import type { WikiRevision } from "@multica/core/wiki";
import { Button } from "@multica/ui/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { useT } from "../i18n";

interface WikiHistoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  revisions: readonly WikiRevision[];
  currentRevisionNumber: number;
  isLoading: boolean;
  isError: boolean;
  isRestoring: boolean;
  actionError?: string | null;
  onRetry: () => void;
  onRestore: (revisionId: string) => void;
}

export function WikiHistoryDialog({
  open,
  onOpenChange,
  revisions,
  currentRevisionNumber,
  isLoading,
  isError,
  isRestoring,
  actionError,
  onRetry,
  onRestore,
}: WikiHistoryDialogProps) {
  const { t } = useT("wiki");
  const ordered = useMemo(
    () => [...revisions].sort((a, b) => b.revisionNumber - a.revisionNumber),
    [revisions],
  );
  const [leftId, setLeftId] = useState("");
  const [rightId, setRightId] = useState("");
  const [restoreId, setRestoreId] = useState("");

  useEffect(() => {
    if (ordered.length === 0) return;
    setRightId((current) => current || ordered[0]?.id || "");
    setLeftId((current) => current || ordered[1]?.id || ordered[0]?.id || "");
  }, [ordered]);

  const left = ordered.find((revision) => revision.id === leftId);
  const right = ordered.find((revision) => revision.id === rightId);
  const restoreTarget = ordered.find((revision) => revision.id === restoreId);
  const items = ordered.map((revision) => ({
    value: revision.id,
    label: t(($) => $.history.revision_label, { number: revision.revisionNumber }),
  }));

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <History className="size-4" aria-hidden="true" />
              {t(($) => $.history.title)}
            </DialogTitle>
            <DialogDescription>{t(($) => $.history.description)}</DialogDescription>
          </DialogHeader>

          {isLoading ? (
            <p className="py-8 text-center text-body text-muted-foreground" role="status">
              {t(($) => $.history.loading)}
            </p>
          ) : isError ? (
            <div className="space-y-3 py-8 text-center">
              <p className="text-body text-destructive" role="alert">{t(($) => $.history.error)}</p>
              <Button variant="outline" onClick={onRetry}>{t(($) => $.actions.retry)}</Button>
            </div>
          ) : ordered.length === 0 ? (
            <p className="py-8 text-center text-body text-muted-foreground">{t(($) => $.history.empty)}</p>
          ) : (
            <div className="space-y-5">
              {actionError ? <p className="text-body text-destructive" role="alert">{actionError}</p> : null}
              <div className="grid gap-3 sm:grid-cols-2">
                <RevisionSelect
                  label={t(($) => $.history.compare_from)}
                  items={items}
                  value={leftId}
                  onValueChange={setLeftId}
                />
                <RevisionSelect
                  label={t(($) => $.history.compare_to)}
                  items={items}
                  value={rightId}
                  onValueChange={setRightId}
                />
              </div>

              <div className="grid min-h-64 gap-3 sm:grid-cols-2">
                <RevisionPreview revision={left} />
                <RevisionPreview revision={right} />
              </div>

              <div className="space-y-2" aria-label={t(($) => $.history.timeline)}>
                {ordered.map((revision) => (
                  <div
                    key={revision.id}
                    className="flex flex-col gap-2 border-t border-surface-border py-3 first:border-t-0 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0">
                      <p className="text-body font-medium text-foreground">
                        {t(($) => $.history.revision_label, { number: revision.revisionNumber })}
                      </p>
                      <p className="break-words text-caption text-muted-foreground">
                        {t(($) => $.history.provenance, {
                          source: revision.sourceKind,
                          actor: revision.actorType,
                        })}
                      </p>
                      <p className="font-mono text-caption text-muted-foreground">{revision.contentDigest}</p>
                    </div>
                    {revision.revisionNumber < currentRevisionNumber ? (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setRestoreId(revision.id)}
                      >
                        <RotateCcw data-icon="inline-start" />
                        {t(($) => $.history.restore)}
                      </Button>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(restoreId)} onOpenChange={(next) => !next && setRestoreId("")}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(($) => $.history.restore_title)}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(($) => $.history.restore_description, {
                number: restoreTarget?.revisionNumber ?? 0,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isRestoring}>{t(($) => $.actions.cancel)}</AlertDialogCancel>
            <AlertDialogAction
              disabled={isRestoring || !restoreId}
              onClick={() => {
                if (!restoreId) return;
                onRestore(restoreId);
                setRestoreId("");
              }}
            >
              {isRestoring ? t(($) => $.states.saving) : t(($) => $.history.restore)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function RevisionSelect({
  label,
  items,
  value,
  onValueChange,
}: {
  label: string;
  items: { value: string; label: string }[];
  value: string;
  onValueChange: (value: string) => void;
}) {
  return (
    <label className="space-y-1.5 text-caption text-muted-foreground">
      <span>{label}</span>
      <Select items={items} value={value || null} onValueChange={(next) => onValueChange(next ?? "")}>
        <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
        <SelectContent>
          {items.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
        </SelectContent>
      </Select>
    </label>
  );
}

function RevisionPreview({ revision }: { revision?: WikiRevision }) {
  const { t } = useT("wiki");
  if (!revision) return <div className="min-h-64 rounded-md bg-muted/40" />;
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-surface-border bg-surface">
      <header className="border-b border-surface-border px-3 py-2">
        <p className="truncate text-body font-medium text-foreground">{revision.title || revision.path}</p>
        <p className="truncate font-mono text-caption text-muted-foreground">{revision.path}</p>
      </header>
      <pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words p-3 font-mono text-caption text-foreground">
        {revision.content || t(($) => $.history.empty_content)}
      </pre>
    </section>
  );
}
