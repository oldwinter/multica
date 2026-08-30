#!/usr/bin/env bash
# Registry-level behaviour of scripts/dev-env.sh, with no services started.
#
# Everything here runs against a throwaway MULTICA_DEV_HOME holding hand-written
# manifests, so the verbs are exercised end to end without a database, a
# backend, or a port.
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

export MULTICA_DEV_HOME="$tmp_dir/dev"
export MULTICA_DEV_WORKSPACES_PARENT="$tmp_dir/workspaces-parent"
export MULTICA_DEV_DESKTOP_APP_DATA="$tmp_dir/app-data"
export MULTICA_DEV_PROFILES_HOME="$tmp_dir/profiles"

fake_bin="$tmp_dir/bin"
mkdir -p "$fake_bin"
cat > "$fake_bin/psql" <<'EOF'
#!/usr/bin/env bash
case " $* " in
  *" DROP DATABASE "*) [ "${FAIL_DROP:-0}" != 1 ] ;;
  *) printf '1\n' ;;
esac
EOF

# These are registry fixtures, so their hand-written ports must remain stopped
# even when a shared runner has unrelated listeners on the same machine ports.
cat > "$fake_bin/curl" <<'EOF'
#!/usr/bin/env bash
exit 7
EOF
cat > "$fake_bin/lsof" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
cat > "$fake_bin/ss" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
cat > "$fake_bin/multica" <<'EOF'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "daemon status") printf '{"status":"stopped"}\n' ;;
  "daemon stop") exit 0 ;;
  *) echo "unexpected multica fixture command: $*" >&2; exit 64 ;;
esac
EOF
chmod +x \
  "$fake_bin/psql" \
  "$fake_bin/curl" \
  "$fake_bin/lsof" \
  "$fake_bin/ss" \
  "$fake_bin/multica"
export PATH="$fake_bin:$PATH"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

require_contains() {
  local file=$1 expected=$2
  if ! grep -Fq "$expected" "$file"; then
    echo "Expected output to contain: $expected" >&2
    echo "Observed:" >&2
    sed 's/^/  /' "$file" >&2
    exit 1
  fi
}

dev_env() {
  bash "$root_dir/scripts/dev-env.sh" "$@"
}

write_manifest() {
  local name=$1 dir=$2 offset=$3
  local profile="dev-dev-env-test-$offset"
  mkdir -p "$MULTICA_DEV_HOME/envs/$name/logs"
  cat > "$MULTICA_DEV_HOME/envs/$name/manifest.env" <<EOF
NAME=$name
DIR=$(printf '%q' "$dir")
CREATED_AT=2026-01-01T00:00:00Z
OWNER=agent
TTL_HOURS=0
ENV_FILE=.env.example
OFFSET=$offset
BACKEND_PORT=$((18080 + offset))
FRONTEND_PORT=$((13000 + offset))
DB_NAME=multica_dev_env_test_$offset
DATABASE_URL=postgres://multica:multica@localhost:5432/multica_dev_env_test_$offset?sslmode=disable
PROFILE=$profile
WORKSPACES_ROOT=$(printf '%q' "$MULTICA_DEV_WORKSPACES_PARENT/multica_workspaces_$profile")
DESKTOP_RENDERER_PORT=$((5174 + offset))
DESKTOP_APP_SUFFIX=$name
EOF
}

prepare_checkout() {
  local dir=$1
  mkdir -p "$dir/server/bin"
  cp "$fake_bin/multica" "$dir/server/bin/multica"
  chmod +x "$dir/server/bin/multica"
}

out="$tmp_dir/out"

# ---------------------------------------------------------------------------
# An empty registry is a normal state, not an error.
# ---------------------------------------------------------------------------
dev_env list > "$out" 2>&1 || fail "list on an empty registry must succeed"
require_contains "$out" "No environments registered"

dev_env list --json > "$out" 2>&1 || fail "list --json on an empty registry must succeed"
if [ "$(cat "$out")" != "[]" ]; then
  fail "list --json on an empty registry = $(cat "$out"), want []"
fi

# ---------------------------------------------------------------------------
# Manifest serialization and user-provided names are safe. A manifest is
# sourced by Bash, so values must be shell-escaped and a name must never be
# able to walk outside envs/ before destroy eventually runs rm -rf.
# ---------------------------------------------------------------------------
quoted="$tmp_dir/quoted.env"
dangerous='a path with spaces;$(touch should-not-exist)'
bash -c 'source "$1"; write_manifest_value DIR "$2"' _ "$root_dir/scripts/dev-env.sh" "$dangerous" > "$quoted"
loaded="$(bash -c 'source "$1"; printf %s "$DIR"' _ "$quoted")"
[ "$loaded" = "$dangerous" ] || fail "manifest value did not round-trip safely"
[ ! -e "$root_dir/should-not-exist" ] || fail "loading a manifest executed its value"

status=0
dev_env up --name ../../escape > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up accepted a path-traversing environment name"
require_contains "$out" "Invalid environment name"

status=0
dev_env up --ttl nope > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up accepted a non-numeric TTL"
require_contains "$out" "TTL must be a positive integer"

# Rewriting an allocated database name must preserve the existing connection
# endpoint, credentials and query parameters.
rewritten="$(bash -c 'source "$1"; database_url_with_name "$2" "$3"' _ \
  "$root_dir/scripts/dev-env.sh" \
  'postgres://dev:p%40ss@127.0.0.1:55432/old_db?sslmode=require&application_name=dev' \
  'new_db')"
[ "$rewritten" = 'postgres://dev:p%40ss@127.0.0.1:55432/new_db?sslmode=require&application_name=dev' ] \
  || fail "database URL rewrite changed more than the database name: $rewritten"

# ---------------------------------------------------------------------------
# A registered environment is visible to both renderings, and the JSON one
# parses — agents read it, so a stray log line in it is a broken contract.
# ---------------------------------------------------------------------------
prepare_checkout "$tmp_dir/checkout"
write_manifest "probe-901" "$tmp_dir/checkout" 901

dev_env list > "$out" 2>&1 || fail "list must succeed with one environment"
require_contains "$out" "probe-901"
require_contains "$out" "18981"

dev_env status probe-901 --json > "$out" 2>&1 || fail "status --json must succeed"
node -e '
  const fs = require("fs");
  const payload = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  if (payload.name !== "probe-901") throw new Error("name = " + payload.name);
  if (payload.backend_port !== 18981) throw new Error("backend_port = " + payload.backend_port);
  for (const key of ["api", "web", "daemon", "desktop"]) {
    if (!payload.components[key]) throw new Error("missing component " + key);
    if (payload.components[key].state !== "stopped") {
      throw new Error(key + " state = " + payload.components[key].state);
    }
  }
' "$out" || fail "status --json is not machine-readable"

# ---------------------------------------------------------------------------
# Stopping an environment that is not running is a no-op that SUCCEEDS.
#
# This is the regression that made `make down` exit 1 after reporting success:
# on bash 3.2 a command substitution whose function ends in a failing command
# aborts the whole script under `set -e`, and "no process is listening on this
# port" is that function's normal answer.
# ---------------------------------------------------------------------------
status=0
dev_env down probe-901 --components api,web > "$out" 2>&1 || status=$?
if [ "$status" -ne 0 ]; then
  echo "Observed:" >&2
  sed 's/^/  /' "$out" >&2
  fail "down on a stopped environment exited $status, want 0"
fi
require_contains "$out" "stopped"

# Commands launched through env-exec must not inherit the daemon-task identity
# hints that make human/profile CLI commands reject --profile.
write_manifest "clean-env-903" "$root_dir" 903
MULTICA_TASK_CONFIG_ROOT=/task/config \
MULTICA_TASK_WORKSPACES_ROOT=/task/workspaces \
MULTICA_WORKSPACES_ROOT=/owner/workspaces \
  dev_env exec clean-env-903 -- sh -c '
    test -z "${MULTICA_TASK_CONFIG_ROOT:-}" &&
    test -z "${MULTICA_TASK_WORKSPACES_ROOT:-}" &&
    test "$MULTICA_WORKSPACES_ROOT" = "$1"
  ' _ "$MULTICA_DEV_WORKSPACES_PARENT/multica_workspaces_dev-dev-env-test-903" \
  > "$out" 2>&1 || fail "env-exec leaked daemon task identity or owner workspaces root"

# A health response without process identity is never proof that the process is
# this checkout's freshly launched API.
if bash -c 'source "$1"; api_started_after '\''{"status":"ok"}'\'' 1' _ "$root_dir/scripts/dev-env.sh"; then
  fail "legacy /health without started_at was accepted as current"
fi

# System lsof can repeatedly miss a stable Linux Next listener that ss reports.
# Keep lsof as the primary and macOS lookup; use the exact ss LISTEN query only
# after an empty lsof result on Linux.
bash -c '
  set -euo pipefail
  source "$1"
  trace="$2/listener-lookup.trace"

  uname() { [ "$1" = -s ] && printf "Linux\n"; }
  lsof() {
    printf "lsof:%s:<%s>:<%s>:<%s>:<%s>\n" "$#" "$1" "$2" "$3" "$4" >> "$trace"
    printf "31337\n"
  }
  ss() {
    printf "ss:%s:<%s>:<%s>:<%s>\n" "$#" "$1" "$2" "$3" >> "$trace"
    printf "%s\n" "LISTEN 0 511 *:13999 *:* users:((\"listener\",pid=4242,fd=22))"
  }
  : > "$trace"
  result="$(port_listener_pid 13999)"
  [ "$result" = 31337 ] || { echo "lsof PID was not preferred: $result"; exit 1; }
  [ "$(cat "$trace")" = "lsof:4:<-nP>:<-iTCP:13999>:<-sTCP:LISTEN>:<-t>" ] \
    || { echo "listener lookup bypassed lsof: $(cat "$trace")"; exit 1; }

  lsof() {
    printf "lsof:%s:<%s>:<%s>:<%s>:<%s>\n" "$#" "$1" "$2" "$3" "$4" >> "$trace"
    return 1
  }
  : > "$trace"
  result="$(port_listener_pid 13999)"
  [ "$result" = 4242 ] || { echo "Linux ss fallback PID = $result"; exit 1; }
  expected="$(printf "lsof:4:<-nP>:<-iTCP:13999>:<-sTCP:LISTEN>:<-t>\nss:3:<-H>:<-ltnp>:<sport = :13999>")"
  [ "$(cat "$trace")" = "$expected" ] \
    || { echo "Linux fallback query/order changed: $(cat "$trace")"; exit 1; }

  uname() { [ "$1" = -s ] && printf "Darwin\n"; }
  : > "$trace"
  result="$(port_listener_pid 13999)"
  [ -z "$result" ] || { echo "Darwin used Linux fallback: $result"; exit 1; }
  [ "$(cat "$trace")" = "lsof:4:<-nP>:<-iTCP:13999>:<-sTCP:LISTEN>:<-t>" ] \
    || { echo "Darwin listener lookup changed: $(cat "$trace")"; exit 1; }
' _ "$root_dir/scripts/dev-env.sh" "$tmp_dir" \
  || fail "listener lookup must be lsof-first with a Linux-only ss fallback"

# Next.js sits in a turbo/pnpm process group that is not the make launcher.
# Ownership must follow PPID, not only PGID, or `make up` kills a healthy web.
bash -c '
  set -euo pipefail
  source "$1"
  child_pid=""
  nested_pid=""
  cleanup() {
    [ -n "$child_pid" ] && kill "$child_pid" 2>/dev/null || true
    [ -n "$nested_pid" ] && kill "$nested_pid" 2>/dev/null || true
  }
  trap cleanup EXIT
  sleep 30 &
  child_pid=$!
  process_is_descendant_of "$child_pid" "$$" || { echo "direct child was not a descendant"; exit 1; }
  if process_is_descendant_of "$$" "$child_pid"; then
    echo "parent was treated as a descendant of its child"; exit 1
  fi
  python3 -c "import os, time; os.setsid(); time.sleep(30)" &
  nested_pid=$!
  # Give the child a moment to call setsid before we inspect PGID.
  for _ in 1 2 3 4 5; do
    [ "$(process_group_id "$nested_pid")" = "$nested_pid" ] && break
    sleep 0.05
  done
  [ "$(process_group_id "$nested_pid")" = "$nested_pid" ] || { echo "nested child did not become a process-group leader"; exit 1; }
  [ "$(process_group_id "$nested_pid")" != "$$" ] || { echo "nested child stayed in the test shell process group"; exit 1; }
  process_is_descendant_of "$nested_pid" "$$" || { echo "new-process-group child was not a descendant"; exit 1; }
' _ "$root_dir/scripts/dev-env.sh" \
  || fail "listener ownership must follow PPID across a new process group"

# The live pnpm/turbo chain can leave the launcher ancestry while both the
# launcher and Next listener remain alive. A dedicated launch session survives
# that reparenting, while an unrelated listener cannot join the session.
bash -c '
  set -euo pipefail
  source "$1"
  STATE_DIR="$2/session-state"
  LOG_DIR="$STATE_DIR/logs"
  mkdir -p "$LOG_DIR"
  launcher_pid=""
  listener_pid=""
  unrelated_pid=""
  cleanup() {
    [ -z "$launcher_pid" ] || kill -TERM -"$launcher_pid" 2>/dev/null || kill "$launcher_pid" 2>/dev/null || true
    [ -z "$listener_pid" ] || kill "$listener_pid" 2>/dev/null || true
    [ -z "$unrelated_pid" ] || kill "$unrelated_pid" 2>/dev/null || true
  }
  trap cleanup EXIT

  launch_detached web python3 -c "
import os, sys, time

middle = os.fork()
if middle == 0:
    listener = os.fork()
    if listener == 0:
        os.setpgid(0, 0)
        time.sleep(30)
        os._exit(0)
    fd = os.open(sys.argv[1], os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    os.write(fd, str(listener).encode())
    os.close(fd)
    os._exit(0)

time.sleep(30)
" "$STATE_DIR/listener.pid"
  launcher_pid="$(cat "$(pid_file web)")"

  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    [ -s "$STATE_DIR/listener.pid" ] && break
    sleep 0.05
  done
  [ -s "$STATE_DIR/listener.pid" ] || { echo "reparented listener pid was not written"; exit 1; }
  listener_pid="$(cat "$STATE_DIR/listener.pid")"
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    ! process_is_descendant_of "$listener_pid" "$launcher_pid" && break
    sleep 0.05
  done

  launcher_sid="$(ps -p "$launcher_pid" -o sid= 2>/dev/null | tr -d " ")"
  [ "$launcher_sid" = "$launcher_pid" ] || { echo "launcher did not start in a dedicated session"; exit 1; }
  [ "$(ps -p "$listener_pid" -o sid= 2>/dev/null | tr -d " ")" = "$launcher_sid" ] \
    || { echo "reparented listener left the launch session"; exit 1; }
  [ "$(process_group_id "$listener_pid")" != "$launcher_pid" ] \
    || { echo "reparented listener stayed in the launcher process group"; exit 1; }
  if process_is_descendant_of "$listener_pid" "$launcher_pid"; then
    echo "listener remained a launcher descendant"; exit 1
  fi

  port_listener_pid() { printf "%s" "$listener_pid"; }
  listener_belongs_to_component web 13999 \
    || { echo "same-session reparented listener was rejected"; exit 1; }

  python3 -c "import os, time; os.setsid(); time.sleep(30)" &
  unrelated_pid=$!
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    [ "$(ps -p "$unrelated_pid" -o sid= 2>/dev/null | tr -d " ")" = "$unrelated_pid" ] && break
    sleep 0.05
  done
  port_listener_pid() { printf "%s" "$unrelated_pid"; }
  if listener_belongs_to_component web 13999; then
    echo "unrelated session was accepted as the component listener"; exit 1
  fi
' _ "$root_dir/scripts/dev-env.sh" "$tmp_dir" \
  || fail "listener ownership must survive reparenting without trusting an unrelated port owner"

# ---------------------------------------------------------------------------
# Unknown names and components fail loudly instead of doing something else.
# ---------------------------------------------------------------------------
status=0
dev_env status no-such-env > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "status on an unknown environment must fail"
require_contains "$out" "Unknown environment"

status=0
dev_env up --components nope > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "up with an unknown component must fail"
require_contains "$out" "Unknown component"

# ---------------------------------------------------------------------------
# gc reports what it would collect and touches nothing in --dry-run. An
# environment whose checkout is gone has no owner left to stop it, which is how
# 152 databases accumulated with nothing on the machine able to list them.
# ---------------------------------------------------------------------------
write_manifest "orphan-902" "$tmp_dir/deleted-checkout" 902

dev_env gc --dry-run > "$out" 2>&1 || fail "gc --dry-run must succeed"
require_contains "$out" "orphan-902 would be collected"
if grep -Fq "probe-901 would be collected" "$out"; then
  fail "gc must not collect an environment whose directory still exists"
fi
[ -f "$MULTICA_DEV_HOME/envs/orphan-902/manifest.env" ] || fail "gc --dry-run deleted a manifest"

# A failed database drop keeps the manifest and slot so cleanup can be retried;
# destroy must never print success and forget the only deletion recipe.
prepare_checkout "$tmp_dir/drop-checkout"
write_manifest "drop-fails-904" "$tmp_dir/drop-checkout" 904
status=0
FAIL_DROP=1 dev_env destroy drop-fails-904 --yes > "$out" 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "destroy succeeded after DROP DATABASE failed"
[ -f "$MULTICA_DEV_HOME/envs/drop-fails-904/manifest.env" ] \
  || fail "destroy discarded the manifest after DROP DATABASE failed"
require_contains "$out" "manifest and slot were kept"
dev_env destroy drop-fails-904 --yes > "$out" 2>&1 || fail "retrying destroy after database recovery failed"

# ---------------------------------------------------------------------------
# destroy consumes the manifest: the slot is free afterwards, which is what
# makes the registry an allocator rather than a second place to leak.
# ---------------------------------------------------------------------------
dev_env destroy probe-901 --yes > "$out" 2>&1 || fail "destroy must succeed"
[ ! -d "$MULTICA_DEV_HOME/envs/probe-901" ] || fail "destroy left the environment directory behind"

dev_env list > "$out" 2>&1 || fail "list must succeed after destroy"
if grep -Fq "probe-901" "$out"; then
  fail "destroyed environment is still listed"
fi

# Declining the confirmation is a successful no-op, not a failure.
printf 'n\n' | dev_env destroy orphan-902 > "$out" 2>&1 || fail "declining destroy must exit 0"
require_contains "$out" "Cancelled."
[ -d "$MULTICA_DEV_HOME/envs/orphan-902" ] || fail "declined destroy removed the environment anyway"

echo "✓ dev-env.sh registry behaviour verified"
