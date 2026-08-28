// @vitest-environment jsdom

import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { Agent } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";

const mocks = vi.hoisted(() => ({
  invalidateQueries: vi.fn(),
  modalOpen: vi.fn(),
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
  useModalStore: {
    getState: () => ({ open: mocks.modalOpen }),
  },
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
  AppLink: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
  useIntentNavigate: () => vi.fn(),
}));

vi.mock("./agent-mention-menu-item", () => ({
  AgentMentionMenuItem: () => <button type="button">Copy mention</button>,
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DropdownMenuItem: ({
    children,
    onClick,
    render,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    render?: React.ReactNode;
  }) => render ?? <button type="button" onClick={onClick}>{children}</button>,
  DropdownMenuSeparator: () => <hr />,
}));

vi.mock("@multica/ui/components/ui/alert-dialog", () => ({
  AlertDialog: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  AlertDialogAction: ({ children }: { children: React.ReactNode }) => <button type="button">{children}</button>,
  AlertDialogCancel: ({ children }: { children: React.ReactNode }) => <button type="button">{children}</button>,
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
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
  it("opens quick create with the selected agent", () => {
    renderActions();

    fireEvent.click(screen.getByRole("button", { name: "Assign work" }));

    expect(mocks.modalOpen).toHaveBeenCalledWith("quick-create-issue", {
      agent_id: "agent-1",
    });
    expect(mocks.toastError).not.toHaveBeenCalled();
  });

  it("explains when the agent needs a runtime", () => {
    renderActions({ ...AGENT, runtime_id: "" });

    fireEvent.click(screen.getByRole("button", { name: "Assign work" }));

    expect(mocks.modalOpen).not.toHaveBeenCalled();
    expect(mocks.toastError).toHaveBeenCalledWith(
      "Bind a runtime before running this agent.",
    );
  });

  it("hides the action without invocation permission", () => {
    renderActions(AGENT, false);

    expect(
      screen.queryByRole("button", { name: "Assign work" }),
    ).not.toBeInTheDocument();
  });

  it("hides the action for archived agents", () => {
    renderActions({ ...AGENT, archived_at: "2026-08-29T00:00:00Z" });

    expect(
      screen.queryByRole("button", { name: "Assign work" }),
    ).not.toBeInTheDocument();
  });
});
