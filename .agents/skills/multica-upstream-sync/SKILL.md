---
name: multica-upstream-sync
description: Synchronize oldwinter/multica with multica-ai/multica upstream/main, including divergence preview, ownership-aware conflict resolution, migration-ledger compatibility, generated-file regeneration, and merge validation. Use in this repository when asked to sync or merge upstream or resolve conflicts caused by upstream/main; do not use for ordinary branch merges.
---

# Multica Upstream Sync

Merge the published downstream history. Never rebase it.

Before acting, read the root `CLAUDE.md` and the complete
[upstream sync history](../../../docs/downstream/upstream-sync.md). That page is
the authority for path ownership, generated files, migration history, and prior
conflict decisions.

## 1. Establish The Baseline

Confirm the repository root, current worktree, remotes, current branch, dirty
state, and `main` versus `origin/main`. Preserve unrelated changes. A dirty
`main`, unexpected remote, detached HEAD, or divergent unpublished work must be
accounted for before the merge starts.

Record the starting `main`, upstream tip, merge base, tags, and divergence:

```bash
git fetch upstream main --tags
git rev-parse main upstream/main
git merge-base main upstream/main
git rev-list --left-right --count main...upstream/main
git describe --tags --always upstream/main
```

Completion criterion: the exact downstream start and fetched upstream tip are
known, and the intended merge cannot consume unrelated work.

## 2. Preview The Merge

Run the merge machinery without changing the index or worktree:

```bash
git merge-tree --write-tree main upstream/main
```

A non-zero exit is expected when conflicts are predicted. Inspect its staged
entries and auto-merge messages, then compare paths changed on both sides. Do
not infer the final conflict count from auto-merge output; capture the unique
unmerged paths immediately after the real merge.

Preview migration history separately. Git can merge two migration sequences
without a text conflict even though the resulting upgrade path is invalid:

```bash
merge_base=$(git merge-base main upstream/main)
git diff --name-status "$merge_base"..main -- server/migrations server/cmd/migrate
git diff --name-status "$merge_base"..upstream/main -- server/migrations server/cmd/migrate
```

The ledger key is the complete migration filename stem. Treat every migration
that may have reached a database as immutable: never rename it to solve a
numeric-prefix collision. A duplicate numeric prefix is safer than replaying
published DDL; freeze the exact duplicate stem set in the migration lint.

Completion criterion: likely conflict paths and their ownership classes are
known before editing begins, and both migration sequences have been compared by
full identity rather than numeric prefix alone.

## 3. Merge And Resolve

```bash
git merge --no-ff --no-edit upstream/main
git diff --name-only --diff-filter=U | sort -u
```

If conflicts exist, use the `resolving-merge-conflicts` skill. For each path,
inspect all three primary sources and the commits that created both intents:

```bash
git show :1:<path>  # merge base
git show :2:<path>  # downstream
git show :3:<path>  # fetched upstream
git log --all --oneline -- <path>
```

Resolve by lifecycle and ownership, not by textual union:

- In upstream-owned shells and registries, accept upstream rewrites or retired
  behavior, then reattach only the still-live downstream registration.
- In downstream-owned Rooms, Twin, Wiki, and named-skin leaves, preserve the
  local behavior while adopting upstream type and helper changes.
- When upstream retires a rollout flag beside a local flag, remove the retired
  lifecycle completely and preserve the independent local flag. Keeping the
  whole downstream block can resurrect obsolete behavior.
- Resolve SQL query sources before running `make sqlc`. Regenerate lockfiles and
  reserved-slug output from their sources rather than merging generated output.
- Accept upstream deletion of an upstream-owned surface; move a still-required
  local hook to the new extension point.
- If an already-published migration was renamed in an earlier release, add an
  explicit current-to-legacy identity mapping in the migrator only after
  proving the `up.sql` contents are identical. Reconcile the ledger without
  executing SQL; do not scatter `IF NOT EXISTS` across DDL to hide the replay.
- Do not assume identical current and renamed migration files prove every
  deployed database has the same schema. Inspect the blob from the actual
  deployed commit: a migration may have been edited after its first release.
  If a published migration gained DDL in place, keep it immutable from now on
  and add a new forward compatibility migration for the missing schema. Never
  mark that repair as applied through an identity alias.

Completion criterion: every hunk has a source-backed intent, with no invented
compatibility behavior.

## 4. Prove Each Resolution

Before committing, compare every resolved path with both parents:

```bash
git diff upstream/main -- <path>
git diff ORIG_HEAD -- <path>
git diff --name-only --diff-filter=U
git diff --check
```

The upstream comparison should show only the deliberate downstream delta. The
pre-merge comparison should show the intended upstream change without losing
the local feature. Search tracked files for conflict markers and run the
narrowest tests covering each resolution, followed by the broader checks
required by the touched surfaces.

When either side changes migrations, test two database histories:

1. A fresh database, which proves the merged migration sequence builds.
2. A representative database already migrated by the pre-merge downstream,
   which proves published ledger identities upgrade without replaying DDL.

```bash
(cd server && go run ./cmd/migrate up)
(cd server && go test ./cmd/migrate ./internal/migrations -count=1)
```

If the pre-merge database is unavailable, create a hermetic test that seeds its
exact `schema_migrations.version` values and the corresponding schema objects.
Do not claim upgrade compatibility from a fresh-database run alone. Run
`migrate up` a second time to prove the reconciled ledger is idempotent.
Compare the resulting columns, constraints, and indexes required by current
code, not only the migration ledger. A successful migrator exit can still hide
schema drift when an old migration file was modified after publication.

Finish the merge non-interactively and prove its topology:

```bash
git commit --no-edit
git merge-base --is-ancestor upstream/main HEAD
git show -s --format='%H%n%P%n%s' HEAD
```

Completion criterion: there are no unmerged paths or markers, relevant checks
are green, fresh and existing database paths are green when migrations changed,
the merge has two expected parents, and upstream is an ancestor.

## 5. Preserve The Learning

Update `docs/downstream/upstream-sync.md` with the date, exact SHAs, divergence,
actual conflict count, conflict paths, semantic decision, generated artifacts,
and commands actually verified. Update the ownership map or durable rules only
when the merge produced a genuinely reusable lesson.

Report the merge commit, upstream version/SHA, conflict resolution, validation,
and local ahead/behind state. A request to fetch, merge, or resolve conflicts
does not authorize pushing, opening a PR, tagging, or releasing; perform those
only when explicitly requested.
