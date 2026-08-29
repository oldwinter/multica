// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WikiKnowledgeReadiness } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";
import { WikiKnowledgeMaintenanceQueue } from "./wiki-knowledge-maintenance-queue";

const readiness: WikiKnowledgeReadiness = {
  schemaVersion: 1,
  policy: {
    sourceClasses: ["wiki_page"],
    wikiPages: [{ pageId: "page-1", revisionNumber: 2 }],
    remoteGenerationEnabled: false,
    policyVersion: 4,
    policyDigest: "sha256:policy-4",
    exclusions: [],
  },
  sources: [],
  maintenanceItems: [{
    id: "source_newer_revision:page-1:2:4",
    kind: "source_newer_revision",
    severity: "warning",
    reasonCode: "wiki_source_newer_revision_available",
    responsibleRole: "owner_admin",
    pageId: "page-1",
    selectedRevisionNumber: 2,
    policyVersion: 4,
    nextAction: { kind: "pin_revision", pageId: "page-1", revisionId: "revision-3", revisionNumber: 3 },
  }],
  truncated: false,
  canManage: true,
};

describe("WikiKnowledgeMaintenanceQueue", () => {
  it("renders the server-owned item and dispatches only its declared next action", () => {
    const onAction = vi.fn();
    render(
      <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
        <WikiKnowledgeMaintenanceQueue
          readiness={readiness}
          pages={[]}
          isLoading={false}
          isError={false}
          isPending={false}
          onRetry={vi.fn()}
          onAction={onAction}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("A pinned source has a newer revision")).toBeInTheDocument();
    expect(screen.getByText("Source policy version 4")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Review newer revision" }));
    expect(onAction).toHaveBeenCalledWith(readiness.maintenanceItems[0]!.nextAction);
  });

  it("keeps domain actions read-only for non-managers", () => {
    render(
      <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
        <WikiKnowledgeMaintenanceQueue
          readiness={{ ...readiness, canManage: false }}
          pages={[]}
          isLoading={false}
          isError={false}
          isPending={false}
          onRetry={vi.fn()}
          onAction={vi.fn()}
        />
      </I18nProvider>,
    );
    expect(screen.getByRole("button", { name: "Review newer revision" })).toBeDisabled();
  });
});
