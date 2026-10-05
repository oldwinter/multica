package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type TwinProposalDetail struct {
	Proposal       db.TwinProposal
	Review         *db.TwinProposalReview
	Version        *db.TwinVersion
	SourceRevision db.LmWikiRevision
	Citations      []db.LmWikiCitation
	RunEvidence    *TwinDepositionRunEvidence
}

// TwinDepositionRunEvidence is the bounded audit projection exposed during
// deposition review. Raw task output and feedback notes are deliberately not
// representable here.
type TwinDepositionRunEvidence struct {
	TaskID            pgtype.UUID
	BaseTwinVersionID pgtype.UUID
	EvidenceDigest    string
	TaskStatus        string
	CompletedAt       pgtype.Timestamptz
	FeedbackRating    pgtype.Text
}

type TwinVersionDetail struct {
	Version        db.TwinVersion
	Proposal       db.TwinProposal
	SourceRevision db.LmWikiRevision
	Citations      []db.LmWikiCitation
}

type TwinOverview struct {
	Current   *db.TwinVersion
	Pending   *db.TwinProposal
	Proposals []TwinProposalDetail
	Versions  []db.TwinVersion
}

// TwinAcceptedEvidence is the immutable, egress-reviewable evidence envelope
// consumed by Twin generation. CanonicalContent is intentionally opaque here:
// Wiki owns its schema, while Twin owns citation validation and generated output.
type TwinAcceptedEvidence struct {
	RevisionID       string
	SourceDigest     string
	CanonicalContent json.RawMessage
	Citations        []LMWikiCitation
}

type TwinEvidenceProvider interface {
	LoadAcceptedEvidence(ctx context.Context, workspaceID, revisionID pgtype.UUID) (TwinAcceptedEvidence, error)
}

type transactionalTwinEvidenceProvider interface {
	withQueries(*db.Queries) TwinEvidenceProvider
}

type dbTwinEvidenceProvider struct{ queries *db.Queries }

func NewDBTwinEvidenceProvider(queries *db.Queries) TwinEvidenceProvider {
	return dbTwinEvidenceProvider{queries: queries}
}

func (p dbTwinEvidenceProvider) withQueries(queries *db.Queries) TwinEvidenceProvider {
	return dbTwinEvidenceProvider{queries: queries}
}

func (p dbTwinEvidenceProvider) LoadAcceptedEvidence(ctx context.Context, workspaceID, revisionID pgtype.UUID) (TwinAcceptedEvidence, error) {
	revision, err := loadTwinSourceForBuild(ctx, p.queries, workspaceID, revisionID)
	if err != nil {
		return TwinAcceptedEvidence{}, err
	}
	canonical, err := validateTwinAcceptedEvidenceRevision(revision)
	if err != nil {
		return TwinAcceptedEvidence{}, err
	}
	rows, err := p.queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: revision.ID})
	if err != nil {
		return TwinAcceptedEvidence{}, fmt.Errorf("load twin source citations: %w", err)
	}
	citations := make([]LMWikiCitation, len(rows))
	for index, citation := range rows {
		sourceUpdatedAt := ""
		if citation.SourceUpdatedAt.Valid {
			sourceUpdatedAt = citation.SourceUpdatedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		}
		citations[index] = LMWikiCitation{
			CitationKey:     citation.CitationKey,
			SourceType:      citation.SourceType,
			SourceID:        citation.SourceID.String(),
			SourceUpdatedAt: sourceUpdatedAt,
			Locator:         citation.Locator,
			Label:           citation.Label,
			SafeMetadata:    append(json.RawMessage(nil), citation.SafeMetadata...),
			SourceDigest:    citation.SourceDigest,
		}
	}
	return TwinAcceptedEvidence{
		RevisionID:       revision.ID.String(),
		SourceDigest:     revision.SourceDigest,
		CanonicalContent: canonical,
		Citations:        citations,
	}, nil
}

func validateTwinAcceptedEvidenceRevision(revision db.LmWikiRevision) (json.RawMessage, error) {
	policy, err := parseTwinModelEgressPolicy(revision.Content)
	if err != nil {
		return nil, ErrTwinGenerationDenied
	}
	canonical, err := canonicalizeTwinAcceptedEvidence(revision.Content)
	if err != nil || revision.SchemaVersion != 2 ||
		revision.SourcePolicyVersion != policy.PolicyVersion ||
		revision.SourcePolicyDigest != policy.PolicyDigest ||
		revision.RemoteGenerationEnabled != policy.RemoteGenerationEnabled ||
		!revision.RemoteGenerationEnabled ||
		revision.SourceDigest != digestLMWiki(canonical) {
		return nil, ErrTwinGenerationDenied
	}
	return canonical, nil
}

// PostgreSQL JSONB does not preserve the key order used by Wiki's canonical
// encoder. Decode through Wiki's closed v2 schema and encode it again before
// verifying the digest that was frozen at revision creation time.
func canonicalizeTwinAcceptedEvidence(content json.RawMessage) ([]byte, error) {
	var decoded LMWikiContent
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || decoded.SchemaVersion != 2 {
		return nil, ErrTwinGenerationDenied
	}
	if err := ensureTwinJSONEOF(decoder); err != nil {
		return nil, ErrTwinGenerationDenied
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return nil, ErrTwinGenerationDenied
	}
	return canonical, nil
}

func (s *TwinService) Overview(ctx context.Context, workspaceID pgtype.UUID) (TwinOverview, error) {
	proposals, err := s.Queries.ListTwinProposals(ctx, db.ListTwinProposalsParams{WorkspaceID: workspaceID, ResultLimit: 100})
	if err != nil {
		return TwinOverview{}, fmt.Errorf("list twin proposals: %w", err)
	}
	reviews, err := s.Queries.ListTwinProposalReviews(ctx, workspaceID)
	if err != nil {
		return TwinOverview{}, fmt.Errorf("list twin proposal reviews: %w", err)
	}
	versions, err := s.Queries.ListTwinVersions(ctx, db.ListTwinVersionsParams{WorkspaceID: workspaceID, ResultLimit: 100})
	if err != nil {
		return TwinOverview{}, fmt.Errorf("list twin versions: %w", err)
	}
	reviewsByProposal := make(map[pgtype.UUID]db.TwinProposalReview, len(reviews))
	for _, review := range reviews {
		reviewsByProposal[review.ProposalID] = review
	}
	versionsByProposal := make(map[pgtype.UUID]db.TwinVersion, len(versions))
	for _, version := range versions {
		versionsByProposal[version.ProposalID] = version
	}
	overview := TwinOverview{Proposals: make([]TwinProposalDetail, 0, len(proposals)), Versions: versions}
	if len(versions) > 0 {
		overview.Current = &versions[0]
	}
	for _, proposal := range proposals {
		detail := TwinProposalDetail{Proposal: proposal}
		if review, ok := reviewsByProposal[proposal.ID]; ok {
			detail.Review = &review
		}
		if version, ok := versionsByProposal[proposal.ID]; ok {
			detail.Version = &version
		}
		if detail.Review == nil && overview.Pending == nil {
			overview.Pending = &proposal
		}
		overview.Proposals = append(overview.Proposals, detail)
	}
	return overview, nil
}

func (s *TwinService) ProposalDetail(ctx context.Context, workspaceID, proposalID pgtype.UUID) (TwinProposalDetail, error) {
	proposal, err := s.Queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: proposalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalDetail{}, ErrTwinNotFound
	}
	if err != nil {
		return TwinProposalDetail{}, fmt.Errorf("load twin proposal: %w", err)
	}
	detail := TwinProposalDetail{Proposal: proposal}
	review, err := s.Queries.GetTwinProposalReview(ctx, db.GetTwinProposalReviewParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err == nil {
		detail.Review = &review
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalDetail{}, fmt.Errorf("load twin proposal review: %w", err)
	}
	version, err := s.Queries.GetTwinVersionByProposal(ctx, db.GetTwinVersionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err == nil {
		detail.Version = &version
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return TwinProposalDetail{}, fmt.Errorf("load twin proposal version: %w", err)
	}
	detail.SourceRevision, detail.Citations, err = loadTwinEvidence(ctx, s.Queries, workspaceID, proposal.SourceWikiRevisionID)
	if err != nil {
		return TwinProposalDetail{}, err
	}
	if proposal.Kind == "deposition" {
		detail.RunEvidence, err = loadTwinDepositionRunEvidence(ctx, s.Queries, workspaceID, proposal.ID)
		if err != nil {
			return TwinProposalDetail{}, err
		}
	}
	return detail, nil
}

func loadTwinDepositionRunEvidence(ctx context.Context, queries *db.Queries, workspaceID, proposalID pgtype.UUID) (*TwinDepositionRunEvidence, error) {
	deposition, err := queries.GetTwinDepositionByProposal(ctx, db.GetTwinDepositionByProposalParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTwinNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load Twin deposition review evidence: %w", err)
	}
	evidence := &TwinDepositionRunEvidence{
		TaskID: deposition.TaskID, BaseTwinVersionID: deposition.BaseTwinVersionID,
		EvidenceDigest: deposition.EvidenceDigest,
	}
	task, err := queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{WorkspaceID: workspaceID, ID: deposition.TaskID})
	if err == nil {
		evidence.TaskStatus = task.Status
		evidence.CompletedAt = task.CompletedAt
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load Twin deposition task state: %w", err)
	}
	feedback, err := queries.GetTwinRunFeedback(ctx, db.GetTwinRunFeedbackParams{WorkspaceID: workspaceID, TaskID: deposition.TaskID})
	if err == nil {
		evidence.FeedbackRating = pgtype.Text{String: feedback.Rating, Valid: true}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load Twin deposition feedback: %w", err)
	}
	return evidence, nil
}

func (s *TwinService) VersionDetail(ctx context.Context, workspaceID, versionID pgtype.UUID) (TwinVersionDetail, error) {
	version, err := s.Queries.GetTwinVersion(ctx, db.GetTwinVersionParams{WorkspaceID: workspaceID, ID: versionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TwinVersionDetail{}, ErrTwinNotFound
	}
	if err != nil {
		return TwinVersionDetail{}, fmt.Errorf("load twin version: %w", err)
	}
	proposal, err := s.Queries.GetTwinProposal(ctx, db.GetTwinProposalParams{WorkspaceID: workspaceID, ID: version.ProposalID})
	if err != nil {
		return TwinVersionDetail{}, fmt.Errorf("load twin version proposal: %w", err)
	}
	revision, citations, err := loadTwinEvidence(ctx, s.Queries, workspaceID, version.SourceWikiRevisionID)
	if err != nil {
		return TwinVersionDetail{}, err
	}
	return TwinVersionDetail{Version: version, Proposal: proposal, SourceRevision: revision, Citations: citations}, nil
}

func loadTwinSourceForBuild(ctx context.Context, queries *db.Queries, workspaceID, revisionID pgtype.UUID) (db.LmWikiRevision, error) {
	if _, err := queries.GetLMWikiRevision(ctx, db.GetLMWikiRevisionParams{WorkspaceID: workspaceID, ID: revisionID}); errors.Is(err, pgx.ErrNoRows) {
		return db.LmWikiRevision{}, ErrTwinNotFound
	} else if err != nil {
		return db.LmWikiRevision{}, fmt.Errorf("load twin source wiki: %w", err)
	}
	revision, err := queries.GetAcceptedLMWikiRevision(ctx, db.GetAcceptedLMWikiRevisionParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.LmWikiRevision{}, ErrTwinWikiNotAccepted
	}
	if err != nil {
		return db.LmWikiRevision{}, fmt.Errorf("load accepted twin source wiki: %w", err)
	}
	latest, err := queries.GetLatestAcceptedLMWikiRevision(ctx, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && latest.ID != revision.ID) {
		return db.LmWikiRevision{}, ErrTwinWikiStale
	}
	if err != nil {
		return db.LmWikiRevision{}, fmt.Errorf("load latest accepted twin source wiki: %w", err)
	}
	return revision, nil
}

func twinAssertions(content []byte) ([]TwinAssertion, error) {
	var proposal TwinProposalContent
	if err := json.Unmarshal(content, &proposal); err != nil {
		return nil, fmt.Errorf("decode current twin content: %w", err)
	}
	return proposal.Assertions, nil
}

func validateTwinProposalFreshness(ctx context.Context, queries *db.Queries, workspaceID pgtype.UUID, proposal db.TwinProposal) error {
	latest, err := queries.GetLatestAcceptedLMWikiRevision(ctx, workspaceID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && latest.ID != proposal.SourceWikiRevisionID) {
		return ErrTwinWikiStale
	}
	if err != nil {
		return fmt.Errorf("load latest accepted wiki for twin review: %w", err)
	}
	current, err := queries.GetCurrentTwinVersion(ctx, workspaceID)
	if proposal.BaseTwinVersionID.Valid {
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.ID != proposal.BaseTwinVersionID) {
			return ErrTwinBaseStale
		}
		if err != nil {
			return fmt.Errorf("load current twin for base review: %w", err)
		}
		return nil
	}
	if err == nil {
		return ErrTwinBaseStale
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load current twin for review: %w", err)
	}
	return nil
}

func loadTwinEvidence(ctx context.Context, queries *db.Queries, workspaceID, revisionID pgtype.UUID) (db.LmWikiRevision, []db.LmWikiCitation, error) {
	revision, err := queries.GetLMWikiRevision(ctx, db.GetLMWikiRevisionParams{WorkspaceID: workspaceID, ID: revisionID})
	if err != nil {
		return db.LmWikiRevision{}, nil, fmt.Errorf("load twin evidence revision: %w", err)
	}
	citations, err := queries.ListLMWikiCitations(ctx, db.ListLMWikiCitationsParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if err != nil {
		return db.LmWikiRevision{}, nil, fmt.Errorf("load twin evidence citations: %w", err)
	}
	return revision, citations, nil
}
