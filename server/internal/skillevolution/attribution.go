package skillevolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

type AttributionReason string

const (
	AttributionReasonExactRevisionMatch      AttributionReason = "exact_revision_match"
	AttributionReasonCapabilityNotProven     AttributionReason = "capability_not_proven"
	AttributionReasonInvalidInput            AttributionReason = "invalid_input"
	AttributionReasonInvalidDispatch         AttributionReason = "invalid_dispatch"
	AttributionReasonIncompleteManifest      AttributionReason = "incomplete_manifest"
	AttributionReasonUnexpectedManifestSkill AttributionReason = "unexpected_manifest_skill"
	AttributionReasonDispatchMismatch        AttributionReason = "dispatch_mismatch"
	AttributionReasonRevisionNotFound        AttributionReason = "revision_not_found"
	AttributionReasonRevisionAmbiguous       AttributionReason = "revision_ambiguous"
	AttributionReasonRevisionNotProven       AttributionReason = "revision_not_proven"
	AttributionReasonRevisionSkillMismatch   AttributionReason = "revision_skill_mismatch"
	AttributionReasonRevisionSourceMismatch  AttributionReason = "revision_source_mismatch"
	AttributionReasonRevisionHashMismatch    AttributionReason = "revision_hash_mismatch"
	AttributionReasonOwnershipIneligible     AttributionReason = "ownership_ineligible"
	AttributionReasonTaskDispatchNotProven   AttributionReason = "task_dispatch_not_proven"
	AttributionReasonPersistenceConflict     AttributionReason = "persistence_conflict"
	AttributionReasonPersistenceUnavailable  AttributionReason = "persistence_unavailable"
)

// DispatchSkillIdentity is the content-free identity of a Multica bundle that
// the server dispatched. Bundle hashes are deliberately excluded: the daemon
// may resolve a newer bundle before execution, and its post-resolution hash is
// the value that must be attributed.
type DispatchSkillIdentity struct {
	Source  string
	SkillID string
}

// AttributionInput carries only identities and a manifest that has already
// crossed the neutral skillbundle normalizer. Record defensively normalizes it
// again so direct callers cannot accidentally weaken the eligibility gate.
type AttributionInput struct {
	WorkspaceID      pgtype.UUID
	TaskID           pgtype.UUID
	RuntimeID        pgtype.UUID
	CapabilityProven bool
	DispatchedSkills []DispatchSkillIdentity
	Manifest         skillbundle.ExecutionManifest
}

type AttributionOutcome struct {
	SkillID     string
	RevisionID  string
	Eligibility EvidenceEligibility
	Reason      AttributionReason
}

// AttributionReport is diagnostic only. Record never returns an error, so an
// optional evolution recorder cannot change terminal task completion.
type AttributionReport struct {
	Eligibility EvidenceEligibility
	Reason      AttributionReason
	Recorded    int
	Outcomes    []AttributionOutcome
}

type attributionRevisionMatch struct {
	WorkspaceID pgtype.UUID
	SkillID     pgtype.UUID
	RevisionID  pgtype.UUID
	Source      string
	BundleHash  Digest
}

type attributionRevision struct {
	ID             pgtype.UUID
	WorkspaceID    pgtype.UUID
	SkillID        pgtype.UUID
	Source         string
	BundleHash     Digest
	OwnershipClass OwnershipClass
}

type attributionRepository interface {
	resolveAttributionRevisions(context.Context, attributionRevisionMatch) ([]attributionRevision, error)
	recordAttributionBatch(context.Context, []TaskAttributionInput) error
}

type AttributionRecorder struct {
	repository attributionRepository
}

func NewAttributionRecorder(store *Store) *AttributionRecorder {
	return newAttributionRecorder(store)
}

func newAttributionRecorder(repository attributionRepository) *AttributionRecorder {
	return &AttributionRecorder{repository: repository}
}

// Record resolves the complete manifest before writing any eligible row. Any
// missing proof returns an explicit content-free ineligibility reason; storage
// failures are also contained here rather than propagated into task completion.
func (r *AttributionRecorder) Record(ctx context.Context, input AttributionInput) AttributionReport {
	if !input.CapabilityProven {
		return ineligibleAttributionReport(AttributionReasonCapabilityNotProven)
	}
	if r == nil || r.repository == nil || !validUUID(input.WorkspaceID) || !validUUID(input.TaskID) || !validUUID(input.RuntimeID) {
		return ineligibleAttributionReport(AttributionReasonInvalidInput)
	}

	manifest, err := renormalizeExecutionManifest(input.Manifest)
	if err != nil {
		return ineligibleAttributionReport(AttributionReasonInvalidInput)
	}
	if reason := validateDispatchManifest(input.DispatchedSkills, manifest); reason != "" {
		return ineligibleAttributionReport(reason)
	}

	digest := digestExecutionManifest(manifest)
	records := append([]skillbundle.ExecutionRecord(nil), manifest.Skills...)
	sort.Slice(records, func(i, j int) bool {
		if records[i].Source == records[j].Source {
			return records[i].SkillID < records[j].SkillID
		}
		return records[i].Source < records[j].Source
	})

	inputs := make([]TaskAttributionInput, 0, len(records))
	outcomes := make([]AttributionOutcome, 0, len(records))
	for _, record := range records {
		inputRow, outcome, reason := r.resolveRecord(ctx, input, manifest.Version, digest, record)
		if reason != "" {
			return AttributionReport{
				Eligibility: EvidenceEligibilityIneligible,
				Reason:      reason,
				Outcomes:    []AttributionOutcome{outcome},
			}
		}
		inputs = append(inputs, inputRow)
		outcomes = append(outcomes, outcome)
	}

	if err := r.repository.recordAttributionBatch(ctx, inputs); err != nil {
		reason := AttributionReasonPersistenceUnavailable
		switch {
		case errors.Is(err, ErrPersistenceConflict):
			reason = AttributionReasonPersistenceConflict
		case errors.Is(err, ErrPersistenceNotFound):
			reason = AttributionReasonTaskDispatchNotProven
		}
		return ineligibleAttributionReport(reason)
	}
	return AttributionReport{
		Eligibility: EvidenceEligibilityEligible,
		Reason:      AttributionReasonExactRevisionMatch,
		Recorded:    len(inputs),
		Outcomes:    outcomes,
	}
}

func (r *AttributionRecorder) resolveRecord(
	ctx context.Context,
	input AttributionInput,
	manifestVersion int,
	manifestDigest Digest,
	record skillbundle.ExecutionRecord,
) (TaskAttributionInput, AttributionOutcome, AttributionReason) {
	outcome := AttributionOutcome{
		SkillID:     record.SkillID,
		Eligibility: EvidenceEligibilityIneligible,
	}
	if !boundedToken(record.Source, 80) {
		outcome.Reason = AttributionReasonRevisionNotProven
		return TaskAttributionInput{}, outcome, outcome.Reason
	}
	skillID, err := parseUUID(record.SkillID)
	if err != nil {
		outcome.Reason = AttributionReasonRevisionNotProven
		return TaskAttributionInput{}, outcome, outcome.Reason
	}
	var revisionID pgtype.UUID
	if record.RevisionID != "" {
		revisionID, err = parseUUID(record.RevisionID)
		if err != nil {
			outcome.Reason = AttributionReasonRevisionNotProven
			return TaskAttributionInput{}, outcome, outcome.Reason
		}
	}
	match := attributionRevisionMatch{
		WorkspaceID: input.WorkspaceID,
		SkillID:     skillID,
		RevisionID:  revisionID,
		Source:      record.Source,
		BundleHash:  Digest(record.BundleHash),
	}
	revisions, err := r.repository.resolveAttributionRevisions(ctx, match)
	if err != nil {
		outcome.Reason = AttributionReasonPersistenceUnavailable
		return TaskAttributionInput{}, outcome, outcome.Reason
	}
	if len(revisions) == 0 {
		outcome.Reason = AttributionReasonRevisionNotFound
		return TaskAttributionInput{}, outcome, outcome.Reason
	}
	if len(revisions) != 1 {
		outcome.Reason = AttributionReasonRevisionAmbiguous
		return TaskAttributionInput{}, outcome, outcome.Reason
	}

	revision := revisions[0]
	outcome.RevisionID = attributionUUIDString(revision.ID)
	switch {
	case !validUUID(revision.ID) || !validUUID(revision.WorkspaceID):
		outcome.Reason = AttributionReasonRevisionNotProven
	case revision.WorkspaceID != input.WorkspaceID:
		outcome.Reason = AttributionReasonRevisionNotFound
	case revision.SkillID != skillID:
		outcome.Reason = AttributionReasonRevisionSkillMismatch
	case revisionID.Valid && revision.ID != revisionID:
		outcome.Reason = AttributionReasonRevisionNotProven
	case revision.Source != record.Source:
		outcome.Reason = AttributionReasonRevisionSourceMismatch
	case revision.BundleHash != Digest(record.BundleHash):
		outcome.Reason = AttributionReasonRevisionHashMismatch
	case revision.OwnershipClass != OwnershipWorkspace:
		outcome.Reason = AttributionReasonOwnershipIneligible
	}
	if outcome.Reason != "" {
		return TaskAttributionInput{}, outcome, outcome.Reason
	}

	outcome.Eligibility = EvidenceEligibilityEligible
	outcome.Reason = AttributionReasonExactRevisionMatch
	return TaskAttributionInput{
		WorkspaceID:     input.WorkspaceID,
		TaskID:          input.TaskID,
		RuntimeID:       input.RuntimeID,
		SkillID:         skillID,
		RevisionID:      revision.ID,
		ManifestVersion: manifestVersion,
		Source:          record.Source,
		BundleHash:      Digest(record.BundleHash),
		ManifestDigest:  manifestDigest,
		Eligibility:     EvidenceEligibilityEligible,
		Reason:          string(AttributionReasonExactRevisionMatch),
	}, outcome, ""
}

func renormalizeExecutionManifest(manifest skillbundle.ExecutionManifest) (skillbundle.ExecutionManifest, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return skillbundle.ExecutionManifest{}, err
	}
	return skillbundle.NormalizeExecutionManifest(raw)
}

func validateDispatchManifest(dispatched []DispatchSkillIdentity, manifest skillbundle.ExecutionManifest) AttributionReason {
	if len(dispatched) == 0 {
		return AttributionReasonInvalidDispatch
	}
	if len(manifest.Skills) < len(dispatched) {
		return AttributionReasonIncompleteManifest
	}
	if len(manifest.Skills) > len(dispatched) {
		return AttributionReasonUnexpectedManifestSkill
	}

	expected := make(map[string]struct{}, len(dispatched))
	for _, skill := range dispatched {
		if !validAttributionIdentity(skill.Source) || !validAttributionIdentity(skill.SkillID) {
			return AttributionReasonInvalidDispatch
		}
		identity := skill.Source + "\x00" + skill.SkillID
		if _, duplicate := expected[identity]; duplicate {
			return AttributionReasonInvalidDispatch
		}
		expected[identity] = struct{}{}
	}
	for _, skill := range manifest.Skills {
		if _, found := expected[skill.Source+"\x00"+skill.SkillID]; !found {
			return AttributionReasonDispatchMismatch
		}
	}
	return ""
}

func digestExecutionManifest(manifest skillbundle.ExecutionManifest) Digest {
	canonical := append([]skillbundle.ExecutionRecord(nil), manifest.Skills...)
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].Source == canonical[j].Source {
			return canonical[i].SkillID < canonical[j].SkillID
		}
		return canonical[i].Source < canonical[j].Source
	})
	raw, _ := json.Marshal(skillbundle.ExecutionManifest{Version: manifest.Version, Skills: canonical})
	sum := sha256.Sum256(append([]byte("skill-execution-attribution-v1\x00"), raw...))
	return Digest("sha256:" + hex.EncodeToString(sum[:]))
}

func ineligibleAttributionReport(reason AttributionReason) AttributionReport {
	return AttributionReport{Eligibility: EvidenceEligibilityIneligible, Reason: reason}
}

func validAttributionIdentity(value string) bool {
	if value == "" || strings.TrimSpace(value) == "" || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func attributionUUIDString(value pgtype.UUID) string {
	if !validUUID(value) {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func (s *Store) resolveAttributionRevisions(ctx context.Context, match attributionRevisionMatch) ([]attributionRevision, error) {
	if s == nil || s.queries == nil || !validUUID(match.WorkspaceID) || !validUUID(match.SkillID) ||
		!validOptionalUUID(match.RevisionID) || !boundedToken(match.Source, 80) || !match.BundleHash.Valid() {
		return nil, ErrPersistenceInvalidInput
	}

	var (
		row db.SkillEvolutionRevision
		err error
	)
	if match.RevisionID.Valid {
		row, err = s.queries.GetSkillEvolutionRevision(ctx, db.GetSkillEvolutionRevisionParams{
			WorkspaceID: match.WorkspaceID,
			ID:          match.RevisionID,
		})
	} else {
		row, err = s.queries.GetSkillEvolutionRevisionByHash(ctx, db.GetSkillEvolutionRevisionByHashParams{
			WorkspaceID: match.WorkspaceID,
			SkillID:     match.SkillID,
			BundleHash:  string(match.BundleHash),
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, persistenceError(err)
	}
	return []attributionRevision{{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		SkillID:        row.SkillID,
		Source:         row.Source,
		BundleHash:     Digest(row.BundleHash),
		OwnershipClass: OwnershipClass(row.OwnershipClass),
	}}, nil
}

// recordAttributionBatch makes an all-or-nothing eligibility decision durable.
// It delegates each row to RecordTaskAttribution so the existing server-side
// task/runtime/workspace/revision joins and idempotency checks stay authoritative.
func (s *Store) recordAttributionBatch(ctx context.Context, inputs []TaskAttributionInput) error {
	if s == nil || s.queries == nil || s.txStarter == nil || len(inputs) == 0 {
		return ErrPersistenceTransactionsRequired
	}
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txStore := &Store{queries: s.queries.WithTx(tx)}
	for _, input := range inputs {
		if _, err := txStore.RecordTaskAttribution(ctx, input); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
