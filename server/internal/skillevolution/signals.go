package skillevolution

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const MaxResolvedEvidenceBytes = 2 << 20

var (
	ErrSignalSourceInvalid = errors.New("invalid skill evolution signal source")
	ErrSignalSourceDrift   = errors.New("skill evolution signal changed after discovery")
)

// SignalQuery is the bounded, content-free discovery request shared by all
// source-owned adapters. ActorID is optional because scheduled observation is
// authorized by the adapter at composition time rather than impersonating a
// human reviewer.
type SignalQuery struct {
	WorkspaceID pgtype.UUID
	SkillID     pgtype.UUID
	ActorID     pgtype.UUID
	Limit       int
}

// ResolvedEvidence exists only for one generation call. Persistence receives
// Ref, never Payload.
type ResolvedEvidence struct {
	Ref     EvidenceRef
	Payload []byte
}

// SignalSource adapts one source-owned two-stage reader. Implementations must
// repeat authorization and digest validation in Load; List must be content-free.
type SignalSource interface {
	Kind() EvidenceKind
	List(context.Context, SignalQuery) ([]EvidenceRef, error)
	Load(context.Context, SignalQuery, EvidenceRef) (ResolvedEvidence, error)
}

type SignalListFunc func(context.Context, SignalQuery) ([]EvidenceRef, error)
type SignalLoadFunc func(context.Context, SignalQuery, EvidenceRef) (ResolvedEvidence, error)

// SignalAdapter lets Task, Wiki, Room, and Twin keep their native reference
// types and expose only narrow conversion closures at the composition root.
type SignalAdapter struct {
	kind EvidenceKind
	list SignalListFunc
	load SignalLoadFunc
}

func NewSignalAdapter(kind EvidenceKind, list SignalListFunc, load SignalLoadFunc) *SignalAdapter {
	return &SignalAdapter{kind: kind, list: list, load: load}
}

func (a *SignalAdapter) Kind() EvidenceKind {
	if a == nil {
		return ""
	}
	return a.kind
}

func (a *SignalAdapter) List(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
	if a == nil || !a.kind.Valid() || a.list == nil || !validSignalQuery(query) {
		return nil, ErrSignalSourceInvalid
	}
	refs, err := a.list(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(refs) > query.Limit {
		return nil, ErrSignalSourceInvalid
	}
	workspaceID := uuid.UUID(query.WorkspaceID.Bytes).String()
	for _, ref := range refs {
		if ref.Kind != a.kind || ref.WorkspaceID != workspaceID || ref.Validate() != nil {
			return nil, ErrSignalSourceInvalid
		}
	}
	return append([]EvidenceRef(nil), refs...), nil
}

func (a *SignalAdapter) Load(ctx context.Context, query SignalQuery, expected EvidenceRef) (ResolvedEvidence, error) {
	if a == nil || !a.kind.Valid() || a.load == nil || !validSignalQuery(query) ||
		expected.Kind != a.kind || expected.Validate() != nil {
		return ResolvedEvidence{}, ErrSignalSourceInvalid
	}
	resolved, err := a.load(ctx, query, expected)
	if err != nil {
		return ResolvedEvidence{}, err
	}
	if !sameEvidenceRef(resolved.Ref, expected) || len(resolved.Payload) > MaxResolvedEvidenceBytes {
		return ResolvedEvidence{}, ErrSignalSourceDrift
	}
	resolved.Payload = append([]byte(nil), resolved.Payload...)
	return resolved, nil
}

type signalSet struct {
	sources map[EvidenceKind]SignalSource
	ordered []SignalSource
}

func newSignalSet(sources []SignalSource) (*signalSet, error) {
	set := &signalSet{sources: make(map[EvidenceKind]SignalSource, len(sources))}
	for _, source := range sources {
		if source == nil || !source.Kind().Valid() {
			return nil, ErrSignalSourceInvalid
		}
		if _, duplicate := set.sources[source.Kind()]; duplicate {
			return nil, ErrSignalSourceInvalid
		}
		set.sources[source.Kind()] = source
		set.ordered = append(set.ordered, source)
	}
	sort.Slice(set.ordered, func(i, j int) bool { return set.ordered[i].Kind() < set.ordered[j].Kind() })
	return set, nil
}

func (s *signalSet) discover(ctx context.Context, query SignalQuery) ([]EvidenceRef, error) {
	if s == nil || !validSignalQuery(query) {
		return nil, ErrSignalSourceInvalid
	}
	workspaceID := uuid.UUID(query.WorkspaceID.Bytes).String()
	skillID := uuid.UUID(query.SkillID.Bytes).String()
	refs := make([]EvidenceRef, 0, query.Limit)
	seen := make(map[string]struct{}, query.Limit)
	for _, source := range s.ordered {
		pageQuery := query
		page, err := source.List(ctx, pageQuery)
		if err != nil {
			return nil, err
		}
		for _, ref := range page {
			if ref.WorkspaceID != workspaceID || ref.Eligibility != EvidenceEligibilityEligible ||
				(ref.TargetSkillID != "" && ref.TargetSkillID != skillID) {
				continue
			}
			identity := string(ref.Kind) + "\x00" + ref.SourceID + "\x00" + ref.SourceRevisionID
			if _, duplicate := seen[identity]; duplicate {
				continue
			}
			seen[identity] = struct{}{}
			refs = append(refs, ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if !refs[i].ObservedAt.Equal(refs[j].ObservedAt) {
			return refs[i].ObservedAt.After(refs[j].ObservedAt)
		}
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		if refs[i].SourceID != refs[j].SourceID {
			return refs[i].SourceID < refs[j].SourceID
		}
		return refs[i].SourceRevisionID < refs[j].SourceRevisionID
	})
	if len(refs) > query.Limit {
		refs = refs[:query.Limit]
	}
	return refs, nil
}

func (s *signalSet) resolve(ctx context.Context, query SignalQuery, refs []EvidenceRef) ([]ResolvedEvidence, error) {
	resolved := make([]ResolvedEvidence, 0, len(refs))
	totalBytes := 0
	for _, ref := range refs {
		source := s.sources[ref.Kind]
		if source == nil {
			return nil, ErrSignalSourceInvalid
		}
		evidence, err := source.Load(ctx, query, ref)
		if err != nil {
			return nil, err
		}
		totalBytes += len(evidence.Payload)
		if totalBytes > MaxResolvedEvidenceBytes {
			return nil, ErrSignalSourceInvalid
		}
		resolved = append(resolved, evidence)
	}
	return resolved, nil
}

func validSignalQuery(query SignalQuery) bool {
	return validUUID(query.WorkspaceID) && validUUID(query.SkillID) && validOptionalUUID(query.ActorID) &&
		query.Limit > 0 && query.Limit <= MaxEvidenceRefs
}

func sameEvidenceRef(left, right EvidenceRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.Kind == right.Kind && left.SourceID == right.SourceID &&
		left.SourceRevisionID == right.SourceRevisionID && left.TargetSkillID == right.TargetSkillID &&
		left.SourceState == right.SourceState && left.Digest == right.Digest && left.Eligibility == right.Eligibility &&
		left.ObservedAt.Equal(right.ObservedAt)
}
