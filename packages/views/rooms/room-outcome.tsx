"use client";

import { useState } from "react";
import {
  AlertTriangle,
  Check,
  CircleHelp,
  FileCheck2,
  GitCompareArrows,
  Lightbulb,
  Link2,
  Loader2,
  MessageSquareWarning,
  RotateCw,
  Sparkles,
  X,
} from "lucide-react";
import type {
  RoomDetail,
  RoomOutcomeState,
  RoomRecommendation,
  RoomSynthesis,
  RoomSynthesisItem,
} from "@multica/core/rooms";
import { useActorName } from "@multica/core/workspace/hooks";
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
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";

interface RoomOutcomeProps {
  readonly detail: RoomDetail;
  readonly state: RoomOutcomeState;
  readonly className?: string;
  readonly reviewPending: boolean;
  readonly retryPending: boolean;
  readonly recommendationPending: boolean;
  readonly onReview: (action: "accept" | "reject" | "correct", correction?: RoomSynthesis) => void;
  readonly onRetry: () => void;
  readonly onCitation: (entryId: string) => void;
  readonly onPromoteRecommendation: (
    revisionId: string,
    recommendation: RoomRecommendation,
  ) => void;
  readonly onRejectRecommendation: (revisionId: string, recommendationKey: string) => void;
}

export function RoomOutcome({
  detail,
  state,
  className,
  reviewPending,
  retryPending,
  recommendationPending,
  onReview,
  onRetry,
  onCitation,
  onPromoteRecommendation,
  onRejectRecommendation,
}: RoomOutcomeProps) {
  const { t } = useT("rooms");
  const { getActorName } = useActorName();
  const revision = state.latestOutcome;
  const synthesis = revision?.synthesis;
  const [correctionOpen, setCorrectionOpen] = useState(false);
  const failedTurns = detail.turns.filter((turn) => turn.status === "failed").length;
  const revisionCycle = revision
    ? detail.cycles.find((cycle) => cycle.id === revision.cycle_id)
    : null;
  const promotionReady = revision?.review_status === "accepted" &&
    revisionCycle?.status === "completed" && revisionCycle.phase === "completed";
  const synthesisAttempt = state.latestCycle
    ? Math.max(0, ...detail.turns
      .filter((turn) => turn.cycle_id === state.latestCycle?.id && turn.turn_kind === "synthesis")
      .map((turn) => turn.attempt))
    : 0;

  return (
    <section
      className={cn("min-h-0 overflow-y-auto border-surface-border bg-surface", className)}
      aria-labelledby="room-outcome-heading"
      data-testid="room-outcome"
    >
      <div className="sticky top-0 z-10 flex min-h-11 items-center gap-2 border-b border-surface-border bg-surface px-4">
        <h2 id="room-outcome-heading" className="min-w-0 flex-1 text-body font-medium text-foreground">
          {t(($) => $.outcome.title)}
        </h2>
        <Badge variant="outline">{t(($) => $.phase[state.phase])}</Badge>
        <span className="sr-only" aria-live="polite">
          {t(($) => $.phase[state.phase])}
        </span>
      </div>

      <div className="space-y-5 px-4 py-4">
        <OutcomeGoal detail={detail} />

        {failedTurns > 0 ? (
          <div className="flex gap-2 rounded-md border border-warning/30 bg-warning/5 px-3 py-2 text-caption text-foreground">
            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden="true" />
            <span>{t(($) => $.outcome.partial_failure, { count: failedTurns })}</span>
          </div>
        ) : null}

        {state.latestCycle?.synthesis_error ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-3">
            <div className="flex gap-2">
              <MessageSquareWarning className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden="true" />
              <div className="min-w-0">
                <p className="text-body font-medium text-foreground">{t(($) => $.outcome.synthesis_failed)}</p>
                <p className="mt-1 break-words text-caption text-muted-foreground">
                  {state.latestCycle.synthesis_error.message}
                </p>
                {synthesisAttempt > 0 ? (
                  <p className="mt-1 text-caption text-muted-foreground">
                    {t(($) => $.outcome.synthesis_attempt, { attempt: synthesisAttempt })}
                  </p>
                ) : null}
              </div>
            </div>
            {state.latestCycle.synthesis_error.retryable ? (
              <Button type="button" size="xs" variant="outline" className="mt-3" disabled={retryPending} onClick={onRetry}>
                {retryPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <RotateCw aria-hidden="true" />}
                {t(($) => $.actions.retry_synthesis)}
              </Button>
            ) : null}
          </div>
        ) : null}

        {!synthesis ? (
          <div className="py-8 text-center">
            <Sparkles className="mx-auto size-5 text-muted-foreground" aria-hidden="true" />
            <p className="mt-2 text-body text-muted-foreground">
              {state.phase === "gathering" || state.phase === "synthesizing"
                ? t(($) => $.outcome.in_progress)
                : t(($) => $.outcome.empty)}
            </p>
          </div>
        ) : (
          <>
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-caption font-medium text-muted-foreground">
                  {t(($) => $.detail.memory_version, { version: revision.version })}
                </span>
                <Badge variant="secondary">{t(($) => $.review.status[revision.review_status])}</Badge>
                <Confidence value={synthesis.confidence} />
              </div>
              <p className="mt-1 text-caption text-muted-foreground">
                {t(($) => $.outcome.created_by, {
                  name: getActorName(revision.creator_type, revision.creator_id),
                })}
              </p>
              <p className="mt-2 whitespace-pre-wrap break-words text-body leading-6 text-foreground">
                {synthesis.summary}
              </p>
            </div>

            {(state.memoryDiff.added.length > 0 || state.memoryDiff.removed.length > 0 || state.memoryDiff.summaryChanged) ? (
              <div className="rounded-md border border-surface-border px-3 py-2">
                <h3 className="flex items-center gap-1.5 text-caption font-medium text-muted-foreground">
                  <GitCompareArrows className="size-3.5" aria-hidden="true" />
                  {t(($) => $.outcome.changes)}
                </h3>
                <div className="mt-2 grid gap-2 text-caption sm:grid-cols-2">
                  <DiffList sign="+" items={state.memoryDiff.added} className="text-success" />
                  <DiffList sign="-" items={state.memoryDiff.removed} className="text-destructive" />
                </div>
              </div>
            ) : null}

            <OutcomeList title={t(($) => $.detail.facts)} items={synthesis.facts} detail={detail} onCitation={onCitation} />
            <OutcomeList title={t(($) => $.detail.decisions)} items={synthesis.decisions} detail={detail} onCitation={onCitation} />
            <OutcomeList title={t(($) => $.detail.open_questions)} items={synthesis.open_questions} detail={detail} onCitation={onCitation} icon={CircleHelp} />
            <OutcomeList title={t(($) => $.outcome.disagreements)} items={synthesis.disagreements} detail={detail} onCitation={onCitation} icon={MessageSquareWarning} />
            <OutcomeList title={t(($) => $.outcome.action_items)} items={synthesis.action_items} detail={detail} onCitation={onCitation} icon={FileCheck2} />

            {synthesis.recommendations.length > 0 ? (
              <div>
                <h3 className="mb-2 flex items-center gap-1.5 text-caption font-medium text-muted-foreground">
                  <Lightbulb className="size-3.5" aria-hidden="true" />
                  {t(($) => $.outcome.recommendations)}
                </h3>
                {!promotionReady ? (
                  <p className="mb-2 text-caption text-muted-foreground" id="room-recommendation-promotion-gate">
                    {t(($) => $.recommendation.requires_accepted)}
                  </p>
                ) : null}
                <ul className="space-y-3">
                  {synthesis.recommendations.map((recommendation) => {
                    const review = detail.recommendation_reviews.find(
                      (candidate) => candidate.memory_revision_id === revision.id && candidate.recommendation_key === recommendation.key,
                    );
                    return (
                      <li key={recommendation.key} className="border-l-2 border-brand/40 pl-3">
                        <div className="flex flex-wrap items-start gap-2">
                          <div className="min-w-0 flex-1">
                            <p className="break-words text-body font-medium text-foreground">{recommendation.title}</p>
                            <p className="mt-1 whitespace-pre-wrap break-words text-caption leading-5 text-muted-foreground">{recommendation.rationale}</p>
                          </div>
                          <Badge variant="outline">{t(($) => $.promote.kinds[recommendation.kind])}</Badge>
                        </div>
                        {review ? (
                          <p className="mt-2 text-caption text-muted-foreground" data-testid={`room-recommendation-${recommendation.key}-${review.status}`}>
                            {t(($) => $.recommendation.status[review.status])}
                          </p>
                        ) : (
                          <div className="mt-2 flex flex-wrap gap-2">
                            <Button
                              type="button"
                              size="xs"
                              disabled={recommendationPending || !promotionReady}
                              aria-describedby={!promotionReady ? "room-recommendation-promotion-gate" : undefined}
                              data-testid={`room-approve-recommendation-${recommendation.key}`}
                              onClick={() => onPromoteRecommendation(revision.id, recommendation)}
                            >
                              <Check aria-hidden="true" />
                              {t(($) => $.recommendation.approve)}
                            </Button>
                            <Button
                              type="button"
                              size="xs"
                              variant="outline"
                              disabled={recommendationPending}
                              data-testid={`room-reject-recommendation-${recommendation.key}`}
                              onClick={() => onRejectRecommendation(revision.id, recommendation.key)}
                            >
                              <X aria-hidden="true" />
                              {t(($) => $.recommendation.reject)}
                            </Button>
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : null}

            {revision.review_status === "pending" ? (
              <div className="sticky bottom-0 -mx-4 flex flex-wrap gap-2 border-t border-surface-border bg-surface px-4 py-3" role="group" aria-label={t(($) => $.review.actions_label)}>
                <Button type="button" size="sm" disabled={reviewPending} data-testid="room-accept-outcome" onClick={() => onReview("accept")}>
                  <Check aria-hidden="true" />
                  {t(($) => $.review.accept)}
                </Button>
                <Button type="button" size="sm" variant="outline" disabled={reviewPending} data-testid="room-correct-outcome" onClick={() => setCorrectionOpen(true)}>
                  {t(($) => $.review.correct)}
                </Button>
                <Button type="button" size="sm" variant="ghost" disabled={reviewPending} data-testid="room-reject-outcome" onClick={() => onReview("reject")}>
                  {t(($) => $.review.reject)}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </div>

      {synthesis ? (
        <CorrectionDialog
          key={revision?.id ?? "room-correction"}
          open={correctionOpen}
          synthesis={synthesis}
          pending={reviewPending}
          onOpenChange={setCorrectionOpen}
          onSubmit={(correction) => {
            onReview("correct", correction);
            setCorrectionOpen(false);
          }}
        />
      ) : null}
    </section>
  );
}

function OutcomeGoal({ detail }: { readonly detail: RoomDetail }) {
  const { t } = useT("rooms");
  const objective = detail.room.objective || detail.room.instructions;
  return (
    <div>
      <p className="text-caption font-medium text-muted-foreground">{t(($) => $.outcome.objective)}</p>
      <p className="mt-1 break-words text-body font-medium leading-6 text-foreground">{objective || t(($) => $.outcome.no_objective)}</p>
      {(detail.room.success_criteria.length > 0 || detail.room.stop_conditions.length > 0) ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <PlainList title={t(($) => $.outcome.success_criteria)} items={detail.room.success_criteria} />
          <PlainList title={t(($) => $.outcome.stop_conditions)} items={detail.room.stop_conditions} />
        </div>
      ) : null}
    </div>
  );
}

function PlainList({ title, items }: { readonly title: string; readonly items: readonly string[] }) {
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="text-caption font-medium text-muted-foreground">{title}</h3>
      <ul className="mt-1 space-y-1 text-caption leading-5 text-foreground">
        {items.map((item) => <li key={item} className="break-words">{item}</li>)}
      </ul>
    </div>
  );
}

function OutcomeList({ title, items, detail, onCitation, icon: Icon = Check }: {
  readonly title: string;
  readonly items: readonly RoomSynthesisItem[];
  readonly detail: RoomDetail;
  readonly onCitation: (entryId: string) => void;
  readonly icon?: typeof Check;
}) {
  const { t } = useT("rooms");
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="mb-2 text-caption font-medium text-muted-foreground">{title}</h3>
      <ul className="space-y-2">
        {items.map((item, index) => (
          <li key={`${item.text}-${index}`} className="flex gap-2">
            <Icon className="mt-1 size-3.5 shrink-0 text-brand" aria-hidden="true" />
            <div className="min-w-0 flex-1">
              <p className="break-words text-body leading-5 text-foreground">{item.text}</p>
              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                <Confidence value={item.confidence} />
                {item.citation_entry_ids.map((entryId) => {
                  const ordinal = detail.entries.find((entry) => entry.id === entryId)?.ordinal;
                  return (
                    <button
                      key={entryId}
                      type="button"
                      className="inline-flex items-center gap-1 rounded px-1 py-0.5 text-caption text-brand outline-none hover:bg-brand/10 focus-visible:ring-2 focus-visible:ring-ring"
                      aria-label={t(($) => $.outcome.citation, {
                        citation: ordinal ? `#${ordinal}` : entryId.slice(0, 8),
                      })}
                      onClick={() => onCitation(entryId)}
                    >
                      <Link2 className="size-3" aria-hidden="true" />
                      {ordinal ? `#${ordinal}` : entryId.slice(0, 8)}
                    </button>
                  );
                })}
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Confidence({ value }: { readonly value: number }) {
  const { t } = useT("rooms");
  const level = value >= 0.75 ? "high" : value >= 0.45 ? "medium" : "low";
  return <span className="text-caption text-muted-foreground">{t(($) => $.confidence[level])}</span>;
}

function DiffList({ sign, items, className }: { readonly sign: string; readonly items: readonly string[]; readonly className: string }) {
  if (items.length === 0) return null;
  return <ul className={cn("space-y-1", className)}>{items.map((item) => <li key={item} className="break-words">{sign} {item}</li>)}</ul>;
}

function CorrectionDialog({ open, synthesis, pending, onOpenChange, onSubmit }: {
  readonly open: boolean;
  readonly synthesis: RoomSynthesis;
  readonly pending: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onSubmit: (synthesis: RoomSynthesis) => void;
}) {
  const { t } = useT("rooms");
  const [summary, setSummary] = useState(synthesis.summary);
  const [sections, setSections] = useState(() => editableSections(synthesis));

  const submit = () => {
    onSubmit({
      ...synthesis,
      summary: summary.trim(),
      facts: correctedItems(sections.facts),
      decisions: correctedItems(sections.decisions),
      open_questions: correctedItems(sections.open_questions),
      disagreements: correctedItems(sections.disagreements),
      action_items: correctedItems(sections.action_items),
    });
  };
  return (
    <Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <DialogContent className="max-h-[min(46rem,calc(100dvh-2rem))] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.review.correct_title)}</DialogTitle>
          <DialogDescription>{t(($) => $.review.correct_description)}</DialogDescription>
        </DialogHeader>
        <label className="block space-y-1.5 text-caption font-medium text-muted-foreground">
          <span>{t(($) => $.review.correct_summary)}</span>
          <Textarea autoFocus className="min-h-36 resize-y" value={summary} onChange={(event) => setSummary(event.target.value)} />
        </label>
        <div className="grid gap-4 sm:grid-cols-2">
          <CorrectionSection
            title={t(($) => $.detail.facts)}
            items={sections.facts}
            removeLabel={t(($) => $.review.remove_item)}
            onChange={(facts) => setSections((current) => ({ ...current, facts }))}
          />
          <CorrectionSection
            title={t(($) => $.detail.decisions)}
            items={sections.decisions}
            removeLabel={t(($) => $.review.remove_item)}
            onChange={(decisions) => setSections((current) => ({ ...current, decisions }))}
          />
          <CorrectionSection
            title={t(($) => $.detail.open_questions)}
            items={sections.open_questions}
            removeLabel={t(($) => $.review.remove_item)}
            onChange={(open_questions) => setSections((current) => ({ ...current, open_questions }))}
          />
          <CorrectionSection
            title={t(($) => $.outcome.disagreements)}
            items={sections.disagreements}
            removeLabel={t(($) => $.review.remove_item)}
            onChange={(disagreements) => setSections((current) => ({ ...current, disagreements }))}
          />
          <CorrectionSection
            title={t(($) => $.outcome.action_items)}
            items={sections.action_items}
            removeLabel={t(($) => $.review.remove_item)}
            onChange={(action_items) => setSections((current) => ({ ...current, action_items }))}
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" disabled={pending} onClick={() => onOpenChange(false)}>{t(($) => $.actions.cancel)}</Button>
          <Button type="button" disabled={pending || !summary.trim()} data-testid="room-submit-correction" onClick={submit}>
            {pending ? <Loader2 className="animate-spin" aria-hidden="true" /> : null}
            {t(($) => $.review.submit_correction)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type CorrectableSection = "facts" | "decisions" | "open_questions" | "disagreements" | "action_items";
interface EditableItem {
  readonly source: RoomSynthesisItem;
  readonly text: string;
}
type EditableSections = Record<CorrectableSection, EditableItem[]>;

function editableSections(synthesis: RoomSynthesis): EditableSections {
  return {
    facts: synthesis.facts.map((source) => ({ source, text: source.text })),
    decisions: synthesis.decisions.map((source) => ({ source, text: source.text })),
    open_questions: synthesis.open_questions.map((source) => ({ source, text: source.text })),
    disagreements: synthesis.disagreements.map((source) => ({ source, text: source.text })),
    action_items: synthesis.action_items.map((source) => ({ source, text: source.text })),
  };
}

function correctedItems(
  items: readonly EditableItem[],
): RoomSynthesisItem[] {
  return items.flatMap((item) => {
    const trimmed = item.text.trim();
    if (!trimmed) return [];
    return [{ ...item.source, text: trimmed }];
  });
}

function CorrectionSection({ title, items, removeLabel, onChange }: {
  readonly title: string;
  readonly items: readonly EditableItem[];
  readonly removeLabel: string;
  readonly onChange: (items: EditableItem[]) => void;
}) {
  if (items.length === 0) return null;
  return (
    <fieldset className="min-w-0 space-y-2">
      <legend className="text-caption font-medium text-muted-foreground">{title}</legend>
      {items.map((item, index) => (
        <div key={`${title}-${index}`} className="rounded-md border border-surface-border p-2">
          <div className="flex items-start gap-2">
            <Textarea
              value={item.text}
              className="min-h-20 resize-y"
              aria-label={`${title} ${index + 1}`}
              onChange={(event) => onChange(items.map((candidate, candidateIndex) => candidateIndex === index ? { ...candidate, text: event.target.value } : candidate))}
            />
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              aria-label={removeLabel}
              onClick={() => onChange(items.filter((_, candidateIndex) => candidateIndex !== index))}
            >
              <X aria-hidden="true" />
            </Button>
          </div>
          <p className="mt-1 flex items-center gap-1 break-all text-caption text-muted-foreground">
            <Link2 className="size-3 shrink-0" aria-hidden="true" />
            {item.source.citation_entry_ids.join(", ")}
          </p>
        </div>
      ))}
    </fieldset>
  );
}
