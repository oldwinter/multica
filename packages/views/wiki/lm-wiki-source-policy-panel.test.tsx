// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WikiPageSummary } from "@multica/core/wiki";
import enWiki from "../locales/en/wiki.json";
import { LMWikiSourcePolicyPanel } from "./lm-wiki-source-policy-panel";

const workspacePage: WikiPageSummary = {
  id: "page-1",
  workspaceId: "ws-1",
  scope: "workspace",
  projectId: null,
  ownerUserId: null,
  path: "handbook.md",
  title: "Handbook",
  createdBy: "member-1",
  currentRevisionNumber: 5,
  currentRevisionId: "revision-5",
  contentDigest: "sha256:current",
  lastSourceKind: "human",
  lastActorType: "member",
  lastActorId: "member-1",
  createdAt: "2026-08-23T10:00:00Z",
  updatedAt: "2026-08-23T11:00:00Z",
};

const personalPage: WikiPageSummary = {
  ...workspacePage,
  id: "page-personal",
  workspaceId: null,
  scope: "user",
  ownerUserId: "member-1",
  path: "private.md",
  title: "Private notes",
};

function renderPolicy(props: Partial<React.ComponentProps<typeof LMWikiSourcePolicyPanel>> = {}) {
  const onSave = props.onSave ?? vi.fn();
  render(
    <I18nProvider locale="en" resources={{ en: { wiki: enWiki } }}>
      <LMWikiSourcePolicyPanel
        policy={{
          sourceClasses: ["issue", "wiki_page"],
          wikiPages: [{ pageId: "page-1", revisionNumber: 2 }],
          remoteGenerationEnabled: false,
          policyVersion: 3,
          policyDigest: "sha256:policy",
          exclusions: [
            {
              sourceClass: "personal_wiki",
              state: "always_excluded",
              reason: "personal_scope_never_eligible",
            },
            {
              sourceClass: "local_only",
              state: "always_excluded",
              reason: "local_only_never_leaves_owner_daemon",
            },
          ],
        }}
        pages={[workspacePage, personalPage]}
        canManage
        canManageRemoteGeneration
        onSave={onSave}
        {...props}
      />
    </I18nProvider>,
  );
  return { onSave };
}

describe("LMWikiSourcePolicyPanel", () => {
  it("keeps a pinned historical revision selectable before history has loaded", () => {
    renderPolicy({ revisionsByPage: {} });
    expect(screen.getByRole("combobox", { name: "Revision 2" })).toHaveTextContent("Revision 2");
  }, 15_000);

  it("disables page selection and omits pinned pages when wiki_page is disabled", () => {
    const { onSave } = renderPolicy();
    fireEvent.click(screen.getByRole("switch", { name: "Pinned Wiki pages" }));
    expect(screen.getByRole("checkbox", { name: "Handbook" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("button", { name: "Save source policy" }));
    expect(onSave).toHaveBeenCalledWith({
      sourceClasses: ["issue"],
      wikiPages: [],
      remoteGenerationEnabled: false,
      expectedPolicyVersion: 3,
      expectedPolicyDigest: "sha256:policy",
    });
  }, 15_000);

  it("shows personal pages as excluded and never enables them", () => {
    renderPolicy();
    expect(screen.getByText("Personal pages are private and cannot be LM Wiki evidence.")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Private notes" })).toHaveAttribute("aria-disabled", "true");
  }, 15_000);

  it("keeps every policy control read-only for a non-manager", () => {
    renderPolicy({ canManage: false, canManageRemoteGeneration: false });
    expect(screen.getByText("Only workspace owners and admins can change this policy.")).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Issues" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("checkbox", { name: "Handbook" })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("button", { name: "Save source policy" })).toBeDisabled();
  }, 15_000);

  it("requests revision history after an eligible page is selected", () => {
    const onPageSelectionChange = vi.fn();
    renderPolicy({
      policy: {
        sourceClasses: ["wiki_page"],
        wikiPages: [],
        remoteGenerationEnabled: false,
        policyVersion: 3,
        policyDigest: "sha256:policy",
        exclusions: [],
      },
      onPageSelectionChange,
    });

    fireEvent.click(screen.getByRole("checkbox", { name: "Handbook" }));
    expect(onPageSelectionChange).toHaveBeenCalledWith("page-1", true);
  }, 15_000);

  it("keeps remote generation off by default and lets an owner save the opt-in", () => {
    const { onSave } = renderPolicy();
    const remoteSwitch = screen.getByRole("switch", { name: "Allow remote generation" });
    expect(remoteSwitch).toHaveAttribute("aria-checked", "false");
    fireEvent.click(remoteSwitch);
    fireEvent.click(screen.getByRole("button", { name: "Save source policy" }));
    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ remoteGenerationEnabled: true }));
  }, 15_000);

  it("keeps remote generation manager-only and protects narrow long-copy layouts", () => {
    renderPolicy({ canManageRemoteGeneration: false });
    const remoteSwitch = screen.getByRole("switch", { name: "Allow remote generation" });
    expect(remoteSwitch).toHaveAttribute("aria-disabled", "true");
    expect(remoteSwitch).toHaveClass("shrink-0");
    const exclusions = screen.getByText("Local-only sources and personal Wiki pages are always excluded.");
    expect(exclusions).toHaveClass("break-words");
    expect(exclusions.parentElement).toHaveClass("min-w-0");
    expect(screen.getByText("Only workspace owners and admins can change remote generation.")).toBeInTheDocument();
    expect(screen.getAllByText("Always excluded")).toHaveLength(2);
    expect(screen.getByText("Personal Wiki pages")).toBeInTheDocument();
    expect(screen.getByText("Local-only sources")).toBeInTheDocument();
  }, 15_000);
});
