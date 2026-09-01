// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import type { WikiKnowledgeReadiness } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";

const harness = vi.hoisted(() => ({
  query: { isPending: false, isError: false, refetch: vi.fn(), data: undefined as WikiKnowledgeReadiness | undefined },
  pin: { mutate: vi.fn(), isPending: false, isError: false },
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return { ...actual, useQuery: () => harness.query };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/wiki", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/wiki")>();
  return {
    ...actual,
    wikiKnowledgeReadinessOptions: () => ({ queryKey: ["wiki", "workspace-1", "knowledge-readiness"] }),
    usePinWikiRevisionAsLMWikiEvidence: () => harness.pin,
  };
});

import {
  PersonalWikiKnowledgeActivation,
  WorkspaceWikiKnowledgeActivation,
  type WikiKnowledgeActivationTarget,
} from "./wiki-knowledge-activation";

const target: WikiKnowledgeActivationTarget = {
  pageId: "page-1",
  revisionId: "revision-4",
  revisionNumber: 4,
  title: "Release policy",
  path: "operations/release.md",
  contentDigest: "sha256:revision-4",
  sourceKind: "human",
  actorType: "member",
};

const readiness: WikiKnowledgeReadiness = {
  schemaVersion: 1,
  policy: {
    sourceClasses: ["issue"],
    wikiPages: [],
    remoteGenerationEnabled: false,
    policyVersion: 7,
    policyDigest: "sha256:policy-7",
    exclusions: [
      { sourceClass: "personal_wiki", state: "always_excluded", reason: "personal_scope_never_eligible" },
      { sourceClass: "local_only", state: "always_excluded", reason: "local_only_never_leaves_owner_daemon" },
    ],
  },
  sources: [{
    pageId: "page-1",
    scope: "workspace",
    state: "eligible_unpinned",
    reasonCode: "wiki_source_eligible_unpinned",
    responsibleRole: "owner_admin",
    currentRevisionId: "revision-4",
    currentRevisionNumber: 4,
    policyVersion: 7,
    nextAction: { kind: "pin_revision", pageId: "page-1", revisionId: "revision-4", revisionNumber: 4 },
  }],
  maintenanceItems: [],
  truncated: false,
  canManage: true,
};

function renderWithI18n(node: React.ReactNode) {
  return render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      {node}
    </I18nProvider>,
  );
}

describe("WikiKnowledgeActivation", () => {
  it("confirms the exact revision and current policy identity without refreshing LM Wiki", () => {
    harness.query.data = readiness;
    harness.pin.mutate.mockReset();
    harness.pin.isError = false;
    renderWithI18n(<WorkspaceWikiKnowledgeActivation target={target} />);

    expect(screen.getByText("Eligible, not pinned")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Use as LM Wiki evidence" }));
    expect(screen.getByRole("dialog")).toHaveAttribute("data-wiki-interaction-region");
    expect(screen.getByRole("dialog")).toHaveClass(
      "max-lg:[&_button]:min-h-11",
      "max-lg:[&_[data-slot=dialog-header]]:pr-12",
    );
    expect(screen.getByRole("dialog")).toHaveTextContent("Release policy");
    expect(screen.getByRole("dialog")).toHaveTextContent("operations/release.md");
    expect(screen.getByRole("dialog")).toHaveTextContent("sha256:revision-4");
    expect(screen.getByRole("dialog")).toHaveTextContent("Version 7 · sha256:policy-7");
    expect(screen.getByRole("dialog")).toHaveTextContent("Remote generation is disabled");
    expect(screen.getByRole("dialog")).toHaveTextContent("Personal Wiki pages");
    expect(screen.getByRole("dialog")).toHaveTextContent("Local-only sources");

    fireEvent.click(screen.getByRole("button", { name: "Pin exact revision" }));
    expect(harness.pin.mutate).toHaveBeenCalledWith({
      pageId: "page-1",
      revisionId: "revision-4",
      expectedPolicyVersion: 7,
      expectedPolicyDigest: "sha256:policy-7",
    }, expect.any(Object));
  });

  it("keeps the reviewed revision visible after a structured stale-policy conflict", () => {
    harness.query.data = readiness;
    harness.pin.isError = true;
    harness.pin.mutate.mockImplementation((_input, options) => options.onError(new ApiError(
      "stale",
      409,
      "Conflict",
      {
        code: "wiki_source_policy_stale",
        current_policy: {
          source_classes: ["issue"],
          wiki_pages: [],
          remote_generation_enabled: false,
          policy_version: 8,
          policy_digest: "sha256:policy-8",
          exclusions: [],
        },
      },
    )));
    renderWithI18n(<WorkspaceWikiKnowledgeActivation target={target} />);
    fireEvent.click(screen.getByRole("button", { name: "Use as LM Wiki evidence" }));
    fireEvent.click(screen.getByRole("button", { name: "Pin exact revision" }));

    expect(screen.getByRole("alert")).toHaveTextContent("policy version 8");
    expect(screen.getByRole("dialog")).toHaveTextContent("Release policy");
    expect(screen.getByRole("button", { name: "Reload policy details" })).toBeInTheDocument();
  });

  it("allows an excluded retained selection to re-enable the Wiki source class", () => {
    harness.query.data = {
      ...readiness,
      policy: {
        ...readiness.policy,
        wikiPages: [{ pageId: target.pageId, revisionNumber: target.revisionNumber }],
      },
      sources: [{
        ...readiness.sources[0]!,
        state: "excluded",
        selectedRevisionId: target.revisionId,
        selectedRevisionNumber: target.revisionNumber,
        nextAction: {
          kind: "pin_revision",
          pageId: target.pageId,
          revisionId: target.revisionId,
          revisionNumber: target.revisionNumber,
        },
      }],
    };
    harness.pin.isError = false;
    harness.pin.mutate.mockReset();
    renderWithI18n(<WorkspaceWikiKnowledgeActivation target={target} />);

    const action = screen.getByRole("button", { name: "Use as LM Wiki evidence" });
    expect(action).toBeEnabled();
    fireEvent.click(action);
    fireEvent.click(screen.getByRole("button", { name: "Pin exact revision" }));
    expect(harness.pin.mutate).toHaveBeenCalled();
  });

  it("fails personal knowledge closed with a permanent exclusion explanation", () => {
    renderWithI18n(<PersonalWikiKnowledgeActivation />);
    expect(screen.getByText("Always excluded")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Use as LM Wiki evidence" })).toBeDisabled();
    expect(screen.getByText(/permanently excluded/)).toBeInTheDocument();
  });
});
