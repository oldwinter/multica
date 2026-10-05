package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const LMWikiReviewReasonLimit = 2000

var ErrLMWikiNotFound = errors.New("lm wiki revision not found")
var ErrLMWikiAlreadyDecided = errors.New("lm wiki revision already decided")
var ErrLMWikiStale = errors.New("lm wiki revision is stale")
var ErrLMWikiInvalidReview = errors.New("invalid lm wiki review")

const lmWikiRefreshMaxAttempts = 2

type LMWikiTxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error)
}

type WikiService struct {
	Queries   *db.Queries
	TxStarter LMWikiTxStarter
	Events    *events.Bus
}

type LMWikiRefreshResult struct {
	Created  bool
	Revision db.LmWikiRevision
}

type LMWikiRevisionDetail struct {
	Revision  db.LmWikiRevision
	Citations []db.LmWikiCitation
	Review    *db.LmWikiReview
}

type LMWikiOverview struct {
	Latest, Accepted, Pending *db.LmWikiRevision
	Revisions                 []LMWikiRevisionDetail
}

func NewWikiService(queries *db.Queries, txStarter LMWikiTxStarter) *WikiService {
	return &WikiService{Queries: queries, TxStarter: txStarter}
}

func (s *WikiService) Refresh(ctx context.Context, workspaceID pgtype.UUID, trigger string, requestedBy pgtype.UUID, planKey string) (LMWikiRefreshResult, error) {
	if trigger != "manual" && trigger != "scheduled" {
		return LMWikiRefreshResult{}, fmt.Errorf("trigger %q: %w", trigger, ErrLMWikiInvalidReview)
	}
	for attempt := range lmWikiRefreshMaxAttempts {
		result, err := s.refreshOnce(ctx, workspaceID, trigger, requestedBy)
		if err == nil || attempt == lmWikiRefreshMaxAttempts-1 || !isRetryableLMWikiRefreshError(err) {
			return result, err
		}
	}
	return LMWikiRefreshResult{}, errors.New("lm wiki refresh attempts exhausted")
}

func (s *WikiService) refreshOnce(ctx context.Context, workspaceID pgtype.UUID, trigger string, requestedBy pgtype.UUID) (LMWikiRefreshResult, error) {
	tx, err := s.TxStarter.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return LMWikiRefreshResult{}, fmt.Errorf("begin lm wiki refresh: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LMWikiRefreshResult{}, ErrLMWikiNotFound
		}
		return LMWikiRefreshResult{}, fmt.Errorf("lock lm wiki workspace: %w", err)
	}
	if err := qtx.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
		return LMWikiRefreshResult{}, fmt.Errorf("lock lm wiki lifecycle: %w", err)
	}
	snapshot, err := loadLMWikiSnapshot(ctx, qtx, workspaceID)
	if err != nil {
		return LMWikiRefreshResult{}, err
	}
	latest, err := qtx.GetLatestLMWikiRevision(ctx, workspaceID)
	if err == nil && latest.SourceDigest == snapshot.SourceDigest {
		if err := tx.Commit(ctx); err != nil {
			return LMWikiRefreshResult{}, fmt.Errorf("commit unchanged lm wiki refresh: %w", err)
		}
		return LMWikiRefreshResult{Revision: latest}, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LMWikiRefreshResult{}, fmt.Errorf("load latest lm wiki revision: %w", err)
	}
	revision, err := qtx.CreateLMWikiRevision(ctx, db.CreateLMWikiRevisionParams{
		WorkspaceID: workspaceID, SourceDigest: snapshot.SourceDigest, Content: snapshot.CanonicalJSON,
		SourcePolicyVersion:     snapshot.Content.EgressPolicy.PolicyVersion,
		SourcePolicyDigest:      snapshot.Content.EgressPolicy.PolicyDigest,
		RemoteGenerationEnabled: snapshot.Content.EgressPolicy.RemoteGenerationEnabled,
		TriggerKind:             trigger, RequestedByID: requestedBy,
	})
	if err != nil {
		return LMWikiRefreshResult{}, fmt.Errorf("create lm wiki revision: %w", err)
	}
	citations, err := marshalLMWikiCitations(snapshot.Citations)
	if err != nil {
		return LMWikiRefreshResult{}, err
	}
	if err := qtx.CreateLMWikiCitations(ctx, db.CreateLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: revision.ID, Citations: citations}); err != nil {
		return LMWikiRefreshResult{}, fmt.Errorf("create lm wiki citations: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LMWikiRefreshResult{}, fmt.Errorf("commit lm wiki refresh: %w", err)
	}
	s.publishLMWikiRevisionChanged(workspaceID, requestedBy, revision)
	return LMWikiRefreshResult{Created: true, Revision: revision}, nil
}

func isRetryableLMWikiRefreshError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "40001" ||
		(pgErr.Code == "23505" && pgErr.ConstraintName == "lm_wiki_revision_workspace_number_uidx")
}

func (s *WikiService) Overview(ctx context.Context, workspaceID pgtype.UUID) (LMWikiOverview, error) {
	rows, err := s.Queries.ListLMWikiRevisions(ctx, db.ListLMWikiRevisionsParams{WorkspaceID: workspaceID, ResultLimit: 100})
	if err != nil {
		return LMWikiOverview{}, fmt.Errorf("list lm wiki revisions: %w", err)
	}
	reviews, err := s.Queries.ListLMWikiReviews(ctx, workspaceID)
	if err != nil {
		return LMWikiOverview{}, fmt.Errorf("list lm wiki reviews: %w", err)
	}
	reviewsByRevision := make(map[pgtype.UUID]db.LmWikiReview, len(reviews))
	for _, review := range reviews {
		reviewsByRevision[review.RevisionID] = review
	}
	overview := LMWikiOverview{Revisions: make([]LMWikiRevisionDetail, 0, len(rows))}
	for index := range rows {
		revision := rows[index]
		detail := LMWikiRevisionDetail{Revision: revision}
		if review, ok := reviewsByRevision[revision.ID]; ok {
			detail.Review = &review
			if review.Decision == "accepted" && overview.Accepted == nil {
				overview.Accepted = &revision
			}
		}
		overview.Revisions = append(overview.Revisions, detail)
	}
	if len(rows) > 0 {
		overview.Latest = &rows[0]
		if overview.Revisions[0].Review == nil {
			overview.Pending = &rows[0]
		}
	}
	return overview, nil
}

func (s *WikiService) Detail(ctx context.Context, workspaceID, revisionID pgtype.UUID) (LMWikiRevisionDetail, error) {
	revision, err := s.Queries.GetLMWikiRevision(ctx, db.GetLMWikiRevisionParams{WorkspaceID: workspaceID, ID: revisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return LMWikiRevisionDetail{}, ErrLMWikiNotFound
	}
	if err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("load lm wiki revision: %w", err)
	}
	citations, err := s.Queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("list lm wiki citations: %w", err)
	}
	detail := LMWikiRevisionDetail{Revision: revision, Citations: citations}
	review, err := s.Queries.GetLMWikiReview(ctx, db.GetLMWikiReviewParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if err == nil {
		detail.Review = &review
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return LMWikiRevisionDetail{}, fmt.Errorf("load lm wiki review: %w", err)
	}
	return detail, nil
}

func (s *WikiService) Review(ctx context.Context, workspaceID, revisionID, reviewerID pgtype.UUID, decision, reason string) (LMWikiRevisionDetail, error) {
	reason = strings.TrimSpace(reason)
	if (decision != "accepted" && decision != "rejected") || len([]rune(reason)) > LMWikiReviewReasonLimit || (decision == "accepted" && reason != "") {
		return LMWikiRevisionDetail{}, ErrLMWikiInvalidReview
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("begin lm wiki review: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := s.Queries.WithTx(tx)
	if _, err := qtx.LockWorkspaceForWikiArtifactCreate(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LMWikiRevisionDetail{}, ErrLMWikiNotFound
		}
		return LMWikiRevisionDetail{}, fmt.Errorf("lock lm wiki review workspace: %w", err)
	}
	if err := qtx.LockLMWikiLifecycle(ctx, workspaceID); err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("lock lm wiki review: %w", err)
	}
	revision, err := qtx.GetLMWikiRevision(ctx, db.GetLMWikiRevisionParams{WorkspaceID: workspaceID, ID: revisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return LMWikiRevisionDetail{}, ErrLMWikiNotFound
	}
	if err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("load reviewed lm wiki revision: %w", err)
	}
	existing, err := qtx.GetLMWikiReview(ctx, db.GetLMWikiReviewParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if err == nil {
		if existing.Decision != decision {
			return LMWikiRevisionDetail{}, ErrLMWikiAlreadyDecided
		}
		if err := tx.Commit(ctx); err != nil {
			return LMWikiRevisionDetail{}, fmt.Errorf("commit repeated lm wiki review: %w", err)
		}
		return s.Detail(ctx, workspaceID, revisionID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LMWikiRevisionDetail{}, fmt.Errorf("load existing lm wiki review: %w", err)
	}
	if decision == "accepted" {
		latest, latestErr := qtx.GetLatestLMWikiRevision(ctx, workspaceID)
		if latestErr != nil || latest.ID != revision.ID {
			return LMWikiRevisionDetail{}, ErrLMWikiStale
		}
		policy, policyErr := loadLMWikiSourcePolicyState(ctx, qtx, workspaceID)
		if policyErr != nil {
			return LMWikiRevisionDetail{}, policyErr
		}
		if revision.SourcePolicyVersion != policy.PolicyVersion ||
			revision.SourcePolicyDigest != policy.PolicyDigest ||
			revision.RemoteGenerationEnabled != policy.RemoteGenerationEnabled {
			return LMWikiRevisionDetail{}, ErrLMWikiStale
		}
	}
	reasonValue := pgtype.Text{String: reason, Valid: reason != ""}
	_, err = qtx.CreateLMWikiReview(ctx, db.CreateLMWikiReviewParams{WorkspaceID: workspaceID, Decision: decision, ReviewerID: reviewerID, Reason: reasonValue, RevisionID: revisionID})
	if err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("create lm wiki review: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LMWikiRevisionDetail{}, fmt.Errorf("commit lm wiki review: %w", err)
	}
	s.publishLMWikiReviewChanged(workspaceID, reviewerID, revision, decision)
	return s.Detail(ctx, workspaceID, revisionID)
}
