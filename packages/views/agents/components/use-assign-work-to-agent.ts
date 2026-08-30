"use client";

import { useCallback } from "react";
import { toast } from "sonner";
import { isAgentRuntimeBound } from "@multica/core/agents";
import { openQuickCreateForAgent } from "@multica/core/modals";
import type { Agent } from "@multica/core/types";
import { useT } from "../../i18n";

export function useAssignWorkToAgent() {
  const { t } = useT("agents");

  return useCallback(
    (agent: Agent) => {
      if (!isAgentRuntimeBound(agent)) {
        toast.error(t(($) => $.detail.runtime_required_toast));
        return;
      }

      openQuickCreateForAgent({ agentId: agent.id });
    },
    [t],
  );
}
