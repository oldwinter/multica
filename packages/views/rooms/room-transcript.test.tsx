// @vitest-environment jsdom

import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { EMPTY_ROOM_DETAIL, type RoomDetail } from "@multica/core/rooms";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { RoomTranscript } from "./room-transcript";

const { copyTextMock, toastErrorMock, toastSuccessMock } = vi.hoisted(() => ({
  copyTextMock: vi.fn(),
  toastErrorMock: vi.fn(),
  toastSuccessMock: vi.fn(),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Facilitator Agent" }),
}));

vi.mock("@multica/ui/lib/clipboard", () => ({
  copyText: copyTextMock,
}));

vi.mock("sonner", () => ({
  toast: { error: toastErrorMock, success: toastSuccessMock },
}));

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

vi.mock("./room-composer", () => ({
  RoomComposer: () => <div data-testid="room-composer" />,
}));

vi.mock("./use-room-transcript-scroll", () => ({
  useRoomTranscriptScroll: () => ({
    scrollRef: { current: null },
    onScroll: vi.fn(),
    unseenEntryCount: 0,
    scrollToLatest: vi.fn(),
  }),
}));

const TEST_RESOURCES = { en: { rooms: enRooms } };

const detail: RoomDetail = {
  ...EMPTY_ROOM_DETAIL,
  room: {
    ...EMPTY_ROOM_DETAIL.room,
    id: "room-1",
    title: "Release review",
  },
  entries: [
    {
      id: "entry-message",
      cycle_id: null,
      turn_id: null,
      ordinal: 1,
      type: "message",
      author_type: "member",
      author_id: "member-1",
      body: "Keep **this Markdown** intact.",
      mentions: [],
      created_at: "2026-08-30T00:00:00Z",
    },
    {
      id: "entry-result",
      cycle_id: "cycle-1",
      turn_id: "turn-1",
      ordinal: 2,
      type: "result",
      author_type: "agent",
      author_id: "agent-1",
      body: "The canary passed.",
      mentions: [],
      created_at: "2026-08-30T00:01:00Z",
    },
  ],
};

function renderTranscript() {
  const onPromoteEntry = vi.fn();
  const view = render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <RoomTranscript
        detail={detail}
        agents={[]}
        draft={{ body: "", mentionAgentIds: [], status: "idle", idempotencyKey: "draft-1" }}
        onBodyChange={vi.fn()}
        onMentionChange={vi.fn()}
        onPost={vi.fn()}
        onPromoteEntry={onPromoteEntry}
      />
    </I18nProvider>,
  );
  return { ...view, onPromoteEntry };
}

beforeEach(() => {
  vi.clearAllMocks();
  copyTextMock.mockResolvedValue(true);
});

afterEach(cleanup);

describe("RoomTranscript copy actions", () => {
  it("copies the exact Markdown body and confirms success", async () => {
    const view = renderTranscript();

    const copyButton = view.getByTestId("room-copy-entry-entry-message");
    fireEvent.click(copyButton);

    expect(copyTextMock).toHaveBeenCalledWith("Keep **this Markdown** intact.");
    await waitFor(() => {
      expect(toastSuccessMock).toHaveBeenCalledWith("Message copied");
      expect(copyButton.getAttribute("aria-label")).toBe("Message copied");
      expect(copyButton.getAttribute("data-copy-status")).toBe("copied");
    });
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it("reports a clipboard failure and keeps result promotion available", async () => {
    copyTextMock.mockResolvedValue(false);
    const view = renderTranscript();

    const copyButton = view.getByTestId("room-copy-entry-entry-result");
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(toastErrorMock).toHaveBeenCalledWith("Message could not be copied");
      expect(copyButton.getAttribute("aria-label")).toBe("Message could not be copied");
      expect(copyButton.getAttribute("data-copy-status")).toBe("failed");
    });
    fireEvent.click(view.getByTestId("room-promote-entry-entry-result"));
    expect(view.onPromoteEntry).toHaveBeenCalledWith("entry-result", "Release review");
    expect(toastSuccessMock).not.toHaveBeenCalled();
  });
});
