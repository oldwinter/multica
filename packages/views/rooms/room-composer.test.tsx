// @vitest-environment jsdom

import { useState } from "react";
import { cleanup, fireEvent, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { RoomStatus } from "@multica/core/rooms";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { RoomComposer } from "./room-composer";

const TEST_RESOURCES = { en: { rooms: enRooms } };

afterEach(cleanup);

function renderComposer({
  initialBody = "",
  roomStatus = "active",
  showStarters = true,
}: {
  readonly initialBody?: string;
  readonly roomStatus?: RoomStatus;
  readonly showStarters?: boolean;
} = {}) {
  const onPost = vi.fn();

  function ComposerHarness() {
    const [body, setBody] = useState(initialBody);

    return (
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <RoomComposer
          roomStatus={roomStatus}
          participants={[]}
          agents={[]}
          draft={{
            body,
            mentionAgentIds: [],
            idempotencyKey: "draft-key",
            status: "idle",
          }}
          showStarters={showStarters && body.length === 0}
          onBodyChange={setBody}
          onMentionChange={vi.fn()}
          onPost={onPost}
        />
      </I18nProvider>
    );
  }

  return { ...render(<ComposerHarness />), onPost };
}

describe("RoomComposer starters", () => {
  it("fills and focuses the draft without posting when a starter is picked", () => {
    const view = renderComposer();

    expect(view.getAllByTestId(/^room-starter-/)).toHaveLength(3);
    fireEvent.click(view.getByTestId("room-starter-unblock"));

    const input = view.getByTestId("room-message-input");
    expect(input).not.toHaveValue("");
    expect(input).toHaveFocus();
    expect(view.onPost).not.toHaveBeenCalled();
    expect(view.queryByTestId("room-starter-unblock")).not.toBeInTheDocument();
  });

  it("stays out of the way when a draft exists or the room is archived", () => {
    const drafted = renderComposer({ initialBody: "Existing draft" });
    expect(drafted.queryByTestId("room-starter-unblock")).not.toBeInTheDocument();
    drafted.unmount();

    const whitespaceDraft = renderComposer({ initialBody: " " });
    expect(whitespaceDraft.queryByTestId("room-starter-unblock")).not.toBeInTheDocument();
    whitespaceDraft.unmount();

    const archived = renderComposer({ roomStatus: "archived" });
    expect(archived.queryByTestId("room-starter-unblock")).not.toBeInTheDocument();
  });
});
