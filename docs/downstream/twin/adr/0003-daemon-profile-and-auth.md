# ADR 0003: Daemon profile and authentication

Status: Accepted

`daemon_id` identifies a machine and is shared by CLI profiles. Authorization is
the immutable tuple `(workspace_id, daemon_id, daemon_profile_id, owner_user_id)`.
No mutable runtime owner, process-local WebSocket presence, profile name, or
client-supplied field can replace that tuple.

<!-- twin-contract: auth-decisions -->
| id | decision | lock or proof | failure | semantic_sha256 |
| --- | --- | --- | --- | --- |
| auth-callback-identity | Every HTTP/RPC/WS Twin callback requires auth path plus exact workspace+machine+owner+profile and current callback generation | PostgreSQL pre-body check and durable connection lease | 401 or hidden row before body decode | 6158be23110274f447a38bb9d2b70930c6e339fe429ff89ce65a4bb8ee910fbe |
| auth-token-kind | Twin callbacks accept only `mdt_tw_` through `DaemonAuthPathTwinDaemonToken` | middleware plus operation/profile binding | PAT, JWT, cloud PAT, ordinary daemon token, and cache fallback reject | 54f7e7d937b48b8a5f252649273c22d4499f97c57c94af6a2f6cda2c8c07d4d0 |
| auth-machine-insufficient | machine-scoped `daemon_id` is capability routing, never actor authority | immutable profile binding | owner takeover and mutable `agent_runtime.owner_id` trust reject | f9bd6edb6d8da0c502298c5f9b572ed5fce5695a73a2a46364f8b80f167d387e |
| auth-local-control | profile-scoped Unix socket uses same-UID peer credentials, boot-rotated 256-bit secret, HMAC nonce, connection-bound methods | 0700 directory and 0600 socket/secret | renderer token exposure, stale session, and unsupported Windows reject | 70a0feb559758ed04b4d2d9a169a476f0c168110dec2b69d4129738bd3483e58 |
| auth-diagnostics-exception | ordinary `DaemonStatus.profile` main/preload/renderer diagnostics remain byte-compatible | ordinary flow only | any Twin API, IPC, event, or auth use of local profile name rejects | 7745f63dfa783b07db86d65fa6cade52354daa0d36eaeec1a07bb4d85011acf1 |

WebSocket identity is copied from the authenticated server context into the hub
and reconstructed for RPC. Linked wakeups target only the exact four-field
connection and reveal neither ID nor hint to account or sibling-profile sockets.
