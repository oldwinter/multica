// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const params = vi.hoisted(() => ({ current: {} as Record<string, string> }));

vi.mock("react-router-dom", () => ({ useParams: () => params.current }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    personalWiki: () => "/acme/personal-wiki",
    personalWikiPage: (id: string) => `/acme/personal-wiki/${id}`,
    personalWikiRevision: (id: string) => `/acme/personal-wiki/revisions/${id}`,
  }),
}));
vi.mock("@multica/views/wiki", () => ({
  WikiPageView: ({ pageId, personalWikiPath }: { pageId?: string; personalWikiPath?: string }) => (
    <div data-testid="wiki">{pageId ?? "list"}:{personalWikiPath}</div>
  ),
  WorkspaceWikiRevisionView: ({ revisionId }: { revisionId: string }) => (
    <div data-testid="wiki-revision">{revisionId}</div>
  ),
  PersonalWikiPageView: ({ pageId, routePaths }: { pageId?: string; routePaths: { list: () => string } }) => (
    <div data-testid="personal-wiki">{pageId ?? "list"}:{routePaths.list()}</div>
  ),
  PersonalWikiRevisionView: ({ revisionId, listPath }: { revisionId: string; listPath: string }) => (
    <div data-testid="personal-revision">{revisionId}:{listPath}</div>
  ),
}));

import {
  PersonalWikiDetailPage,
  PersonalWikiListPage,
  PersonalWikiRevisionPage,
  WikiListPage,
  WikiRevisionPage,
} from "./wiki-page";

afterEach(() => {
  cleanup();
  params.current = {};
});

describe("Desktop Wiki route leaves", () => {
  it("routes the Workspace Wiki Personal tab into the workspace-scoped global alias shell", () => {
    render(<WikiListPage />);
    expect(screen.getByTestId("wiki").textContent).toBe("list:/acme/personal-wiki");
  });

  it("mounts Personal Wiki list, detail, and stable revision leaves", () => {
    const list = render(<PersonalWikiListPage />);
    expect(screen.getByTestId("personal-wiki").textContent).toBe("list:/acme/personal-wiki");
    list.unmount();

    params.current = { id: "page-1" };
    const detail = render(<PersonalWikiDetailPage />);
    expect(screen.getByTestId("personal-wiki").textContent).toBe("page-1:/acme/personal-wiki");
    detail.unmount();

    params.current = { revisionId: "revision-1" };
    render(<PersonalWikiRevisionPage />);
    expect(screen.getByTestId("personal-revision").textContent).toBe("revision-1:/acme/personal-wiki");
  });

  it("mounts the immutable shared revision leaf", () => {
    params.current = { revisionId: "shared-revision-1" };
    render(<WikiRevisionPage />);
    expect(screen.getByTestId("wiki-revision").textContent).toBe("shared-revision-1");
  });
});
