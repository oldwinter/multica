# Workspace Wiki in Multica

User-editable Markdown knowledge base inspired by Karpathy's llm-wiki pattern.

## Relationship to LM Wiki

Workspace Wiki and LM Wiki serve different knowledge workflows:

- `/{slug}/wiki` stores member-authored Markdown pages for workspace, project,
  and personal notes.
- `/{slug}/twins` exposes the LM Wiki: immutable, cited revisions compiled from
  allowlisted workspace issues, projects, repository resources, and completed
  Autopilot runs plus explicitly pinned workspace/project Wiki revisions.
  Owner or admin review controls which revision is accepted evidence.

LM Wiki never reads a mutable Wiki page implicitly. Its source policy pins an
exact revision and freezes the page bytes, digest, provenance, citation key,
policy version, policy digest, and remote-generation decision into canonical
schema v2 content. Personal and local-only sources are permanent exclusions.

Accepting LM Wiki evidence commits only the human review. Twin proposal
generation is a separate, explicit, idempotent operation, so a model outage
cannot roll back accepted evidence.

## Product choices

| Choice | Decision |
| --- | --- |
| Visibility | First-class UI under `/{slug}/wiki` |
| Knowledge boundary | Three scopes: **workspace**, **project**, **user** |
| Storage | **Markdown** text (page body + `.md` path), Postgres-backed like Skills |
| History | Append-only revisions; restore creates a new revision |
| Concurrency | Every mutation compares the expected current revision |
| Agent writes | Shared changes are proposals requiring human review |
| Personal access | Global human API and `/personal-wiki`; no workspace membership required |

## Data model

Workspace and project `wiki_page` rows are workspace-tenanted. Personal rows
belong to the signed-in user across workspaces and keep `workspace_id` null.

| Scope | Who can read | Who can write | Tenancy |
| --- | --- | --- | --- |
| `workspace` | any member | any member | `workspace_id` required |
| `project` | any member | any member | `workspace_id` + `project_id` |
| `user` | owner only | owner only | **Cross-workspace**: `workspace_id` is null; unique on `(owner_user_id, path)` |

Personal pages follow the signed-in user across every workspace. Deleting a
workspace removes only workspace/project pages, never the personal library.

Each live page has:

- `path` — relative posix path ending in `.md` (e.g. `index.md`, `concepts/agents.md`)
- `title` — display title
- `content` — full markdown body

Every save also creates an immutable revision containing its revision number,
content digest, actor provenance, source kind, and timestamp. Deleting a live
page removes it from normal browsing but preserves revisions until their tenant
is deleted, so published citations remain inspectable.

## API surface

Workspace-member routes:

- `GET /api/wiki/search?q=&scope=&project_id=`
- `GET|POST /api/wiki/pages`
- `GET|PUT|DELETE /api/wiki/pages/{id}`
- `GET /api/wiki/pages/{id}/revisions`
- `GET|POST /api/wiki/pages/{id}/proposals`
- `POST /api/wiki/pages/{id}/revisions/{revisionId}/restore`
- `GET /api/wiki/revisions/{revisionId}` for an immutable citation target

User-owned routes that work without a workspace membership:

- `GET /api/personal-wiki/search`
- `GET|POST /api/personal-wiki/pages`
- `GET|PUT|DELETE /api/personal-wiki/pages/{id}`
- `GET /api/personal-wiki/pages/{id}/revisions`
- `POST /api/personal-wiki/pages/{id}/revisions/{revisionId}/restore`
- `GET /api/personal-wiki/revisions/{revisionId}`

The personal routes accept only human user authentication and return not found
for workspace/project page IDs.

## Agent access

The `multica wiki` CLI provides typed `list`, `get`, `search`, and `propose`
operations. Agent task credentials cannot update or delete shared pages
directly. Proposals require a non-empty rationale and server-verified task or
Room evidence from the same workspace; acceptance creates one new revision.

## Non-goals (v1)

- Vector search / embeddings
- Implicitly compiling mutable or personal Wiki pages into LM Wiki revisions
- Git-backed export (content is already markdown; export can come later)
