"use client";

import { use } from "react";
import { PersonalWikiPageView } from "@multica/views/wiki";

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <PersonalWikiPageView pageId={id} />;
}
