# Issue comments

A user opens an issue and posts a comment from the composer. The comment then appears in the issue activity.

## Sub-features

- `comment-shell` shows the inactive composer (`Leave a comment...`) with submit disabled.
- `comment-post` activates the editor, types a comment, submits, and shows the text in activity.

## How to get to it (user POV)

- Open any issue detail: from the board/list, or `/{slug}/issues/{id}`.
- The composer sits under the issue body / activity, labeled by placeholder `Leave a comment...`.

## Driving it with control-multica

Preconditions:

- Doctor is ok.
- Disposable session. Create or open one issue in this workspace (API seed is allowed here — the thing under test is commenting, not create). Title known.
- Chat overlay closed.

- **Open issue.** `goto /{slug}/issues/{id}`. Wait for the issue title and `Properties`.
- **Idle composer.** `getByTestId("comment-composer-shell")` is visible and contains `Leave a comment...`. The send control (arrow-up button in that composer) is disabled while empty. Screenshot idle.
- **Activate.** Click the shell. A ProseMirror editor with placeholder `Leave a comment...` is visible.
- **Type and send.** Type `Verify comment <run-id>`. Press `ControlOrMeta+Enter` (Windows: Control+Enter). The comment text is visible in the page within a few seconds.
- **Proof.** Screenshot after post. Body text dump contains the comment. Reloading the issue still shows it. Artifacts: `artifacts/<run-id>/comments/`.

## Gotchas

- The shell is not a textbox. Click it before typing. Filling the shell node does nothing useful.
- Submit is `ControlOrMeta+Enter`, not a guaranteed labeled `Send` name.
- Empty submit must stay disabled on the shell; do not treat a missing toast as proof of that — check disabled state.
- Reloading is the second view. A comment that appears then vanishes is a failed proof.
- Do not prove commenting by inserting rows into the comment table.
