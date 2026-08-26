// @vitest-environment jsdom

import { cleanup, fireEvent, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { twinExecutionKeys, type TwinActivationReadiness as Readiness } from "@multica/core/twins";
import { renderWithI18n } from "../../test/i18n";
import { TwinActivationReadiness } from "./twin-activation-readiness";

const wsId = "00000000-0000-4000-8000-000000000001";

function renderReadiness(readiness: Readiness, onNavigate = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  queryClient.setQueryData(twinExecutionKeys.activation(wsId), readiness);
  const rendered = renderWithI18n(
    <QueryClientProvider client={queryClient}>
      <TwinActivationReadiness wsId={wsId} onNavigate={onNavigate} />
    </QueryClientProvider>,
  );
  return { ...rendered, queryClient, onNavigate };
}

afterEach(cleanup);

describe("TwinActivationReadiness", () => {
  it("renders one deterministic primary action and navigates to its owned surface", () => {
    const { onNavigate, queryClient } = renderReadiness({
      contractVersion: 1,
      ready: false,
      canManage: true,
      stages: [
        { key: "source_policy", complete: true, count: 0 },
        { key: "evidence", complete: true, count: 0 },
        { key: "signed_twin", complete: true, count: 0 },
        { key: "preview", complete: false, count: 0 },
      ],
      nextAction: {
        key: "compile_preview",
        reason: "current_twin_not_previewed",
        target: "use",
        responsibleRole: "member",
        canAct: true,
      },
      blockers: [{ kind: "missing_state", reason: "current_twin_not_previewed", responsibleRole: "member" }],
      inspectionLinks: [
        { key: "evidence_history", target: "wiki" },
        { key: "twin_history", target: "twin" },
        { key: "execution_evidence", target: "use" },
      ],
      maintenance: [],
    });

    const action = screen.getByRole("button", { name: "Compile preview" });
    expect(screen.getByTestId("twin-primary-action")).toBe(action);
    fireEvent.click(action);
    expect(onNavigate).toHaveBeenCalledWith("use");
    queryClient.clear();
  });

  it("keeps an owner-only action visible but unavailable to a member", () => {
    const { queryClient } = renderReadiness({
      contractVersion: 1,
      ready: false,
      canManage: false,
      stages: [{ key: "source_policy", complete: false, count: 0 }],
      nextAction: {
        key: "configure_source",
        reason: "source_policy_missing",
        target: "wiki",
        responsibleRole: "owner_admin",
        canAct: false,
      },
      blockers: [
        { kind: "missing_state", reason: "source_policy_missing", responsibleRole: "owner_admin" },
        { kind: "missing_capability", reason: "owner_or_admin_required", responsibleRole: "owner_admin" },
      ],
      inspectionLinks: [],
      maintenance: [],
    });

    expect(screen.getByRole("button", { name: "Configure sources" })).toBeDisabled();
    expect(screen.getByText("Responsible: owner or admin")).toBeInTheDocument();
    expect(screen.getByText("A required lifecycle state is incomplete.")).toBeInTheDocument();
    expect(screen.getByText("Owner or admin action is required.")).toBeInTheDocument();
    queryClient.clear();
  });
});
