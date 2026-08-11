---
name: multica-wiki
description: "Use when reading or writing Multica Workspace Wiki markdown pages (workspace, project, or personal scope)."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica Wiki

The workspace has a user-visible markdown knowledge base (Workspace Wiki). Pages are
stored as markdown (`path` ends in `.md`) with three scopes:

| Scope | Visibility | Required params |
| --- | --- | --- |
| `workspace` | all members | current workspace |
| `project` | all members | `project_id` + current workspace |
| `user` | owner only, **cross-workspace** | (uses current user; same library in every workspace) |

Workspace Wiki is separate from the evidence-generated LM Wiki shown under
`/{slug}/twins`. LM Wiki v1 does not ingest these mutable `wiki_page` records;
combining the stores requires an explicit provenance and review policy.

## API (via Multica HTTP / CLI if exposed)

List pages:

```bash
# Workspace scope (default)
curl -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WS" \
  "$API/api/wiki/pages?scope=workspace"

# Project scope
curl -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WS" \
  "$API/api/wiki/pages?scope=project&project_id=$PROJECT_ID"

# Personal scope
curl -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WS" \
  "$API/api/wiki/pages?scope=user"
```

Create a page (markdown body):

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" -H "X-Workspace-ID: $WS" \
  -H "Content-Type: application/json" \
  -d '{"scope":"workspace","path":"concepts/agents.md","title":"Agents","content":"# Agents\n\n..."}' \
  "$API/api/wiki/pages"
```

Get / update / delete:

```text
GET    /api/wiki/pages/{id}
PUT    /api/wiki/pages/{id}   body: { "path"?, "title"?, "content"? }
DELETE /api/wiki/pages/{id}
```

## Path rules

- Relative posix path ending in `.md`
- No absolute paths, no `..` segments
- Examples: `index.md`, `concepts/agents.md`

## When to write the wiki

- User asks to capture durable knowledge, decisions, or glossary entries
- After research that should stay queryable by later agent runs
- Prefer updating an existing page at the same path over creating duplicates

## Privacy

Personal (`user`) pages are private to the signed-in human. Do not attempt to
read another member's personal wiki.
