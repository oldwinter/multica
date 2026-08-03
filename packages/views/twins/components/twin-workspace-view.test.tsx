// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { previewTwinOverview } from "@multica/core/twins";
import { TwinWorkspaceView, type TwinCopy } from "./twin-workspace-view";

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const copy: TwinCopy = {
  eyebrow: "Reviewable profile",
  title: "Twin workspace",
  description: "Review evidence before it informs work.",
  previewBadge: "Preview data",
  actions: {
    openIssues: "Open issues",
    openAgents: "Open agents",
    reviewProfile: "Review profile",
    tryAgain: "Try again",
    connectEvidence: "Connect evidence",
  },
  status: {
    pending: { label: "Pending sign-off", title: "Review before use", description: "This profile cannot guide execution until a person signs off." },
    signedOff: { label: "Signed off", title: "Ready to inform work", description: "The profile matches the reviewed evidence snapshot." },
    invalid: { label: "Invalid package", title: "Fix the package first", description: "The profile is unavailable until its source files are repaired." },
  },
  summary: { sources: "Sources", assertions: "Assertions", skills: "Skills", rules: "Rules", lastReviewed: "Last reviewed" },
  review: {
    title: "Review path",
    description: "The Twin loop keeps acceptance separate from runtime completion.",
    steps: {
      import: { label: "Import", description: "Select evidence" },
      generate: { label: "Generate Twin", description: "Draft assertions" },
      topic: { label: "Open topic and dispatch", description: "Bound the objective" },
      coordinate: { label: "Coordinate execution", description: "Observe progress" },
      accept: { label: "Report and accept", description: "Human decision" },
      deposition: { label: "Deposition", description: "Archive a delta" },
    },
  },
  tabs: { overview: "Overview", evidence: "Evidence snapshot", topics: "Issue-backed topics" },
  evidence: { title: "Evidence snapshot", description: "Assertions stay linked to their source material.", sourceCount: "sources", viewDetail: "View evidence" },
  topics: { title: "Issue-backed topics", description: "Dispatch bounded work through the workspace issue model.", openIssue: "Open issue", empty: "No topics yet." },
  stateLabels: { complete: "Complete", current: "In review", upcoming: "Upcoming" },
  topicStates: { active: "Active", waiting: "Waiting", accepted: "Accepted" },
  states: {
    loading: "Loading Twin profile",
    emptyTitle: "No Twin profile yet",
    emptyDescription: "Connect evidence to create a reviewable profile.",
    errorTitle: "Twin profile unavailable",
    errorDescription: "We could not load the profile.",
  },
};

const links = { issues: "/acme/issues", agents: "/acme/agents" };

describe("TwinWorkspaceView", () => {
  it("renders the reviewable overview and links execution back to issues", () => {
    render(<TwinWorkspaceView data={previewTwinOverview} copy={copy} links={links} onRetry={vi.fn()} />);

    expect(screen.getByRole("heading", { name: "Twin workspace" })).toBeInTheDocument();
    expect(screen.getByText("Pending sign-off")).toBeInTheDocument();
    expect(screen.getByText("Review path")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open issues" })).toHaveAttribute("href", "/acme/issues");
    const topicLink = screen
      .getAllByRole("link", { name: "Open issue" })
      .find((link) => link.getAttribute("href") === "/acme/issues/MUL-42");
    expect(topicLink).toBeDefined();
  });

  it.each([
    ["loading", "Loading Twin profile"],
    ["empty", "No Twin profile yet"],
    ["error", "Twin profile unavailable"],
  ] as const)("renders the %s state with an actionable message", (state, label) => {
    const onRetry = vi.fn();
    render(<TwinWorkspaceView state={state} copy={copy} links={links} onRetry={onRetry} />);

    expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();

    if (state === "error") {
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
      expect(onRetry).toHaveBeenCalledOnce();
    }
    if (state === "empty") {
      expect(screen.getByRole("link", { name: "Connect evidence" })).toHaveAttribute("href", "/acme/issues");
    }
  });

  it("allows the evidence and topic views to be selected", () => {
    render(<TwinWorkspaceView data={previewTwinOverview} copy={copy} links={links} onRetry={vi.fn()} />);

    fireEvent.click(screen.getByRole("tab", { name: "Evidence snapshot" }));
    expect(screen.getByText("Assertions stay linked to their source material.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Issue-backed topics" }));
    expect(screen.getByText("Dispatch bounded work through the workspace issue model.")).toBeInTheDocument();
  });
});
