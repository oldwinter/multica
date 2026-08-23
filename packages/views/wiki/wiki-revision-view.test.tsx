// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WikiRevision } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";
import { ImmutableWikiRevision } from "./wiki-revision-view";

const revision: WikiRevision = {
  id: "revision-2",
  pageId: "page-1",
  revisionNumber: 2,
  path: "handbook/质量标准.md",
  title: "跨工作区长期保留的质量标准与操作约束",
  content: "# Exact content\n\n[Open source issue](/issues/MUL-1)",
  contentDigest: "sha256:exact-content-digest",
  actorType: "member",
  actorId: "member-1",
  sourceKind: "human",
  sourceRefId: null,
  createdAt: "2026-08-23T11:00:00Z",
};

function renderRevision(overrides: Partial<React.ComponentProps<typeof ImmutableWikiRevision>> = {}) {
  const props: React.ComponentProps<typeof ImmutableWikiRevision> = {
    revision,
    isPending: false,
    isError: false,
    onRetry: vi.fn(),
    onBack: vi.fn(),
    citationPrefix: "wiki_page_revision",
    personal: false,
    ...overrides,
  };
  render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <ImmutableWikiRevision {...props} />
    </I18nProvider>,
  );
  return props;
}

describe("ImmutableWikiRevision", () => {
  it("renders exact read-only provenance and copies the canonical revision identity", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    renderRevision();

    expect(screen.getByText("Read only")).toBeInTheDocument();
    expect(screen.getByText("sha256:exact-content-digest")).toBeInTheDocument();
    expect(screen.getByText("human by member")).toBeInTheDocument();
    expect(document.querySelector("time")).toHaveAttribute("datetime", "2026-08-23T11:00:00Z");
    expect(screen.getByRole("heading", { name: revision.title })).toHaveClass("break-words");
    expect(screen.getByRole("link", { name: "Open source issue" })).toHaveAttribute(
      "href",
      expect.stringMatching(/\/issues\/MUL-1$/),
    );
    fireEvent.click(screen.getByTitle("Copy revision citation"));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("wiki_page_revision:revision-2"));
  });

  it("renders a recoverable error state for malformed or inaccessible revisions", () => {
    const props = renderRevision({ revision: undefined, isError: true });
    expect(screen.getByRole("alert")).toHaveTextContent("could not be loaded");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(props.onRetry).toHaveBeenCalledOnce();
  });

  it("labels personal immutable snapshots separately from shared evidence", () => {
    renderRevision({ personal: true, citationPrefix: "personal_wiki_revision" });
    expect(screen.getByText("Private evidence")).toBeInTheDocument();
    expect(screen.getByText("personal_wiki_revision:revision-2")).toBeInTheDocument();
  });
});
