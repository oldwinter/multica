// @vitest-environment jsdom

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance } from "@multica/core/api";
import { ApiClient } from "@multica/core/api/client";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
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
  useNavigation: () => ({ push: vi.fn() }),
  useOptionalNavigation: () => null,
}));

const resources = { en: { common: enCommon, twins: enTwins } };

describe("TwinsPage concurrent actions", () => {
  let client: ApiClient;

  beforeEach(() => {
    client = new ApiClient("https://api.example.test");
    setApiInstance(client);
  });

  it("surfaces an older action failure after a newer action succeeds", async () => {
    const fixture = lifecycleFixture();
    let rejectRefresh: (error: Error) => void = () => undefined;
    vi.spyOn(client, "getLMWiki").mockResolvedValue(fixture.wiki);
    vi.spyOn(client, "getTwins").mockResolvedValue(fixture.twin);
    vi.spyOn(client, "getTwinOverview").mockResolvedValue({ twin: null });
    vi.spyOn(client, "getLMWikiRevision").mockResolvedValue(fixture.wikiDetail);
    vi.spyOn(client, "getTwinProposal").mockResolvedValue(fixture.proposalDetail);
    vi.spyOn(client, "getTwinVersion").mockResolvedValue(fixture.versionDetail);
    const refresh = vi.spyOn(client, "refreshLMWiki").mockImplementation(() => (
      new Promise((_resolve, reject) => {
        rejectRefresh = reject;
      })
    ));
    const acceptTwin = vi.spyOn(client, "acceptTwinProposal").mockResolvedValue({
      created: true,
      version: fixture.versionDetail.version,
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <I18nProvider locale="en" resources={resources}>
        <WorkspaceSlugProvider slug="acme">
          <QueryClientProvider client={queryClient}><TwinsPage /></QueryClientProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Refresh Wiki" }));
    await waitFor(() => expect(refresh).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    fireEvent.click(await screen.findByRole("button", { name: "Sign off proposal" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm sign-off" }));
    await waitFor(() => expect(acceptTwin).toHaveBeenCalledOnce());
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

    await act(async () => rejectRefresh(new Error("offline")));

    expect(
      await screen.findByText("Couldn't save the decision. Try again."),
    ).toBeInTheDocument();
  });
});
