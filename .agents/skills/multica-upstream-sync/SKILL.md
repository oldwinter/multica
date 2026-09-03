---
name: multica-upstream-sync
description: Plan downstream features away from upstream hotspots, synchronize oldwinter/multica with multica-ai/multica upstream/main, and reconcile PR conflicts created by that sync. Use before adding downstream behavior beside upstream, merging upstream/main, or updating in-flight PRs after an upstream sync. Do not use for unrelated branch merges.
---

# Multica upstream sync

Keep downstream behavior in owned leaves. Merge published history without a
rebase.

Before acting, read the root `CLAUDE.md`. In the
[upstream sync history](../../../docs/downstream/upstream-sync.md), always read
`Rules That Prevent The Next 40-File Merge` and `Local Ownership Map`. For a
sync, also read the latest two sync records and older entries that match the
overlapping paths or migration prefixes. For PR propagation, read the latest
PR reconciliation and its corresponding sync record.

Choose the modes that match the request:

- **Placement mode** applies before a downstream feature chooses its files.
  Read [feature placement](references/feature-placement.md) and stop there
  unless the request also includes a sync.
- **Sync mode** merges `upstream/main` into downstream `main`.
- **PR propagation mode** updates in-flight PRs after the sync changes `main`.
  Read [PR propagation](references/pr-propagation.md) after the sync merge is
  verified.

A request to plan, assess, or review does not authorize a fetch, merge,
commit, generated-file update, or documentation edit. Use cached refs for a
provisional plan, state their exact SHAs, and mark them as potentially stale.
Run the mutating steps only when the request asks to perform the sync or build
the feature.

## Establish the sync baseline

Confirm the repository root, worktree, remotes, branch, dirty state, and
`main` versus `origin/main`. Preserve unrelated changes. Account for a dirty
`main`, detached HEAD, unexpected remote, or unpublished divergence before the
merge starts.

Fetch exact refs, then run the worktree-safe preview helper:

```bash
git fetch upstream main --tags
git fetch origin main
.agents/skills/multica-upstream-sync/scripts/preview-sync.sh \
  main upstream/main origin/main
```

The helper prints exact SHAs, divergence, both-side path overlap, migration
changes, worktree status, and `git merge-tree` output. It leaves refs, the
index, and the worktree unchanged. Treat `origin/main` as a second
reconciliation boundary. An upstream ancestor check does not prove that local
`main` contains every published fork commit.

Completion criterion: the downstream start, fork tip, upstream tip, merge
base, divergence, and unrelated work are known by exact identity.

## Build the preservation ledger

Classify every overlapping path before editing. Include shared paths that Git
auto-merges, not only paths that `merge-tree` marks conflicted.

| Path or contract | Upstream intent | Downstream intent | Decision | Proof |
| --- | --- | --- | --- | --- |
| `<path>` | `<behavior>` | `<behavior>` | `upstream`, `downstream`, `both`, `superseded`, or `generated` | `<test or comparison>` |

Use `superseded` when the new upstream architecture replaces an old local API.
Use `generated` only after naming the source that will regenerate the file.
Inspect migrations separately because Git can merge two sequences cleanly
while producing an invalid upgrade path:

```bash
merge_base=$(git merge-base <downstream-sha> <upstream-sha>)
git diff --name-status "$merge_base"..<downstream-sha> -- server/migrations server/cmd/migrate
git diff --name-status "$merge_base"..<upstream-sha> -- server/migrations server/cmd/migrate
```

The migration ledger key is the complete filename stem. Keep every migration
that may have reached a database immutable. Preserve duplicate numeric
prefixes and freeze their exact stem sets in the migration lint.

Completion criterion: every overlap has an ownership decision and an explicit
proof before the worktree changes.

## Merge and resolve

```bash
git merge --no-ff --no-commit <upstream-sha>
git diff --name-only --diff-filter=U | sort -u
```

For every conflict, inspect the merge base, downstream side, upstream side,
and the commits that created both intents:

```bash
git show :1:<path>
git show :2:<path>
git show :3:<path>
git log --all --oneline -- <path>
```

Resolve by lifecycle and ownership:

- In upstream-owned shells and registries, keep the upstream lifecycle and
  reattach only the live downstream registration.
- In downstream-owned leaves, keep local behavior and adopt upstream type,
  helper, authorization, and transaction changes.
- Delete a local API when upstream's current architecture supersedes it. Audit
  its callers and test doubles even when Git auto-merged them.
- Resolve SQL query sources before `make sqlc`. Resolve workspace manifests
  before `pnpm install`. Never hand-merge generated Go or `pnpm-lock.yaml`.
- Parse locale JSON structurally, retain independent keys, apply the current
  glossary, and run the locale parity suite.
- Keep independent migration-runner guards from both histories. Preserve
  published filenames. Use an identity alias only for byte-identical renamed
  `up.sql` files, and add a forward migration for DDL added after publication.
- Accept an upstream deletion when upstream owns the old surface. Move a live
  downstream hook to the replacement extension point.

Completion criterion: every resolution implements the preservation ledger,
with no compatibility behavior invented from the conflict markers.

## Audit semantic conflicts

Textual conflict resolution is only the first pass. Review every path changed
on both sides, including clean auto-merges. Search for removed APIs, old import
targets, stale mocks, retired environment variables, and downstream consumers
of changed upstream contracts.

When manifests or toolchain versions changed, verify all packaging and runtime
consumers:

```bash
pnpm check:toolchain
pnpm install --frozen-lockfile
```

Confirm that the actual CI workflow invokes any new consistency checker. A
local script that CI bypasses is documentation, not a guard.

Completion criterion: every `both` and `superseded` ledger entry accounts for
auto-merged callers, fixtures, generated output, and delivery configuration.

## Prove the merged tree

Compare each resolved path with both parents:

```bash
git diff <upstream-sha> -- <path>
git diff ORIG_HEAD -- <path>
git diff --name-only --diff-filter=U
git diff --check
```

Relative to upstream, only the intended downstream delta should remain.
Relative to the pre-merge tree, the intended upstream change should remain.
Search tracked files for conflict markers. Run the narrowest tests named in
the preservation ledger, then broader checks for every touched package.

Run generators and require a clean second run. If locales changed, run
the structural parity suite:

```bash
pnpm --dir packages/views exec vitest run locales/parity.test.ts
```

If migrations changed, test both database histories:

1. Run the complete migration sequence on a fresh database.
2. Clone or seed a database with the pre-merge downstream ledger and schema.
3. Run `migrate up` twice on the existing-ledger database.
4. Inspect the columns, constraints, and indexes used by current code.

```bash
(cd server && go run ./cmd/migrate up)
(cd server && go test ./cmd/migrate ./internal/migrations -count=1)
```

A successful migrator exit does not prove schema compatibility. Exercise the
feature that reads the migrated schema. Separate new failures from failures
that reproduce at the exact downstream start.

Finish the merge non-interactively and prove its topology:

```bash
git commit --no-edit
git merge-base --is-ancestor <upstream-sha> HEAD
git show -s --format='%H%n%P%n%s' HEAD
```

Completion criterion: the index and marker scan are clean, focused and broad
checks match the touched risk, generated files are stable, both database
histories pass when required, and the merge has the expected two parents.

## Preserve the learning

Update `docs/downstream/upstream-sync.md` with the date, exact SHAs,
divergence, actual conflict count and paths, semantic decisions, regenerated
artifacts, and commands actually run. Add a durable rule only for a repeated
failure mode. Prefer a lint, parity test, generator, or CI check when the rule
can be enforced.

Report the merge commit, upstream version and SHA, conflict decisions,
validation, known baseline failures, and local ahead or behind state. Keep
local tests, CI, release, deployment, live runtime, and human acceptance as
separate claims. A sync request does not authorize pushing, opening a PR,
tagging, releasing, or merging another PR.
