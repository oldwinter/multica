// @vitest-environment jsdom

import { cleanup, fireEvent, render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import {
  deriveRoomOutcomeState,
  EMPTY_ROOM_DETAIL,
  type RoomCycle,
  type RoomMemoryRevision,
  type RoomSynthesis,
} from "@multica/core/rooms";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { RoomOutcome } from "./room-outcome";

const TEST_RESOURCES = { en: { rooms: enRooms } };

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Facilitator Agent" }),
}));

afterEach(cleanup);

const synthesis: RoomSynthesis = {
  schema_version: 1,
  summary: "Ship after the canary passes.",
  facts: [{ text: "The canary is healthy.", citation_entry_ids: ["entry-1"], confidence: 0.9 }],
  decisions: [{ text: "Use a staged release.", citation_entry_ids: ["entry-1"], confidence: 0.8 }],
  open_questions: [],
  disagreements: [],
  action_items: [],
  recommendations: [{
    key: "recommendation-1",
    kind: "decision",
    title: "Record the release decision",
    body: "Use a staged release.",
    rationale: "The evidence supports a canary.",
    citation_entry_ids: ["entry-1"],
    confidence: 0.8,
  }],
  confidence: 0.84,
};

const roomCycle = (completed: boolean): RoomCycle => ({
  id: "cycle-3",
  sequence: 3,
  source: "manual",
  wake_key: "wake-3",
  triggering_entry_id: null,
  status: completed ? "completed" : "running",
  phase: completed ? "completed" : "awaiting_review",
  refusal_reason: null,
  synthesis_error: null,
  synthesis_turn_id: "turn-synthesis",
  memory_revision_id: "revision-3",
  expected_max_turns: 3,
	cost_limit_ticks: null,
  planned_at: null,
  created_at: "2026-08-23T00:00:00Z",
  started_at: "2026-08-23T00:00:01Z",
  completed_at: completed ? "2026-08-23T00:01:00Z" : null,
});

const revision = (status: RoomMemoryRevision["review_status"]): RoomMemoryRevision => ({
  id: "revision-3",
  room_id: "room-1",
  cycle_id: "cycle-3",
  synthesis_turn_id: "turn-synthesis",
  version: 3,
  schema_version: 1,
  synthesis,
  digest: "digest-3",
  creator_type: "agent",
  creator_id: "agent-facilitator",
  review_status: status,
  reviewed_by_user_id: null,
  reviewed_at: null,
  corrected_from_revision_id: null,
  created_at: "2026-08-23T00:01:00Z",
});

function renderOutcome(status: RoomMemoryRevision["review_status"] = "pending") {
  const currentRevision = revision(status);
  const completed = status === "accepted";
  const detail = {
    ...EMPTY_ROOM_DETAIL,
    room: {
      ...EMPTY_ROOM_DETAIL.room,
      id: "room-1",
      objective: "Choose a release strategy",
      accepted_memory_revision_id: completed ? currentRevision.id : null,
    },
    entries: [{
      id: "entry-1",
      cycle_id: "cycle-3",
      turn_id: "turn-1",
      ordinal: 4,
      type: "result" as const,
      author_type: "agent" as const,
      author_id: "agent-1",
      body: "Canary evidence",
      mentions: [],
      created_at: "2026-08-23T00:00:30Z",
    }],
    cycles: [roomCycle(completed)],
    memory_revisions: [currentRevision],
  };
  const props = {
    detail,
    state: deriveRoomOutcomeState(detail),
    reviewPending: false,
    retryPending: false,
    recommendationPending: false,
    onReview: vi.fn(),
    onRetry: vi.fn(),
    onCitation: vi.fn(),
    onPromoteRecommendation: vi.fn(),
    onRejectRecommendation: vi.fn(),
  };
  return {
    ...render(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <RoomOutcome {...props} />
      </I18nProvider>,
    ),
    props,
  };
}

describe("RoomOutcome", () => {
  it("shows who created the outcome revision", () => {
    const view = renderOutcome();

    expect(view.getByText("Created by Facilitator Agent")).toBeTruthy();
  });

  it("jumps to citations and keeps promotion gated until acceptance", () => {
    const view = renderOutcome();

    fireEvent.click(view.getAllByRole("button", { name: "Open citation #4" })[0]!);
    expect(view.props.onCitation).toHaveBeenCalledWith("entry-1");
    expect(view.getByTestId("room-approve-recommendation-recommendation-1")).toBeDisabled();
    expect(view.getByTestId("room-reject-recommendation-recommendation-1")).toBeEnabled();
  });

  it("edits structured facts while preserving their citations", async () => {
    const user = userEvent.setup();
    const view = renderOutcome();

    await user.click(view.getByTestId("room-correct-outcome"));
    const fact = view.getByRole("textbox", { name: "Facts 1" });
    await user.clear(fact);
    await user.type(fact, "The canary remained healthy for 30 minutes.");
    await user.click(view.getByTestId("room-submit-correction"));

    expect(view.props.onReview).toHaveBeenCalledWith(
      "correct",
      expect.objectContaining({
        facts: [{
          text: "The canary remained healthy for 30 minutes.",
          citation_entry_ids: ["entry-1"],
          confidence: 0.9,
        }],
      }),
    );
  });

  it("preserves a correction draft when the same revision is refreshed", async () => {
    const user = userEvent.setup();
    const view = renderOutcome();

    await user.click(view.getByTestId("room-correct-outcome"));
    const summary = view.getByRole("textbox", { name: "Summary" });
    await user.clear(summary);
    await user.type(summary, "Keep this local correction draft.");

    const refreshedDetail = {
      ...view.props.detail,
      memory_revisions: view.props.detail.memory_revisions.map((candidate) => ({
        ...candidate,
        synthesis: { ...candidate.synthesis },
      })),
    };
    view.rerender(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <RoomOutcome
          {...view.props}
          detail={refreshedDetail}
          state={deriveRoomOutcomeState(refreshedDetail)}
        />
      </I18nProvider>,
    );

    expect(view.getByRole("textbox", { name: "Summary" })).toHaveValue(
      "Keep this local correction draft.",
    );
  });

  it("enables promotion only for an accepted outcome from a completed cycle", () => {
    const view = renderOutcome("accepted");
    const approve = view.getByTestId("room-approve-recommendation-recommendation-1");

    expect(approve).toBeEnabled();
    fireEvent.click(approve);
    expect(view.props.onPromoteRecommendation).toHaveBeenCalledWith(
      "revision-3",
      synthesis.recommendations[0],
    );
  });
});
