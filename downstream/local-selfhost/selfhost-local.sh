#!/usr/bin/env bash
# Isolated LAN self-host stack (compose project: multica-local).
#
# Wraps upstream docker-compose.selfhost.yml + docker-compose.selfhost.build.yml
# with compose.bind.yml. Does not edit those upstream files.
#
# Usage, from the git tree you want in the image:
#   bash downstream/local-selfhost/selfhost-local.sh build
#   bash downstream/local-selfhost/selfhost-local.sh up
set -euo pipefail

PROJECT="${SELFHOST_LOCAL_PROJECT:-multica-local}"
OVERLAY_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$OVERLAY_DIR/../.." && pwd)"

if [[ -f "$PWD/docker-compose.selfhost.yml" ]]; then
  SOURCE_ROOT="$PWD"
else
  SOURCE_ROOT="$REPO_ROOT"
fi

if [[ -n "${SELFHOST_LOCAL_ENV:-}" ]]; then
  ENV_FILE="${SELFHOST_LOCAL_ENV}"
elif [[ -f "$REPO_ROOT/.env.selfhost.local" ]]; then
  ENV_FILE="$REPO_ROOT/.env.selfhost.local"
elif [[ -f "$OVERLAY_DIR/.env.selfhost.local" ]]; then
  ENV_FILE="$OVERLAY_DIR/.env.selfhost.local"
else
  echo "Missing .env.selfhost.local" >&2
  echo "Copy downstream/local-selfhost/env.example to the repo root as .env.selfhost.local" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

ENV_FILE="$(readlink -f "$ENV_FILE")"
SOURCE_ROOT="$(readlink -f "$SOURCE_ROOT")"

cmd="${1:-}"
shift || true

stamp_env() {
  local commit="$1" version="$2" date="$3"
  local tmp
  tmp="$(mktemp)"
  awk -v commit="$commit" -v version="$version" -v date="$date" '
    BEGIN { tag=commit }
    /^MULTICA_IMAGE_TAG=/ { print "MULTICA_IMAGE_TAG=" tag; next }
    /^VERSION=/ { print "VERSION=" version; next }
    /^COMMIT=/ { print "COMMIT=" commit; next }
    /^DATE=/ { print "DATE=" date; next }
    { print }
  ' "$ENV_FILE" >"$tmp"
  cat "$tmp" >"$ENV_FILE"
  rm -f "$tmp"
}

compose() {
  env -i \
    PATH="$PATH" \
    HOME="${HOME:-}" \
    USER="${USER:-}" \
    LOGNAME="${LOGNAME:-}" \
    TERM="${TERM:-}" \
    LANG="${LANG:-C.UTF-8}" \
    LC_ALL="${LC_ALL:-}" \
    DOCKER_HOST="${DOCKER_HOST:-}" \
    DOCKER_CONTEXT="${DOCKER_CONTEXT:-}" \
    DOCKER_CONFIG="${DOCKER_CONFIG:-}" \
    XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-}" \
    VERSION="${VERSION:-}" \
    COMMIT="${COMMIT:-}" \
    DATE="${DATE:-}" \
    docker compose \
      -p "$PROJECT" \
      --project-directory "$SOURCE_ROOT" \
      --env-file "$ENV_FILE" \
      -f "$SOURCE_ROOT/docker-compose.selfhost.yml" \
      -f "$SOURCE_ROOT/docker-compose.selfhost.build.yml" \
      -f "$OVERLAY_DIR/compose.bind.yml" \
      "$@"
}

wait_ready() {
  local published backend_port backend_url
  published="$(compose port backend 8080 2>/dev/null | tail -n 1 || true)"
  backend_port="${published##*:}"
  backend_port="${backend_port%$'\r'}"
  case "$backend_port" in
    '' | *[!0-9]*) backend_port=8180 ;;
  esac
  backend_url="http://127.0.0.1:${backend_port}"
  echo "==> Waiting for backend at $backend_url ..."
  local i
  for i in $(seq 1 30); do
    if curl -sf "${backend_url}/health" >/dev/null 2>&1; then
      echo ""
      echo "✓ Multica local stack is running"
      echo "  Frontend: http://192.168.10.118:3100  (or http://localhost:3100)"
      echo "  Backend:  http://192.168.10.118:${backend_port}"
      echo "  Login:    any email + code 888888"
      return 0
    fi
    sleep 2
  done
  echo "Services are still starting. Check logs:" >&2
  echo "  bash $OVERLAY_DIR/selfhost-local.sh logs" >&2
  return 1
}

case "$cmd" in
build)
  if ! git -C "$SOURCE_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "build must run from a git checkout (got $SOURCE_ROOT)" >&2
    exit 1
  fi
  COMMIT="$(git -C "$SOURCE_ROOT" rev-parse HEAD)"
  SHORT="$(git -C "$SOURCE_ROOT" rev-parse --short HEAD)"
  VERSION="$(git -C "$SOURCE_ROOT" describe --tags --always --dirty)"
  DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  export VERSION COMMIT DATE
  echo "==> Building $PROJECT from $VERSION ($SHORT)"
  echo "    source: $SOURCE_ROOT"
  echo "    env:    $ENV_FILE"
  compose up -d postgres
  compose build \
    --build-arg "VERSION=$VERSION" \
    --build-arg "COMMIT=$COMMIT" \
    --build-arg "DATE=$DATE" \
    backend frontend
  compose up -d --force-recreate --no-deps backend frontend
  docker tag multica-backend:dev "multica-backend:$SHORT"
  docker tag multica-web:dev "multica-web:$SHORT"
  stamp_env "$COMMIT" "$VERSION" "$DATE"
  wait_ready
  echo "Images also tagged: multica-backend:$SHORT  multica-web:$SHORT"
  ;;
up)
  echo "==> Recreating $PROJECT from current images"
  echo "    env: $ENV_FILE"
  compose up -d --force-recreate --no-deps backend frontend
  wait_ready
  ;;
status)
  compose ps
  wait_ready
  ;;
logs)
  compose logs -f "$@"
  ;;
stop)
  echo "==> Stopping $PROJECT (volumes kept)"
  compose down
  ;;
*)
  echo "usage: $0 {build|up|status|logs|stop}" >&2
  exit 2
  ;;
esac
