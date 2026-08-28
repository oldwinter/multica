package skillevolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

const (
	MaxStorePageSize            = 100
	MaxReviewReasonRunes        = 2000
	MaxIdempotencyKeyBytes      = 160
	MaxSafeMetricsBytes         = 16 * 1024
	MaxDispatchSnapshotSkills   = 512
	TaskDispatchContractVersion = 1
)

var (
	ErrPersistenceInvalidInput         = errors.New("invalid skill evolution persistence input")
	ErrPersistenceNotFound             = errors.New("skill evolution record not found")
	ErrPersistenceConflict             = errors.New("skill evolution persistence conflict")
	ErrPersistenceTransactionsRequired = errors.New("skill evolution persistence requires transactions")
)

type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Store keeps table details and idempotency checks inside SkillEvolution.
// Pass the pool as starter when constructing the production store; only the
// immutable multi-row revision operation requires it.
type Store struct {
	queries   *db.Queries
	txStarter TxStarter
}

func NewStore(queries *db.Queries, starter ...TxStarter) *Store {
	store := &Store{queries: queries}
	if len(starter) > 0 {
		store.txStarter = starter[0]
	}
	return store
}

type LoopConfig struct {
	WorkspaceID      pgtype.UUID
	SkillID          pgtype.UUID
	Enabled          bool
	Mode             LoopMode
	Cooldown         time.Duration
	MinimumSignals   int
	MaxEvidenceRefs  int
	MaxReplaySamples int
	MaxCostUSDTicks  int64
	PolicyVersion    string
	NextEligibleAt   time.Time
}

func (s *Store) ConfigureLoop(ctx context.Context, input LoopConfig) (db.SkillEvolutionLoop, error) {
	if s == nil || s.queries == nil || !validUUID(input.WorkspaceID) || !validUUID(input.SkillID) ||
		!input.Mode.Valid() || input.Cooldown < time.Minute || input.Cooldown > 30*24*time.Hour ||
		input.Cooldown%time.Second != 0 || input.MinimumSignals < 1 || input.MinimumSignals > MaxEvidenceRefs ||
		input.MaxEvidenceRefs < 1 || input.MaxEvidenceRefs > MaxEvidenceRefs ||
		input.MaxReplaySamples < 0 || input.MaxReplaySamples > 32 || input.MaxCostUSDTicks < 0 ||
		input.MaxCostUSDTicks > 1_000_000_000 || !boundedToken(input.PolicyVersion, 80) {
		return db.SkillEvolutionLoop{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.UpsertSkillEvolutionLoop(ctx, db.UpsertSkillEvolutionLoopParams{
		WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, Enabled: input.Enabled,
		Mode: string(input.Mode), CooldownSeconds: int32(input.Cooldown / time.Second),
		MinimumSignals: int32(input.MinimumSignals), MaxEvidenceRefs: int32(input.MaxEvidenceRefs),
		MaxReplaySamples: int32(input.MaxReplaySamples), MaxCostUsdTicks: input.MaxCostUSDTicks,
		PolicyVersion: input.PolicyVersion, NextEligibleAt: optionalTime(input.NextEligibleAt),
	})
	return row, persistenceError(err)
}

func (s *Store) GetLoop(ctx context.Context, workspaceID, skillID pgtype.UUID) (db.SkillEvolutionLoop, error) {
	if !validUUID(workspaceID) || !validUUID(skillID) {
		return db.SkillEvolutionLoop{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.GetSkillEvolutionLoop(ctx, db.GetSkillEvolutionLoopParams{WorkspaceID: workspaceID, SkillID: skillID})
	return row, persistenceError(err)
}

func (s *Store) ListEligibleLoops(ctx context.Context, eligibleAt time.Time, afterID pgtype.UUID, limit int) ([]db.SkillEvolutionLoop, error) {
	if eligibleAt.IsZero() || !validOptionalUUID(afterID) || !validPageSize(limit) {
		return nil, ErrPersistenceInvalidInput
	}
	rows, err := s.queries.ListEligibleSkillEvolutionLoops(ctx, db.ListEligibleSkillEvolutionLoopsParams{
		EligibleAt: requiredTime(eligibleAt), AfterID: afterID, PageSize: int32(limit),
	})
	return rows, persistenceError(err)
}

func (s *Store) ListScheduledLoops(ctx context.Context, eligibleAt time.Time, afterID pgtype.UUID, limit int) ([]db.SkillEvolutionLoop, error) {
	if eligibleAt.IsZero() || !validOptionalUUID(afterID) || !validPageSize(limit) {
		return nil, ErrPersistenceInvalidInput
	}
	rows, err := s.queries.ListScheduledSkillEvolutionLoops(ctx, db.ListScheduledSkillEvolutionLoopsParams{
		EligibleAt: requiredTime(eligibleAt), AfterID: afterID, PageSize: int32(limit),
	})
	return rows, persistenceError(err)
}

func (s *Store) RecordLoopObservation(ctx context.Context, workspaceID, loopID pgtype.UUID, observedAt, nextEligibleAt time.Time) (db.SkillEvolutionLoop, error) {
	if !validUUID(workspaceID) || !validUUID(loopID) || observedAt.IsZero() {
		return db.SkillEvolutionLoop{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.RecordSkillEvolutionLoopObservation(ctx, db.RecordSkillEvolutionLoopObservationParams{
		WorkspaceID: workspaceID, ID: loopID, ObservedAt: requiredTime(observedAt), NextEligibleAt: optionalTime(nextEligibleAt),
	})
	return row, persistenceError(err)
}

type RevisionFileInput struct {
	Path    string
	Content string
}

type RevisionInput struct {
	WorkspaceID    pgtype.UUID
	SkillID        pgtype.UUID
	Kind           string
	Ownership      OwnershipClass
	Source         string
	BundleHash     Digest
	MetadataDigest Digest
	Name           string
	Description    string
	PrimaryContent string
	Files          []RevisionFileInput
	CreatedByID    pgtype.UUID
}

type RevisionSnapshot struct {
	Revision db.SkillEvolutionRevision
	Files    []db.SkillEvolutionRevisionFile
}

func (s *Store) SaveRevision(ctx context.Context, input RevisionInput) (RevisionSnapshot, error) {
	manifest, sortedFiles, err := validateRevisionInput(input)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	if s == nil || s.queries == nil || s.txStarter == nil {
		return RevisionSnapshot{}, ErrPersistenceTransactionsRequired
	}
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return RevisionSnapshot{}, fmt.Errorf("begin skill evolution revision: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := s.queries.WithTx(tx)
	revision, createErr := queries.CreateSkillEvolutionRevision(ctx, db.CreateSkillEvolutionRevisionParams{
		WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, Kind: input.Kind,
		OwnershipClass: string(input.Ownership), Source: input.Source, BundleHash: string(input.BundleHash),
		MetadataDigest: string(input.MetadataDigest), Name: input.Name, Description: input.Description,
		PrimaryContent: input.PrimaryContent, ByteCount: manifest.SizeBytes,
		SupportFileCount: int32(manifest.FileCount), CreatedByID: input.CreatedByID,
	})
	created := createErr == nil
	if errors.Is(createErr, pgx.ErrNoRows) {
		revision, createErr = queries.GetSkillEvolutionRevisionByHash(ctx, db.GetSkillEvolutionRevisionByHashParams{
			WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, BundleHash: string(input.BundleHash),
		})
	}
	if createErr != nil {
		return RevisionSnapshot{}, persistenceError(createErr)
	}
	if !sameRevision(revision, input, manifest) {
		return RevisionSnapshot{}, ErrPersistenceConflict
	}
	if created {
		for index, file := range sortedFiles {
			if _, err := queries.CreateSkillEvolutionRevisionFile(ctx, db.CreateSkillEvolutionRevisionFileParams{
				WorkspaceID: input.WorkspaceID, RevisionID: revision.ID, Path: file.Path,
				Content: file.Content, Digest: manifest.Files[index].SHA256, ByteCount: int32(len(file.Content)),
			}); err != nil {
				return RevisionSnapshot{}, fmt.Errorf("create skill evolution revision file: %w", err)
			}
		}
	}
	files, err := queries.ListSkillEvolutionRevisionFiles(ctx, db.ListSkillEvolutionRevisionFilesParams{
		WorkspaceID: input.WorkspaceID, RevisionID: revision.ID,
	})
	if err != nil {
		return RevisionSnapshot{}, fmt.Errorf("list skill evolution revision files: %w", err)
	}
	if !sameRevisionFiles(files, sortedFiles, manifest) {
		return RevisionSnapshot{}, ErrPersistenceConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return RevisionSnapshot{}, fmt.Errorf("commit skill evolution revision: %w", err)
	}
	return RevisionSnapshot{Revision: revision, Files: files}, nil
}

func (s *Store) GetRevisionSnapshot(ctx context.Context, workspaceID, revisionID pgtype.UUID) (RevisionSnapshot, error) {
	if !validUUID(workspaceID) || !validUUID(revisionID) {
		return RevisionSnapshot{}, ErrPersistenceInvalidInput
	}
	revision, err := s.queries.GetSkillEvolutionRevision(ctx, db.GetSkillEvolutionRevisionParams{WorkspaceID: workspaceID, ID: revisionID})
	if err != nil {
		return RevisionSnapshot{}, persistenceError(err)
	}
	files, err := s.queries.ListSkillEvolutionRevisionFiles(ctx, db.ListSkillEvolutionRevisionFilesParams{WorkspaceID: workspaceID, RevisionID: revisionID})
	if err != nil {
		return RevisionSnapshot{}, persistenceError(err)
	}
	return RevisionSnapshot{Revision: revision, Files: files}, nil
}

type ProposalInput struct {
	WorkspaceID    pgtype.UUID
	SkillID        pgtype.UUID
	LoopID         pgtype.UUID
	BaseRevisionID pgtype.UUID
	BaseHash       Digest
	GenerationKey  string
	RequestedByID  pgtype.UUID
}

func (s *Store) CreateProposal(ctx context.Context, input ProposalInput) (db.SkillEvolutionProposal, error) {
	if !validUUID(input.WorkspaceID) || !validUUID(input.SkillID) || !validUUID(input.LoopID) ||
		!validUUID(input.BaseRevisionID) || !input.BaseHash.Valid() || !boundedToken(input.GenerationKey, MaxIdempotencyKeyBytes) ||
		!validOptionalUUID(input.RequestedByID) {
		return db.SkillEvolutionProposal{}, ErrPersistenceInvalidInput
	}
	params := db.CreateSkillEvolutionProposalParams{
		WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, LoopID: input.LoopID,
		BaseRevisionID: input.BaseRevisionID, BaseHash: string(input.BaseHash),
		GenerationIdempotencyKey: input.GenerationKey, RequestedByID: input.RequestedByID,
	}
	proposal, err := s.queries.CreateSkillEvolutionProposal(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		proposal, err = s.queries.GetSkillEvolutionProposalByGenerationKey(ctx, db.GetSkillEvolutionProposalByGenerationKeyParams{
			WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, GenerationIdempotencyKey: input.GenerationKey,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.SkillEvolutionProposal{}, ErrPersistenceConflict
		}
		return db.SkillEvolutionProposal{}, err
	}
	if proposal.LoopID != input.LoopID || proposal.BaseRevisionID != input.BaseRevisionID || proposal.BaseHash != string(input.BaseHash) ||
		proposal.RequestedByID != input.RequestedByID {
		return db.SkillEvolutionProposal{}, ErrPersistenceConflict
	}
	return proposal, nil
}

type ProposalTransition struct {
	WorkspaceID         pgtype.UUID
	ProposalID          pgtype.UUID
	ExpectedState       ProposalState
	NextState           ProposalState
	CandidateRevisionID pgtype.UUID
	CandidateHash       Digest
	RationaleDigest     Digest
	ObservedPattern     string
	ExpectedBenefit     string
	RegressionRisk      string
	FailureReason       string
	StaleReason         string
}

func (s *Store) TransitionProposal(ctx context.Context, input ProposalTransition) (db.SkillEvolutionProposal, error) {
	rationaleProvided := input.ObservedPattern != "" || input.ExpectedBenefit != "" || input.RegressionRisk != ""
	if !validUUID(input.WorkspaceID) || !validUUID(input.ProposalID) ||
		ValidateProposalTransition(input.ExpectedState, input.NextState) != nil ||
		!validOptionalUUID(input.CandidateRevisionID) ||
		(input.CandidateRevisionID.Valid != input.CandidateHash.Valid()) ||
		(input.RationaleDigest != "" && !input.RationaleDigest.Valid()) ||
		(rationaleProvided && (!input.RationaleDigest.Valid() || !validRationale(input.ObservedPattern) ||
			!validRationale(input.ExpectedBenefit) || !validRationale(input.RegressionRisk))) ||
		!boundedOptionalToken(input.FailureReason, 160) || !boundedOptionalToken(input.StaleReason, 160) {
		return db.SkillEvolutionProposal{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.TransitionSkillEvolutionProposal(ctx, db.TransitionSkillEvolutionProposalParams{
		WorkspaceID: input.WorkspaceID, ID: input.ProposalID, ExpectedState: string(input.ExpectedState), NextState: string(input.NextState),
		CandidateRevisionID: input.CandidateRevisionID, CandidateHash: optionalText(string(input.CandidateHash)),
		RationaleDigest: optionalText(string(input.RationaleDigest)), ObservedPattern: optionalText(input.ObservedPattern),
		ExpectedBenefit: optionalText(input.ExpectedBenefit), RegressionRisk: optionalText(input.RegressionRisk),
		FailureReason: optionalText(input.FailureReason), StaleReason: optionalText(input.StaleReason),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, lookupErr := s.queries.GetSkillEvolutionProposal(ctx, db.GetSkillEvolutionProposalParams{WorkspaceID: input.WorkspaceID, ID: input.ProposalID}); errors.Is(lookupErr, pgx.ErrNoRows) {
			return db.SkillEvolutionProposal{}, ErrPersistenceNotFound
		}
		return db.SkillEvolutionProposal{}, ErrPersistenceConflict
	}
	return row, persistenceError(err)
}

func (s *Store) GetProposal(ctx context.Context, workspaceID, proposalID pgtype.UUID) (db.SkillEvolutionProposal, error) {
	if !validUUID(workspaceID) || !validUUID(proposalID) {
		return db.SkillEvolutionProposal{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.GetSkillEvolutionProposal(ctx, db.GetSkillEvolutionProposalParams{WorkspaceID: workspaceID, ID: proposalID})
	return row, persistenceError(err)
}

func (s *Store) RecordEvidence(ctx context.Context, proposalID pgtype.UUID, ref EvidenceRef) (db.SkillEvolutionEvidence, error) {
	if err := ref.Validate(); err != nil || !validUUID(proposalID) {
		return db.SkillEvolutionEvidence{}, ErrPersistenceInvalidInput
	}
	workspaceID, err := parseUUID(ref.WorkspaceID)
	if err != nil {
		return db.SkillEvolutionEvidence{}, ErrPersistenceInvalidInput
	}
	var targetSkillID pgtype.UUID
	if ref.TargetSkillID != "" {
		targetSkillID, err = parseUUID(ref.TargetSkillID)
		if err != nil {
			return db.SkillEvolutionEvidence{}, ErrPersistenceInvalidInput
		}
	}
	params := db.CreateSkillEvolutionEvidenceParams{
		WorkspaceID: workspaceID, ProposalID: proposalID, Kind: string(ref.Kind), SourceID: ref.SourceID,
		SourceRevisionID: ref.SourceRevisionID, TargetSkillID: targetSkillID, SourceState: ref.SourceState,
		Digest: string(ref.Digest), Eligibility: string(ref.Eligibility), ObservedAt: requiredTime(ref.ObservedAt),
	}
	row, createErr := s.queries.CreateSkillEvolutionEvidence(ctx, params)
	if errors.Is(createErr, pgx.ErrNoRows) {
		row, createErr = s.queries.GetSkillEvolutionEvidenceByIdentity(ctx, db.GetSkillEvolutionEvidenceByIdentityParams{
			WorkspaceID: workspaceID, ProposalID: proposalID, Kind: string(ref.Kind), SourceID: ref.SourceID, SourceRevisionID: ref.SourceRevisionID,
		})
	}
	if createErr != nil {
		return db.SkillEvolutionEvidence{}, persistenceError(createErr)
	}
	if row.Digest != string(ref.Digest) || row.Eligibility != string(ref.Eligibility) || row.SourceState != ref.SourceState ||
		row.TargetSkillID != targetSkillID || !sameDatabaseTime(row.ObservedAt, ref.ObservedAt) {
		return db.SkillEvolutionEvidence{}, ErrPersistenceConflict
	}
	return row, nil
}

type EvaluationInput struct {
	WorkspaceID    pgtype.UUID
	ProposalID     pgtype.UUID
	Kind           string
	Result         EvaluationResult
	Adapter        string
	AdapterVersion string
	PolicyVersion  string
	ResultDigest   Digest
	SafeMetrics    json.RawMessage
	CostUSDTicks   pgtype.Int8
	Duration       time.Duration
	IdempotencyKey string
}

func (s *Store) RecordEvaluation(ctx context.Context, input EvaluationInput) (db.SkillEvolutionEvaluation, error) {
	if !validEvaluationInput(input) {
		return db.SkillEvolutionEvaluation{}, ErrPersistenceInvalidInput
	}
	params := db.CreateSkillEvolutionEvaluationParams{
		WorkspaceID: input.WorkspaceID, ProposalID: input.ProposalID, Kind: input.Kind, Result: string(input.Result),
		Adapter: input.Adapter, AdapterVersion: input.AdapterVersion, PolicyVersion: input.PolicyVersion,
		ResultDigest: string(input.ResultDigest), SafeMetrics: input.SafeMetrics, CostUsdTicks: input.CostUSDTicks,
		DurationMs: input.Duration.Milliseconds(), IdempotencyKey: input.IdempotencyKey,
	}
	row, err := s.queries.CreateSkillEvolutionEvaluation(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetSkillEvolutionEvaluationByIdempotencyKey(ctx, db.GetSkillEvolutionEvaluationByIdempotencyKeyParams{
			WorkspaceID: input.WorkspaceID, ProposalID: input.ProposalID, IdempotencyKey: input.IdempotencyKey,
		})
	}
	if err != nil {
		return db.SkillEvolutionEvaluation{}, persistenceError(err)
	}
	if row.Kind != input.Kind || row.Result != string(input.Result) || row.Adapter != input.Adapter ||
		row.AdapterVersion != input.AdapterVersion || row.PolicyVersion != input.PolicyVersion ||
		row.ResultDigest != string(input.ResultDigest) || !jsonEqual(row.SafeMetrics, input.SafeMetrics) ||
		row.CostUsdTicks != input.CostUSDTicks || row.DurationMs != input.Duration.Milliseconds() {
		return db.SkillEvolutionEvaluation{}, ErrPersistenceConflict
	}
	return row, nil
}

type ReviewInput struct {
	WorkspaceID         pgtype.UUID
	ProposalID          pgtype.UUID
	CandidateRevisionID pgtype.UUID
	Decision            string
	ActorID             pgtype.UUID
	Reason              string
	IdempotencyKey      string
}

func (s *Store) RecordReview(ctx context.Context, input ReviewInput) (db.SkillEvolutionReview, error) {
	if !validUUID(input.WorkspaceID) || !validUUID(input.ProposalID) || !validUUID(input.CandidateRevisionID) ||
		!validUUID(input.ActorID) || (input.Decision != "rejected" && input.Decision != "publish") ||
		len([]rune(input.Reason)) > MaxReviewReasonRunes || !boundedToken(input.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return db.SkillEvolutionReview{}, ErrPersistenceInvalidInput
	}
	params := db.CreateSkillEvolutionReviewParams{
		WorkspaceID: input.WorkspaceID, ProposalID: input.ProposalID, CandidateRevisionID: input.CandidateRevisionID,
		Decision: input.Decision, ActorID: input.ActorID, Reason: optionalText(input.Reason), IdempotencyKey: input.IdempotencyKey,
	}
	row, err := s.queries.CreateSkillEvolutionReview(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetSkillEvolutionReviewByIdempotencyKey(ctx, db.GetSkillEvolutionReviewByIdempotencyKeyParams{
			WorkspaceID: input.WorkspaceID, ProposalID: input.ProposalID, IdempotencyKey: input.IdempotencyKey,
		})
	}
	if err != nil {
		return db.SkillEvolutionReview{}, persistenceError(err)
	}
	if row.CandidateRevisionID != input.CandidateRevisionID || row.Decision != input.Decision || row.ActorID != input.ActorID || row.Reason != optionalText(input.Reason) {
		return db.SkillEvolutionReview{}, ErrPersistenceConflict
	}
	return row, nil
}

type ReleaseInput struct {
	WorkspaceID      pgtype.UUID
	SkillID          pgtype.UUID
	ProposalID       pgtype.UUID
	SourceReleaseID  pgtype.UUID
	RevisionID       pgtype.UUID
	Kind             ReleaseKind
	ExpectedBaseHash Digest
	ActorID          pgtype.UUID
	IdempotencyKey   string
}

func (s *Store) CreateRelease(ctx context.Context, input ReleaseInput) (db.SkillEvolutionRelease, error) {
	if !validReleaseInput(input) {
		return db.SkillEvolutionRelease{}, ErrPersistenceInvalidInput
	}
	params := db.CreateSkillEvolutionReleaseParams{
		WorkspaceID: input.WorkspaceID, SkillID: input.SkillID, ProposalID: input.ProposalID,
		SourceReleaseID: input.SourceReleaseID, RevisionID: input.RevisionID, Kind: string(input.Kind),
		ExpectedBaseHash: string(input.ExpectedBaseHash), ActorID: input.ActorID, IdempotencyKey: input.IdempotencyKey,
	}
	row, err := s.queries.CreateSkillEvolutionRelease(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetSkillEvolutionReleaseByIdempotencyKey(ctx, db.GetSkillEvolutionReleaseByIdempotencyKeyParams{
			WorkspaceID: input.WorkspaceID, IdempotencyKey: input.IdempotencyKey,
		})
	}
	if err != nil {
		return db.SkillEvolutionRelease{}, persistenceError(err)
	}
	if row.SkillID != input.SkillID || row.ProposalID != input.ProposalID || row.SourceReleaseID != input.SourceReleaseID ||
		row.RevisionID != input.RevisionID || row.Kind != string(input.Kind) || row.ExpectedBaseHash != string(input.ExpectedBaseHash) ||
		row.ActorID != input.ActorID {
		return db.SkillEvolutionRelease{}, ErrPersistenceConflict
	}
	return row, nil
}

type ReleaseTransition struct {
	WorkspaceID     pgtype.UUID
	ReleaseID       pgtype.UUID
	ExpectedOutcome ReleaseOutcome
	NextOutcome     ReleaseOutcome
	PreHash         Digest
	PostHash        Digest
	ErrorCode       string
}

func (s *Store) TransitionRelease(ctx context.Context, input ReleaseTransition) (db.SkillEvolutionRelease, error) {
	if !validUUID(input.WorkspaceID) || !validUUID(input.ReleaseID) ||
		ValidateReleaseTransition(input.ExpectedOutcome, input.NextOutcome) != nil ||
		(input.PreHash != "" && !input.PreHash.Valid()) || (input.PostHash != "" && !input.PostHash.Valid()) ||
		(input.NextOutcome == ReleaseOutcomeSucceeded && (!input.PreHash.Valid() || !input.PostHash.Valid())) ||
		(input.NextOutcome == ReleaseOutcomeSucceeded && input.ErrorCode != "") ||
		(input.NextOutcome != ReleaseOutcomeSucceeded && !boundedToken(input.ErrorCode, 160)) ||
		!boundedOptionalToken(input.ErrorCode, 160) {
		return db.SkillEvolutionRelease{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.TransitionSkillEvolutionRelease(ctx, db.TransitionSkillEvolutionReleaseParams{
		WorkspaceID: input.WorkspaceID, ID: input.ReleaseID, ExpectedOutcome: string(input.ExpectedOutcome), NextOutcome: string(input.NextOutcome),
		PreHash: optionalText(string(input.PreHash)), PostHash: optionalText(string(input.PostHash)), ErrorCode: optionalText(input.ErrorCode),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, lookupErr := s.queries.GetSkillEvolutionRelease(ctx, db.GetSkillEvolutionReleaseParams{WorkspaceID: input.WorkspaceID, ID: input.ReleaseID}); errors.Is(lookupErr, pgx.ErrNoRows) {
			return db.SkillEvolutionRelease{}, ErrPersistenceNotFound
		}
		return db.SkillEvolutionRelease{}, ErrPersistenceConflict
	}
	return row, persistenceError(err)
}

type TaskAttributionInput struct {
	WorkspaceID        pgtype.UUID
	TaskID             pgtype.UUID
	RuntimeID          pgtype.UUID
	SkillID            pgtype.UUID
	RevisionID         pgtype.UUID
	ManifestVersion    int
	Source             string
	BundleHash         Digest
	ManifestDigest     Digest
	Eligibility        EvidenceEligibility
	Reason             string
	DispatchSnapshotID pgtype.UUID
	TaskDispatchedAt   time.Time
}

func (s *Store) RecordTaskAttribution(ctx context.Context, input TaskAttributionInput) (db.SkillEvolutionTaskAttribution, error) {
	if !validAttributionInput(input) {
		return db.SkillEvolutionTaskAttribution{}, ErrPersistenceInvalidInput
	}
	params := db.RecordSkillEvolutionTaskAttributionParams{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, RuntimeID: input.RuntimeID,
		SkillID: input.SkillID, RevisionID: input.RevisionID, ManifestVersion: int32(input.ManifestVersion),
		Source: input.Source, BundleHash: string(input.BundleHash), ManifestDigest: string(input.ManifestDigest),
		Eligibility: string(input.Eligibility), Reason: input.Reason, DispatchSnapshotID: input.DispatchSnapshotID,
		TaskDispatchedAt: requiredTime(input.TaskDispatchedAt),
	}
	row, err := s.queries.RecordSkillEvolutionTaskAttribution(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetSkillEvolutionTaskAttribution(ctx, db.GetSkillEvolutionTaskAttributionParams{
			WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, SkillID: input.SkillID,
		})
	}
	if err != nil {
		return db.SkillEvolutionTaskAttribution{}, persistenceError(err)
	}
	if row.RuntimeID != input.RuntimeID || row.RevisionID != input.RevisionID || row.ManifestVersion != int32(input.ManifestVersion) ||
		row.Source != input.Source || row.BundleHash != string(input.BundleHash) || row.ManifestDigest != string(input.ManifestDigest) ||
		row.Eligibility != string(input.Eligibility) || row.Reason != input.Reason || row.DispatchSnapshotID != input.DispatchSnapshotID ||
		!sameDatabaseTime(row.TaskDispatchedAt, input.TaskDispatchedAt) {
		return db.SkillEvolutionTaskAttribution{}, ErrPersistenceConflict
	}
	return row, nil
}

type TaskDispatchSnapshotInput struct {
	WorkspaceID      pgtype.UUID
	TaskID           pgtype.UUID
	AgentID          pgtype.UUID
	RuntimeID        pgtype.UUID
	TaskDispatchedAt time.Time
	Skills           []DispatchSkillIdentity
}

type TaskDispatchSnapshot struct {
	Snapshot db.SkillEvolutionTaskDispatchSnapshot
	Skills   []DispatchSkillIdentity
}

type taskDispatchSnapshotSkill struct {
	Source  string `json:"source"`
	SkillID string `json:"skill_id"`
}

func (s *Store) RecordTaskDispatchSnapshot(ctx context.Context, input TaskDispatchSnapshotInput) (TaskDispatchSnapshot, error) {
	if s == nil || s.queries == nil || !validUUID(input.WorkspaceID) || !validUUID(input.TaskID) ||
		!validUUID(input.AgentID) || !validUUID(input.RuntimeID) || input.TaskDispatchedAt.IsZero() {
		return TaskDispatchSnapshot{}, ErrPersistenceInvalidInput
	}
	encoded, digest, canonical, err := canonicalTaskDispatchSnapshot(input.Skills)
	if err != nil {
		return TaskDispatchSnapshot{}, err
	}
	params := db.RecordSkillEvolutionTaskDispatchSnapshotParams{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, AgentID: input.AgentID, RuntimeID: input.RuntimeID,
		TaskDispatchedAt: requiredTime(input.TaskDispatchedAt), ContractVersion: TaskDispatchContractVersion,
		Identities: encoded, IdentityCount: int32(len(canonical)), IdentitiesDigest: string(digest),
	}
	row, err := s.queries.RecordSkillEvolutionTaskDispatchSnapshot(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.queries.GetSkillEvolutionTaskDispatchSnapshot(ctx, db.GetSkillEvolutionTaskDispatchSnapshotParams{
			WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, AgentID: input.AgentID, RuntimeID: input.RuntimeID,
			TaskDispatchedAt: requiredTime(input.TaskDispatchedAt),
		})
	}
	if err != nil {
		return TaskDispatchSnapshot{}, persistenceError(err)
	}
	if row.WorkspaceID != input.WorkspaceID || row.TaskID != input.TaskID || row.AgentID != input.AgentID || row.RuntimeID != input.RuntimeID ||
		!sameDatabaseTime(row.TaskDispatchedAt, input.TaskDispatchedAt) || row.ContractVersion != TaskDispatchContractVersion ||
		row.IdentityCount != int32(len(canonical)) || row.IdentitiesDigest != string(digest) || !jsonEqual(row.Identities, encoded) {
		return TaskDispatchSnapshot{}, ErrPersistenceConflict
	}
	return TaskDispatchSnapshot{Snapshot: row, Skills: canonical}, nil
}

func (s *Store) GetTaskDispatchSnapshot(ctx context.Context, workspaceID, taskID, agentID, runtimeID pgtype.UUID, taskDispatchedAt time.Time) (TaskDispatchSnapshot, error) {
	if s == nil || s.queries == nil || !validUUID(workspaceID) || !validUUID(taskID) || !validUUID(agentID) || !validUUID(runtimeID) || taskDispatchedAt.IsZero() {
		return TaskDispatchSnapshot{}, ErrPersistenceInvalidInput
	}
	row, err := s.queries.GetSkillEvolutionTaskDispatchSnapshot(ctx, db.GetSkillEvolutionTaskDispatchSnapshotParams{
		WorkspaceID: workspaceID, TaskID: taskID, AgentID: agentID, RuntimeID: runtimeID, TaskDispatchedAt: requiredTime(taskDispatchedAt),
	})
	if err != nil {
		return TaskDispatchSnapshot{}, persistenceError(err)
	}
	var wire []taskDispatchSnapshotSkill
	if row.ContractVersion != TaskDispatchContractVersion || row.IdentityCount < 1 || row.IdentityCount > MaxDispatchSnapshotSkills ||
		json.Unmarshal(row.Identities, &wire) != nil || len(wire) != int(row.IdentityCount) {
		return TaskDispatchSnapshot{}, ErrPersistenceConflict
	}
	skills := make([]DispatchSkillIdentity, len(wire))
	for index, item := range wire {
		skills[index] = DispatchSkillIdentity{Source: item.Source, SkillID: item.SkillID}
	}
	encoded, digest, canonical, err := canonicalTaskDispatchSnapshot(skills)
	if err != nil || !jsonEqual(encoded, row.Identities) || row.IdentitiesDigest != string(digest) {
		return TaskDispatchSnapshot{}, ErrPersistenceConflict
	}
	return TaskDispatchSnapshot{Snapshot: row, Skills: canonical}, nil
}

func canonicalTaskDispatchSnapshot(skills []DispatchSkillIdentity) ([]byte, Digest, []DispatchSkillIdentity, error) {
	if len(skills) == 0 || len(skills) > MaxDispatchSnapshotSkills {
		return nil, "", nil, ErrPersistenceInvalidInput
	}
	canonical := append([]DispatchSkillIdentity(nil), skills...)
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].Source == canonical[j].Source {
			return canonical[i].SkillID < canonical[j].SkillID
		}
		return canonical[i].Source < canonical[j].Source
	})
	wire := make([]taskDispatchSnapshotSkill, len(canonical))
	for index, skill := range canonical {
		if !validTaskDispatchSource(skill.Source) || !validAttributionIdentity(skill.SkillID) ||
			(index > 0 && canonical[index-1].Source == skill.Source && canonical[index-1].SkillID == skill.SkillID) {
			return nil, "", nil, ErrPersistenceInvalidInput
		}
		wire[index] = taskDispatchSnapshotSkill{Source: skill.Source, SkillID: skill.SkillID}
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, "", nil, ErrPersistenceInvalidInput
	}
	sum := sha256.Sum256(append([]byte("skill-execution-dispatch-snapshot-v1\x00"), encoded...))
	return encoded, Digest("sha256:" + hex.EncodeToString(sum[:])), canonical, nil
}

func validTaskDispatchSource(source string) bool {
	return source == skillbundle.SourceWorkspace || source == skillbundle.SourceBuiltin || source == skillbundle.SourcePlugin
}

type ProposalDetail struct {
	Proposal    db.SkillEvolutionProposal
	Evidence    []db.SkillEvolutionEvidence
	Evaluations []db.SkillEvolutionEvaluation
	Reviews     []db.SkillEvolutionReview
	Rationale   *ImprovementRationale
}

func (s *Store) GetProposalDetail(ctx context.Context, workspaceID, proposalID pgtype.UUID) (ProposalDetail, error) {
	proposal, err := s.GetProposal(ctx, workspaceID, proposalID)
	if err != nil {
		return ProposalDetail{}, err
	}
	evidence, err := s.queries.ListSkillEvolutionEvidence(ctx, db.ListSkillEvolutionEvidenceParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err != nil {
		return ProposalDetail{}, persistenceError(err)
	}
	evaluations, err := s.queries.ListSkillEvolutionEvaluations(ctx, db.ListSkillEvolutionEvaluationsParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err != nil {
		return ProposalDetail{}, persistenceError(err)
	}
	reviews, err := s.queries.ListSkillEvolutionReviews(ctx, db.ListSkillEvolutionReviewsParams{WorkspaceID: workspaceID, ProposalID: proposalID})
	if err != nil {
		return ProposalDetail{}, persistenceError(err)
	}
	detail := ProposalDetail{Proposal: proposal, Evidence: evidence, Evaluations: evaluations, Reviews: reviews}
	if proposal.ObservedPattern.Valid || proposal.ExpectedBenefit.Valid || proposal.RegressionRisk.Valid {
		if !proposal.ObservedPattern.Valid || !proposal.ExpectedBenefit.Valid || !proposal.RegressionRisk.Valid || !proposal.RationaleDigest.Valid {
			return ProposalDetail{}, ErrPersistenceConflict
		}
		evidenceDigests := make([]Digest, len(evidence))
		for index, item := range evidence {
			evidenceDigests[index] = Digest(item.Digest)
		}
		candidate := ImprovementCandidate{
			ObservedPattern: proposal.ObservedPattern.String, ExpectedBenefit: proposal.ExpectedBenefit.String,
			RegressionRisk: proposal.RegressionRisk.String, EvidenceDigests: evidenceDigests,
		}
		digest, err := improvementRationaleDigest(candidate)
		if err != nil || string(digest) != proposal.RationaleDigest.String {
			return ProposalDetail{}, ErrPersistenceConflict
		}
		detail.Rationale = &ImprovementRationale{
			ObservedPattern: candidate.ObservedPattern, ExpectedBenefit: candidate.ExpectedBenefit, RegressionRisk: candidate.RegressionRisk,
		}
	}
	return detail, nil
}

type ProposalSummary = db.ListSkillEvolutionProposalsRow

type Overview struct {
	Loop      db.SkillEvolutionLoop
	Revisions []db.SkillEvolutionRevision
	Proposals []ProposalSummary
	Releases  []db.SkillEvolutionRelease
}

func (s *Store) GetOverview(ctx context.Context, workspaceID, skillID pgtype.UUID, limit int) (Overview, error) {
	if !validPageSize(limit) {
		return Overview{}, ErrPersistenceInvalidInput
	}
	loop, err := s.GetLoop(ctx, workspaceID, skillID)
	if err != nil {
		return Overview{}, err
	}
	revisions, err := s.queries.ListSkillEvolutionRevisions(ctx, db.ListSkillEvolutionRevisionsParams{WorkspaceID: workspaceID, SkillID: skillID, PageSize: int32(limit)})
	if err != nil {
		return Overview{}, persistenceError(err)
	}
	proposals, err := s.queries.ListSkillEvolutionProposals(ctx, db.ListSkillEvolutionProposalsParams{WorkspaceID: workspaceID, SkillID: skillID, PageSize: int32(limit)})
	if err != nil {
		return Overview{}, persistenceError(err)
	}
	releases, err := s.queries.ListSkillEvolutionReleases(ctx, db.ListSkillEvolutionReleasesParams{WorkspaceID: workspaceID, SkillID: skillID, PageSize: int32(limit)})
	if err != nil {
		return Overview{}, persistenceError(err)
	}
	return Overview{Loop: loop, Revisions: revisions, Proposals: proposals, Releases: releases}, nil
}

func validateRevisionInput(input RevisionInput) (skillbundle.Manifest, []RevisionFileInput, error) {
	if !validUUID(input.WorkspaceID) || !validUUID(input.SkillID) || !validOptionalUUID(input.CreatedByID) ||
		(input.Kind != "base" && input.Kind != "candidate" && input.Kind != "release") || !input.Ownership.Valid() ||
		!input.BundleHash.Valid() || !input.MetadataDigest.Valid() {
		return skillbundle.Manifest{}, nil, ErrPersistenceInvalidInput
	}
	files := append([]RevisionFileInput(nil), input.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	bundleFiles := make([]skillbundle.File, len(files))
	for index, file := range files {
		bundleFiles[index] = skillbundle.File{Path: file.Path, Content: file.Content}
	}
	manifest, err := skillbundle.BuildValidatedManifest(skillbundle.Skill{
		ID: uuid.UUID(input.SkillID.Bytes).String(), Source: input.Source, Name: input.Name,
		Description: input.Description, Content: input.PrimaryContent, Files: bundleFiles,
	})
	if err != nil || manifest.Hash != string(input.BundleHash) {
		return skillbundle.Manifest{}, nil, ErrPersistenceInvalidInput
	}
	return manifest, files, nil
}

func sameRevision(row db.SkillEvolutionRevision, input RevisionInput, manifest skillbundle.Manifest) bool {
	return row.WorkspaceID == input.WorkspaceID && row.SkillID == input.SkillID && row.Kind == input.Kind &&
		row.OwnershipClass == string(input.Ownership) && row.Source == input.Source && row.BundleHash == string(input.BundleHash) &&
		row.MetadataDigest == string(input.MetadataDigest) && row.Name == input.Name && row.Description == input.Description &&
		row.PrimaryContent == input.PrimaryContent && row.ByteCount == manifest.SizeBytes && row.SupportFileCount == int32(manifest.FileCount) &&
		row.CreatedByID == input.CreatedByID
}

func sameRevisionFiles(rows []db.SkillEvolutionRevisionFile, files []RevisionFileInput, manifest skillbundle.Manifest) bool {
	if len(rows) != len(files) || len(rows) != len(manifest.Files) {
		return false
	}
	for index := range rows {
		if rows[index].Path != files[index].Path || rows[index].Content != files[index].Content ||
			rows[index].Digest != manifest.Files[index].SHA256 || rows[index].ByteCount != int32(len(files[index].Content)) {
			return false
		}
	}
	return true
}

func validEvaluationInput(input EvaluationInput) bool {
	if !validUUID(input.WorkspaceID) || !validUUID(input.ProposalID) ||
		(input.Kind != "deterministic_validation" && input.Kind != "behavioral_replay") || !input.Result.Valid() ||
		!boundedToken(input.Adapter, 80) || !boundedToken(input.AdapterVersion, 80) || !boundedToken(input.PolicyVersion, 80) ||
		!input.ResultDigest.Valid() || len(input.SafeMetrics) > MaxSafeMetricsBytes || !json.Valid(input.SafeMetrics) ||
		input.Duration < 0 || input.Duration > 24*time.Hour || !boundedToken(input.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.SafeMetrics, &object); err != nil || object == nil {
		return false
	}
	return !input.CostUSDTicks.Valid || (input.CostUSDTicks.Int64 >= 0 && input.CostUSDTicks.Int64 <= 1_000_000_000)
}

func validReleaseInput(input ReleaseInput) bool {
	if !validUUID(input.WorkspaceID) || !validUUID(input.SkillID) || !validUUID(input.RevisionID) || !validUUID(input.ActorID) ||
		!input.Kind.Valid() || !input.ExpectedBaseHash.Valid() || !boundedToken(input.IdempotencyKey, MaxIdempotencyKeyBytes) {
		return false
	}
	if input.Kind == ReleaseKindPublish {
		return validUUID(input.ProposalID) && !input.SourceReleaseID.Valid
	}
	return !input.ProposalID.Valid && validUUID(input.SourceReleaseID)
}

func validAttributionInput(input TaskAttributionInput) bool {
	return validUUID(input.WorkspaceID) && validUUID(input.TaskID) && validUUID(input.RuntimeID) &&
		validUUID(input.SkillID) && validUUID(input.RevisionID) && input.ManifestVersion == SkillExecutionManifestVersion &&
		boundedToken(input.Source, 80) && input.BundleHash.Valid() && input.ManifestDigest.Valid() && input.Eligibility.Valid() &&
		boundedToken(input.Reason, 160) && validUUID(input.DispatchSnapshotID) && !input.TaskDispatchedAt.IsZero()
}

func validUUID(value pgtype.UUID) bool { return value.Valid && value.Bytes != [16]byte{} }

func validOptionalUUID(value pgtype.UUID) bool { return !value.Valid || value.Bytes != [16]byte{} }

func parseUUID(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return pgtype.UUID{}, ErrPersistenceInvalidInput
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func validPageSize(limit int) bool { return limit > 0 && limit <= MaxStorePageSize }

func boundedToken(value string, max int) bool {
	return value != "" && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n\t")
}

func boundedOptionalToken(value string, max int) bool { return value == "" || boundedToken(value, max) }

func optionalText(value string) pgtype.Text { return pgtype.Text{String: value, Valid: value != ""} }

func requiredTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optionalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

func sameDatabaseTime(value pgtype.Timestamptz, expected time.Time) bool {
	return value.Valid && value.Time.Equal(expected.Truncate(time.Microsecond))
}

func jsonEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}

func persistenceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPersistenceNotFound
	}
	return err
}
