// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WikiRevision } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";
import { WikiHistoryDialog } from "./wiki-history-dialog";

const revisions: WikiRevision[] = [
  {
    id: "revision-2", pageId: "page-1", revisionNumber: 2, path: "guide.md", title: "Guide",
    content: "current", contentDigest: "sha256:2", actorType: "member", actorId: "member-1",
    sourceKind: "human", sourceRefId: null, createdAt: "2026-08-23T11:00:00Z",
  },
  {
    id: "revision-1", pageId: "page-1", revisionNumber: 1, path: "guide.md", title: "Guide",
    content: "old", contentDigest: "sha256:1", actorType: "system", actorId: null,
    sourceKind: "room_promotion", sourceRefId: "room-1", createdAt: "2026-08-23T10:00:00Z",
  },
];

function renderHistory(onRestore = vi.fn()) {
  render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <WikiHistoryDialog
        open
        onOpenChange={vi.fn()}
        revisions={revisions}
        currentRevisionNumber={2}
        isLoading={false}
        isError={false}
        isRestoring={false}
        onRetry={vi.fn()}
        onRestore={onRestore}
      />
    </I18nProvider>,
  );
  return onRestore;
}

describe("WikiHistoryDialog", () => {
  it("compares immutable content and restores an older revision as a new write", () => {
    const onRestore = renderHistory();
    expect(screen.getByRole("dialog")).toHaveAttribute("data-wiki-interaction-region");
    expect(screen.getByRole("dialog")).toHaveClass(
      "grid-cols-[minmax(0,1fr)]",
      "max-lg:[&_button]:min-h-11",
    );
    expect(screen.getByText("current")).toBeInTheDocument();
    expect(screen.getByText("old")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: enWiki.history.timeline })).toBeInTheDocument();
    expect(screen.getByText("sha256:2")).toHaveClass("break-all");
    expect(screen.getByText("sha256:1")).toHaveClass("break-all");
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    expect(screen.getByRole("alertdialog")).toHaveAttribute("data-wiki-interaction-region");
    expect(screen.getByRole("alertdialog")).toHaveClass("max-lg:[&_button]:min-h-11");
    expect(screen.getByRole("alertdialog")).toHaveTextContent("Revision 1 will become a new revision");
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    expect(onRestore).toHaveBeenCalledWith("revision-1");
  }, 15_000);

  it("keeps restore failures visible inside the history dialog", () => {
    render(
      <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
        <WikiHistoryDialog
          open
          onOpenChange={vi.fn()}
          revisions={revisions}
          currentRevisionNumber={2}
          isLoading={false}
          isError={false}
          isRestoring={false}
          actionError="The page changed before restore."
          onRetry={vi.fn()}
          onRestore={vi.fn()}
        />
      </I18nProvider>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("The page changed before restore.");
  });
});
