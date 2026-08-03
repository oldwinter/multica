# ADR 0001: Selective borrowing

Status: Accepted

Twin Workspace borrows Harness Studio concepts only where they fit Multica's
workspace membership, Agent, Issue, React Query, daemon, and PostgreSQL model.
The legacy system is design evidence, not a second runtime or compatibility
surface. Mobile, rebranding, wholesale source copies, a generic plugin system,
and changes to ordinary global navigation search are excluded.

<!-- twin-contract: decisions -->
| id | decision | owner | forbidden | semantic_sha256 |
| --- | --- | --- | --- | --- |
| decision-selective-borrowing | Preserve persona evidence, Topics, Runs, Asks, signed versions, and deposition semantics | downstream Twin modules | wholesale copy or legacy runtime | 9be336fecb7ff339e124d65b86768a5fc685804797588eaf69fa38bb60259b8c |
| decision-existing-domain | Use Multica workspace membership, user Agents, Issues, daemon profiles, React Query, and existing routing adapters | owning Todo | parallel identity, ACL, or state systems | 89763695cdd1d4ed8b076f8b670ae2c1e563116ebdb401fa89bed1e230b92359 |
| decision-scope | Desktop owns local artifacts and Web owns safe metadata views | Todos 13, 26-31 | Mobile, global-search edits, renderer secrets | 45fe6b8817361a83d947a24767e7045b3184b6cbd04e5976b9daa0ba08d68b23 |
| decision-claude-only | Initial production eligibility is signed Claude Code 2.1.220 with proven control transport and OS containment | Todos 21 and 31 | ACP auto-grant or bypass eligibility | 67576690235f3943e85ef1cbe6fa317e95257dcfb942c16b30324491bce7cdd2 |
| decision-observations | Upstream rehearsal observes read-only; only Todo 32 may persist append-only observations | Todo 32 and F4 | baseline mutation or read-only writes | 46d3e774999b6ed30092cca83f03d2af3182cd23a47a1720d8165ca2f9f0d1e4 |
| decision-evidence-custody | Final evidence is external, canonical, non-link, SHA-bound, and outside every worktree | start-work and F1-F4 | relative, worktree-local, synthetic, or mixed-SHA evidence | 0ffefa9454903155aafb633bad01357484be4436c37f426783200351a0ad2fd0 |

The Claude eligibility statement is a runtime constraint only; it does not
define a fixed Twin Brain or product-domain contract.

The exact Todo 2 document set is an atomic predecessor of the Todo 3 boundary
manifest. Todo 3 cannot commit until the committed Todo 1 and Todo 2 paths exist.
