"use client";

import { use } from "react";
import { WikiPageView } from "@multica/views/wiki";

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <WikiPageView pageId={id} rootElement="div" />;
}
