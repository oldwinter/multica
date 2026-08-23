import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { personalWikiKeys, wikiKeys } from "./queries";
import type {
  AcceptWikiProposalInput,
  CreateWikiPageInput,
  CreatePersonalWikiPageInput,
  CreateWikiProposalInput,
  RejectWikiProposalInput,
  RestoreWikiRevisionInput,
  UpdateWikiPageInput,
  UpdateLMWikiSourcePolicyInput,
} from "./types";

export function useCreateWikiPage(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateWikiPageInput) => api.createWikiPage(input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useUpdateWikiPage(wsId: string, pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateWikiPageInput) => api.updateWikiPage(pageId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useDeleteWikiPage(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (pageId: string) => api.deleteWikiPage(pageId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useRestoreWikiRevision(wsId: string, pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RestoreWikiRevisionInput) => (
      api.restoreWikiRevision(pageId, input.revisionId, input.expectedRevisionNumber)
    ),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useCreateWikiProposal(wsId: string, pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateWikiProposalInput) => api.createWikiProposal(pageId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useAcceptWikiProposal(wsId: string, pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AcceptWikiProposalInput) => api.acceptWikiProposal(pageId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useRejectWikiProposal(wsId: string, pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RejectWikiProposalInput) => api.rejectWikiProposal(pageId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
  });
}

export function useUpdateLMWikiSourcePolicy(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateLMWikiSourcePolicyInput) => api.updateLMWikiSourcePolicy(input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: wikiKeys.sourcePolicy(wsId) }),
  });
}

export function useCreatePersonalWikiPage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreatePersonalWikiPageInput) => api.createPersonalWikiPage(input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: personalWikiKeys.all }),
  });
}

export function useUpdatePersonalWikiPage(pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateWikiPageInput) => api.updatePersonalWikiPage(pageId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: personalWikiKeys.all }),
  });
}

export function useDeletePersonalWikiPage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (pageId: string) => api.deletePersonalWikiPage(pageId),
    onSettled: () => queryClient.invalidateQueries({ queryKey: personalWikiKeys.all }),
  });
}

export function useRestorePersonalWikiRevision(pageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RestoreWikiRevisionInput) => (
      api.restorePersonalWikiRevision(pageId, input.revisionId, input.expectedRevisionNumber)
    ),
    onSettled: () => queryClient.invalidateQueries({ queryKey: personalWikiKeys.all }),
  });
}
