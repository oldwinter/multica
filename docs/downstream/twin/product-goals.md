# Twin Product Goals

## Product Thesis

Harness Studio's thesis was a local-first software factory for one technical
person: turn personal evidence into a reviewable text artifact, then use that
artifact to guide delegated software work. Multica adopts the durable product
intent, not the legacy application's code, topology, or passing status.

The Multica adaptation is intentionally narrower: a reviewed Twin profile can
inform work that is already represented by Multica workspaces, issues, agents,
and execution records. Existing Multica tenancy, API, runtime, and UI
boundaries remain authoritative.

## Six-Step Loop

The legacy loop is preserved as a product sequence, with each step explicitly
classified in the migration matrix.

1. **Import**: collect user-selected evidence with explicit policy.
2. **Generate Twin**: derive a reviewable, evidence-backed profile rather than
   training a model.
3. **Open Topic and Dispatch**: express the objective and delegate bounded
   work; Multica adapts this through its issue and agent model.
4. **Coordinate Execution**: observe work, runtime progress, and exceptions.
5. **Report and Accept**: make the human acceptance point explicit.
6. **Deposition**: propose small, reviewable deltas from execution evidence;
   accept or reject them before archival.

## Classification

### Adopted

- Evidence-backed Twin artifacts, first-use sign-off, and incremental
  deposition are retained as product goals.
- Egress and local-only policy remain explicit user-control requirements.
- Acceptance remains a human decision, distinct from runtime completion.

### Adapted

- A legacy Topic becomes an issue-backed collaboration flow, not a new
  parallel chat domain.
- Dispatch and Run concepts attach to Multica agents and execution records.
- The four-layer injection protocol is provenance for future bounded runtime
  integration, not an instruction to reproduce the legacy local daemon.

### Out of Scope

- The Harness Studio single-user localhost daemon, npm installer, SQLite-only
  topology, and Claude-Code-only brain are not inherited as Multica goals.
- Legacy implementation decisions, including its application code and test
  posture, are not a baseline assertion.
- A new product named Harness Studio is not being introduced.

See [legacy-migration-matrix.md](./legacy-migration-matrix.md) for one source
path and one permitted destination (or `Scope OUT`) for every recorded term and
user story.
