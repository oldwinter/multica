// @vitest-environment jsdom

import { cleanup, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { WorkspaceLoader } from "./workspace-loader";

afterEach(cleanup);

describe("WorkspaceLoader", () => {
  it("exposes a visible, localized busy main landmark while workspace identity resolves", () => {
    renderWithI18n(<WorkspaceLoader />, { locale: "zh-Hans" });

    const main = screen.getByRole("main");
    expect(main).toHaveAttribute("aria-busy", "true");
    expect(within(main).getByRole("status")).toHaveTextContent("正在加载工作区...");
  });
});
