// @vitest-environment jsdom

import { cleanup, fireEvent, render } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { CreateRoomInput, RoomDetail } from "@multica/core/rooms";
import type { Agent } from "@multica/core/types";
import { afterEach, describe, expect, it, vi } from "vitest";
import enRooms from "../locales/en/rooms.json";
import { CreateRoomDialog } from "./create-room-dialog";

const TEST_RESOURCES = { en: { rooms: enRooms } };

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

afterEach(cleanup);

function renderDialog(input?: {
  readonly agents?: readonly Agent[];
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
        agents={input?.agents ?? []}
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

  it("focuses the name field instead of a template card", () => {
    const view = renderDialog();

    expect(view.getByRole("textbox", { name: "Name" })).toHaveFocus();
    expect(view.getByTestId("room-template-research")).not.toHaveFocus();
    expect(view.getByTestId("room-create-summary")).toBeInTheDocument();
    expect(view.getByTestId("room-create-submit")).toBeInTheDocument();
  });

  it("explains an empty facilitator directory instead of opening a blank menu", () => {
    const view = renderDialog();
    const facilitator = view.getByRole("combobox", { name: "Facilitator" });

    expect(facilitator).toBeDisabled();
    expect(view.getByRole("status")).toHaveTextContent(
      "No active Agents. Create or restore an Agent before creating a Room.",
    );

    fireEvent.click(view.getByRole("button", { name: "Squad" }));

    expect(view.getByRole("status")).toHaveTextContent(
      "No active Squads. Create or restore a Squad, or switch to Agent.",
    );
  });

  it("keeps mobile form content and footer actions in the bounded dialog scroller", () => {
    const view = renderDialog();
    const dialog = view.getByRole("dialog");
    const formScroll = view.getByTestId("room-create-scroll");

    expect(dialog).toHaveClass(
      "max-h-[min(48rem,calc(100dvh-2rem))]",
      "overflow-y-auto",
      "sm:overflow-hidden",
    );
    expect(dialog).not.toHaveClass("overflow-hidden");
    expect(formScroll).toHaveClass("sm:min-h-0", "sm:flex-1", "sm:overflow-y-auto");
    expect(formScroll).not.toHaveClass("flex-1", "overflow-y-auto");

    fireEvent.click(view.getByRole("button", { name: "Advanced configuration" }));
    expect(view.getByRole("textbox", { name: "Success criteria" })).toBeInTheDocument();
    expect(view.getByTestId("room-create-submit")).toBeInTheDocument();
  });

  it("submits a scheduled duplicate as a paused fresh Room", () => {
    const initialInput = scheduledDuplicateInput("agent-1");
    const view = renderDialog({
      agents: [makeAgent({ id: "agent-1", name: "Facilitator" })],
      initialInput,
      mode: "duplicate",
    });

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

  it("blocks a copied facilitator that is absent from the active directory", () => {
    const view = renderDialog({
      agents: [makeAgent({ id: "agent-2", name: "Available facilitator" })],
      initialInput: scheduledDuplicateInput("agent-1"),
      mode: "duplicate",
    });

    expect(view.getByRole("combobox", { name: "Facilitator" })).toBeEnabled();
    expect(view.getByTestId("room-create-submit")).toBeDisabled();
  });
});

function scheduledDuplicateInput(facilitatorAgentId: string): CreateRoomInput {
  return {
    title: "Weekly review",
    template_id: "planning",
    objective: "Review the week",
    success_criteria: ["Priorities are ordered"],
    stop_conditions: ["Owners are assigned"],
    facilitator_agent_id: facilitatorAgentId,
    daily_turn_limit: 8,
    max_cost_ticks: 40,
    schedule_interval_minutes: 1440,
  };
}

function makeAgent(overrides: Pick<Agent, "id" | "name">): Agent {
  return {
    id: overrides.id,
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    name: overrides.name,
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_args: [],
    visibility: "private",
    permission_mode: "private",
    invocation_targets: [],
    status: "idle",
    max_concurrent_tasks: 1,
    model: "",
    owner_id: "user-1",
    skills: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
  };
}
