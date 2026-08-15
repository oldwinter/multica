"use client";

import { useState } from "react";
import { CheckCircle2, Hammer, History, ShieldCheck } from "lucide-react";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Separator } from "@multica/ui/components/ui/separator";
import { useT } from "../../i18n";
import { projectTwinAssertions, projectTwinDiff, projectTwinTopics } from "./content-projection";
import { AssertionDiff, CitationList, ContentList } from "./lifecycle-detail";
import { TwinHistorySelectors } from "./lifecycle-selectors";
import { ReviewDialog } from "./review-dialog";
import { TwinReviewSpine } from "./twin-review-spine";
import { TwinTopics } from "./twin-topics";
import type { TwinWorkspaceProps } from "./twin-workspace-types";

export function TwinPanel(props: TwinWorkspaceProps) {
  const { t } = useT("twins");
  const [dialog, setDialog] = useState<"accept-twin" | "reject-twin" | null>(null);
  const proposal = props.proposalDetail?.proposal ?? null;
  const currentVersion = props.twin.current_version;
  const selectedVersionIsCurrent = !props.selectedVersionId || props.selectedVersionId === currentVersion?.id;
  const selectedVersion = props.versionDetail?.version ?? (selectedVersionIsCurrent ? currentVersion : null);
  const proposalPending = proposal !== null && proposal.review === null && proposal.signed_version === null;
  const acceptedWikiId = props.wiki.accepted_revision?.id ?? "";
  const acceptedWikiHasProposal = props.twin.proposals.some(
    (item) => item.source_wiki_revision_id === acceptedWikiId,
  );
  const acceptedWikiIsCurrent = props.twin.current_version?.source_wiki_revision_id === acceptedWikiId;
  const canBuildProposal = props.canManageTwin && Boolean(acceptedWikiId)
    && !acceptedWikiHasProposal && !acceptedWikiIsCurrent;
  const proposalItems = proposal ? projectTwinAssertions(proposal.content) : [];
  const versionItems = selectedVersion ? projectTwinAssertions(selectedVersion.content) : [];

  return (
    <div className="space-y-6">
      <TwinReviewSpine steps={props.reviewSteps} />

      <section className="flex flex-col gap-4 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)] sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <ShieldCheck className="size-4 text-success" aria-hidden="true" />
            <h2 className="text-title font-medium text-foreground">
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
      </section>

      <TwinHistorySelectors
        proposals={props.twin.proposals}
        versions={props.twin.versions}
        proposalId={props.selectedProposalId}
        versionId={props.selectedVersionId}
        onProposalChange={props.onSelectProposal}
        onVersionChange={props.onSelectVersion}
        disabled={props.detailLoading}
      />

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
        </section>
      ) : (
        <p className="text-body text-muted-foreground">
          {acceptedWikiId
            ? t(($) => $.twin.no_proposal)
            : t(($) => $.twin.awaiting_wiki)}
        </p>
      )}

      {selectedVersion ? (
        <section className="space-y-4 rounded-lg border border-surface-border bg-surface p-4 shadow-[var(--surface-shadow)]">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 text-success" aria-hidden="true" />
            <h3 className="text-title font-medium text-foreground">{t(($) => $.twin.version_title, { number: selectedVersion.version_number })}</h3>
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
    </div>
  );
}
