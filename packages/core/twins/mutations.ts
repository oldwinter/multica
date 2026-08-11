import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { twinKeys, twinProfileKeys, wikiKeys } from "./queries";

export function useRefreshLMWiki(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.refreshLMWiki(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useAcceptLMWikiRevision(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (revisionId: string) => api.acceptLMWikiRevision(revisionId),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) });
    },
  });
}

export function useRejectLMWikiRevision(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ revisionId, reason }: { revisionId: string; reason?: string }) =>
      api.rejectLMWikiRevision(revisionId, reason),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useEnsureTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (wikiRevisionId: string) => api.ensureTwinProposal(wikiRevisionId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
  });
}

export function useAcceptTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (proposalId: string) => api.acceptTwinProposal(proposalId),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) });
      queryClient.invalidateQueries({ queryKey: twinProfileKeys.all(wsId) });
    },
  });
}

export function useRejectTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ proposalId, reason }: { proposalId: string; reason?: string }) =>
      api.rejectTwinProposal(proposalId, reason),
    onSettled: () => queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
  });
}
