package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDecodeTwinJSON(t *testing.T) {
	t.Parallel()

	t.Run("empty_raw_is_noop", func(t *testing.T) {
		var target []TwinAssertionResponse
		if err := decodeTwinJSON(nil, &target); err != nil {
			t.Fatalf("nil raw: %v", err)
		}
		if err := decodeTwinJSON([]byte{}, &target); err != nil {
			t.Fatalf("empty raw: %v", err)
		}
		if target != nil {
			t.Fatalf("expected target left untouched, got %+v", target)
		}
	})

	t.Run("whitespace_only_raw_is_invalid_json", func(t *testing.T) {
		var target []TwinAssertionResponse
		if err := decodeTwinJSON([]byte("   \n\t"), &target); err == nil {
			t.Fatal("expected whitespace-only raw to fail JSON unmarshal")
		}
	})

	t.Run("null_json_is_rejected", func(t *testing.T) {
		var target []TwinAssertionResponse
		err := decodeTwinJSON([]byte("null"), &target)
		if err == nil || !strings.Contains(err.Error(), "cannot be null") {
			t.Fatalf("expected null rejection, got %v", err)
		}
		err = decodeTwinJSON([]byte("  null  "), &target)
		if err == nil || !strings.Contains(err.Error(), "cannot be null") {
			t.Fatalf("expected whitespace-padded null rejection, got %v", err)
		}
	})

	t.Run("valid_array_unmarshals", func(t *testing.T) {
		var target []TwinAssertionResponse
		raw := []byte(`[{"id":"a1","text":"Prefer small changes","sourceCount":2,"sourceRefs":["n1","n2"],"reviewed":true}]`)
		if err := decodeTwinJSON(raw, &target); err != nil {
			t.Fatalf("valid JSON: %v", err)
		}
		if len(target) != 1 || target[0].ID != "a1" || target[0].SourceCount != 2 || !target[0].Reviewed {
			t.Fatalf("unexpected decode result: %+v", target)
		}
		if len(target[0].SourceRefs) != 2 || target[0].SourceRefs[0] != "n1" {
			t.Fatalf("unexpected source refs: %+v", target[0].SourceRefs)
		}
	})

	t.Run("invalid_json_surfaces_unmarshal_error", func(t *testing.T) {
		var target []TwinTopicResponse
		err := decodeTwinJSON([]byte(`{"not":"an-array"}`), &target)
		if err == nil {
			t.Fatal("expected unmarshal error for object into slice")
		}
		err = decodeTwinJSON([]byte(`{`), &target)
		if err == nil {
			t.Fatal("expected unmarshal error for truncated JSON")
		}
	})
}

func TestTwinProfileToResponse(t *testing.T) {
	t.Parallel()

	id := util.MustParseUUID("11111111-1111-1111-1111-111111111111")
	wsID := util.MustParseUUID("22222222-2222-2222-2222-222222222222")
	updated := pgtype.Timestamptz{Time: time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC), Valid: true}

	base := db.TwinProfile{
		ID:             id,
		WorkspaceID:    wsID,
		Name:           "Product partner",
		State:          "pending-signoff",
		ReviewDigest:   "sha256:review",
		SourceCount:    3,
		AssertionCount: 1,
		SkillCount:     2,
		RuleCount:      4,
		Assertions:     []byte(`[{"id":"a1","text":"Keep changes reviewable","sourceCount":1,"sourceRefs":["note"],"reviewed":false}]`),
		Topics:         []byte(`[{"id":"t1","issueIdentifier":"MUL-1","title":"Bound the work","state":"active","owner":"You","updatedAt":"Today"}]`),
		ReviewSteps:    []byte(`[{"id":"import","state":"complete"},{"id":"generate","state":"current"}]`),
		UpdatedAt:      updated,
	}

	t.Run("maps_scalars_and_metadata", func(t *testing.T) {
		resp, err := twinProfileToResponse(base)
		if err != nil {
			t.Fatalf("twinProfileToResponse: %v", err)
		}
		if resp.ID != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("id = %q", resp.ID)
		}
		if resp.Name != "Product partner" || resp.State != "pending-signoff" || resp.ReviewDigest != "sha256:review" {
			t.Fatalf("unexpected scalars: %+v", resp)
		}
		if resp.SourceCount != 3 || resp.AssertionCount != 1 || resp.SkillCount != 2 || resp.RuleCount != 4 {
			t.Fatalf("unexpected counts: %+v", resp)
		}
		if resp.UpdatedAt != "2026-08-03T12:00:00Z" {
			t.Fatalf("updatedAt = %q", resp.UpdatedAt)
		}
		if len(resp.Assertions) != 1 || resp.Assertions[0].Text != "Keep changes reviewable" {
			t.Fatalf("assertions: %+v", resp.Assertions)
		}
		if len(resp.Topics) != 1 || resp.Topics[0].IssueIdentifier != "MUL-1" {
			t.Fatalf("topics: %+v", resp.Topics)
		}
		if len(resp.ReviewSteps) != 2 || resp.ReviewSteps[1].State != "current" {
			t.Fatalf("reviewSteps: %+v", resp.ReviewSteps)
		}
	})

	t.Run("empty_metadata_defaults_to_empty_slices", func(t *testing.T) {
		profile := base
		profile.Assertions = nil
		profile.Topics = []byte{}
		profile.ReviewSteps = []byte("[]")
		resp, err := twinProfileToResponse(profile)
		if err != nil {
			t.Fatalf("empty metadata: %v", err)
		}
		if resp.Assertions == nil || len(resp.Assertions) != 0 {
			t.Fatalf("assertions default: %+v", resp.Assertions)
		}
		if resp.Topics == nil || len(resp.Topics) != 0 {
			t.Fatalf("topics default: %+v", resp.Topics)
		}
		if resp.ReviewSteps == nil || len(resp.ReviewSteps) != 0 {
			t.Fatalf("reviewSteps default: %+v", resp.ReviewSteps)
		}
	})

	t.Run("null_assertions_rejected", func(t *testing.T) {
		profile := base
		profile.Assertions = []byte("null")
		_, err := twinProfileToResponse(profile)
		if err == nil || err.Error() != "invalid Twin assertions metadata" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("invalid_topics_rejected", func(t *testing.T) {
		profile := base
		profile.Topics = []byte(`{"broken":true}`)
		_, err := twinProfileToResponse(profile)
		if err == nil || err.Error() != "invalid Twin topics metadata" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("null_review_steps_rejected", func(t *testing.T) {
		profile := base
		profile.ReviewSteps = []byte("null")
		_, err := twinProfileToResponse(profile)
		if err == nil || err.Error() != "invalid Twin review path metadata" {
			t.Fatalf("got %v", err)
		}
	})
}

// twinProfileMockDB drives GetTwinProfileByWorkspace through sqlc's QueryRow/Scan path.
type twinProfileMockDB struct {
	db.DBTX
	err     error
	profile db.TwinProfile
}

func (m *twinProfileMockDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return &twinProfileMockRow{err: m.err, profile: m.profile}
}

func (m *twinProfileMockDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

type twinProfileMockRow struct {
	err     error
	profile db.TwinProfile
}

func (m *twinProfileMockRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) != 14 {
		return errors.New("unexpected scan arity for TwinProfile")
	}
	*(dest[0].(*pgtype.UUID)) = m.profile.ID
	*(dest[1].(*pgtype.UUID)) = m.profile.WorkspaceID
	*(dest[2].(*string)) = m.profile.Name
	*(dest[3].(*string)) = m.profile.State
	*(dest[4].(*string)) = m.profile.ReviewDigest
	*(dest[5].(*int64)) = m.profile.SourceCount
	*(dest[6].(*int64)) = m.profile.AssertionCount
	*(dest[7].(*int64)) = m.profile.SkillCount
	*(dest[8].(*int64)) = m.profile.RuleCount
	*(dest[9].(*[]byte)) = m.profile.Assertions
	*(dest[10].(*[]byte)) = m.profile.Topics
	*(dest[11].(*[]byte)) = m.profile.ReviewSteps
	*(dest[12].(*pgtype.Timestamptz)) = m.profile.CreatedAt
	*(dest[13].(*pgtype.Timestamptz)) = m.profile.UpdatedAt
	return nil
}

func sampleTwinProfile() db.TwinProfile {
	return db.TwinProfile{
		ID:             util.MustParseUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		WorkspaceID:    util.MustParseUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Name:           "Unit Twin",
		State:          "signed-off",
		ReviewDigest:   "sha256:unit",
		SourceCount:    1,
		AssertionCount: 0,
		SkillCount:     0,
		RuleCount:      0,
		Assertions:     []byte(`[]`),
		Topics:         []byte(`[]`),
		ReviewSteps:    []byte(`[]`),
		UpdatedAt:      pgtype.Timestamptz{Time: time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), Valid: true},
	}
}

func TestGetTwinOverview(t *testing.T) {
	t.Parallel()

	const workspaceID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	call := func(t *testing.T, wsHeader string, mock *twinProfileMockDB) *httptest.ResponseRecorder {
		t.Helper()
		h := &Handler{Queries: db.New(mock)}
		req := httptest.NewRequest(http.MethodGet, "/api/twin/overview", nil)
		if wsHeader != "" {
			req.Header.Set("X-Workspace-ID", wsHeader)
		}
		w := httptest.NewRecorder()
		h.GetTwinOverview(w, req)
		return w
	}

	t.Run("rejects_invalid_workspace_id", func(t *testing.T) {
		w := call(t, "not-a-uuid", &twinProfileMockDB{err: pgx.ErrNoRows})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("returns_null_twin_when_missing", func(t *testing.T) {
		w := call(t, workspaceID, &twinProfileMockDB{err: pgx.ErrNoRows})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
		}
		var env TwinOverviewEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if env.Twin != nil {
			t.Fatalf("expected null twin, got %+v", env.Twin)
		}
	})

	t.Run("surfaces_database_errors", func(t *testing.T) {
		w := call(t, workspaceID, &twinProfileMockDB{err: errors.New("db down")})
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "failed to load Twin profile") {
			t.Fatalf("body = %s", w.Body.String())
		}
	})

	t.Run("surfaces_invalid_metadata", func(t *testing.T) {
		profile := sampleTwinProfile()
		profile.Assertions = []byte("null")
		w := call(t, workspaceID, &twinProfileMockDB{profile: profile})
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "invalid Twin assertions metadata") {
			t.Fatalf("body = %s", w.Body.String())
		}
	})

	t.Run("returns_mapped_overview", func(t *testing.T) {
		profile := sampleTwinProfile()
		w := call(t, workspaceID, &twinProfileMockDB{profile: profile})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
		}
		var env TwinOverviewEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if env.Twin == nil {
			t.Fatal("expected twin payload")
		}
		if env.Twin.Name != "Unit Twin" || env.Twin.State != "signed-off" || env.Twin.ReviewDigest != "sha256:unit" {
			t.Fatalf("unexpected twin: %+v", env.Twin)
		}
		if env.Twin.ID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
			t.Fatalf("id = %q", env.Twin.ID)
		}
		if env.Twin.Assertions == nil || env.Twin.Topics == nil || env.Twin.ReviewSteps == nil {
			t.Fatalf("expected non-nil empty metadata slices: %+v", env.Twin)
		}
	})
}
