"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { BookCheck, RefreshCw, ShieldAlert } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  parseLMWikiSourcePolicyStale,
  usePinWikiRevisionAsLMWikiEvidence,
  wikiKnowledgeReadinessOptions,
  type LMWikiSourcePolicyStaleConflict,
  type WikiActorType,
  type WikiKnowledgeSourceReadiness,
  type WikiRevision,
  type WikiSourceKind,
} from "@multica/core/wiki";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../i18n";

export interface WikiKnowledgeActivationTarget {
  pageId: string;
  revisionId: string;
  revisionNumber: number;
  title: string;
  path: string;
  contentDigest: string;
  sourceKind: WikiSourceKind;
  actorType: WikiActorType;
}

export function activationTargetFromRevision(revision: WikiRevision): WikiKnowledgeActivationTarget {
  return {
    pageId: revision.pageId,
    revisionId: revision.id,
    revisionNumber: revision.revisionNumber,
    title: revision.title,
    path: revision.path,
    contentDigest: revision.contentDigest,
    sourceKind: revision.sourceKind,
    actorType: revision.actorType,
  };
}

export function WorkspaceWikiKnowledgeActivation({
  target,
}: {
  target: WikiKnowledgeActivationTarget;
}) {
  const { t } = useT("wiki");
  const wsId = useWorkspaceId();
  const readinessQuery = useQuery(wikiKnowledgeReadinessOptions(wsId));
  const pinRevision = usePinWikiRevisionAsLMWikiEvidence(wsId);
  const [open, setOpen] = useState(false);
  const [staleConflict, setStaleConflict] = useState<LMWikiSourcePolicyStaleConflict | null>(null);
  const readiness = readinessQuery.data;
  const source = readiness?.sources.find((candidate) => candidate.pageId === target.pageId);
  const exactRevisionPinned = source?.selectedRevisionId === target.revisionId
    && source.state !== "excluded";
  const canPin = Boolean(readiness?.canManage && source && source.state !== "source_deleted"
    && (source.state !== "excluded" || source.nextAction.kind === "pin_revision")
    && !exactRevisionPinned);

  const openConfirmation = () => {
    setStaleConflict(null);
    setOpen(true);
  };

  const confirmPin = () => {
    if (!readiness || !source) return;
    setStaleConflict(null);
    pinRevision.mutate({
      pageId: target.pageId,
      revisionId: target.revisionId,
      expectedPolicyVersion: readiness.policy.policyVersion,
      expectedPolicyDigest: readiness.policy.policyDigest,
    }, {
      onSuccess: () => setOpen(false),
      onError: (error) => {
        const conflict = error instanceof ApiError
          ? parseLMWikiSourcePolicyStale(error.body)
          : null;
        setStaleConflict(conflict);
      },
    });
  };

  return (
    <div className="flex min-w-0 max-w-sm flex-col items-start gap-1.5" data-testid="wiki-knowledge-activation">
      <SourceStateBadge source={source} loading={readinessQuery.isPending} error={readinessQuery.isError} />
      <Button
        type="button"
        variant={canPin ? "brand" : "outline"}
        size="sm"
        className="max-w-full whitespace-normal text-left"
        disabled={!canPin || pinRevision.isPending}
        onClick={openConfirmation}
      >
        <BookCheck data-icon="inline-start" />
        {exactRevisionPinned
          ? t(($) => $.activation.pinned_exact)
          : t(($) => $.activation.use_revision)}
      </Button>
      {!readinessQuery.isPending && !readinessQuery.isError && readiness && !readiness.canManage ? (
        <p className="break-words text-caption text-muted-foreground">{t(($) => $.activation.manager_only)}</p>
      ) : null}
      {readinessQuery.isError || (!readinessQuery.isPending && !source) ? (
        <p className="break-words text-caption text-destructive" role="alert">
          {t(($) => $.activation.readiness_unavailable)}
        </p>
      ) : null}

      <Dialog open={open} onOpenChange={(next) => {
        setOpen(next);
        if (!next) setStaleConflict(null);
      }}>
        <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto rounded-lg sm:max-w-xl">
          <DialogHeader className="pr-8">
            <DialogTitle>{t(($) => $.activation.confirm_title)}</DialogTitle>
            <DialogDescription>{t(($) => $.activation.confirm_description)}</DialogDescription>
          </DialogHeader>

          {readiness && source ? (
            <>
              <dl className="grid min-w-0 gap-x-5 gap-y-3 border-y border-surface-border py-4 sm:grid-cols-2">
                <ConfirmationField label={t(($) => $.activation.page)} value={target.title || target.path} />
                <ConfirmationField label={t(($) => $.fields.path)} value={target.path} mono />
                <ConfirmationField label={t(($) => $.activation.scope)} value={sourceScopeLabel(source, t)} />
                <ConfirmationField label={t(($) => $.activation.exact_revision)} value={String(target.revisionNumber)} />
                <ConfirmationField label={t(($) => $.revision.digest)} value={target.contentDigest} mono />
                <ConfirmationField
                  label={t(($) => $.revision.provenance)}
                  value={t(($) => $.meta.provenance, { source: target.sourceKind, actor: target.actorType })}
                />
                <ConfirmationField
                  label={t(($) => $.activation.policy_identity)}
                  value={t(($) => $.activation.policy_value, {
                    version: readiness.policy.policyVersion,
                    digest: readiness.policy.policyDigest,
                  })}
                  mono
                />
                <ConfirmationField
                  label={t(($) => $.source_policy.remote_title)}
                  value={readiness.policy.remoteGenerationEnabled
                    ? t(($) => $.activation.remote_enabled)
                    : t(($) => $.activation.remote_disabled)}
                />
              </dl>

              <div className="space-y-2">
                <p className="break-words text-body text-foreground">
                  {readiness.policy.remoteGenerationEnabled
                    ? t(($) => $.activation.egress_enabled)
                    : t(($) => $.activation.egress_disabled)}
                </p>
                <p className="text-caption font-medium text-foreground">
                  {t(($) => $.source_policy.exclusions_title)}
                </p>
                <ul className="space-y-1 text-caption text-muted-foreground">
                  {readiness.policy.exclusions.map((exclusion) => (
                    <li key={`${exclusion.sourceClass}:${exclusion.reason}`} className="break-words">
                      {exclusionSourceLabel(exclusion.sourceClass, t)}: {exclusionReasonLabel(exclusion.reason, t)}
                    </li>
                  ))}
                </ul>
              </div>
            </>
          ) : null}

          {staleConflict ? (
            <div className="space-y-2 rounded-md border border-warning/40 bg-warning/10 p-3" role="alert">
              <p className="flex items-center gap-2 text-body font-medium text-foreground">
                <ShieldAlert className="size-4 shrink-0" aria-hidden="true" />
                {t(($) => $.activation.conflict_title)}
              </p>
              <p className="break-words text-caption text-muted-foreground">
                {t(($) => $.activation.conflict_description, {
                  version: staleConflict.currentPolicy.policyVersion,
                })}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setStaleConflict(null);
                  void readinessQuery.refetch();
                }}
              >
                <RefreshCw data-icon="inline-start" />
                {t(($) => $.activation.reload_policy)}
              </Button>
            </div>
          ) : pinRevision.isError ? (
            <p className="break-words text-body text-destructive" role="alert">
              {t(($) => $.activation.pin_error)}
            </p>
          ) : null}

          <DialogFooter>
            <Button type="button" variant="outline" disabled={pinRevision.isPending} onClick={() => setOpen(false)}>
              {t(($) => $.actions.cancel)}
            </Button>
            <Button type="button" variant="brand" disabled={!readiness || !source || pinRevision.isPending || Boolean(staleConflict)} onClick={confirmPin}>
              <BookCheck data-icon="inline-start" />
              {pinRevision.isPending ? t(($) => $.activation.pinning) : t(($) => $.activation.confirm)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export function PersonalWikiKnowledgeActivation() {
  const { t } = useT("wiki");
  return (
    <div className="flex min-w-0 max-w-sm flex-col items-start gap-1.5" data-testid="wiki-knowledge-activation">
      <Badge variant="outline">{t(($) => $.activation.states.excluded)}</Badge>
      <Button type="button" variant="outline" size="sm" className="max-w-full whitespace-normal text-left" disabled>
        <BookCheck data-icon="inline-start" />
        {t(($) => $.activation.use_revision)}
      </Button>
      <p className="break-words text-caption text-muted-foreground">
        {t(($) => $.activation.personal_excluded)}
      </p>
    </div>
  );
}

function SourceStateBadge({
  source,
  loading,
  error,
}: {
  source?: WikiKnowledgeSourceReadiness;
  loading: boolean;
  error: boolean;
}) {
  const { t } = useT("wiki");
  if (loading) return <Badge variant="outline">{t(($) => $.activation.loading)}</Badge>;
  if (error || !source) return <Badge variant="destructive">{t(($) => $.activation.unavailable)}</Badge>;
  return <Badge variant={source.state === "pinned_current" ? "secondary" : "outline"}>{sourceStateLabel(source, t)}</Badge>;
}

function sourceStateLabel(
  source: WikiKnowledgeSourceReadiness,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  switch (source.state) {
    case "eligible_unpinned": return t(($) => $.activation.states.eligible_unpinned);
    case "pinned_current": return t(($) => $.activation.states.pinned_current);
    case "newer_revision_available": return t(($) => $.activation.states.newer_revision_available);
    case "source_deleted": return t(($) => $.activation.states.source_deleted);
    case "excluded": return t(($) => $.activation.states.excluded);
    case "policy_stale": return t(($) => $.activation.states.policy_stale);
    default: return source.state;
  }
}

function sourceScopeLabel(
  source: WikiKnowledgeSourceReadiness,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  if (source.scope === "workspace") return t(($) => $.scopes.workspace);
  if (source.scope === "project") return t(($) => $.scopes.project);
  return "-";
}

function exclusionSourceLabel(
  sourceClass: string,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  if (sourceClass === "personal_wiki") return t(($) => $.source_policy.exclusion_personal);
  if (sourceClass === "local_only") return t(($) => $.source_policy.exclusion_local_only);
  return sourceClass;
}

function exclusionReasonLabel(
  reason: string,
  t: ReturnType<typeof useT<"wiki">>["t"],
): string {
  if (reason === "personal_scope_never_eligible") {
    return t(($) => $.source_policy.exclusion_personal_reason);
  }
  if (reason === "local_only_never_leaves_owner_daemon") {
    return t(($) => $.source_policy.exclusion_local_only_reason);
  }
  return reason;
}

function ConfirmationField({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className="text-caption text-muted-foreground">{label}</dt>
      <dd className={`mt-1 break-all text-body text-foreground${mono ? " font-mono text-caption" : ""}`}>{value}</dd>
    </div>
  );
}
