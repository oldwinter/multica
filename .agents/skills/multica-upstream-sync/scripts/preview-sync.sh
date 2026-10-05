#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "preview-sync: $*" >&2
  exit 2
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" ||
  die "run this command inside a Git worktree"
cd "$repo_root"

downstream_ref="${1:-main}"
upstream_ref="${2:-upstream/main}"
fork_ref="${3:-origin/main}"

resolve_commit() {
  local ref="$1"
  git rev-parse --verify "${ref}^{commit}" 2>/dev/null ||
    die "cannot resolve commit ref ${ref}"
}

print_file() {
  local heading="$1"
  local path="$2"
  printf '\n[%s]\n' "$heading"
  if [[ ! -s "$path" ]]; then
    printf '(none)\n'
    return
  fi
  command cat "$path"
}

downstream_sha="$(resolve_commit "$downstream_ref")"
upstream_sha="$(resolve_commit "$upstream_ref")"
fork_sha="$(resolve_commit "$fork_ref")"
merge_base="$(git merge-base "$downstream_sha" "$upstream_sha")"
read -r downstream_only upstream_only < <(
  git rev-list --left-right --count "$downstream_sha...$upstream_sha"
)
read -r local_only fork_only < <(
  git rev-list --left-right --count "$downstream_sha...$fork_sha"
)

preview_dir="$(mktemp -d "${TMPDIR:-/tmp}/multica-sync-preview.XXXXXX")" ||
  die "cannot create the preview directory"
worktree_status_file="$preview_dir/worktree-status"
downstream_paths_file="$preview_dir/downstream-paths"
upstream_paths_file="$preview_dir/upstream-paths"
overlap_paths_file="$preview_dir/overlap-paths"

cleanup() {
  rm -f -- \
    "$worktree_status_file" \
    "$downstream_paths_file" \
    "$upstream_paths_file" \
    "$overlap_paths_file"
  rmdir "$preview_dir" 2>/dev/null || true
}
trap cleanup EXIT HUP INT TERM

git status --short >"$worktree_status_file"
git diff --name-only "$merge_base..$downstream_sha" |
  LC_ALL=C sort -u >"$downstream_paths_file"
git diff --name-only "$merge_base..$upstream_sha" |
  LC_ALL=C sort -u >"$upstream_paths_file"
comm -12 "$downstream_paths_file" "$upstream_paths_file" >"$overlap_paths_file"

downstream_path_count="$(wc -l <"$downstream_paths_file" | tr -d ' ')"
upstream_path_count="$(wc -l <"$upstream_paths_file" | tr -d ' ')"
overlap_path_count="$(wc -l <"$overlap_paths_file" | tr -d ' ')"

printf '[refs]\n'
printf 'repo_root=%s\n' "$repo_root"
printf 'branch=%s\n' "$(git symbolic-ref --short -q HEAD || printf '(detached)')"
printf 'downstream_ref=%s\n' "$downstream_ref"
printf 'downstream_sha=%s\n' "$downstream_sha"
printf 'fork_ref=%s\n' "$fork_ref"
printf 'fork_sha=%s\n' "$fork_sha"
printf 'upstream_ref=%s\n' "$upstream_ref"
printf 'upstream_sha=%s\n' "$upstream_sha"
printf 'upstream_describe=%s\n' "$(git describe --tags --always "$upstream_sha")"
printf 'merge_base=%s\n' "$merge_base"
printf 'downstream_only=%s\n' "$downstream_only"
printf 'upstream_only=%s\n' "$upstream_only"
printf 'local_only_vs_fork=%s\n' "$local_only"
printf 'fork_only_vs_local=%s\n' "$fork_only"
printf 'downstream_changed_paths=%s\n' "$downstream_path_count"
printf 'upstream_changed_paths=%s\n' "$upstream_path_count"
printf 'overlap_paths=%s\n' "$overlap_path_count"

print_file "worktree_status" "$worktree_status_file"
print_file "overlap_paths" "$overlap_paths_file"

printf '\n[downstream_migration_changes]\n'
git diff --name-status "$merge_base..$downstream_sha" -- \
  server/migrations server/cmd/migrate || true
printf '\n[upstream_migration_changes]\n'
git diff --name-status "$merge_base..$upstream_sha" -- \
  server/migrations server/cmd/migrate || true

set +e
merge_tree_output="$(
  git merge-tree --write-tree --name-only "$downstream_sha" "$upstream_sha" 2>&1
)"
merge_tree_status=$?
set -e

printf '\n[merge_tree]\n'
case "$merge_tree_status" in
  0) printf 'status=clean\n' ;;
  1) printf 'status=conflicts\n' ;;
  *)
    printf '%s\n' "$merge_tree_output" >&2
    die "git merge-tree failed with status ${merge_tree_status}"
    ;;
esac
printf '%s\n' "$merge_tree_output"
