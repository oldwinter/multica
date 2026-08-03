# Twin field-placement and privacy contract

<!-- twin-contract: fields -->
| id | field class | placement | allowed channels | forbidden | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| field-shared-metadata | lifecycle, safe display labels, title/items/messages, opaque IDs, owner IDs, digests, classifications, redacted summaries, state | PostgreSQL | Twin API/event | raw/local derivation | 9444e01cb2a20a32a819a18cedc2ce43dba00e9f90c72cb4b8281d99f34db112 |
| field-local-resource | basename, path, `TwinLocalInputResource` registry, raw source/input | owner daemon L0 | local socket only | PostgreSQL, remote prompt, shared context | 4eef6eb8d27c117d7a60ec953a7982d9e1651b798189c12dca67865b12122bc5 |
| field-local-artifact | draft, excerpt, edit, package, diff, log, candidate, Run workspace/session/workdir/raw frame | owner daemon | local socket and local spool | ordinary task_message/task_usage/activity | bb892222154fd56bd3dade8728a3130ba641fc418e3f6c22523dcfee810d86f6 |
| field-local-secret | provider key, callback/cleanup bearer token, raw Ask/model payload | owner daemon protected memory/keyring/file | pipe/socket capability only | renderer, Web, SQL plaintext, logs | dcc3d81c37a3f3a60cd0e28be9013489548a804c643713e9ab0a0ddee10b55ea |
| field-profile-opaque | `daemon_profile_id`, provider auth profile ID/revision/status | SQL and Twin IPC | exact opaque ID | CLI profile name | 3d077843c1546d56bc2e681a96992f37804eb0edf294566733bdfd6f8eb0c8b5 |
| field-safe-progress | typed phase, bounded counters, redacted reason/fingerprint, digest | PostgreSQL | member API/event | excerpt, path, token, config | 9f28bffafcf965c2add36d49294111bd23bac103d8f4e685e6ddd03287f506fc |
| field-topic-body | bounded user/Twin conversation | PostgreSQL | Topic API/event | source excerpt or raw Task frame | d61b2bd82bdadb00274da529c2f2c7fac1a83d6929102aa164b2b89bec60022b |
| field-report | deterministic per-Run record and aggregate digest | PostgreSQL | Topic/Run API/event | model-selected Run IDs or raw logs | 49d9195e1dc30ee29de375adfa15028e03677f6001df57cdb26e774ebb132bfc |
| field-deposition | candidate bytes and edits local; accepted bounded Topic/Issue projection shared | split local/shared | owner local review then typed commit | remote-only commit or raw diff upload | a399d7b41d5cf7a27de7005f764e2d18502abee2710cf4ce9a0951799961ff5a |
| field-label | explicit sanitized label, maximum 80 code points | PostgreSQL | member entry | separator, control, secret pattern, basename derivation | 3a916c601da772496c62132a022e782656685e400e63ebcba24c50fcd9b6da45 |
| field-global-search | no Twin raw or metadata integration | none | explicit Twin surfaces only | edits to ordinary global navigation search | 93eca7329bcfeffe647dd13ee871225bc09a7353b5892dee24f4c0874045989b |

<!-- twin-contract: source-policy -->
| id | source policy | local user search | persona generation | shared/remote/effect | revision behavior | semantic_sha256 |
| --- | --- | --- | --- | --- | --- | --- |
| policy-local-only | `local_only` | allowed in L0 | forbidden even for local generator | forbidden before ranking and handle resolution | permanent; approval cannot override | d6681a3517980732d6bb7b83e7b349ee59568b0e1d27c8418c78f40deb334b8f |
| policy-remote-allowed | `remote_allowed` | allowed | allowed with provenance | owner-attested, purpose-bound release | tightening revokes all dependents | fa411b8ef6b1b4a30cc14e9aa8d35ce80831e997c2b2f97224071569362aba4a |
| policy-unknown | omitted or unknown | reject import | reject | reject | no partial corpus | dc49e7ebb7600f162c9936c864f2f780cfba2e3ea57b7411ab5ebb359b386b55 |
| policy-tighten | new stricter revision | local bytes retained | old draft/version unusable | bindings/permits revoked; queued/preparing cancel; running hard-stop | atomic and immutable old bytes | dcdafc1271de83028f2cf4329d72d50e3f76ffeff35af7b529a6cb33050b0677 |
| policy-loosen | new looser revision | reindex permitted | full regeneration required | new sign/bind required | never resurrect old authorization | af0b76cf1022a4de986a590cbf64161c50ccb94499c258e2dfd4673907704108 |

The matrix is exhaustive: any new schema, API, event, or local-control field must
be added to exactly one placement class before implementation.
