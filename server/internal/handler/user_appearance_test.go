package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func newAppearanceTestUser(t *testing.T, email string, over ...testutil.Cols) string {
	t.Helper()
	return dbfx.User(t, "Appearance Test", email, over...)
}

func appearanceFields(skin, appearance, updatedAt string, tokenVersion int) string {
	body, _ := json.Marshal(map[string]any{
		"skin":                     skin,
		"appearance":               appearance,
		"appearance_updated_at":    updatedAt,
		"appearance_token_version": tokenVersion,
	})
	return string(body)
}

func appearanceUndoFields(skin, appearance, updatedAt, expectedUpdatedAt string) string {
	body, _ := json.Marshal(map[string]any{
		"skin":                           skin,
		"appearance":                     appearance,
		"appearance_updated_at":          updatedAt,
		"appearance_token_version":       1,
		"appearance_expected_updated_at": expectedUpdatedAt,
	})
	return string(body)
}

func TestGetMeReturnsUnsetAppearancePreferences(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-get-unset@multica.ai")
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("X-User-ID", userID)

	var response map[string]any
	testutil.Call(t, testHandler.GetMe, req).Want(http.StatusOK).JSON(&response)

	for _, field := range []string{"skin", "appearance", "appearance_updated_at", "appearance_token_version"} {
		if response[field] != nil {
			t.Errorf("%s = %v, want null before first sync", field, response[field])
		}
	}
}

func TestUpdateMePersistsAppearancePreferenceTuple(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-set@multica.ai")
	const updatedAt = "2026-08-22T10:11:12.123456Z"

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID,
		appearanceFields("relay", "dark", updatedAt, 1))).Want(http.StatusOK).JSON(&response)

	if response["skin"] != "relay" || response["appearance"] != "dark" {
		t.Fatalf("appearance response = skin %v, appearance %v", response["skin"], response["appearance"])
	}
	responseUpdatedAt, err := time.Parse(time.RFC3339Nano, response["appearance_updated_at"].(string))
	if err != nil || !responseUpdatedAt.Equal(time.Date(2026, time.August, 22, 10, 11, 12, 123456000, time.UTC)) {
		t.Errorf("appearance_updated_at = %v, want instant %s", response["appearance_updated_at"], updatedAt)
	}
	if response["appearance_token_version"] != float64(1) {
		t.Errorf("appearance_token_version = %v, want 1", response["appearance_token_version"])
	}

	var skin, appearance string
	var storedUpdatedAt string
	var tokenVersion int
	dbfx.QueryRow(t, `
		SELECT skin, appearance, to_char(appearance_updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), appearance_token_version
		FROM "user" WHERE id = $1`, userID,
	).Scan(&skin, &appearance, &storedUpdatedAt, &tokenVersion)
	if skin != "relay" || appearance != "dark" || storedUpdatedAt != updatedAt || tokenVersion != 1 {
		t.Fatalf("stored tuple = (%q, %q, %q, %d)", skin, appearance, storedUpdatedAt, tokenVersion)
	}
}

func TestUpdateMeKeepsNewerAppearancePreference(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-stale@multica.ai", testutil.Cols{
		"skin":                     "field",
		"appearance":               "light",
		"appearance_updated_at":    "2026-08-22T12:00:00Z",
		"appearance_token_version": 1,
	})

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID,
		appearanceFields("relay", "dark", "2026-08-22T11:59:59Z", 1))).Want(http.StatusOK).JSON(&response)

	responseUpdatedAt, err := time.Parse(time.RFC3339Nano, response["appearance_updated_at"].(string))
	if response["skin"] != "field" || response["appearance"] != "light" || err != nil || !responseUpdatedAt.Equal(time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("stale write replaced current preference: %#v", response)
	}
}

func TestUpdateMeAcceptsEqualTimestampLocalPreference(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-equal@multica.ai", testutil.Cols{
		"skin":                     "field",
		"appearance":               "light",
		"appearance_updated_at":    "2026-08-22T12:00:00Z",
		"appearance_token_version": 1,
	})

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID,
		appearanceFields("relay", "dark", "2026-08-22T12:00:00Z", 1))).Want(http.StatusOK).JSON(&response)

	if response["skin"] != "relay" || response["appearance"] != "dark" {
		t.Fatalf("equal-timestamp local preference was not accepted: %#v", response)
	}

	var skin, appearance string
	dbfx.QueryRow(t, `SELECT skin, appearance FROM "user" WHERE id = $1`, userID).Scan(&skin, &appearance)
	if skin != "relay" || appearance != "dark" {
		t.Fatalf("stored equal-timestamp tuple = (%q, %q), want (relay, dark)", skin, appearance)
	}
}

func TestUpdateMeAppliesAppearanceUndoWhenExpectedTimestampMatches(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-undo-match@multica.ai", testutil.Cols{
		"skin":                     "field",
		"appearance":               "dark",
		"appearance_updated_at":    "2026-08-22T12:00:00Z",
		"appearance_token_version": 1,
	})

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID,
		appearanceUndoFields("relay", "system", "2026-08-22T12:01:00Z", "2026-08-22T12:00:00Z"))).Want(http.StatusOK).JSON(&response)

	if response["skin"] != "relay" || response["appearance"] != "system" {
		t.Fatalf("matching Undo was not applied: %#v", response)
	}
}

func TestUpdateMeExpiresAppearanceUndoWhenExpectedTimestampIsStale(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-undo-stale@multica.ai", testutil.Cols{
		"skin":                     "field",
		"appearance":               "dark",
		"appearance_updated_at":    "2026-08-22T12:00:00Z",
		"appearance_token_version": 1,
	})

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID,
		appearanceUndoFields("relay", "system", "2026-08-22T12:01:00Z", "2026-08-22T11:59:00Z"))).Want(http.StatusOK).JSON(&response)

	if response["skin"] != "field" || response["appearance"] != "dark" {
		t.Fatalf("stale Undo replaced current preference: %#v", response)
	}
}

func TestUpdateMePreservesAppearanceWhenOmitted(t *testing.T) {
	userID := newAppearanceTestUser(t, "appearance-preserve@multica.ai", testutil.Cols{
		"skin":                     "relay",
		"appearance":               "system",
		"appearance_updated_at":    "2026-08-22T12:00:00Z",
		"appearance_token_version": 1,
	})

	var response map[string]any
	testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID, `{"name":"Renamed"}`)).Want(http.StatusOK).JSON(&response)
	if response["skin"] != "relay" || response["appearance"] != "system" {
		t.Fatalf("profile patch changed appearance preference: %#v", response)
	}
}

func TestUpdateMeRejectsInvalidAppearancePreferences(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"skin":`},
		{name: "partial tuple", body: `{"skin":"relay"}`},
		{name: "orphaned expected timestamp", body: `{"appearance_expected_updated_at":"2026-08-22T12:00:00Z"}`},
		{name: "unsupported skin", body: appearanceFields("midnight", "dark", "2026-08-22T12:00:00Z", 1)},
		{name: "unsupported appearance", body: appearanceFields("relay", "sepia", "2026-08-22T12:00:00Z", 1)},
		{name: "malformed timestamp", body: appearanceFields("relay", "dark", "yesterday", 1)},
		{name: "malformed expected timestamp", body: appearanceUndoFields("relay", "dark", "2026-08-22T12:00:00Z", "yesterday")},
		{name: "future timestamp", body: appearanceFields("relay", "dark", time.Now().Add(appearanceMaxClockSkew+time.Minute).UTC().Format(time.RFC3339Nano), 1)},
		{name: "unsupported token version", body: appearanceFields("relay", "dark", "2026-08-22T12:00:00Z", 2)},
		{name: "wrong field type", body: `{"skin":42,"appearance":"dark","appearance_updated_at":"2026-08-22T12:00:00Z","appearance_token_version":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := newAppearanceTestUser(t, "appearance-invalid-"+strings.ReplaceAll(tt.name, " ", "-")+"@multica.ai")
			testutil.Call(t, testHandler.UpdateMe, newPatchMeRequest(userID, tt.body)).Want(http.StatusBadRequest)

			var count int
			if err := testPool.QueryRow(context.Background(), `
				SELECT count(*) FROM "user"
				WHERE id = $1 AND skin IS NULL AND appearance IS NULL
					AND appearance_updated_at IS NULL AND appearance_token_version IS NULL`, userID,
			).Scan(&count); err != nil {
				t.Fatalf("lookup appearance preference: %v", err)
			}
			if count != 1 {
				t.Fatal("invalid request changed appearance preference")
			}
		})
	}
}
