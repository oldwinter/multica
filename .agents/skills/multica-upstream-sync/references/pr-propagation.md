# Propagate a sync to in-flight PRs

Use this mode only when the request includes PRs affected by the new `main`.
Record each PR's exact base SHA, head SHA, merge state, checks, and changed
paths. Compare changed-path overlap to choose an order. Use one isolated
worktree per PR.

Merge the synchronized `origin/main` into each PR head that predates the sync.
Preserve review history with a merge commit. Do not rebase or force-push an old
shared PR branch. Resolve manifests at their sources and regenerate the
lockfile. Audit auto-merged shared hubs for obsolete APIs just as in the
upstream merge.

After pushing a resolved head, wait for the host to recompute mergeability and
CI. An interim `UNKNOWN` state is not a failure. Immediately before an
authorized merge, fetch again and prove the exact pair locally:

```bash
main_sha=$(git rev-parse origin/main)
git merge-tree --write-tree "$main_sha" <pr-head-sha>
gh pr merge <number> --merge --match-head-commit <pr-head-sha>
```

After one PR merges, repeat the exact-ref preview for the next PR against the
new `origin/main`. A prior clean result is stale. If the new merge tree is
clean, keep the existing PR head and avoid another branch merge. If it
conflicts, merge the new `origin/main` into the PR and repeat its checks. Push
or merge a PR only when the user authorized that external action.

Completion criterion: each PR contains the synchronized baseline, its exact
head merges cleanly with the then-current `origin/main`, required checks pass,
and any merge is locked to the verified head SHA.
