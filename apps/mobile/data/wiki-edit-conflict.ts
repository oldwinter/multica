import type { UpdateWikiPageInput } from "@/data/wiki-schema";

export interface WikiEditDraft {
  path: string;
  title: string;
  content: string;
  baseRevision: number;
}

export function rebaseWikiDraftAfterConflict(
  draft: WikiEditDraft,
  currentRevisionNumber: number,
): WikiEditDraft {
  return { ...draft, baseRevision: currentRevisionNumber };
}

export function buildWikiUpdateBody(
  draft: WikiEditDraft,
): UpdateWikiPageInput {
  return {
    path: draft.path.trim(),
    title: draft.title.trim(),
    content: draft.content,
    expectedRevisionNumber: draft.baseRevision,
  };
}
