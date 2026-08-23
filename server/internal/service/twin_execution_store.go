package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	twinStoredBriefingMaxBytes    = 8 * 1024
	twinAssertionIDMaxCount       = 64
	twinAssertionIDMaxBytes       = 160
	twinCitationKeyMaxCount       = 128
	twinCitationKeyMaxBytes       = 200
	twinCompilerVersionMaxBytes   = 80
	twinFeedbackNoteMaxCodepoints = 2000
)

var (
	ErrTwinExecutionInvalidInput = errors.New("invalid twin execution input")
	ErrTwinExecutionNotFound     = errors.New("twin execution record not found")
	ErrTwinExecutionConflict     = errors.New("twin execution record conflicts with existing state")
)

type TwinExecutionInputError struct {
	Field string
}

func (e *TwinExecutionInputError) Error() string {
	return "invalid twin execution input: " + e.Field
}

func (e *TwinExecutionInputError) Unwrap() error {
	return ErrTwinExecutionInvalidInput
}

type TwinExecutionStore struct {
	queries *db.Queries
}

// TwinExecutionMetrics contains counts only. It intentionally cannot carry a
// briefing, assertion, citation, feedback note, or any other user content.
type TwinExecutionMetrics struct {
	AttributedRuns      int64 `json:"attributed_runs"`
	FeedbackTotal       int64 `json:"feedback_total"`
	FeedbackHelped      int64 `json:"feedback_helped"`
	FeedbackIrrelevant  int64 `json:"feedback_irrelevant"`
	FeedbackMismatch    int64 `json:"feedback_mismatch"`
	DepositionsTotal    int64 `json:"depositions_total"`
	DepositionsPending  int64 `json:"depositions_pending"`
	DepositionsAccepted int64 `json:"depositions_accepted"`
	DepositionsRejected int64 `json:"depositions_rejected"`
	BindingsOff         int64 `json:"bindings_off"`
	BindingsPreview     int64 `json:"bindings_preview"`
	BindingsEnabled     int64 `json:"bindings_enabled"`
}

// NewTwinExecutionStore accepts ordinary or transaction-scoped queries. Claim
// finalization must pass queries.WithTx(tx) so attribution commits atomically
// with the exact task claim tuple it records.
func NewTwinExecutionStore(queries *db.Queries) *TwinExecutionStore {
	return &TwinExecutionStore{queries: queries}
}

func (s *TwinExecutionStore) GetMetrics(ctx context.Context, workspaceID pgtype.UUID) (TwinExecutionMetrics, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return TwinExecutionMetrics{}, err
	}
	row, err := s.queries.GetTwinExecutionMetrics(ctx, workspaceID)
	if err != nil {
		return TwinExecutionMetrics{}, fmt.Errorf("get Twin execution metrics: %w", err)
	}
	return TwinExecutionMetrics{
		AttributedRuns:      row.AttributedRuns,
		FeedbackTotal:       row.FeedbackTotal,
		FeedbackHelped:      row.FeedbackHelped,
		FeedbackIrrelevant:  row.FeedbackIrrelevant,
		FeedbackMismatch:    row.FeedbackMismatch,
		DepositionsTotal:    row.DepositionsTotal,
		DepositionsPending:  row.DepositionsPending,
		DepositionsAccepted: row.DepositionsAccepted,
		DepositionsRejected: row.DepositionsRejected,
		BindingsOff:         row.BindingsOff,
		BindingsPreview:     row.BindingsPreview,
		BindingsEnabled:     row.BindingsEnabled,
	}, nil
}

type TwinBindingInput struct {
	WorkspaceID   pgtype.UUID
	ScopeType     string
	ScopeID       pgtype.UUID
	State         string
	TwinVersionID pgtype.UUID
}

func (s *TwinExecutionStore) ListBindings(ctx context.Context, workspaceID pgtype.UUID) ([]db.TwinBinding, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return nil, err
	}
	bindings, err := s.queries.ListTwinBindings(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list Twin bindings: %w", err)
	}
	return bindings, nil
}

func (s *TwinExecutionStore) UpsertBinding(ctx context.Context, input TwinBindingInput) (db.TwinBinding, error) {
	if err := validateTwinBindingInput(input); err != nil {
		return db.TwinBinding{}, err
	}
	binding, err := s.queries.UpsertTwinBinding(ctx, db.UpsertTwinBindingParams{
		WorkspaceID: input.WorkspaceID, ScopeType: input.ScopeType,
		ScopeID: input.ScopeID, State: input.State, TwinVersionID: input.TwinVersionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TwinBinding{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinBinding{}, fmt.Errorf("upsert Twin binding: %w", err)
	}
	return binding, nil
}

func (s *TwinExecutionStore) DeleteBinding(ctx context.Context, workspaceID, bindingID pgtype.UUID) error {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return err
	}
	if err := requireTwinExecutionUUID("binding id", bindingID); err != nil {
		return err
	}
	deleted, err := s.queries.DeleteTwinBinding(ctx, db.DeleteTwinBindingParams{WorkspaceID: workspaceID, ID: bindingID})
	if err != nil {
		return fmt.Errorf("delete Twin binding: %w", err)
	}
	if deleted == 0 {
		return ErrTwinExecutionNotFound
	}
	return nil
}

type TwinTaskAttributionInput struct {
	WorkspaceID      pgtype.UUID
	TaskID           pgtype.UUID
	AgentID          pgtype.UUID
	RuntimeID        pgtype.UUID
	TaskDispatchedAt pgtype.Timestamptz
	TwinVersionID    pgtype.UUID
	Briefing         string
	BriefingDigest   string
	AssertionIDs     []string
	CitationKeys     []string
	PolicyScopeType  string
	PolicyScopeID    pgtype.UUID
	PolicyState      string
	CompilerVersion  string
}

func (s *TwinExecutionStore) CreateTwinTaskAttributionForClaim(ctx context.Context, input TwinTaskAttributionInput) (db.TwinTaskAttribution, error) {
	assertionIDs, citationKeys, err := validateTwinTaskAttributionInput(input)
	if err != nil {
		return db.TwinTaskAttribution{}, err
	}
	row, err := s.queries.CreateTwinTaskAttributionForClaim(ctx, db.CreateTwinTaskAttributionForClaimParams{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, AgentID: input.AgentID,
		RuntimeID: input.RuntimeID, TaskDispatchedAt: input.TaskDispatchedAt,
		TwinVersionID: input.TwinVersionID, Briefing: input.Briefing,
		BriefingDigest: input.BriefingDigest, AssertionIds: assertionIDs,
		CitationKeys: citationKeys, PolicyScopeType: input.PolicyScopeType,
		PolicyScopeID: input.PolicyScopeID, PolicyState: input.PolicyState,
		CompilerVersion: input.CompilerVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, existingErr := s.queries.GetTwinTaskAttributionByClaim(ctx, db.GetTwinTaskAttributionByClaimParams{
			WorkspaceID: input.WorkspaceID, TaskID: input.TaskID,
			RuntimeID: input.RuntimeID, TaskDispatchedAt: input.TaskDispatchedAt,
		})
		if existingErr == nil {
			return db.TwinTaskAttribution{}, ErrTwinExecutionConflict
		}
		if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
			return db.TwinTaskAttribution{}, fmt.Errorf("resolve Twin attribution conflict: %w", existingErr)
		}
		return db.TwinTaskAttribution{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinTaskAttribution{}, fmt.Errorf("create Twin task attribution: %w", err)
	}
	return twinTaskAttributionFromCreate(row), nil
}

func (s *TwinExecutionStore) GetTaskAttribution(ctx context.Context, workspaceID, attributionID pgtype.UUID) (db.TwinTaskAttribution, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return db.TwinTaskAttribution{}, err
	}
	if err := requireTwinExecutionUUID("attribution id", attributionID); err != nil {
		return db.TwinTaskAttribution{}, err
	}
	attribution, err := s.queries.GetTwinTaskAttribution(ctx, db.GetTwinTaskAttributionParams{WorkspaceID: workspaceID, ID: attributionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TwinTaskAttribution{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinTaskAttribution{}, fmt.Errorf("get Twin task attribution: %w", err)
	}
	return attribution, nil
}

func (s *TwinExecutionStore) ListTaskAttributions(ctx context.Context, workspaceID, taskID pgtype.UUID) ([]db.TwinTaskAttribution, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return nil, err
	}
	if err := requireTwinExecutionUUID("task id", taskID); err != nil {
		return nil, err
	}
	attributions, err := s.queries.ListTwinTaskAttributions(ctx, db.ListTwinTaskAttributionsParams{WorkspaceID: workspaceID, TaskID: taskID})
	if err != nil {
		return nil, fmt.Errorf("list Twin task attributions: %w", err)
	}
	return attributions, nil
}

type TwinRunFeedbackInput struct {
	WorkspaceID pgtype.UUID
	TaskID      pgtype.UUID
	Rating      string
	Note        *string
}

func (s *TwinExecutionStore) UpsertRunFeedback(ctx context.Context, input TwinRunFeedbackInput) (db.TwinRunFeedback, error) {
	if err := validateTwinRunFeedbackInput(input); err != nil {
		return db.TwinRunFeedback{}, err
	}
	note := pgtype.Text{}
	if input.Note != nil {
		note = pgtype.Text{String: *input.Note, Valid: true}
	}
	feedback, err := s.queries.UpsertTwinRunFeedback(ctx, db.UpsertTwinRunFeedbackParams{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID,
		Rating: input.Rating, Note: note,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TwinRunFeedback{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinRunFeedback{}, fmt.Errorf("upsert Twin run feedback: %w", err)
	}
	return feedback, nil
}

func (s *TwinExecutionStore) GetRunFeedback(ctx context.Context, workspaceID, taskID pgtype.UUID) (db.TwinRunFeedback, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return db.TwinRunFeedback{}, err
	}
	if err := requireTwinExecutionUUID("task id", taskID); err != nil {
		return db.TwinRunFeedback{}, err
	}
	feedback, err := s.queries.GetTwinRunFeedback(ctx, db.GetTwinRunFeedbackParams{WorkspaceID: workspaceID, TaskID: taskID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TwinRunFeedback{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinRunFeedback{}, fmt.Errorf("get Twin run feedback: %w", err)
	}
	return feedback, nil
}

type TwinDepositionInput struct {
	WorkspaceID            pgtype.UUID
	TaskID                 pgtype.UUID
	BaseTwinVersionID      pgtype.UUID
	ProposalID             pgtype.UUID
	ReplacesProposalID     pgtype.UUID
	EvidenceDigest         string
	EditedAssertionsDigest string
}

func (s *TwinExecutionStore) LinkDeposition(ctx context.Context, input TwinDepositionInput) (db.TwinDeposition, error) {
	if err := validateTwinDepositionInput(input); err != nil {
		return db.TwinDeposition{}, err
	}
	row, err := s.queries.LinkTwinDeposition(ctx, db.LinkTwinDepositionParams{
		WorkspaceID: input.WorkspaceID, TaskID: input.TaskID,
		BaseTwinVersionID: input.BaseTwinVersionID, ProposalID: input.ProposalID,
		ReplacesProposalID: input.ReplacesProposalID, EvidenceDigest: input.EvidenceDigest,
		EditedAssertionsDigest: input.EditedAssertionsDigest,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, existingErr := s.queries.GetTwinDepositionByProposal(ctx, db.GetTwinDepositionByProposalParams{
			WorkspaceID: input.WorkspaceID, ProposalID: input.ProposalID,
		})
		if existingErr == nil {
			return db.TwinDeposition{}, ErrTwinExecutionConflict
		}
		if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
			return db.TwinDeposition{}, fmt.Errorf("resolve Twin deposition conflict: %w", existingErr)
		}
		return db.TwinDeposition{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return db.TwinDeposition{}, ErrTwinExecutionConflict
		}
		return db.TwinDeposition{}, fmt.Errorf("link Twin deposition: %w", err)
	}
	return twinDepositionFromLink(row), nil
}

func (s *TwinExecutionStore) GetDeposition(ctx context.Context, workspaceID, depositionID pgtype.UUID) (db.TwinDeposition, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return db.TwinDeposition{}, err
	}
	if err := requireTwinExecutionUUID("deposition id", depositionID); err != nil {
		return db.TwinDeposition{}, err
	}
	deposition, err := s.queries.GetTwinDeposition(ctx, db.GetTwinDepositionParams{WorkspaceID: workspaceID, ID: depositionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TwinDeposition{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinDeposition{}, fmt.Errorf("get Twin deposition: %w", err)
	}
	return deposition, nil
}

func (s *TwinExecutionStore) ListDepositionsForTask(ctx context.Context, workspaceID, taskID pgtype.UUID) ([]db.TwinDeposition, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return nil, err
	}
	if err := requireTwinExecutionUUID("task id", taskID); err != nil {
		return nil, err
	}
	depositions, err := s.queries.ListTwinDepositionsForTask(ctx, db.ListTwinDepositionsForTaskParams{WorkspaceID: workspaceID, TaskID: taskID})
	if err != nil {
		return nil, fmt.Errorf("list Twin depositions: %w", err)
	}
	return depositions, nil
}

func (s *TwinExecutionStore) UpdateDepositionState(ctx context.Context, workspaceID, depositionID pgtype.UUID, state string) (db.TwinDeposition, error) {
	if err := requireTwinExecutionUUID("workspace id", workspaceID); err != nil {
		return db.TwinDeposition{}, err
	}
	if err := requireTwinExecutionUUID("deposition id", depositionID); err != nil {
		return db.TwinDeposition{}, err
	}
	if !isOneOf(state, "pending", "accepted", "rejected") {
		return db.TwinDeposition{}, invalidTwinExecutionInput("deposition state")
	}
	deposition, err := s.queries.UpdateTwinDepositionState(ctx, db.UpdateTwinDepositionStateParams{
		WorkspaceID: workspaceID, ID: depositionID, State: state,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_, existingErr := s.queries.GetTwinDeposition(ctx, db.GetTwinDepositionParams{WorkspaceID: workspaceID, ID: depositionID})
		if existingErr == nil {
			return db.TwinDeposition{}, ErrTwinExecutionConflict
		}
		if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
			return db.TwinDeposition{}, fmt.Errorf("resolve Twin deposition state conflict: %w", existingErr)
		}
		return db.TwinDeposition{}, ErrTwinExecutionNotFound
	}
	if err != nil {
		return db.TwinDeposition{}, fmt.Errorf("update Twin deposition state: %w", err)
	}
	return deposition, nil
}

func TwinBriefingDigest(briefing string) string {
	digest := sha256.Sum256([]byte(briefing))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func validateTwinBindingInput(input TwinBindingInput) error {
	for field, value := range map[string]pgtype.UUID{
		"workspace id": input.WorkspaceID, "scope id": input.ScopeID,
		"Twin version id": input.TwinVersionID,
	} {
		if err := requireTwinExecutionUUID(field, value); err != nil {
			return err
		}
	}
	if !isOneOf(input.ScopeType, "workspace", "agent", "project", "issue") {
		return invalidTwinExecutionInput("binding scope type")
	}
	if input.ScopeType == "workspace" && input.ScopeID != input.WorkspaceID {
		return invalidTwinExecutionInput("workspace binding scope id")
	}
	if !isOneOf(input.State, "off", "preview", "enabled") {
		return invalidTwinExecutionInput("binding state")
	}
	return nil
}

func validateTwinTaskAttributionInput(input TwinTaskAttributionInput) ([]byte, []byte, error) {
	for field, value := range map[string]pgtype.UUID{
		"workspace id": input.WorkspaceID, "task id": input.TaskID,
		"agent id": input.AgentID, "runtime id": input.RuntimeID,
		"Twin version id": input.TwinVersionID, "policy scope id": input.PolicyScopeID,
	} {
		if err := requireTwinExecutionUUID(field, value); err != nil {
			return nil, nil, err
		}
	}
	if !input.TaskDispatchedAt.Valid || input.TaskDispatchedAt.Time.IsZero() {
		return nil, nil, invalidTwinExecutionInput("task dispatched at")
	}
	if !utf8.ValidString(input.Briefing) || strings.IndexByte(input.Briefing, 0) >= 0 || len(input.Briefing) > twinStoredBriefingMaxBytes {
		return nil, nil, invalidTwinExecutionInput("briefing")
	}
	if input.BriefingDigest != TwinBriefingDigest(input.Briefing) {
		return nil, nil, invalidTwinExecutionInput("briefing digest")
	}
	assertionIDs, err := marshalTwinExecutionStrings("assertion ids", input.AssertionIDs, twinAssertionIDMaxCount, twinAssertionIDMaxBytes, 16*1024)
	if err != nil {
		return nil, nil, err
	}
	citationKeys, err := marshalTwinExecutionStrings("citation keys", input.CitationKeys, twinCitationKeyMaxCount, twinCitationKeyMaxBytes, 32*1024)
	if err != nil {
		return nil, nil, err
	}
	if !isOneOf(input.PolicyScopeType, "workspace", "agent", "project", "issue", "one_off") {
		return nil, nil, invalidTwinExecutionInput("policy scope type")
	}
	if input.PolicyScopeType == "workspace" && input.PolicyScopeID != input.WorkspaceID {
		return nil, nil, invalidTwinExecutionInput("workspace policy scope id")
	}
	if input.PolicyScopeType == "agent" && input.PolicyScopeID != input.AgentID {
		return nil, nil, invalidTwinExecutionInput("agent policy scope id")
	}
	if input.PolicyScopeType == "one_off" && input.PolicyScopeID != input.TaskID {
		return nil, nil, invalidTwinExecutionInput("one-off policy scope id")
	}
	if input.PolicyState != "enabled" {
		return nil, nil, invalidTwinExecutionInput("policy state")
	}
	if !validTwinExecutionText(input.CompilerVersion, twinCompilerVersionMaxBytes) {
		return nil, nil, invalidTwinExecutionInput("compiler version")
	}
	return assertionIDs, citationKeys, nil
}

func validateTwinRunFeedbackInput(input TwinRunFeedbackInput) error {
	if err := requireTwinExecutionUUID("workspace id", input.WorkspaceID); err != nil {
		return err
	}
	if err := requireTwinExecutionUUID("task id", input.TaskID); err != nil {
		return err
	}
	if !isOneOf(input.Rating, "helped", "irrelevant", "mismatch") {
		return invalidTwinExecutionInput("feedback rating")
	}
	if input.Note != nil && (!utf8.ValidString(*input.Note) || strings.IndexByte(*input.Note, 0) >= 0 || utf8.RuneCountInString(*input.Note) > twinFeedbackNoteMaxCodepoints) {
		return invalidTwinExecutionInput("feedback note")
	}
	return nil
}

func validateTwinDepositionInput(input TwinDepositionInput) error {
	for field, value := range map[string]pgtype.UUID{
		"workspace id": input.WorkspaceID, "task id": input.TaskID,
		"base Twin version id": input.BaseTwinVersionID, "proposal id": input.ProposalID,
	} {
		if err := requireTwinExecutionUUID(field, value); err != nil {
			return err
		}
	}
	if input.ReplacesProposalID.Valid && input.ReplacesProposalID.Bytes == ([16]byte{}) {
		return invalidTwinExecutionInput("replaced proposal id")
	}
	if !validTwinExecutionDigest(input.EvidenceDigest) {
		return invalidTwinExecutionInput("evidence digest")
	}
	if input.EditedAssertionsDigest != "" && !validTwinExecutionDigest(input.EditedAssertionsDigest) {
		return invalidTwinExecutionInput("edited assertions digest")
	}
	return nil
}

func requireTwinExecutionUUID(field string, value pgtype.UUID) error {
	if !value.Valid || value.Bytes == ([16]byte{}) {
		return invalidTwinExecutionInput(field)
	}
	return nil
}

func invalidTwinExecutionInput(field string) error {
	return &TwinExecutionInputError{Field: field}
}

func validTwinExecutionDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && strings.ToLower(value) == value
}

func validTwinExecutionText(value string, maxBytes int) bool {
	return value != "" && len(value) <= maxBytes && utf8.ValidString(value) && strings.IndexByte(value, 0) < 0
}

func marshalTwinExecutionStrings(field string, values []string, maxCount, maxItemBytes, maxJSONBytes int) ([]byte, error) {
	if len(values) > maxCount {
		return nil, invalidTwinExecutionInput(field)
	}
	if values == nil {
		values = []string{}
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validTwinExecutionText(value, maxItemBytes) {
			return nil, invalidTwinExecutionInput(field)
		}
		if _, exists := seen[value]; exists {
			return nil, invalidTwinExecutionInput("duplicate " + field)
		}
		seen[value] = struct{}{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal Twin execution %s: %w", field, err)
	}
	if len(encoded) > maxJSONBytes {
		return nil, invalidTwinExecutionInput(field)
	}
	return encoded, nil
}

func isOneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func twinTaskAttributionFromCreate(row db.CreateTwinTaskAttributionForClaimRow) db.TwinTaskAttribution {
	return db.TwinTaskAttribution{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TaskID: row.TaskID,
		AgentID: row.AgentID, RuntimeID: row.RuntimeID,
		TaskDispatchedAt: row.TaskDispatchedAt, TwinVersionID: row.TwinVersionID,
		Briefing: row.Briefing, BriefingDigest: row.BriefingDigest,
		AssertionIds: row.AssertionIds, CitationKeys: row.CitationKeys,
		PolicyScopeType: row.PolicyScopeType, PolicyScopeID: row.PolicyScopeID,
		PolicyState: row.PolicyState, CompilerVersion: row.CompilerVersion,
		CreatedAt: row.CreatedAt,
	}
}

func twinDepositionFromLink(row db.LinkTwinDepositionRow) db.TwinDeposition {
	return db.TwinDeposition{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TaskID: row.TaskID,
		BaseTwinVersionID: row.BaseTwinVersionID, ProposalID: row.ProposalID,
		ReplacesProposalID: row.ReplacesProposalID, EvidenceDigest: row.EvidenceDigest,
		EditedAssertionsDigest: row.EditedAssertionsDigest, State: row.State,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
