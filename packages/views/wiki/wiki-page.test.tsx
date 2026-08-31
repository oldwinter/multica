// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import enWiki from "../locales/en/wiki.json";

const harness = vi.hoisted(() => ({
  list: { isLoading: false, isError: false, data: [] as unknown[] },
  detail: {
    isLoading: false,
    isError: false,
    data: undefined as unknown,
    refetch: vi.fn(),
  },
  revisions: { isPending: false, isError: false, data: [] as unknown[], refetch: vi.fn() },
  proposals: { isPending: false, isError: false, data: [] as unknown[], refetch: vi.fn() },
  search: { isPending: false, isError: false, data: [] as unknown[] },
  readiness: { isPending: false, isError: false, data: undefined as unknown, refetch: vi.fn() },
  projects: { isLoading: false, data: [{ id: "project-1", title: "Roadmap" }] },
  push: vi.fn(),
  replace: vi.fn(),
  pathname: "/acme/wiki",
  urlSearch: "",
  hash: "",
  create: { mutate: vi.fn(), isPending: false },
  update: { mutate: vi.fn(), isPending: false },
  remove: { mutate: vi.fn(), isPending: false },
  restore: { mutate: vi.fn(), isPending: false },
  accept: { mutate: vi.fn(), isPending: false },
  reject: { mutate: vi.fn(), isPending: false },
  pin: { mutate: vi.fn(), isPending: false, isError: false },
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: { queryKey?: readonly unknown[] }) => {
      const key = options.queryKey ?? [];
      if (key[0] === "projects") return harness.projects;
      if (key.includes("revisions")) return harness.revisions;
      if (key.includes("proposals")) return harness.proposals;
      if (key.includes("search")) return harness.search;
      if (key.includes("knowledge-readiness")) return harness.readiness;
      if (key.includes("detail")) return harness.detail;
      return harness.list;
    },
  };
});

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({
  paths: { personalWiki: () => "/personal-wiki" },
  useWorkspaceSlug: () => "acme",
  useWorkspacePaths: () => ({
    wiki: () => "/acme/wiki",
    wikiPage: (id: string) => `/acme/wiki/${id}`,
    wikiRevision: (id: string) => `/acme/wiki/revisions/${id}`,
  }),
}));
vi.mock("../navigation", () => ({
  resolveClickIntent: () => "push",
  useAppOrigin: () => null,
  useNavigation: () => ({
    push: harness.push,
    replace: harness.replace,
    pathname: harness.pathname,
    searchParams: new URLSearchParams(harness.urlSearch),
    hash: harness.hash,
  }),
  useOptionalNavigation: () => ({
    push: harness.push,
    replace: harness.replace,
    pathname: harness.pathname,
    searchParams: new URLSearchParams(harness.urlSearch),
    hash: harness.hash,
  }),
}));
vi.mock("@multica/core/wiki", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/wiki")>();
  return {
    ...actual,
    useCreateWikiPage: () => harness.create,
    useUpdateWikiPage: () => harness.update,
    useDeleteWikiPage: () => harness.remove,
    useRestoreWikiRevision: () => harness.restore,
    useAcceptWikiProposal: () => harness.accept,
    useRejectWikiProposal: () => harness.reject,
    usePinWikiRevisionAsLMWikiEvidence: () => harness.pin,
  };
});

import { WikiPageView } from "./index";

const page = {
  id: "page-1",
  workspaceId: "workspace-1",
  scope: "workspace",
  projectId: null,
  ownerUserId: null,
  path: "guide.md",
  title: "Guide",
  content: "# Shared guide",
  createdBy: "member-1",
  currentRevisionNumber: 4,
  currentRevisionId: "revision-4",
  contentDigest: "sha256:guide",
  lastSourceKind: "human",
  lastActorType: "member",
  lastActorId: "member-1",
  createdAt: "2026-08-23T10:00:00Z",
  updatedAt: "2026-08-23T11:00:00Z",
};

function renderWiki(pageId?: string) {
  return render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <WikiPageView pageId={pageId} />
    </I18nProvider>,
  );
}

describe("WikiPageView", () => {
  beforeEach(() => {
    harness.list = { isLoading: false, isError: false, data: [] };
    harness.detail = { isLoading: false, isError: false, data: undefined, refetch: vi.fn() };
    harness.revisions = { isPending: false, isError: false, data: [], refetch: vi.fn() };
    harness.proposals = { isPending: false, isError: false, data: [], refetch: vi.fn() };
    harness.search = { isPending: false, isError: false, data: [] };
    harness.readiness = { isPending: false, isError: false, data: undefined, refetch: vi.fn() };
    harness.push.mockClear();
    harness.replace.mockClear();
    harness.pathname = "/acme/wiki";
    harness.urlSearch = "";
    harness.hash = "";
    for (const mutation of [harness.create, harness.update, harness.remove, harness.restore, harness.accept, harness.reject, harness.pin]) {
      mutation.mutate.mockReset();
      mutation.isPending = false;
    }
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  it("renders loading, empty, and error states without dropping the shell", () => {
    harness.list = { isLoading: true, isError: false, data: [] };
    const { unmount } = renderWiki();
    expect(screen.getByRole("heading", { name: "Wiki" })).toBeInTheDocument();
    expect(screen.getByText("Loading wiki…")).toBeInTheDocument();
    unmount();

    harness.list = { isLoading: false, isError: true, data: [] };
    renderWiki();
    expect(screen.getByText("Could not load wiki pages.")).toBeInTheDocument();
  });

  it("keeps the default main landmark opt-in for standalone callers", () => {
    const { container, unmount } = renderWiki();
    expect(container.querySelector('[data-testid="wiki-page"]')?.tagName).toBe("MAIN");
    unmount();

    render(
      <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
        <WikiPageView rootElement="div" />
      </I18nProvider>,
    );
    expect(document.querySelector('[data-testid="wiki-page"]')?.tagName).toBe("DIV");
  });

  it("opens the current immutable revision from document metadata", () => {
    harness.detail = { isLoading: false, isError: false, data: page, refetch: vi.fn() };
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Open stable revision" }));
    expect(harness.push).toHaveBeenCalledWith("/acme/wiki/revisions/revision-4");
  });

  it("groups cross-scope search results and opens the chosen page", () => {
    harness.search = {
      isPending: false,
      isError: false,
      data: [
        { ...page, id: "workspace-page", title: "Workspace guide" },
        { ...page, id: "project-page", title: "Project guide", scope: "project", projectId: "project-1" },
        { ...page, id: "personal-page", title: "Personal guide", workspaceId: null, scope: "user", ownerUserId: "member-1" },
      ],
    };
    renderWiki();
    fireEvent.change(screen.getByRole("textbox", { name: "Search Wiki" }), { target: { value: "guide" } });
    expect(screen.getByRole("heading", { name: "Workspace" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Project" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Personal" })).toBeInTheDocument();

    fireEvent.click(screen.getByText("Workspace guide"));
    expect(harness.push).toHaveBeenCalledWith("/acme/wiki/workspace-page");
    fireEvent.click(screen.getByText("Project guide"));
    expect(harness.push).toHaveBeenCalledWith("/acme/wiki/project-page");
    fireEvent.click(screen.getByText("Personal guide"));
    expect(harness.push).toHaveBeenCalledWith("/personal-wiki/personal-page");
  });

  it("aligns a project deep link with its scope and project picker", async () => {
    harness.detail.data = { ...page, scope: "project", projectId: "project-1" };
    renderWiki("page-1");
    await waitFor(() => expect(screen.getByRole("tab", { name: "Project" })).toHaveAttribute("aria-selected", "true"));
    expect(screen.getByRole("combobox")).toHaveTextContent("Roadmap");
  });

  it("hydrates the project collection from the URL", async () => {
    harness.urlSearch = "scope=project&project_id=project-1";
    renderWiki();

    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Project" })).toHaveAttribute("aria-selected", "true");
    });
    expect(screen.getByRole("combobox")).toHaveTextContent("Roadmap");
  });

  it("uses replace for collection scope changes and preserves unrelated location state", () => {
    harness.urlSearch = "view=grid";
    harness.hash = "#wiki-note";
    renderWiki();

    fireEvent.click(screen.getByRole("tab", { name: "Project" }));

    expect(harness.replace).toHaveBeenCalledWith(
      "/acme/wiki?view=grid&scope=project#wiki-note",
    );
    expect(harness.push).not.toHaveBeenCalled();
  });

  it("writes the selected project id to the collection URL", async () => {
    harness.urlSearch = "scope=project&view=grid";
    const user = userEvent.setup();
    renderWiki();

    await user.click(screen.getByRole("combobox"));
    await user.click(await screen.findByRole("option", { name: "Roadmap" }));

    expect(harness.replace).toHaveBeenCalledWith(
      "/acme/wiki?scope=project&view=grid&project_id=project-1",
    );
  });

  it("keeps a project URL visible but disables list creation until a project is selected", () => {
    harness.urlSearch = "scope=project";
    renderWiki();

    expect(screen.getByRole("tab", { name: "Project" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("combobox")).toHaveTextContent("Select a project");
    expect(screen.getByRole("button", { name: "New page" })).toBeDisabled();
  });

  it("leaves a project detail page before changing its collection project", async () => {
    harness.projects.data = [
      { id: "project-1", title: "Roadmap" },
      { id: "project-2", title: "Launch" },
    ];
    harness.detail = {
      isLoading: false,
      isError: false,
      data: { ...page, scope: "project", projectId: "project-1" },
      refetch: vi.fn(),
    };
    const user = userEvent.setup();
    renderWiki("page-1");

    await waitFor(() => expect(screen.getByRole("tab", { name: "Project" })).toHaveAttribute("aria-selected", "true"));
    await user.click(screen.getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Launch" }));

    expect(harness.push).toHaveBeenCalledWith(
      "/acme/wiki?scope=project&project_id=project-2",
    );
    expect(harness.replace).not.toHaveBeenCalled();
  });

  it("labels a new page action as create instead of save", () => {
    renderWiki();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });

  it("keeps workspace Wiki creation disabled until the title and content are meaningful", () => {
    renderWiki();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    const create = screen.getByRole("button", { name: "Create" });
    const title = screen.getByLabelText("Title");
    const content = screen.getByLabelText("Content");

    expect(create).toBeDisabled();
    fireEvent.change(title, { target: { value: "  " } });
    fireEvent.change(content, { target: { value: "A useful guide" } });
    expect(create).toBeDisabled();
    fireEvent.change(title, { target: { value: "Guide" } });
    expect(create).toBeEnabled();

    fireEvent.change(content, { target: { value: " \n\t" } });
    expect(create).toBeDisabled();
    fireEvent.change(content, { target: { value: "# " } });
    expect(create).toBeDisabled();
    fireEvent.change(content, { target: { value: "# Guide\n\nA useful guide" } });
    expect(create).toBeEnabled();
  });

  it("lets the short mobile Wiki shell scroll to detail actions", () => {
    renderWiki();
    expect(screen.getByTestId("wiki-page")).toHaveClass("overflow-y-auto", "lg:overflow-hidden");
  });

  it("sends expectedRevisionNumber with direct edits", () => {
    harness.detail.data = page;
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.change(screen.getByDisplayValue("# Shared guide"), { target: { value: "# Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(harness.update.mutate).toHaveBeenCalledWith({
      expectedRevisionNumber: 4,
      path: "guide.md",
      title: "Guide",
      content: "# Updated",
    }, expect.any(Object));
  });

  it("copies the immutable revision citation instead of the mutable path", async () => {
    harness.detail.data = page;
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Copy revision citation" }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      "wiki_page_revision:revision-4",
    ));
    expect(screen.getByRole("button", { name: "Revision citation copied" })).toBeInTheDocument();
  });

  it("preserves the local draft and offers explicit reload or merge on a stale edit", async () => {
    harness.detail.data = page;
    harness.detail.refetch.mockResolvedValue({ data: { ...page, content: "# Server edit", currentRevisionNumber: 5 } });
    harness.update.mutate.mockImplementation((_input, options) => options.onError(
      new ApiError("stale", 409, "Conflict", { code: "wiki_revision_conflict", current_revision_number: 5 }),
    ));
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.change(screen.getByDisplayValue("# Shared guide"), { target: { value: "# My draft" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("alertdialog")).toHaveTextContent("This page changed while you were editing");
    fireEvent.click(screen.getByRole("button", { name: "Merge my draft" }));
    await waitFor(() => expect(screen.getByDisplayValue("# My draft")).toBeInTheDocument());
    expect(screen.getByText("Review your merge")).toBeInTheDocument();
  });

  it("uses an accessible delete dialog and surfaces a failed delete inside it", () => {
    harness.detail.data = page;
    harness.remove.mutate.mockImplementation((_id, options) => options.onError(new Error("failed")));
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByRole("alertdialog")).toHaveTextContent("Delete this Wiki page?");
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByRole("alert")).toHaveTextContent("The action could not be completed");
  });
});
