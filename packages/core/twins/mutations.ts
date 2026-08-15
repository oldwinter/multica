import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { twinKeys, twinProfileKeys, wikiKeys } from "./queries";

const LIFECYCLE_WRITE_TIMEOUT_MS = 30_000;
const CACHE_SETTLEMENT_TIMEOUT_MS = 5_000;

async function withLifecycleWriteTimeout<T>(request: (signal: AbortSignal) => Promise<T>): Promise<T> {
  const controller = new AbortController();
  const timeout = setTimeout(() => {
    const error = new Error("Review request timed out. Try again.");
    error.name = "TimeoutError";
    controller.abort(error);
  }, LIFECYCLE_WRITE_TIMEOUT_MS);
  try {
    return await request(controller.signal);
  } finally {
    clearTimeout(timeout);
  }
}

async function settleLifecycleQueries(invalidations: readonly Promise<unknown>[]): Promise<void> {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<void>((resolve) => {
    timeout = setTimeout(resolve, CACHE_SETTLEMENT_TIMEOUT_MS);
  });
  try {
    await Promise.race([
      Promise.allSettled(invalidations).then(() => undefined),
      deadline,
    ]);
  } finally {
    clearTimeout(timeout);
  }
}

export function useRefreshLMWiki(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => withLifecycleWriteTimeout((signal) => api.refreshLMWiki(signal)),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
    ]),
  });
}

export function useAcceptLMWikiRevision(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (revisionId: string) => withLifecycleWriteTimeout(
      (signal) => api.acceptLMWikiRevision(revisionId, signal),
    ),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
    ]),
  });
}

export function useRejectLMWikiRevision(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ revisionId, reason }: { revisionId: string; reason?: string }) => (
      withLifecycleWriteTimeout((signal) => api.rejectLMWikiRevision(revisionId, reason, signal))
    ),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) }),
    ]),
  });
}

export function useEnsureTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (wikiRevisionId: string) => withLifecycleWriteTimeout(
      (signal) => api.ensureTwinProposal(wikiRevisionId, signal),
    ),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
    ]),
  });
}

export function useAcceptTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (proposalId: string) => withLifecycleWriteTimeout(
      (signal) => api.acceptTwinProposal(proposalId, signal),
    ),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
      queryClient.invalidateQueries({ queryKey: twinProfileKeys.all(wsId) }),
    ]),
  });
}

export function useRejectTwinProposal(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ proposalId, reason }: { proposalId: string; reason?: string }) => (
      withLifecycleWriteTimeout((signal) => api.rejectTwinProposal(proposalId, reason, signal))
    ),
    onSettled: () => settleLifecycleQueries([
      queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
    ]),
  });
}
