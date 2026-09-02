import type { ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const mockSelectSkin = vi.hoisted(() => vi.fn());
const mockSelectAppearance = vi.hoisted(() => vi.fn());

vi.mock("next/link", () => ({
  default: ({ children, ...props }: { children: ReactNode; href: string }) => (
    <a {...props}>{children}</a>
  ),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: { user: null }) => unknown) =>
    selector({ user: null }),
}));

vi.mock("@multica/views/appearance", () => ({
  useAppearancePreferences: () => ({
    preferences: {
      skin: "tension",
      requestedAppearance: "system",
    },
    selectSkin: mockSelectSkin,
    selectAppearance: mockSelectAppearance,
  }),
}));

vi.mock("../i18n", () => ({
  docsHrefForLocale: () => "/docs",
  useLocale: () => ({
    locale: "en",
    t: {
      header: {
        github: "GitHub",
        cta: "Get started",
        dashboard: "Dashboard",
        docs: "Docs",
        changelog: "Changelog",
        useCases: "Use cases",
        navigation: "Main navigation",
        openMenu: "Open menu",
        closeMenu: "Close menu",
        skipToContent: "Skip to content",
      },
    },
  }),
}));

vi.mock("../utils/use-dashboard-cta", () => ({
  useDashboardCtaHref: () => "/login",
}));

vi.mock("../utils/use-github-stars", () => ({
  formatStarCount: (value: number) => String(value),
  useGithubStars: () => null,
}));

import { LandingHeader } from "./landing-header";

describe("LandingHeader appearance picker", () => {
  it("routes skin and mode selections through the appearance bridge", async () => {
    const user = userEvent.setup();
    render(<LandingHeader />);

    fireEvent.click(screen.getByRole("button", { name: "Appearance" }));
    await user.click(screen.getByRole("menuitemradio", { name: "Field" }));
    expect(mockSelectSkin).toHaveBeenCalledWith("field");

    fireEvent.click(screen.getByRole("button", { name: "Appearance" }));
    await user.click(screen.getByRole("menuitemradio", { name: "Dark" }));
    expect(mockSelectAppearance).toHaveBeenCalledWith("dark");
  });
});
