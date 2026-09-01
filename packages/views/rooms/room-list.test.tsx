// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { EMPTY_ROOM, type Room } from "@multica/core/rooms";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enCommon from "../locales/en/common.json";
import enRooms from "../locales/en/rooms.json";
import { RoomList } from "./room-list";

const NOW = new Date("2026-08-31T12:00:00.000Z");

function makeRoom(overrides: Partial<Room> = {}): Room {
  return {
    ...EMPTY_ROOM,
    id: "room-1",
    title: "Research council",
    objective: "Keep the answer current",
    status: "active",
    updated_at: NOW.toISOString(),
    ...overrides,
  };
}

function renderRoomList(
  rooms: readonly Room[],
  selectedId: string,
  options: { readonly mobileStandalone?: boolean } = {},
) {
  const onSelect = vi.fn();
  const view = render(
    <I18nProvider locale="en" resources={{ en: { common: enCommon, rooms: enRooms } }}>
      <RoomList
        rooms={rooms}
        selectedId={selectedId}
        loading={false}
        showValueReview={false}
        mobileStandalone={options.mobileStandalone ?? false}
        onSelect={onSelect}
        onCreate={vi.fn()}
      />
    </I18nProvider>,
  );
  return { ...view, onSelect };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("RoomList", () => {
  it("owns the stacked mobile workspace when the Room directory is empty", () => {
    const view = renderRoomList([], "", { mobileStandalone: true });

    expect(view.getByTestId("room-list")).toHaveClass(
      "max-lg:row-span-2",
      "max-lg:max-h-none",
    );
    expect(view.getByTestId("room-list-scroll")).toHaveClass(
      "max-lg:flex",
      "max-lg:flex-1",
      "max-lg:items-center",
    );
    expect(view.getByTestId("room-create-open")).toHaveClass("max-lg:size-11");
    expect(view.getAllByRole("button", { name: /new room/i })[1]).toHaveClass(
      "max-lg:min-h-11",
    );
  });

  it("marks the selected room as the current list item", () => {
    const view = renderRoomList(
      [makeRoom(), makeRoom({ id: "room-2", title: "Planning council" })],
      "room-2",
    );

    expect(view.getByTestId("room-list-item-room-2")).toHaveAttribute(
      "aria-current",
      "true",
    );
    expect(view.getByTestId("room-list-item-room-1")).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("shows an active room's next wake as a future relative time", () => {
    const view = renderRoomList(
      [makeRoom({ next_wake_at: "2026-08-31T12:05:00.000Z" })],
      "room-1",
    );

    expect(view.getByTestId("room-list-item-room-1")).toHaveTextContent("Next in 5m");
    expect(view.getByTestId("room-list-item-room-1")).not.toHaveTextContent("Next just now");
    expect(view.getByRole("searchbox", { name: "Search Rooms" })).toHaveClass(
      "max-lg:h-11",
    );
  });

  it.each(["paused", "archived"] as const)(
    "does not show an imminent next wake for a %s scheduled room",
    (status) => {
      const view = renderRoomList(
        [makeRoom({ status, next_wake_at: "2026-08-31T12:05:00.000Z" })],
        "room-1",
      );

      expect(view.getByTestId("room-list-item-room-1")).not.toHaveTextContent("Next");
    },
  );
});
