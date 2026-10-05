# Skill Evolution Ownership

The Skill evolution feature is a downstream-owned, removable sidecar. The
upstream `skill` and `skill_file` rows remain the runtime source of truth.

## Leaf Ownership

- `server/internal/skillevolution/`: domain contracts, lifecycle, adapters,
  persistence, HTTP, scheduling, cleanup, and metrics.
- `server/internal/handler/task_run_review.go`,
  `server/internal/service/task_run_review.go`, and
  `server/pkg/db/queries/task_run_review.sql`: the task-review backend leaf and
  its UUID-normalized HTTP boundary.
- `packages/core/skill-evolution/`: client schemas, queries, and mutations.
- `packages/core/task-run-reviews/`: task-review client schemas, queries,
  mutations, and validation.
- `packages/views/skill-evolution/`: shared management UI.
- `packages/views/task-run-reviews/`: the complete task-review form, state,
  query/mutation composition, and interaction policy.
- Task-review fields in `packages/views/locales/*/agents.json`.
- Web and Desktop evolution leaf routes and the four evolution locale files.
- `skill_evolution_*` tables, their migrations, queries, and generated sqlc
  output.

## Shared Exceptions

Shared files may change only for narrow additive registration or compatibility
fields. The current allowlist is:

- `server/pkg/skillbundle/hash.go`: canonical bundle validation and hashing.
- `server/pkg/protocol/messages.go`, `server/internal/daemon/types.go`, and the
  daemon result/report/generation path: the optional, content-free resolved
  bundle execution manifest.
- `server/cmd/server/main.go` and `server/cmd/server/router.go`: one scheduler
  registration, module composition, and one grouped route registration.
- `server/internal/handler/contributors.go` and
  `server/internal/handler/workspace.go`: neutral observation contributors and
  one transactional cleanup contributor invocation.
- `server/internal/scheduler/manager.go`: the workspace claim fence shared by
  the evolution scheduler job and workspace deletion.
- `server/cmd/migrate/main.go` and its migration ledger tests: migration
  cleanup/compatibility only; published identities remain immutable.
- `server/internal/metrics/registry.go`: generic collector registration.
- `server/pkg/llm/client.go`, its outbound-contract test, `.env.example`, and
  the four environment-variable documentation pages: centralized outbound
  composition and operator disclosure for bounded behavioral replay.
- Wiki, Room, and Twin source-owned adapters and the Room target router.
- `packages/core/api/client.ts`, task-review/evolution schema exports, and their
  contract tests: JSON DTO compatibility and content-free privacy boundaries.
- `packages/views/common/task-transcript/agent-transcript-dialog.tsx`: one
  opaque `ReactNode` slot, with no task-review policy or data access.
- `packages/views/common/task-transcript/transcript-button.tsx`: one leaf import
  and action registration that supplies the task-review slot.
- Core/view package exports, locale exports, the Skill detail action, the Web
  and Desktop route tables, and E2E fixtures.
- `server/pkg/db/generated/`: generated sqlc output only; edits originate in
  query or schema sources and regeneration must be reproducible.

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
