# Upstream Sync And Conflict Hygiene

This fork tracks `multica-ai/multica`. Local product lives beside that
upstream, not inside it: Rooms, Twin / LM Wiki, named skins, and a few
search/issue commands.

Use this page when merging `upstream/main`. The short pointer lives in
`AGENTS.md`.

## What This Sync Looked Like

- Merge base: `37f3bb7d` (`MUL-5587`, 2026-07-31 era).
- Upstream ahead: 347 commits through `38c992ad` (`v0.4.28`, 2026-08-17).
- Local unique commits: 13 (Twin, Rooms, skins, search/issue extras).
- Conflict files: 40. Most were not semantic fights. They were the same
  hub files receiving both an upstream rewrite and a local additive patch.

Typical conflict classes:

| Class | Examples | Resolution |
| --- | --- | --- |
| Additive export / import lists | `paths.ts`, permissions, draft registry, daemon capabilities, API schema imports | Keep both sides. |
| Hub files rewritten upstream | `auth-initializer.tsx`, onboarding steps, issue-detail highlight/scroll | Take upstream, then re-apply the local hook if it still exists (`warmWorkspaces`, room events). |
| Shared vocabulary | `zh-Hans` / `ja` / `ko` layout and search JSON | Keep local keys (`rooms`, `copy_page_link`, `surprise_issue`) and accept upstream glossary (`issue` → 任务 / タスク / 태스크). |
| Parallel task systems | `task.go` fail/retry/broadcast | Keep both locks and both event paths: chat session lock + room workspace lock; `taskEvent` helper + room lifecycle event. |
| Generated sqlc | `*.sql.go`, `models.go` | Resolve `pkg/db/queries/*.sql` first, then `sqlc generate`. Never hand-merge generated Go. |
| Deleted upstream surface | `runtime-aside-panel.tsx`, old onboarding `h1` / aside markup | Accept the deletion. Local theme commits had restyled the old shell; the new shell already owns layout. |

Also reserved the `rooms` workspace slug during this sync. New
`/{slug}/{section}` names must land in
`server/internal/handler/reserved_slugs.json` in the same change.

## How To Sync Next Time

1. `git fetch upstream main` and merge, do not rebase published `main`.
2. Classify each conflict before editing: additive list, hub rewrite,
   glossary, generated, or deleted surface.
3. Resolve SQL sources, then regenerate:

   ```bash
   make sqlc
   ```

4. Keep local modules out of upstream hubs when a follow-up refactor is
   cheap. Prefer `packages/core/rooms`, `packages/core/twins`,
   `packages/core/wiki` and thin registration points.
5. After the merge, search for leftover conflict markers, then run the
   narrowest compile/test you can (`make test`, `pnpm typecheck`, or the
   packages you touched).

## Rules That Prevent The Next 40-File Merge

These are the durable lessons. Follow them on every local feature, not
only during a sync.

1. **Own a leaf, register at a point.** New product (Rooms, Twin, Wiki)
   should add files under its own package/query/migration prefix. Touch a
   shared hub only to register one symbol: a path helper, a draft import,
   a capability string, a delete-manifest table, a reserved slug.
2. **Do not restyle or rewrite upstream shells you do not own.** Theme
   work that edits onboarding markup, auth bootstrap, or issue-detail
   scroll will conflict the next time upstream rewrites that shell. Put
   visual overrides in tokens, utilities, or a local wrapper.
3. **Do not duplicate a hub that upstream is actively changing.** Task
   lifecycle, workspace delete, daemon claim, and realtime sync are
   shared pipes. Extend them with a field, a capability, or a branch
   (`if task.RoomTurnID.Valid`), not a second copy of the function.
4. **Treat generated files as unmergeable.** `server/pkg/db/generated/*`
   and `packages/core/paths/reserved-slugs.ts` are outputs. Conflict in
   the source (`queries/*.sql`, `reserved_slugs.json`) and regenerate.
5. **Keep additive lists mechanically mergeable.** Import lists, path
   maps, i18n keys, capability slices, and delete-manifest tables should
   be one entry per line, sorted or grouped by owner, with no wrapping
   refactors in the same commit.
6. **Follow the glossary instead of protecting local wording.** Chinese
   `issue` is `任务`. When a local string and an upstream glossary
   change collide, keep the local key and use the upstream term.
7. **Reserve the slug, the delete table, and the capability in the
   feature PR.** A new workspace section that forgets
   `reserved_slugs.json`, `workspaceDeletionManifest`, or
   `workspace_delete.sql` will surface as a merge or CI failure later.
8. **Sync more often than the feature batch.** 13 local commits vs 347
   upstream commits is what turned additive patches into hub rewrites.
   Merge `upstream/main` after each local feature lands, not after a
   stack of them.

## Local Ownership Map

Use this to decide who wins a conflict:

| Owner | Paths |
| --- | --- |
| Local Rooms | `server/internal/room/`, `server/pkg/db/queries/room.sql`, `packages/core/rooms/`, `packages/views/rooms/`, `apps/*/rooms` |
| Local Twin / Wiki | `server/internal/service/twin*`, `lm_wiki*`, `wiki*`, `packages/core/twins/`, `packages/core/wiki/`, `packages/views/twins/`, `packages/views/wiki/` |
| Local skins / search extras | theme tokens, `data-twin-copy`, search commands `copy_page_link` / `surprise_issue` |
| Upstream | onboarding shell, auth recovery, plugins, share links, custom issue statuses, chat/task event contract, sqlc output |

When a conflict is inside an upstream-owned shell, take upstream and
re-attach the local registration. When it is inside a local-owned
module, keep local and replay any upstream type or helper rename.
