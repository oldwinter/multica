// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  buildWikiUpdateBody,
  rebaseWikiDraftAfterConflict,
} from "./wiki-edit-conflict";
import {
  buildUpdateWikiPageBody,
  getWikiRevisionConflict,
} from "./wiki-schema";

describe("Mobile Wiki edit conflict recovery", () => {
  it("keeps the draft and retries against the server revision", () => {
    const draft = {
      path: "playbook/on-call.md",
      title: "值班交接清单与异常处理说明",
      content: "# Local draft\n\nKeep this unsaved change.",
      baseRevision: 3,
    };

    const conflict = getWikiRevisionConflict({
      code: "wiki_revision_conflict",
      current_revision_number: 4,
    });
    expect(conflict).toEqual({ currentRevisionNumber: 4 });

    const rebased = rebaseWikiDraftAfterConflict(
      draft,
      conflict!.currentRevisionNumber,
    );

    expect(rebased).toEqual({ ...draft, baseRevision: 4 });
    const retry = buildWikiUpdateBody(rebased);
    expect(retry).toEqual({
      path: "playbook/on-call.md",
      title: "值班交接清单与异常处理说明",
      content: "# Local draft\n\nKeep this unsaved change.",
      expectedRevisionNumber: 4,
    });
    expect(buildUpdateWikiPageBody(retry)).toEqual({
      path: "playbook/on-call.md",
      title: "值班交接清单与异常处理说明",
      content: "# Local draft\n\nKeep this unsaved change.",
      expected_revision_number: 4,
    });
  });
});
