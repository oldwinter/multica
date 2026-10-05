#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SCRIPT="$SCRIPT_DIR/next-downstream-tag.sh"
TEST_DIR=$(mktemp -d "${TMPDIR:-/tmp}/multica-next-downstream-tag.XXXXXX")

cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

expect_eq() {
  local got="$1" want="$2" label="$3"
  if [[ "$got" != "$want" ]]; then
    fail "$label: got '$got', want '$want'"
  fi
}

run_script() {
  (cd "$TEST_DIR" && bash "$SCRIPT" "$@")
}

git -C "$TEST_DIR" init -q -b main
git -C "$TEST_DIR" config user.email "test@example.com"
git -C "$TEST_DIR" config user.name "test"

echo one >"$TEST_DIR/file"
git -C "$TEST_DIR" add file
git -C "$TEST_DIR" commit -q -m "one"

if got=$(run_script 2>"$TEST_DIR/err"); then
  fail "should reject a history with no stable semver tag (got '$got')"
fi
if ! grep -q "no stable vX.Y.Z tag" "$TEST_DIR/err"; then
  fail "missing stable-tag error: $(cat "$TEST_DIR/err")"
fi

git -C "$TEST_DIR" tag v0.4.32
git -C "$TEST_DIR" tag v0.4.32-oldwinter.2
git -C "$TEST_DIR" tag v0.4.36

echo two >"$TEST_DIR/file"
git -C "$TEST_DIR" add file
git -C "$TEST_DIR" commit -q -m "two"

got=$(run_script)
expect_eq "$got" "v0.4.36-oldwinter.1" "first downstream tag after newer upstream base"

git -C "$TEST_DIR" tag v0.4.36-oldwinter.1
git -C "$TEST_DIR" tag v0.4.36-oldwinter.9
git -C "$TEST_DIR" tag v0.4.36-oldwinter.10

echo three >"$TEST_DIR/file"
git -C "$TEST_DIR" add file
git -C "$TEST_DIR" commit -q -m "three"

got=$(run_script)
expect_eq "$got" "v0.4.36-oldwinter.11" "numeric suffix increment across .9 and .10"

git -C "$TEST_DIR" tag v0.4.36-oldwinter.11
got=$(run_script)
expect_eq "$got" "v0.4.36-oldwinter.11" "HEAD already tagged stays idempotent"

echo "next-downstream-tag tests passed"
