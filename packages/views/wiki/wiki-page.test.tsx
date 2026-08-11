// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import enWiki from "../locales/en/wiki.json";

const harness = vi.hoisted(() => ({
  list: {
    isLoading: false,
    isError: false,
    data: [] as unknown[],
  },
  detail: {
    isLoading: false,
    isError: false,
    data: undefined as unknown,
  },
  projects: {
    isLoading: false,
    data: [] as { id: string; title: string }[],
  },
  mutate: vi.fn(),
  invalidateQueries: vi.fn(),
  push: vi.fn(),
  lastMutation: null as null | {
    mutationFn: () => Promise<unknown>;
    onSuccess?: (data: unknown) => Promise<void> | void;
  },
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: { queryKey?: unknown[] }) => {
      const key = options.queryKey ?? [];
      if (Array.isArray(key) && key.includes("detail")) return harness.detail;
      if (Array.isArray(key) && key[0] === "projects") return harness.projects;
      return harness.list;
    },
    useMutation: (options: {
      mutationFn: () => Promise<unknown>;
      onSuccess?: (data: unknown) => Promise<void> | void;
    }) => {
      harness.lastMutation = options;
      return {
        mutate: () => {
          harness.mutate();
          void Promise.resolve(options.mutationFn()).then((data) => options.onSuccess?.(data));
        },
        isPending: false,
        isError: false,
        error: null,
      };
    },
    useQueryClient: () => ({ invalidateQueries: harness.invalidateQueries }),
  };
});

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    wiki: () => "/acme/wiki",
    wikiPage: (id: string) => `/acme/wiki/${id}`,
  }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: harness.push }),
}));

vi.mock("@multica/core/api", () => ({
  api: {
    createWikiPage: vi.fn(async () => ({
      id: "new-page",
      path: "created.md",
      title: "Created",
      content: "# Created",
      scope: "workspace",
      workspace_id: "workspace-1",
    })),
    updateWikiPage: vi.fn(async () => ({
      id: "page-1",
      path: "notes.md",
      title: "Updated",
      content: "# Updated",
      scope: "user",
      workspace_id: null,
    })),
    deleteWikiPage: vi.fn(async () => undefined),
    listProjects: vi.fn(async () => ({ projects: [] })),
  },
}));

import { WikiPageView } from "./index";

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
    harness.detail = { isLoading: false, isError: false, data: undefined };
    harness.projects = { isLoading: false, data: [{ id: "proj-1", title: "Roadmap" }] };
    harness.mutate.mockClear();
    harness.invalidateQueries.mockClear();
    harness.push.mockClear();
    harness.lastMutation = null;
    vi.spyOn(window, "confirm").mockReturnValue(true);
  });

  it("renders the wiki shell and scope tabs", () => {
    renderWiki();
    expect(screen.getByRole("heading", { name: "Wiki" })).toBeInTheDocument();
    expect(screen.getByText("Workspace")).toBeInTheDocument();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New page" })).toBeInTheDocument();
  });

  it("lists pages when the query returns data", () => {
    harness.list = {
      isLoading: false,
      isError: false,
      data: [{ id: "page-1", path: "index.md", title: "Home" }],
    };
    renderWiki();
    expect(screen.getByText("Home")).toBeInTheDocument();
    expect(screen.getByText("index.md")).toBeInTheDocument();
  });

  it("shows loading and error list states", () => {
    harness.list = { isLoading: true, isError: false, data: [] };
    const { unmount } = renderWiki();
    expect(screen.getByText("Loading wiki…")).toBeInTheDocument();
    unmount();

    harness.list = { isLoading: false, isError: true, data: [] };
    renderWiki();
    expect(screen.getByText("Could not load wiki pages.")).toBeInTheDocument();
  });

  it("shows the cross-workspace personal hint on the personal scope", () => {
    renderWiki();
    fireEvent.click(screen.getByText("Personal"));
    expect(
      screen.getByText("Personal pages follow you across every workspace."),
    ).toBeInTheDocument();
  });

  it("shows project picker on project scope and selects a project", async () => {
    renderWiki();
    fireEvent.click(screen.getByRole("tab", { name: "Project" }));
    expect(screen.getAllByText("Select a project to browse its wiki.").length).toBeGreaterThan(0);
    const combo = screen.getByRole("combobox");
    fireEvent.click(combo);
    const option = await screen.findByRole("option", { name: "Roadmap" });
    fireEvent.click(option);
    // Selecting a project clears the empty-state pick message in the list.
    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });

  it("opens create form and creates a page", async () => {
    renderWiki();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    expect(screen.getByText("Path")).toBeInTheDocument();

    const pathInput = screen.getByPlaceholderText("index.md");
    fireEvent.change(pathInput, { target: { value: "created.md" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(harness.mutate).toHaveBeenCalled();
    // onSuccess navigates to the new page
    await vi.waitFor(() => {
      expect(harness.push).toHaveBeenCalledWith("/acme/wiki/new-page");
    });
  });

  it("cancels create form", () => {
    renderWiki();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByPlaceholderText("index.md")).not.toBeInTheDocument();
  });

  it("renders detail content and edit/delete flows", async () => {
    harness.detail = {
      isLoading: false,
      isError: false,
      data: {
        id: "page-1",
        path: "notes.md",
        title: "Notes",
        content: "# Hello wiki",
        workspace_id: null,
        scope: "user",
      },
    };
    renderWiki("page-1");
    expect(screen.getByRole("heading", { name: "Notes" })).toBeInTheDocument();
    expect(screen.getByText("Hello wiki")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByDisplayValue("notes.md")).toBeInTheDocument();
    fireEvent.change(screen.getByDisplayValue("Notes"), { target: { value: "Updated" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => {
      expect(harness.invalidateQueries).toHaveBeenCalled();
    });

    // re-open edit then cancel
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await vi.waitFor(() => {
      expect(harness.push).toHaveBeenCalledWith("/acme/wiki");
    });
  });

  it("navigates when a list item is clicked", () => {
    harness.list = {
      isLoading: false,
      isError: false,
      data: [{ id: "page-9", path: "a.md", title: "A" }],
    };
    renderWiki();
    fireEvent.click(screen.getByText("A"));
    expect(harness.push).toHaveBeenCalledWith("/acme/wiki/page-9");
  });

  it("shows detail loading and error states", () => {
    harness.detail = { isLoading: true, isError: false, data: undefined };
    const { unmount } = renderWiki("page-1");
    expect(screen.getByText("Loading wiki…")).toBeInTheDocument();
    unmount();

    harness.detail = { isLoading: false, isError: true, data: undefined };
    renderWiki("page-1");
    expect(screen.getByText("Could not load wiki pages.")).toBeInTheDocument();
  });

  it("navigates to wiki root when scope changes while viewing a page", () => {
    harness.detail = {
      isLoading: false,
      isError: false,
      data: {
        id: "page-1",
        path: "notes.md",
        title: "Notes",
        content: "body",
        workspace_id: "workspace-1",
        scope: "workspace",
      },
    };
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("tab", { name: "Personal" }));
    expect(harness.push).toHaveBeenCalledWith("/acme/wiki");
  });

  it("fills create form fields", () => {
    renderWiki();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    fireEvent.change(screen.getByPlaceholderText("index.md"), { target: { value: "x.md" } });
    const titleInputs = screen.getAllByRole("textbox");
    // path, title, content textareas/inputs
    fireEvent.change(titleInputs[1]!, { target: { value: "Title" } });
    fireEvent.change(titleInputs[2]!, { target: { value: "# body" } });
    expect(screen.getByDisplayValue("x.md")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Title")).toBeInTheDocument();
  });

  it("edits path and content fields in edit mode", () => {
    harness.detail = {
      isLoading: false,
      isError: false,
      data: {
        id: "page-1",
        path: "notes.md",
        title: "Notes",
        content: "old",
        workspace_id: null,
        scope: "user",
      },
    };
    renderWiki("page-1");
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.change(screen.getByDisplayValue("notes.md"), { target: { value: "new.md" } });
    fireEvent.change(screen.getByDisplayValue("old"), { target: { value: "new body" } });
    expect(screen.getByDisplayValue("new.md")).toBeInTheDocument();
    expect(screen.getByDisplayValue("new body")).toBeInTheDocument();
  });
});
