// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { deriveRoomOutcomeState, EMPTY_ROOM_DETAIL } from "@multica/core/rooms";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { RoomDetail } from "./room-detail";

vi.mock("./room-transcript", () => ({ RoomTranscript: () => <div /> }));
vi.mock("./room-outcome", () => ({ RoomOutcome: () => <div /> }));
vi.mock("./room-inspector", () => ({ RoomInspector: () => <div /> }));

afterEach(cleanup);

describe("RoomDetail responsive controls", () => {
  it("uses the stacked layout breakpoint for tablet-size touch targets", () => {
    const detail = {
      ...EMPTY_ROOM_DETAIL,
      room: {
        ...EMPTY_ROOM_DETAIL.room,
        id: "room-1",
        title: "Release review",
        objective: "Decide whether to ship",
        status: "active" as const,
      },
    };
    const view = render(
      <I18nProvider locale="en" resources={{ en: { rooms: enRooms } }}>
        <RoomDetail
          detail={detail}
          detailTab="transcript"
          agents={[]}
          draft={{ body: "", mentionAgentIds: [], status: "idle", idempotencyKey: "draft-1" }}
          outcomeState={deriveRoomOutcomeState(detail)}
          preflight={undefined}
          scheduledPreflight={undefined}
          onDraftBodyChange={vi.fn()}
          onDraftMentionChange={vi.fn()}
          onDetailTabChange={vi.fn()}
          waking={false}
          preflightPending={false}
          preflightError={false}
          statusPending={false}
          reviewPending={false}
          retryPending={false}
          cancelPending={false}
          recommendationPending={false}
          canManageBudget
          onPost={vi.fn()}
          onWake={vi.fn()}
          onRetryPreflight={vi.fn()}
          onStatus={vi.fn()}
          onReview={vi.fn()}
          onRetrySynthesis={vi.fn()}
          onCancelCycle={vi.fn()}
          onRejectRecommendation={vi.fn()}
          onPromote={vi.fn()}
          onDuplicate={vi.fn()}
          onManageBudget={vi.fn()}
        />
      </I18nProvider>,
    );

    const titleBlock = view.getByRole("heading", { name: "Release review" }).parentElement?.parentElement;
    expect(titleBlock).toHaveClass("max-lg:basis-full");
    expect(view.getByTestId("room-status-toggle")).toHaveClass(
      "max-lg:min-h-11",
      "max-lg:min-w-11",
    );
    for (const testId of ["room-duplicate", "room-copy-link", "room-manage-budget", "room-archive"]) {
      expect(view.getByTestId(testId)).toHaveClass("max-lg:size-11");
    }
    expect(view.getByTestId("room-wake")).toHaveClass(
      "max-lg:min-h-11",
      "max-lg:min-w-11",
    );
    for (const tab of view.getAllByRole("tab")) {
      expect(tab).toHaveClass("max-lg:min-h-11");
    }
    expect(view.getAllByRole("tab")[0]?.closest('[data-slot="tabs-list"]')).toHaveClass(
      "max-lg:group-data-horizontal/tabs:h-11",
    );
  });
});
