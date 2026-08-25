package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/attributionbackfill"
	"github.com/multica-ai/multica/server/internal/dbstartup"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/migrations"
	"github.com/multica-ai/multica/server/internal/taskusagebackfill"
)

// preMigrationHook runs work that must happen before a specific migration is
// applied in the direction whose hook map selected it. Hooks are idempotent and
// must not depend on the migration loop's session-pinned advisory lock
// — they run on the pool, not on the loop's pinned conn, so they can
// safely acquire other session-level locks (e.g. advisory lock 4246
// for the task_usage hourly rollup).
//
// Returning an error aborts the migration run. The corresponding migration is
// not added to (up) or removed from (down) schema_migrations, so the next run
// retries the hook + migration.
type preMigrationHook func(ctx context.Context, pool *pgxpool.Pool) error

// migrationCondition decides whether a pending migration's SQL should execute.
// A false result still records the migration as applied (or rolled back), which
// lets one migration encode environment-specific DDL without blocking later
// versions. Conditions must be read-only and idempotent; schema changes belong
// in the migration file so they remain visible to review and rollback tooling.
type migrationCondition func(ctx context.Context, conn *pgxpool.Conn) (apply bool, reason string, err error)

// usableIndexRequirement describes the exact index shape that may replace a
// fallback. Checking only extension or relation existence is unsafe: historical
// pg_bigm migrations swallowed CREATE INDEX errors, and an interrupted
// concurrent build can leave an INVALID relation with the expected name.
type usableIndexRequirement struct {
	IndexRegclass string
	TableRegclass string
	AccessMethod  string
	OperatorClass string
	Expression    string
	Extension     string
}

var commentContentBigramIndex = usableIndexRequirement{
	IndexRegclass: "public.idx_comment_content_bigm",
	TableRegclass: "public.comment",
	AccessMethod:  "gin",
	OperatorClass: "gin_bigm_ops",
	Expression:    "lower(content)",
	Extension:     "pg_bigm",
}

// preMigrationHooks wires migration version → hook. The version key is
// the file basename without the `.up.sql` suffix, matching what
// `migrations.ExtractVersion` returns.
//
// MUL-2957: the v0.3.4 → current direct-upgrade path needs the hourly
// rollup seeded BEFORE migration 103 evaluates its fail-closed lag
// guard, because at `cmd/migrate up` time the server has not yet
// started so neither the legacy pg_cron job nor the new app scheduler
// can advance the watermark. The hook runs the same idempotent
// monthly-slice backfill that
// `cmd/backfill_task_usage_hourly` exposes to operators.
//
// MUL-4897 / GH #5544: migration 198 VALIDATEs the strict attribution
// constraint installed by 197, which drops migration 190's
// originator_source IS NULL exemption. Self-hosted databases never ran the
// out-of-band backfill that Multica's cloud did, so their legacy rows make
// 198 fail closed and the backend refuses to start. The hook reconciles
// those rows (accountable_user_id := originator_user_id) idempotently BEFORE
// VALIDATE, so a stuck-at-197 instance auto-heals on `migrate up` with no
// manual SQL. A higher-numbered migration cannot help — the instance never
// reaches a version above the failing 198.
//
// GH #6388: migration 257 builds a replacement unique index concurrently. A
// failed build can leave an INVALID relation that IF NOT EXISTS would otherwise
// mistake for a successful retry. The hook removes only that invalid leftover;
// migration 257 can then rebuild it while the valid v1 index remains in place.
//
// MUL-5823: migration 261 replaces the terminal-task partial index the same
// way, so it carries the same hazard — an INVALID v2 leftover recorded as
// success would let migration 262 drop the still-valid v1, leaving all four
// dashboard rollups on a full table scan.
// concurrentIndexCleanups maps a migration version to the index it builds with
// CREATE INDEX CONCURRENTLY. Every entry gets an invalid-index cleanup hook, so
// an interrupted build cannot be mistaken for success on retry.
//
// The mapping is data rather than individual hand-written hook registrations so a
// test can check each entry against the index its migration file actually
// creates — a typo here would be invisible at runtime, because a hook that names
// a nonexistent index is a silent no-op.
//
// MUL-5999: migrations 273–277 each build one index concurrently, three of them
// on hot tables (agent_task_queue is the largest table in the database). They
// carry the same hazard as 257 / 261: an interrupted build leaves an INVALID
// index of the same name, `IF NOT EXISTS` then skips the rebuild, the runner
// records the migration as applied, and the queries that need the index silently
// stay on a full scan — the exact regression these migrations exist to fix.
//
// MUL-6288: registration used to be opt-in per batch, so 316 / 317 / 326 / 328 /
// 330 / 331 shipped without a hook and the hazard came back. The map is now
// total — every up migration that builds an index concurrently is listed, the
// same invariant `concurrentDownIndexCleanups` already holds for rollbacks — and
// TestEveryConcurrentUpBuildHasCleanup fails the build if a new migration is
// added without its entry. Registering the already-applied historical
// migrations costs one to_regclass lookup each, and only on a database where
// they are still pending: a fresh self-hosted install, which is exactly where an
// interrupted build would otherwise leave a permanently unusable index.
var concurrentIndexCleanups = map[string]string{
	"035_task_queue_issue_id_index":                             "idx_agent_task_queue_issue_id",
	"067_task_queue_claim_candidate_index":                      "idx_agent_task_queue_claim_candidates",
	"074_task_usage_updated_at_index":                           "idx_task_usage_updated_at",
	"075_task_usage_created_at_index":                           "idx_task_usage_created_at",
	"078_task_usage_created_at_legacy_index":                    "idx_task_usage_created_at_legacy",
	"080_agent_task_queue_queued_index":                         "idx_agent_task_queue_queued_created_at",
	"106_member_user_workspace_index":                           "idx_member_user_workspace",
	"114_agent_task_queue_running_started_at_index":             "idx_agent_task_queue_running_started_at",
	"115_agent_runtime_last_seen_at_index":                      "idx_agent_runtime_last_seen_at",
	"119_user_created_at_index":                                 "idx_user_created_at",
	"125_agent_task_queue_dispatched_prepare_index":             "idx_agent_task_queue_dispatched_prepare",
	"135_comment_workspace_index":                               "idx_comment_workspace",
	"138_issue_title_trgm_index":                                "idx_issue_title_trgm",
	"139_issue_description_trgm_index":                          "idx_issue_description_trgm",
	"140_comment_content_trgm_index":                            "idx_comment_content_trgm",
	"141_project_title_trgm_index":                              "idx_project_title_trgm",
	"142_project_description_trgm_index":                        "idx_project_description_trgm",
	"143_agent_task_queue_chat_pending_v2":                      "idx_agent_task_queue_chat_pending_v2",
	"153_chat_pinned_agent_user_ws_index":                       "idx_chat_pinned_agent_user_ws",
	"156_chat_session_pinned_index":                             "idx_chat_session_pinned",
	"160_chat_message_input_owner_index":                        "idx_chat_message_input_owner",
	"165_attachment_task_id_index":                              "idx_attachment_task",
	"167_resource_label_namespace_index":                        "issue_label_workspace_type_name_lower_idx",
	"168_resource_label_type_index":                             "issue_label_workspace_type_idx",
	"169_agent_label_lookup_index":                              "agent_to_label_label_idx",
	"170_skill_label_lookup_index":                              "skill_to_label_label_idx",
	"172_agent_system_identity_index":                           "agent_system_identity_unique",
	"177_autopilot_run_webhook_delivery_index":                  "uq_autopilot_run_webhook_delivery",
	"178_webhook_delivery_queue_index":                          "idx_webhook_delivery_queue",
	"181_task_chat_finalize_deferred_index":                     "idx_task_chat_finalize_deferred",
	"183_chat_draft_restore_index":                              "idx_chat_draft_restore_session",
	"187_autopilot_rule_version_index":                          "idx_autopilot_rule_version_active",
	"192_issue_properties_gin_index":                            "idx_issue_properties_gin",
	"194_issue_property_workspace_name_index":                   "idx_issue_property_ws_name",
	"195_issue_property_workspace_index":                        "idx_issue_property_workspace",
	"200_inbox_archived_listing_index":                          "idx_inbox_recipient_archived_created",
	"201_inbox_active_by_issue_index":                           "idx_inbox_active_by_issue",
	"203_issue_workspace_assignee_index":                        "idx_issue_workspace_assignee",
	"204_issue_workspace_parent_index":                          "idx_issue_workspace_parent",
	"205_issue_workspace_position_index":                        "idx_issue_workspace_position",
	"208_client_usage_daily_unique_index":                       "client_usage_daily_identity_date_uidx",
	"210_client_usage_daily_query_index":                        "client_usage_daily_activity_client_user_idx",
	"211_client_usage_daily_workspace_index":                    "client_usage_daily_workspace_idx",
	"215_chat_session_project_index":                            "idx_chat_session_project",
	"217_vcs_connection_workspace_index":                        "idx_vcs_connection_workspace",
	"218_vcs_pull_request_workspace_index":                      "idx_vcs_pull_request_workspace",
	"219_vcs_pull_request_connection_index":                     "idx_vcs_pull_request_connection",
	"220_issue_vcs_pull_request_pr_index":                       "idx_issue_vcs_pull_request_pr",
	"221_vcs_commit_status_lookup_index":                        "idx_vcs_commit_status_lookup",
	"223_github_pr_check_run_pr_ordinal_index":                  "github_pull_request_check_run_pr_ordinal_idx",
	"228_channel_media_pending_object_key_index":                "channel_media_pending_object_storage_key_uidx",
	"230_channel_media_pending_object_claim_index":              "idx_channel_media_pending_object_claim",
	"231_agent_task_queue_terminal_completed_at_index":          "idx_agent_task_queue_terminal_completed_at",
	"232_channel_media_pending_object_due_index":                "idx_channel_media_pending_object_due",
	"233_agent_task_queue_agent_terminal_latest_index":          "idx_agent_task_queue_agent_terminal_latest",
	"238_quick_action_workspace_index":                          "idx_quick_action_workspace_status_usage",
	"241_comment_parent_lookup_index":                           "idx_comment_workspace_issue_parent",
	"244_issue_dependency_issue_index":                          "idx_issue_dependency_issue_id",
	"245_issue_dependency_depends_on_index":                     "idx_issue_dependency_depends_on_issue_id",
	"246_inbox_item_issue_index":                                "idx_inbox_item_issue_id",
	"247_comment_parent_index":                                  "idx_comment_parent_id",
	"248_agent_task_trigger_comment_index":                      "idx_agent_task_queue_trigger_comment_id",
	"252_twin_profile_workspace_index":                          "twin_profile_workspace_id_idx",
	"253_twin_profile_id_index":                                 "twin_profile_pkey",
	"255_agent_task_queue_chat_pending_deferred_v3":             "idx_agent_task_queue_chat_pending_v3",
	"256_wiki_page_id_index":                                    "wiki_page_pkey",
	"257_agent_task_queue_channel_media_pending_unique_v2":      "idx_one_pending_task_per_issue_agent_v2",
	"257_wiki_page_workspace_list_index":                        "wiki_page_workspace_scope_idx",
	"258_wiki_page_workspace_path_unique":                       "wiki_page_workspace_path_uidx",
	"259_wiki_page_project_path_unique":                         "wiki_page_project_path_uidx",
	"260_wiki_page_user_path_unique":                            "wiki_page_user_path_uidx",
	"261_agent_task_queue_terminal_completed_at_v2":             "idx_agent_task_queue_terminal_completed_at_v2",
	"264_wiki_page_user_path_global_unique":                     "wiki_page_user_path_uidx",
	"266_lm_wiki_revision_id_index":                             "lm_wiki_revision_id_uidx",
	"266_issue_view_owner_index":                                "idx_issue_view_owner",
	"267_lm_wiki_citation_id_index":                             "lm_wiki_citation_id_uidx",
	"267_issue_view_shared_index":                               "idx_issue_view_shared",
	"268_lm_wiki_review_id_index":                               "lm_wiki_review_id_uidx",
	"270_lm_wiki_revision_number_index":                         "lm_wiki_revision_workspace_number_uidx",
	"271_lm_wiki_revision_created_index":                        "lm_wiki_revision_workspace_created_idx",
	"272_lm_wiki_citation_source_index":                         "lm_wiki_citation_workspace_source_uidx",
	"273_lm_wiki_citation_ordinal_index":                        "lm_wiki_citation_workspace_ordinal_uidx",
	"273_agent_task_queue_runtime_id_index":                     "idx_agent_task_queue_runtime_id",
	"274_lm_wiki_review_revision_index":                         "lm_wiki_review_workspace_revision_uidx",
	"274_task_token_workspace_id_index":                         "idx_task_token_workspace_id",
	"275_task_token_agent_id_index":                             "idx_task_token_agent_id",
	"276_chat_draft_restore_task_id_index":                      "idx_chat_draft_restore_task_id",
	"276_twin_proposal_id_index":                                "twin_proposal_id_uidx",
	"277_autopilot_run_task_id_index":                           "idx_autopilot_run_task_id",
	"277_twin_proposal_review_id_index":                         "twin_proposal_review_id_uidx",
	"278_agent_task_queue_agent_id_keyset_index":                "idx_agent_task_queue_agent_id_keyset",
	"278_twin_version_id_index":                                 "twin_version_id_uidx",
	"279_agent_task_queue_issue_id_keyset_index":                "idx_agent_task_queue_issue_id_keyset",
	"280_twin_proposal_identity_index":                          "twin_proposal_workspace_identity_uidx",
	"281_agent_workspace_id_keyset_index":                       "idx_agent_workspace_id_keyset",
	"281_twin_proposal_created_index":                           "twin_proposal_workspace_created_idx",
	"282_issue_workspace_id_keyset_index":                       "idx_issue_workspace_id_keyset",
	"282_twin_proposal_review_index":                            "twin_proposal_review_workspace_proposal_uidx",
	"283_agent_runtime_workspace_id_keyset_index":               "idx_agent_runtime_workspace_id_keyset",
	"283_twin_version_number_index":                             "twin_version_workspace_number_uidx",
	"284_twin_version_proposal_index":                           "twin_version_workspace_proposal_uidx",
	"286_plugin_identity_key_index":                             "idx_plugin_identity_key",
	"286_room_id_index":                                         "room_id_uidx",
	"287_plugin_release_version_index":                          "idx_plugin_release_version",
	"287_room_participant_id_index":                             "room_participant_id_uidx",
	"288_plugin_installation_workspace_plugin_index":            "idx_plugin_installation_workspace_plugin_active",
	"288_room_entry_id_index":                                   "room_entry_id_uidx",
	"289_plugin_contribution_key_index":                         "idx_plugin_contribution_release_key",
	"289_room_cycle_id_index":                                   "room_cycle_id_uidx",
	"290_plugin_contribution_ordinal_index":                     "idx_plugin_contribution_release_ordinal",
	"290_room_turn_id_index":                                    "room_turn_id_uidx",
	"291_plugin_grant_revision_index":                           "idx_plugin_grant_revision",
	"291_room_artifact_id_index":                                "room_artifact_id_uidx",
	"292_plugin_binding_revision_index":                         "idx_plugin_binding_revision",
	"293_plugin_installation_workspace_index":                   "idx_plugin_installation_workspace",
	"293_room_workspace_index":                                  "room_workspace_idx",
	"294_room_participant_identity_index":                       "room_participant_identity_uidx",
	"295_plugin_artifact_file_index":                            "idx_plugin_artifact_file_release_path",
	"295_room_entry_ordinal_index":                              "room_entry_ordinal_uidx",
	"296_plugin_snapshot_revision_index":                        "idx_plugin_snapshot_workspace_revision",
	"296_room_cycle_sequence_index":                             "room_cycle_sequence_uidx",
	"297_plugin_execution_task_index":                           "idx_plugin_execution_manifest_task",
	"297_room_cycle_wake_index":                                 "room_cycle_wake_uidx",
	"298_plugin_health_index":                                   "idx_plugin_health_installation_observed",
	"298_room_active_cycle_index":                               "room_active_cycle_uidx",
	"299_agent_task_plugin_manifest_index":                      "idx_agent_task_plugin_execution_manifest",
	"299_room_turn_participant_index":                           "room_turn_participant_uidx",
	"300_room_artifact_kind_index":                              "room_artifact_kind_uidx",
	"301_room_due_index":                                        "room_due_idx",
	"302_agent_task_room_turn_index":                            "agent_task_room_turn_attempt_uidx",
	"303_room_artifact_room_index":                              "room_artifact_room_idx",
	"304_room_turn_room_index":                                  "room_turn_room_idx",
	"305_dingtalk_group_route_installation_conversation_unique": "idx_dingtalk_group_route_installation_conversation",
	"305_room_participant_room_index":                           "room_participant_room_idx",
	"306_dingtalk_group_route_workspace_index":                  "idx_dingtalk_group_route_workspace",
	"307_agent_task_room_turn_lookup_index":                     "agent_task_room_turn_idx",
	"307_dingtalk_group_route_id_unique":                        "idx_dingtalk_group_route_id_unique",
	"308_room_entry_turn_result_index":                          "room_entry_turn_result_uidx",
	"384_create_dingtalk_group_presence_identity_index":         "idx_dingtalk_group_presence_installation_conversation",
	"385_create_dingtalk_group_presence_activity_index":         "idx_dingtalk_group_presence_workspace_activity",
	"388_create_dingtalk_bot_identity_installation_index":       "idx_dingtalk_bot_identity_installation",
	"309_agent_runtime_id_index":                                "idx_agent_runtime_id",
	"311_plugin_identity_scoped_key_index":                      "idx_plugin_identity_scoped_key",
	"316_workspace_mcp_server_name_unique":                      "idx_workspace_mcp_server_workspace_name",
	"317_agent_mcp_server_server_index":                         "idx_agent_mcp_server_server",
	"320_plugin_installation_config_revision_index":             "idx_plugin_installation_config_contribution_revision",
	"321_plugin_installation_config_workspace_index":            "idx_plugin_installation_config_workspace",
	"322_plugin_remote_mcp_secret_revision_index":               "idx_plugin_remote_mcp_secret_revision",
	"323_plugin_remote_mcp_secret_workspace_index":              "idx_plugin_remote_mcp_secret_workspace",
	"324_plugin_remote_mcp_one_active_secret_index":             "idx_plugin_remote_mcp_one_active_secret",
	"326_plugin_remote_mcp_oauth_state_expiry_index":            "plugin_remote_mcp_oauth_state_expiry_idx",
	"328_workspace_share_link_id_uidx":                          "workspace_share_link_pkey_uidx",
	"330_workspace_share_link_active_ws_uidx":                   "workspace_share_link_active_ws_uidx",
	"331_workspace_share_link_code_uidx":                        "workspace_share_link_code_uidx",
	"333_issue_status_pkey_index":                               "issue_status_pkey_uidx",
	"335_issue_status_workspace_key_index":                      "idx_issue_status_workspace_key",
	"336_issue_status_workspace_name_index":                     "idx_issue_status_workspace_name_active",
	"343_comment_delegated_failure_pending_index":               "idx_comment_delegated_failure_pending",
	"345_plugin_installation_workspace_key_index":               "idx_plugin_installation_workspace_key",
	"346_plugin_storage_scope_key_index":                        "idx_plugin_storage_scope_key",
	"347_plugin_secret_installation_key_index":                  "idx_plugin_secret_installation_key",
	"349_agent_task_queue_chat_terminal_resume_index":           "idx_agent_task_queue_chat_terminal_resume",
	"350_agent_task_queue_chat_retired_session_index":           "idx_agent_task_queue_chat_retired_session",
	"353_autopilot_quota_period_scope_index":                    "uq_autopilot_quota_period_scope",
	"354_autopilot_quota_reservation_id_index":                  "autopilot_quota_reservation_pkey_uidx",
	"355_autopilot_quota_reservation_key_index":                 "uq_autopilot_quota_reservation_key",
	"356_autopilot_run_quota_reservation_index":                 "uq_autopilot_run_quota_reservation",
	"357_webhook_delivery_replay_idempotency_index":             "uq_webhook_delivery_replay_idempotency",
	"358_autopilot_quota_reservation_state_index":               "idx_autopilot_quota_reservation_state",
	"361_issue_last_activity_index":                             "idx_issue_workspace_last_activity",
	"363_plugin_invocation_installation_index":                  "idx_plugin_invocation_installation_created",
	"364_plugin_invocation_created_at_index":                    "idx_plugin_invocation_created_at",
	"378_channel_chat_context_generation_key":                   "channel_chat_context_generation_session_revision_idx",
	"390_agent_task_queue_dispatched_reclaim_v2_index":          "idx_agent_task_queue_dispatched_reclaim_v2",
	"393_plugin_package_workspace_key_index":                    "idx_plugin_package_workspace_key",
	"394_plugin_package_version_unique_index":                   "idx_plugin_package_version_unique",
	"395_plugin_package_version_package_index":                  "idx_plugin_package_version_package",
	"396_plugin_package_file_path_index":                        "idx_plugin_package_file_path",
	"397_plugin_installation_package_version_index":             "idx_plugin_installation_package_version",
	"398_issue_workspace_status_position_index":                 "idx_issue_workspace_status_position",
	"400_plugin_hook_schedule_installation_key_index":           "idx_plugin_hook_schedule_installation_key",
	"401_plugin_hook_schedule_enabled_index":                    "idx_plugin_hook_schedule_enabled",
	"404_room_memory_revision_id_index":                         "room_memory_revision_id_uidx",
	"406_room_memory_revision_version_index":                    "room_memory_revision_version_uidx",
	"407_room_cycle_phase_index":                                "room_cycle_phase_idx",
	"408_room_artifact_memory_revision_index":                   "room_artifact_memory_revision_idx",
	"410_room_recommendation_review_id_index":                   "room_recommendation_review_id_uidx",
	"412_room_recommendation_review_identity_index":             "room_recommendation_review_identity_uidx",
	"415_wiki_page_revision_id_index":                           "wiki_page_revision_id_uidx",
	"416_wiki_page_revision_number_index":                       "wiki_page_revision_page_number_uidx",
	"417_wiki_page_revision_list_index":                         "wiki_page_revision_page_created_idx",
	"418_wiki_page_search_index":                                "wiki_page_search_fts_idx",
	"419_wiki_page_proposal_id_index":                           "wiki_page_edit_proposal_id_uidx",
	"420_wiki_page_proposal_idempotency_index":                  "wiki_page_edit_proposal_idempotency_uidx",
	"421_wiki_page_proposal_list_index":                         "wiki_page_edit_proposal_page_status_idx",
	"422_lm_wiki_source_policy_index":                           "lm_wiki_source_policy_workspace_uidx",
	"423_lm_wiki_source_page_index":                             "lm_wiki_source_wiki_page_identity_uidx",
	"425_twin_binding_id_index":                                 "twin_binding_id_uidx",
	"426_twin_task_attribution_id_index":                        "twin_task_attribution_id_uidx",
	"427_twin_run_feedback_id_index":                            "twin_run_feedback_id_uidx",
	"428_twin_deposition_id_index":                              "twin_deposition_id_uidx",
	"430_twin_binding_scope_index":                              "twin_binding_workspace_scope_uidx",
	"431_twin_task_attribution_claim_index":                     "twin_task_attribution_claim_uidx",
	"432_twin_run_feedback_task_index":                          "twin_run_feedback_workspace_task_uidx",
	"433_twin_deposition_proposal_index":                        "twin_deposition_workspace_proposal_uidx",
	"434_twin_deposition_task_index":                            "twin_deposition_workspace_task_idx",
	"439_twin_proposal_identity_partial_index":                  "twin_proposal_workspace_identity_uidx",
	"440_twin_deposition_request_index":                         "twin_deposition_workspace_request_uidx",
	"443_twin_proposal_replacement_index":                       "twin_proposal_workspace_replacement_uidx",
	"445_room_turn_kind_attempt_index":                          "room_turn_kind_attempt_uidx",
	"447_room_synthesis_retry_key_index":                        "room_synthesis_retry_key_uidx",
	"450_room_memory_review_key_index":                          "room_memory_review_key_uidx",
}

// concurrentDownIndexCleanups covers every migration whose down direction
// rebuilds an index with CREATE INDEX CONCURRENTLY. An interrupted rollback
// can leave an INVALID relation behind. IF NOT EXISTS would then silently skip
// the retry, while a bare CREATE would stay wedged on "already exists"; both
// cases need direction-specific cleanup before the rollback can retry safely.
var concurrentDownIndexCleanups = map[string]string{
	"444_room_turn_identity_index_drop":                     "room_turn_participant_uidx",
	"144_drop_agent_task_queue_chat_pending_v1":             "idx_agent_task_queue_chat_pending",
	"171_drop_legacy_label_namespace_index":                 "issue_label_workspace_name_lower_idx",
	"256_drop_agent_task_queue_chat_pending_v2":             "idx_agent_task_queue_chat_pending_v2",
	"258_drop_pending_issue_agent_v1":                       "idx_one_pending_task_per_issue_agent",
	"262_drop_agent_task_queue_terminal_completed_at_v1":    "idx_agent_task_queue_terminal_completed_at",
	"263_wiki_page_user_path_unique_drop":                   "wiki_page_user_path_uidx",
	"300_drop_redundant_issue_workspace_number_index":       "idx_issue_workspace_number",
	"301_drop_redundant_sys_cron_job_plan_index":            "idx_sys_cron_exec_job_plan",
	"302_drop_redundant_channel_chat_session_binding_index": "idx_channel_chat_session_binding_session",
	"303_drop_redundant_lark_chat_session_binding_index":    "idx_lark_chat_session_binding_session",
	"312_drop_global_plugin_identity_key_index":             "idx_plugin_identity_key",
	"371_comment_content_search_index_strategy":             "idx_comment_content_trgm",
	"375_drop_issue_last_activity_index":                    "idx_issue_workspace_last_activity",
	"391_drop_agent_task_queue_dispatched_prepare_index":    "idx_agent_task_queue_dispatched_prepare",
	"438_drop_twin_proposal_identity_index":                 "twin_proposal_workspace_identity_uidx",
}

var preMigrationHooks = func() map[string]preMigrationHook {
	hooks := map[string]preMigrationHook{
		"103_drop_legacy_daily_rollups":                         runTaskUsageHourlyHook,
		"198_agent_task_attribution_strict_constraint_validate": runAttributionStrictHook,
	}
	for version, index := range concurrentIndexCleanups {
		hooks[version] = cleanupInvalidConcurrentIndexHook(index)
	}
	return hooks
}()

var preRollbackHooks = func() map[string]preMigrationHook {
	hooks := make(map[string]preMigrationHook, len(concurrentDownIndexCleanups))
	for version, index := range concurrentDownIndexCleanups {
		hooks[version] = cleanupInvalidConcurrentIndexHook(index)
	}
	hooks["438_drop_twin_proposal_identity_index"] = chainMigrationHooks(
		hooks["438_drop_twin_proposal_identity_index"],
		blockTwinProposalIdentityRollbackHook(pgx.Identifier{"public", "twin_proposal"}.Sanitize()),
	)

	// Migration 418 restores the v1 one-turn-per-participant invariant. A v2
	// database may legitimately contain retries or synthesis attempts with the
	// same (cycle_id, agent_id), so never let CREATE UNIQUE INDEX discover that
	// incompatibility after starting the build. Clean up an interrupted prior
	// build first, then fail with an actionable, data-preserving diagnostic.
	const roomTurnIdentityVersion = "444_room_turn_identity_index_drop"
	hooks[roomTurnIdentityVersion] = chainMigrationHooks(
		hooks[roomTurnIdentityVersion],
		preflightRoomTurnIdentityRollback,
	)
	return hooks
}()

func chainMigrationHooks(hooks ...preMigrationHook) preMigrationHook {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			if err := hook(ctx, pool); err != nil {
				return err
			}
		}
		return nil
	}
}

// Migration 412's down direction restores the old all-kind proposal identity
// index. Deposition replacements intentionally reuse that old identity, so a
// database with append-only replacement history cannot safely represent the
// pre-413 schema. Fail before starting the concurrent index build instead of
// leaving an INVALID index and an opaque unique-violation retry loop.
func blockTwinProposalIdentityRollbackHook(tableName string) preMigrationHook {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		var conflicts bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM `+tableName+`
				GROUP BY workspace_id, kind, source_wiki_revision_id,
				         COALESCE(base_twin_version_id, '00000000-0000-0000-0000-000000000000'::uuid)
				HAVING count(*) > 1
			)
		`).Scan(&conflicts)
		if err != nil {
			return fmt.Errorf("inspect Twin proposal history before rollback: %w", err)
		}
		if conflicts {
			return errors.New("rollback blocked: Twin proposal replacement history cannot be represented by the pre-413 identity index; remain on migration 413+ or use an approved data migration")
		}
		return nil
	}
}

func preflightRoomTurnIdentityRollback(ctx context.Context, pool *pgxpool.Pool) error {
	var duplicateGroups, turnCount int64
	var cycleID, agentID string
	err := pool.QueryRow(ctx, `
		WITH duplicate_turns AS (
			SELECT cycle_id, agent_id, count(*) AS turn_count
			FROM room_turn
			GROUP BY cycle_id, agent_id
			HAVING count(*) > 1
		), ranked_duplicates AS (
			SELECT cycle_id, agent_id, turn_count, count(*) OVER () AS duplicate_groups
			FROM duplicate_turns
			ORDER BY turn_count DESC, cycle_id, agent_id
			LIMIT 1
		)
		SELECT duplicate_groups, cycle_id::text, agent_id::text, turn_count
		FROM ranked_duplicates
	`).Scan(&duplicateGroups, &cycleID, &agentID, &turnCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Room turns before restoring the v1 identity index: %w", err)
	}

	return fmt.Errorf(
		"cannot roll back 444_room_turn_identity_index_drop: room_turn contains %d duplicate (cycle_id, agent_id) group(s), including cycle_id=%s agent_id=%s with %d turns; the v1 unique index cannot represent v2 retry/synthesis history. Recovery: keep the v2 schema, or back up each duplicate group plus agent_task_queue.room_turn_id, room_entry.turn_id, room_artifact.turn_id, room_cycle.synthesis_turn_id, and room_memory_revision.synthesis_turn_id references; choose one legacy-compatible turn per cycle/agent; archive or relink every dependent row before removing extra turns; then verify `SELECT cycle_id, agent_id, count(*) FROM room_turn GROUP BY cycle_id, agent_id HAVING count(*) > 1` returns no rows and rerun migrate down",
		duplicateGroups,
		cycleID,
		agentID,
		turnCount,
	)
}

var upMigrationConditions = map[string]migrationCondition{
	// Fresh databases that successfully built the CJK-friendly bigram index do
	// not need to build the trigram fallback only to remove it at migration 371.
	"140_comment_content_trgm_index": whenIndexNotUsable(commentContentBigramIndex),
	// Existing pg_bigm deployments already have both indexes. Remove the
	// fallback only after proving the preferred index has the exact usable shape;
	// pg_bigm-less self-hosted databases keep trgm and record 371 as a no-op.
	"371_comment_content_search_index_strategy": whenIndexUsable(commentContentBigramIndex),
}

// legacyMigrationVersionAliases preserves the schema_migrations identities of
// the four downstream outcome-loop features across two historical renumbering
// passes. Those filenames reached self-hosted databases before upstream added
// migrations in the same numeric range. Replaying the renamed SQL is unsafe:
// most statements intentionally are not idempotent and would fail on existing
// columns, tables, or indexes.
//
// Keep this map explicit and append-only. A match means the exact migration was
// already applied under an older filename; runMigrations records the current
// identity without executing its SQL. New migrations must use a fresh prefix
// and must never be added here merely to avoid resolving a real DDL conflict.
var legacyMigrationVersionAliases = map[string][]string{
	"403_room_outcome_lifecycle":                    {"400_room_outcome_lifecycle", "377_room_outcome_lifecycle"},
	"404_room_memory_revision_id_index":             {"401_room_memory_revision_id_index", "378_room_memory_revision_id_index"},
	"405_room_memory_revision_primary_key":          {"402_room_memory_revision_primary_key", "379_room_memory_revision_primary_key"},
	"406_room_memory_revision_version_index":        {"403_room_memory_revision_version_index", "380_room_memory_revision_version_index"},
	"407_room_cycle_phase_index":                    {"404_room_cycle_phase_index", "381_room_cycle_phase_index"},
	"408_room_artifact_memory_revision_index":       {"405_room_artifact_memory_revision_index", "382_room_artifact_memory_revision_index"},
	"409_room_recommendation_review":                {"406_room_recommendation_review", "383_room_recommendation_review"},
	"410_room_recommendation_review_id_index":       {"407_room_recommendation_review_id_index", "384_room_recommendation_review_id_index"},
	"411_room_recommendation_review_primary_key":    {"408_room_recommendation_review_primary_key", "385_room_recommendation_review_primary_key"},
	"412_room_recommendation_review_identity_index": {"409_room_recommendation_review_identity_index", "386_room_recommendation_review_identity_index"},
	"413_user_appearance_preferences":               {"410_user_appearance_preferences", "387_user_appearance_preferences"},
	"414_wiki_knowledge_loop":                       {"420_wiki_knowledge_loop", "388_wiki_knowledge_loop"},
	"415_wiki_page_revision_id_index":               {"421_wiki_page_revision_id_index", "389_wiki_page_revision_id_index"},
	"416_wiki_page_revision_number_index":           {"422_wiki_page_revision_number_index", "390_wiki_page_revision_number_index"},
	"417_wiki_page_revision_list_index":             {"423_wiki_page_revision_list_index", "391_wiki_page_revision_list_index"},
	"418_wiki_page_search_index":                    {"424_wiki_page_search_index", "392_wiki_page_search_index"},
	"419_wiki_page_proposal_id_index":               {"425_wiki_page_proposal_id_index", "393_wiki_page_proposal_id_index"},
	"420_wiki_page_proposal_idempotency_index":      {"426_wiki_page_proposal_idempotency_index", "394_wiki_page_proposal_idempotency_index"},
	"421_wiki_page_proposal_list_index":             {"427_wiki_page_proposal_list_index", "395_wiki_page_proposal_list_index"},
	"422_lm_wiki_source_policy_index":               {"428_lm_wiki_source_policy_index", "396_lm_wiki_source_policy_index"},
	"423_lm_wiki_source_page_index":                 {"429_lm_wiki_source_page_index", "397_lm_wiki_source_page_index"},
	"424_twin_execution_tables":                     {"430_twin_execution_tables", "398_twin_execution_tables"},
	"425_twin_binding_id_index":                     {"431_twin_binding_id_index", "399_twin_binding_id_index"},
	"426_twin_task_attribution_id_index":            {"432_twin_task_attribution_id_index", "400_twin_task_attribution_id_index"},
	"427_twin_run_feedback_id_index":                {"433_twin_run_feedback_id_index", "401_twin_run_feedback_id_index"},
	"428_twin_deposition_id_index":                  {"434_twin_deposition_id_index", "402_twin_deposition_id_index"},
	"429_twin_execution_primary_keys":               {"435_twin_execution_primary_keys", "403_twin_execution_primary_keys"},
	"430_twin_binding_scope_index":                  {"436_twin_binding_scope_index", "404_twin_binding_scope_index"},
	"431_twin_task_attribution_claim_index":         {"437_twin_task_attribution_claim_index", "405_twin_task_attribution_claim_index"},
	"432_twin_run_feedback_task_index":              {"438_twin_run_feedback_task_index", "406_twin_run_feedback_task_index"},
	"433_twin_deposition_proposal_index":            {"439_twin_deposition_proposal_index", "407_twin_deposition_proposal_index"},
	"434_twin_deposition_task_index":                {"440_twin_deposition_task_index", "408_twin_deposition_task_index"},
	"435_twin_schema_v2_and_deposition":             {"441_twin_schema_v2_and_deposition", "409_twin_schema_v2_and_deposition"},
	"436_agent_task_twin_use_snapshot":              {"442_agent_task_twin_use_snapshot", "410_agent_task_twin_use_snapshot"},
	"437_twin_deposition_replacement":               {"443_twin_deposition_replacement", "411_twin_deposition_replacement"},
	"438_drop_twin_proposal_identity_index":         {"444_drop_twin_proposal_identity_index", "412_drop_twin_proposal_identity_index"},
	"439_twin_proposal_identity_partial_index":      {"445_twin_proposal_identity_partial_index", "413_twin_proposal_identity_partial_index"},
	"440_twin_deposition_request_index":             {"446_twin_deposition_request_index", "414_twin_deposition_request_index"},
	"441_twin_deposition_edit_digest":               {"447_twin_deposition_edit_digest", "415_twin_deposition_edit_digest"},
	"442_twin_proposal_correction":                  {"448_twin_proposal_correction", "416_twin_proposal_correction"},
	"443_twin_proposal_replacement_index":           {"449_twin_proposal_replacement_index", "417_twin_proposal_replacement_index"},
	"444_room_turn_identity_index_drop":             {"450_room_turn_identity_index_drop", "418_room_turn_identity_index_drop"},
	"445_room_turn_kind_attempt_index":              {"451_room_turn_kind_attempt_index", "419_room_turn_kind_attempt_index"},
	"446_room_lifecycle_idempotency":                {"452_room_lifecycle_idempotency", "420_room_lifecycle_idempotency"},
	"447_room_synthesis_retry_key_index":            {"453_room_synthesis_retry_key_index", "421_room_synthesis_retry_key_index"},
	"448_room_capability_rollout":                   {"454_room_capability_rollout", "422_room_capability_rollout"},
	"449_room_artifact_recommendation_key":          {"455_room_artifact_recommendation_key", "423_room_artifact_recommendation_key"},
	"450_room_memory_review_key_index":              {"456_room_memory_review_key_index", "424_room_memory_review_key_index"},
	"451_room_memory_revision_creator":              {"457_room_memory_revision_creator", "425_room_memory_revision_creator"},
	"452_room_cycle_cost_limit":                     {"458_room_cycle_cost_limit", "426_room_cycle_cost_limit"},
	"453_room_spend_limit_refusal":                  {"459_room_spend_limit_refusal", "427_room_spend_limit_refusal"},
	"454_wiki_knowledge_primary_keys":               {"460_wiki_knowledge_primary_keys", "428_wiki_knowledge_primary_keys"},
}

func hooksForDirection(direction string) map[string]preMigrationHook {
	switch direction {
	case "up":
		return preMigrationHooks
	case "down":
		return preRollbackHooks
	default:
		return nil
	}
}

func conditionsForDirection(direction string) map[string]migrationCondition {
	if direction == "up" {
		return upMigrationConditions
	}
	// Rollbacks intentionally ignore environment gates: they restore the
	// portable pre-migration schema regardless of which up SQL actually ran.
	return nil
}

func whenIndexUsable(requirement usableIndexRequirement) migrationCondition {
	return func(ctx context.Context, conn *pgxpool.Conn) (bool, string, error) {
		usable, err := indexIsUsable(ctx, conn, requirement)
		if err != nil {
			return false, "", err
		}
		if !usable {
			return false, fmt.Sprintf("preferred index %s is unavailable or unusable", requirement.IndexRegclass), nil
		}
		return true, "", nil
	}
}

func whenIndexNotUsable(requirement usableIndexRequirement) migrationCondition {
	return func(ctx context.Context, conn *pgxpool.Conn) (bool, string, error) {
		usable, err := indexIsUsable(ctx, conn, requirement)
		if err != nil {
			return false, "", err
		}
		if usable {
			return false, fmt.Sprintf("preferred index %s is ready", requirement.IndexRegclass), nil
		}
		return true, "", nil
	}
}

// indexIsUsable fails closed: every property needed by the search query must
// match before a fallback index may be skipped or removed. In particular, the
// preferred relation must be a live, ready, valid, non-partial GIN expression
// index owned by the expected extension. A same-named stale or drifted index is
// treated as unusable, preserving the fallback.
func indexIsUsable(ctx context.Context, conn *pgxpool.Conn, requirement usableIndexRequirement) (bool, error) {
	var usable bool
	err := conn.QueryRow(ctx, `
		SELECT COALESCE(
			idx.relkind = 'i'
			AND i.indisvalid
			AND i.indisready
			AND i.indislive
			AND NOT i.indisunique
			AND i.indpred IS NULL
			AND i.indexprs IS NOT NULL
			AND i.indrelid = to_regclass($2)
			AND i.indnkeyatts = 1
			AND i.indnatts = 1
			AND am.amname = $3
			AND opc.opcname = $4
			AND pg_get_indexdef(i.indexrelid, 1, FALSE) = $5
			AND EXISTS (
				SELECT 1
				FROM pg_depend dep
				JOIN pg_extension ext ON ext.oid = dep.refobjid
				WHERE dep.classid = 'pg_opclass'::regclass
				  AND dep.objid = opc.oid
				  AND dep.refclassid = 'pg_extension'::regclass
				  AND dep.deptype = 'e'
				  AND ext.extname = $6
			),
			FALSE
		)
		FROM pg_class idx
		LEFT JOIN pg_index i ON i.indexrelid = idx.oid
		LEFT JOIN pg_am am ON am.oid = idx.relam
		-- pg_index.indclass is an int2vector, whose subscripts start at zero.
		LEFT JOIN pg_opclass opc ON opc.oid = i.indclass[0]
		WHERE idx.oid = to_regclass($1)
	`,
		requirement.IndexRegclass,
		requirement.TableRegclass,
		requirement.AccessMethod,
		requirement.OperatorClass,
		requirement.Expression,
		requirement.Extension,
	).Scan(&usable)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect preferred index %q: %w", requirement.IndexRegclass, err)
	}
	return usable, nil
}

// cleanupInvalidConcurrentIndexHook removes an INVALID index left by an
// interrupted or failed CREATE INDEX CONCURRENTLY before the migration retries.
// Without this guard, CREATE INDEX ... IF NOT EXISTS would treat the leftover
// relation as success and allow a later migration to drop the still-valid old
// index. Non-index relations fail closed instead of being dropped implicitly.
func cleanupInvalidConcurrentIndexHook(indexRegclass string) preMigrationHook {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		var schemaName, relationName string
		var isIndex, isValid bool
		err := pool.QueryRow(ctx, `
			SELECT n.nspname, c.relname, c.relkind = 'i', COALESCE(i.indisvalid, FALSE)
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			LEFT JOIN pg_index i ON i.indexrelid = c.oid
			WHERE c.oid = to_regclass($1)
		`, indexRegclass).Scan(&schemaName, &relationName, &isIndex, &isValid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect concurrent index %q: %w", indexRegclass, err)
		}
		if !isIndex {
			return fmt.Errorf("relation %q exists but is not an index", indexRegclass)
		}
		if isValid {
			return nil
		}

		qualifiedName := pgx.Identifier{schemaName, relationName}.Sanitize()
		if _, err := pool.Exec(ctx, "DROP INDEX CONCURRENTLY IF EXISTS "+qualifiedName); err != nil {
			return fmt.Errorf("drop invalid concurrent index %s: %w", qualifiedName, err)
		}
		slog.Warn("removed invalid index before migration retry", "index", qualifiedName)
		return nil
	}
}

func runTaskUsageHourlyHook(ctx context.Context, pool *pgxpool.Pool) error {
	res, err := taskusagebackfill.Hook(ctx, pool, taskusagebackfill.HookOptions{})
	if err != nil {
		return fmt.Errorf("task_usage_hourly pre-103 hook: %w", err)
	}
	if res.Skipped != "" {
		slog.Info("task_usage hourly rollup hook: skipped",
			"reason", res.Skipped,
			"watermark_stamped", res.WatermarkStamped)
		return nil
	}
	slog.Info("task_usage hourly rollup hook: backfill complete",
		"slices", res.SlicesProcessed,
		"rows_touched", res.RowsTouched,
		"from", res.From.Format("2006-01-02T15:04:05Z07:00"),
		"to", res.To.Format("2006-01-02T15:04:05Z07:00"))
	return nil
}

// runAttributionStrictHook backfills accountable_user_id from
// originator_user_id before migration 198 validates the strict attribution
// constraint, so self-hosted upgrades that never ran the out-of-band
// backfill recover automatically (GH #5544 / MUL-4897).
func runAttributionStrictHook(ctx context.Context, pool *pgxpool.Pool) error {
	res, err := attributionbackfill.Hook(ctx, pool, attributionbackfill.HookOptions{})
	if err != nil {
		return fmt.Errorf("attribution strict-constraint pre-198 hook: %w", err)
	}
	slog.Info("attribution backfill hook: complete",
		"rows_backfilled", res.RowsBackfilled,
		"batches", res.Batches,
		"mismatch_normalized", res.MismatchNormalized)
	return nil
}

// migrationAdvisoryLockKey is the int64 identifier used with Postgres
// pg_advisory_lock to serialize the migration loop across concurrent
// runners (multi-replica backend Deployment, scale-up, or a manual
// `migrate up` overlapping with pod startup). The exact value is
// arbitrary — it just needs to be stable across every process that runs
// migrations against the same database. See GitHub multica-ai/multica#3647.
const migrationAdvisoryLockKey int64 = 7244554146635925501

// defaultSchemaMigrationsTable is the unqualified name of the bookkeeping
// table that tracks which migrations have been applied. Tests override
// this so a concurrent-race harness can run against the same shared
// Postgres without colliding with the production table.
const defaultSchemaMigrationsTable = "schema_migrations"

// runOptions carries everything runMigrations needs that is not the
// pool itself. Tests use it to inject a hermetic migrations directory,
// a unique per-test bookkeeping table, and a unique advisory-lock key
// that doesn't collide with any other migration runner sharing the same
// Postgres instance.
type runOptions struct {
	// Direction is "up" or "down".
	Direction string
	// Files is the ordered list of .sql files to apply. Production callers
	// pass migrations.Files(direction); tests pass a curated set written
	// to a t.TempDir().
	Files []string
	// SchemaMigrationsTable is the bookkeeping table to read/write.
	// May be schema-qualified (e.g. "migrate_test_xyz.schema_migrations").
	// Empty means defaultSchemaMigrationsTable.
	SchemaMigrationsTable string
	// AdvisoryLockKey is the int64 used with pg_advisory_lock. Zero means
	// migrationAdvisoryLockKey. Tests pass a unique key per run so
	// concurrent test workers do not block on the production migration
	// runner if it happens to share the database.
	AdvisoryLockKey int64
	// Hooks maps migration version → pre-migration hook. The hook
	// receives the pool (not the loop's pinned conn) so it can take
	// its own session-level locks. nil or missing entries mean "no
	// hook" and the migration runs straight through. Production main()
	// passes the direction-specific hook map; tests leave this nil unless they
	// exercise a hook.
	Hooks map[string]preMigrationHook
	// Conditions maps migration version → a read-only gate that decides
	// whether the SQL file should execute. A false result still advances the
	// migration ledger, allowing later migrations to run in environments where
	// this version's DDL is intentionally unnecessary.
	Conditions map[string]migrationCondition
	// VersionAliases maps the current migration identity to older published
	// identities for the same SQL. Production passes the frozen downstream map;
	// tests may inject a small fixture map. Never use aliases for different DDL.
	VersionAliases map[string][]string
}

func main() {
	logger.Init()

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/migrate <up|down>")
		os.Exit(1)
	}

	direction := os.Args[1]
	if direction != "up" && direction != "down" {
		fmt.Println("Usage: go run ./cmd/migrate <up|down>")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
	}

	startupSettings := dbstartup.SettingsFromEnv()
	pool, err := dbstartup.NewPool(context.Background(), dbURL, startupSettings.ConnectTimeout)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	files, err := migrations.Files(direction)
	if err != nil {
		slog.Error("failed to find migration files", "error", err)
		os.Exit(1)
	}

	options := runOptions{
		Direction:      direction,
		Files:          files,
		Hooks:          hooksForDirection(direction),
		Conditions:     conditionsForDirection(direction),
		VersionAliases: legacyMigrationVersionAliases,
	}
	startupCtx, stopStartup := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopStartup()
	retryOptions := startupSettings.RetryOptions()
	retryOptions.ShouldRetry = dbstartup.IsTransientDatabaseError
	retryOptions.OnRetry = func(event dbstartup.RetryEvent) {
		slog.Warn("database unavailable before migrations; retrying",
			"attempt", event.Attempt,
			"retry_in", event.Delay,
			"error", event.Err,
		)
	}
	if err := dbstartup.Retry(startupCtx, retryOptions, pool.Ping); err != nil {
		slog.Error("unable to ping database", "error", err)
		os.Exit(1)
	}

	migrationErr := runMigrations(startupCtx, pool, options)
	if migrationErr != nil && startupSettings.StartupTimeout > 0 && dbstartup.IsTransientDatabaseError(migrationErr) {
		slog.Warn("migration interrupted by database unavailability; retrying", "error", migrationErr)
		migrationRetryOptions := startupSettings.RetryOptions()
		migrationRetryOptions.ShouldRetry = dbstartup.IsTransientDatabaseError
		migrationRetryOptions.AllowOperationPastTimeout = true
		migrationRetryOptions.OnRetry = func(event dbstartup.RetryEvent) {
			slog.Warn("database unavailable during migration retry",
				"attempt", event.Attempt,
				"retry_in", event.Delay,
				"error", event.Err,
			)
		}
		migrationErr = dbstartup.Retry(startupCtx, migrationRetryOptions, func(attemptCtx context.Context) error {
			if err := pool.Ping(attemptCtx); err != nil {
				return fmt.Errorf("ping database: %w", err)
			}
			return runMigrations(attemptCtx, pool, options)
		})
	}
	if migrationErr != nil {
		slog.Error("migration run failed", "error", migrationErr)
		os.Exit(1)
	}

	fmt.Println("Done.")
}

// runMigrations applies (direction="up") or rolls back (direction="down")
// the given file list against the supplied pool, serialized through a
// Postgres session-level advisory lock so multiple concurrent runners
// (multi-replica startup, scale-up, manual migrate overlap) take turns
// instead of racing each other.
//
// It is safe to invoke concurrently from multiple goroutines or
// processes against the same database with the same options: every
// caller blocks on pg_advisory_lock, and once it is their turn the
// already-applied EXISTS check turns each finished migration into a
// no-op skip. See GitHub multica-ai/multica#3647 / MUL-2923.
func runMigrations(ctx context.Context, pool *pgxpool.Pool, opts runOptions) error {
	switch opts.Direction {
	case "up", "down":
		// ok
	default:
		return fmt.Errorf("invalid direction %q (want \"up\" or \"down\")", opts.Direction)
	}

	table := opts.SchemaMigrationsTable
	if table == "" {
		table = defaultSchemaMigrationsTable
	}
	tableIdent, err := quoteQualifiedIdentifier(table)
	if err != nil {
		return fmt.Errorf("invalid schema migrations table %q: %w", table, err)
	}
	lockKey := opts.AdvisoryLockKey
	if lockKey == 0 {
		lockKey = migrationAdvisoryLockKey
	}

	// pg_advisory_lock is scoped to a single session, so we must pin one
	// *pgxpool.Conn for the whole run — calling pool.Exec would attach the
	// lock to a random connection that pgxpool could hand back out before
	// the loop finishes, making the lock effectively a no-op. We use the
	// blocking pg_advisory_lock (not pg_try_*) so a late-arriving runner
	// queues behind the current one instead of crash-looping; once it
	// acquires the lock the EXISTS checks below turn finished migrations
	// into no-op skips.
	//
	// We deliberately do NOT wrap the loop in a single transaction: the
	// repo already ships migrations using CREATE INDEX CONCURRENTLY,
	// which Postgres rejects inside a transaction block.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	// Best-effort explicit unlock on the success path. On error returns
	// the defer still runs; on os.Exit error paths in main() it does not,
	// but session-level advisory locks are released automatically when
	// the connection closes at process exit, so the next runner is never
	// permanently blocked.
	defer func() {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey); err != nil {
			slog.Warn("failed to release migration advisory lock", "error", err)
		}
	}()

	// Create migrations tracking table.
	if _, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, tableIdent)); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	existsSQL := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE version = $1)", tableIdent)
	insertSQL := fmt.Sprintf("INSERT INTO %s (version) VALUES ($1)", tableIdent)
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE version = $1", tableIdent)

	for _, file := range opts.Files {
		version := migrations.ExtractVersion(file)
		aliases := opts.VersionAliases[version]

		var exists bool
		if err := conn.QueryRow(ctx, existsSQL, version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %q: %w", version, err)
		}
		legacyVersion := ""
		if !exists {
			for _, alias := range aliases {
				var aliasExists bool
				if err := conn.QueryRow(ctx, existsSQL, alias).Scan(&aliasExists); err != nil {
					return fmt.Errorf("check legacy migration %q for %q: %w", alias, version, err)
				}
				if aliasExists {
					legacyVersion = alias
					break
				}
			}
		}

		if opts.Direction == "up" {
			if exists {
				fmt.Printf("  skip  %s (already applied)\n", version)
				continue
			}
			if legacyVersion != "" {
				if _, err := conn.Exec(ctx, insertSQL, version); err != nil {
					return fmt.Errorf("record migration alias %q for %q: %w", legacyVersion, version, err)
				}
				fmt.Printf("  skip  %s (already applied as %s; recorded current identity)\n", version, legacyVersion)
				continue
			}
		} else {
			if !exists && legacyVersion == "" {
				fmt.Printf("  skip  %s (not applied)\n", version)
				continue
			}
		}

		sql, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", file, err)
		}

		// Run any pre-migration hook before the SQL file. Hooks
		// receive the *pgxpool.Pool (not the loop's pinned conn), so
		// they can acquire other session-level locks without
		// colliding with migrationAdvisoryLockKey. Hook failures
		// abort the run before schema_migrations is updated, so the
		// same version retries cleanly on the next invocation.
		if hook, ok := opts.Hooks[version]; ok && hook != nil {
			slog.Info("running pre-migration hook", "version", version, "direction", opts.Direction)
			if err := hook(ctx, pool); err != nil {
				return fmt.Errorf("pre-migration hook for %q (%s): %w", version, opts.Direction, err)
			}
		}

		applySQL := true
		conditionReason := ""
		if condition, ok := opts.Conditions[version]; ok && condition != nil {
			applySQL, conditionReason, err = condition(ctx, conn)
			if err != nil {
				return fmt.Errorf("evaluate migration condition for %q (%s): %w", version, opts.Direction, err)
			}
		}

		if applySQL {
			if _, err := conn.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("apply migration %q: %w", file, err)
			}
		}

		if opts.Direction == "up" {
			_, err = conn.Exec(ctx, insertSQL, version)
		} else {
			for _, recordedVersion := range append([]string{version}, aliases...) {
				if _, err = conn.Exec(ctx, deleteSQL, recordedVersion); err != nil {
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("record migration %q: %w", version, err)
		}

		if applySQL {
			fmt.Printf("  %s  %s\n", opts.Direction, version)
		} else {
			fmt.Printf("  %s  %s (SQL skipped: %s)\n", opts.Direction, version, conditionReason)
		}
	}

	return nil
}

// quoteQualifiedIdentifier safely quotes either an unqualified table
// name ("foo") or a schema-qualified name ("schema.foo") for embedding
// into a SQL statement. Postgres does not let parametrized queries
// supply identifiers, so we have to interpolate, but pgx.Identifier
// does the right escaping (double-quotes, embedded-quote handling).
//
// The accepted shape is exactly one or two dot-separated components.
// Names containing more than one dot are rejected outright rather than
// silently sanitized into a "schema"."b.c" reference, which is valid
// SQL but almost certainly not what the caller meant.
func quoteQualifiedIdentifier(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("identifier %q has more than one dot; only schema.table is supported", name)
	}
	for _, p := range parts {
		if p == "" {
			return "", fmt.Errorf("empty component in %q", name)
		}
	}
	return pgx.Identifier(parts).Sanitize(), nil
}
