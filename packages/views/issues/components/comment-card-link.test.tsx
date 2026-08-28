import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import type { TimelineEntry } from "@multica/core/types";
import { useCommentCollapseStore } from "@multica/core/issues/stores";
import { renderWithI18n } from "../../test/i18n";

const copyText = vi.hoisted(() => vi.fn());
const toastSuccess = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
const replyInputProps = vi.hoisted(() => ({
  insertRequest: undefined as { id: number; markdown: string } | undefined,
}));

vi.mock("@multica/ui/lib/clipboard", () => ({ copyText }));
vi.mock("sonner", () => ({
  toast: { success: toastSuccess, error: toastError },
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    getShareableUrl: (path: string) => `https://app.example${path}`,
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Ada" }),
}));

vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("@multica/ui/components/common/reaction-bar", () => ({
  ReactionBar: () => null,
}));
vi.mock("./reply-input", () => ({
  ReplyInput: ({ insertRequest }: { insertRequest?: { id: number; markdown: string } }) => {
    replyInputProps.insertRequest = insertRequest;
    return null;
  },
}));
vi.mock("./comment-trigger-chips", () => ({ CommentTriggerChips: () => null }));
vi.mock("../hooks/use-comment-trigger-preview", () => ({
  useCommentTriggerPreview: () => ({ agents: [], blocked: [] }),
}));
vi.mock("../../editor", () => ({
  ContentEditor: () => null,
  ReadonlyContent: ({ content }: { content: string }) => <div>{content}</div>,
  useFileDropZone: () => ({ isDragOver: false, dropZoneProps: {} }),
  FileDropOverlay: () => null,
  Attachment: () => null,
  AttachmentDownloadProvider: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  useUploadGate: () => ({
    uploading: false,
    onUploadingChange: vi.fn(),
    runWhenReady: vi.fn(),
  }),
  useComposerSubmit: () => ({ submitting: false, submit: vi.fn() }),
}));
vi.mock("./use-comment-uploads", () => ({
  useCommentUploads: () => ({
    uploads: [],
    attachments: [],
    handleUpload: vi.fn(),
    removeUpload: vi.fn(),
    gate: { uploading: false, onUploadingChange: vi.fn() },
  }),
}));

import { CommentCard, formatCommentQuote } from "./comment-card";

function comment(
  id: string,
  parentId: string | null,
  content: string,
): TimelineEntry {
  return {
    id,
    parent_id: parentId,
    actor_type: "member",
    actor_id: "user-1",
    content,
    type: "comment",
    created_at: "2026-08-15T08:00:00Z",
    updated_at: "2026-08-15T08:00:00Z",
    attachments: [],
    reactions: [],
  };
}

function renderCard(targetCommentId?: string) {
  renderWithI18n(
    <CommentCard
      issueId="issue-1"
      issueHref="/acme/issues/MUL-1"
      entry={comment("comment-1", null, "Root comment")}
      replies={[comment("reply-1", "comment-1", "Nested reply")]}
      currentUserId="user-1"
      onReply={vi.fn().mockResolvedValue(true)}
      onEdit={vi.fn().mockResolvedValue(undefined)}
      onDelete={vi.fn()}
      onToggleReaction={vi.fn()}
      targetCommentId={targetCommentId}
    />,
  );
}

async function copyLinkFromMenu(triggerIndex: number) {
  const triggers = document.querySelectorAll('button[aria-haspopup="menu"]');
  const trigger = triggers.item(triggerIndex);
  if (!(trigger instanceof HTMLButtonElement)) {
    throw new Error(`Expected comment menu trigger ${triggerIndex}`);
  }
  fireEvent.click(trigger);
  fireEvent.click(await screen.findByText("Copy comment link"));
}

beforeEach(() => {
  vi.clearAllMocks();
  copyText.mockResolvedValue(true);
  useCommentCollapseStore.setState({ collapsedByIssue: {} });
  replyInputProps.insertRequest = undefined;
});

describe("comment quote reply actions", () => {
  it("formats multiline Markdown as one block quote", () => {
    expect(formatCommentQuote(" First line\r\n\r\nSecond line ")).toBe(
      ">  First line\n>\n> Second line ",
    );
    expect(formatCommentQuote("  \n ")).toBe("");
    expect(formatCommentQuote("\n    const answer = 42;\n")).toBe(
      ">     const answer = 42;",
    );
  });

  it("quotes a root comment into its reply composer", async () => {
    renderCard();

    const trigger = document.querySelectorAll('button[aria-haspopup="menu"]').item(0);
    fireEvent.click(trigger);
    fireEvent.click(await screen.findByText("Quote in reply"));

    expect(replyInputProps.insertRequest).toEqual({ id: 1, markdown: "> Root comment" });
  });

  it("quotes a nested reply into the same reply composer", async () => {
    renderCard();

    const trigger = document.querySelectorAll('button[aria-haspopup="menu"]').item(1);
    fireEvent.click(trigger);
    fireEvent.click(await screen.findByText("Quote in reply"));

    expect(replyInputProps.insertRequest).toEqual({ id: 1, markdown: "> Nested reply" });
  });
});

describe("comment permalink actions", () => {
  it("copies the root comment's shareable URL", async () => {
    renderCard();

    await copyLinkFromMenu(0);

    expect(copyText).toHaveBeenCalledWith(
      "https://app.example/acme/issues/MUL-1?comment=comment-1",
    );
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("copies a nested reply's shareable URL", async () => {
    renderCard();

    await copyLinkFromMenu(1);

    expect(copyText).toHaveBeenCalledWith(
      "https://app.example/acme/issues/MUL-1?comment=reply-1",
    );
    expect(toastSuccess).toHaveBeenCalledOnce();
  });

  it("shows an error when copying a comment link fails", async () => {
    copyText.mockResolvedValue(false);
    renderCard();

    await copyLinkFromMenu(0);

    await waitFor(() => expect(toastError).toHaveBeenCalledOnce());
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("renders a permalinked root comment inside a manually collapsed thread", () => {
    useCommentCollapseStore.setState({
      collapsedByIssue: { "issue-1": ["comment-1"] },
    });

    renderCard("comment-1");

    expect(document.getElementById("comment-body-comment-1")).not.toBeNull();
  });

  it("renders a permalinked reply inside a manually collapsed thread", () => {
    useCommentCollapseStore.setState({
      collapsedByIssue: { "issue-1": ["comment-1"] },
    });

    renderCard("reply-1");

    expect(screen.getByText("Nested reply")).toBeVisible();
  });
});
