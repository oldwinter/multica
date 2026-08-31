"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWikiTwinPermissions } from "@multica/core/permissions";
import {
  twinOverviewOptions,
  twinProfileOverviewOptions,
  twinProposalOptions,
  twinVersionOptions,
  useAcceptLMWikiRevision,
  useAcceptTwinProposal,
  useCorrectTwinProposal,
  useCreateTwinDeposition,
  useEnsureTwinProposal,
  useRefreshLMWiki,
  useRejectLMWikiRevision,
  useRejectTwinProposal,
  wikiOverviewOptions,
  wikiRevisionOptions,
  type LMWikiOverview,
  type LifecycleContent,
  type TwinOverview,
} from "@multica/core/twins";
import { TwinWorkspaceView, type TwinDetailState, type TwinViewState } from "./components/twin-workspace-view";
import { LMWikiSourcePolicyContainer } from "../wiki/lm-wiki-source-policy-container";
import { useT } from "../i18n";

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

function messageFrom(
  errors: readonly unknown[],
  timeoutMessage: string,
  staleMessage: string,
  fallbackMessage: string,
): string | null {
  for (const error of errors) {
    if (error instanceof Error && error.name === "TimeoutError") return timeoutMessage;
    if (isStaleConflict(error)) return staleMessage;
    if (error instanceof Error) return fallbackMessage;
  }
  return null;
}

function isStaleConflict(error: unknown): boolean {
  return error instanceof ApiError && error.status === 409;
}

function detailQueryState(selectedId: string, query: { isPending: boolean; isError: boolean; data: unknown }): TwinDetailState {
  if (!selectedId) return { kind: "none" };
  if (query.isPending) return { kind: "loading" };
  if (query.isError) return query.data === undefined ? { kind: "error" } : { kind: "stale" };
  return { kind: "ready" };
}

export type { TwinViewState } from "./components/twin-workspace-view";
export { TwinWorkspaceView } from "./components/twin-workspace-view";

export function TwinsPage({ rootElement = "main" }: { rootElement?: "main" | "div" } = {}) {
  const { t } = useT("twins");
  const wsId = useWorkspaceId();
  const [revisionId, setRevisionId] = useState("");
  const [proposalId, setProposalId] = useState("");
  const [versionId, setVersionId] = useState("");
  const actionSequence = useRef(0);
  const [actionFailure, setActionFailure] = useState<{ attempt: number; error: unknown } | null>(null);

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
  const wikiDetailState = detailQueryState(selectedRevisionId, wikiDetailQuery);
  const proposalDetailState = detailQueryState(selectedProposalId, proposalDetailQuery);
  const versionDetailState = detailQueryState(selectedVersionId, versionDetailQuery);

  const wikiPermissions = useWikiTwinPermissions(wsId, wiki.can_manage === true);
  const twinPermissions = useWikiTwinPermissions(wsId, twin.can_manage === true);
  const refreshWiki = useRefreshLMWiki(wsId);
  const acceptWiki = useAcceptLMWikiRevision(wsId);
  const rejectWiki = useRejectLMWikiRevision(wsId);
  const ensureTwin = useEnsureTwinProposal(wsId);
  const acceptTwin = useAcceptTwinProposal(wsId);
  const correctTwin = useCorrectTwinProposal(wsId);
  const rejectTwin = useRejectTwinProposal(wsId);
  const editDeposition = useCreateTwinDeposition(
    wsId,
    proposalDetailQuery.data?.run_evidence?.taskId ?? "",
  );
  const beginLifecycleAction = () => {
    const attempt = ++actionSequence.current;
    setActionFailure(null);
    return attempt;
  };
  const completeLifecycleAction = (attempt: number) => {
    setActionFailure((current) => current?.attempt === attempt ? null : current);
  };
  const failLifecycleAction = (attempt: number, error: unknown) => {
    setActionFailure((current) => current && current.attempt > attempt ? current : { attempt, error });
  };

  const overviewLoading = wikiQuery.isPending || twinQuery.isPending || twinProfileQuery.isPending
    || wikiPermissions.isLoading || twinPermissions.isLoading;
  const overviewMissing = (wikiQuery.isError && wikiQuery.data === undefined)
    || (twinQuery.isError && twinQuery.data === undefined)
    || (twinProfileQuery.isError && twinProfileQuery.data === undefined);
  const overviewStale = !overviewMissing
    && ((wikiQuery.isError && wikiQuery.data !== undefined)
      || (twinQuery.isError && twinQuery.data !== undefined)
      || (twinProfileQuery.isError && twinProfileQuery.data !== undefined));
  const state: TwinViewState = overviewLoading
    ? "loading"
    : overviewMissing ? "error" : "ready";
  const wikiMutationPending = refreshWiki.isPending || acceptWiki.isPending || rejectWiki.isPending;
  const twinMutationPending = ensureTwin.isPending || acceptTwin.isPending || rejectTwin.isPending
    || correctTwin.isPending
    || editDeposition.isPending;
  const actionError = messageFrom(
    [actionFailure?.error],
    t(($) => $.errors.request_timed_out),
    t(($) => $.errors.stale_review),
    t(($) => $.errors.request_failed),
  );

  return (
    <TwinWorkspaceView
      wsId={wsId}
      rootElement={rootElement}
      state={state}
      overviewStale={overviewStale}
      wiki={wiki}
      wikiDetail={wikiDetailQuery.data ?? null}
      wikiDetailState={wikiDetailState}
      twin={twin}
      proposalDetail={proposalDetailQuery.data ?? null}
      proposalDetailState={proposalDetailState}
      versionDetail={versionDetailQuery.data ?? null}
      versionDetailState={versionDetailState}
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
      sourcePolicyPanel={<LMWikiSourcePolicyContainer canManage={wikiPermissions.canMutate} />}
      onSelectRevision={setRevisionId}
      onSelectProposal={setProposalId}
      onSelectVersion={setVersionId}
      onRetryWikiDetail={() => { void wikiDetailQuery.refetch(); }}
      onRetryProposalDetail={() => { void proposalDetailQuery.refetch(); }}
      onRetryVersionDetail={() => { void versionDetailQuery.refetch(); }}
      onRefreshWiki={() => {
        const attempt = beginLifecycleAction();
        refreshWiki.mutate(undefined, {
          onSuccess: (result) => {
            completeLifecycleAction(attempt);
            setRevisionId(result.revision.id);
          },
          onError: (error) => failLifecycleAction(attempt, error),
        });
      }}
      onAcceptWiki={async (id) => {
        const attempt = beginLifecycleAction();
        try {
          await acceptWiki.mutateAsync(id);
          completeLifecycleAction(attempt);
          setProposalId("");
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setRevisionId("");
          throw error;
        }
      }}
      onRejectWiki={async (id, reason) => {
        const attempt = beginLifecycleAction();
        try {
          await rejectWiki.mutateAsync({ revisionId: id, reason });
          completeLifecycleAction(attempt);
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setRevisionId("");
          throw error;
        }
      }}
      onEnsureTwin={(id) => {
        const attempt = beginLifecycleAction();
        ensureTwin.mutate(id, {
          onSuccess: (result) => {
            completeLifecycleAction(attempt);
            setProposalId(result.proposal.id);
          },
          onError: (error) => failLifecycleAction(attempt, error),
        });
      }}
      onAcceptTwin={async (id) => {
        const attempt = beginLifecycleAction();
        try {
          const result = await acceptTwin.mutateAsync(id);
          completeLifecycleAction(attempt);
          setVersionId(result.version.id);
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setProposalId("");
          throw error;
        }
      }}
      onRejectTwin={async (id, reason) => {
        const attempt = beginLifecycleAction();
        try {
          await rejectTwin.mutateAsync({ proposalId: id, reason });
          completeLifecycleAction(attempt);
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setProposalId("");
          throw error;
        }
      }}
      onCorrectTwin={async (replacesProposalId, editedAssertions: readonly LifecycleContent[]) => {
        const attempt = beginLifecycleAction();
        try {
          const result = await correctTwin.mutateAsync({ proposalId: replacesProposalId, editedAssertions });
          completeLifecycleAction(attempt);
          setProposalId(result.proposal.id);
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setProposalId("");
          throw error;
        }
      }}
      onEditDeposition={async (replacesProposalId, editedAssertions: readonly LifecycleContent[]) => {
        const attempt = beginLifecycleAction();
        try {
          const result = await editDeposition.mutateAsync({ replacesProposalId, editedAssertions });
          if (!result.proposal) throw new Error("Replacement proposal was not returned");
          completeLifecycleAction(attempt);
          setProposalId(result.proposal.id);
        } catch (error) {
          failLifecycleAction(attempt, error);
          if (isStaleConflict(error)) return setProposalId("");
          throw error;
        }
      }}
      onRetry={() => {
        wikiQuery.refetch();
        twinQuery.refetch();
        twinProfileQuery.refetch();
      }}
    />
  );
}
