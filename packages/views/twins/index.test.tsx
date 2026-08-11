import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { ApiClient } from "@multica/core/api/client";
import { setApiInstance } from "@multica/core/api";
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
});
