import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const wikiKeys = {
  all: (wsId: string) => ["workspaces", wsId, "lm-wiki"] as const,
  overview: (wsId: string) => [...wikiKeys.all(wsId), "overview"] as const,
  revision: (wsId: string, revisionId: string) =>
    [...wikiKeys.all(wsId), "revisions", revisionId] as const,
};

export const twinKeys = {
  all: (wsId: string) => ["workspaces", wsId, "twins"] as const,
  overview: (wsId: string) => [...twinKeys.all(wsId), "overview"] as const,
  proposal: (wsId: string, proposalId: string) =>
    [...twinKeys.all(wsId), "proposals", proposalId] as const,
  version: (wsId: string, versionId: string) =>
    [...twinKeys.all(wsId), "versions", versionId] as const,
};

export const twinProfileKeys = {
  all: (wsId: string) => ["workspaces", wsId, "twin-profile"] as const,
  overview: (wsId: string) => [...twinProfileKeys.all(wsId), "overview"] as const,
};

export function wikiOverviewOptions(wsId: string) {
  return queryOptions({
    queryKey: wikiKeys.overview(wsId),
    queryFn: () => api.getLMWiki(),
    enabled: Boolean(wsId),
  });
}

export function wikiRevisionOptions(wsId: string, revisionId: string) {
  return queryOptions({
    queryKey: wikiKeys.revision(wsId, revisionId),
    queryFn: () => api.getLMWikiRevision(revisionId),
    enabled: Boolean(wsId) && Boolean(revisionId),
  });
}

export function twinOverviewOptions(wsId: string) {
  return queryOptions({
    queryKey: twinKeys.overview(wsId),
    queryFn: () => api.getTwins(),
    enabled: Boolean(wsId),
  });
}

export function twinProposalOptions(wsId: string, proposalId: string) {
  return queryOptions({
    queryKey: twinKeys.proposal(wsId, proposalId),
    queryFn: () => api.getTwinProposal(proposalId),
    enabled: Boolean(wsId) && Boolean(proposalId),
  });
}

export function twinVersionOptions(wsId: string, versionId: string) {
  return queryOptions({
    queryKey: twinKeys.version(wsId, versionId),
    queryFn: () => api.getTwinVersion(versionId),
    enabled: Boolean(wsId) && Boolean(versionId),
  });
}

export function twinProfileOverviewOptions(wsId: string) {
  return queryOptions({
    queryKey: twinProfileKeys.overview(wsId),
    queryFn: () => api.getTwinOverview(),
    enabled: Boolean(wsId),
    select: (response) => response.twin,
  });
}
