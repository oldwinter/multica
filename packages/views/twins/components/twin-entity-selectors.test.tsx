// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import enIssues from "../../locales/en/issues.json";
import enTwins from "../../locales/en/twins.json";
import {
  TwinAgentSelector,
  TwinIssueSelector,
  TwinProjectSelector,
} from "./twin-entity-selectors";

const AGENTS = [
  { id: "agent-ada-id", name: "Ada Lovelace", archived_at: null },
  { id: "agent-archived-id", name: "Archived Agent", archived_at: "2026-08-01T00:00:00Z" },
];
const PROJECTS = [
  { id: "project-apollo-id", title: "Apollo", status: "active", icon: null },
];
const ISSUES = [
  { id: "issue-auth-id", identifier: "MUL-42", title: "Review auth", project_id: "project-apollo-id", status: "in_progress" },
];

const selectorErrors = vi.hoisted(() => ({ agents: false, projects: false, issues: false }));
const selectorPending = vi.hoisted(() => ({ agents: false, projects: false, issues: false }));
const selectorRefetch = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", () => ({
  useQuery: ({ queryKey }: { queryKey: readonly string[] }) => {
    if (queryKey[0] === "twin-agents") return { data: selectorErrors.agents || selectorPending.agents ? undefined : AGENTS, isError: selectorErrors.agents, isPending: selectorPending.agents, refetch: selectorRefetch };
    if (queryKey[0] === "twin-projects") return { data: selectorErrors.projects || selectorPending.projects ? undefined : PROJECTS, isError: selectorErrors.projects, isPending: selectorPending.projects, refetch: selectorRefetch };
    if (queryKey[0] === "twin-issues") return { data: selectorErrors.issues ? undefined : (queryKey[1] ? ISSUES : []), isError: selectorErrors.issues, isFetching: selectorPending.issues, isPending: selectorPending.issues, refetch: selectorRefetch };
    return { data: [], isFetching: false };
  },
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ queryKey: ["twin-agents"] }),
}));
vi.mock("@multica/core/projects/queries", () => ({
  projectListOptions: () => ({ queryKey: ["twin-projects"] }),
}));
vi.mock("@multica/core/twins", () => ({
  twinIssueSelectorOptions: (_wsId: string, search: string) => ({ queryKey: ["twin-issues", search] }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span aria-hidden="true" /> }));
vi.mock("../../projects/components/project-icon", () => ({ ProjectIcon: () => <span aria-hidden="true" /> }));

function withI18n(children: React.ReactNode) {
  return (
    <I18nProvider locale="en" resources={{ en: { twins: enTwins, issues: enIssues } }}>
      {children}
    </I18nProvider>
  );
}

afterEach(() => {
  cleanup();
  selectorErrors.agents = false;
  selectorErrors.projects = false;
  selectorErrors.issues = false;
  selectorPending.agents = false;
  selectorPending.projects = false;
  selectorPending.issues = false;
  selectorRefetch.mockReset();
});

describe("Twin entity selectors", () => {
  it("selects an eligible Agent by keyboard and disables archived Agents", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(withI18n(
      <TwinAgentSelector wsId="workspace-id" value={null} onChange={onChange} ariaLabel="Agent" />,
    ));

    await user.click(screen.getByRole("button", { name: "Agent" }));
    expect(screen.getByRole("button", { name: /Archived Agent/ })).toBeDisabled();
    await user.type(screen.getByPlaceholderText("Search agents"), "ada");
    await user.keyboard("{Enter}");

    expect(onChange).toHaveBeenCalledWith({ id: "agent-ada-id", name: "Ada Lovelace", archived_at: null });
    expect(screen.queryByText("agent-ada-id")).not.toBeInTheDocument();
  });

  it("submits canonical Project and Issue IDs while showing human labels", async () => {
    const user = userEvent.setup();
    const onProject = vi.fn();
    const onIssue = vi.fn();
    render(withI18n(
      <div>
        <TwinProjectSelector wsId="workspace-id" value={null} onChange={onProject} ariaLabel="Project" />
        <TwinIssueSelector wsId="workspace-id" value={null} onChange={onIssue} ariaLabel="Issue" />
      </div>,
    ));

    await user.click(screen.getByRole("button", { name: "Project" }));
    await user.click(screen.getByRole("button", { name: /Apollo/ }));
    expect(onProject).toHaveBeenCalledWith({ id: "project-apollo-id", title: "Apollo", status: "active", icon: null });

    await user.click(screen.getByRole("button", { name: "Issue" }));
    await user.type(screen.getByPlaceholderText("Search by issue identifier or title"), "MUL-42");
    await user.keyboard("{Enter}");
    expect(onIssue).toHaveBeenCalledWith({
      id: "issue-auth-id",
      identifier: "MUL-42",
      title: "Review auth",
      project_id: "project-apollo-id",
      status: "in_progress",
    });
    expect(screen.queryByText("issue-auth-id")).not.toBeInTheDocument();
  });

  it("distinguishes a failed Issue search from an empty result and offers retry", async () => {
    selectorErrors.issues = true;
    const user = userEvent.setup();
    render(withI18n(
      <TwinIssueSelector wsId="workspace-id" value={null} onChange={vi.fn()} ariaLabel="Issue" />,
    ));

    await user.click(screen.getByRole("button", { name: "Issue" }));
    await user.type(screen.getByPlaceholderText("Search by issue identifier or title"), "MUL-42");
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load options.");
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(selectorRefetch).toHaveBeenCalled();
  });

  it.each([
    ["agents", "Agent"],
    ["projects", "Project"],
  ] as const)("shows loading feedback while %s are pending", async (kind, label) => {
    selectorPending[kind] = true;
    const user = userEvent.setup();
    render(withI18n(
      kind === "agents"
        ? <TwinAgentSelector wsId="workspace-id" value={null} onChange={vi.fn()} ariaLabel={label} />
        : <TwinProjectSelector wsId="workspace-id" value={null} onChange={vi.fn()} ariaLabel={label} />,
    ));

    await user.click(screen.getByRole("button", { name: label }));
    expect(screen.getByRole("status")).toHaveTextContent("Searching");
    expect(screen.queryByText("No options")).not.toBeInTheDocument();
  });
});
