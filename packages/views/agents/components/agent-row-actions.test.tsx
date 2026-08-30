// @vitest-environment jsdom

import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";

const mocks = vi.hoisted(() => ({
  invalidateQueries: vi.fn(),
  openQuickCreateForAgent: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({ invalidateQueries: mocks.invalidateQueries }),
}));

vi.mock("@multica/core/agents", () => ({
  isAgentRuntimeBound: (agent: { runtime_id: string; runtime_bound?: boolean }) =>
    agent.runtime_bound !== false && agent.runtime_id.length > 0,
}));

vi.mock("@multica/core/api", () => ({
  api: {
    archiveAgent: vi.fn(),
    restoreAgent: vi.fn(),
    cancelAgentTasks: vi.fn(),
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/modals", () => ({
  openQuickCreateForAgent: mocks.openQuickCreateForAgent,
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    agentDetail: (id: string) => `/acme/agents/${id}`,
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  workspaceKeys: { agents: (wsId: string) => ["agents", wsId] },
}));

vi.mock("sonner", () => ({
  toast: {
    error: mocks.toastError,
    success: vi.fn(),
  },
}));

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: React.ComponentProps<"a">) => (
    <a href={href} {...props}>{children}</a>
  ),
  useIntentNavigate: () => vi.fn(),
}));

vi.mock("./agent-mention-menu-item", () => ({
  AgentMentionMenuItem: () => null,
}));

import { AgentRowActions } from "./agent-row-actions";

const AGENT = {
  id: "agent-1",
  workspace_id: "workspace-1",
  runtime_id: "runtime-1",
  name: "Patch Pilot",
  description: "Keeps small fixes moving",
  instructions: "Ship focused patches.",
  avatar_url: null,
  runtime_mode: "cloud",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "private",
  invocation_targets: [],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "codex",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-08-01T00:00:00Z",
  updated_at: "2026-08-29T00:00:00Z",
  archived_at: null,
  archived_by: null,
} satisfies Agent;

function renderActions(
  agent: Agent = AGENT,
  canAssign = true,
) {
  return render(
    <I18nProvider
      locale="en"
      resources={{ en: { agents: enAgents, common: enCommon } }}
    >
      <AgentRowActions
        agent={agent}
        presence={null}
        canManage={false}
        canAssign={canAssign}
        duplicateHref="/acme/agents/new/manual?template=agent-1"
      />
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("AgentRowActions assign work", () => {
  it("opens quick create with the selected agent from the keyboard-accessible menu", async () => {
    const user = userEvent.setup();
    renderActions();

    const trigger = screen.getByRole("button", { name: "Row actions" });
    trigger.focus();
    await user.keyboard("{Enter}");

    const assignItem = await screen.findByRole("menuitem", { name: "Assign work" });
    expect(assignItem).toHaveFocus();
    await user.keyboard("{Enter}");

    expect(mocks.openQuickCreateForAgent).toHaveBeenCalledWith({
      agentId: "agent-1",
    });
    expect(mocks.toastError).not.toHaveBeenCalled();
    expect(trigger).toHaveFocus();
  });

  it("explains when the agent needs a runtime", async () => {
    const user = userEvent.setup();
    renderActions({ ...AGENT, runtime_id: "" });

    await user.click(screen.getByRole("button", { name: "Row actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Assign work" }));

    expect(mocks.openQuickCreateForAgent).not.toHaveBeenCalled();
    expect(mocks.toastError).toHaveBeenCalledWith(
      "Bind a runtime before running this agent.",
    );
  });

  it("hides the action without invocation permission", async () => {
    const user = userEvent.setup();
    renderActions(AGENT, false);

    await user.click(screen.getByRole("button", { name: "Row actions" }));
    expect(
      screen.queryByRole("menuitem", { name: "Assign work" }),
    ).not.toBeInTheDocument();
  });

  it("hides the action for archived agents", async () => {
    const user = userEvent.setup();
    renderActions({ ...AGENT, archived_at: "2026-08-29T00:00:00Z" });

    await user.click(screen.getByRole("button", { name: "Row actions" }));
    expect(
      screen.queryByRole("menuitem", { name: "Assign work" }),
    ).not.toBeInTheDocument();
  });
});
