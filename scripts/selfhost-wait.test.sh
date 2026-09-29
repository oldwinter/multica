#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_DIR=$(mktemp -d "${TMPDIR:-/tmp}/multica-selfhost-wait.XXXXXX")
trap 'rm -rf "$TEST_DIR"' EXIT
mkdir -p "$TEST_DIR/bin"

cat >"$TEST_DIR/bin/docker" <<'STUB'
#!/usr/bin/env bash
if [[ "$*" == *" port backend 8080"* ]]; then
  echo 127.0.0.1:18080
elif [[ "$*" == *" port frontend 3000"* ]]; then
  echo 127.0.0.1:13000
fi
STUB
chmod +x "$TEST_DIR/bin/docker"

cat >"$TEST_DIR/bin/curl" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$SELFHOST_TEST_CURL_LOG"
[[ "${SELFHOST_TEST_HEALTHY:-0}" == 1 ]]
STUB
chmod +x "$TEST_DIR/bin/curl"

export PATH="$TEST_DIR/bin:/usr/bin:/bin"
export SELFHOST_TEST_CURL_LOG="$TEST_DIR/curl.log"
export SELFHOST_WAIT_ATTEMPTS=2
export SELFHOST_WAIT_INTERVAL_SECONDS=0
export SELFHOST_HEALTH_MAX_TIME_SECONDS=3

if SELFHOST_TEST_HEALTHY=0 bash "$ROOT_DIR/scripts/selfhost-wait.sh" >"$TEST_DIR/out" 2>"$TEST_DIR/err"; then
  echo "unhealthy self-host wait returned success" >&2
  exit 1
fi
if grep -q "Multica is running" "$TEST_DIR/out"; then
  echo "unhealthy self-host wait printed success" >&2
  exit 1
fi
if ! grep -q -- '--connect-timeout 3 --max-time 3' "$TEST_DIR/curl.log"; then
  echo "health probes omitted timeout flags" >&2
  cat "$TEST_DIR/curl.log" >&2
  exit 1
fi

: >"$TEST_DIR/curl.log"
SELFHOST_TEST_HEALTHY=1 bash "$ROOT_DIR/scripts/selfhost-wait.sh" >"$TEST_DIR/out"
grep -q "Multica is running" "$TEST_DIR/out"
grep -q "Frontend: http://localhost:13000" "$TEST_DIR/out"
grep -q "Backend:  http://localhost:18080" "$TEST_DIR/out"

echo "selfhost-wait.sh tests passed"
