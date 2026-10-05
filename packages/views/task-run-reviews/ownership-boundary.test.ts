// @vitest-environment node
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

const VIEWS_ROOT = join(__dirname, "..");
const LEAF_ROOT = join(VIEWS_ROOT, "task-run-reviews");
const OWNERSHIP_DOC = join(
  VIEWS_ROOT,
  "..",
  "..",
  "docs/downstream/skill-evolution/ownership.md",
);

function walkProductionSources(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry.startsWith(".")) continue;
    const path = join(dir, entry);
    if (path === LEAF_ROOT) continue;
    if (statSync(path).isDirectory()) walkProductionSources(path, out);
    else if (/\.tsx?$/.test(path) && !/\.test\.tsx?$/.test(path)) out.push(path);
  }
  return out;
}

describe("task run review ownership boundary", () => {
  it("keeps review policy in the leaf and only registered slots in shared transcript", () => {
    const sharedReferences = walkProductionSources(VIEWS_ROOT)
      .filter((path) => /TaskRunReview|taskRunReview|task-run-reviews/.test(readFileSync(path, "utf8")))
      .map((path) => relative(VIEWS_ROOT, path))
      .sort();

    expect(sharedReferences).toEqual([
      "common/task-transcript/agent-transcript-dialog.tsx",
      "common/task-transcript/transcript-button.tsx",
    ]);

    const dialog = readFileSync(
      join(VIEWS_ROOT, "common/task-transcript/agent-transcript-dialog.tsx"),
      "utf8",
    );
    expect(dialog).toContain("taskRunReviewSlot?: React.ReactNode");
    expect(dialog).not.toMatch(
      /@multica\/core\/task-run-reviews|TaskRunReviewBand|useCreateTaskRunReview|validateTaskRunReview|MAX_TASK_RUN_REVIEW|CreateTaskRunReview|idempotencyKeyRef|transcript\.review\./,
    );

    const button = readFileSync(
      join(VIEWS_ROOT, "common/task-transcript/transcript-button.tsx"),
      "utf8",
    );
    expect(button).toContain('import { TaskRunReviewSlot } from "../../task-run-reviews"');
    expect(button).not.toMatch(
      /@multica\/core\/task-run-reviews|TaskRunReviewBand|useCreateTaskRunReview|taskRunReviewMutation|taskRunReviewSkillsQuery/,
    );
  });

  it("registers every shared view path in the downstream ownership contract", () => {
    const ownership = readFileSync(OWNERSHIP_DOC, "utf8");
    for (const path of [
      "packages/views/task-run-reviews/",
      "packages/views/common/task-transcript/agent-transcript-dialog.tsx",
      "packages/views/common/task-transcript/transcript-button.tsx",
    ]) {
      expect(ownership).toContain(`\`${path}\``);
    }
  });
});
