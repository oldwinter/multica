// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

const setTitle = vi.fn();

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/views/skill-evolution", () => ({
  SkillEvolutionPage: (props: Record<string, string>) => (
    <output data-testid="skill-evolution-route">{JSON.stringify(props)}</output>
  ),
}));

vi.mock("@/hooks/use-document-title", () => ({
  useDocumentTitle: (title: string) => setTitle(title),
}));

import { SkillEvolutionPage } from "./skill-evolution-page";

describe("desktop SkillEvolutionPage route", () => {
  it("passes session route identities to the shared page", () => {
    render(
      <MemoryRouter initialEntries={["/acme/skills/skill-1/evolution"]}>
        <Routes>
          <Route
            path=":workspaceSlug/skills/:id/evolution"
            element={<SkillEvolutionPage />}
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(JSON.parse(screen.getByTestId("skill-evolution-route").textContent ?? ""))
      .toEqual({
        workspaceId: "workspace-1",
        workspaceSlug: "acme",
        skillId: "skill-1",
      });
    expect(setTitle).toHaveBeenCalledWith("Evolution");
  });
});
