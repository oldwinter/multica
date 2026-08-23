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
`/{slug}/twins`. Owners and admins may explicitly select an immutable
workspace or project page revision as an LM Wiki source.
Personal pages are never eligible LM Wiki evidence.

## Typed CLI

List pages:

```bash
# Workspace scope (default)
multica wiki list --scope workspace --output json

# Project scope
multica wiki list --scope project --project-id <project-id> --output json

# Personal scope
multica wiki list --scope user --output json
```

Search and read the exact current revision:

```bash
multica wiki search "review policy" --scope workspace --output json
multica wiki get <page-id> --output json
```

The list, search, and get responses include `current_revision_id`,
`current_revision_number`, `content_digest`, and provenance. Cite the immutable
revision as `wiki_page_revision:{current_revision_id}`. Never cite only the
mutable page ID.

Shared clients use these additional lifecycle routes:

```text
GET    /api/wiki/pages/{id}
GET    /api/wiki/pages/{id}/revisions
```

Human updates use optimistic concurrency. Read `current_revision_number` from
the page, then include it in every write. A `409` response with code
`wiki_revision_conflict` means the page changed; reload it and merge the new
content before retrying. Never blindly retry a stale write.

```text
PUT    /api/wiki/pages/{id}
       body: { "expected_revision_number": 4, "path"?, "title"?, "content"? }
POST   /api/wiki/pages/{id}/revisions/{revisionId}/restore
       body: { "expected_revision_number": 4 }
DELETE /api/wiki/pages/{id}
```

Every successful create, update, accepted proposal, or restore creates one
append-only revision. Cite the exact revision ID and content digest used in
work; do not cite only the mutable page ID.

## Agent edit proposals

Agents must not directly update shared workspace or project pages. Submit a
reviewable proposal against the exact base revision instead:

```bash
multica wiki propose <page-id> \
  --base-revision 4 \
  --path concepts/agents.md \
  --title "Agents" \
  --content-file ./proposal.md \
  --rationale "Record the reviewed operating rule" \
  --evidence-ref task:<task-id> \
  --idempotency-key <stable-key> \
  --output json
```

Only a human reviewer may accept, edit-and-accept, or reject a proposal.
Acceptance rechecks the base revision and creates one new page revision. Agent
retries must reuse the same idempotency key. `MULTICA_AGENT_ID` supplies the
authoritative Agent identity in task context; do not claim another Agent ID.

The typed command submits this contract:

```text
POST /api/wiki/pages/{id}/proposals
body: {
  "base_revision_number": 4,
  "proposed_path": "concepts/agents.md",
  "proposed_title": "Agents",
  "proposed_content": "# Agents\n...",
  "rationale": "Record the reviewed operating rule",
  "evidence_refs": ["task:<task-id>"],
  "agent_id": "<MULTICA_AGENT_ID>",
  "idempotency_key": "<stable-key>"
}
```

## Path rules

- Relative posix path ending in `.md`
- No absolute paths, no `..` segments
- Examples: `index.md`, `concepts/agents.md`

## When to write the wiki

- User asks to capture durable knowledge, decisions, or glossary entries
- After research that should stay queryable by later agent runs
- Search before proposing a page so the Agent does not create duplicates
- For shared knowledge, prefer a small proposal against the current revision
- Personal pages may be written only on the signed-in human's explicit request

## Privacy

Personal (`user`) pages are private to the signed-in human. Do not attempt to
read another member's personal wiki, cite them as shared evidence, or include
them in an LM Wiki source policy.

See `references/wiki-source-map.md` for the implementation surfaces behind
this contract.
