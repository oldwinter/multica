# ADR 0004: Execution, egress, and recovery

Status: Accepted

Each Run uses a fresh private workspace and session. The model gateway is the
only network endpoint exposed to an eligible parent runtime; external effects
use typed daemon proxies. Release follows prepare, owner review, one-use
attestation, execution, and reconciliation. Unknown external results become
`unknown_outcome` and are never blindly retried.

<!-- twin-contract: execution-decisions -->
| id | decision | proof owner | forbidden | semantic_sha256 |
| --- | --- | --- | --- | --- |
| execution-workspace | Git uses fresh `twin/run/<run-id>` worktree at base SHA; non-Git uses fresh 0700 allowlisted directory | Todo 20 | shared checkout, merge, push, path escape | c2abb73f0e85c44e204daeeb83e0b7b55d58e5e71f7abc730c21df56fc60ab80 |
| execution-possession | S1 is mandatory first prompt bytes; digest-equal read-only S2 `.twin` overlay is mandatory; S3 is optional | Todos 18, 20, 21 | LocalOnly excerpt, collision, writable overlay, unavailable-as-empty | 44ac6e118ef4bd7ea988d22e5c609bd8431b336996dfc564d6d86cbd86d6c238 |
| execution-claude | Exact signed Claude 2.1.220, typed handler, default permission mode, stdio control request, frozen executable lease | Todo 21 | PATH-only validation, reopened canonical path, ACP, nil handler | f186786631770163338d8aec28189ae923df8e500d1f3979244a27b8cf728b38 |
| execution-effect | typed model/effect descriptors and idempotency keys cross gateway; provider keys remain daemon-only | Todos 21-22 | child credential, child remote, raw config, blind retry | 2c9ea01000c6b377d14df92a19e0c86be959a46d09a62686ad2177080a1846f6 |
| execution-ownership | Todo 20 is process-neutral; Todo 21 proves denial; Todo 22 proves fake release; Todo 31/F2 compose real success | evidence validators | evidence claimed before its dependency owner | e81b579472425876f996a24037d4c883024cdab7006c3daa3345c8d3955a9e7f |
