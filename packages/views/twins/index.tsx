"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWikiTwinPermissions } from "@multica/core/permissions";
import {
  twinOverviewOptions,
  twinProfileOverviewOptions,
  twinProposalOptions,
  twinVersionOptions,
  useAcceptLMWikiRevision,
  useAcceptTwinProposal,
  useEnsureTwinProposal,
  useRefreshLMWiki,
  useRejectLMWikiRevision,
  useRejectTwinProposal,
  wikiOverviewOptions,
  wikiRevisionOptions,
  type LMWikiOverview,
  type TwinOverview,
} from "@multica/core/twins";
import { TwinWorkspaceView, type TwinViewState } from "./components/twin-workspace-view";

const EMPTY_WIKI: LMWikiOverview = {
  latest_revision: null,
  accepted_revision: null,
  pending_revision: null,
  revisions: [],
  can_manage: false,
};

const EMPTY_TWIN: TwinOverview = {
  current_version: null,
  pending_proposal: null,
  proposals: [],
  versions: [],
  can_manage: false,
};

function messageFrom(errors: readonly unknown[]): string | null {
  for (const error of errors) {
    if (error instanceof Error && error.message) return error.message;
  }
  return null;
}

export type { TwinViewState } from "./components/twin-workspace-view";
export { TwinWorkspaceView } from "./components/twin-workspace-view";

export function TwinsPage() {
  const wsId = useWorkspaceId();
  const [revisionId, setRevisionId] = useState("");
  const [proposalId, setProposalId] = useState("");
  const [versionId, setVersionId] = useState("");

  const wikiQuery = useQuery(wikiOverviewOptions(wsId));
  const twinQuery = useQuery(twinOverviewOptions(wsId));
  const twinProfileQuery = useQuery(twinProfileOverviewOptions(wsId));
  const wiki = wikiQuery.data ?? EMPTY_WIKI;
  const twin = twinQuery.data ?? EMPTY_TWIN;
  const selectedRevisionId = revisionId || wiki.pending_revision?.id || wiki.accepted_revision?.id || wiki.latest_revision?.id || "";
  const selectedProposalId = proposalId || twin.pending_proposal?.id || twin.proposals[0]?.id || "";
  const selectedVersionId = versionId || twin.current_version?.id || twin.versions[0]?.id || "";
  const wikiDetailQuery = useQuery(wikiRevisionOptions(wsId, selectedRevisionId));
  const proposalDetailQuery = useQuery(twinProposalOptions(wsId, selectedProposalId));
  const versionDetailQuery = useQuery(twinVersionOptions(wsId, selectedVersionId));

  const wikiPermissions = useWikiTwinPermissions(wsId, wiki.can_manage === true);
  const twinPermissions = useWikiTwinPermissions(wsId, twin.can_manage === true);
  const refreshWiki = useRefreshLMWiki(wsId);
  const acceptWiki = useAcceptLMWikiRevision(wsId);
  const rejectWiki = useRejectLMWikiRevision(wsId);
  const ensureTwin = useEnsureTwinProposal(wsId);
  const acceptTwin = useAcceptTwinProposal(wsId);
  const rejectTwin = useRejectTwinProposal(wsId);

  const overviewLoading = wikiQuery.isPending || twinQuery.isPending || twinProfileQuery.isPending
    || wikiPermissions.isLoading || twinPermissions.isLoading;
  const state: TwinViewState = overviewLoading
    ? "loading"
    : wikiQuery.isError || twinQuery.isError || twinProfileQuery.isError ? "error" : "ready";
  const wikiMutationPending = refreshWiki.isPending || acceptWiki.isPending || rejectWiki.isPending;
  const twinMutationPending = ensureTwin.isPending || acceptTwin.isPending || rejectTwin.isPending;
  const actionError = messageFrom([
    refreshWiki.error,
    acceptWiki.error,
    rejectWiki.error,
    ensureTwin.error,
    acceptTwin.error,
    rejectTwin.error,
    wikiDetailQuery.error,
    proposalDetailQuery.error,
    versionDetailQuery.error,
  ]);

  return (
    <TwinWorkspaceView
      state={state}
      wiki={wiki}
      wikiDetail={wikiDetailQuery.data ?? null}
      twin={twin}
      proposalDetail={proposalDetailQuery.data ?? null}
      versionDetail={versionDetailQuery.data ?? null}
      reviewSteps={twinProfileQuery.data?.reviewSteps ?? []}
      selectedRevisionId={selectedRevisionId}
      selectedProposalId={selectedProposalId}
      selectedVersionId={selectedVersionId}
      canManageWiki={wikiPermissions.canMutate}
      canManageTwin={twinPermissions.canMutate}
      wikiMutationPending={wikiMutationPending}
      twinMutationPending={twinMutationPending}
      detailLoading={
        (selectedRevisionId.length > 0 && wikiDetailQuery.isPending) ||
        (selectedProposalId.length > 0 && proposalDetailQuery.isPending) ||
        (selectedVersionId.length > 0 && versionDetailQuery.isPending)
      }
      actionError={actionError}
      onSelectRevision={setRevisionId}
      onSelectProposal={setProposalId}
      onSelectVersion={setVersionId}
      onRefreshWiki={() => refreshWiki.mutate()}
      onAcceptWiki={(id) => acceptWiki.mutate(id)}
      onRejectWiki={(id, reason) => rejectWiki.mutate({ revisionId: id, reason })}
      onEnsureTwin={(id) => ensureTwin.mutate(id)}
      onAcceptTwin={(id) => acceptTwin.mutate(id)}
      onRejectTwin={(id, reason) => rejectTwin.mutate({ proposalId: id, reason })}
      onRetry={() => {
        wikiQuery.refetch();
        twinQuery.refetch();
        twinProfileQuery.refetch();
      }}
    />
  );
}
