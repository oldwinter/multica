// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { describe, expect, it, vi } from "vitest";
import enCommon from "../locales/en/common.json";
import enTwins from "../locales/en/twins.json";
import { lifecycleFixture } from "./components/twin-workspace-view.test-fixture";
import { TwinWorkspaceView } from "./components/twin-workspace-view";

vi.mock("../navigation", () => ({
  AppLink: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
  useOptionalNavigation: () => null,
}));
vi.mock("./components/twin-activation-readiness", () => ({
  TwinActivationReadiness: () => null,
}));

describe("Twin review profile integration", () => {
  it("does not synthesize a review spine without persisted profile steps", () => {
    render(
      <I18nProvider locale="en" resources={{ en: { common: enCommon, twins: enTwins } }}>
        <WorkspaceSlugProvider slug="acme">
          <TwinWorkspaceView {...lifecycleFixture()} reviewSteps={[]} />
        </WorkspaceSlugProvider>
      </I18nProvider>,
    );

    fireEvent.click(screen.getByRole("tab", { name: "Twin Builder" }));
    expect(screen.queryByRole("region", { name: "Twin review progress" })).not.toBeInTheDocument();
  });
});
