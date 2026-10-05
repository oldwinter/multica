#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_DIR=$(mktemp -d "${TMPDIR:-/tmp}/multica-test-go-isolated.XXXXXX")
trap 'rm -rf "$TEST_DIR"' EXIT
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/home"

cat >"$TEST_DIR/bin/docker" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >"$MULTICA_TEST_DOCKER_ARGS"
STUB
chmod +x "$TEST_DIR/bin/docker"

module_cache="$TEST_DIR/job-module-cache"
build_cache="$TEST_DIR/job-build-cache"
PATH="$TEST_DIR/bin:/usr/bin:/bin" \
HOME="$TEST_DIR/home" \
GOMODCACHE="$module_cache" \
GOCACHE="$build_cache" \
GO_TEST_NETWORK=test-network \
MULTICA_TEST_DOCKER_ARGS="$TEST_DIR/docker.args" \
bash "$ROOT_DIR/scripts/test-go-isolated.sh" --only regular

grep -Fq -- "-v $module_cache:/gomodcache" "$TEST_DIR/docker.args"
grep -Fq -- "-v $build_cache:/gocache" "$TEST_DIR/docker.args"
[[ -d "$module_cache" && -d "$build_cache" ]]

echo "test-go-isolated.sh tests passed"
