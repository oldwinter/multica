# Runtime topology and race contract

<!-- twin-contract: topology -->
| id | participant | lock order | revalidation and outcome | semantic_sha256 |
| --- | --- | --- | --- | --- |
| topology-creator | every owner-dependent creator | member subscriber advisory first, workspace, topology/profile, runtime, Agent, dependency rows | membership/owner/profile remains exact before insert | 0b3c428579b76362ea19af31b7b93e90fef5b77de1e5a63f9cfc683e0c92acdd |
| topology-register | full daemon register request including `Runtimes` and `FailedProfiles` | topology; workspace+daemon advisory; sorted absent-row-safe profile guards; canonical/legacy runtimes UUID order; Agents; tasks | discover complete set and preflight every entry before any mutation; one qtx; publish after commit | f2f13368585c65c3b55b5f051a787c493164d2e29585007c3c99ed529141b7b2 |
| topology-register-writes | canonical upsert, inherited name, FailedProfiles upsert, Agent/task reassignment, legacy marker, guarded old-runtime deletion | same registration qtx | any first/last dependency conflict yields zero mutation | ef597bf267dd198803715471e95b4dca62cb3c73d4679f32c1fbe34bee6f546e |
| topology-enqueue | linked enqueue | pinned runtime `FOR KEY SHARE`, then binding/operation | post-lock runtime/profile/schema/index proof | 28ff7c88dc4cdf27a57110dcf7aea3c2cd25239df4256ef8f93e6be1d2c927d8 |
| topology-bind | user-Agent binding only | subscriber, workspace, runtime `FOR KEY SHARE`, Agent `FOR KEY SHARE`, profile/Twin/version | `Agent.kind='user'`; hidden builder/system ID rejects | 919051520d89c573a2a9dc8ab89f3ff7b99b7bc3767c512cdc3bcea695643a9e |
| topology-agent-move | runtime-changing `UpdateAgent` | topology; discover old; sorted old/new runtimes `FOR UPDATE`; Agent `FOR UPDATE`; locked actor permission and binding | old drift rolls back/restarts; bound returns whole-request 409 with no field changes | dc5dd5fe150ec89b81c8735f64b693a093d181c7bb94f5ebf9b76580233bb04e |
| topology-agent-qtx | main update, nullable clears, invocation-target replacement | one qtx after locked authorization | any late failure rolls back every field; publish only after commit | 1ec076665e5c04383e59d39e34ba60c4da6758500da4a20687c5222a783be61a |
| topology-builder | `SwitchAgentBuilderRuntime` | topology before existing chat-session lock | byte-compatible carrier/session lifecycle; never bindable | 8638afee9e06aab1d63345967150cf86bd06d515514a91e17cd53118ec376504 |
| topology-profile | profile register/update/delete | subscriber; locked admin membership; topology; exact absent-row-safe profile advisory; profile `FOR UPDATE` | monotonic microsecond revision; one retry on stale; publish after commit | b4b9230b9062db2e4c6e392d41537203fd6dfc5c5bad76d77f195e5f3c19223c |
| topology-gc | stale-runtime GC | main pool owner; workspace UUID order; one workspace advisory per transaction; topology/rows | post-lock linked/binding/dependency recheck; both enqueue/GC winner outcomes | c6de69e9c7a05c83e9d5f11ea8c3b2945649df67920c865d306deea140d3eada |
| topology-teardown | Agent/runtime/profile/member delete | subscriber first where owner-dependent; topology; runtime before Agent; dependency rows | conflict has zero cancel/archive/reassign/delete mutation | 01544b2754f496bf677fb32d2765e22f665af0cab8757c8f3f7adf52c802a0a4 |
| topology-issue-delete | single/batch Issue delete | set-lock all targets before cancellation/attachments | any Topic/Run/operation/linked Task returns whole-request 409 | de70fb90547378d69b7382c0d0c5db232090a2e24e7c3fe19e2ade319fe1b010 |

Runtime deletion repeats a no-linked-row predicate. Registration, binding,
enqueue, move, GC, and teardown cover both winner orders with bounded completion,
no deadlock, no dangling binding, and no post-rollback event.

<!-- twin-contract: realtime -->
| id | transport | identity/order | race contract | semantic_sha256 |
| --- | --- | --- | --- | --- |
| realtime-http | Twin callback | pre-body PostgreSQL token row, generation, workspace+machine+owner+profile | rotation/revocation rejects stale generation | 2b0bc4d5f4781a14d98f8b29b789a1b1e2bc8631392e86b6daf143ca4e6f88bd |
| realtime-ws | profile-scoped `mdt_tw_` connection | server-authenticated immutable identity retained across hub/RPC | no body/header override; durable lease current | 11b3f850e181645a16da7f0b3e78331577049d04991dea3ab83f10c6bae8aa62 |
| realtime-wakeup | linked enqueue | exact four-field connection only | account/sibling profile receives no task ID or hint | 433017086367d2f0bfd5ccb249f77680a76df8286addbc3f2ee1e5ac5286ced1 |
| realtime-ordinary | ordinary enqueue and account socket | existing bytes unchanged; ordinary-only claims | cannot opt into linked scope | 98ac85ff3333e470d5e883502053e1c178c85a9d8d5bf1c511de3ce0ab17ced1 |
| realtime-sequence | workspace Twin envelope | monotonic workspace sequence and reconnect gap recovery | duplicate idempotent; gap invalidates/refetches | b6108cb0fcf8479cc5ee4a2919f1897993140f6929ff7afae9afb905ed0e450e |
