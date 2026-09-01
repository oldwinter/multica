// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { EMPTY_ROOM } from "@multica/core/rooms";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { RoomBudgetDialog } from "./room-budget-dialog";

afterEach(cleanup);

describe("RoomBudgetDialog", () => {
  it("keeps budget inputs and actions at least 44px tall on mobile", () => {
    const view = render(
      <I18nProvider locale="en" resources={{ en: { rooms: enRooms } }}>
        <RoomBudgetDialog
          open
          room={{ ...EMPTY_ROOM, id: "room-1", daily_turn_limit: 12 }}
          pending={false}
          onOpenChange={vi.fn()}
          onSave={vi.fn()}
        />
      </I18nProvider>,
    );

    expect(view.getByRole("dialog")).toHaveClass(
      "max-h-[calc(100dvh-2rem)]",
      "overflow-y-auto",
      "max-lg:[&_[data-slot=dialog-close]]:size-11",
    );
    for (const input of view.getAllByRole("spinbutton")) {
      expect(input).toHaveClass("max-lg:h-11");
    }
    expect(view.getByRole("button", { name: "Cancel" })).toHaveClass("max-lg:h-11");
    expect(view.getByRole("button", { name: "Save budget" })).toHaveClass("max-lg:h-11");
  });
});
