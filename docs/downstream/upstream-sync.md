# Upstream Sync And Conflict Hygiene

This fork tracks `multica-ai/multica`. Local product lives beside that
upstream, not inside it: Rooms, Twin / LM Wiki, named skins, and a few
search/issue commands.

Use this page when merging `upstream/main`. The short pointer lives in
`AGENTS.md`.

## 2026-08-27 Sync

- Downstream start: `f56235422`.
- Merge base: `3c4288dde`.
- Upstream ahead: 54 commits through `3d37828e9`
  (`v0.4.35-10-g3d37828e9`).
- Local unique commits: 47.
- Conflict files: 18.
- Merge commit: `b53ba948b`.

The conflicts were concentrated in shared lifecycle hubs rather than local
feature leaves:

| Class | Conflict paths |
| --- | --- |
| Core and realtime | `packages/core/api/client.test.ts`, `packages/core/issues/queries.test.ts`, `packages/core/realtime/use-realtime-sync.ts` |
| Shared views | `packages/views/inbox/components/inbox-page.tsx`, `packages/views/issues/components/comment-card.tsx`, `packages/views/issues/components/issue-detail-route.test.tsx`, `packages/views/issues/components/issue-detail-route.tsx` |
| Daemon and handlers | `server/cmd/migrate/main.go`, `server/cmd/server/listeners.go`, `server/internal/daemon/types.go`, `server/internal/handler/daemon.go`, `server/internal/handler/handler.go`, `server/internal/handler/issue.go`, `server/internal/handler/workspace_delete_manifest_test.go` |
| Task, metrics, and sqlc | `server/internal/metrics/labels.go`, `server/internal/service/task.go`, `server/pkg/db/queries/agent.sql`, `server/pkg/db/generated/agent.sql.go` |

Additive frontend behavior stayed additive: Wiki and Room realtime events live
beside upstream chat-session creation, the inbox keeps both the issue-limit
upgrade path and Room navigation, comment menus keep copy-link and source-child
creation, and issue deep links accept both the downstream `?comment=` form and
upstream `#comment-` form. The query parameter wins when both are present, and
canonical links preserve the current search before adding the hash.

The task lifecycle resolution kept upstream runtime authorization,
source-context transfer, and channel delivery in the same retry transaction.
Room retries additionally take the workspace write lock inside that
transaction. Daemon claiming retained the capability-aware Room refill loop
while incorporating upstream authorization and finalization. Issue deletion
kept upstream batch, child-detach, and source-context cleanup plus downstream
Twin binding cleanup; the workspace deletion manifest now covers both local
Rooms/Wiki/Twin tables and upstream channel, source-context, and seat-capacity
tables.

Upstream replaced the old issue-window entitlement with the Cloud issue-count
limit. This was a lifecycle retirement rather than an additive conflict, so the
old handler and its metric label stayed deleted. The canonical Cloud endpoint
is now `MULTICA_CLOUD_URL`; three downstream human-actor tests were updated from
the retired fleet variable.

The agent claim query was resolved at the SQL source: upstream's runtime access
fence plus downstream `include_room_tasks`. `sqlc` 1.31.1 then regenerated the
Go output; a second generation produced no diff. The two published migration
histories overlap again at 404, 407-432, and 437-439. Their complete filename
identities remain unchanged and the exact duplicate stems are frozen in
`migrations_lint_test.go`; new migrations must not extend those sets.

Upgrade validation covered both database histories. The existing downstream
database advanced from 565 to 595 ledger identities and a second `migrate up`
was a no-op. A temporary empty database applied the complete merged history to
584 identities, passed a second no-op run, and was removed. Migration, handler,
service, and server tests passed.

The full Go package run exposed two container-identity artifacts: root can read
a chmod-denied fixture, and an artificial HOME that does not contain the root
username cannot be masked by the home-path test. The permission case passed as
UID 1000 and the redaction case passed with root's normal HOME. Frontend
verification passed core (1,852 tests), web (251), desktop (528), docs (17),
and UI token contracts (7). The views run passed 5,011 tests, then exposed
seven stale Twin test fixtures and one load-sensitive five-second timeout; the
fixtures passed after restoring their navigation/query boundaries (7 tests),
and the timed-out source-context test passed in isolation. Typecheck passed 7/7
workspace tasks and lint completed with zero errors. Conflict-marker search,
unmerged-path search, `git diff --check`, generated-output checks, and the
upstream ancestor check were clean.

Two compatibility lessons are worth retaining. First, tests that parse shared
CSS tokens must support grouped selectors such as `:root,` plus an appearance
fixture selector; exact string lookup silently couples the test to formatting.
Second, a full-module mock must preserve newly added exports used by nested
children, or use a partial mock. Both failures appeared only after upstream
changed a shared registration point while downstream tests still modeled the
older boundary.

## 2026-08-25 Sync

- Downstream start: `017a79956`.
- Merge base: `0a54725fe`.
- Upstream ahead: 21 commits through `3c4288dde` (`v0.4.33` plus two commits).
- Local unique commits: 30.
- Conflict files: 1.
- Merge commit: `f93799f74`.

The only conflict was `server/internal/featureflags/keys.go`. Upstream retired
the `CustomIssueStatuses` rollout gate after custom statuses became generally
available, and deliberately stopped publishing that key so pre-v0.4.33 clients
remain fail-closed. The downstream branch had added `TwinExecution` beside the
old gate. Keeping the whole local block would have silently resurrected the
retired status gate; taking the whole upstream block would have removed Twin's
operational kill switch. The resolution accepted the complete upstream
lifecycle change and reattached only the independent Twin constant, helper, and
frontend decision.

This sync added three useful proof techniques:

1. `git merge-tree --write-tree main upstream/main` predicted the single
   conflict before the worktree changed.
2. The index stages (`git show :1:<path>`, `:2:<path>`, and `:3:<path>`) plus
   the path's commit history exposed lifecycle intent that conflict markers did
   not.
3. `git diff upstream/main -- <path>` proved the resolved file differed from
   upstream only by Twin, while `git diff ORIG_HEAD -- <path>` proved the
   upstream gate retirement survived.

Targeted verification covered `server/internal/featureflags` and the handler
config/Twin contract. Both passed with Go 1.26.7.

The Git merge had only one textual conflict, but the existing downstream
database exposed a second, runtime-only conflict. Downstream outcome-loop
migrations had been published, then renumbered twice while their original full
filename stems remained in `schema_migrations`. `migrate up` therefore tried to
replay current `414_wiki_knowledge_loop` over a database that had already
applied the same DDL as `420_wiki_knowledge_loop`, failing on the existing
`wiki_page.current_revision_number` column.

Commit `56dd91f25` repairs the migration-identity portion of the upgrade path
with an explicit, append-only alias
map for all 52 affected Rooms, Wiki, Twin, and appearance migrations. When an
old identity is present, the migrator records the current identity without
executing its SQL; rollback removes both current and old identities. Tests
prove SQL is skipped on upgrade, rollback does not leave a stale identity, all
alias targets exist, and both historical `up.sql` generations are byte-for-byte
identical to the current files. The real pre-merge database then upgraded to
completion: ten Wiki migrations plus the Wiki primary-key migration were
reconciled, while genuinely pending Twin and Room migrations executed normally.

That successful migration still left a schema-level conflict. The original
published `420_wiki_knowledge_loop` did not contain the evidence-egress fields;
commit `c1b7d048a` later added three `lm_wiki_revision` fields and
`lm_wiki_source_policy.remote_generation_enabled` to the same migration file.
Comparing only the two later renumbered generations therefore gave a false
assurance: their SQL matched each other, but not the blob the database had
actually executed. The Wiki suite exposed the drift after migration completed.

Migration `455_wiki_evidence_egress_compatibility` is the forward repair. It
adds the four missing fields idempotently, installs and validates the two
evidence constraints, and deliberately has a no-op down direction because the
version-454 application already depends on this schema. This is not a pattern
for making ordinary migrations broadly idempotent; it is a narrow recovery
from mutating published history. The pre-merge database then passed the Wiki Go
suite and all targeted core, views, web, desktop, and mobile Wiki tests. A
temporary empty database also applied the complete 001-455 sequence and exposed
all four fields before it was removed.

This yields two additional proof techniques: always run the merged migrator
against a database carrying the pre-merge downstream ledger, and then exercise
the affected feature or inspect its required schema. A clean Git index, fresh
database, migration lint, or successful migration command cannot reveal every
published-history drift.

TypeScript typecheck passed 7/7 workspace tasks. A broad Vitest pass was stopped
after it created sustained worker load; its two search-command timeouts passed
36/36 when isolated. `git diff --check`, marker search, unmerged-path search,
sqlc regeneration, and the upstream ancestor check were clean.

## 2026-08-20 Sync

- Merge base: `38c992ad` (`v0.4.28`, 2026-08-17).
- Upstream ahead: 111 commits through `f737974b7` (`v0.4.31`).
- Local unique commits: 14.
- Conflict files: 25.

The same ownership rules held. Custom issue status support stayed upstream-owned,
with local completion effects and skin tokens reattached at the picker boundary.
Plugin v2 stayed upstream-owned, while the workspace deletion manifest retained
the local Twin, Wiki, and Rooms tables. Runtime-scoped task claiming and health
gates stayed upstream-owned, with room-task admission added as one extra scope.

This sync also joined two published migration histories that both used numeric
prefixes 251-309. The migration ledger stores the complete filename stem, so
renumbering either history would make existing installations replay DDL. The
merged checkout therefore freezes the exact duplicate stem sets and registers
every local concurrent index with the migrator's invalid-index cleanup hooks.
New migrations must still use the next unique prefix after the repository
maximum. Later outcome-loop renumbering violated this identity rule; the
2026-08-25 compatibility map repairs those already-published names, but is not
permission to rename another migration.

## 2026-08-17 Sync

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

1. `git fetch upstream main --tags`, record both tips and divergence, then
   preview with `git merge-tree --write-tree main upstream/main`.
2. Merge with `git merge --no-ff --no-edit upstream/main`; do not rebase
   published `main`.
3. Capture the actual conflict paths, then classify each before editing:
   additive list, hub rewrite,
   glossary, generated, or deleted surface.
4. Read the base/downstream/upstream index stages and the commits that created
   each side. A nearby local addition does not keep an upstream-retired
   lifecycle alive.
5. Resolve SQL sources, then regenerate:

   ```bash
   make sqlc
   ```

6. Compare each resolution to both parents; relative to upstream, only the
   intended downstream delta should remain.
7. If migrations changed, run `migrate up` against both a fresh database and a
   database carrying the pre-merge downstream `schema_migrations` ledger. Run
   it twice on the latter to prove reconciliation is idempotent, then run the
   feature tests that query the migrated schema.
8. Keep local modules out of upstream hubs when a follow-up refactor is
   cheap. Prefer `packages/core/rooms`, `packages/core/twins`,
   `packages/core/wiki` and thin registration points.
9. After the merge, search for leftover conflict markers, then run the
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
9. **Migration identity is immutable after publication.**
   `schema_migrations` stores the complete filename stem. Never renumber a
   migration that may have reached a user database, even to remove a numeric
   collision. Preserve both stems and freeze the duplicate in lint. If a
   historical rename already shipped, reconcile only byte-identical `up.sql`
   identities through the migrator's explicit alias map and test an existing
   ledger; making the DDL broadly idempotent can conceal a different schema.
10. **Migration contents are immutable after publication.** Comparing renamed
   files at today's tips is insufficient: inspect the migration blob from each
   actually deployed release. If an old file was changed after release, repair
   the missing schema with a new forward migration and validate the resulting
   columns and constraints on an old database. Never use a ledger alias to
   claim unapplied DDL ran.

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
