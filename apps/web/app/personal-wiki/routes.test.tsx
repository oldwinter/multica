// @vitest-environment jsdom

import { Suspense } from "react";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@multica/views/wiki", () => ({
  PersonalWikiPageView: ({ pageId }: { pageId?: string }) => (
    <div data-testid="personal-wiki-view">{pageId ?? "list"}</div>
  ),
  PersonalWikiRevisionView: ({ revisionId }: { revisionId: string }) => (
    <div data-testid="personal-revision-view">{revisionId}</div>
  ),
  WorkspaceWikiRevisionView: ({ revisionId }: { revisionId: string }) => (
    <div data-testid="workspace-revision-view">{revisionId}</div>
  ),
}));

import PersonalWikiPage from "./page";
import PersonalWikiDetailPage from "./[id]/page";
import PersonalWikiRevisionPage from "./revisions/[revisionId]/page";
import WorkspaceWikiRevisionPage from "../[workspaceSlug]/(dashboard)/wiki/revisions/[revisionId]/page";

afterEach(cleanup);

describe("Wiki Web routes", () => {
  it("mounts the global Personal Wiki list without workspace params", () => {
    render(<PersonalWikiPage />);
    expect(screen.getByTestId("personal-wiki-view").textContent).toBe("list");
  });

  it("passes decoded route identities to personal detail and immutable views", async () => {
    await act(async () => {
      render(
        <Suspense>
          <PersonalWikiDetailPage params={Promise.resolve({ id: "page-1" })} />
          <PersonalWikiRevisionPage params={Promise.resolve({ revisionId: "personal-revision-1" })} />
        </Suspense>,
      );
    });
    expect(screen.getByTestId("personal-wiki-view").textContent).toBe("page-1");
    expect(screen.getByTestId("personal-revision-view").textContent).toBe("personal-revision-1");
  });

  it("passes the immutable shared revision identity inside the workspace route", async () => {
    await act(async () => {
      render(
        <Suspense>
          <WorkspaceWikiRevisionPage params={Promise.resolve({ revisionId: "shared-revision-1" })} />
        </Suspense>,
      );
    });
    expect(screen.getByTestId("workspace-revision-view").textContent).toBe("shared-revision-1");
  });
});
