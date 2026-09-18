# Upstream Sync And Conflict Hygiene

This fork tracks `multica-ai/multica`. Local product lives beside that
upstream, not inside it: Rooms, Twin / LM Wiki, named skins, and a few
search/issue commands.

Use this page when merging `upstream/main`. The short pointer lives in
`AGENTS.md`.

## 2026-09-18 v0.5.0 Sync

- Original checkout: `codex/skill-evolution` at
  `2b025b7b0437210d3a5d677a9139ab201f541b62`, already contained in published
  fork history. Local `main` was `b0a443dba0cc03deca453de633a4267a4ddcd684`.
- Fork reconciliation: fast-forwarded `main` to published `origin/main`
  `fe63cf530c054d9706f06e80cb6aede02ea5bdc9` (the merge's first parent).
- Upstream: `2df765a3c8f39789c9fb76316378bcffc20d22d9` (`v0.5.0`).
- Merge base: `3551e72e76d2c276e550b668303646d1280fb1e2`.
- Divergence: 283 downstream-only commits, 104 upstream-only commits,
  170 overlapping paths and 32 textual conflicts.
- Untracked `.factory/` was left untouched. The untracked `droid-wiki/`
  directory overlapped files newly published on `origin/main`; its original
  contents were preserved outside the checkout at
  `/home/cdd/multica-sync-backup-20260918-XSdLN7/droid-wiki` before the
  fast-forward. That backup is not part of the merge.

| Conflict group | Paths |
| --- | --- |
| CI and instructions | `.github/workflows/ci.yml`, `AGENTS.md`, `CLAUDE.md`, `apps/mobile/CLAUDE.md`, `apps/mobile/docs/rnr-migration.md` |
| Mobile | `apps/mobile/app/(app)/[workspace]/more/settings/notifications.tsx`, `apps/mobile/data/auth-store.ts`, `apps/mobile/lib/theme.ts` |
| Web and dependency graph | `apps/web/features/landing/components/{about-page-client,contact-sales-page-client}.tsx`, `apps/web/package.json`, `pnpm-lock.yaml` |
| Core, tokens and shared views | `packages/core/types/agent.ts`, `packages/ui/styles/tokens.css`, `packages/views/issues/components/{comment-card.tsx,pickers/status-picker.tsx}`, `packages/views/locales/index.ts`, `packages/views/settings/components/preferences-tab.test.tsx`, `packages/views/skills/components/skill-list-actions.tsx` |
| Backend | `server/cmd/migrate/main.go`, `server/cmd/server/listeners.go`, `server/internal/daemon/{client,daemon}.go`, `server/internal/featureflags/keys.go`, `server/internal/handler/{daemon.go,workspace_delete_manifest_test.go}`, `server/internal/service/task.go` |
| Generated database code | `server/pkg/db/generated/{agent.sql,autopilot.sql,chat.sql,models,runtime.sql}.go` |

The preservation ledger classified all 170 overlaps, including clean
auto-merges. Shared task, auth, issue, comment and CI lifecycles remain
upstream-owned; downstream Rooms, Twin/Wiki, Office, Skill Evolution,
appearance and copy actions remain registered at their existing seams.
Notable integration decisions:

- Task delivery now carries the upstream locked runtime/agent authorization
  and issue snapshot through the downstream Twin finalizer. Authorization,
  token issuance, Twin attribution, snapshot and comment receipt share one
  transaction. Room capability filtering, refill and retry locking remain.
- Upstream terminal-report persistence also stores the downstream dispatch
  timestamp and exact skill execution manifest. Queue comparisons use value
  equality after JSON decoding; the round-trip regression covers identical
  manifest re-enqueue. Completion contributors run only on the successful
  terminal transition, alongside upstream's other completion side effects.
- Upstream's four lifecycle categories coexist with concrete built-in status
  glyphs, custom icon choices and downstream completion feedback. Office
  fixtures follow the new categories; UI Lab's user fixture includes the
  downstream nullable appearance fields. Mobile consumers use the live skin
  theme; category colors reuse the existing built-in palette without adding
  raw-color debt.
- Personal Wiki events retain recipient-only routing; upstream invitation
  accept/decline events retain their new recipient routing. Both Triage and
  Twin flags and every workspace deletion ownership entry remain.
- Upstream French bundles retain their translations. Missing downstream keys
  and the five downstream namespaces use explicit English fallback copy;
  the landing appearance menu uses the same fallback. French translation of
  those local strings is still outstanding. All five
  locales have the same key and interpolation structure.
- CI adopts upstream scope selection, aggregate gates and the shared quality
  action while retaining trusted self-hosted runners, isolated service ports,
  disk-backed Go temporary files, Node 26 and Go 1.27. `go.mod` now agrees
  with that existing downstream Go toolchain. The consistency check runs in
  CI and follows upstream's new `CLAUDE.md` -> `AGENTS.md` pointer structure.

Published migration filenames and SQL contents were preserved. The lint
freezes the exact 20 new duplicate-prefix pairs, `480` through `499`.
Skill Evolution's isolated schema fixture selects its owned migration names
instead of importing unrelated upstream DDL through a numeric range.
The merged migrator retains both parents' retry cleanup hooks and skips the
retired search indexes and superseded lifecycle migration as upstream does.

PostgreSQL 17.11 validation used two disposable databases. The fresh path
applied all 690 migration identities. The upgrade path was initialized by
the migrator and schema from the exact first parent, then reached the same
690 identities. Second runs on both were no-ops. Both schemas had no invalid
or unready indexes, the four-category constraint and system mappings agreed,
and claim snapshots, triage, Twin, Wiki policy and Skill Evolution columns
were present. Tests used the disposable upgrade database, not the normal
development database.

Verification used Node 26.9.0, pnpm 10.28.2 and Go 1.27.1 on Linux:

- All workspace typechecks passed across the root run and final package
  checks. After fixing the new UI Lab user fixture and French appearance
  menu entry, `pnpm --dir apps/{web,desktop,ui-lab} typecheck` was run for
  each app sequentially. Core and mobile also passed explicit final
  `typecheck` / `tsc --noEmit` runs. The initial root run exposed those
  integration errors; it was not itself a green run.
- `pnpm install --lockfile-only` and `pnpm install --frozen-lockfile` passed.
  `make sqlc` ran twice; every generated Go file had the same SHA-256 after
  the second run. Reserved-slug generation also remained byte-identical.
- `go test -p 2 ./... -run '^$'` compiled every Go test package.
  `go test ./cmd/migrate ./internal/migrations -count=1` passed, along with
  daemon terminal/skill/Room tests and database-backed claim, authorization,
  snapshot, Twin, Wiki, Room, Skill Evolution, Triage, deletion and listener
  regressions. The extended Twin transaction test checks rejected and
  successful delivery together with snapshot persistence.
- Core: 2,267 tests verified (the obsolete total-invalidation-count
  assertion was replaced with the actual async inbox-summary contract, then
  its 25-test file passed). Mobile: 295 tests and the iOS runner shell suite.
  Web: 275 tests. Desktop: 9 focused path, daemon locale and Wiki tests.
  UI Lab's product fixture: 5 tests.
- Shared conflict/locale tests: 289 passed. UI token contracts: 7 tests,
  all six skin/mode combinations and the raw-color budget passed. Resolved
  shared view files passed ESLint. Toolchain consistency, wildcard exports
  and the 39 CI scope/image-budget tests passed.
- Extended downstream UI: the initial 57-file run passed 374 tests and had
  10 failures during concurrent compiler/CI load. All 79 tests in its five
  failing files passed when rerun with `--maxWorkers=1 --testTimeout=30000`,
  without further product or test changes. This covers all 384 selected
  downstream UI tests; it is not a claim that the original parallel run was
  green.
- Initial validation found the same two bare `rounded` classes reported on
  September 12. During PR preparation, the new upstream CI radius gate was
  reconciled with those downstream Rooms controls by using `rounded-sm` in
  `packages/views/rooms/create-room-dialog.tsx` and
  `packages/views/rooms/room-outcome.tsx`. The named token resolves to the
  same 0.25rem (4px) as the old utility; the radius gate now passes.

PR #52 validation found two additional integration issues. The clean merge
in `issue-detail.tsx` retained ordinary resolved-thread expansion but lost
upstream's equivalent branch for assignment-run comments. The shared landing
condition now covers both kinds while preserving downstream's valid-target
virtualization guard. The existing regression failed before the fix; all
83 tests in `issue-detail.test.tsx` passed afterward, as did ESLint.

Browser verification used a separate worktree, ports 18136/13056 and a
disposable `verify-*` account/workspace. Inbox notification clicks, `?comment=`
links and `#comment-` links all kept the reply hidden before the fix; all three
expanded it and applied the highlight afterward. Recordings and JSON results
are kept locally under
`.agents/skills/verify-multica/artifacts/upstream-sync-20260918/{baseline,fixed}/`.
The verification account, workspace and issue were deleted and its services
stopped; the isolated environment database remains for normal environment GC.

The first hosted backend job could not start because another project's
container owned port 55432. Both database-backed workflows now let Docker
allocate free PostgreSQL/Redis ports and read the assigned ports through
`job.services`, so they never attach to another project's service. YAML,
toolchain consistency and all 39 CI scope/image-budget tests passed locally.
Hosted checks continue on PR #52; this record does not claim a green initial
run. The initial hosted mobile, downstream feature gates, SQL generation,
vulnerability scan, Windows runtime and installer checks passed.

The original disposable migration databases and baseline worktree were
removed; the untracked-file backup remains. No native-device, live-agent,
deployment or release verification is implied.

## 2026-09-12 v0.4.43 Sync

- Downstream before sync: `5ccd181e3127e69fc9ae375b3b89ff2ea0083371`.
- Fork reconciliation: fast-forwarded published `origin/main` to
  `6027fc79bec2da34048c2db3895d8938b0c71520` (four downstream commits).
- Merge base: `d8fa885d26acbdecde49010276e905cc9de5a056`.
- Upstream: `3551e72e76d2c276e550b668303646d1280fb1e2` (`v0.4.43-2`).
- Divergence at preview: downstream unique 268, upstream unique 86,
  123 overlapping paths, 30 textual conflicts.
- Conflict decisions: upstream lifecycle and API contracts were retained in
  shared CI, daemon, handler, realtime, and issue shells. Downstream Rooms,
  Twin/Wiki, appearance, and listener-ownership behavior was reattached at
  its existing extension points. Migration lint retained both published
  filename sets and generated database code was regenerated from merged SQL.
- Lockfile and toolchain manifests were regenerated after resolving package
  manifests. Local worktree edits were stashed before the merge and restored
  afterward.

Validation run: `git diff --check`, exact conflict-marker scan,
`pnpm install --lockfile-only`, locale parity (190 tests), `pnpm typecheck`,
and `go build` for daemon, service, and handler packages. The full Go test
compile still has unrelated upstream daemon and handler fixtures; the
repository toolchain checker also reports the existing 1.26/1.27 documentation
drift and two Rooms radius literals. No push, release, deployment, browser,
device, or real-agent check is implied.

## 2026-09-07 v0.4.41 Sync

- Original checkout: `c49b8d2746a7886e1f31540ec78f22ee3289044f`.
- Fork reconciliation: fast-forward one published search-history commit to
  `9df6e7ee4ffdb2fdf656b475272a6f176bfa6824`, the merge's first parent.
- Merge base: `d4a712abf3880dfbd3daeac5daac1bd4bfb39b6f`.
- Upstream: 64 commits through
  `d8fa885d26acbdecde49010276e905cc9de5a056`
  (`v0.4.41-6-gd8fa885d2`). Downstream unique commits after reconciliation: 266.
- Both-side path overlap: 140. Predicted and actual conflict files: 37.
- Existing Web development files stayed outside the merge commit. This sync
  finishes locally; it does not push, release, deploy, or reconcile PR branches.

| Conflict group | Paths |
| --- | --- |
| Authentication | `apps/web/components/web-providers.tsx`, `packages/core/auth/store.ts`, `packages/core/platform/{auth-initializer,core-provider}.tsx` |
| Core contracts and tests | `packages/core/issues/mutations.ts`, `packages/core/types/api.ts`, `packages/core/paths/consistency.test.ts`, `packages/core/workspace/mutations.test.tsx` |
| Navigation and run confirmation | `packages/views/layout/{app-sidebar.tsx,sidebar-auto-collapse.test.tsx}`, `packages/views/modals/run-confirm{,.test}.tsx` |
| Settings | `packages/views/settings/components/{preferences-tab,settings-page}{,.test}.tsx`, all four `packages/views/locales/*/settings.json` bundles |
| Server composition and metrics | `server/cmd/server/{main.go,notification_mute_group_test.go}`, `server/internal/metrics/{registry.go,business_pairing_test.go}` |
| Daemon and handler contracts | `server/internal/daemon/{types.go,execenv/context.go}`, `server/internal/handler/{agent.go,daemon.go,daemon_batch_claim_test.go,issue_trigger.go,squad.go}` |
| Task, migration and other tests | `server/internal/service/{task.go,builtin_skills_test.go}`, `server/internal/migrations/migrations_lint_test.go`, `server/internal/analytics/events_test.go`, `server/internal/featureflags/keys_test.go`, `server/pkg/llm/outbound_contract_test.go` |

The auth resolution uses upstream's single idempotent session-expiry teardown.
The downstream authenticated-account marker is cleared inside that action,
including boot-time rejection, while appearance sync retains its account and
stale-response guards. Web keeps both locale-route gating and appearance sync.
The settings shell adopts upstream's grouped navigation and compact selector;
appearance controls live inside the new General preferences panel. Work owns
Rooms, Office and Wiki; AI Team owns Twin. Personal Wiki remains in the account
menu. Locale JSON was merged by keys and values from all three versions; no
scalar settings key required an arbitrary winner.

The run confirmation removes the retired handoff-note editor and runtime gate
while retaining the signed Twin preview and exact-version submission. The
backend still carries upstream's installed-client handoff input and the
downstream Twin override. Deferred channel enqueues preserve upstream's wakeup.
New actor-specific enqueue callers pass the downstream snapshot argument.
Rooms retain transaction-bound runtime lookup, claim capability checks,
generation precision, retry locks, and per-turn prompt context. The deleted
issue-context sidecar stays deleted; its old Room test now relies on the
existing per-turn prompt regression. Replica pools and routing remain upstream
owned, with the Skill Evolution metrics collector registered alongside them.

Upstream's declaration-only test removals are retained. Downstream analytics
privacy tests and the published migration-identity lint remain active. Wiki's
maintainer source map moved to [its documentation location](wiki-source-map.md)
so the shipped skill satisfies upstream's source-free payload contract. The
merged built-in skill tests accept Windows CRLF frontmatter. New upstream
archive-cancellation tests use an explicit pool for transactions and observers
beside downstream's transaction-capable fixture interface. The new OpenClaw
smoke workflow uses Node 26 and Go 1.27, matching the downstream toolchain.

Both `450_drop_comment_delegated_failure_pending_index` and
`450_room_memory_review_key_index` remain immutable published identities; the
lint freezes their exact pair. A temporary PostgreSQL 17.6 instance applied all
641 migrations to a fresh database. Another database was seeded by the exact
pre-sync migrator and schema (640 identities), then upgraded to the same 641.
Both second runs were no-ops. The retired delegated-failure index was absent,
the Room review-key index and Room/Twin/Skill Evolution columns were present,
and neither database had invalid or unready indexes. The normal local database
was not modified. Optional extensions followed the migrator's existing gates.
`sqlc` 1.31.1 ran twice with no generated-file difference.

Verification uses Node 26.7.0, pnpm 10.28.2 and Go 1.27.0 on Windows. Frozen
install, toolchain consistency, 9/9 root typecheck tasks and 6/6 lint tasks
passed. The Core suite passed 182 files / 1,988 tests, Web passed 36 files /
274 tests, and Docs passed three files / 15 tests. The focused merge suites
passed 276 tests. Views passed 479 of 480 files and 5,399 of 5,400 tests; its
remaining ownership-boundary assertion compares native Windows separators
with POSIX paths and reproduces at the original checkout. Desktop passed 58
files / 602 tests; its packaging module fails to load with the same syntax
error at the original checkout, while both script files pass `node --check`.
Mobile typecheck passed; its Vitest run passed 37 files / 223 tests with one
unchanged Room fixture URL/path loading failure. These are partial suite
results, not a green root `pnpm test` claim.

The complete Go module compiled with `go test -p 2 ./... -run '^$'`.
Database-backed migration, service, Room, server composition and Skill
Evolution checks passed. Read routing and skill-bundle package tests passed.
Focused handler checks passed after excluding three
test functions reproduced at the exact original checkout:
`TestParseSkillArchive_RejectsUnsafeSkillMdPath`, `TestWikiHandlersWithMockDB`
and `TestValidateWikiPath`, which assume POSIX filesystem paths. Root
`pnpm test` also hits the original Office generator's Windows separator bug;
the UI token suite hits original Windows URL/path conversion failures.
These two failures were reproduced in a detached copy of the original commit.
The unrestricted execenv run encountered Windows Git CRLF, home-path and
OpenClaw cache fixtures; it is not reported as passing. No production build,
browser E2E, real-agent smoke, Desktop display, Mobile device or human
acceptance check is claimed by this sync.

Commands used include `pnpm install --frozen-lockfile`,
`node scripts/check-toolchain-consistency.mjs`, `pnpm typecheck`, `pnpm lint`,
and `pnpm --dir <workspace> exec vitest run --maxWorkers=2` (Views used four
workers). pnpm was invoked through `npm exec --package=pnpm@10.28.2` because
the machine's default shim selected pnpm 11 with Node 24. The Go checks used
`go test -p 2`, the packages above, and focused Room/Twin/Wiki/Skill/Claim/
Autopilot/Replica/RuntimeLookup/Handoff patterns; the handler rerun's `-skip`
listed only the three reproduced baseline test functions. Migration runs
used explicit temporary `DATABASE_URL` values on `127.0.0.1:55439`.

## 2026-09-03 PR #17 And #19 Reconciliation

- Shared sync base: `c6eb0098e73367d6db736541f364b7460837e1f6`.
- PR #17 started at `581ef157d46f5fbbd031893d38dd3739370c6655`
  with 12 textual conflicts. Its conflict-resolution merge is
  `5b418ba502ed5b3ea0d4983ba8a02b37d23b1d50`. Its verified final head is
  `4f19536dfe03d6c3a6ddf667be502050a6e38616`. GitHub merged it as
  `bb335e941b4a06fd9a695efda33c3bf2f6eb7982`.
- PR #19 started at `7f0cf4d599683a79d4f09faa109d4204406332b7`
  with two textual conflicts. Its conflict-resolution and final head is
  `6a8250d19b3425cef782a327bfbe0f5affad27b6`. GitHub merged it as
  `517f06a45bd215c163b6c0d9d09bef2b1acf3b3c`.

The conflicts covered the Mobile and Views package manifests,
`pnpm-lock.yaml`, this log, the Wiki page primitives, the migration runner and
lint ledger, the runtime-lookup metric label, and four Room/task-service files.
The resolution keeps the PR's Node 26, Go 1.27, Electron 44, Expo 57,
React Native 0.86, Next.js 16.3, TypeScript 6, Vite 7, and Vitest 4 stack while
adopting the current main implementation. The lockfile was regenerated from
the resolved manifests instead of hand-edited. Expo Doctor also identified 11
SDK 57 packages that were one or two patch releases behind; those declarations
now match the SDK matrix.

Room runtime checks keep main's transaction-bound `AgentRuntimeLookupFactory`.
The obsolete `TaskEnqueuer.LookupRoomRuntime` method from the PR branch had
auto-merged outside the textual conflicts; it and its test stub were removed
instead of widening `TaskService` to support both architectures. The migration
runner keeps both independent protections: PR #17's schema-qualified migration
422 cleanup and main's optional `pg_bigm` operator-class gate for migration
446. Published migration filenames and the duplicate 444-449 ledger remain
unchanged.

Vitest 4 exposed a pre-existing test isolation error: the use-case locale test
mocked `@/.source`, while production imports `@/.source/server`. Before the
fix, Vite parsed generated MDX as JavaScript and failed the suite; after the
mock target was corrected, the focused five-test suite and the full Web suite
passed.

PR #19 conflicted only in `twin-panel.tsx` and
`twin-workspace-view.test.tsx`. The resolution keeps the PR's signed-version
copy action beside main's `TwinDestination` navigation contract and preserves
main's layout-ownership assertion. PR #17 and PR #19 had no changed-path
overlap. After PR #17 merged, a fresh exact-SHA `merge-tree` proved that PR #19
still merged cleanly with the new main, so its branch did not need another main
merge. GitHub briefly reported `UNKNOWN` while recomputing mergeability. The
merge waited for the clean result and was locked to the verified head SHA.

Independent shipping verification then found that the dependency and CI
upgrade had left current runtime surfaces on Node 22 and Go 1.26. The Web and
server Docker builders, `.nvmrc`, developer prerequisites, Mobile stack notes,
and generated Droid Wiki pages now derive from or agree with the Node 26,
Go 1.27, Expo 57, React Native 0.86, Electron 44, and Next.js 16 manifests.
`scripts/check-toolchain-consistency.mjs` makes those boundaries a build gate
so future manifest-only upgrades fail before packaging.

Verification passed the frozen pnpm install, 9/9 typecheck tasks, all six
frontend test tasks, and all five production build tasks. Views passed 481
files and 5,443 tests, Desktop passed 59 files and 578 tests, Web passed 34
files and 258 tests, and Mobile passed 36 files and 222 tests plus the iOS run
script assertions. Lint reported zero errors; Mobile retained seven existing
warnings. Expo Doctor passed 21/21 checks. A fresh temporary database ran the
complete migration graph, the second run skipped all 640 identities, and no
invalid or unready indexes remained before the database was removed.

Targeted Go verification passed migration, Room, service, handler, server, and
Skill Evolution packages. The full race run passed every other package and
retained the current-main baseline: four local agent-executable discovery
tests and three repo-cache fixed-branch fixtures fail in this environment. One
150 ms daemon watcher assertion also timed out under aggregate load and passed
immediately when rerun alone with the race detector. This reconciliation did
not run deployment, production, Desktop display, Mobile device, or human
acceptance checks.

## 2026-09-03 v0.4.38 Sync

- Downstream start: `7f7ae0f9bb0796035bc58a3b657ce93b616b3c6d`.
- Merge base: `11861145abc59a0d39c8c8f24ad837d4584e664f`.
- Upstream ahead: 28 commits through
  `d4a712abf3880dfbd3daeac5daac1bd4bfb39b6f`
  (`v0.4.38-3-gd4a712abf`).
- Local unique commits: 250.
- Predicted and actual conflict files: 14.
- Upstream merge commit: `02378bc1380799195363170d871fe89437be17d4`.
- Fork-remote reconciliation: not performed; `origin/main` still pointed at
  the downstream start during this local sync.

The conflicts were:

| Class | Conflict paths |
| --- | --- |
| Mobile Inbox and API | `apps/mobile/app/(app)/[workspace]/(tabs)/inbox.tsx`, `apps/mobile/components/inbox/detail-label.tsx`, `apps/mobile/data/api.ts` |
| Core contracts | `packages/core/modals/index.ts`, `packages/core/types/inbox.ts` |
| Shared Views | `packages/views/common/actor-avatar.tsx`, `packages/views/inbox/components/inbox-detail-label.tsx`, `packages/views/inbox/components/inbox-page.tsx`, `packages/views/issues/components/comment-card.tsx` |
| Inbox locales | `packages/views/locales/en/inbox.json`, `packages/views/locales/ja/inbox.json`, `packages/views/locales/ko/inbox.json`, `packages/views/locales/zh-Hans/inbox.json` |
| Migration runner | `server/cmd/migrate/main.go` |

The Inbox resolution keeps downstream Room identity, deduplication, and review
navigation beside upstream issue-less autopilot notices and quota recovery.
Mobile routes both behaviors through one navigation helper, with regression
tests for Room context, ordinary issues, and issue-less notices. The API keeps
the downstream appearance preference transport while adopting upstream's
canonical `getConfig` method and workspace subscription summary. Core and
Views retain the downstream quick-create command and accessible avatar profile
label while adopting upstream issue-limit recovery types, departed-actor
identity, localized dates, and quota notice UI.

Both published histories use migration prefixes 444 through 449. Upstream owns
comment recovery, delegated-failure and issue-property indexes, autopilot quota
notification, and trigger-creator identities. Downstream owns the Room turn,
lifecycle, synthesis, capability, and recommendation identities. Every
filename identity remains unchanged, and `migrations_lint_test.go` freezes the
exact duplicate pairs. The migration runner keeps both concurrent-index cleanup
registries and treats the optional `pg_bigm` property index as a recorded no-op
when its operator class is unavailable.

A fresh database reached 640 migration identities and made a second `migrate
up` a no-op. A clone of the pre-merge downstream database advanced from 634 to
640, retained all twelve 444-449 identities, created the upstream columns and
delegated-failure index, retained the Room tables and indexes, reported no
invalid or unready indexes, and also made a second migration run a no-op. The
clone was removed after verification; the source database was not modified.
`sqlc` 1.31.1 regeneration produced no diff.

Frontend verification passed all six test tasks; Views passed 481 files and
5,443 tests, and the Office asset contract passed all seven checks. Typecheck
passed 9/9 tasks, lint passed with zero errors (32 existing non-Mobile warnings
and seven existing Mobile warnings), and production build passed 5/5 tasks.
Database-backed Go verification passed the migration, Room, service, and
handler packages. The wider `make test` race run passed the remaining packages
except four daemon executable-discovery tests and three repo-cache fixture
tests; the same seven failures reproduce on the pre-merge downstream start.
The repo-cache fixtures reuse all-zero UUID tails even though branch identity
uses the random UUID tail. The full run also hit one 150 ms daemon polling
timeout under load; the daemon package's isolated rerun did not reproduce it.
This sync did not run deployment, production, Desktop display, Mobile device,
or human acceptance checks.

## 2026-09-01 v0.4.37 Sync

- Downstream start: `1f08aaa508b50d6fb8494480746e21b63aac6070`.
- Merge base: `64ec7f54163d918d5d7fd4dcae857f241b7842d0`.
- Upstream ahead: 30 commits through
  `11861145abc59a0d39c8c8f24ad837d4584e664f`
  (`v0.4.37-2-g11861145a`).
- Local unique commits: 205.
- Predicted and actual conflict files: 5.
- Upstream merge commit: `43cc9a99914ee0344dfffe2d47fd61138b1c48d0`.
- Fork-remote reconciliation: 8 commits, no textual conflicts, merge
  `949b9d417ca11f75dd974e88e6258a2922b560f7`.

The five conflicts were `packages/views/package.json`, `pnpm-lock.yaml`,
`server/cmd/migrate/main.go`, `server/cmd/server/main.go`, and
`server/internal/metrics/registry.go`. The package manifest keeps the
downstream Vitest coverage dependency and the upstream parser and ESLint
changes. A YAML-aware reconciliation script rebuilt the lockfile closure from
the merged workspace manifests. The resulting lockfile passed both an offline
frozen lockfile install and a full frozen install from the local package store.

The server resolution keeps upstream WeCom relay registration and graceful
shutdown. It also keeps the downstream Skill Evolution jobs, Room listeners,
and extra metric collectors. Upstream retired the old business-sampler and
seat-capacity collectors, so the merge does not restore them. The migration
cleanup registry contains both histories.

Both published migration histories use prefixes 441 through 443. Upstream owns
`runtime_profile_add_codearts`, `vcs_reference_only_repair`, and
`issue_project_status_index`. Downstream owns
`twin_deposition_edit_digest`, `twin_proposal_correction`, and
`twin_proposal_replacement_index`. Every filename identity remains unchanged,
and `migrations_lint_test.go` freezes the exact duplicate sets. A fresh
database and a clone of the existing downstream database both reached 634
migration identities after fork migration 527. Both histories produced the
same version set, retained all six 441-443 identities, created the expected
upstream and downstream indexes, and made a second `migrate up` a no-op.

Upstream also replaced direct runtime reads with `service.RuntimeLookup`.
Rooms now receive a required `AgentRuntimeLookupFactory` that binds the lookup
to the exact operation transaction. This preserves upstream authorization and
metrics without adding a Room-to-service import cycle or falling back to a
pool query outside the transaction. Source tests reject new direct
`GetAgentRuntime` calls in the Room package.

Validation found one additional frontend integration defect: the Wiki path
field still had a hard-coded `index.md` placeholder. The placeholder now uses
the Wiki locale contract in English, Japanese, Korean, and Simplified Chinese.
The final TypeScript tests passed all 6 workspace tasks, including 472 Views
files and 5,358 Views tests. Typecheck passed 9/9 tasks, lint passed 6/6 with
zero errors, and production build passed 5/5 tasks. Targeted Go verification
passed the migration, Room, service, metrics, handler, server, and Skill
Evolution packages. `sqlc` 1.31.1 regeneration produced no diff. This sync did
not run deployment, production, Desktop display, or human acceptance checks.

## 2026-08-29 Skill Evolution Final-Fix Audit

- Audited upstream tip: `64ec7f54163d918d5d7fd4dcae857f241b7842d0`.
- Implementation base: `07d32c6b78c3ef9fb9edd1d5f24c9f1b6096917d`.
- Audited implementation commit: `5af50d15e1e04c908e964b1f4c51be112271bd75`.
- Audited implementation tree: `9fa37a8a23fbadb9f4d0ef22d10dc69059945406`.
- Textual conflicts: 0. The audited upstream tip is an ancestor of the
  implementation base, and the final-fix tree has no unmerged paths or
  conflict markers.

The migration audit preserved every published identity from 482 through 525.
Follow-up migration 525 adds the durable exact-feedback marker, makes legacy
attributions without dispatch proof ineligible, and moves the storage boolean
from `enabled` to `is_enabled` while keeping the JSON DTO field `enabled`.
Migration 526 adds the content-free synthesis/held-out evidence provenance
role without adding an index or changing any published migration identity.
Migrations 525-526 create no index; all existing indexes remain in their original
single-statement concurrent migrations. Existing-514 and fresh ledgers both
advanced through 526, crash-window replay succeeded, a second full `migrate
up` was a no-op, and PostgreSQL reported no invalid indexes. The fresh ledger
retained the published 482, 515, 524, and 525 entries unchanged.

`sqlc` 1.31.1 was generated twice from the migration/query sources. Both runs
produced the same generated-tree digest,
`284826c729da105f81bd444e1942937dbf2f65f54ad0dbb528b04f16a73127e9`.
The path audit against the implementation base found changes only in the
registered Skill Evolution/task-review leaves and the shared exceptions listed
in `docs/downstream/skill-evolution/ownership.md`; the ownership and workspace
deletion gates passed.

The shared-transcript blocker is resolved in this tree. Task-review form,
state, validation, query, and mutation composition now live under
`packages/views/task-run-reviews/`. The shared transcript dialog exposes only
an opaque slot, and the transcript button contains only the leaf import and
slot registration. A source-based gate rejects task-review policy symbols in
the dialog/button and rejects any unregistered shared view path, without using
a brittle total-line-count threshold.

## 2026-08-28 v0.4.36 Sync

- Downstream start: `d3d9359d3`.
- Merge base: `d06b6b6e5`.
- Upstream ahead: 13 commits through `64ec7f541`
  (`v0.4.36-2-g64ec7f541`).
- Local unique commits: 59.
- Textual conflict files: 0.
- Merge commit: `0a940fd8a`.
- Fork-remote reconciliation: none; `origin/main` was already an ancestor of
  the downstream start.

This update brought in the two-hour inactivity and tool budgets, live-end chat
scrolling, issue-property filters, native OMP MCP configuration, public web
URLs for desktop navigation, GitHub pull-request head-SHA lookup, CLI
pagination documentation, and several daemon/service cleanup fixes. The Git
merge was textual-conflict free, but validation found two semantic integration
points that still required downstream work.

First, both published histories used migration prefix 440:
`440_github_pr_head_sha_index` upstream and
`440_twin_deposition_request_index` downstream. Both filename identities stay
unchanged so existing installations do not re-run either migration. The lint
ledger freezes the exact pair and continues to reject any new unpublished
collision. The existing downstream database advanced from 595 to 596 migration
identities, retained the downstream 440, applied the upstream 440 and its
index, and made a second `migrate up` a no-op. A temporary empty database
applied all 585 current identities, recorded both 440 stems, created the index,
and also made the second run a no-op before the database was removed.

Second, upstream made `hash` part of the required `NavigationAdapter`
location contract. Two downstream-only issue-detail test adapters still
modeled the old boundary. Adding the empty fragment to those fixtures restored
the shared contract without changing production behavior. This is why a
conflict-free merge must still run workspace typechecking: downstream-only
consumers cannot appear in Git's same-line conflict report.

Targeted frontend verification passed 16 files and 200 tests across core,
views, web, and desktop. Targeted Go verification passed the migration lint,
timeout sweeper, watchdog, OMP MCP, repository-cache, property-facet, GitHub
snapshot, and channel-media reconciliation paths. `sqlc` 1.31.1 regeneration
produced no diff. Typecheck passed 7/7 workspace tasks; lint passed 6/6 with
zero errors and 34 pre-existing warnings.

## 2026-08-28 Sync

- Downstream start: `5d6feabfe`.
- Merge base: `3d37828e9`.
- Upstream ahead: 6 commits through `d06b6b6e5`
  (`v0.4.35-16-gd06b6b6e5`).
- Local unique commits: 53.
- Upstream conflict files: 0.
- Upstream merge commit: `9d8f04c4b`.
- Fork-remote reconciliation: 3 unique commits, one conflict, merge
  `aa0dedf41`.

The upstream changes covered archived-runtime garbage collection, non-ASCII
issue-status keys, OpenClaw and agent process-tree ownership, pricing tag
normalization, and a stable working directory for repo-cache Git commands.
The merge was textual-conflict free. Upstream changed `runtime.sql` and its
generated Go output but no migrations; regenerating with `sqlc` 1.31.1
produced no diff, so the two migration-history upgrade exercise was not
required for this sync.

`origin/main` still had three published Fork features that were not in the
local branch: create an issue from a project menu, quote a comment in a reply,
and copy issue identifiers. Its only conflict was
`packages/views/issues/components/comment-card.tsx`. The local side registered
"create source sub-issue" actions in both root and nested comment menus while
the Fork side registered "quote in reply" in the same two positions. The
resolution retained both actions, both icons, and the complete quote insertion
path. Comparing the result to each parent showed only the other side's intended
feature delta.

Targeted frontend verification passed 179 core tests, 22 runtime-view tests,
and 132 Fork/source-context view tests. Typecheck passed 7/7 workspace tasks;
lint passed 6/6 with zero errors and existing warnings. Go verification passed
the full issue-status, metrics, and repo-cache packages, the previously failing
agent process-lifecycle cases, and targeted runtime-GC, runtime teardown, and
non-Latin status handler tests against the existing local PostgreSQL database.
The execenv suite's unreadable-file case fails when tests run as container root
because root can read the fixture; the exact case passed as UID 1000.

Process-tree tests need a real container init and sandbox identity. Run their
Go container with `--init` and `IS_SANDBOX=1`; run permission-denial fixtures as
a non-root UID. Without init, unreaped orphan descendants can look like cleanup
regressions and turn bounded cancellation tests into multi-second timeouts.

## 2026-08-27 Sync

- Downstream start: `f56235422`.
- Merge base: `3c4288dde`.
- Upstream ahead: 54 commits through `3d37828e9`
  (`v0.4.35-10-g3d37828e9`).
- Local unique commits: 47.
- Upstream conflict files: 18.
- Merge commit: `b53ba948b`.
- Fork-remote reconciliation: `af311c57a` (one additional conflict).

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

After the upstream merge, `origin/main` still contained PR #5's copyable Agent
mention feature on two commits that were not ancestors of the local branch.
Merging the fork remote preserved that already-published work. Its only
conflict was `packages/views/agents/agents-i18n-parity.test.ts`: upstream had
added task-failure, visual-label, and diagnostics parity contracts while the
fork feature had added mention-action parity. These suites describe independent
behavior, so the resolution retained all of them rather than choosing a whole
side. The merged i18n, mention-menu, and agent-detail tests passed 33/33.

Treat the fork remote as a second reconciliation boundary: after merging
`upstream/main`, compare the result with `origin/main` before declaring the
checkout synchronized. An upstream ancestor check alone can still leave
published fork commits absent from local `main`.

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

1. `git fetch upstream main --tags`, record both tips and divergence, then run
   `.agents/skills/multica-upstream-sync/scripts/preview-sync.sh`. Use its
   overlap list to audit clean auto-merges as well as textual conflicts.
2. Merge the recorded SHA with
   `git merge --no-ff --no-commit <upstream-sha>`. Validate before committing,
   and do not rebase published `main`.
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
11. **Audit clean auto-merges as semantic conflicts.** Review every path changed
    on both sides, then search callers, mocks, and fixtures for retired symbols.
    PR #17's obsolete `TaskEnqueuer.LookupRoomRuntime` API auto-merged even
    though current main had replaced it with transaction-bound
    `AgentRuntimeLookupFactory`. A clean index would have preserved both
    architectures.
12. **Treat version changes as a consistency graph.** A Node, Go, Expo,
    Electron, or framework bump must update manifests, lockfiles, CI, Docker
    builders, local version files, developer checks, and maintained docs. Derive
    a consistency check from manifests and invoke it directly from CI. Do not
    match explanatory prose that can change without changing the toolchain.
13. **Propagate a sync through exact PR heads.** Merge the synchronized
    `origin/main` into each older in-flight PR with a merge commit. Preserve
    review history. Do not rebase or force-push old shared branches. After each
    preceding PR merges, fetch again and rerun `merge-tree` against the next
    exact head. If the result is clean, keep the head unchanged. If it
    conflicts, merge the new main and repeat the checks. Wait for the host's
    mergeability state, and lock an authorized merge to the verified head SHA.
14. **Resolve locale bundles structurally.** Parse the JSON, retain independent
    keys from both histories, apply the current glossary, and run the locale
    parity suite. A textual union can produce valid JSON while dropping a key
    in one of the four maintained locales.

## Local Ownership Map

Use this to decide who wins a conflict:

| Owner | Paths |
| --- | --- |
| Local Rooms | `server/internal/room/`, `server/pkg/db/queries/room.sql`, `packages/core/rooms/`, `packages/views/rooms/`, `apps/*/rooms` |
| Local Twin / Wiki | `server/internal/service/twin*`, `lm_wiki*`, `wiki*`, `packages/core/twins/`, `packages/core/wiki/`, `packages/views/twins/`, `packages/views/wiki/` |
| Local skins / search extras | theme tokens, `data-twin-copy`, search commands `copy_page_link` / `surprise_issue` |
| Local ops overlay | `downstream/` (LAN self-host scripts, compose bind override, extra docs). Do not patch upstream `Makefile` / `docker-compose.selfhost.yml` for this. |
| Upstream | onboarding shell, auth recovery, plugins, share links, custom issue statuses, chat/task event contract, sqlc output |

When a conflict is inside an upstream-owned shell, take upstream and
re-attach the local registration. When it is inside a local-owned
module, keep local and replay any upstream type or helper rename.
