import type { WikiProposalStatus } from "@/data/wiki-schema";

export function canReviewWikiProposal(status: WikiProposalStatus): boolean {
  return status === "pending";
}

export function wikiProjectPickerScreenOptions<T extends object>(
  sharedSheetOptions: T,
) {
  return {
    ...sharedSheetOptions,
    headerShown: true as const,
    title: "Project" as const,
  };
}

export function wikiProposalReviewRoute(
  workspace: string,
  pageId: string,
  proposalId: string,
) {
  return {
    pathname: "/[workspace]/wiki/[id]/proposal/[proposalId]" as const,
    params: { workspace, id: pageId, proposalId },
  };
}
