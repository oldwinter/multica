# Twin Product Provenance

This directory preserves the product intent that Multica may adopt from the
frozen Harness Studio design. It is a downstream planning record, not a claim
that the Harness Studio application is a working or passing baseline.

- Frozen Harness source commit: `1c864890c496115a65e6eafc3e4193caa157aee1`
- Multica source commit at capture: `37f3bb7dd9c0fe665051ce26dadab03b090dc1af`
- Product thesis and six-step loop: [product-goals.md](./product-goals.md)
- Terms and their adoption status: [glossary.md](./glossary.md)
- Machine-checked term and story provenance: [legacy-migration-matrix.md](./legacy-migration-matrix.md)

Set `TWIN_LEGACY_ROOT` to a checkout of the legacy repository, then run
`node scripts/check-twin-provenance.mjs` from the repository root before
changing this record. The checker parses the JSON data block in the migration
matrix, reads legacy paths from the frozen Git object, and rejects untracked
concepts, stories, commits, or destinations.

`adopted` means the product intent is retained. `adapted` means Multica keeps
the user outcome while using its own domain model and deployment shape.
`out-of-scope` is retained only as provenance and must use `Scope OUT` instead
of implying an implementation destination.
