# Downstream overlay

This directory is the fork-owned overlay for `oldwinter/multica`.

Put local ops, extra scripts, and extra docs here. Do not patch upstream
files (`Makefile`, `docker-compose.selfhost.yml`, `.github/workflows/release.yml`,
and so on) for local debugging: those diffs conflict on every `upstream/main`
sync.

| Path | What it is |
| --- | --- |
| `local-selfhost/` | LAN Docker stack: bind `0.0.0.0`, rebuild from current source, replace images |
| `docs/downstream/` | Product specs and the upstream-sync ledger (already local-owned) |

Rule: if a change can live as a compose override, a wrapper script, or a
doc in this tree, it belongs here.
