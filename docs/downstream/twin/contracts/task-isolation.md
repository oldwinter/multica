# Twin-linked AgentTask isolation contract

`twin_operation_id IS NOT NULL` selects linked behavior. A linked-task guard
loads the immutable operation before any Task bytes or mutation and requires the
exact workspace+machine+owner+profile callback identity. Runtime capability must
match, while mutable `agent_runtime.owner_id` is ignored.

<!-- twin-contract: daemon-task-routes -->
| id | daemon surface | linked behavior | wrong profile | semantic_sha256 |
| --- | --- | --- | --- | --- |
| daemon-claim | single, batch, alias, WebSocket claim and runtime pending list | exact-profile linked scope only | hidden | 8012a7b10dba25caebb4488262d9b83781604bedebd7691efca281d796540783 |
| daemon-prepare | prepare lease and skill-bundle resolve | guard before result | hidden | 98af30d2dd3ded14a98148532e433a5d4c49eb21533cb4a1e4accef62fb88564 |
| daemon-status | status, start, progress, complete, fail, usage | safe Twin callbacks only | no mutation | bdf775bee763b4e335b4d6efb182e2452391a04b781f08276df500be48bf1a3d |
| daemon-wait-local | wait-local-directory | identity-check then reject before body decode | hidden/rejected | a64dca088d0eb4a96117eea13d2bc2e795364cae0e7c0b29790e6fcf97bdf485 |
| daemon-message | message write/read | identity-check then reject before body decode; local spool | hidden/rejected | 7729a4a35f9dd9f5ab226179f0f06622c19e4d377ffbd4795fe8984125c0f7a4 |
| daemon-cancel-gc | cancel acknowledgement and task GC check | safe metadata only | hidden/no mutation | 553befab149809a8d60a2f443708ef37b9efe3e10bb65a306485b57a54d8d306 |
| daemon-recovery | runtime orphan recovery and session pin | recovery is linked-safe; session pin rejects before decode | hidden/rejected | 3bb8ce580f6dd752d33e12912b60cb1528a6373975b9f7b878b1861f4e65b533 |
| daemon-terminal-locality | complete/fail | never accept/write ordinary `session_id` or `work_dir` | no mutation | 227ec4b70fd6fcf0d6020efc24124f4b82a656831c6ff9a269acf8a13672e6b3 |

<!-- twin-contract: ordinary-task-behavior -->
| id | consumer class | required predicate | linked result | semantic_sha256 |
| --- | --- | --- | --- | --- |
| ordinary-claim | account PAT/JWT/cloud claims, reclaim, retry, recovery, expiry, offline sweeper | `twin_operation_id IS NULL` in minimal selector | invisible | 6a1d14cf3f5e0a2abb91b239a4c30c7adf5905bae081ed8991d5d0eea8d41bff |
| ordinary-session | resume, rollout, continuity, comment anchor, task role, pending/active dedup | `twin_operation_id IS NULL` | not found before sibling/session lookup | 13d03c372c92086105727484dd0aa8d61925c3831dffb6454623eb484c982982 |
| ordinary-comments | coalescing, registration, delivered comments and pending chat input | `twin_operation_id IS NULL` | cannot deduplicate against or mutate linked row | bd5c52fcb97993edf56af4344cdfb96ef45f88ca30c48507a4ac1008e6cd16f5 |
| ordinary-mutations | single/bulk cancel, manual rerun, message read/write | `twin_operation_id IS NULL` before body decode | standard not found | 8a984e939e106c97b4ff8db8f4d26704058d469ff1738c20114493291f3cb3e4 |
| ordinary-projections | Issue/Agent/workspace/Squad activity/history/status and Issue-table working/facets | `twin_operation_id IS NULL` | excluded | aa22a2b3c3de093f58677ae8d01b5367c4ef38d69315d2a510b07b7f94274f6d |
| ordinary-aggregates | runtime/dashboard/Issue/Agent/business metrics and usage-duration-failure rollups | `twin_operation_id IS NULL` | excluded | f7ed736cef09a13ca0c601c62054f89b1aa5cd3918bdafbcd0ff3402bdccd3db |
| ordinary-storage | task_message, task_usage/hourly, ordinary lifecycle events and session continuity | ordinary rows only | zero writes | 5ea312838940610c425addad53d74f04235ec6518daa46b63a71ab7f5bf6db48 |
| linked-storage | Twin progress, metering, terminal event and raw local spool | Twin-owned persistence/realtime | no ordinary projection | 46bc9bcbabb4764ebdc40ada4fef57e7a9e4633801273e18394377363c7bb795 |
| rerun-source | ordinary manual rerun/resume source | source must be ordinary | linked source returns not found before cancellation/lookups | 71592a606a8c992c02632915f1ccb63cb02da57307e561e5b03b04fabc218814 |

<!-- twin-contract: indexes -->
| id | index contract | migration ownership | downgrade | semantic_sha256 |
| --- | --- | --- | --- | --- |
| index-ordinary-pending | ordinary-only partial unique pending index | Todo 8 exact adjacent up/down pair | safe only before linked enqueue proof | a5021acba2d912a5ce582204ab06dc7c9ecc81f68c40cd85e0de27fa8dadaedb |
| index-linked-operation | separate unique Twin operation key | Todo 8 exact adjacent up/down pair | conditional; never claim unconditional reversibility after linked enqueue | 9b9b683932dad68a65d222d7f3bc7f602ead6f07f98fab3a7ca4864b845b01c9 |
| index-file-count | exactly two up and two down pending-index files | Todo 8 | real runner validates both winner orders | 79ea4248f1d052ce0e5403cbfea8285bd04314f532c3ab267a85137043c0bae0 |
| index-enqueue-proof | linked enqueue locks runtime `FOR KEY SHARE`, revalidates binding, then verifies schema/index before insert | Todo 20 consumes committed Todo 8 bytes | rollback on drift | ab7d2ccb6677d7e065b39a3376e29ef06bdc10816dd5cdbed23942262453d7d9 |

<!-- twin-contract: task-consumers -->
| id | repository path | class | linked contract | semantic_sha256 |
| --- | --- | --- | --- | --- |
| consumer:server/cmd/backfill_codex_usage_cache/main.go | server/cmd/backfill_codex_usage_cache/main.go | aggregate | ordinary-only; identities=go:loadDryRunSummary,go:executeBackfill | ae06675ff601bff55e9e3f9e3a137817f43ac6840c66f5f7e2d75c53b2807340 |
| consumer:server/cmd/backfill_task_usage_hourly/main.go | server/cmd/backfill_task_usage_hourly/main.go | aggregate | ordinary-only | 8954fa4917739375ca079dd4f61ab5fd7929c6bf9183397d435a84ffb5428020 |
| consumer:server/internal/attribution/attribution.go | server/internal/attribution/attribution.go | shared | explicit linked actor from operation owner | 56105734a8b11de3b926509559688df76de66b8d8b7172f0b57def961f76c673 |
| consumer:server/internal/attributionbackfill/backfill.go | server/internal/attributionbackfill/backfill.go | shared | migration/backfill classification; identities=go:Hook | bf2a703ccd455589070ac34035b7d368f6c63083d7968bde5910821a2adcba55 |
| consumer:server/internal/daemon/client.go | server/internal/daemon/client.go | daemon | exact-profile or safe metadata | 359e157a5896d406f4cf14856fdb5ec4b3e4b8ab2353e84327b8341bd4fd3faa |
| consumer:server/internal/daemon/gc.go | server/internal/daemon/gc.go | daemon-gc | dependency recheck | 788735f7123759acd107797a64ffb5ea75de5405dd8aa6a284867b961f9519f8 |
| consumer:server/internal/handler/chat.go | server/internal/handler/chat.go | ordinary-user | ordinary-only before decode | dcc4a26ec31b247bd0cd65b155f9010238a0d8ad73ef5bd53f867f0e458c2d1f |
| consumer:server/internal/handler/daemon.go | server/internal/handler/daemon.go | daemon | exact-profile linked guard | 12e56be2980ed5acf911aa56efb15f7277355c18387a11bb44d08ebb117ba54a |
| consumer:server/internal/handler/dashboard.go | server/internal/handler/dashboard.go | aggregate | ordinary-only | 843e9599b32ed50f44c5b5aab3f97a15f90f3133737c7d451216cb51b796dad8 |
| consumer:server/internal/handler/issue.go | server/internal/handler/issue.go | ordinary-user | ordinary-only | dcf3682021e5a8083a1216cf71f2c3beabe878c23b614370ec45f1e5195c9fe2 |
| consumer:server/internal/handler/issue_table_facets.go | server/internal/handler/issue_table_facets.go | issue-table | ordinary-only; identities=go:Handler.issueTableFacetQuery | 5bb3b2f5085d3505f838459b6193acf99c7172ded996df88ee915a54a590d9fc |
| consumer:server/internal/handler/issue_table_query.go | server/internal/handler/issue_table_query.go | issue-table | ordinary-only; identities=go:Handler.compileIssueTableQuery | c91d4ba650e69d5a8233a7b419e7c19854786225c5a8fba0fda9d32e0fd5ebdd |
| consumer:server/internal/handler/runtime.go | server/internal/handler/runtime.go | topology | linked dependency preflight | 7f80748527960fd17dd8f6d5fefa6c09a3649784546b44f9f529649e898749a0 |
| consumer:server/internal/handler/squad.go | server/internal/handler/squad.go | squad | ordinary-only | 4026d692e67aba22d9702ef00f6297ced322242f6deb493eea917c7f1446938d |
| consumer:server/internal/handler/workspace_revoke.go | server/internal/handler/workspace_revoke.go | teardown | owner dependency preflight | 0900ea9c69bf16430285dc2c281796070ed93953668ff5adb11304c39a54c43e |
| consumer:server/internal/integrations/composio/dispatch.go | server/internal/integrations/composio/dispatch.go | ordinary-shared | ordinary-only or linked-safe overlay | 7b93a22c6d7938f50797712f25f6fe8e6bb3676500527ec75ea6c7d72869f4f2 |
| consumer:server/internal/metrics/business.go | server/internal/metrics/business.go | metric | ordinary-only | 8976f8355d83acc6e5c2e8ca4f43ed71882ece9f5422bbae25245ef8fbf35cbd |
| consumer:server/internal/metrics/business_sampler.go | server/internal/metrics/business_sampler.go | metric | ordinary-only | 03df9cd1e8f58c8e92752716f5fbc706f3d963254806709709d9e07964b927b0 |
| consumer:server/internal/metrics/business_sampler_queries.go | server/internal/metrics/business_sampler_queries.go | metric | ordinary-only; identities=go:BusinessSamplerCollector.queryActiveUsers,go:BusinessSamplerCollector.queryActiveWorkspaces,go:BusinessSamplerCollector.queryTaskQueued,go:BusinessSamplerCollector.queryTaskRunning,go:BusinessSamplerCollector.queryTaskStuck | 50dc6f7bb4aba77eb23ac7b1611207e31400aa7340db1acd93d02aef834d01ad |
| consumer:server/internal/metrics/labels.go | server/internal/metrics/labels.go | metric | ordinary-only | 50a94d704ce11fde58ef44cc8942a5574cf2c8f6bb3fc06259cfdcbbc7a88357 |
| consumer:server/internal/service/autopilot.go | server/internal/service/autopilot.go | ordinary-user | ordinary-only enqueue/dedup | c445ba062d3ed1197323dba33e21314fd1ae43ab18cfdba5075f61b78ad7de6f |
| consumer:server/internal/service/issue.go | server/internal/service/issue.go | ordinary-user | ordinary-only activity | 558e22bbb7f6766bbd8af92221ce1c0dc9fb9779f4b569c5d7fac532405c8b2e |
| consumer:server/internal/service/task.go | server/internal/service/task.go | shared | branch on linked guard; ordinary predicates | 9eccfb762e98b3c9dd9c0e1c9a242f5e957b6a5e00e2cf9ae4249d3e1a682728 |
| consumer:server/pkg/db/queries/agent.sql | server/pkg/db/queries/agent.sql | sql-shared | every selector/mutator classified; identities=query:ListAgentTasks,query:CreateAgentTask,query:CreateQuickCreateTask,query:CreateDeferredAgentTask,query:LinkTaskToIssue,query:CreateRetryTask,query:CancelAgentTasksByIssue,query:CancelAgentTasksByIssueAndAgent,query:CancelAgentTasksByAgent,query:CancelAgentTasksByTriggerComment,query:CancelAgentTasksByChatSession,query:GetAgentTask,query:GetAgentTaskInWorkspace,query:ClaimAgentTask,query:SetTaskDeliveredCommentIDs,query:RequeueAgentTaskAfterClaimFailure,query:ReclaimStaleDispatchedTaskForRuntime,query:ReclaimStaleDispatchedTasksForRuntimes,query:ExtendAgentTaskPrepareLease,query:StartAgentTask,query:MarkAgentTaskWaitingLocalDirectory,query:CompleteAgentTask,query:GetLastTaskSession,query:GetLatestTaskRolloutMissing,query:GetLatestChatTaskRolloutMissing,query:GetLastTaskStartedAtForIssueAndAgent,query:FailAgentTask,query:UpdateAgentTaskSession,query:RecoverOrphanedTasksForRuntime,query:FailStaleTasks,query:ExpireStaleQueuedTasks,query:CancelAgentTask,query:MarkChatFinalizeDeferred,query:ClaimChatFinalizeDeferred,query:ListChatFinalizeDeferredExpired,query:CountRunningTasks,query:HasActiveTaskForIssue,query:HasPendingTaskForIssue,query:HasPendingTaskForIssueAndAgent,query:HasPendingTaskForIssueAndAgentExcludingTriggerComment,query:MergeCommentIntoPendingTask,query:RegisterPlannedCommentForActiveTask,query:HasActiveTaskForIssueAndAgent,query:GetLatestTaskRoleForIssueAndAgent,query:ListPendingTasksByRuntime,query:ListQueuedClaimCandidatesByRuntime,query:PromoteDueDeferredTasksForRuntime,query:ListQueuedClaimCandidatesByRuntimes,query:PromoteDueDeferredTasksForRuntimes,query:CancelDeferredEscalationsForTask,query:CancelDeferredEscalationsForIssueAgent,query:ListActiveTasksByIssue,query:GetWorkspaceAgentRunCounts,query:GetWorkspaceAgentActivity30d,query:ListWorkspaceAgentTaskSnapshot,query:ListWorkspaceWorkingAgents,query:ListTasksByIssue,query:RefreshAgentStatusFromTasks | b5290a04440d21dee21319663c4e9c82745a2aebe95c84a525a8dc07360509b3 |
| consumer:server/pkg/db/queries/autopilot.sql | server/pkg/db/queries/autopilot.sql | sql-ordinary | ordinary-only; identities=query:CreateAutopilotTask,query:GetAutopilotTaskByRun | bebefaaa75bbe5fce4cb765d949fc9d0ea691d779e7da5a6e0a5f1da5accb0a1 |
| consumer:server/pkg/db/queries/chat.sql | server/pkg/db/queries/chat.sql | sql-ordinary | all mutation/session/active/pending consumers ordinary-only; identities=query:DeferChatTaskForSealedPendingMedia,query:CreateChatTask,query:PromoteChannelChatTasksIfMediaReady,query:SetChatTaskInputOwnerSelf,query:GetLastChatTaskSession,query:HasActiveChatTaskForSession,query:GetPendingChatTask,query:ListPendingChatTasksByCreator,query:HasPendingChatTasksByCreator,query:LockChatSessionForTask | bfd4409f64d3b6b43db8961cb4dc1768c468e1a54ed3d34f56a802e418bbad26 |
| consumer:server/pkg/db/queries/issue.sql | server/pkg/db/queries/issue.sql | sql-ordinary | ordinary-only activity | be37ea6f543238ba8454355e12dbaa427636cf25c2c23c3266acf50364385a52 |
| consumer:server/pkg/db/queries/runtime.sql | server/pkg/db/queries/runtime.sql | sql-topology | linked dependency predicates; identities=query:FailTasksForOfflineRuntimes,query:CancelAgentTasksByRuntimeOrAgent,query:ReassignTasksToRuntime | d0404f1993b7ce5ff22e158d8e9c1ce2dc1bc13a74dcd0114043837998d7273b |
| consumer:server/pkg/db/queries/runtime_usage.sql | server/pkg/db/queries/runtime_usage.sql | sql-aggregate | ordinary-only; identities=query:GetRuntimeTaskHourlyActivity,query:ListRuntimeUsageByAgent,query:GetRuntimeUsageByHour | e189435350f8660b2f333b2ca392dc3f7560e767c38de2a01aa1d14d386122e0 |
| consumer:server/pkg/db/queries/squad.sql | server/pkg/db/queries/squad.sql | sql-aggregate | ordinary-only; identities=query:ListSquadMemberStatusRows | f32f87bd507ff756c7453009d73efc668ce055d662343b4294241d23d8010a57 |
| consumer:server/pkg/db/queries/task_usage.sql | server/pkg/db/queries/task_usage.sql | sql-aggregate | ordinary-only; identities=query:GetIssueUsageSummary,query:ListDashboardRunTimeDaily,query:ListDashboardAgentRunTime,query:ListDashboardFailuresDaily,query:ListDashboardFailuresByAgent | c758cbf0e8d924ee444bc79c95b9a720acc5e7d8464adabd3f6c916dd577e48d |
| consumer:server/pkg/db/queries/workspace.sql | server/pkg/db/queries/workspace.sql | sql-teardown | coordinated dependency order | 4944796f4363af68640ff4b3c1cda2ece2808dd4db7ffb855ab3aca47a73523b |
| consumer:server/pkg/db/queries/workspace_delete.sql | server/pkg/db/queries/workspace_delete.sql | sql-teardown | explicit all-or-nothing cleanup; identities=query:PrepareWorkspaceDeletionLinks,query:DeleteWorkspaceLeafData,query:DeleteWorkspaceTasks | 1963181fd54edd9b3793c5e1fdb7d17e270127e93bc4b7e962952c615cdc5af8 |
| consumer:server/pkg/protocol/events.go | server/pkg/protocol/events.go | event | linked events use Twin schema | 7f3cfa2345a50154280c60decb94c2ed08d126a0e890f83347331e6aa3a006f7 |
| consumer:server/pkg/taskfailure/failure.go | server/pkg/taskfailure/failure.go | shared | safe failure reason only | 441f76b2aed85be3beb619306e84d9fb8540addd18d84913dde4eaa67c154fc0 |

Generated sqlc files, migrations, tests, comments-only documentation, and built-in
operator prose are derived/non-runtime inventory and are intentionally excluded.
