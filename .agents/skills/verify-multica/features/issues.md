# Issues

Issues is the workspace board/list of work. A user creates a titled issue from **New Issue**, can dismiss an unfinished draft, and opens an issue to its detail page with Properties.

## Sub-features

- `issues-board` shows the issues page with board columns.
- `issues-list` switches the same page to List.
- `issues-create` creates an issue from the manual dialog and opens it.
- `issues-cancel` dismisses the create dialog without creating.
- `issues-detail` opens an issue from the board/list into the detail view.

## How to get to it (user POV)

- After sign-in, the default workspace URL is `/{slug}/issues`.
- Sidebar link `Issues` (exact name).
- Button `New Issue` on the issues page, or shortcut `C` when focus is not in an editable field.
- Click an issue title/link on the board or list.

## Driving it with control-multica

Preconditions:

- Doctor is ok.
- `control-multica login` (or `drive issues`, which logs in) produced a `verify-*` workspace.
- Init script set `multica_create_mode` lastMode `manual` and closed chat.
- No leftover create dialog is open.

Scripted path:

```bash
node .agents/skills/verify-multica/control-multica.mjs drive issues
```

Manual equivalent:

- **Open board.** `goto /{slug}/issues`. Button `New Issue` is visible. Columns `Backlog`, `Todo`, `In Progress` are visible (empty workspace still shows the columns). Tab title `Issues | Multica`. Screenshot `01-issues-board.png`.
- **Switch list.** Choose `List`. The list view is showing (the created title, if any, is a row, not only a board card). Return to `Board` if the next step expects the board.
- **Open editor.** Choose `New Issue`. Textbox `Issue title` appears. Screenshot `02-create-modal.png`.
- **Save.** Fill a unique title `Verify issue <run-id>`. Choose `Create Issue`. Text `Issue created` appears. Notifications region contains the title. Choose `View issue`. URL matches `/issues/{id}`. `Properties` is visible. Tab title contains the title and `| Multica`. Screenshot `03-issue-detail.png` and save body text.
- **Cancel draft.** From issues, `New Issue`, type `Discard me`, press `Escape`. Title textbox is gone; `New Issue` is visible; no issue titled `Discard me`.
- **Proof.** `artifacts/<run-id>/issues/evidence.json` plus the PNGs. Opening the issue is required; the toast alone is not.

## Gotchas

- If lastMode is agent, the dialog is `Quick create issue`, not the manual title field. Set `multica_create_mode` or choose the manual breadcrumb `Create manually`.
- Shortcut `C` while a textbox is focused types the letter.
- Empty workspaces show `No issues yet` / `Create an issue to get started.` — columns should still be there on Board.
- Issue links may be `/issues/{uuid}` or `/{slug}/issues/{id}`. Match href with a suffix `/issues/{id}`.
- Chat overlay hides **New Issue** if `multica:chat:isOpen` is true.
- Creating via `POST /api/issues` is allowed only to seed extra rows, not to prove `issues-create`.
