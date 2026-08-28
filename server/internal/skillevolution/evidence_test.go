package skillevolution

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCanonicalEvidenceDigestIsOrderIndependentAndBounded(t *testing.T) {
	left, err := CanonicalEvidenceDigest("task_review", []DigestPart{{Key: "task_id", Value: "task-1"}, {Key: "note", Value: "fix timeout"}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := CanonicalEvidenceDigest("task_review", []DigestPart{{Key: "note", Value: "fix timeout"}, {Key: "task_id", Value: "task-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if left != right || !left.Valid() {
		t.Fatalf("canonical digests differ or are invalid: %q != %q", left, right)
	}
	if got, want := left, Digest("sha256:c8de8077a54e11f4f5a77ace8cad41d1dff9f2f64671ce779be96bf62a0d7697"); got != want {
		t.Fatalf("canonical evidence digest = %q, want %q", got, want)
	}
	if _, err := CanonicalEvidenceDigest("task_review", []DigestPart{{Key: "note", Value: "a"}, {Key: "note", Value: "b"}}); !errors.Is(err, ErrInvalidDigestInput) {
		t.Fatalf("duplicate part error = %v", err)
	}
	if _, err := CanonicalEvidenceDigest("task_review", []DigestPart{{Key: "note", Value: strings.Repeat("x", MaxDigestPartBytes+1)}}); !errors.Is(err, ErrInvalidDigestInput) {
		t.Fatalf("oversized part error = %v", err)
	}
}

func TestEvidenceRefValidationAndContentFreeShape(t *testing.T) {
	digest, err := CanonicalEvidenceDigest("task_review", []DigestPart{{Key: "task_id", Value: "task-1"}})
	if err != nil {
		t.Fatal(err)
	}
	ref := EvidenceRef{
		WorkspaceID: "workspace-1", Kind: EvidenceKindTaskReview, SourceID: "review-1",
		SourceRevisionID: "revision-1", TargetSkillID: "skill-1", SourceState: "accepted",
		Digest: digest, Eligibility: EvidenceEligibilityEligible, ObservedAt: time.Unix(1, 0).UTC(),
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("valid ref rejected: %v", err)
	}
	for _, field := range []string{"Content", "Body", "Prompt", "Output", "Note", "Path", "Citation"} {
		if _, ok := reflect.TypeOf(ref).FieldByName(field); ok {
			t.Fatalf("EvidenceRef must be content-free; found field %q", field)
		}
	}

	bad := ref
	bad.SourceID = "/private/task-1"
	if err := bad.Validate(); !errors.Is(err, ErrInvalidEvidenceRef) {
		t.Fatalf("path-like source identity error = %v", err)
	}
	bad = ref
	bad.Digest = "sha256:ABC"
	if err := bad.Validate(); !errors.Is(err, ErrInvalidEvidenceRef) {
		t.Fatalf("bad digest ref error = %v", err)
	}
}

func TestValidateEvidenceRefsRejectsDuplicateAndBounds(t *testing.T) {
	digest, _ := CanonicalEvidenceDigest("room", []DigestPart{{Key: "id", Value: "one"}})
	ref := EvidenceRef{
		WorkspaceID: "workspace-1", Kind: EvidenceKindRoomOutcome, SourceID: "room-1",
		SourceState: "accepted", Digest: digest, Eligibility: EvidenceEligibilityEligible,
		ObservedAt: time.Unix(1, 0).UTC(),
	}
	if err := ValidateEvidenceRefs([]EvidenceRef{ref}, 1); err != nil {
		t.Fatalf("valid refs rejected: %v", err)
	}
	if err := ValidateEvidenceRefs([]EvidenceRef{ref, ref}, 2); !errors.Is(err, ErrDuplicateEvidenceRef) {
		t.Fatalf("duplicate refs error = %v", err)
	}
	if err := ValidateEvidenceRefs([]EvidenceRef{ref}, 0); !errors.Is(err, ErrTooManyEvidenceRefs) {
		t.Fatalf("over-limit refs error = %v", err)
	}
	otherWorkspace := ref
	otherWorkspace.SourceID = "room-2"
	otherWorkspace.WorkspaceID = "workspace-2"
	if err := ValidateEvidenceRefs([]EvidenceRef{ref, otherWorkspace}, 2); !errors.Is(err, ErrInvalidEvidenceRef) {
		t.Fatalf("mixed-workspace refs error = %v", err)
	}
	if err := ValidateEvidenceRefsForWorkspace("workspace-2", []EvidenceRef{ref}, 1); !errors.Is(err, ErrInvalidEvidenceRef) {
		t.Fatalf("unexpected-workspace ref error = %v", err)
	}
}
