# Skill Evolution Ownership

The Skill evolution feature is a downstream-owned, removable sidecar. The
upstream `skill` and `skill_file` rows remain the runtime source of truth.

## Leaf Ownership

- `server/internal/skillevolution/`: domain contracts, lifecycle, adapters,
  persistence, HTTP, scheduling, cleanup, and metrics.
- `packages/core/skill-evolution/`: client schemas, queries, and mutations.
- `packages/views/skill-evolution/`: shared management UI.
- Web and Desktop evolution leaf routes and the four evolution locale files.
- `skill_evolution_*` tables, their migrations, queries, and generated sqlc
  output.

## Shared Exceptions

Shared files may change only for narrow additive registration or compatibility
fields. The current allowlist is:

- `server/pkg/skillbundle/hash.go`: canonical bundle validation and hashing.
- `server/pkg/protocol/messages.go` and the daemon task result/report/client
  boundary: optional resolved-bundle execution manifest.
- `server/cmd/server/main.go`, `server/cmd/server/router.go`: one scheduler and
  one grouped route registration.
- `server/internal/handler/workspace.go`: one cleanup contributor invocation.
- `server/internal/metrics/registry.go`: generic collector registration.
- Wiki, Room, and Twin source-owned adapters and the Room target router.
- `packages/core/api/client.ts`, core path/package exports, view package/locale
  exports, the Skill detail action, the Desktop route table, and E2E fixtures.

Any new shared exception requires an explicit justification in review. The
feature does not own task selection/retry algorithms, Skill CRUD, realtime,
Inbox, Mobile management, navigation shells, sidebars, or generated files by
hand.

## Ownership Classification

Evolution eligibility comes from persisted database ownership, not runtime
bundle `source`:

- Built-ins are ineligible.
- A non-null `plugin_installation_id` is plugin-owned and requires a fork.
- `config.origin.type` values `github`, `skills_sh`, and `clawhub` are external
  and require a fork.
- `runtime_local` requires a fork.
- Unknown, malformed, or non-object origins fail closed and require a fork.
- With no plugin installation and no origin, the Skill is treated as a manual
  workspace Skill and is directly eligible.

Historical archive imports did not record an origin and are therefore
indistinguishable from manual Skills. This is a compatibility limitation, not
evidence that the archive was workspace-authored.

## Canonical Bundle Contract

Strict evolution bundles use slash-only, relative UTF-8 paths. Paths are
unique under Unicode-aware lowercase comparison and may not contain aliases,
traversal, backslashes, controls, Windows-reserved segments, binary extensions,
or the reserved root `SKILL.md`. Supporting files are capped at 256 files and
1 MiB each; primary content is capped at 1 MiB; paths are capped at 1024 bytes;
and the complete unhashed input is capped at 8 MiB.

The complete-input budget includes source, ID, name, description, primary
content, supporting paths, and supporting content. `Manifest.SizeBytes` keeps
its pre-existing content-only meaning for daemon protocol compatibility.
