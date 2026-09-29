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

git -C "$TEST_DIR" tag v0.4.36-oldXwinter.7
got=$(DOWNSTREAM_TAG_SUFFIX=old.winter run_script)
expect_eq "$got" "v0.4.36-old.winter.1" "dot in suffix is matched literally"

for invalid in '.hidden' 'bad..suffix' 'trailing.' 'slash/name' 'regex+'; do
  if DOWNSTREAM_TAG_SUFFIX="$invalid" run_script >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
    fail "invalid suffix '$invalid' was accepted"
  fi
  if ! grep -q "invalid downstream tag suffix" "$TEST_DIR/err"; then
    fail "invalid suffix '$invalid' did not report validation error"
  fi
done

echo "next-downstream-tag tests passed"
