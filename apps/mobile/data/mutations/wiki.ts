import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/data/api";
import { wikiKeys } from "@/data/queries/wiki";
import { useWorkspaceStore } from "@/data/workspace-store";
import {
  getWikiRevisionConflict,
  type PinWikiRevisionAsLMWikiEvidenceInput,
  type AcceptWikiProposalInput,
  type CreateWikiPageInput,
  type RejectWikiProposalInput,
  type UpdateWikiPageInput,
  type WikiPage,
} from "@/data/wiki-schema";

export function wikiConflictFromError(
  error: unknown,
): { currentRevisionNumber: number } | null {
  return error instanceof ApiError
    ? getWikiRevisionConflict(error.body)
    : null;
}

export function useCreateWikiPage() {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationFn: (body: CreateWikiPageInput) => api.createWikiPage(body),
    onSuccess: (page) => {
      qc.setQueryData<WikiPage>(wikiKeys.detail(wsId, page.id), page);
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function useUpdateWikiPage(pageId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["updateWikiPage", pageId] as const,
    mutationFn: (body: UpdateWikiPageInput) =>
      api.updateWikiPage(pageId, body),
    onSuccess: (page) => {
      qc.setQueryData<WikiPage>(wikiKeys.detail(wsId, pageId), page);
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function useDeleteWikiPage(pageId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["deleteWikiPage", pageId] as const,
    mutationFn: () => api.deleteWikiPage(pageId),
    onSuccess: () => {
      qc.removeQueries({ queryKey: wikiKeys.detail(wsId, pageId) });
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function usePinWikiRevisionAsLMWikiEvidence() {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["pinWikiRevisionAsLMWikiEvidence", wsId] as const,
    mutationFn: (input: PinWikiRevisionAsLMWikiEvidenceInput) =>
      api.pinWikiRevisionAsLMWikiEvidence(input),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function useRestoreWikiPageRevision(pageId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["restoreWikiPageRevision", pageId] as const,
    mutationFn: ({
      revisionId,
      expectedRevisionNumber,
    }: {
      revisionId: string;
      expectedRevisionNumber: number;
    }) =>
      api.restoreWikiPageRevision(
        pageId,
        revisionId,
        expectedRevisionNumber,
      ),
    onSuccess: (page) => {
      qc.setQueryData<WikiPage>(wikiKeys.detail(wsId, pageId), page);
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function useAcceptWikiPageProposal(pageId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["acceptWikiPageProposal", pageId] as const,
    mutationFn: ({
      proposalId,
      ...body
    }: AcceptWikiProposalInput & { proposalId: string }) =>
      api.acceptWikiPageProposal(pageId, proposalId, body),
    onSuccess: (page) => {
      qc.setQueryData<WikiPage>(wikiKeys.detail(wsId, pageId), page);
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}

export function useRejectWikiPageProposal(pageId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["rejectWikiPageProposal", pageId] as const,
    mutationFn: ({
      proposalId,
      ...body
    }: RejectWikiProposalInput & { proposalId: string }) =>
      api.rejectWikiPageProposal(pageId, proposalId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
    },
  });
}
