"use client";

import { useEffect, useMemo, useState } from "react";
import { BookLock, Database } from "lucide-react";
import type {
  LMWikiSourceClass,
  LMWikiSourcePolicy,
  UpdateLMWikiSourcePolicyInput,
  WikiPageSummary,
  WikiRevision,
} from "@multica/core/wiki";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "../i18n";

const SOURCE_CLASSES: readonly LMWikiSourceClass[] = [
  "issue",
  "project",
  "project_resource",
  "autopilot_run",
  "wiki_page",
];

export interface LMWikiSourcePolicyPanelProps {
  policy: LMWikiSourcePolicy | null;
  pages: readonly WikiPageSummary[];
  revisionsByPage?: Readonly<Record<string, readonly WikiRevision[]>>;
  canManage: boolean;
  canManageRemoteGeneration: boolean;
  isLoading?: boolean;
  isError?: boolean;
  isSaving?: boolean;
  saved?: boolean;
  errorMessage?: string | null;
  conflictPolicyVersion?: number | null;
  onRetry?: () => void;
  onResolveConflict?: () => void;
  onPageSelectionChange?: (pageId: string, enabled: boolean) => void;
  onSave: (policy: UpdateLMWikiSourcePolicyInput) => void;
}

export function LMWikiSourcePolicyPanel({
  policy,
  pages,
  revisionsByPage = {},
  canManage,
  canManageRemoteGeneration,
  isLoading = false,
  isError = false,
  isSaving = false,
  saved = false,
  errorMessage,
  conflictPolicyVersion,
  onRetry,
  onResolveConflict,
  onPageSelectionChange,
  onSave,
}: LMWikiSourcePolicyPanelProps) {
  const { t } = useT("wiki");
  const [sourceClasses, setSourceClasses] = useState<readonly LMWikiSourceClass[]>([]);
  const [selectedRevisions, setSelectedRevisions] = useState<Record<string, number>>({});
  const [remoteGenerationEnabled, setRemoteGenerationEnabled] = useState(false);

  useEffect(() => {
    setSourceClasses(policy?.sourceClasses ?? []);
    setSelectedRevisions(Object.fromEntries(
      (policy?.wikiPages ?? []).map((selection) => [selection.pageId, selection.revisionNumber]),
    ));
    setRemoteGenerationEnabled(policy?.remoteGenerationEnabled === true);
  }, [policy]);

  const orderedPages = useMemo(
    () => [...pages].sort((a, b) => {
      const scopeOrder = { workspace: 0, project: 1, user: 2 };
      return scopeOrder[a.scope] - scopeOrder[b.scope] || a.path.localeCompare(b.path);
    }),
    [pages],
  );

  if (isLoading) {
    return <p className="py-10 text-center text-body text-muted-foreground" role="status">{t(($) => $.source_policy.loading)}</p>;
  }
  if (isError || !policy) {
    return (
      <div className="space-y-3 py-10 text-center">
        <p className="text-body text-destructive" role="alert">{t(($) => $.source_policy.error)}</p>
        {onRetry ? <Button variant="outline" onClick={onRetry}>{t(($) => $.actions.retry)}</Button> : null}
      </div>
    );
  }

  const toggleClass = (sourceClass: LMWikiSourceClass, enabled: boolean) => {
    setSourceClasses((current) => enabled
      ? [...new Set([...current, sourceClass])]
      : current.filter((value) => value !== sourceClass));
  };
  const togglePage = (page: WikiPageSummary, enabled: boolean) => {
    onPageSelectionChange?.(page.id, enabled);
    setSelectedRevisions((current) => {
      const next = { ...current };
      if (enabled) next[page.id] = page.currentRevisionNumber;
      else delete next[page.id];
      return next;
    });
  };
  const wikiPageClassEnabled = sourceClasses.includes("wiki_page");

  return (
    <div className="space-y-6" data-testid="lm-wiki-source-policy">
      <header className="space-y-1">
        <h2 className="flex items-center gap-2 text-title-sm font-medium text-foreground">
          <Database className="size-4" aria-hidden="true" />
          {t(($) => $.source_policy.title)}
        </h2>
        <p className="max-w-3xl break-words text-body text-muted-foreground">{t(($) => $.source_policy.description)}</p>
        {!canManage ? (
          <p className="flex items-center gap-2 text-caption text-muted-foreground">
            <BookLock className="size-3.5" aria-hidden="true" />
            {t(($) => $.source_policy.read_only)}
          </p>
        ) : null}
      </header>

      <section className="space-y-3" aria-labelledby="lm-wiki-remote-generation">
        <div className="flex min-w-0 flex-col gap-3 rounded-md bg-muted/40 p-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <h3 id="lm-wiki-remote-generation" className="break-words text-body font-medium text-foreground">
              {t(($) => $.source_policy.remote_title)}
            </h3>
            <p className="break-words text-caption text-muted-foreground">
              {t(($) => $.source_policy.remote_description)}
            </p>
            <p className="break-words text-caption text-muted-foreground">
              {t(($) => $.source_policy.remote_exclusions)}
            </p>
            {policy.exclusions.length > 0 ? (
              <ul className="space-y-2 pt-1" aria-label={t(($) => $.source_policy.exclusions_title)}>
                {policy.exclusions.map((exclusion, index) => (
                  <li key={`${exclusion.sourceClass}:${exclusion.state}:${index}`} className="min-w-0 text-caption text-muted-foreground">
                    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                      <span className="break-words font-medium text-foreground">
                        {exclusionClassLabel(exclusion.sourceClass, t)}
                      </span>
                      <Badge variant="outline" className="shrink-0">
                        {exclusionStateLabel(exclusion.state, t)}
                      </Badge>
                    </div>
                    <p className="break-words">{exclusionReasonLabel(exclusion.reason, t)}</p>
                  </li>
                ))}
              </ul>
            ) : null}
            {!canManageRemoteGeneration ? (
              <p className="break-words text-caption text-muted-foreground">
                {t(($) => $.source_policy.remote_owner_only)}
              </p>
            ) : null}
          </div>
          <Switch
            className="shrink-0"
            checked={remoteGenerationEnabled}
            onCheckedChange={setRemoteGenerationEnabled}
            disabled={!canManageRemoteGeneration || isSaving}
            aria-label={t(($) => $.source_policy.remote_label)}
          />
        </div>
      </section>

      <section className="space-y-3" aria-labelledby="lm-wiki-source-classes">
        <h3 id="lm-wiki-source-classes" className="text-body font-medium text-foreground">{t(($) => $.source_policy.classes)}</h3>
        <div className="grid gap-2 sm:grid-cols-2">
          {SOURCE_CLASSES.map((sourceClass) => (
            <div key={sourceClass} className="flex min-h-10 items-center justify-between gap-4 rounded-md bg-muted/40 px-3 py-2 text-body text-foreground">
              <span className="break-words">{sourceClassLabel(sourceClass, t)}</span>
              <Switch
                checked={sourceClasses.includes(sourceClass)}
                onCheckedChange={(checked) => toggleClass(sourceClass, checked)}
                disabled={!canManage || isSaving}
                aria-label={sourceClassLabel(sourceClass, t)}
              />
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-3 border-t border-surface-border pt-5" aria-labelledby="lm-wiki-source-pages">
        <h3 id="lm-wiki-source-pages" className="text-body font-medium text-foreground">{t(($) => $.source_policy.pages)}</h3>
        {orderedPages.length === 0 ? (
          <p className="text-body text-muted-foreground">{t(($) => $.source_policy.empty_pages)}</p>
        ) : (
          <ul className="divide-y divide-surface-border">
            {orderedPages.map((page) => {
              const eligible = page.scope !== "user";
              const selectedRevisionNumber = selectedRevisions[page.id];
              const checked = selectedRevisionNumber !== undefined;
              const revisions = revisionsByPage[page.id] ?? [];
              const revisionItems = revisionSelectItems(page, revisions, selectedRevisionNumber, t);
              const selectedRevision = revisions.find((revision) => revision.revisionNumber === selectedRevisionNumber);
              const digest = selectedRevision?.contentDigest
                ?? (selectedRevisionNumber === page.currentRevisionNumber ? page.contentDigest : "");
              const source = selectedRevision?.sourceKind ?? page.lastSourceKind;
              const actor = selectedRevision?.actorType ?? page.lastActorType;

              return (
                <li key={page.id} className="flex flex-col gap-3 py-3 sm:flex-row sm:items-start">
                  <div className="flex min-w-0 flex-1 items-start gap-3">
                    <Checkbox
                      checked={checked}
                      onCheckedChange={(next) => togglePage(page, next === true)}
                      disabled={!eligible || !wikiPageClassEnabled || !canManage || isSaving}
                      aria-label={page.title || page.path}
                      className="mt-1"
                    />
                    <div className="min-w-0 space-y-1">
                      <p className="break-words text-body font-medium text-foreground">{page.title || page.path}</p>
                      <p className="break-all font-mono text-caption text-muted-foreground">{page.path}</p>
                      <p className="flex flex-wrap gap-1.5 text-caption text-muted-foreground">
                        <Badge variant="outline">{t(($) => $.scopes[page.scope])}</Badge>
                        <Badge variant={eligible ? "secondary" : "outline"}>
                          {eligible ? t(($) => $.source_policy.eligible) : t(($) => $.source_policy.excluded)}
                        </Badge>
                      </p>
                      {!eligible ? (
                        <p className="break-words text-caption text-muted-foreground">{t(($) => $.source_policy.personal_excluded)}</p>
                      ) : checked ? (
                        <div className="space-y-0.5 text-caption text-muted-foreground">
                          <p>{t(($) => $.source_policy.provenance, { source, actor })}</p>
                          {digest ? <p className="break-all font-mono">{t(($) => $.source_policy.digest, { digest })}</p> : null}
                        </div>
                      ) : null}
                    </div>
                  </div>
                  {eligible && checked ? (
                    <Select
                      items={revisionItems}
                      value={String(selectedRevisionNumber)}
                      onValueChange={(value) => setSelectedRevisions((current) => ({
                        ...current,
                        [page.id]: Number(value ?? page.currentRevisionNumber),
                      }))}
                      disabled={!wikiPageClassEnabled || !canManage || isSaving}
                    >
                      <SelectTrigger className="w-full sm:w-44" aria-label={t(($) => $.source_policy.revision, { number: selectedRevisionNumber })}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {revisionItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
                      </SelectContent>
                    </Select>
                  ) : null}
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {conflictPolicyVersion !== undefined && conflictPolicyVersion !== null ? (
        <div className="flex flex-col items-start gap-2 rounded-md border border-warning/40 bg-warning/10 p-3" role="alert">
          <p className="break-words text-body font-medium text-foreground">
            {t(($) => $.source_policy.conflict_title)}
          </p>
          <p className="break-words text-caption text-muted-foreground">
            {t(($) => $.source_policy.conflict_description, { version: conflictPolicyVersion })}
          </p>
          {onResolveConflict ? (
            <Button type="button" variant="outline" size="sm" onClick={onResolveConflict}>
              {t(($) => $.source_policy.reload_policy)}
            </Button>
          ) : null}
        </div>
      ) : errorMessage ? <p className="text-body text-destructive" role="alert">{errorMessage}</p> : null}
      {saved ? <p className="text-body text-success" role="status">{t(($) => $.source_policy.saved)}</p> : null}
      <Button
        disabled={!canManage || isSaving}
        onClick={() => onSave({
          sourceClasses: sourceClasses,
          wikiPages: wikiPageClassEnabled
            ? Object.entries(selectedRevisions)
              .map(([pageId, revisionNumber]) => ({ pageId, revisionNumber }))
              .sort((a, b) => a.pageId.localeCompare(b.pageId))
            : [],
          remoteGenerationEnabled,
          expectedPolicyVersion: policy.policyVersion,
          expectedPolicyDigest: policy.policyDigest,
        })}
      >
        {isSaving ? t(($) => $.source_policy.saving) : t(($) => $.source_policy.save)}
      </Button>
    </div>
  );
}

function revisionSelectItems(
  page: WikiPageSummary,
  revisions: readonly WikiRevision[],
  selectedRevisionNumber: number | undefined,
  t: ReturnType<typeof useT<"wiki">>["t"],
) {
  const numbers = new Set(revisions.map((revision) => revision.revisionNumber));
  numbers.add(page.currentRevisionNumber);
  if (selectedRevisionNumber !== undefined) numbers.add(selectedRevisionNumber);
  return [...numbers]
    .sort((a, b) => b - a)
    .map((number) => ({
      value: String(number),
      label: t(($) => $.source_policy.revision, { number }),
    }));
}

function sourceClassLabel(sourceClass: LMWikiSourceClass, t: ReturnType<typeof useT<"wiki">>["t"]): string {
  switch (sourceClass) {
    case "issue": return t(($) => $.source_policy.classes_issue);
    case "project": return t(($) => $.source_policy.classes_project);
    case "project_resource": return t(($) => $.source_policy.classes_project_resource);
    case "autopilot_run": return t(($) => $.source_policy.classes_autopilot_run);
    case "wiki_page": return t(($) => $.source_policy.classes_wiki_page);
    default: return sourceClass;
  }
}

function exclusionClassLabel(sourceClass: string, t: ReturnType<typeof useT<"wiki">>["t"]): string {
  switch (sourceClass) {
    case "personal_wiki": return t(($) => $.source_policy.exclusion_personal);
    case "local_only": return t(($) => $.source_policy.exclusion_local_only);
    default: return sourceClass;
  }
}

function exclusionStateLabel(state: string, t: ReturnType<typeof useT<"wiki">>["t"]): string {
  return state === "always_excluded"
    ? t(($) => $.source_policy.exclusion_always)
    : state;
}

function exclusionReasonLabel(reason: string, t: ReturnType<typeof useT<"wiki">>["t"]): string {
  switch (reason) {
    case "personal_scope_never_eligible": return t(($) => $.source_policy.exclusion_personal_reason);
    case "local_only_never_leaves_owner_daemon": return t(($) => $.source_policy.exclusion_local_only_reason);
    default: return reason;
  }
}
