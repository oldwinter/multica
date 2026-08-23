"use client";

import { use } from "react";
import { WorkspaceWikiRevisionView } from "@multica/views/wiki";

export default function Page({ params }: { params: Promise<{ revisionId: string }> }) {
  const { revisionId } = use(params);
  return <WorkspaceWikiRevisionView revisionId={revisionId} />;
}
