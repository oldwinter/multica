// @vitest-environment jsdom

import { Suspense } from "react";
import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/views/skill-evolution", () => ({
  SkillEvolutionPage: (props: Record<string, string>) => (
    <output data-testid="skill-evolution-route">{JSON.stringify(props)}</output>
  ),
}));

import SkillEvolutionRoute from "./page";

describe("SkillEvolutionRoute", () => {
  it("passes route and workspace identities to the shared page", async () => {
    await act(async () => {
      render(
        <Suspense>
          <SkillEvolutionRoute
            params={Promise.resolve({ workspaceSlug: "acme", id: "skill-1" })}
          />
        </Suspense>,
      );
    });

    expect(JSON.parse(screen.getByTestId("skill-evolution-route").textContent ?? ""))
      .toEqual({
        workspaceId: "workspace-1",
        workspaceSlug: "acme",
        skillId: "skill-1",
      });
  });
});
