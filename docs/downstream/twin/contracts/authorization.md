# Twin authorization contract

<!-- twin-contract: actions -->
| id | action | allowed actor | local attestation | no-leak result | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| action-create | create Twin; link readable Issue Topic; append user speech | active member; creator becomes immutable Twin and artifact-daemon owner | no | hidden cross-workspace | 8fed0403197ef86191cd28c7f50dcb6e8eed77ecf77d6a0f37ea28884a4697ce |
| action-central-lifecycle | generate, relabel, bind/unbind, tighten policy, and request central lifecycle cleanup | Twin owner or workspace owner/admin; only the Twin owner may authorize local source/input byte deletion | local byte deletion requires owner-daemon attestation | hidden cross-workspace | a0f6280135535f7a936c811ee262540514ea85f46ac55ddb537ca979326fa689 |
| action-local-content | acquire/edit/provider-auth, egress, uncertain resolution, sign, inspect/edit/accept/reject/commit deposition | Twin owner on owner daemon | action/digest/actor-bound one-use attestation where content leaves local custody | 403/hidden resource | 14a62510163955ed15d58cfacc6c16533a24f6b5e38ee9276c03725159e85186 |
| action-daemon-append | append Twin message or intent | exact owning daemon profile | callback tuple and generation | wrong profile hides row | a990cbae5f2b5d64e7fb4b57d0b279925d98fc57e05f441d3b1b177e61fa01e0 |
| action-cycle-control | manual dispatch/send-back/attested acceptance/archive | Topic creator, Twin owner, or owner/admin | owner-reviewed Run attestation for acceptance | no remote socket reach | 40bc87e80a0b3a258e226fa08be9d6b9ee6a31fbc679ea39d5ab64bb7fde8c3b |
| action-assume-cycle | assume cycle after actor membership loss | owner/admin | no | audit assumption | 36654e70569bad076b4492acedb992beac9962f62a59aa953a9146ebd497e866 |
| action-clear-run | clear archived Run | Run initiator, Twin owner, or owner/admin | no | terminal-only conflict | acc11ca0e5eeeb8e746959608f8ca84141c185119b806476bef45eb03008a7b8 |
| action-hard-stop | hard-stop or non-content rejection | Run initiator or owner/admin remotely | no | idempotent terminal result | 32d60b90f9168e92845207ba487429ca5929835f4dbdb2a6eb2ef44f35593d67 |
| action-budget-approve | approve exact current/proposed tick extension | Twin owner from either client | no content; exact tick display | other roles cannot approve | 6d813ebea870f251ce8695457a3d7c78dacac8e9401a5510f671378210fef4a9 |
| action-budget-decline | decline extension or hard-stop | Twin owner, Run initiator, or owner/admin | no | no provider spend | c6be4758c0111cc32a5ef13495d7df393d2321594d72d2c0de3309c463bde2e6 |
| action-run-accept | consume owner-reviewed aggregate attestation | Topic creator, Twin owner, or owner/admin | `run_review` exact aggregate/reviewer/action, one use | stale digest rejects | e1c8428d8d77cd541026ff240118eacd5d3921387ea2ac640ddc4d77c8978efa |
| action-workspace-delete | delete workspace | authenticated workspace owner only | no | admin deletion rejects | 5286c02353de2dd805fe5f63f53cd41a683216e178c27aba437da0325420b35a |
| action-post-delete | act on residue after workspace deletion | recorded authenticated former owner only | delete-only cleanup lease | no tenant/callback API access | 0c0fd40058487daee663c16458b1329b7d8c18ad0698bce1f0e3210c683687c9 |

<!-- twin-contract: review-attestations -->
| id | action | reviewer | consumer | binding | semantic_sha256 |
| --- | --- | --- | --- | --- | --- |
| attest-egress | content egress | Twin owner on owner daemon | same actor | operation, content digest, action, expiry | c4dcb81e61bab742529e7ce0fac5da5e65681d3d91fcd50a93bf1eafd0531ae8 |
| attest-uncertain | uncertain effect resolution | Twin owner on owner daemon | same actor | effect, proof digest, action, expiry | 249b44ed2a025f13c59a5afdfab66d2d685a8fb8e89d44f14f567a52d38a0b80 |
| attest-sign | version sign-off | Twin owner on owner daemon | same actor | artifact digest, action, expiry | 63983114e894db44ec0f1d06d519c67b9366feaca06ccf34a901be8bef00a5d7 |
| attest-deposition | deposition commit | Twin owner on owner daemon | same actor | candidate/diff digest, action, expiry | 324a36e1a28527b4aab73c57e335d6d368f61ea2d8f51554ce97b85bf32b53a6 |
| attest-run-review | aggregate Run acceptance | Twin owner on owner daemon | Topic creator, Twin owner, or owner/admin | ordered aggregate digest, reviewer, consumer role, one use | 59482e0955f8caa4dedaf78982ef0ec36eed600f87bb3f69aa9fefc7eb764987 |

All ordinary Issue/Agent/Chat task endpoints filter linked rows before lookup or
body decoding. Run initiator and admins cannot approve spending against another
person's provider credential, and no remote actor is ever the sole local approver.
