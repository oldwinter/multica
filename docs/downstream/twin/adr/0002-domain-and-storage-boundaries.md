# ADR 0002: Domain and storage boundaries

Status: Accepted

PostgreSQL is the shared lifecycle ledger. The immutable owner daemon is the
custodian of raw local material. No compatibility copy crosses that boundary.

<!-- twin-contract: domain-boundaries -->
| id | shared authority | local authority | invariant | semantic_sha256 |
| --- | --- | --- | --- | --- |
| boundary-twin | lifecycle, immutable owner IDs, opaque machine/profile IDs, digest, policy, state | source registry, draft, signed package, raw evidence | shared rows never reveal path, basename, local profile name, or credential | 6e8303fc567e67fad4b2b4a0fcef58f76eb9269d88108243304a9ca586db7843 |
| boundary-topic | bounded title, user/Twin messages, typed safe intents and refs | raw Brain request/response and source excerpts | Topic text is conversation, never an excerpt transport | 78133f13c018e8a4cb342ab6d66c867c1837561e5d1c3eb7130785772f903c05 |
| boundary-run | operation links, budget ticks, safe progress, deterministic report, aggregate digest | workspace, session, workdir, raw frames, diffs, logs, candidates | linked Tasks never use ordinary raw-message, usage, activity, or session stores | 1883ba3967d69b21bddfdb3262d5af6fe3550a9e08aef55d550048fe2c5d76db |
| boundary-input | opaque `TwinLocalInputResource` ID, safe label, digest, classification | ID-to-path registry and bytes | `project_resource.local_directory` is ineligible and has no fallback copy | 8eeb6ab47f6dc4a6090cceb7ab37093df5b8bae358413f4b18b64ebdb071edbd |
| boundary-profile | opaque `daemon_profile_id` and auth-profile ID/revision/status | CLI profile name, callback file, provider key | ordinary `DaemonStatus.profile` remains unchanged but is never Twin identity | dafe5ac055264de29c452458762454d4692577020d279178df19533545a22660 |

Labels are explicitly entered, at most 80 Unicode code points, and reject path
separators, control characters, basename derivation, and secret-like patterns.
