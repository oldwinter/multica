#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

feature="${1:-}"

run_go() {
  (
    cd server
    go test "$@" -count=1
  )
}

run_vitest() {
  local package_dir="$1"
  shift
  pnpm --dir "$package_dir" exec vitest run "$@"
}

case "$feature" in
  rooms)
    run_go ./internal/room
    run_go \
      ./cmd/migrate ./cmd/server ./internal/analytics ./internal/daemon \
      ./internal/daemon/execenv ./internal/handler ./internal/metrics \
      ./internal/migrations ./internal/scheduler ./internal/service ./pkg/protocol \
      -run Room
    run_vitest packages/core rooms
    run_vitest packages/views rooms
    run_vitest apps/mobile \
      data/rooms-schema.test.ts \
      data/realtime/room-ws-updaters.test.ts \
      data/stores/room-drafts-store.test.ts \
      lib/room-interactions.test.ts \
      lib/room-selectors.test.ts
    ;;
  wiki)
    run_go \
      ./cmd/multica ./cmd/server ./internal/analytics ./internal/handler \
      ./internal/metrics ./internal/scheduler ./internal/service ./pkg/protocol \
      -run 'Wiki|LMWiki'
    run_vitest packages/core \
      wiki \
      api/lm-wiki-twin-schemas.test.ts \
      realtime/use-realtime-sync-wiki.test.ts
    run_vitest packages/views wiki
    run_vitest apps/web app/personal-wiki/routes.test.tsx
    run_vitest apps/desktop src/renderer/src/pages/wiki-page.test.tsx
    run_vitest apps/mobile \
      data/queries/wiki.test.ts \
      data/realtime/wiki-ws-updaters.test.ts \
      data/wiki-edit-conflict.test.ts \
      data/wiki-navigation.test.ts \
      data/wiki-schema.test.ts
    ;;
  twin)
    run_go \
      ./cmd/migrate ./cmd/server ./internal/daemon ./internal/handler \
      ./internal/service ./pkg/protocol \
      -run Twin
    run_vitest packages/core \
      twins \
      api/lm-wiki-twin-schemas.test.ts \
      realtime/twin-realtime.test.ts
    run_vitest packages/views \
      twins \
      common/task-transcript/agent-transcript-dialog.test.tsx \
      common/task-transcript/transcript-button.test.tsx \
      issues/components/issue-agent-header-chip.test.tsx
    ;;
  appearance)
    run_go ./internal/handler -run Appearance
    run_vitest packages/core appearance
    run_vitest packages/views \
      appearance \
      locales/parity.test.ts \
      settings/components/preferences-tab.test.tsx
    run_vitest apps/web app/text-contrast.test.ts
    run_vitest apps/mobile \
      data/appearance-analytics.test.ts \
      lib/appearance-preferences.test.ts \
      lib/appearance-sync-coordinator.test.ts \
      lib/appearance-sync.test.ts \
      lib/theme.test.ts
    node --test packages/ui/styles/token-contract.test.mjs
    node packages/ui/styles/check-token-contract.mjs
    ;;
  contracts)
    if git grep -n -E '^(<<<<<<<|=======|>>>>>>>)' -- ':!pnpm-lock.yaml'; then
      echo "Merge conflict markers remain in tracked files." >&2
      exit 1
    fi
    run_go ./internal/migrations -run 'Migration|Room'
    ;;
  *)
    echo "usage: $0 {rooms|wiki|twin|appearance|contracts}" >&2
    exit 2
    ;;
esac
