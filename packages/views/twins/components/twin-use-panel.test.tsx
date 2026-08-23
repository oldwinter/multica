// @vitest-environment jsdom

import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import {
  twinExecutionKeys,
  type TwinBindingsResponse,
  type TwinExecutionMetrics,
  type TwinKillSwitch,
} from "@multica/core/twins";
import { renderWithI18n } from "../../test/i18n";
import { lifecycleFixture } from "./twin-workspace-view.test-fixture";
import { TwinUsePanel } from "./twin-use-panel";

const fixture = lifecycleFixture();
const wsId = fixture.wsId;
const enabledKillSwitch: TwinKillSwitch = { enabled: true, reason: null };

function metrics(killSwitch: TwinKillSwitch = enabledKillSwitch): TwinExecutionMetrics {
  return {
    attributedRuns: 12,
    feedback: { total: 8, helped: 6, irrelevant: 1, mismatch: 1 },
    depositions: { total: 3, pending: 1, accepted: 2, rejected: 0 },
    bindings: { off: 0, preview: 0, enabled: 0 },
    helpfulnessRate: 0.75,
    killSwitch,
  };
}

function bindings(
  killSwitch: TwinKillSwitch = enabledKillSwitch,
  canManage = true,
): TwinBindingsResponse {
  return { bindings: [], canManage, killSwitch };
}

function renderPanel(qc: QueryClient) {
  return renderWithI18n(
    <QueryClientProvider client={qc}>
      <TwinUsePanel
        wsId={wsId}
        versions={fixture.twin.versions}
        currentVersionId={fixture.twin.current_version!.id}
        canManage
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

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    qc.setQueryData(twinExecutionKeys.bindings(wsId), bindings());
    qc.setQueryData(twinExecutionKeys.metrics(wsId), metrics());
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
    setApiInstance({
      getTwinBindings,
      getTwinExecutionMetrics,
      upsertTwinBinding,
      deleteTwinBinding,
      previewTwinBriefing,
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
  });

  it("compiles and displays the exact briefing with all optional context", async () => {
    renderPanel(qc);
    fireEvent.change(screen.getByLabelText("Agent ID"), { target: { value: "agent-1" } });
    fireEvent.change(screen.getByLabelText("Project ID (optional)"), { target: { value: "project-1" } });
    fireEvent.change(screen.getByLabelText("Issue ID (optional)"), { target: { value: "issue-1" } });
    fireEvent.change(screen.getByLabelText("Run request"), { target: { value: "Review auth" } });
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
