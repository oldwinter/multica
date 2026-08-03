# Twin lifecycle, departure, and retention contract

<!-- twin-contract: lifecycle-controls -->
| id | control | exact surface and identity | lock/order | terminal and compensation | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| lifecycle-logout | reversible profile auth logout | PAT-only `POST /api/twin-profile/logout`; CLI/Desktop/Core caller carries workspace+machine+profile+owner | subscriber, workspace, PAT, profile; revoke generation and callback/connection leases while preserving the delete-only cleanup lease | remove callback file; preserve binding, profile ID, artifacts, delete-only lease | e3610fd7f0cb055df7f946698c5bd50b1048200fa590e302513011f34371d6a4 |
| lifecycle-retire | explicit online owner departure | Twin member route plus owner-daemon acknowledgement; idempotency key | subscriber first, workspace/profile/dependencies | revoke versions/bindings/inputs/asks; cleanup acknowledged; retired never auto-revives | 1d7f3e5d10ea620aa50b4f3b91c31110482f3f586e89769f4e6803640b6ca8ba |
| lifecycle-abandon | audited offline departure | Twin member route by authorized actor, exact owner/profile and operation ID | subscriber first; durable outbox and delete-only lease | `credential_residue_unknown`; no background keyring deletion | 5945a7865ec9794a8bc63699cfc3220c5f5fdf01310c444d0e55df6738a21a3e |
| lifecycle-member-revoke | member teardown | member revoke actor plus target owner identity | target subscriber first, then workspace/topology/profile/rows | callback tokens and owner-dependent usability revoked; outbox local cleanup | d0348641812335f2802d3642378d2eb762f7ed65100b2ada46f308ca3e4b9ad3 |
| lifecycle-profile-revoke | runtime/profile teardown | exact profile target | subscriber, topology, absent-row-safe profile guard, profile row | tokens/bindings/dependencies preflight; no orphan/cascade | 20814ce3dd3afcbaf2f4cb9d9c93e6699c455b5d8a5b3a3778874a2855a3095b |
| lifecycle-workspace-delete | owner-only deletion | authenticated deleting owner recorded as former owner | workspace/subscriber coordination and all-or-nothing dependency cleanup | live rows removed; minimal selector outbox and cleanup lease retained | d7fe9c13d40c4559d07f63ed9650ec1d3c9d08f1c6893382e989ff21ffbc43d9 |
| lifecycle-reinvite | later membership | new active member identity | ordinary membership path | does not revive retired/abandoned versions, bindings, inputs, asks, or tokens | f91be657db7de50ba3e7df5e42bc7c269a74fd6744e84eaefc876c7f74b60f99 |

<!-- twin-contract: cleanup-leases -->
| id | phase | authority | allowed operation | forbidden | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| cleanup-issue | issue durable delete-only lease before destructive auth loss | exact former owner/profile | claim by idempotency key | content or tenant mutation | 9062df0357f076d0e87b86ef7de8ad706e475e298eba4c07f67f168c74eedb51 |
| cleanup-claim | owner daemon claims while online | same UID/local session plus lease bearer | enumerate opaque artifact IDs | callback bootstrap or provider use | 5f04015a11d1ded35f1da7e7b244fe7ec931e35980aa9fd7621e20f812602134 |
| cleanup-ack | acknowledge each local deletion | exact lease and operation | advance durable receipt | shared content write | c44ed84ea26b012511c6547d6b81f25d52f146cbd56d47f3cff9f0d2af6a5e4c |
| cleanup-abandon | operator records irrecoverable offline residue | former owner authority | terminal audit with `credential_residue_unknown` | pretending credential deletion | c8b061eec9bab48d39e2472c78ba044f4edf501ebbf3f268bb5ccb78552a64c6 |
| cleanup-revoke | revoke after complete acknowledgement or explicit custody transfer | server reconciler under lock | delete lease metadata | revoke on ordinary logout | ca60c0dcf0fff4fff76d78b1a736161461ca4f2fb5f8484d0b76ef97aeec5d44 |

<!-- twin-contract: retention -->
| id | class | retention trigger | expiry | deletion authority | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| retention-signed | signed version/package/evidence proof | immutable while referenced; preserve audit after archive | no automatic expiry | explicit owner lifecycle with dependency preflight | 7b26571ece4836636614e7a4b00db7be702d16bf89b801ac1ff48e1bd24d89d1 |
| retention-run | branch/workspace/raw logs/candidates | retain through review, archive, and configured manual cleanup | no automatic expiry | Run initiator/Twin owner/admin after terminal archive | 57485b6b50a9e4664c788136a978418e03fecb721464b29c42865f26bd18ab3d |
| retention-staging | prepared artifact and reconciliation state | until commit or compensated reconciliation | no blind timer delete | operation reconciler | bcb3f322228cf159417cd6898ac9216b31d08335894d5919e91761040bdd73af |
| retention-audit | authorization, Ask, ledger, receipt, deletion residue | durable audit | no automatic expiry | policy-governed operator action | 04c073ff6bac8ce5455128c6de08620b39c5cf84090a7d703cd9ded50c02cc18 |
| retention-local-source | local source/input bytes | owner daemon until explicit deletion | no server expiry | Twin owner locally | 3a8adebb0a7860827afd5655e50e1f1bbf9bcbfdd320f9268fbeef56e038c665 |

Every operation has `prepare -> release -> reconcile -> terminal`; crashes resume
from durable operation state and compensation never invents missing ownership.
