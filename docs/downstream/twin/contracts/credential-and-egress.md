# Credential, budget, egress, and reporting contract

<!-- twin-contract: exchange -->
| id | phase | input/locks | durable result | semantic_sha256 |
| --- | --- | --- | --- | --- |
| exchange-selector | bodyless `POST /api/daemon-token/exchange` outside Twin callback prefix | one bounded workspace UUID, machine UUID, profile UUID, idempotency UUID header plus `mul_` PAT owner | duplicate/missing/body/query rejects | 3b400cffaff5347a6f0b943b8dd42461c993960d09bd0968ddff5d934101dcec |
| exchange-preflight | cache-free PostgreSQL actor, PAT, active membership, immutable binding | no issuance mutation | invalid PAT/JWT/cloud/membership/binding rejects | cefaf3201f3c2251f0d17735d2b52439e51ce605a0afca9ae4af5415fa165e14 |
| exchange-postlock-auth | subscriber advisory; workspace `FOR KEY SHARE`; PAT `FOR UPDATE`; profile row; reload PAT+membership+workspace+machine+owner+profile+generation | only then rotate one 60-minute token | revocation-first fails; exchange-first revoker invalidates successor | 2fc216cefb47dc8fdd11886e132a248d7a4fb329cc9b445f80ddbecb9563c51c |
| exchange-serialize | partial unique concurrent token index | exact four-field identity and generation | one live successor | b579831639a6f9f647d7ff32cf0df241872e3fa347879543e75732d072f42497 |
| exchange-attempt | UUID key bound to PAT ID and exact request fingerprint | no plaintext; successor row/hash metadata only | committed once | 33aebfff2f587223cf00c4751ac827289278806b65f59c2923505b231330e0da |
| exchange-replay | same key and fingerprint | no rotation | typed 409 `exchange_committed` | 01c9592b844b03b48eafa3561b16ab8933115f5c893dcdbe68516321e5f194bb |
| exchange-conflict | same key, different fingerprint | no lookup leak | typed conflict | 78f7a7a8ead7e98abd88e65a01a858673780ee6c0a9c67d9f3611548a63c3d02 |
| exchange-recovery | lost response confirms same key, fsyncs fresh key, explicit recovery exchange | atomically replaces unknown successor | exactly one known token | 51f71ed3a0d912a50e3f9e420046f4894343c9e869996b8e9a27e5d1ea4e6a11 |
| exchange-client | callback credential record keyed by workspace/machine/profile/owner/generation | atomic 0600; refresh half-life/reconnect/401 validates the full identity and generation | retry only idempotent operation callbacks or specified recovery | b5d334fdfe1f4300e65db08de94dd4d4eca6562794f9ebcaf22e175e1ce20ce6 |

<!-- twin-contract: credential-store -->
| id | contract | exact value | failure/receipt | semantic_sha256 |
| --- | --- | --- | --- | --- |
| credential-module | adapter module and version | `github.com/zalando/go-keyring v0.2.8`, exact direct/transitive graph | module drift rejects | 355b47d09d8f533a6b1db35a1fdaf47f91648d373b7d79a4537d533c4bd8448b |
| credential-build | package/build scope | `server/pkg/twin/credentialstore`, Darwin-or-Linux build tags | unsupported platform unavailable | 2a488eb688da954301cf4621a7255a09f0f2f99aaa12a2c2d9f51d69aa914b30 |
| credential-key | service/account/value | `ai.multica.twin.provider-auth.v1`; `<daemon_profile_id>/<provider>/<provider_auth_profile_id>`; raw API key | no Multica envelope or central copy | bc1e2c61b45caf6bdaa0ae4c6af24e1d22c33ce4668d1d3bfeda31fb4d1ec9a6 |
| credential-helper | persistent set/get/revoke | short-lived owned helper, pipe-only secret, 4-second deadline | timeout/dismiss/missing/locked/corrupt/D-Bus/fingerprint mismatch fail closed | ae8abff9532f8ebbcc4b2028aa91771f01cbef161a6dacaacea367d2ea690193 |
| credential-scenario | real receipt ceiling | separate 7-second scenario ceiling | cleanup cannot mask helper timeout | fcef5a228030cf7e77d8f331f42fd166edc42498071d9807f9b66022a1bfafd2 |
| credential-headless | startup/background | session-memory import or already unlocked in-memory revision only | never prompt or background-delete | d0be844a2fdb29470828d3ae1dfa3f1d82bb84de49099a5ed7def98c3a18eaa4 |
| credential-macos | `/usr/bin/security` shipped behavior | set/get/revoke/replace/headless/corrupt/timeout/cleanup rows | encoded representation permitted; exact SHA receipt | 7f122f47657505f1da78b4b126f687e58ca5cd9ce245efd1bf6e37cba0f40ff9 |
| credential-linux | Secret Service | login collection if present, else default alias; record alias, selected collection, item paths | concrete collection-path proof, prompt/missing D-Bus rows | 4c11b3eacf787e1cc685c328847480c0caf4172fac4a88a9a5345c79af34234f |
| credential-workflow | automatic draft-PR workflow | read-only permissions, exact head checkout/TARGET_SHA/head_sha, SHA-named macOS/Linux artifacts | manual/local substitution rejects | 07cec2ac958ed554a2ff2bd10f2ca2b857cbf74b45fd83344511b2d7e0ab5ed6 |

<!-- twin-contract: model-budget-effects -->
| id | subject | contract | terminal/reconcile | semantic_sha256 |
| --- | --- | --- | --- | --- |
| brain-profile | Topic pins same-daemon Brain runtime, local auth profile/revision, signed version, model and pricing | Claude no-tools structured JSON; fresh stateless process; no fallback | offline/missing/revoked/unpriced becomes retryable blocked_runtime without reply | 17f8c15b735ea280be42b20b17903451aa71f3b8e3428930d093406330856214 |
| brain-budget | operation budget | USD 1.00 default, USD 5.00 maximum | immutable snapshot; exact metering | a55bf7956057f596bb171aa203dc67f53d8b204a287e13d389233f631faadbed |
| run-budget | cost representation | signed int64 ticks at 1e-10 USD; exact or explicit conservative rate | no float drift or unknown/unbounded admission | 3a03afe00c9d822c853a7f62a89947922f8b82c424af9949ee56a1aa2020bbd8 |
| budget-extension | single extension Ask | owner sees exact current/proposed ticks; owner alone approves; initiator/admin may decline/hard-stop | first valid terminal answer wins | 9817fa2e06f963f0aef48e136c18352c10f5eae0def521101a2911e332f122e6 |
| ask-time | intervention timing | 120-second ordinary stall default; content/egress Ask has no timeout | no silent approve or auto-expiry | 9ec00485bd725f2ff63a29f9073b2d5b211eca65ef4f105233c75996e52901f0 |
| model-gateway | only parent-runtime network endpoint | sanitized typed config and provider/profile fingerprint; key daemon-only | deny/hard-stop on unknown transport | 585c6628fe327c78524c3d993b60dae3c4ab5f1c36be35c4893b19ed715fd7e4 |
| effect-adapter | typed external effect | prepare permit, owner attestation, idempotency key, release, proof, reconcile | uncertain becomes unknown_outcome; no blind retry | 13a59884252d6ceb59e9ca60617eb56cc6d0b0e80c5e05848f293358541ef23e |
| run-input | input identity | empty, one safe `github_repo`, or opaque Twin input resource | path-bearing local_directory and credential locator reject | d049c39060fb4ab4762ce37325e4f2a1ef1abcaabb8cb3c1def3d603eeec9a43 |
| run-report | per-Run report | deterministic server record independent of Brain | always available at terminal state | f72164751c0dbdad5b1d69fdfd7437178421921e335c6478d45fe466bb7fadda |
| aggregate-report | once per complete cycle | authoritative ordered complete Run set and digests | Brain may only emit submit_for_acceptance; exactly once | 60f918801372ac5219740b4b2858b18b8ece63f402aaa41ff090c9d9549c37cd |
| deposition | candidates/diffs local | owner inspects/edits/accepts/rejects and owner-local attestation commits bounded projection | targeted or empty commit explicit; no remote sole approver | dbb071e8b8ac217a91c846349d857ef6774d9e89ecabcf8e469bb34bad198d79 |

Every egress/effect operation has explicit prepare, release, reconcile, and
terminal rows. Raw credentials, source bytes, paths, local profile names, and
unredacted config never enter child environments, SQL, events, logs, or receipts.
