// @vitest-environment jsdom

import { cleanup, fireEvent, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { PromoteRoomDialog } from "./promote-room-dialog";

const TEST_RESOURCES = { en: { rooms: enRooms } };

afterEach(cleanup);

describe("PromoteRoomDialog idempotency", () => {
  it("reuses the same key when a network result is unknown", () => {
    const onPromote = vi.fn();
    const view = render(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <PromoteRoomDialog
          source={{
            memoryRevisionId: "revision-3",
            recommendationKey: "recommendation-1",
            citationEntryIds: ["entry-1"],
            suggestedTitle: "Record decision",
            suggestedBody: "Use a staged release.",
          }}
          pending={false}
          onOpenChange={vi.fn()}
          onPromote={onPromote}
        />
      </I18nProvider>,
    );

    const submit = view.getByTestId("room-promote-submit");
    expect(view.getByTestId("room-promotion-kind")).toBeDisabled();
    fireEvent.click(submit);
    fireEvent.click(submit);

    expect(onPromote).toHaveBeenCalledTimes(2);
    expect(onPromote.mock.calls[1]?.[0].idempotency_key)
      .toBe(onPromote.mock.calls[0]?.[0].idempotency_key);
    expect(onPromote.mock.calls[0]?.[0]).toMatchObject({
      memory_revision_id: "revision-3",
      recommendation_key: "recommendation-1",
      citation_entry_ids: ["entry-1"],
    });
    expect(onPromote.mock.calls[0]?.[0]).not.toHaveProperty("entry_id");
    expect(onPromote.mock.calls[0]?.[0]).not.toHaveProperty("cycle_id");
  });
});
