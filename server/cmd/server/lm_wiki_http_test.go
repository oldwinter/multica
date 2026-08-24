package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

func TestLMWikiHTTPEmptyAndProtectedRoutes(t *testing.T) {
	resetLMWikiHTTPFixture(t)
	response := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("empty GET status = %d", response.StatusCode)
	}
	var overview struct {
		Latest    any   `json:"latest_revision"`
		Accepted  any   `json:"accepted_revision"`
		Pending   any   `json:"pending_revision"`
		Revisions []any `json:"revisions"`
		CanManage bool  `json:"can_manage"`
	}
	decodeLMWikiHTTP(t, response, &overview)
	if overview.Latest != nil || overview.Accepted != nil || overview.Pending != nil || len(overview.Revisions) != 0 || !overview.CanManage {
		t.Fatalf("empty overview = %+v", overview)
	}

	unauthenticated := lmWikiHTTPRequest(t, "", testWorkspaceID, http.MethodGet, "/api/lm-wiki", nil)
	defer unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.StatusCode)
	}

	memberID, memberToken := createLMWikiHTTPUser(t, "member")
	if _, err := testPool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, testWorkspaceID, memberID); err != nil {
		t.Fatalf("add LM Wiki member: %v", err)
	}
	memberWrite := lmWikiHTTPRequest(t, memberToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	defer memberWrite.Body.Close()
	if memberWrite.StatusCode != http.StatusForbidden {
		t.Fatalf("member refresh status = %d", memberWrite.StatusCode)
	}
	memberRead := lmWikiHTTPRequest(t, memberToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki", nil)
	defer memberRead.Body.Close()
	if memberRead.StatusCode != http.StatusOK {
		t.Fatalf("member GET status = %d", memberRead.StatusCode)
	}

	adminID, adminToken := createLMWikiHTTPUser(t, "admin")
	if _, err := testPool.Exec(context.Background(), `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'admin')`, testWorkspaceID, adminID); err != nil {
		t.Fatalf("add LM Wiki admin: %v", err)
	}
	adminWrite := lmWikiHTTPRequest(t, adminToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	defer adminWrite.Body.Close()
	if adminWrite.StatusCode != http.StatusCreated {
		t.Fatalf("admin refresh status = %d", adminWrite.StatusCode)
	}

	_, outsiderToken := createLMWikiHTTPUser(t, "outsider")
	outsider := lmWikiHTTPRequest(t, outsiderToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki", nil)
	defer outsider.Body.Close()
	if outsider.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider GET status = %d", outsider.StatusCode)
	}
}

func TestLMWikiHTTPLifecycleSourcesAndConflicts(t *testing.T) {
	resetLMWikiHTTPFixture(t)
	previousGenerator := testHandler.TwinService.ProposalGenerator
	testHandler.TwinService.ProposalGenerator = service.InventoryTwinProposalGenerator{}
	t.Cleanup(func() { testHandler.TwinService.ProposalGenerator = previousGenerator })
	projectID, resourceID := seedLMWikiHTTPSources(t)
	policyResponse := lmWikiHTTPRequest(
		t,
		testToken,
		testWorkspaceID,
		http.MethodPut,
		"/api/lm-wiki/source-policy",
		strings.NewReader(`{"source_classes":["autopilot_run","issue","project","project_resource"],"wiki_pages":[],"remote_generation_enabled":true}`),
	)
	defer policyResponse.Body.Close()
	if policyResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(policyResponse.Body)
		t.Fatalf("enable Twin generation policy status = %d body = %s", policyResponse.StatusCode, body)
	}
	badRefresh := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", bytes.NewBufferString(`{}`))
	defer badRefresh.Body.Close()
	if badRefresh.StatusCode != http.StatusBadRequest {
		t.Fatalf("refresh with body status = %d", badRefresh.StatusCode)
	}
	firstResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	if firstResponse.StatusCode != http.StatusCreated {
		t.Fatalf("first refresh status = %d", firstResponse.StatusCode)
	}
	var first lmWikiHTTPRefresh
	decodeLMWikiHTTP(t, firstResponse, &first)
	serialized := string(first.Revision.Content)
	for _, forbidden := range []string{"local_path", "private-chat", "top-secret", "trigger_payload", "result-secret", "failure_reason"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("serialized Wiki contains forbidden value %q: %s", forbidden, serialized)
		}
	}
	detailResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki/revisions/"+first.Revision.ID, nil)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d", detailResponse.StatusCode)
	}
	detailBody, err := io.ReadAll(detailResponse.Body)
	detailResponse.Body.Close()
	if err != nil {
		t.Fatalf("read detail response: %v", err)
	}
	for _, forbidden := range []string{"private-chat", "top-secret", "trigger_payload", "result-secret", "failure_reason"} {
		if strings.Contains(string(detailBody), forbidden) {
			t.Fatalf("serialized detail contains forbidden value %q: %s", forbidden, detailBody)
		}
	}
	var detail struct {
		Citations []json.RawMessage `json:"citations"`
	}
	if err := json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if len(detail.Citations) != 4 {
		t.Fatalf("citation count = %d, want 4", len(detail.Citations))
	}
	malformedID := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki/revisions/not-a-uuid", nil)
	defer malformedID.Body.Close()
	if malformedID.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed revision status = %d", malformedID.StatusCode)
	}
	otherWorkspaceID := createLMWikiHTTPWorkspace(t)
	foreign := lmWikiHTTPRequest(t, testToken, otherWorkspaceID, http.MethodGet, "/api/lm-wiki/revisions/"+first.Revision.ID, nil)
	defer foreign.Body.Close()
	if foreign.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong-workspace detail status = %d", foreign.StatusCode)
	}
	invalidReview := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/reject", bytes.NewBufferString(`{"reason":"valid"} {}`))
	defer invalidReview.Body.Close()
	if invalidReview.StatusCode != http.StatusBadRequest {
		t.Fatalf("trailing reject JSON status = %d", invalidReview.StatusCode)
	}
	oversizedReview := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/reject", bytes.NewBufferString(`{"reason":"`+strings.Repeat("a", 64*1024)+`"}`))
	defer oversizedReview.Body.Close()
	if oversizedReview.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized reject JSON status = %d, want 413", oversizedReview.StatusCode)
	}
	invalidAccept := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/accept", bytes.NewBufferString(`{}`))
	defer invalidAccept.Body.Close()
	if invalidAccept.StatusCode != http.StatusBadRequest {
		t.Fatalf("accept body status = %d", invalidAccept.StatusCode)
	}
	unchangedResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	var unchanged lmWikiHTTPRefresh
	decodeLMWikiHTTP(t, unchangedResponse, &unchanged)
	if unchangedResponse.StatusCode != http.StatusOK || unchanged.Created || unchanged.Revision.ID != first.Revision.ID {
		t.Fatalf("unchanged refresh = %+v, status = %d", unchanged, unchangedResponse.StatusCode)
	}
	beforeAcceptResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/twins", nil)
	var beforeAccept struct {
		Pending   *json.RawMessage  `json:"pending_proposal"`
		Proposals []json.RawMessage `json:"proposals"`
	}
	decodeLMWikiHTTP(t, beforeAcceptResponse, &beforeAccept)
	if beforeAcceptResponse.StatusCode != http.StatusOK || beforeAccept.Pending != nil || len(beforeAccept.Proposals) != 0 {
		t.Fatalf("Twin overview before Wiki acceptance = %+v, status = %d", beforeAccept, beforeAcceptResponse.StatusCode)
	}
	accepted := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/accept", nil)
	defer accepted.Body.Close()
	if accepted.StatusCode != http.StatusOK {
		t.Fatalf("accept status = %d", accepted.StatusCode)
	}
	repeatedAccept := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/accept", nil)
	defer repeatedAccept.Body.Close()
	if repeatedAccept.StatusCode != http.StatusOK {
		t.Fatalf("repeated accept status = %d", repeatedAccept.StatusCode)
	}
	afterAcceptResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/twins", nil)
	var afterAccept struct {
		Pending   *json.RawMessage  `json:"pending_proposal"`
		Proposals []json.RawMessage `json:"proposals"`
	}
	decodeLMWikiHTTP(t, afterAcceptResponse, &afterAccept)
	if afterAcceptResponse.StatusCode != http.StatusOK || afterAccept.Pending != nil || len(afterAccept.Proposals) != 0 {
		t.Fatalf("Twin overview before explicit proposal = %+v, status = %d", afterAccept, afterAcceptResponse.StatusCode)
	}
	proposalResponse := lmWikiHTTPRequest(
		t,
		testToken,
		testWorkspaceID,
		http.MethodPost,
		"/api/twins/proposals",
		strings.NewReader(`{"wiki_revision_id":"`+first.Revision.ID+`"}`),
	)
	defer proposalResponse.Body.Close()
	if proposalResponse.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(proposalResponse.Body)
		t.Fatalf("explicit Twin proposal status = %d body = %s", proposalResponse.StatusCode, body)
	}
	twinResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/twins", nil)
	var twinOverview struct {
		Pending struct {
			Kind                 string `json:"kind"`
			SourceWikiRevisionID string `json:"source_wiki_revision_id"`
			Review               any    `json:"review"`
		} `json:"pending_proposal"`
		Versions []json.RawMessage `json:"versions"`
	}
	decodeLMWikiHTTP(t, twinResponse, &twinOverview)
	if twinResponse.StatusCode != http.StatusOK || twinOverview.Pending.Kind != "initial" || twinOverview.Pending.SourceWikiRevisionID != first.Revision.ID || twinOverview.Pending.Review != nil || len(twinOverview.Versions) != 0 {
		t.Fatalf("explicit Twin overview = %+v, status = %d", twinOverview, twinResponse.StatusCode)
	}

	if _, err := testPool.Exec(context.Background(), `UPDATE issue SET title = 'LM Wiki HTTP changed issue', updated_at = now() WHERE workspace_id = $1 AND title = 'LM Wiki HTTP safe issue'`, testWorkspaceID); err != nil {
		t.Fatalf("update LM Wiki source: %v", err)
	}
	secondResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	var second lmWikiHTTPRefresh
	decodeLMWikiHTTP(t, secondResponse, &second)
	if secondResponse.StatusCode != http.StatusCreated || second.Revision.ID == first.Revision.ID {
		t.Fatalf("second refresh = %+v, status = %d", second, secondResponse.StatusCode)
	}
	overviewResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodGet, "/api/lm-wiki", nil)
	var heads struct {
		Accepted struct {
			ID string `json:"id"`
		} `json:"accepted_revision"`
		Pending struct {
			ID string `json:"id"`
		} `json:"pending_revision"`
	}
	decodeLMWikiHTTP(t, overviewResponse, &heads)
	if heads.Accepted.ID != first.Revision.ID || heads.Pending.ID != second.Revision.ID {
		t.Fatalf("overview heads = %+v", heads)
	}
	if _, err := testPool.Exec(context.Background(), `DELETE FROM project_resource WHERE id = $1 AND workspace_id = $2`, resourceID, testWorkspaceID); err != nil {
		t.Fatalf("delete LM Wiki source: %v", err)
	}
	thirdResponse := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
	var third lmWikiHTTPRefresh
	decodeLMWikiHTTP(t, thirdResponse, &third)
	if strings.Contains(string(third.Revision.Content), resourceID) || !strings.Contains(string(third.Revision.Content), projectID) {
		t.Fatalf("third revision did not reflect source deletion: %s", third.Revision.Content)
	}
	longReason := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+third.Revision.ID+"/reject", bytes.NewBufferString(`{"reason":"`+strings.Repeat("x", 2001)+`"}`))
	defer longReason.Body.Close()
	if longReason.StatusCode != http.StatusBadRequest {
		t.Fatalf("long rejection reason status = %d", longReason.StatusCode)
	}
	stale := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+second.Revision.ID+"/accept", nil)
	var conflict map[string]string
	decodeLMWikiHTTP(t, stale, &conflict)
	if stale.StatusCode != http.StatusConflict || conflict["code"] != "wiki_revision_stale" {
		t.Fatalf("stale accept status = %d, response = %v", stale.StatusCode, conflict)
	}
	opposite := lmWikiHTTPRequest(t, testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/revisions/"+first.Revision.ID+"/reject", bytes.NewBufferString(`{"reason":"opposite"}`))
	decodeLMWikiHTTP(t, opposite, &conflict)
	if opposite.StatusCode != http.StatusConflict || conflict["code"] != "wiki_revision_already_decided" {
		t.Fatalf("opposite review status = %d, response = %v", opposite.StatusCode, conflict)
	}
}

func TestLMWikiHTTPConcurrentRefreshCreatesOneRevision(t *testing.T) {
	resetLMWikiHTTPFixture(t)
	seedLMWikiHTTPSources(t)
	const workers = 8
	statuses := make(chan int, workers)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			response, err := lmWikiHTTPDo(testToken, testWorkspaceID, http.MethodPost, "/api/lm-wiki/refresh", nil)
			if err != nil {
				errors <- err
				return
			}
			defer response.Body.Close()
			statuses <- response.StatusCode
		}()
	}
	group.Wait()
	close(statuses)
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent refresh request: %v", err)
	}
	created := 0
	for status := range statuses {
		if status == http.StatusCreated {
			created++
		} else if status != http.StatusOK {
			t.Fatalf("concurrent refresh status = %d", status)
		}
	}
	var revisions int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM lm_wiki_revision WHERE workspace_id = $1`, testWorkspaceID).Scan(&revisions); err != nil {
		t.Fatalf("count concurrent revisions: %v", err)
	}
	if created != 1 || revisions != 1 {
		t.Fatalf("created responses = %d, revisions = %d", created, revisions)
	}
}
