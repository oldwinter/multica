import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { ApiClient, ApiError } from "@multica/core/api/client";
import { setApiInstance } from "@multica/core/api";
import type { LMWikiRevision } from "@multica/core/twins";
import { beforeEach, describe, expect, it, vi } from "vitest";
import enCommon from "../locales/en/common.json";
import enTwins from "../locales/en/twins.json";
import { lifecycleFixture } from "./components/twin-workspace-view.test-fixture";
import { TwinsPage } from "./index";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/permissions", () => ({
  useWikiTwinPermissions: (_wsId: string, serverCanManage: boolean) => ({
    canManage: { allowed: serverCanManage },
    canMutate: serverCanManage,
    isLoading: false,
  }),
}));
vi.mock("../navigation", () => ({
  AppLink: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const resources = { en: { common: enCommon, twins: enTwins } };

describe("TwinsPage", () => {
  let client: ApiClient;

  beforeEach(() => {
    client = new ApiClient("https://api.example.test");
    setApiInstance(client);
  });

  it("keeps first-run Wiki creation explicit until a manager clicks refresh", async () => {
    const refresh = vi.spyOn(client, "refreshLMWiki").mockResolvedValue({
      created: true,
      revision: {
        id: "wiki-1",
        revision_number: 1,
        schema_version: 1,
        source_digest: "sha256:wiki",
        content: {},
        trigger_kind: "manual",
        requested_by_id: "member-1",
        created_at: "2026-08-11T08:00:00Z",
        review: null,
      },
    });
    vi.spyOn(client, "getLMWiki").mockResolvedValue({
      latest_revision: null,
      accepted_revision: null,
      pending_revision: null,
      revisions: [],
      can_manage: true,
    });
    vi.spyOn(client, "getTwins").mockResolvedValue({
      current_version: null,
      pending_proposal: null,
      proposals: [],
      versions: [],
      can_manage: true,
    });
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({ twin: null });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    const button = await screen.findByRole("button", { name: "Refresh Wiki" });
    expect(refresh).not.toHaveBeenCalled();
    fireEvent.click(button);
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
  });

  it("renders review-step states from the Twin Profile overview query", async () => {
    const fixture = lifecycleFixture();
    vi.spyOn(client, "getLMWiki").mockResolvedValue(fixture.wiki);
    vi.spyOn(client, "getTwins").mockResolvedValue(fixture.twin);
    vi.spyOn(client, "getLMWikiRevision").mockResolvedValue(fixture.wikiDetail);
    vi.spyOn(client, "getTwinProposal").mockResolvedValue(fixture.proposalDetail);
    vi.spyOn(client, "getTwinVersion").mockResolvedValue(fixture.versionDetail);
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({
      twin: {
        id: "twin-profile-1",
        name: "Workspace Twin",
        state: "pending-signoff",
        reviewDigest: "Review current workspace evidence",
        updatedAt: "2026-08-11T08:00:00Z",
        sourceCount: 1,
        assertionCount: 1,
        skillCount: 0,
        ruleCount: 0,
        assertions: [],
        topics: [],
        reviewSteps: fixture.reviewSteps,
      },
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    fireEvent.click(await screen.findByRole("tab", { name: "Twin Builder" }));
    const currentStep = await screen.findByText("Coordinate execution");
    expect(currentStep.closest("li")).toHaveAttribute("data-state", "current");
    expect(screen.getAllByTestId("twin-review-step")).toHaveLength(6);
  });

  it("shows the revision returned by a manual refresh while inspecting history", async () => {
    const user = userEvent.setup();
    const fixture = lifecycleFixture();
    const revision3: LMWikiRevision = {
      ...fixture.wikiDetail.revision,
      id: "wiki-3",
      revision_number: 3,
      source_digest: "sha256:wiki-3",
    };
    let wiki = fixture.wiki;
    vi.spyOn(client, "getLMWiki").mockImplementation(async () => wiki);
    vi.spyOn(client, "getTwins").mockResolvedValue(fixture.twin);
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({ twin: null });
    vi.spyOn(client, "getLMWikiRevision").mockImplementation(async (id) => {
      const revision = wiki.revisions.find((item) => item.id === id);
      if (!revision) throw new Error(`Missing test revision ${id}`);
      return { ...fixture.wikiDetail, revision };
    });
    vi.spyOn(client, "getTwinProposal").mockResolvedValue(fixture.proposalDetail);
    vi.spyOn(client, "getTwinVersion").mockResolvedValue(fixture.versionDetail);
    vi.spyOn(client, "refreshLMWiki").mockImplementation(async () => {
      wiki = {
        ...wiki,
        latest_revision: revision3,
        pending_revision: revision3,
        revisions: [revision3, ...wiki.revisions],
      };
      return { created: true, revision: revision3 };
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    expect(await screen.findByRole("heading", { name: "Revision r2" })).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "Wiki revision" }));
    await user.click(await screen.findByRole("option", { name: "r1 / manual" }));
    expect(await screen.findByRole("heading", { name: "Revision r1" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Refresh Wiki" }));
    expect(await screen.findByRole("heading", { name: "Revision r3" })).toBeInTheDocument();
  });

  it("follows the canonical revision after a stale review conflict", async () => {
    const user = userEvent.setup();
    const fixture = lifecycleFixture();
    const revision3: LMWikiRevision = {
      ...fixture.wikiDetail.revision,
      id: "wiki-3",
      revision_number: 3,
      source_digest: "sha256:wiki-3",
    };
    let wiki = fixture.wiki;
    vi.spyOn(client, "getLMWiki").mockImplementation(async () => wiki);
    vi.spyOn(client, "getTwins").mockResolvedValue(fixture.twin);
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({ twin: null });
    vi.spyOn(client, "getLMWikiRevision").mockImplementation(async (id) => {
      const revision = wiki.revisions.find((item) => item.id === id);
      if (!revision) throw new Error(`Missing test revision ${id}`);
      return { ...fixture.wikiDetail, revision };
    });
    vi.spyOn(client, "getTwinProposal").mockResolvedValue(fixture.proposalDetail);
    vi.spyOn(client, "getTwinVersion").mockResolvedValue(fixture.versionDetail);
    vi.spyOn(client, "acceptLMWikiRevision").mockImplementation(async () => {
      wiki = {
        ...wiki,
        latest_revision: revision3,
        pending_revision: revision3,
        revisions: [revision3, ...wiki.revisions],
      };
      throw new ApiError("LM Wiki revision is stale", 409, "Conflict");
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    expect(await screen.findByRole("heading", { name: "Revision r2" })).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "Wiki revision" }));
    await user.click(await screen.findByRole("option", { name: "r2 / manual" }));
    await user.click(screen.getByRole("button", { name: "Accept revision" }));
    await user.click(screen.getByRole("button", { name: "Confirm acceptance" }));

    expect(await screen.findByText("This review is out of date. Check the latest version and try again."))
      .toBeInTheDocument();
    expect(screen.queryByText("Couldn't save the decision. Try again.")).not.toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Revision r3" })).toBeInTheDocument();
  });

  it("selects the replacement proposal returned after editing a deposition", async () => {
    const fixture = lifecycleFixture();
    const taskId = "00000000-0000-4000-8000-000000000010";
    const depositionDetail = {
      ...fixture.proposalDetail,
      proposal: {
        ...fixture.proposalDetail.proposal,
        kind: "deposition",
        schema_version: 2,
      },
      run_evidence: {
        taskId,
        baseTwinVersionId: "00000000-0000-4000-8000-000000000011",
        evidenceDigest: `sha256:${"a".repeat(64)}`,
        taskStatus: "completed",
        completedAt: "2026-08-11T09:00:00Z",
        feedbackRating: "helped" as const,
        safeMetadata: {},
      },
    };
    const replacementId = "00000000-0000-4000-8000-000000000012";
    const replacementDetail = {
      ...depositionDetail,
      proposal: {
        ...depositionDetail.proposal,
        id: replacementId,
        content_digest: `sha256:${"b".repeat(64)}`,
      },
    };
    vi.spyOn(client, "getLMWiki").mockResolvedValue(fixture.wiki);
    vi.spyOn(client, "getTwins").mockResolvedValue({
      ...fixture.twin,
      pending_proposal: depositionDetail.proposal,
      proposals: [depositionDetail.proposal],
    });
    vi.spyOn(client, "getLMWikiRevision").mockResolvedValue(fixture.wikiDetail);
    const getProposal = vi.spyOn(client, "getTwinProposal").mockImplementation(async (id) => (
      id === replacementId ? replacementDetail : depositionDetail
    ));
    vi.spyOn(client, "getTwinVersion").mockResolvedValue(fixture.versionDetail);
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({ twin: null });
    const createDeposition = vi.spyOn(client, "createTwinDeposition").mockResolvedValue({
      deposition: null,
      proposal: {
        id: replacementId,
        kind: "deposition",
        schemaVersion: 2,
        contentDigest: `sha256:${"b".repeat(64)}`,
        createdAt: "2026-08-11T10:00:00Z",
      },
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    fireEvent.click(await screen.findByRole("tab", { name: "Twin Builder" }));
    fireEvent.click(await screen.findByRole("button", { name: "Edit deposition" }));
    const editedAssertions = [{
      id: "assertion-new",
      type: "quality_bar",
      text: "Keep the review decision explicit and auditable.",
      applicability: { keywords: ["review"] },
      evidence_citations: ["issue:42"],
    }];
    fireEvent.change(screen.getByLabelText("Assertions JSON"), {
      target: { value: JSON.stringify(editedAssertions, null, 2) },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create replacement" }));

    await waitFor(() => expect(createDeposition).toHaveBeenCalledWith(taskId, {
      replacesProposalId: "proposal-2",
      editedAssertions,
    }));
    await waitFor(() => expect(getProposal).toHaveBeenCalledWith(replacementId));
    expect(await screen.findAllByText(replacementId)).toHaveLength(2);
  });
});
