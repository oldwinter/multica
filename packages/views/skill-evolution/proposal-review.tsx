"use client";

import type { SkillEvolutionProposalDetail } from "@multica/core/skill-evolution";
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardCheck,
  Database,
  Loader2,
  MessageSquareText,
  ShieldCheck,
  XCircle,
} from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@multica/ui/components/ui/alert";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { BundleDiff } from "./bundle-diff";
import { isProposalActionable, proposalStatusTone } from "./status";
import { EvolutionStatusBadge } from "./status-badge";

function formatDate(value: string | null | undefined, locale: string, fallback: string) {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function HashValue({ value }: { value: string | null | undefined }) {
  const { t } = useT("skill-evolution");
  return (
    <code className="min-w-0 truncate text-micro" title={value ?? undefined}>
      {value ? value.slice(0, 16) : t(($) => $.page.not_available)}
    </code>
  );
}

function ProposalWarning({ detail }: { detail: SkillEvolutionProposalDetail }) {
  const { t } = useT("skill-evolution");
  const state = detail.proposal.state;
  let title: string | null = null;
  let description: string | null = null;

  switch (state) {
    case "stale":
      title = t(($) => $.status.stale);
      description = detail.proposal.staleReason
        ? t(($) => $.proposal.stale_reason, { reason: detail.proposal.staleReason })
        : t(($) => $.states.proposal_stale);
      break;
    case "failed":
      title = t(($) => $.status.failed);
      description = detail.proposal.failureReason
        ? t(($) => $.proposal.failure_reason, { reason: detail.proposal.failureReason })
        : t(($) => $.states.proposal_failed);
      break;
    case "publication_unknown":
      title = t(($) => $.status.publication_unknown);
      description = t(($) => $.states.publication_unknown);
      break;
    case "unknown":
      title = t(($) => $.status.unknown);
      description = t(($) => $.states.proposal_unknown);
      break;
    default:
      return null;
  }

  return (
    <Alert variant={state === "failed" ? "destructive" : "default"}>
      <AlertTriangle aria-hidden="true" />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{description}</AlertDescription>
    </Alert>
  );
}

function Rationale({ detail }: { detail: SkillEvolutionProposalDetail }) {
  const { t } = useT("skill-evolution");
  if (!detail.rationale) {
    return <p className="py-5 text-caption text-muted-foreground">{t(($) => $.rationale.empty)}</p>;
  }
  const rows = [
    [t(($) => $.rationale.observed_pattern), detail.rationale.observedPattern],
    [t(($) => $.rationale.expected_benefit), detail.rationale.expectedBenefit],
    [t(($) => $.rationale.regression_risk), detail.rationale.regressionRisk],
  ];
  return (
    <dl className="divide-y rounded-md border">
      {rows.map(([label, value]) => (
        <div key={label} className="grid gap-1 px-3 py-3 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-4">
          <dt className="text-caption font-medium text-muted-foreground">{label}</dt>
          <dd className="whitespace-pre-wrap break-words text-body leading-relaxed">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function Evidence({ detail }: { detail: SkillEvolutionProposalDetail }) {
  const { t, i18n } = useT("skill-evolution");
  if (detail.evidence.length === 0) {
    return <p className="text-caption text-muted-foreground">{t(($) => $.states.no_evidence)}</p>;
  }
  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full min-w-[42rem] text-left text-caption">
        <thead className="border-b bg-muted/40 text-muted-foreground">
          <tr>
            <th className="px-3 py-2 font-medium">{t(($) => $.evidence.kind)}</th>
            <th className="px-3 py-2 font-medium">{t(($) => $.evidence.source)}</th>
            <th className="px-3 py-2 font-medium">{t(($) => $.evidence.state)}</th>
            <th className="px-3 py-2 font-medium">{t(($) => $.evidence.digest)}</th>
            <th className="px-3 py-2 font-medium">{t(($) => $.evidence.observed)}</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {detail.evidence.map((item, index) => (
            <tr key={`${item.kind}:${item.sourceId}:${index}`}>
              <td className="px-3 py-2 font-mono">{item.kind}</td>
              <td className="max-w-56 px-3 py-2">
                <div className="truncate font-mono" title={item.sourceId}>{item.sourceId}</div>
                {item.sourceRevisionId ? (
                  <div className="truncate font-mono text-micro text-muted-foreground" title={item.sourceRevisionId}>
                    {item.sourceRevisionId}
                  </div>
                ) : null}
              </td>
              <td className="px-3 py-2 font-mono">{item.sourceState}</td>
              <td className="max-w-40 px-3 py-2 font-mono" title={item.digest}>
                <span className="block truncate">{item.digest}</span>
              </td>
              <td className="whitespace-nowrap px-3 py-2 text-muted-foreground">
                {formatDate(item.observedAt, i18n.language, t(($) => $.page.not_available))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function metricValue(value: unknown, fallback: string): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  if (typeof value === "boolean") return String(value);
  if (value === null) return fallback;
  return fallback;
}

function Evaluations({ detail }: { detail: SkillEvolutionProposalDetail }) {
  const { t, i18n } = useT("skill-evolution");
  if (detail.evaluations.length === 0) {
    return <p className="text-caption text-muted-foreground">{t(($) => $.states.no_evaluations)}</p>;
  }
  return (
    <ol className="divide-y overflow-hidden rounded-md border">
      {detail.evaluations.map((evaluation) => {
        const metrics = Object.entries(evaluation.safeMetrics ?? {});
        return (
          <li key={evaluation.id} className="px-3 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <EvolutionStatusBadge
                value={evaluation.result}
                tone={evaluation.result === "passed" ? "success" : evaluation.result === "failed" ? "danger" : "neutral"}
              />
              <span className="font-mono text-caption">{evaluation.kind}</span>
              <span className="text-caption text-muted-foreground">
                {t(($) => $.evaluation.adapter, {
                  adapter: evaluation.adapter,
                  version: evaluation.adapterVersion,
                })}
              </span>
              <span className="ms-auto text-caption text-muted-foreground">
                {formatDate(evaluation.createdAt, i18n.language, t(($) => $.page.not_available))}
              </span>
            </div>
            <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground">
              <span>{t(($) => $.evaluation.duration, { duration: evaluation.durationMs })}</span>
              {evaluation.costUsdTicks !== null ? (
                <span>{t(($) => $.evaluation.cost, { cost: evaluation.costUsdTicks })}</span>
              ) : null}
              <span className="font-mono" title={evaluation.resultDigest}>
                {evaluation.resultDigest.slice(0, 16)}
              </span>
            </div>
            {metrics.length > 0 ? (
              <dl className="mt-2 grid gap-x-4 gap-y-1 rounded-md bg-muted/40 px-3 py-2 text-caption sm:grid-cols-2">
                {metrics.map(([key, value]) => (
                  <div key={key} className="flex min-w-0 items-center justify-between gap-3">
                    <dt className="truncate text-muted-foreground" title={key}>{key}</dt>
                    <dd className="truncate font-mono" title={metricValue(value, t(($) => $.page.not_available))}>
                      {metricValue(value, t(($) => $.page.not_available))}
                    </dd>
                  </div>
                ))}
              </dl>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}

function Reviews({ detail }: { detail: SkillEvolutionProposalDetail }) {
  const { t, i18n } = useT("skill-evolution");
  if (detail.reviews.length === 0) {
    return <p className="text-caption text-muted-foreground">{t(($) => $.states.no_reviews)}</p>;
  }
  return (
    <ol className="divide-y overflow-hidden rounded-md border">
      {detail.reviews.map((review) => (
        <li key={review.id} className="px-3 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <EvolutionStatusBadge value={review.decision} />
            <span className="min-w-0 truncate font-mono text-caption" title={review.actorId}>
              {t(($) => $.reviews.actor, { actor: review.actorId.slice(0, 12) })}
            </span>
            <span className="ms-auto text-caption text-muted-foreground">
              {formatDate(review.createdAt, i18n.language, t(($) => $.page.not_available))}
            </span>
          </div>
          {review.reason ? (
            <p className="mt-1 whitespace-pre-wrap break-words text-caption text-muted-foreground">
              {t(($) => $.reviews.reason, { reason: review.reason })}
            </p>
          ) : null}
        </li>
      ))}
    </ol>
  );
}

export function ProposalReview({
  detail,
  loading,
  error,
  canReject,
  canPublish,
  rejecting,
  publishing,
  onRetry,
  onReject,
  onPublish,
}: {
  detail: SkillEvolutionProposalDetail | undefined;
  loading: boolean;
  error: Error | null;
  canReject: boolean;
  canPublish: boolean;
  rejecting: boolean;
  publishing: boolean;
  onRetry: () => void;
  onReject: () => void;
  onPublish: () => void;
}) {
  const { t, i18n } = useT("skill-evolution");

  if (loading) {
    return (
      <div className="flex min-h-64 items-center justify-center gap-2 text-body text-muted-foreground">
        <Loader2 className="size-4 animate-spin" aria-hidden="true" />
        {t(($) => $.states.proposal_loading)}
      </div>
    );
  }
  if (error || !detail) {
    return (
      <div className="flex min-h-64 flex-col items-center justify-center gap-2 px-4 text-center">
        <AlertTriangle className="size-6 text-warning" aria-hidden="true" />
        <div className="text-body font-medium">{t(($) => $.states.proposal_error)}</div>
        {error ? <div className="max-w-lg text-caption text-muted-foreground">{error.message}</div> : null}
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          {t(($) => $.page.retry)}
        </Button>
      </div>
    );
  }

  const actionable = isProposalActionable(detail.proposal.state);
  return (
    <div className="divide-y">
      <section className="space-y-4 px-4 py-5 sm:px-6">
        <div className="flex flex-wrap items-start gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-title-sm font-medium">{t(($) => $.proposal.detail)}</h3>
              <EvolutionStatusBadge
                value={detail.proposal.state}
                tone={proposalStatusTone(detail.proposal.state)}
              />
            </div>
            <p className="mt-1 text-caption text-muted-foreground">
              {t(($) => $.proposal.updated, {
                time: formatDate(detail.proposal.updatedAt, i18n.language, t(($) => $.page.not_available)),
              })}
            </p>
          </div>
          {actionable ? (
            <div className="flex shrink-0 items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!canReject || rejecting || publishing}
                onClick={onReject}
                title={!canReject ? t(($) => $.permissions.configure_required) : undefined}
              >
                <XCircle aria-hidden="true" />
                {rejecting ? t(($) => $.actions.rejecting) : t(($) => $.actions.reject)}
              </Button>
              <Button
                type="button"
                size="sm"
                disabled={!canPublish || publishing || rejecting}
                onClick={onPublish}
                title={!canPublish ? t(($) => $.permissions.publish_required) : undefined}
              >
                <CheckCircle2 aria-hidden="true" />
                {publishing ? t(($) => $.actions.publishing) : t(($) => $.actions.publish)}
              </Button>
            </div>
          ) : null}
        </div>
        <div className="grid gap-2 sm:grid-cols-2">
          <div className="flex min-w-0 items-center justify-between gap-3 rounded-md bg-muted/40 px-3 py-2 text-caption">
            <span className="text-muted-foreground">{t(($) => $.proposal.base_hash)}</span>
            <HashValue value={detail.proposal.baseHash} />
          </div>
          <div className="flex min-w-0 items-center justify-between gap-3 rounded-md bg-muted/40 px-3 py-2 text-caption">
            <span className="text-muted-foreground">{t(($) => $.proposal.candidate_hash)}</span>
            <HashValue value={detail.proposal.candidateHash} />
          </div>
        </div>
        <ProposalWarning detail={detail} />
      </section>

      <section aria-labelledby="evolution-rationale" className="px-4 py-5 sm:px-6">
        <div className="mb-3 flex items-center gap-2">
          <MessageSquareText className="size-4 text-muted-foreground" aria-hidden="true" />
          <h3 id="evolution-rationale" className="text-title-sm font-medium">
            {t(($) => $.rationale.title)}
          </h3>
        </div>
        <Rationale detail={detail} />
      </section>

      <section aria-labelledby="evolution-diff" className="px-4 py-5 sm:px-6">
        <div className="mb-3 flex items-center gap-2">
          <ClipboardCheck className="size-4 text-muted-foreground" aria-hidden="true" />
          <h3 id="evolution-diff" className="text-title-sm font-medium">
            {t(($) => $.diff.title)}
          </h3>
        </div>
        <BundleDiff diff={detail.diff} />
      </section>

      <section aria-labelledby="evolution-evidence" className="px-4 py-5 sm:px-6">
        <div className="mb-3 flex items-center gap-2">
          <Database className="size-4 text-muted-foreground" aria-hidden="true" />
          <h3 id="evolution-evidence" className="text-title-sm font-medium">
            {t(($) => $.evidence.title)}
          </h3>
        </div>
        <Evidence detail={detail} />
      </section>

      <section aria-labelledby="evolution-evaluation" className="px-4 py-5 sm:px-6">
        <div className="mb-3 flex items-center gap-2">
          <ShieldCheck className="size-4 text-muted-foreground" aria-hidden="true" />
          <h3 id="evolution-evaluation" className="text-title-sm font-medium">
            {t(($) => $.evaluation.title)}
          </h3>
        </div>
        <Evaluations detail={detail} />
      </section>

      <section aria-labelledby="evolution-reviews" className="px-4 py-5 sm:px-6">
        <div className="mb-3 flex items-center gap-2">
          <MessageSquareText className="size-4 text-muted-foreground" aria-hidden="true" />
          <h3 id="evolution-reviews" className="text-title-sm font-medium">
            {t(($) => $.reviews.title)}
          </h3>
        </div>
        <Reviews detail={detail} />
      </section>
    </div>
  );
}
