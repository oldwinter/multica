import { describe, expect, it } from "vitest";
import type { InboxItem } from "@multica/core/types";
import { deduplicateInboxItems } from "./inbox-display";

function item(overrides: Partial<InboxItem>): InboxItem {
  return {
    id: "inbox-1",
    workspace_id: "workspace-1",
    recipient_type: "member",
    recipient_id: "member-1",
    actor_type: "agent",
    actor_id: "agent-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    issue_status: null,
    read: false,
    archived: false,
    created_at: "2026-06-15T08:00:00Z",
    details: null,
    ...overrides,
  };
}

describe("deduplicateInboxItems", () => {
  it("keeps the newest issue row while preserving an older comment anchor", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "comment-notification",
        created_at: "2026-06-15T08:00:00Z",
        details: { comment_id: "comment-1" },
      }),
      item({
        id: "status-notification",
        type: "status_changed",
        created_at: "2026-06-15T08:01:00Z",
        details: { from: "in_progress", to: "in_review" },
      }),
    ]);

    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({
      id: "status-notification",
      type: "status_changed",
      details: {
        from: "in_progress",
        to: "in_review",
        comment_id: "comment-1",
      },
    });
  });

  it("deduplicates Room retries by physical review identity", () => {
    const merged = deduplicateInboxItems([
      item({
        id: "outcome-old",
        issue_id: null,
        room_id: "room-1",
        room_cycle_id: "cycle-1",
        room_review_identity: "revision-1",
        type: "room_outcome_review_required",
      }),
      item({
        id: "outcome-new",
        issue_id: null,
        room_id: "room-1",
        room_cycle_id: "cycle-1",
        room_review_identity: "revision-1",
        type: "room_outcome_review_required",
        created_at: "2026-06-15T08:02:00Z",
      }),
      item({
        id: "recommendation",
        issue_id: null,
        room_id: "room-1",
        room_cycle_id: "cycle-1",
        room_review_identity: "recommendation-1",
        type: "room_recommendation_review_required",
        created_at: "2026-06-15T08:03:00Z",
      }),
    ]);

    expect(merged.map((entry) => entry.id)).toEqual([
      "recommendation",
      "outcome-new",
    ]);
  });
});
