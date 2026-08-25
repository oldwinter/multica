package migrations

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const maxLegacyMigrationPrefix = 148

// legacyDuplicateMigrationStems lists prefixes that were already duplicated
// before this lint existed. It is a frozen historical record, not an escape
// hatch: a new collision must be renumbered instead of added here. Prefix 362
// was briefly listed and is deliberately absent again — the later of the two
// migrations was renumbered to 376, which its idempotent DDL made safe.
//
// Prefixes 251–309 and 403 record merges of already-published upstream and
// downstream histories. Renaming either side changes its schema_migrations
// identity and can re-run DDL on existing installations, so their exact stems
// stay frozen. The migration runner carries explicit aliases for downstream
// migrations that had already been renumbered before this rule was enforced.
var legacyDuplicateMigrationStems = map[string][]string{
	"020": {"020_issue_number", "020_task_session"},
	"026": {"026_comment_reactions", "026_task_messages"},
	"029": {"029_attachment", "029_daemon_token", "029_drop_daemon_pairing"},
	"032": {"032_drop_agent_triggers", "032_issue_search_index", "032_runtime_owner", "032_task_usage"},
	"033": {"033_chat", "033_comment_search_index"},
	"035": {"035_project_priority", "035_task_queue_issue_id_index"},
	"040": {"040_agent_custom_env", "040_chat_unread_since"},
	"041": {"041_agent_custom_args", "041_workspace_invitation"},
	"043": {"043_audit_reserved_slugs", "043_fix_orphaned_autopilot_runs"},
	"046": {"046_agent_mcp_config", "046_agent_unique_name", "046_drop_runtime_usage"},
	"050": {"050_add_onboarded_at_to_users", "050_agent_model", "050_issue_first_executed_at"},
	"060": {"060_add_user_language", "060_agent_description_length", "060_chat_session_runtime_id", "060_issue_origin_quick_create"},
	"065": {"065_backfill_onboarded_at", "065_project_resources"},
	"069": {"069_comment_resolved_at", "069_drop_task_last_heartbeat"},
	"079": {"079_autopilot_run_skipped_status", "079_backfill_api_invalid_request", "079_github_integration"},
	"083": {"083_attachment_chat_columns", "083_runtime_visibility"},
	"084": {"084_squad", "084_task_usage_dashboard_rollup"},
	"091": {"091_autopilot_webhook_triggers", "091_issue_start_date", "091_pr_ci_conflict"},
	"095": {"095_agent_thinking_level", "095_backfill_starter_content_state"},
	"096": {"096_autopilot_squad_assignee", "096_pending_check_suite", "096_user_profile_description"},
	"098": {"098_contact_sales_inquiries", "098_user_onboarding_runtime_choice"},
	"109": {"109_agent_task_waiting_local_directory", "109_drop_agent_skills_local", "109_issue_pull_request_close_intent", "109_lark_integration"},
	"111": {"111_issue_origin_lark_chat", "111_workspace_avatar"},
	"112": {"112_issue_dates_to_date", "112_lark_installation_bot_union_id"},
	"113": {"113_lark_inbound_dedup_per_installation", "113_sys_cron_executions"},
	"120": {"120_autopilot_subscriber", "120_comment_source_task_id", "120_github_pending_installation", "120_runtime_profile"},
	"122": {"122_lark_chat_session_binding_thread_reply", "122_task_handoff_note"},
	"124": {"124_autopilot_run_planned_at", "124_channel_generalization", "124_task_prepare_lease"},
	"127": {"127_issue_pull_request_reference_only", "127_task_squad_id", "127_user_composio_connection"},
	"128": {"128_agent_task_queue_runtime_mcp_overlay", "128_autopilot_collaborator", "128_comment_routing_escalation"},
	"251": {"251_agent_runtime_unbind", "251_twin_profile"},
	"252": {"252_agent_builder_draft", "252_twin_profile_workspace_index"},
	"253": {"253_runtime_profile_add_qwenpaw", "253_twin_profile_id_index"},
	"254": {"254_runtime_profile_add_reasonix", "254_twin_profile_primary_key"},
	"255": {"255_agent_task_queue_chat_pending_deferred_v3", "255_wiki_page"},
	"256": {"256_drop_agent_task_queue_chat_pending_v2", "256_wiki_page_id_index"},
	"257": {"257_agent_task_queue_channel_media_pending_unique_v2", "257_wiki_page_workspace_list_index"},
	"258": {"258_drop_pending_issue_agent_v1", "258_wiki_page_workspace_path_unique"},
	"259": {"259_issue_origin_dingtalk_chat", "259_wiki_page_project_path_unique"},
	"260": {"260_issue_origin_dingtalk_chat_validate", "260_wiki_page_user_path_unique"},
	"261": {"261_agent_task_queue_terminal_completed_at_v2", "261_wiki_page_primary_key"},
	"262": {"262_drop_agent_task_queue_terminal_completed_at_v1", "262_wiki_page_personal_global"},
	"263": {"263_issue_origin_wecom_chat", "263_wiki_page_user_path_unique_drop"},
	"264": {"264_issue_origin_wecom_chat_validate", "264_wiki_page_user_path_global_unique"},
	"265": {"265_issue_view", "265_lm_wiki_tables"},
	"266": {"266_issue_view_owner_index", "266_lm_wiki_revision_id_index"},
	"267": {"267_issue_view_shared_index", "267_lm_wiki_citation_id_index"},
	"268": {"268_issue_view_preference", "268_lm_wiki_review_id_index"},
	"269": {"269_issue_view_workspace_variant", "269_lm_wiki_primary_keys"},
	"270": {"270_lm_wiki_revision_number_index", "270_pinned_item_view"},
	"271": {"271_channel_chat_pending_fresh", "271_lm_wiki_revision_created_index"},
	"272": {"272_lm_wiki_citation_source_index", "272_rollup_task_usage_hourly_xact_lock"},
	"273": {"273_agent_task_queue_runtime_id_index", "273_lm_wiki_citation_ordinal_index"},
	"274": {"274_lm_wiki_review_revision_index", "274_task_token_workspace_id_index"},
	"275": {"275_task_token_agent_id_index", "275_twin_tables"},
	"276": {"276_chat_draft_restore_task_id_index", "276_twin_proposal_id_index"},
	"277": {"277_autopilot_run_task_id_index", "277_twin_proposal_review_id_index"},
	"278": {"278_agent_task_queue_agent_id_keyset_index", "278_twin_version_id_index"},
	"279": {"279_agent_task_queue_issue_id_keyset_index", "279_twin_primary_keys"},
	"281": {"281_agent_workspace_id_keyset_index", "281_twin_proposal_created_index"},
	"282": {"282_issue_workspace_id_keyset_index", "282_twin_proposal_review_index"},
	"283": {"283_agent_runtime_workspace_id_keyset_index", "283_twin_version_number_index"},
	"284": {"284_task_owner_row_fence", "284_twin_version_proposal_index"},
	"285": {"285_plugin_lifecycle_v1", "285_room_tables"},
	"286": {"286_plugin_identity_key_index", "286_room_id_index"},
	"287": {"287_plugin_release_version_index", "287_room_participant_id_index"},
	"288": {"288_plugin_installation_workspace_plugin_index", "288_room_entry_id_index"},
	"289": {"289_plugin_contribution_key_index", "289_room_cycle_id_index"},
	"290": {"290_plugin_contribution_ordinal_index", "290_room_turn_id_index"},
	"291": {"291_plugin_grant_revision_index", "291_room_artifact_id_index"},
	"292": {"292_plugin_binding_revision_index", "292_room_primary_keys"},
	"293": {"293_plugin_installation_workspace_index", "293_room_workspace_index"},
	"294": {"294_plugin_snapshot_execution_v1", "294_room_participant_identity_index"},
	"295": {"295_plugin_artifact_file_index", "295_room_entry_ordinal_index"},
	"296": {"296_plugin_snapshot_revision_index", "296_room_cycle_sequence_index"},
	"297": {"297_plugin_execution_task_index", "297_room_cycle_wake_index"},
	"298": {"298_plugin_health_index", "298_room_active_cycle_index"},
	"299": {"299_agent_task_plugin_manifest_index", "299_room_turn_participant_index"},
	"300": {"300_drop_redundant_issue_workspace_number_index", "300_room_artifact_kind_index"},
	"301": {"301_drop_redundant_sys_cron_job_plan_index", "301_room_due_index"},
	"302": {"302_agent_task_room_turn_index", "302_drop_redundant_channel_chat_session_binding_index"},
	"303": {"303_drop_redundant_lark_chat_session_binding_index", "303_room_artifact_room_index"},
	"304": {"304_dingtalk_group_route", "304_room_turn_room_index"},
	"305": {"305_dingtalk_group_route_installation_conversation_unique", "305_room_participant_room_index"},
	"307": {"307_agent_task_room_turn_lookup_index", "307_dingtalk_group_route_id_unique"},
	"308": {"308_agent_task_branch_name", "308_room_entry_turn_result_index"},
	"309": {"309_agent_runtime_id_index", "309_room_invocation_refusal_reason"},
	"403": {"403_room_outcome_lifecycle", "403_runtime_profile_add_zeroclaw"},
}

var migrationPrefixPattern = regexp.MustCompile(`^(\d+)_`)

func TestMigrationFilesHaveMatchingDirections(t *testing.T) {
	files := migrationFilesForLint(t, "*.sql")

	directionsByStem := make(map[string]map[string]bool)
	for _, file := range files {
		stem, direction, ok := splitMigrationFilename(filepath.Base(file))
		if !ok {
			continue
		}
		if directionsByStem[stem] == nil {
			directionsByStem[stem] = make(map[string]bool)
		}
		directionsByStem[stem][direction] = true
	}

	for stem, directions := range directionsByStem {
		if !directions["up"] || !directions["down"] {
			t.Errorf("migration %s must have both .up.sql and .down.sql files", stem)
		}
	}
}

func TestMigrationNumericPrefixesStayUniqueAfterLegacySet(t *testing.T) {
	stemsByPrefix := migrationStemsByPrefix(t)

	for prefix, stems := range stemsByPrefix {
		sort.Strings(stems)

		legacyStems, isLegacyDuplicate := legacyDuplicateMigrationStems[prefix]
		if isLegacyDuplicate {
			expected := append([]string(nil), legacyStems...)
			sort.Strings(expected)
			if !reflect.DeepEqual(stems, expected) {
				t.Errorf("legacy duplicate migration prefix %s changed: got %v, want %v; do not add to or rename historical duplicate-prefix migrations", prefix, stems, expected)
			}
			continue
		}

		if len(stems) > 1 {
			t.Errorf("migration prefix %s is reused by %v; use the next unique prefix instead", prefix, stems)
		}
	}
}

func TestNewMigrationPrefixesStartAfterLegacyRange(t *testing.T) {
	stemsByPrefix := migrationStemsByPrefix(t)

	for prefix, stems := range stemsByPrefix {
		n, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatalf("parse migration prefix %q: %v", prefix, err)
		}
		if n <= maxLegacyMigrationPrefix && !isKnownLegacyPrefix(prefix) {
			t.Errorf("migration prefix %s is in the frozen legacy range 001-%03d: %v; new migrations must start at %03d", prefix, maxLegacyMigrationPrefix, stems, maxLegacyMigrationPrefix+1)
		}
	}
}

func migrationStemsByPrefix(t *testing.T) map[string][]string {
	t.Helper()

	files := migrationFilesForLint(t, "*.up.sql")
	stemsByPrefix := make(map[string][]string)
	for _, file := range files {
		stem := strings.TrimSuffix(filepath.Base(file), ".up.sql")
		match := migrationPrefixPattern.FindStringSubmatch(stem)
		if match == nil {
			t.Fatalf("migration %s does not start with a numeric prefix followed by underscore", stem)
		}
		stemsByPrefix[match[1]] = append(stemsByPrefix[match[1]], stem)
	}
	return stemsByPrefix
}

func migrationFilesForLint(t *testing.T, pattern string) []string {
	t.Helper()

	dir := realMigrationsDir(t)
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no migration files matched %s in %s", pattern, dir)
	}
	sort.Strings(files)
	return files
}

func realMigrationsDir(t *testing.T) string {
	t.Helper()

	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration lint test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(self), "..", "..", "migrations"))
}

func splitMigrationFilename(name string) (stem, direction string, ok bool) {
	for _, candidateDirection := range []string{"up", "down"} {
		suffix := fmt.Sprintf(".%s.sql", candidateDirection)
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix), candidateDirection, true
		}
	}
	return "", "", false
}

func isKnownLegacyPrefix(prefix string) bool {
	if _, ok := legacyDuplicateMigrationStems[prefix]; ok {
		return true
	}

	switch prefix {
	case "001", "002", "003", "004", "005", "006", "007", "008", "009", "010",
		"011", "012", "013", "014", "015", "016", "017", "018", "019", "021",
		"022", "023", "024", "025", "027", "028", "030", "031", "034", "036",
		"037", "038", "039", "042", "044", "045", "047", "048", "049", "051",
		"052", "053", "054", "055", "056", "057", "058", "059", "061", "062",
		"063", "064", "066", "067", "068", "072", "073", "074", "075", "076",
		"077", "078", "080", "081", "082", "085", "086", "087", "088", "089",
		"090", "092", "093", "094", "097", "100", "101", "102", "103", "104",
		"105", "106", "107", "108", "110", "114", "115", "116", "117", "118",
		"119", "121", "123", "125", "126", "129", "130", "131", "132", "133",
		"134", "135", "136", "137", "138", "139", "140", "141", "142", "143",
		"144", "145", "146", "147", "148":
		return true
	default:
		return false
	}
}
