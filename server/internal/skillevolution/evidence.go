package skillevolution

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxEvidenceRefs          = 100
	MaxEvidenceIdentityBytes = 160
	MaxEvidenceStateBytes    = 64
	MaxDigestParts           = 64
	MaxDigestPartBytes       = 1 << 20
	MaxDigestInputBytes      = 2 << 20
)

var (
	ErrInvalidDigest        = errors.New("invalid canonical digest")
	ErrInvalidEvidenceRef   = errors.New("invalid evidence reference")
	ErrTooManyEvidenceRefs  = errors.New("too many evidence references")
	ErrDuplicateEvidenceRef = errors.New("duplicate evidence reference")
	ErrInvalidDigestInput   = errors.New("invalid canonical digest input")
)

type Digest string

func ParseDigest(value string) (Digest, error) {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return "", ErrInvalidDigest
	}
	raw := strings.TrimPrefix(value, "sha256:")
	if raw != strings.ToLower(raw) {
		return "", ErrInvalidDigest
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return "", ErrInvalidDigest
	}
	return Digest(value), nil
}

func (d Digest) Valid() bool {
	_, err := ParseDigest(string(d))
	return err == nil
}

type EvidenceKind string

const (
	EvidenceKindTaskReview      EvidenceKind = "task_review"
	EvidenceKindManualRerun     EvidenceKind = "manual_rerun"
	EvidenceKindWikiProposal    EvidenceKind = "wiki_proposal_review"
	EvidenceKindRoomOutcome     EvidenceKind = "room_accepted_outcome"
	EvidenceKindTwinRunFeedback EvidenceKind = "twin_run_feedback"
	EvidenceKindTwinDeposition  EvidenceKind = "twin_accepted_deposition"
)

func (k EvidenceKind) Valid() bool {
	switch k {
	case EvidenceKindTaskReview, EvidenceKindManualRerun, EvidenceKindWikiProposal,
		EvidenceKindRoomOutcome, EvidenceKindTwinRunFeedback, EvidenceKindTwinDeposition:
		return true
	default:
		return false
	}
}

type EvidenceEligibility string

const (
	EvidenceEligibilityEligible   EvidenceEligibility = "eligible"
	EvidenceEligibilityIneligible EvidenceEligibility = "ineligible"
)

func (e EvidenceEligibility) Valid() bool {
	return e == EvidenceEligibilityEligible || e == EvidenceEligibilityIneligible
}

// EvidenceRole records whether a content-free evidence identity authorized the
// candidate change or was independently selected for held-out replay.
type EvidenceRole string

const (
	EvidenceRoleSynthesis     EvidenceRole = "synthesis"
	EvidenceRoleHeldOutReplay EvidenceRole = "held_out_replay"
)

func (r EvidenceRole) Valid() bool {
	return r == EvidenceRoleSynthesis || r == EvidenceRoleHeldOutReplay
}

// EvidenceRef is deliberately content-free. Source-owned adapters must reload
// and authorize content using these identities, then revalidate Digest before
// returning any bounded generation evidence.
type EvidenceRef struct {
	WorkspaceID      string
	Kind             EvidenceKind
	SourceID         string
	SourceRevisionID string
	TargetSkillID    string
	SourceState      string
	Digest           Digest
	Eligibility      EvidenceEligibility
	ObservedAt       time.Time
}

func (r EvidenceRef) Validate() error {
	if !r.Kind.Valid() || !r.Eligibility.Valid() || r.ObservedAt.IsZero() ||
		!validEvidenceIdentity(r.WorkspaceID, false) || !validEvidenceIdentity(r.SourceID, false) ||
		!validEvidenceIdentity(r.SourceRevisionID, true) || !validEvidenceIdentity(r.TargetSkillID, true) ||
		!validEvidenceState(r.SourceState) || !r.Digest.Valid() {
		return ErrInvalidEvidenceRef
	}
	return nil
}

func ValidateEvidenceRefs(refs []EvidenceRef, limit int) error {
	expectedWorkspaceID := ""
	if len(refs) > 0 {
		expectedWorkspaceID = refs[0].WorkspaceID
	}
	return ValidateEvidenceRefsForWorkspace(expectedWorkspaceID, refs, limit)
}

func ValidateEvidenceRefsForWorkspace(workspaceID string, refs []EvidenceRef, limit int) error {
	if limit < 0 || limit > MaxEvidenceRefs || len(refs) > limit {
		return ErrTooManyEvidenceRefs
	}
	if len(refs) > 0 && !validEvidenceIdentity(workspaceID, false) {
		return ErrInvalidEvidenceRef
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if err := ref.Validate(); err != nil {
			return err
		}
		if ref.WorkspaceID != workspaceID {
			return ErrInvalidEvidenceRef
		}
		identity := string(ref.Kind) + "\x00" + ref.SourceID
		if _, ok := seen[identity]; ok {
			return ErrDuplicateEvidenceRef
		}
		seen[identity] = struct{}{}
	}
	return nil
}

// evidenceCaseLineageKey identifies one behavioral case independently of how
// many reviews or projections the source emits for it. Task-owned projections
// share the task lineage; Wiki proposals and Room recommendations retain their
// source-owned artifact identity.
func evidenceCaseLineageKey(ref EvidenceRef) (string, bool) {
	switch ref.Kind {
	case EvidenceKindTaskReview, EvidenceKindTwinRunFeedback, EvidenceKindTwinDeposition:
		if ref.SourceRevisionID == "" {
			return "", false
		}
		return "task\x00" + ref.SourceRevisionID, true
	case EvidenceKindManualRerun:
		if ref.SourceRevisionID == "" {
			return "", false
		}
		return "task\x00" + ref.SourceRevisionID, true
	case EvidenceKindWikiProposal:
		if ref.SourceID == "" {
			return "", false
		}
		return "wiki_proposal\x00" + ref.SourceID, true
	case EvidenceKindRoomOutcome:
		if ref.SourceID == "" || ref.SourceRevisionID == "" {
			return "", false
		}
		return "room_recommendation\x00" + ref.SourceRevisionID + "\x00" + ref.SourceID, true
	default:
		return "", false
	}
}

type DigestPart struct {
	Key   string
	Value string
}

// CanonicalEvidenceDigest hashes bounded source-owned fields without exposing
// their values. Parts are sorted by key and duplicate keys are rejected, so
// caller map/order choices cannot change the digest.
func CanonicalEvidenceDigest(namespace string, parts []DigestPart) (Digest, error) {
	if !validDigestKey(namespace) || len(parts) == 0 || len(parts) > MaxDigestParts {
		return "", ErrInvalidDigestInput
	}
	canonical := append([]DigestPart(nil), parts...)
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Key < canonical[j].Key })
	total := len(namespace)
	for i, part := range canonical {
		if !validDigestKey(part.Key) || !utf8.ValidString(part.Value) || strings.IndexByte(part.Value, 0) >= 0 ||
			len(part.Value) > MaxDigestPartBytes || (i > 0 && canonical[i-1].Key == part.Key) {
			return "", ErrInvalidDigestInput
		}
		total += len(part.Key) + len(part.Value)
		if total > MaxDigestInputBytes {
			return "", ErrInvalidDigestInput
		}
	}

	h := sha256.New()
	writeDigestValue(h, "skill-evolution-evidence-v1")
	writeDigestValue(h, namespace)
	for _, part := range canonical {
		writeDigestValue(h, part.Key)
		writeDigestValue(h, part.Value)
	}
	return Digest("sha256:" + hex.EncodeToString(h.Sum(nil))), nil
}

func validEvidenceIdentity(value string, optional bool) bool {
	if value == "" {
		return optional
	}
	if len(value) > MaxEvidenceIdentityBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', ':', '.':
			continue
		default:
			return false
		}
	}
	return true
}

func validEvidenceState(value string) bool {
	if value == "" || len(value) > MaxEvidenceStateBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validDigestKey(value string) bool {
	if value == "" || len(value) > MaxEvidenceStateBytes {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func writeDigestValue(h interface{ Write([]byte) (int, error) }, value string) {
	_, _ = fmt.Fprintf(h, "%d:%s\n", len(value), value)
}
