// @vitest-environment node

import { beforeEach, describe, expect, it } from "vitest";
import { openQuickCreateForAgent } from "./commands";
import { useModalStore } from "./store";

describe("openQuickCreateForAgent", () => {
  beforeEach(() => {
    useModalStore.getState().close();
  });

  it("opens quick create with the selected Agent seed", () => {
    openQuickCreateForAgent({ agentId: "agent-1" });

    expect(useModalStore.getState()).toMatchObject({
      modal: "quick-create-issue",
      data: { agent_id: "agent-1" },
    });
  });
});
