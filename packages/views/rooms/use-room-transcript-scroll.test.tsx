// @vitest-environment jsdom

import { cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useRoomTranscriptScroll } from "./use-room-transcript-scroll";

afterEach(cleanup);

function TranscriptHarness({
  roomId,
  entryCount,
}: {
  readonly roomId: string;
  readonly entryCount: number;
}) {
  const { scrollRef, onScroll, unseenEntryCount, scrollToLatest } =
    useRoomTranscriptScroll(roomId, entryCount);

  return (
    <>
      <div ref={scrollRef} onScroll={onScroll} data-testid="transcript" tabIndex={-1} />
      <button type="button" onClick={scrollToLatest}>
        {unseenEntryCount}
      </button>
    </>
  );
}

function renderTranscript(roomId = "room-a", entryCount = 2) {
  Object.defineProperties(HTMLElement.prototype, {
    clientHeight: { configurable: true, get: () => 200 },
    scrollHeight: { configurable: true, get: () => 1000 },
  });

  return render(<TranscriptHarness roomId={roomId} entryCount={entryCount} />);
}

describe("useRoomTranscriptScroll", () => {
  it("shows the latest entry when a room first opens", () => {
    const view = renderTranscript();

    expect(view.getByTestId("transcript").scrollTop).toBe(800);
  });

  it("follows new entries only while the reader is near the end", () => {
    const view = renderTranscript();
    const transcript = view.getByTestId("transcript");

    transcript.scrollTop = 100;
    fireEvent.scroll(transcript);
    view.rerender(<TranscriptHarness roomId="room-a" entryCount={3} />);
    expect(transcript.scrollTop).toBe(100);

    transcript.scrollTop = 790;
    fireEvent.scroll(transcript);
    view.rerender(<TranscriptHarness roomId="room-a" entryCount={4} />);
    expect(transcript.scrollTop).toBe(800);
  });

  it("reports unseen entries while the reader is away and returns to latest", () => {
    const view = renderTranscript();
    const transcript = view.getByTestId("transcript");

    transcript.scrollTop = 100;
    fireEvent.scroll(transcript);
    view.rerender(<TranscriptHarness roomId="room-a" entryCount={4} />);
    expect(view.getByRole("button")).toHaveTextContent("2");

    fireEvent.click(view.getByRole("button"));
    expect(transcript.scrollTop).toBe(800);
    expect(transcript).toHaveFocus();
    expect(view.getByRole("button")).toHaveTextContent("0");
  });

  it("resets to the latest entry when switching rooms", () => {
    const view = renderTranscript();
    const transcript = view.getByTestId("transcript");

    transcript.scrollTop = 100;
    fireEvent.scroll(transcript);
    view.rerender(<TranscriptHarness roomId="room-b" entryCount={2} />);

    expect(transcript.scrollTop).toBe(800);
  });
});
