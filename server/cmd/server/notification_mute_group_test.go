package main

import "testing"

// TestIsNotifMutedGroupSplit pins the contract the `mentions` group exists for:
// muting comment VOLUME must never silence someone asking for you BY NAME.
// `new_comment` and `mentioned` shared the `comments` group until #6468, so a
// user who muted comment churn also lost every mention — and a muted item is
// never created, so it was unrecoverable rather than merely delayed.
func TestIsNotifMutedGroupSplit(t *testing.T) {
	cases := []struct {
		name      string
		prefs     map[string]string
		notifType string
		want      bool
	}{
		// The regression this split fixes.
		{"muted comments still delivers mentions", map[string]string{"comments": "muted"}, "mentioned", false},
		{"muted comments drops new_comment", map[string]string{"comments": "muted"}, "new_comment", true},
		{"muted mentions drops mentioned", map[string]string{"mentions": "muted"}, "mentioned", true},
		{"muted mentions still delivers new_comment", map[string]string{"mentions": "muted"}, "new_comment", false},
		{"both muted drops both", map[string]string{"comments": "muted", "mentions": "muted"}, "mentioned", true},

		// Absent key means the default "all" — preferences are stored sparse,
		// which is why existing `{"comments":"muted"}` rows need no migration.
		{"empty prefs delivers mentions", map[string]string{}, "mentioned", false},
		{"empty prefs delivers comments", map[string]string{}, "new_comment", false},
		{"explicit all delivers", map[string]string{"comments": "all"}, "new_comment", false},

		// Other groups are unaffected by the split.
		{"assignments group intact", map[string]string{"assignments": "muted"}, "issue_assigned", true},
		{"status group intact", map[string]string{"status_changes": "muted"}, "status_changed", true},
		{"muting comments leaves assignments alone", map[string]string{"comments": "muted"}, "issue_assigned", false},
		{"rooms group mutes outcome review delivery", map[string]string{"rooms": "muted"}, "room_outcome_review_required", true},
		{"rooms group leaves comments alone", map[string]string{"rooms": "muted"}, "new_comment", false},

		// Types outside the map are always delivered.
		{"unconfigurable type always delivered", map[string]string{"comments": "muted"}, "reaction_added", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotifMuted(tc.prefs, tc.notifType); got != tc.want {
				t.Fatalf("isNotifMuted(%v, %q) = %v, want %v",
					tc.prefs, tc.notifType, got, tc.want)
			}
		})
	}
}

func TestNotifTypeToGroupRoomsLifecycleTypes(t *testing.T) {
	for _, itemType := range []string{
		"room_outcome_review_required",
		"room_recommendation_review_required",
		"room_cycle_failed",
		"room_cycle_blocked",
	} {
		if got := notifTypeToGroup[itemType]; got != "rooms" {
			t.Fatalf("notifTypeToGroup[%s] = %q, want rooms", itemType, got)
		}
	}
}

// TestNotifTypeToGroupMentionsIsOwnGroup guards the mapping itself: a future
// edit that folds `mentioned` back under `comments` should fail here rather
// than silently reintroduce the coupling.
func TestNotifTypeToGroupMentionsIsOwnGroup(t *testing.T) {
	if got := notifTypeToGroup["mentioned"]; got != "mentions" {
		t.Fatalf("notifTypeToGroup[mentioned] = %q, want %q", got, "mentions")
	}
	if got := notifTypeToGroup["new_comment"]; got != "comments" {
		t.Fatalf("notifTypeToGroup[new_comment] = %q, want %q", got, "comments")
	}
}
