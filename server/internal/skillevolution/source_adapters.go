package skillevolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/room"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrProductionSignalSourceUnavailable = errors.New("production skill evolution signal source is unavailable")

// TaskReviewSignals is the Task-owned, authorization-repeating reader. The
// existing TaskRunReviewService implements this interface directly.
type TaskReviewSignals interface {
	ListTaskRunReviewRefs(context.Context, string, string, string, int) (service.TaskRunReviewRefPage, error)
	LoadTaskRunReviewEvidence(context.Context, string, string, string) (service.TaskRunReviewEvidence, error)
}

// ManualRerunSignals is the Task-owned manual rerun reader. ExactSkillTaskIndex
// independently proves that the rerun used the Skill selected by SignalQuery.
type ManualRerunSignals interface {
	ListManualRerunRefs(context.Context, string, string, string, int) (service.ManualRerunPage, error)
	LoadManualRerunEvidence(context.Context, string, string, string) (service.ManualRerunEvidence, error)
}

// ExactSkillTaskIndex exposes content-free task identities from exact eligible
// execution manifests. Implementations must scope every lookup by workspace.
type ExactSkillTaskIndex interface {
	ListExactSkillTaskIDs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]pgtype.UUID, error)
	HasExactSkillTask(context.Context, pgtype.UUID, pgtype.UUID, pgtype.UUID) (bool, error)
}

type exactSkillTaskDBIndex struct{ queries *db.Queries }

func NewExactSkillTaskIndex(queries *db.Queries) ExactSkillTaskIndex {
	return &exactSkillTaskDBIndex{queries: queries}
}

func (index *exactSkillTaskDBIndex) ListExactSkillTaskIDs(ctx context.Context, workspaceID, skillID pgtype.UUID, limit int) ([]pgtype.UUID, error) {
	if index == nil || index.queries == nil || !validUUID(workspaceID) || !validUUID(skillID) || limit <= 0 || limit > MaxEvidenceRefs {
		return nil, ErrProductionSignalSourceUnavailable
	}
	rows, err := index.queries.ListExactSkillEvolutionTaskIDs(ctx, db.ListExactSkillEvolutionTaskIDsParams{
		WorkspaceID: workspaceID, SkillID: skillID, PageSize: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list exactly attributed Skill tasks: %w", err)
	}
	for _, taskID := range rows {
		if !validUUID(taskID) {
			return nil, ErrProductionSignalSourceUnavailable
		}
	}
	return rows, nil
}

func (index *exactSkillTaskDBIndex) HasExactSkillTask(ctx context.Context, workspaceID, taskID, skillID pgtype.UUID) (bool, error) {
	if index == nil || index.queries == nil || !validUUID(workspaceID) || !validUUID(taskID) || !validUUID(skillID) {
		return false, ErrProductionSignalSourceUnavailable
	}
	eligible, err := index.queries.HasExactSkillEvolutionTask(ctx, db.HasExactSkillEvolutionTaskParams{
		WorkspaceID: workspaceID, TaskID: taskID, SkillID: skillID,
	})
	if err != nil {
		return false, fmt.Errorf("revalidate exact Skill task attribution: %w", err)
	}
	return eligible, nil
}

type TwinSignals interface {
	ListFeedbackRefs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]service.TwinFeedbackSignalRef, error)
	LoadFeedbackEvidence(context.Context, pgtype.UUID, service.TwinFeedbackSignalRef) (service.TwinFeedbackSignalEvidence, error)
	ListAcceptedDepositionRefs(context.Context, pgtype.UUID, pgtype.UUID, int) ([]service.TwinAcceptedDepositionSignalRef, error)
	LoadAcceptedDepositionEvidence(context.Context, pgtype.UUID, service.TwinAcceptedDepositionSignalRef) (service.TwinAcceptedDepositionSignalEvidence, error)
}

// TwinSignalsFactory binds the current human actor into Twin's private-task
// authorizer. Reusing an adapter captured for a different actor is forbidden.
type TwinSignalsFactory func(pgtype.UUID) TwinSignals

func NewTaskReviewSignalSource(signals TaskReviewSignals) SignalSource {
	return NewSignalAdapter(EvidenceKindTaskReview,
		func(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
			if signals == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			// A user-triggered generation carries the reviewer identity. Scheduled
			// observation may compose a Task-owned system reader instead.
			if !query.ActorID.Valid {
				return []EvidenceRef{}, nil
			}
			workspaceID, actorID, skillID := uuidText(query.WorkspaceID), uuidText(query.ActorID), uuidText(query.SkillID)
			page, err := signals.ListTaskRunReviewRefs(ctx, workspaceID, actorID, "", query.Limit)
			if err != nil {
				return nil, err
			}
			refs := make([]EvidenceRef, 0, len(page.Refs))
			for _, source := range page.Refs {
				if source.Target != service.TaskRunReviewTargetSkillProcedure || source.SkillID != skillID {
					continue
				}
				ref, err := taskReviewEvidenceRef(source)
				if err != nil {
					return nil, err
				}
				refs = append(refs, ref)
			}
			return refs, nil
		},
		func(ctx context.Context, query SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			if signals == nil || !query.ActorID.Valid {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			evidence, err := signals.LoadTaskRunReviewEvidence(ctx, uuidText(query.WorkspaceID), uuidText(query.ActorID), expected.SourceID)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			ref, err := taskReviewEvidenceRef(service.TaskRunReviewRef{
				ID: evidence.ID, WorkspaceID: evidence.WorkspaceID, TaskID: evidence.TaskID,
				ReviewerID: evidence.ReviewerID, Outcome: evidence.Outcome, Target: evidence.Target,
				SkillID: evidence.SkillID, Digest: evidence.Digest, CreatedAt: evidence.CreatedAt,
			})
			if err != nil {
				return ResolvedEvidence{}, err
			}
			payload, err := json.Marshal(struct {
				Outcome    service.TaskRunReviewOutcome `json:"outcome"`
				Correction string                       `json:"correction,omitempty"`
				Reason     string                       `json:"reason"`
			}{evidence.Outcome, evidence.Correction, evidence.Reason})
			return ResolvedEvidence{Ref: ref, Payload: payload}, err
		},
	)
}

func NewManualRerunSignalSource(signals ManualRerunSignals, exact ExactSkillTaskIndex) SignalSource {
	return NewSignalAdapter(EvidenceKindManualRerun,
		func(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
			if signals == nil || exact == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			if !query.ActorID.Valid {
				return []EvidenceRef{}, nil
			}
			page, err := signals.ListManualRerunRefs(ctx, uuidText(query.WorkspaceID), uuidText(query.ActorID), "", query.Limit)
			if err != nil {
				return nil, err
			}
			refs := make([]EvidenceRef, 0, len(page.Refs))
			for _, source := range page.Refs {
				taskID, err := parseUUIDText(source.TaskID)
				if err != nil {
					return nil, ErrSignalSourceInvalid
				}
				eligible, err := exact.HasExactSkillTask(ctx, query.WorkspaceID, taskID, query.SkillID)
				if err != nil {
					return nil, err
				}
				if !eligible {
					continue
				}
				ref, err := manualRerunEvidenceRef(source, uuidText(query.SkillID))
				if err != nil {
					return nil, err
				}
				refs = append(refs, ref)
			}
			return refs, nil
		},
		func(ctx context.Context, query SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			if signals == nil || exact == nil || !query.ActorID.Valid {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			taskID, err := parseUUIDText(expected.SourceID)
			if err != nil {
				return ResolvedEvidence{}, ErrSignalSourceInvalid
			}
			eligible, err := exact.HasExactSkillTask(ctx, query.WorkspaceID, taskID, query.SkillID)
			if err != nil || !eligible {
				if err != nil {
					return ResolvedEvidence{}, err
				}
				return ResolvedEvidence{}, ErrSignalSourceDrift
			}
			evidence, err := signals.LoadManualRerunEvidence(ctx, uuidText(query.WorkspaceID), uuidText(query.ActorID), expected.SourceID)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			ref, err := manualRerunEvidenceRef(evidence.ManualRerunRef, uuidText(query.SkillID))
			if err != nil {
				return ResolvedEvidence{}, err
			}
			payload, err := json.Marshal(struct {
				Classification    string `json:"classification"`
				RequestedByUserID string `json:"requested_by_user_id"`
				TaskStatus        string `json:"task_status"`
				SourceTaskStatus  string `json:"source_task_status"`
			}{evidence.Classification, evidence.RequestedByUserID, evidence.TaskStatus, evidence.SourceTaskStatus})
			return ResolvedEvidence{Ref: ref, Payload: payload}, err
		},
	)
}

func NewWikiProposalSignalSource(signals service.WikiReviewedProposalSignals) SignalSource {
	return NewSignalAdapter(EvidenceKindWikiProposal,
		func(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
			if signals == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			sources, err := signals.ListReviewedProposalSignals(ctx, query.WorkspaceID, int32(query.Limit))
			if err != nil {
				return nil, err
			}
			refs := make([]EvidenceRef, 0, len(sources))
			for _, source := range sources {
				ref, err := wikiProposalEvidenceRef(source)
				if err != nil {
					return nil, err
				}
				refs = append(refs, ref)
			}
			return refs, nil
		},
		func(ctx context.Context, _ SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			if signals == nil {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			proposalID, err := parseUUIDText(expected.SourceID)
			if err != nil {
				return ResolvedEvidence{}, ErrSignalSourceInvalid
			}
			revisionID, err := parseOptionalUUIDText(expected.SourceRevisionID)
			if err != nil {
				return ResolvedEvidence{}, ErrSignalSourceInvalid
			}
			evidence, err := signals.LoadReviewedProposalSignal(ctx, service.WikiReviewedProposalSignalRef{
				WorkspaceID: proposalWorkspace(expected), ProposalID: proposalID, AcceptedRevisionID: revisionID,
				Decision: expected.SourceState, Digest: string(expected.Digest), ObservedAt: requiredTimestamp(expected.ObservedAt),
			})
			if err != nil {
				return ResolvedEvidence{}, err
			}
			ref, err := wikiProposalEvidenceRef(evidence.Ref)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			payload, err := json.Marshal(struct {
				Path         string   `json:"path"`
				Title        string   `json:"title"`
				Content      string   `json:"content"`
				Rationale    string   `json:"rationale"`
				ReviewReason string   `json:"review_reason,omitempty"`
				Citations    []string `json:"citations,omitempty"`
			}{evidence.Path, evidence.Title, evidence.Content, evidence.Rationale, evidence.ReviewReason, evidence.Citations})
			return ResolvedEvidence{Ref: ref, Payload: payload}, err
		},
	)
}

func NewRoomOutcomeSignalSource(signals room.AcceptedOutcomeSignals) SignalSource {
	return NewSignalAdapter(EvidenceKindRoomOutcome,
		func(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
			if signals == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			sources, err := signals.ListAcceptedOutcomeRefs(ctx, query.WorkspaceID, query.Limit)
			if err != nil {
				return nil, err
			}
			refs := make([]EvidenceRef, 0, len(sources))
			for _, source := range sources {
				ref, err := roomOutcomeEvidenceRef(source)
				if err != nil {
					return nil, err
				}
				refs = append(refs, ref)
			}
			return refs, nil
		},
		func(ctx context.Context, query SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			if signals == nil {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			listed, err := signals.ListAcceptedOutcomeRefs(ctx, query.WorkspaceID, room.MaxAcceptedOutcomeRefs)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			var source room.AcceptedOutcomeRef
			for _, candidate := range listed {
				ref, conversionErr := roomOutcomeEvidenceRef(candidate)
				if conversionErr == nil && sameEvidenceRef(ref, expected) {
					source = candidate
					break
				}
			}
			if !source.RoomID.Valid {
				return ResolvedEvidence{}, ErrSignalSourceDrift
			}
			evidence, err := signals.LoadAcceptedOutcomeEvidence(ctx, query.WorkspaceID, source)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			ref, err := roomOutcomeEvidenceRef(evidence.Ref)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			payload, err := json.Marshal(roomOutcomePayload{
				RoomID: uuidText(evidence.Ref.RoomID), MemoryRevisionID: uuidText(evidence.Ref.MemoryRevisionID),
				CycleID: uuidText(evidence.Ref.CycleID), RecommendationKey: evidence.Ref.RecommendationKey,
				RecommendationKind: evidence.Ref.RecommendationKind, Title: evidence.Title, Body: evidence.Body,
				Rationale: evidence.Rationale, Confidence: evidence.Confidence,
			})
			return ResolvedEvidence{Ref: ref, Payload: payload}, err
		},
	)
}

func NewTwinFeedbackSignalSource(factory TwinSignalsFactory, exact ExactSkillTaskIndex) SignalSource {
	return newTwinSignalSource(EvidenceKindTwinRunFeedback, factory, exact)
}

func NewTwinDepositionSignalSource(factory TwinSignalsFactory, exact ExactSkillTaskIndex) SignalSource {
	return newTwinSignalSource(EvidenceKindTwinDeposition, factory, exact)
}

func NewProductionSourceAdapters(
	task TaskReviewSignals,
	manual ManualRerunSignals,
	wiki service.WikiReviewedProposalSignals,
	roomSignals room.AcceptedOutcomeSignals,
	twin TwinSignalsFactory,
	exact ExactSkillTaskIndex,
) ([]SignalSource, error) {
	if task == nil || manual == nil || wiki == nil || roomSignals == nil || twin == nil || exact == nil {
		return nil, ErrProductionSignalSourceUnavailable
	}
	return []SignalSource{
		NewTaskReviewSignalSource(task), NewManualRerunSignalSource(manual, exact),
		NewWikiProposalSignalSource(wiki), NewRoomOutcomeSignalSource(roomSignals),
		NewTwinFeedbackSignalSource(twin, exact), NewTwinDepositionSignalSource(twin, exact),
	}, nil
}

type roomOutcomePayload struct {
	RoomID             string  `json:"room_id"`
	MemoryRevisionID   string  `json:"memory_revision_id"`
	CycleID            string  `json:"cycle_id"`
	RecommendationKey  string  `json:"recommendation_key"`
	RecommendationKind string  `json:"recommendation_kind"`
	Title              string  `json:"title"`
	Body               string  `json:"body"`
	Rationale          string  `json:"rationale"`
	Confidence         float64 `json:"confidence"`
}

func newTwinSignalSource(kind EvidenceKind, factory TwinSignalsFactory, exact ExactSkillTaskIndex) SignalSource {
	return NewSignalAdapter(kind,
		func(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
			if factory == nil || exact == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			if !query.ActorID.Valid {
				return []EvidenceRef{}, nil
			}
			signals := factory(query.ActorID)
			if signals == nil {
				return nil, ErrProductionSignalSourceUnavailable
			}
			tasks, err := exact.ListExactSkillTaskIDs(ctx, query.WorkspaceID, query.SkillID, query.Limit)
			if err != nil {
				return nil, err
			}
			refs := make([]EvidenceRef, 0, min(query.Limit, len(tasks)))
			for _, taskID := range tasks {
				if len(refs) == query.Limit {
					break
				}
				if kind == EvidenceKindTwinRunFeedback {
					listed, err := signals.ListFeedbackRefs(ctx, query.WorkspaceID, taskID, 1)
					if err != nil {
						return nil, err
					}
					for _, source := range listed {
						ref, err := twinFeedbackEvidenceRef(source, uuidText(query.SkillID))
						if err != nil {
							return nil, err
						}
						refs = append(refs, ref)
					}
					continue
				}
				listed, err := signals.ListAcceptedDepositionRefs(ctx, query.WorkspaceID, taskID, 1)
				if err != nil {
					return nil, err
				}
				for _, source := range listed {
					ref, err := twinDepositionEvidenceRef(source, uuidText(query.SkillID))
					if err != nil {
						return nil, err
					}
					refs = append(refs, ref)
				}
			}
			return refs, nil
		},
		func(ctx context.Context, query SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
			if factory == nil || exact == nil || !query.ActorID.Valid {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			signals := factory(query.ActorID)
			if signals == nil {
				return ResolvedEvidence{}, ErrProductionSignalSourceUnavailable
			}
			taskID, err := parseUUIDText(expected.SourceRevisionID)
			if err != nil {
				return ResolvedEvidence{}, ErrSignalSourceInvalid
			}
			eligible, err := exact.HasExactSkillTask(ctx, query.WorkspaceID, taskID, query.SkillID)
			if err != nil || !eligible {
				if err != nil {
					return ResolvedEvidence{}, err
				}
				return ResolvedEvidence{}, ErrSignalSourceDrift
			}
			if kind == EvidenceKindTwinRunFeedback {
				feedbackID, err := parseUUIDText(expected.SourceID)
				if err != nil {
					return ResolvedEvidence{}, ErrSignalSourceInvalid
				}
				listed, err := signals.ListFeedbackRefs(ctx, query.WorkspaceID, taskID, 1)
				if err != nil || len(listed) != 1 || listed[0].FeedbackID != feedbackID {
					if err != nil {
						return ResolvedEvidence{}, err
					}
					return ResolvedEvidence{}, ErrSignalSourceDrift
				}
				evidence, err := signals.LoadFeedbackEvidence(ctx, query.WorkspaceID, listed[0])
				if err != nil {
					return ResolvedEvidence{}, err
				}
				ref, err := twinFeedbackEvidenceRef(evidence.Ref, uuidText(query.SkillID))
				if err != nil {
					return ResolvedEvidence{}, err
				}
				payload, err := json.Marshal(struct {
					Rating string  `json:"rating"`
					Note   *string `json:"note,omitempty"`
				}{evidence.Rating, evidence.Note})
				return ResolvedEvidence{Ref: ref, Payload: payload}, err
			}
			depositionID, err := parseUUIDText(expected.SourceID)
			if err != nil {
				return ResolvedEvidence{}, ErrSignalSourceInvalid
			}
			listed, err := signals.ListAcceptedDepositionRefs(ctx, query.WorkspaceID, taskID, 1)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			var source service.TwinAcceptedDepositionSignalRef
			for _, candidate := range listed {
				if candidate.DepositionID == depositionID {
					source = candidate
				}
			}
			if !source.DepositionID.Valid {
				return ResolvedEvidence{}, ErrSignalSourceDrift
			}
			evidence, err := signals.LoadAcceptedDepositionEvidence(ctx, query.WorkspaceID, source)
			if err != nil {
				return ResolvedEvidence{}, err
			}
			ref, err := twinDepositionEvidenceRef(evidence.Ref, uuidText(query.SkillID))
			if err != nil {
				return ResolvedEvidence{}, err
			}
			payload, err := json.Marshal(struct {
				ProposalContent json.RawMessage `json:"proposal_content"`
			}{append(json.RawMessage(nil), evidence.ProposalContent...)})
			return ResolvedEvidence{Ref: ref, Payload: payload}, err
		},
	)
}

func taskReviewEvidenceRef(source service.TaskRunReviewRef) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: source.WorkspaceID, Kind: EvidenceKindTaskReview, SourceID: source.ID,
		SourceRevisionID: source.TaskID, TargetSkillID: source.SkillID, SourceState: string(source.Outcome),
		Digest: digest, Eligibility: EvidenceEligibilityEligible, ObservedAt: source.CreatedAt.UTC()}, nil
}

func manualRerunEvidenceRef(source service.ManualRerunRef, skillID string) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: source.WorkspaceID, Kind: EvidenceKindManualRerun, SourceID: source.TaskID,
		SourceRevisionID: source.SourceTaskID, TargetSkillID: skillID, SourceState: source.Classification,
		Digest: digest, Eligibility: EvidenceEligibilityEligible, ObservedAt: source.ObservedAt.UTC()}, nil
}

func wikiProposalEvidenceRef(source service.WikiReviewedProposalSignalRef) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil || !source.WorkspaceID.Valid || !source.ProposalID.Valid || !source.ObservedAt.Valid {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: uuidText(source.WorkspaceID), Kind: EvidenceKindWikiProposal,
		SourceID: uuidText(source.ProposalID), SourceRevisionID: optionalUUIDText(source.AcceptedRevisionID),
		SourceState: source.Decision, Digest: digest, Eligibility: EvidenceEligibilityEligible,
		ObservedAt: source.ObservedAt.Time.UTC()}, nil
}

func roomOutcomeEvidenceRef(source room.AcceptedOutcomeRef) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil || !source.WorkspaceID.Valid || !source.MemoryRevisionID.Valid {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: uuidText(source.WorkspaceID), Kind: EvidenceKindRoomOutcome,
		SourceID: source.RecommendationKey, SourceRevisionID: uuidText(source.MemoryRevisionID),
		SourceState: source.SourceState, Digest: digest, Eligibility: EvidenceEligibilityEligible,
		ObservedAt: source.ObservedAt.UTC()}, nil
}

func twinFeedbackEvidenceRef(source service.TwinFeedbackSignalRef, skillID string) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil || !source.WorkspaceID.Valid || !source.FeedbackID.Valid || !source.TaskID.Valid {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: uuidText(source.WorkspaceID), Kind: EvidenceKindTwinRunFeedback,
		SourceID: uuidText(source.FeedbackID), SourceRevisionID: uuidText(source.TaskID), TargetSkillID: skillID,
		SourceState: source.State, Digest: digest, Eligibility: EvidenceEligibilityEligible,
		ObservedAt: source.ObservedAt.UTC()}, nil
}

func twinDepositionEvidenceRef(source service.TwinAcceptedDepositionSignalRef, skillID string) (EvidenceRef, error) {
	digest, err := ParseDigest(source.Digest)
	if err != nil || !source.WorkspaceID.Valid || !source.DepositionID.Valid || !source.TaskID.Valid {
		return EvidenceRef{}, ErrSignalSourceInvalid
	}
	return EvidenceRef{WorkspaceID: uuidText(source.WorkspaceID), Kind: EvidenceKindTwinDeposition,
		SourceID: uuidText(source.DepositionID), SourceRevisionID: uuidText(source.TaskID), TargetSkillID: skillID,
		SourceState: source.State, Digest: digest, Eligibility: EvidenceEligibilityEligible,
		ObservedAt: source.ObservedAt.UTC()}, nil
}

func uuidText(value pgtype.UUID) string { return uuid.UUID(value.Bytes).String() }

func optionalUUIDText(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuidText(value)
}

func parseUUIDText(value string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil {
		return pgtype.UUID{}, fmt.Errorf("parse UUID: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func parseOptionalUUIDText(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return parseUUIDText(value)
}

func proposalWorkspace(ref EvidenceRef) pgtype.UUID {
	value, _ := parseUUIDText(ref.WorkspaceID)
	return value
}

func requiredTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
