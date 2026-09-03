"use client";

import { useState } from "react";
import { CheckCircle2, FileCheck2, Hammer, History, Pencil, ShieldCheck } from "lucide-react";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Separator } from "@multica/ui/components/ui/separator";
import { useT } from "../../i18n";
import { projectTwinAssertions, projectTwinDiff, projectTwinTopics } from "./content-projection";
import { AssertionDiff, CitationList, ContentList } from "./lifecycle-detail";
import { assertionsForDepositionEdit, DepositionEditDialog } from "./deposition-edit-dialog";
import { TwinHistorySelectors } from "./lifecycle-selectors";
import { ReviewDialog } from "./review-dialog";
import { TwinReviewSpine } from "./twin-review-spine";
import { TwinSummaryCopyButton } from "./twin-summary-copy";
import { TwinDestination } from "./twin-guided-navigation";
import { TwinTopics } from "./twin-topics";
import type { TwinWorkspaceProps } from "./twin-workspace-types";
import { DetailStateNotice } from "./workspace-state";

const KNOWN_PROPOSAL_KINDS = ["initial", "evolution", "correction", "deposition"] as const;

export function TwinPanel(props: TwinWorkspaceProps) {
  const { t } = useT("twins");
  const [dialog, setDialog] = useState<"accept-twin" | "reject-twin" | null>(null);
  const [editingProposal, setEditingProposal] = useState(false);
  const proposal = props.proposalDetail?.proposal ?? null;
  const currentVersion = props.twin.current_version;
  const selectedVersionIsCurrent = !props.selectedVersionId || props.selectedVersionId === currentVersion?.id;
  const selectedVersion = props.versionDetail?.version ?? (selectedVersionIsCurrent ? currentVersion : null);
  const proposalPending = proposal !== null
    && KNOWN_PROPOSAL_KINDS.includes(proposal.kind as (typeof KNOWN_PROPOSAL_KINDS)[number])
    && proposal.review === null
    && proposal.signed_version === null;
  const acceptedWikiId = props.wiki.accepted_revision?.id ?? "";
  const acceptedWikiHasProposal = props.twin.proposals.some(
    (item) => item.source_wiki_revision_id === acceptedWikiId,
  );
  const acceptedWikiIsCurrent = props.twin.current_version?.source_wiki_revision_id === acceptedWikiId;
  const canBuildProposal = props.canManageTwin && Boolean(acceptedWikiId)
    && !acceptedWikiHasProposal && !acceptedWikiIsCurrent;
  const proposalItems = proposal ? projectTwinAssertions(proposal.content) : [];
  const versionItems = selectedVersion ? projectTwinAssertions(selectedVersion.content) : [];
  const editableAssertions = proposal ? assertionsForDepositionEdit(proposal.content) : [];
  const proposalCanBeEdited = proposal?.kind === "deposition"
    ? Boolean(props.proposalDetail?.run_evidence)
    : proposal?.kind === "initial" || proposal?.kind === "evolution" || proposal?.kind === "correction";

  return (
    <div className="space-y-6">
      <TwinReviewSpine steps={props.reviewSteps} />

      <TwinDestination
        destination="twin-overview"
        className="flex flex-col gap-4 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)] sm:flex-row sm:items-start sm:justify-between"
        aria-labelledby="twin-overview-title"
      >
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <ShieldCheck className="size-4 text-success" aria-hidden="true" />
            <h2 id="twin-overview-title" className="text-title font-medium text-foreground">
              {currentVersion ? t(($) => $.twin.current_title, { number: currentVersion.version_number }) : t(($) => $.twin.no_current)}
            </h2>
            {proposalPending ? <Badge variant="outline">{t(($) => $.status.pending)}</Badge> : null}
          </div>
          <p className="text-body text-muted-foreground">{t(($) => $.twin.description)}</p>
          {currentVersion ? <p className="break-all font-mono text-caption text-muted-foreground">{currentVersion.content_digest}</p> : null}
        </div>
        {canBuildProposal ? (
          <Button variant="outline" disabled={props.twinMutationPending} onClick={() => props.onEnsureTwin(acceptedWikiId)}>
            <Hammer data-icon="inline-start" />
            {props.twinMutationPending ? t(($) => $.actions.building) : t(($) => $.actions.build_proposal)}
          </Button>
        ) : null}
      </TwinDestination>

      <TwinDestination
        destination="twin-history"
        aria-label={t(($) => $.use.inspect_twin)}
      >
        <TwinHistorySelectors
          proposals={props.twin.proposals}
          versions={props.twin.versions}
          proposalId={props.selectedProposalId}
          versionId={props.selectedVersionId}
          onProposalChange={props.onSelectProposal}
          onVersionChange={props.onSelectVersion}
          disabled={props.detailLoading}
        />
      </TwinDestination>

      <DetailStateNotice state={props.proposalDetailState} onRetry={props.onRetryProposalDetail} />

      {proposal ? (
        <section className="space-y-5 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <History className="size-4 text-muted-foreground" aria-hidden="true" />
                <h3 className="text-title font-medium text-foreground">{t(($) => $.twin.proposal_title)}</h3>
                <Badge variant="outline">{proposal.kind}</Badge>
                {proposal.review ? <Badge variant="outline">{proposal.review.decision}</Badge> : null}
              </div>
              <p className="mt-1 break-all font-mono text-caption text-muted-foreground">{proposal.id}</p>
              {proposal.review?.decision === "rejected" ? (
                <p className="mt-2 text-body text-muted-foreground">{t(($) => $.twin.rejected_guidance)}</p>
              ) : null}
            </div>
            {props.canManageTwin && proposalPending ? (
              <div className="flex flex-wrap gap-2">
                {proposalCanBeEdited ? (
                  <Button
                    variant="outline"
                    disabled={props.twinMutationPending}
                    onClick={() => setEditingProposal(true)}
                  >
                    <Pencil data-icon="inline-start" aria-hidden="true" />
                    {proposal.kind === "deposition" ? t(($) => $.deposition.edit_action) : t(($) => $.correction.edit_action)}
                  </Button>
                ) : null}
                <Button variant="outline" disabled={props.twinMutationPending} onClick={() => setDialog("reject-twin")}>{t(($) => $.actions.reject_proposal)}</Button>
                <Button variant="brand" disabled={props.twinMutationPending} onClick={() => setDialog("accept-twin")}>
                  {props.twinMutationPending ? t(($) => $.actions.saving) : t(($) => $.actions.sign_off)}
                </Button>
              </div>
            ) : null}
          </div>
          <Separator />
          <ContentList items={proposalItems} emptyLabel={t(($) => $.twin.empty_assertions)} />
          <Separator />
          <AssertionDiff diff={projectTwinDiff(proposal.content)} />
          <Separator />
          <TwinTopics topics={projectTwinTopics(proposal.content)} />
          <Separator />
          <CitationList citations={props.proposalDetail?.citations ?? []} />
          {proposal.kind === "deposition" && props.proposalDetail?.run_evidence ? (
            <>
              <Separator />
              <section className="space-y-3" aria-label={t(($) => $.deposition.evidence_title)}>
                <div className="flex flex-wrap items-center gap-2">
                  <FileCheck2 className="size-4 text-muted-foreground" aria-hidden="true" />
                  <h3 className="text-title font-medium text-foreground">{t(($) => $.deposition.evidence_title)}</h3>
                  <Badge variant="outline">{props.proposalDetail.run_evidence.taskStatus}</Badge>
                </div>
                <dl className="grid min-w-0 gap-3 text-caption sm:grid-cols-2">
                  <div className="min-w-0"><dt className="text-muted-foreground">{t(($) => $.deposition.task)}</dt><dd className="break-all font-mono text-foreground">{props.proposalDetail.run_evidence.taskId}</dd></div>
                  <div className="min-w-0"><dt className="text-muted-foreground">{t(($) => $.deposition.base_version)}</dt><dd className="break-all font-mono text-foreground">{props.proposalDetail.run_evidence.baseTwinVersionId}</dd></div>
                  <div className="min-w-0"><dt className="text-muted-foreground">{t(($) => $.deposition.status)}</dt><dd className="break-words text-foreground">{props.proposalDetail.run_evidence.taskStatus || "-"}</dd></div>
                  <div className="min-w-0"><dt className="text-muted-foreground">{t(($) => $.deposition.completed)}</dt><dd className="break-words text-foreground">{props.proposalDetail.run_evidence.completedAt ? new Date(props.proposalDetail.run_evidence.completedAt).toLocaleString() : "-"}</dd></div>
                  <div className="min-w-0 sm:col-span-2"><dt className="text-muted-foreground">{t(($) => $.deposition.feedback)}</dt><dd className="break-words text-foreground">{props.proposalDetail.run_evidence.feedbackRating || "-"}</dd></div>
                  <div className="min-w-0 sm:col-span-2"><dt className="text-muted-foreground">{t(($) => $.deposition.evidence_digest)}</dt><dd className="break-all font-mono text-foreground">{props.proposalDetail.run_evidence.evidenceDigest}</dd></div>
                </dl>
                <p className="text-caption text-muted-foreground">{t(($) => $.deposition.sanitized_notice)}</p>
              </section>
            </>
          ) : null}
        </section>
      ) : props.proposalDetailState.kind === "none" || props.proposalDetailState.kind === "ready" ? (
        <p className="text-body text-muted-foreground">
          {acceptedWikiId
            ? t(($) => $.twin.no_proposal)
            : t(($) => $.twin.awaiting_wiki)}
        </p>
      ) : null}

      <DetailStateNotice state={props.versionDetailState} onRetry={props.onRetryVersionDetail} />
      {selectedVersion ? (
        <section className="space-y-4 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-center gap-2">
              <CheckCircle2 className="size-4 shrink-0 text-success" aria-hidden="true" />
              <h3 className="min-w-0 break-words text-title font-medium text-foreground">{t(($) => $.twin.version_title, { number: selectedVersion.version_number })}</h3>
            </div>
            <TwinSummaryCopyButton
              heading={t(($) => $.twin.version_title, { number: selectedVersion.version_number })}
              digest={selectedVersion.content_digest}
              items={versionItems}
            />
          </div>
          <ContentList items={versionItems} emptyLabel={t(($) => $.twin.empty_assertions)} />
          <Separator />
          <TwinTopics topics={projectTwinTopics(selectedVersion.content)} />
          <Separator />
          <CitationList citations={props.versionDetail?.citations ?? []} />
        </section>
      ) : null}

      <ReviewDialog open={dialog !== null} kind={dialog ?? "accept-twin"} pending={props.twinMutationPending} onOpenChange={(open) => !open && setDialog(null)} onConfirm={(reason) => {
        if (!proposal) return Promise.resolve();
        if (dialog === "reject-twin") return props.onRejectTwin(proposal.id, reason);
        return props.onAcceptTwin(proposal.id);
      }} />
      {editingProposal && proposal ? (
        <DepositionEditDialog
          assertions={editableAssertions}
          mode={proposal.kind === "deposition" ? "deposition" : "proposal"}
          pending={props.twinMutationPending}
          onOpenChange={setEditingProposal}
          onSubmit={(assertions) => proposal.kind === "deposition"
            ? props.onEditDeposition(proposal.id, assertions)
            : props.onCorrectTwin(proposal.id, assertions)}
        />
      ) : null}
    </div>
  );
}
