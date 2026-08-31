// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import enWiki from "../locales/en/wiki.json";

const harness = vi.hoisted(() => ({
  list: { isPending: false, isError: false, data: [] as unknown[] },
  search: { isPending: false, isError: false, data: [] as unknown[] },
  detail: { isPending: false, isError: false, data: undefined as unknown, refetch: vi.fn() },
  revisions: { isPending: false, isError: false, data: [] as unknown[], refetch: vi.fn() },
  push: vi.fn(),
  back: vi.fn(),
  create: { mutate: vi.fn(), isPending: false },
  update: { mutate: vi.fn(), isPending: false },
  remove: { mutate: vi.fn(), isPending: false },
  restore: { mutate: vi.fn(), isPending: false },
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: (options: { queryKey?: readonly unknown[] }) => {
      const key = options.queryKey ?? [];
      if (key.includes("search")) return harness.search;
      if (key.includes("revisions")) return harness.revisions;
      if (key.includes("detail")) return harness.detail;
      return harness.list;
    },
  };
});

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: harness.push, back: harness.back }),
  useOptionalNavigation: () => null,
  useAppOrigin: () => null,
  resolveClickIntent: () => "push",
}));

vi.mock("@multica/core/wiki", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/wiki")>();
  return {
    ...actual,
    useCreatePersonalWikiPage: () => harness.create,
    useUpdatePersonalWikiPage: () => harness.update,
    useDeletePersonalWikiPage: () => harness.remove,
    useRestorePersonalWikiRevision: () => harness.restore,
  };
});

import { PersonalWikiPageView } from "./personal-wiki-page";

const page = {
  id: "page-1",
  workspaceId: null,
  scope: "user",
  projectId: null,
  ownerUserId: "user-1",
  path: "private/notes.md",
  title: "Private notes",
  content: "# Private\n\n[Open workspace issue](/acme/issues/MUL-1)",
  createdBy: "user-1",
  currentRevisionNumber: 2,
  currentRevisionId: "revision-2",
  contentDigest: "sha256:private",
  lastSourceKind: "human",
  lastActorType: "member",
  lastActorId: "user-1",
  createdAt: "2026-08-23T10:00:00Z",
  updatedAt: "2026-08-23T11:00:00Z",
};

function renderPersonal(pageId?: string) {
  return render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <PersonalWikiPageView pageId={pageId} />
    </I18nProvider>,
  );
}

describe("PersonalWikiPageView", () => {
  beforeEach(() => {
    harness.list = { isPending: false, isError: false, data: [] };
    harness.search = { isPending: false, isError: false, data: [] };
    harness.detail = { isPending: false, isError: false, data: undefined, refetch: vi.fn() };
    harness.revisions = { isPending: false, isError: false, data: [], refetch: vi.fn() };
    harness.push.mockReset();
    harness.back.mockReset();
    for (const mutation of [harness.create, harness.update, harness.remove, harness.restore]) {
      mutation.mutate.mockReset();
      mutation.isPending = false;
    }
  });

  it("navigates global Personal Wiki list and stable revision routes without proposals", () => {
    harness.list = { isPending: false, isError: false, data: [page] };
    harness.detail = { isPending: false, isError: false, data: page, refetch: vi.fn() };
    renderPersonal("page-1");

    expect(screen.getByRole("heading", { name: "Personal Wiki" })).toBeInTheDocument();
    expect(screen.queryByText("Proposals")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open workspace issue" })).toHaveAttribute(
      "href",
      "/acme/issues/MUL-1",
    );
    fireEvent.click(screen.getByRole("button", { name: "Stable revision" }));
    expect(harness.push).toHaveBeenCalledWith("/personal-wiki/revisions/revision-2");
    fireEvent.click(screen.getByTitle("Back"));
    expect(harness.back).toHaveBeenCalledOnce();
  });

  it("creates a private page and navigates to its global detail route", () => {
    harness.create.mutate.mockImplementation((input, options) => {
      expect(input).toMatchObject({ path: "secret.md", title: "Secret" });
      options.onSuccess(page);
    });
    renderPersonal();

    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    fireEvent.change(screen.getByLabelText(/^Path/), { target: { value: "secret.md" } });
    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Secret" } });
    fireEvent.change(screen.getByLabelText("Content"), { target: { value: "A private secret" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(harness.push).toHaveBeenCalledWith("/personal-wiki/page-1");
  });

  it("keeps personal Wiki creation disabled until the title and content are meaningful", () => {
    renderPersonal();
    fireEvent.click(screen.getByRole("button", { name: "New page" }));
    const create = screen.getByRole("button", { name: "Create" });
    const title = screen.getByLabelText("Title");
    const content = screen.getByLabelText("Content");

    expect(create).toBeDisabled();
    fireEvent.change(title, { target: { value: "Private notes" } });
    expect(create).toBeDisabled();
    fireEvent.change(content, { target: { value: " \n\t" } });
    expect(create).toBeDisabled();
    fireEvent.change(content, { target: { value: "# " } });
    expect(create).toBeDisabled();
    fireEvent.change(content, { target: { value: "A private note" } });
    expect(create).toBeEnabled();
    fireEvent.change(title, { target: { value: "  " } });
    expect(create).toBeDisabled();
  });

  it("lets the short mobile Personal Wiki shell scroll to detail actions", () => {
    renderPersonal();
    expect(screen.getByTestId("personal-wiki-page")).toHaveClass("overflow-y-auto", "lg:overflow-hidden");
  });

  it("preserves the local draft and advances the CAS base for manual merge", async () => {
    harness.detail = {
      isPending: false,
      isError: false,
      data: page,
      refetch: vi.fn().mockResolvedValue({ data: { ...page, content: "# Server", currentRevisionNumber: 3 } }),
    };
    harness.update.mutate.mockImplementation((_input, options) => {
      options.onError(new ApiError("conflict", 409, "Conflict", {
        code: "wiki_revision_conflict",
        current_revision_number: 3,
      }));
    });
    renderPersonal("page-1");

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.change(screen.getByLabelText("Content"), { target: { value: "# My draft" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("alertdialog")).toHaveTextContent("revision 3");
    fireEvent.click(screen.getByRole("button", { name: "Merge my draft" }));

    expect(await screen.findByDisplayValue("# My draft")).toBeInTheDocument();
    expect(harness.detail.refetch).toHaveBeenCalledOnce();
  });

  it("keeps delete failures visible after the confirmation closes", () => {
    harness.list = { isPending: false, isError: false, data: [page] };
    harness.detail = { isPending: false, isError: false, data: page, refetch: vi.fn() };
    harness.remove.mutate.mockImplementation((_id, options) => options.onError(new Error("offline")));
    renderPersonal("page-1");

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByRole("alert")).toHaveTextContent("The action could not be completed");
  });
});
