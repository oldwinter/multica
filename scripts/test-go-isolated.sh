#!/usr/bin/env bash
# Runs scripts/test-go.sh inside a disposable container so the httptest
# listeners it opens stay on the container's loopback interface. The shared
# CI host runs moshi-hook service discovery, which probes every loopback HTTP
# port it can see and previously injected stray `GET /` requests into live Go
# tests (unexpected-request fatals and request-count races). A container
# network namespace is the isolation boundary: nothing on the host can reach
# an in-container loopback listener.
#
# GO_TEST_NETWORK selects the docker network — set it to the job's service
# network so DATABASE_URL/REDIS_TEST_URL can point at the service containers
# by name. When unset the script creates and cleans up a private network.
set -euo pipefail

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
GO_IMAGE=${GO_TEST_IMAGE:-golang:1.27-bookworm}
NETWORK=${GO_TEST_NETWORK:-}
OWNED_NETWORK=""
GOHOME_DIR=""

if [ -z "$NETWORK" ]; then
  NETWORK="test-go-isolated-$$"
  docker network create "$NETWORK" >/dev/null
  OWNED_NETWORK=$NETWORK
fi
cleanup() {
  if [ -n "$OWNED_NETWORK" ]; then
    docker network rm "$OWNED_NETWORK" >/dev/null 2>&1 || true
  fi
  if [ -n "$GOHOME_DIR" ]; then
    rm -rf "$GOHOME_DIR" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Pass Go env configuration through only when set: a `-e GOPROXY` with an
# empty host value would inject an empty GOPROXY and break module resolution.
env_args=()
for var in GOFLAGS GOPROXY GOSUMDB GONOSUMDB GOPRIVATE GOINSECURE DATABASE_URL REDIS_TEST_URL; do
  if [ -n "${!var:-}" ]; then
    env_args+=(-e "$var=${!var}")
  fi
done

# -u keeps files the suite writes (module cache, build cache, any workspace
# artifacts) owned by the runner user instead of root. GOMODCACHE/GOCACHE are
# mounted at fixed container paths so nothing in the image layout is assumed.
# HOME is a mounted subdirectory rather than /tmp itself: validateLocalPath
# treats the bare /tmp root as a protected system path, so a container HOME of
# exactly /tmp would flag the home-directory check with the wrong reason.
GOHOME_DIR=$(mktemp -d "${TMPDIR:-/tmp}/gohome.XXXXXX")
docker run --rm \
  --network "$NETWORK" \
  -u "$(id -u):$(id -g)" \
  -e HOME=/tmp/gohome \
  -v "$GOHOME_DIR:/tmp/gohome" \
  "${env_args[@]}" \
  -e GOMODCACHE=/gomodcache \
  -e GOCACHE=/gocache \
  -v "$HOME/go/pkg/mod:/gomodcache" \
  -v "$HOME/.cache/go-build:/gocache" \
  -v "$REPO_ROOT:$REPO_ROOT" \
  -w "$REPO_ROOT" \
  "$GO_IMAGE" \
  bash scripts/test-go.sh "$@"
