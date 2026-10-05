// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { EMPTY_ROOM_DETAIL, EMPTY_ROOM_USAGE } from "@multica/core/rooms";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enCommon from "../locales/en/common.json";
import enRooms from "../locales/en/rooms.json";
import { RoomInspector } from "./room-inspector";

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/issues/${id}`,
    wikiPage: (id: string) => `/wiki/${id}`,
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Actor" }),
}));

const NOW = new Date("2026-08-31T12:00:00.000Z");

function renderInspector(status: "active" | "paused" | "archived") {
  const detail = {
    ...EMPTY_ROOM_DETAIL,
    room: {
      ...EMPTY_ROOM_DETAIL.room,
      status,
      next_wake_at: "2026-08-31T12:05:00.000Z",
    },
  };

  return render(
    <I18nProvider locale="en" resources={{ en: { common: enCommon, rooms: enRooms } }}>
      <RoomInspector detail={detail} usage={EMPTY_ROOM_USAGE} onPromoteCycle={vi.fn()} />
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("RoomInspector schedule", () => {
  it("shows the active room's next wake with future relative-time copy", () => {
    const view = renderInspector("active");

    expect(view.getByText("Next run in 5m")).toBeInTheDocument();
  });

  it.each(["paused", "archived"] as const)(
    "does not show a next wake for a %s room",
    (status) => {
      const view = renderInspector(status);

      expect(view.queryByText("Next run in 5m")).not.toBeInTheDocument();
      expect(view.getByText("Disabled")).toBeInTheDocument();
    },
  );
});
