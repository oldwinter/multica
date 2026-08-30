// @vitest-environment jsdom

import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AgentTask } from "@multica/core/types/agent";
import { renderWithI18n } from "../test/i18n";
import { TaskRunReviewBand } from "./task-run-review-band";

const terminalTask: AgentTask = {
  id: "task-1",
  agent_id: "agent-1",
  runtime_id: "runtime-1",
  issue_id: "issue-1",
  status: "completed",
  priority: 0,
  dispatched_at: null,
  started_at: "2026-06-08T08:00:00Z",
  completed_at: "2026-06-08T08:01:00Z",
  result: null,
  error: null,
  created_at: "2026-06-08T08:00:00Z",
};

const reviewSkills = [
  { id: "skill-1", name: "Compatibility checks", assignedToTaskAgent: true },
  { id: "skill-2", name: "Release notes", assignedToTaskAgent: false },
];

afterEach(cleanup);

describe("TaskRunReviewBand", () => {
  it("submits helpful feedback with a concrete reason", async () => {
    const onSubmit = vi.fn().mockResolvedValue({ id: "review-1" });
    renderWithI18n(<TaskRunReviewBand task={terminalTask} onSubmit={onSubmit} skills={reviewSkills} />);

    await userEvent.click(screen.getByRole("button", { name: "Review this run" }));
    expect(screen.getByRole("button", { name: "Helpful" })).toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "Helpful" }));
    await userEvent.click(screen.getByRole("button", { name: "Submit review" }));
    expect(screen.getByText("Choose what this feedback concerns.")).toBeInTheDocument();
    expect(screen.getByLabelText("Feedback target")).toHaveFocus();
    await userEvent.selectOptions(screen.getByLabelText("Feedback target"), "knowledge");
    await userEvent.type(screen.getByLabelText("Reason"), "The cited source made the answer verifiable.");
    await userEvent.click(screen.getByRole("button", { name: "Submit review" }));

    expect(onSubmit).toHaveBeenCalledWith({
      outcome: "helpful",
      target: "knowledge",
      reason: "The cited source made the answer verifiable.",
      idempotencyKey: expect.any(String),
    });
  });

  it("requires a correction and a listed workspace skill for skill feedback", async () => {
    const onSubmit = vi.fn().mockResolvedValue({ id: "review-1" });
    renderWithI18n(<TaskRunReviewBand task={terminalTask} onSubmit={onSubmit} skills={reviewSkills} />);

    await userEvent.click(screen.getByRole("button", { name: "Review this run" }));
    await userEvent.click(screen.getByRole("button", { name: "Needs correction" }));
    await userEvent.selectOptions(screen.getByLabelText("Feedback target"), "skill_procedure");
    await userEvent.type(screen.getByLabelText("Reason"), "The run skipped a required compatibility check.");
    await userEvent.click(screen.getByRole("button", { name: "Submit review" }));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText("Choose the skill this review concerns.")).toBeInTheDocument();
    expect(screen.getByLabelText("Target skill")).toHaveFocus();

    await userEvent.selectOptions(screen.getByLabelText("Target skill"), "skill-1");
    await userEvent.type(screen.getByLabelText("Specific correction"), "Keep the fallback and add a regression test.");
    await userEvent.click(screen.getByRole("button", { name: "Submit review" }));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({
      outcome: "needs_correction",
      target: "skill_procedure",
      skillId: "skill-1",
      correction: "Keep the fallback and add a regression test.",
    }));
  });

  it("keeps its idempotency key when a failed submission is retried", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("not found"));
    renderWithI18n(<TaskRunReviewBand task={terminalTask} onSubmit={onSubmit} submitError skills={reviewSkills} />);

    await userEvent.click(screen.getByRole("button", { name: "Review this run" }));
    await userEvent.click(screen.getByRole("button", { name: "Helpful" }));
    await userEvent.selectOptions(screen.getByLabelText("Feedback target"), "product_defect");
    await userEvent.type(screen.getByLabelText("Reason"), "The run exposed a reproducible crash.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(onSubmit).toHaveBeenCalledTimes(2);
    expect(onSubmit.mock.calls[0]?.[0].idempotencyKey).toBe(onSubmit.mock.calls[1]?.[0].idempotencyKey);
  });

  it("returns focus on Escape, focuses success, and stays hidden for live runs", async () => {
    const onSubmit = vi.fn().mockResolvedValue({ id: "review-1" });
    const rendered = renderWithI18n(<TaskRunReviewBand task={terminalTask} onSubmit={onSubmit} />);
    await userEvent.click(screen.getByRole("button", { name: "Review this run" }));
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.getByRole("button", { name: "Review this run" })).toHaveFocus());

    rendered.rerender(<TaskRunReviewBand task={terminalTask} onSubmit={onSubmit} submitted />);
    expect(screen.getByRole("status")).toHaveFocus();

    rendered.rerender(<TaskRunReviewBand task={{ ...terminalTask, status: "running" }} onSubmit={onSubmit} />);
    expect(screen.queryByText("Task run review")).not.toBeInTheDocument();
  });
});
