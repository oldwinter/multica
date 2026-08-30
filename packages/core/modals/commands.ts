"use client";

import { useModalStore } from "./store";

interface OpenQuickCreateForAgentInput {
  agentId: string;
}

export function openQuickCreateForAgent({
  agentId,
}: OpenQuickCreateForAgentInput): void {
  useModalStore.getState().open("quick-create-issue", { agent_id: agentId });
}
