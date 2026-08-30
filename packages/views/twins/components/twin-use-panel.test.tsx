// @vitest-environment jsdom

import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import {
  twinExecutionKeys,
  type TwinActivationReadiness,
  type TwinBindingsResponse,
  type TwinExecutionMetrics,
  type TwinKillSwitch,
} from "@multica/core/twins";
import { renderWithI18n } from "../../test/i18n";
import { lifecycleFixture } from "./twin-workspace-view.test-fixture";
import { TwinUsePanel } from "./twin-use-panel";

vi.mock("./twin-entity-selectors", () => ({
  TwinAgentSelector: ({ onChange, ariaLabel }: { onChange: (value: unknown) => void; ariaLabel: string }) => (
    <button type="button" aria-label={ariaLabel} onClick={() => onChange({ id: "agent-1", name: "Ada", archived_at: null })}>Select Ada</button>
  ),
  TwinProjectSelector: ({ onChange, ariaLabel }: { onChange: (value: unknown) => void; ariaLabel: string }) => (
    <button type="button" aria-label={ariaLabel} onClick={() => onChange({ id: "project-1", title: "Apollo", status: "active", icon: null })}>Select Apollo</button>
  ),
  TwinIssueSelector: ({ onChange, ariaLabel }: { onChange: (value: unknown) => void; ariaLabel: string }) => (
    <button type="button" aria-label={ariaLabel} onClick={() => onChange({ id: "issue-1", identifier: "MUL-42", title: "Review auth", project_id: "project-1", status: "in_progress" })}>Select MUL-42</button>
  ),
}));

const fixture = lifecycleFixture();
const wsId = fixture.wsId;
const enabledKillSwitch: TwinKillSwitch = { enabled: true, reason: null };

const activation: TwinActivationReadiness = {
  contractVersion: 1,
  ready: true,
  canManage: true,
  stages: [],
  nextAction: {
    key: "monitor_effectiveness",
    reason: "activation_complete",
    target: "use",
    responsibleRole: "owner_admin",
    canAct: true,
  },
  blockers: [],
  inspectionLinks: [],
  maintenance: [],
};

function metrics(killSwitch: TwinKillSwitch = enabledKillSwitch): TwinExecutionMetrics {
  return {
    attributedRuns: 12,
    feedback: { total: 8, helped: 6, irrelevant: 1, mismatch: 1 },
    depositions: { total: 3, pending: 1, accepted: 2, rejected: 0 },
    bindings: { off: 0, preview: 0, enabled: 0 },
    helpfulnessRate: 0.75,
    killSwitch,
    effectiveness: {
      windowDays: 28,
      minimumSample: 5,
      cohortDefinition: "Policy state at task dispatch in the fixed 28-day window",
      revisionMeasure: "A later attributed run for the same issue",
      costMeasure: "Provider-reported bounded execution cost",
      cohorts: ["off", "preview", "enabled"].map((policyState) => ({
        policyState: policyState as "off" | "preview" | "enabled",
        sampleSize: 5,
        completedRuns: 5,
        attributedRuns: policyState === "off" ? 0 : 5,
        feedbackTotal: 5,
        feedbackCoverage: 1,
        detailSuppressed: false,
        feedbackHelped: 4,
        feedbackIrrelevant: 0,
        feedbackMismatch: 1,
        helpedRate: 0.8,
        revisionCount: 1,
        revisionRate: 0.2,
        averageLatencyMs: 12_000,
        averageBriefingTokens: policyState === "off" ? 0 : 18,
        costUsdTicks: 20_000_000,
        costedRuns: 5,
        uncostedRuns: 0,
        costCoverage: 1,
        depositionTotal: 2,
        depositionAccepted: 1,
        depositionAcceptanceRate: 0.5,
      })),
      comparison: {
        eligible: true,
        enabledState: "enabled",
        controlState: "off",
        reason: "minimum_sample_met",
      },
    },
  };
}

function bindings(
  killSwitch: TwinKillSwitch = enabledKillSwitch,
  canManage = true,
  rows: TwinBindingsResponse["bindings"] = [],
): TwinBindingsResponse {
  return { bindings: rows, canManage, killSwitch };
}

function renderPanel(qc: QueryClient) {
  return renderWithI18n(
    <QueryClientProvider client={qc}>
      <TwinUsePanel
        wsId={wsId}
        versions={fixture.twin.versions}
        currentVersionId={fixture.twin.current_version!.id}
        canManage
        onNavigate={() => undefined}
      />
    </QueryClientProvider>,
  );
}

describe("TwinUsePanel", () => {
  let qc: QueryClient;
  const upsertTwinBinding = vi.fn();
  const deleteTwinBinding = vi.fn();
  const previewTwinBriefing = vi.fn();
  const getTwinBindings = vi.fn();
  const getTwinExecutionMetrics = vi.fn();
  const getTwinActivationReadiness = vi.fn();
  const getIssue = vi.fn();
  const pauseTwinExecution = vi.fn();

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    qc.setQueryData(twinExecutionKeys.bindings(wsId), bindings());
    qc.setQueryData(twinExecutionKeys.metrics(wsId), metrics());
    qc.setQueryData(twinExecutionKeys.activation(wsId), activation);
    upsertTwinBinding.mockReset().mockResolvedValue({});
    deleteTwinBinding.mockReset().mockResolvedValue(undefined);
    previewTwinBriefing.mockReset().mockResolvedValue({
      policy: {
        state: "preview",
        scopeType: "workspace",
        scopeId: wsId,
        bindingId: null,
        explicit: true,
        reason: "workspace_binding",
        exclusions: [],
      },
      twinVersion: { id: "version-1", versionNumber: 1, contentDigest: "sha256:version-1" },
      briefing: "Keep the review decision explicit.",
      briefingDigest: "sha256:briefing",
      assertionIds: ["assertion-1"],
      citationKeys: ["issue:42"],
      compilerVersion: "twin-compiler/2",
      byteCount: 34,
      tokenCount: 8,
      inject: false,
      exclusionReasons: [],
    });
    getTwinBindings.mockReset().mockResolvedValue(bindings());
    getTwinExecutionMetrics.mockReset().mockResolvedValue(metrics());
    getTwinActivationReadiness.mockReset().mockResolvedValue(activation);
    getIssue.mockReset().mockResolvedValue({
      id: "issue-1",
      identifier: "MUL-42",
      title: "Review auth",
      project_id: "project-1",
      status: "in_progress",
    });
    pauseTwinExecution.mockReset().mockResolvedValue({});
    setApiInstance({
      getTwinActivationReadiness,
      getTwinBindings,
      getTwinExecutionMetrics,
      getIssue,
      upsertTwinBinding,
      deleteTwinBinding,
      previewTwinBriefing,
      pauseTwinExecution,
    } as unknown as ApiClient);
  });

  afterEach(() => {
    cleanup();
    qc.clear();
  });

  it("makes the default-off policy explicit and saves the exact workspace binding", async () => {
    renderPanel(qc);

    expect(screen.getByText("Default: off")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Off" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Save binding" }));

    await waitFor(() => expect(upsertTwinBinding).toHaveBeenCalledWith({
      scopeType: "workspace",
      scopeId: wsId,
      state: "off",
      twinVersionId: "version-1",
    }));
  }, 15_000);

  it("compiles and displays the exact briefing with all optional context", async () => {
    renderPanel(qc);
    fireEvent.click(screen.getByRole("button", { name: "Agent" }));
    fireEvent.click(screen.getByRole("button", { name: "Project (optional)" }));
    fireEvent.click(screen.getByRole("button", { name: "Issue (optional)" }));
    fireEvent.change(screen.getByLabelText("Run request"), { target: { value: "Review auth" } });
    fireEvent.click(screen.getByText("Advanced run context"));
    fireEvent.change(screen.getByLabelText("Tags (comma separated)"), { target: { value: "security, auth" } });
    fireEvent.click(screen.getByRole("button", { name: "Compile preview" }));

    await waitFor(() => expect(previewTwinBriefing).toHaveBeenCalledWith({
      agentId: "agent-1",
      projectId: "project-1",
      issueId: "issue-1",
      runId: undefined,
      request: "Review auth",
      tags: ["security", "auth"],
    }));
    expect(await screen.findByText("Keep the review decision explicit.")).toBeInTheDocument();
    expect(screen.getByText("34 bytes · 8 tokens")).toBeInTheDocument();
    expect(screen.queryByText("agent-1")).not.toBeInTheDocument();
  });

  it("marks a compiled preview out of date when the run request changes", async () => {
    renderPanel(qc);
    fireEvent.click(screen.getByRole("button", { name: "Agent" }));
    fireEvent.change(screen.getByLabelText("Run request"), { target: { value: "Review auth" } });
    fireEvent.click(screen.getByRole("button", { name: "Compile preview" }));

    expect(await screen.findByText("Keep the review decision explicit.")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Run request"), { target: { value: "Review auth and document the result" } });

    expect(screen.queryByText("Keep the review decision explicit.")).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This preview is out of date. Compile it again to inspect the current inputs.",
    );
  });

  it("pauses every future scope only after confirmation and keeps history copy visible", async () => {
    const active = [{
      id: "binding-1",
      scopeType: "workspace" as const,
      scopeId: wsId,
      state: "enabled" as const,
      twinVersionId: "version-1",
      createdAt: "2026-08-26T10:00:00Z",
      updatedAt: "2026-08-26T10:00:00Z",
    }];
    qc.setQueryData(twinExecutionKeys.bindings(wsId), bindings(enabledKillSwitch, true, active));
    getTwinBindings.mockResolvedValue(bindings(enabledKillSwitch, true, active));
    renderPanel(qc);

    fireEvent.click(screen.getByRole("button", { name: "Pause future runs" }));
    expect(screen.getByText(/Signed versions, run attribution, feedback, and depositions remain available/)).toBeInTheDocument();
    expect(pauseTwinExecution).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm pause" }));

    await waitFor(() => expect(pauseTwinExecution).toHaveBeenCalledTimes(1));
  });

  it("resolves a visible Issue binding to its human identifier and name", async () => {
    const issueBinding = [{
      id: "binding-issue",
      scopeType: "issue" as const,
      scopeId: "issue-1",
      state: "preview" as const,
      twinVersionId: "version-1",
      createdAt: "2026-08-26T10:00:00Z",
      updatedAt: "2026-08-26T10:00:00Z",
    }];
    qc.setQueryData(twinExecutionKeys.bindings(wsId), bindings(enabledKillSwitch, true, issueBinding));
    getTwinBindings.mockResolvedValue(bindings(enabledKillSwitch, true, issueBinding));
    renderPanel(qc);

    expect(await screen.findByText("MUL-42 Review auth")).toBeInTheDocument();
    expect(screen.queryByText("issue-1")).not.toBeInTheDocument();
  });

  it("disables every policy mutation when the kill switch is off", () => {
    const stopped = { enabled: false, reason: "Paused during incident response" };
    qc.setQueryData(twinExecutionKeys.bindings(wsId), bindings(stopped));
    qc.setQueryData(twinExecutionKeys.metrics(wsId), metrics(stopped));
    getTwinBindings.mockResolvedValue(bindings(stopped));
    getTwinExecutionMetrics.mockResolvedValue(metrics(stopped));
    renderPanel(qc);

    expect(screen.getByText("Paused during incident response")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save binding" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Enabled" })).toBeDisabled();
  });

  it("keeps cached policy readable but disables writes after an offline refresh", async () => {
    getTwinBindings.mockRejectedValue(new Error("offline"));
    getTwinExecutionMetrics.mockRejectedValue(new Error("offline"));
    renderPanel(qc);

    expect(await screen.findByText("Showing the last known policy")).toBeInTheDocument();
    expect(screen.getByTestId("twin-use-panel")).toBeInTheDocument();
    expect(screen.getByText("Default: off")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save binding" })).toBeDisabled();
  });
});
