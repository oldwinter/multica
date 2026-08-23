# Wiki source map

- `server/internal/handler/wiki.go` enforces page scope, membership, expected-revision writes, revision history, restore, search, and Agent proposal review.
- `server/pkg/db/queries/wiki_page.sql` is the tenanted page, revision, search, and proposal query surface.
- `server/internal/service/lm_wiki.go` owns immutable LM Wiki refresh and review lifecycle.
- `server/internal/service/lm_wiki_domain.go` canonicalizes eligible source revisions and produces stable citation digests.
- `server/pkg/db/queries/lm_wiki.sql` reads the explicit source policy and exact selected Wiki revisions.
- `server/cmd/server/router.go` registers `/api/wiki` and `/api/lm-wiki` routes behind the existing authentication and workspace membership gates.
- `server/cmd/multica/cmd_wiki.go` exposes typed Agent-safe list, get, search, and proposal commands; it intentionally has no direct update or delete command.
- `packages/core/wiki/` contains the drift-safe API types, React Query keys, and mutations used by shared clients.
- `packages/views/wiki/` is the shared Web/Desktop authoring, search, history, conflict, and proposal-review surface.
- Workspace deletion removes workspace/project pages, revisions, proposals, selections, policies, and LM Wiki artifacts explicitly. Personal pages remain owned by the user and survive workspace deletion.
