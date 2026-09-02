#!/usr/bin/env bash
# Print the oldwinter downstream release tag that HEAD should carry.
#
# If HEAD already has vX.Y.Z-<suffix>.N, print that tag so callers can stay
# idempotent. Otherwise take the newest stable vX.Y.Z ancestor and increment
# the matching suffix counter (v0.4.36-oldwinter.1, then .2, ...).
set -euo pipefail

suffix="${DOWNSTREAM_TAG_SUFFIX:-oldwinter}"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not a git repository" >&2
  exit 1
fi

existing="$(git tag --points-at HEAD | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+-${suffix}\.[0-9]+$" | sort -V | tail -1 || true)"
if [[ -n "$existing" ]]; then
  printf '%s\n' "$existing"
  exit 0
fi

base="$(git tag --merged HEAD | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1 || true)"
if [[ -z "$base" ]]; then
  echo "no stable vX.Y.Z tag is an ancestor of HEAD" >&2
  exit 1
fi

max=0
while IFS= read -r tagged; do
  [[ -z "$tagged" ]] && continue
  n="${tagged##*.}"
  if [[ "$n" =~ ^[0-9]+$ ]] && ((n > max)); then
    max=$n
  fi
done < <(git tag --list "${base}-${suffix}.*")

printf '%s\n' "${base}-${suffix}.$((max + 1))"
