// @vitest-environment jsdom

import { cleanup, fireEvent, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { CreateRoomInput, RoomDetail } from "@multica/core/rooms";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { CreateRoomDialog } from "./create-room-dialog";

const TEST_RESOURCES = { en: { rooms: enRooms } };

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

afterEach(cleanup);

function renderDialog(input?: {
  readonly initialInput?: CreateRoomInput;
  readonly mode?: "create" | "duplicate";
}) {
  const onCreate = vi.fn<
    (input: CreateRoomInput, onSuccess: (detail: RoomDetail) => void) => void
  >();
  const view = render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <CreateRoomDialog
        open
        agents={[]}
        squads={[]}
        members={[]}
        pending={false}
        initialInput={input?.initialInput}
        mode={input?.mode}
        onOpenChange={vi.fn()}
        onCreate={onCreate}
      />
    </I18nProvider>,
  );
  return { ...view, onCreate };
}

describe("CreateRoomDialog", () => {
  it("offers an Improvement Room with focused, reviewable defaults", () => {
    const view = renderDialog();
    const card = view.getByTestId("room-template-improvement");

    expect(card).toHaveTextContent("Skill improvement");
    expect(card.querySelector("svg")).not.toBeNull();

    fireEvent.click(card);

    expect(card).toHaveAttribute("aria-pressed", "true");
    expect(view.getByRole("textbox", { name: "Objective" })).toHaveValue(
      "Turn eligible feedback into the smallest evidence-backed Skill improvement recommendation.",
    );
    fireEvent.click(view.getByRole("button", { name: "Advanced configuration" }));
    expect(
      (view.getByRole("textbox", { name: "Success criteria" }) as HTMLTextAreaElement)
        .value,
    ).toContain("The proposed change is small, reviewable, and explains why");
    expect(
      (view.getByRole("textbox", { name: "Instructions" }) as HTMLTextAreaElement).value,
    ).toContain("Propose principles, not exhaustive rules");
  });

  it("changes untouched defaults but preserves user edits across templates", () => {
    const view = renderDialog();
    const objective = view.getByRole("textbox", { name: "Objective" });

    expect(objective).toHaveValue(
      "Answer the research question with cited evidence and explicit uncertainty.",
    );
    fireEvent.click(view.getByTestId("room-template-planning"));
    expect(objective).toHaveValue("Produce an ordered, owner-ready execution plan.");

    fireEvent.change(objective, { target: { value: "Keep this specific objective." } });
    fireEvent.click(view.getByTestId("room-template-risk"));
    expect(objective).toHaveValue("Keep this specific objective.");

    fireEvent.click(view.getByRole("button", { name: "Reset defaults" }));
    expect(objective).toHaveValue("Identify, rank, and mitigate the material risks.");
  });

  it("submits a scheduled duplicate as a paused fresh Room", () => {
    const initialInput: CreateRoomInput = {
      title: "Weekly review",
      template_id: "planning",
      objective: "Review the week",
      success_criteria: ["Priorities are ordered"],
      stop_conditions: ["Owners are assigned"],
      facilitator_agent_id: "agent-1",
      daily_turn_limit: 8,
      max_cost_ticks: 40,
      schedule_interval_minutes: 1440,
    };
    const view = renderDialog({ initialInput, mode: "duplicate" });

    expect(view.getByText(/scheduled copy will be created paused/i)).toBeTruthy();
    fireEvent.click(view.getByTestId("room-create-submit"));

    expect(view.onCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Weekly review (Copy)",
        template_id: "planning",
        objective: "Review the week",
        facilitator_agent_id: "agent-1",
        daily_turn_limit: 8,
        max_cost_ticks: 40,
        schedule_interval_minutes: 1440,
        start_paused: true,
      }),
      expect.any(Function),
    );
  });
});
