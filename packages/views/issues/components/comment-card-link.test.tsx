import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { TimelineEntry } from "@multica/core/types";
import { useCommentCollapseStore } from "@multica/core/issues/stores";
import { renderWithI18n } from "../../test/i18n";

const replyInputProps = vi.hoisted(() => ({
  insertRequest: undefined as { id: number; markdown: string } | undefined,
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <CommentCard
        issueId="issue-1"
        entry={comment("comment-1", null, "Root comment")}
        replies={[comment("reply-1", "comment-1", "Nested reply")]}
        currentUserId="user-1"
        onReply={vi.fn().mockResolvedValue(true)}
        onEdit={vi.fn().mockResolvedValue(undefined)}
        onDelete={vi.fn()}
        onToggleReaction={vi.fn()}
        targetCommentId={targetCommentId}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
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
