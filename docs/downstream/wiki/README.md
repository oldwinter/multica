# Workspace Wiki in Multica

User-editable Markdown knowledge base inspired by Karpathy's llm-wiki pattern.

## Relationship to LM Wiki

Workspace Wiki and LM Wiki serve different knowledge workflows:

- `/{slug}/wiki` stores member-authored Markdown pages for workspace, project,
  and personal notes.
- `/{slug}/twins` exposes the LM Wiki: immutable, cited revisions compiled from
  allowlisted workspace issues, projects, repository resources, and completed
  Autopilot runs. Owner or admin review controls which revision can build or
  evolve the signed Twin.

LM Wiki does not ingest mutable `wiki_page` content in v1. Adding that source
requires an explicit provenance and review policy rather than silently merging
the two stores.

## Product choices

| Choice | Decision |
| --- | --- |
| Visibility | First-class UI under `/{slug}/wiki` |
| Knowledge boundary | Three scopes: **workspace**, **project**, **user** |
| Storage | **Markdown** text (page body + `.md` path), Postgres-backed like Skills |

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

Each page has:

- `path` — relative posix path ending in `.md` (e.g. `index.md`, `concepts/agents.md`)
- `title` — display title
- `content` — full markdown body

## API surface

- `GET /api/wiki/pages?scope=&project_id=`
- `GET /api/wiki/pages/{id}`
- `POST /api/wiki/pages`
- `PUT /api/wiki/pages/{id}`
- `DELETE /api/wiki/pages/{id}`

## Agent access

Builtin skill `multica-wiki` documents CLI/API for ingest-style create/update and query via list/get.

## Non-goals (v1)

- Vector search / embeddings
- Automatically compiling editable Wiki pages into LM Wiki revisions
- Git-backed export (content is already markdown; export can come later)
