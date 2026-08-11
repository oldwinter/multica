package handler

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTwinConcurrentNaturalKeyAndVersionNumbering(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	// Given
	ctx := context.Background()
	workspaceID := createLMWikiTestWorkspace(t, ctx, "twin-concurrent")
	queries := db.New(testPool)
	revision := createAcceptedTwinWikiRevision(t, ctx, queries, workspaceID)

	// When
	const workers = 8
	proposalIDs := make(chan string, workers)
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tx, err := testPool.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			txQueries := queries.WithTx(tx)
			if err = txQueries.LockTwinLifecycle(ctx, workspaceID); err != nil {
				errCh <- err
				return
			}
			proposal, err := txQueries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
				WorkspaceID:          workspaceID,
				Kind:                 "initial",
				SourceWikiRevisionID: revision.ID,
				Content:              json.RawMessage(`{"schema_version":1,"assertions":[]}`),
				ContentDigest:        twinProposalDigest,
			})
			if err == nil {
				err = tx.Commit(ctx)
			}
			if err == nil {
				proposalIDs <- uuidToString(proposal.ID)
			}
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(proposalIDs)

	// Then
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent proposal create: %v", err)
		}
	}
	var naturalKeyID string
	for id := range proposalIDs {
		if naturalKeyID == "" {
			naturalKeyID = id
		}
		if id != naturalKeyID {
			t.Fatalf("natural-key proposal IDs differ: first=%s got=%s", naturalKeyID, id)
		}
	}
	proposals, err := queries.ListTwinProposals(ctx, db.ListTwinProposalsParams{WorkspaceID: workspaceID, ResultLimit: workers})
	if err != nil {
		t.Fatalf("list natural-key proposals: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("natural-key proposal count = %d, want 1", len(proposals))
	}

	versionProposalIDs := make([]pgtype.UUID, 0, 2)
	versionProposalIDs = append(versionProposalIDs, proposals[0].ID)
	evolution, err := queries.CreateTwinProposal(ctx, db.CreateTwinProposalParams{
		WorkspaceID:          workspaceID,
		Kind:                 "evolution",
		SourceWikiRevisionID: revision.ID,
		Content:              json.RawMessage(`{"schema_version":1,"assertions":[],"diff":{}}`),
		ContentDigest:        "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	if err != nil {
		t.Fatalf("create evolution proposal: %v", err)
	}
	versionProposalIDs = append(versionProposalIDs, evolution.ID)
	for _, proposalID := range versionProposalIDs {
		if _, err := queries.CreateTwinProposalReview(ctx, db.CreateTwinProposalReviewParams{
			WorkspaceID: workspaceID,
			ProposalID:  proposalID,
			Decision:    "accepted",
			ReviewerID:  parseUUID(testUserID),
		}); err != nil {
			t.Fatalf("accept version proposal: %v", err)
		}
	}

	numbers := make(chan int64, len(versionProposalIDs))
	errCh = make(chan error, len(versionProposalIDs))
	start = make(chan struct{})
	for _, proposalID := range versionProposalIDs {
		proposalID := proposalID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tx, err := testPool.Begin(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer func() { _ = tx.Rollback(ctx) }()
			txQueries := queries.WithTx(tx)
			if err = txQueries.LockTwinLifecycle(ctx, workspaceID); err != nil {
				errCh <- err
				return
			}
			version, err := txQueries.CreateTwinVersion(ctx, db.CreateTwinVersionParams{
				WorkspaceID:   workspaceID,
				ProposalID:    proposalID,
				SignedOffByID: parseUUID(testUserID),
			})
			if err == nil {
				err = tx.Commit(ctx)
			}
			if err == nil {
				numbers <- version.VersionNumber
			}
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(numbers)
	for err := range errCh {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("concurrent version create: %v", err)
		}
	}
	gotNumbers := make([]int64, 0, len(versionProposalIDs))
	for number := range numbers {
		gotNumbers = append(gotNumbers, number)
	}
	sort.Slice(gotNumbers, func(i, j int) bool { return gotNumbers[i] < gotNumbers[j] })
	if len(gotNumbers) != 2 || gotNumbers[0] != 1 || gotNumbers[1] != 2 {
		t.Fatalf("concurrent version numbers = %v, want [1 2]", gotNumbers)
	}
}
