package service

import (
	"errors"
	"strings"
	"testing"
)

func TestTwinExecutionKillSwitchBlocksActivationBeforeStoreAccess(t *testing.T) {
	execution := &TwinExecutionService{FeatureEnabled: false}
	if got := execution.KillSwitch(); got.Enabled || got.Reason != "disabled_by_operator" {
		t.Fatalf("kill switch = %#v", got)
	}
	_, err := execution.UpsertBinding(t.Context(), TwinBindingInput{State: "enabled"})
	if !errors.Is(err, ErrTwinExecutionDisabled) {
		t.Fatalf("enabled binding error = %v, want ErrTwinExecutionDisabled", err)
	}
}

func TestTwinDepositionEvidenceDigestIsCanonicalAndContentFree(t *testing.T) {
	left, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: "task-1", AttributionID: "attribution-1", TwinVersionID: "version-1",
		BriefingDigest:  twinExecutionTestDigest("briefing"),
		AssertionIDs:    []string{"assertion-b", "assertion-a"},
		CitationKeys:    []string{"issue:b", "issue:a"},
		PolicyScopeType: "issue", PolicyScopeID: "issue-1",
		FeedbackRating: "helped", CompletedAt: "2026-08-23T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("digest deposition evidence: %v", err)
	}
	right, err := twinDepositionEvidenceDigest(twinDepositionEvidence{
		TaskID: "task-1", AttributionID: "attribution-1", TwinVersionID: "version-1",
		BriefingDigest:  twinExecutionTestDigest("briefing"),
		AssertionIDs:    []string{"assertion-a", "assertion-b"},
		CitationKeys:    []string{"issue:a", "issue:b"},
		PolicyScopeType: "issue", PolicyScopeID: "issue-1",
		FeedbackRating: "helped", CompletedAt: "2026-08-23T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("digest reordered deposition evidence: %v", err)
	}
	if left != right || !strings.HasPrefix(left, "sha256:") || len(left) != len("sha256:")+64 {
		t.Fatalf("canonical digests = %q and %q", left, right)
	}
}

func TestDecodeTwinExecutionStringListFailsClosed(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`null`),
		[]byte(`{"not":"an-array"}`),
		[]byte(`["duplicate","duplicate"]`),
		[]byte(`[""]`),
	} {
		if _, ok := decodeTwinExecutionStringList(raw, 10, 100); ok {
			t.Fatalf("decoded malformed critical list %s", raw)
		}
	}
	values, ok := decodeTwinExecutionStringList([]byte(`["assertion:a","assertion:b"]`), 10, 100)
	if !ok || len(values) != 2 {
		t.Fatalf("valid critical list = %#v, %v", values, ok)
	}
}

func TestValidateTwinTaskUseSnapshotPinsSignedSchemaV2(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Pinned Twin run")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("create schema-v2 Twin proposal: %v", err)
	}
	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("sign schema-v2 Twin proposal: %v", err)
	}
	execution := NewTwinExecutionService(fixture.queries, true)

	canonical, err := execution.ValidateTaskUseSnapshot(fixture.ctx, fixture.workspaceID, TwinOneOffUsePolicyOverride{
		State: TwinUseEnabled, VersionID: signed.Version.ID.String(),
	})
	if err != nil {
		t.Fatalf("validate enabled task snapshot: %v", err)
	}
	if canonical.State != TwinUseEnabled || canonical.VersionID != signed.Version.ID.String() {
		t.Fatalf("canonical task snapshot = %#v", canonical)
	}
	if _, err := execution.ValidateTaskUseSnapshot(fixture.ctx, fixture.workspaceID, TwinOneOffUsePolicyOverride{
		State: TwinUseOff, VersionID: signed.Version.ID.String(),
	}); !errors.Is(err, ErrTwinExecutionInvalidInput) {
		t.Fatalf("off snapshot with version error = %v, want invalid input", err)
	}
	if _, err := execution.ValidateTaskUseSnapshot(fixture.ctx, fixture.workspaceID, TwinOneOffUsePolicyOverride{
		State: TwinUseEnabled,
	}); !errors.Is(err, ErrTwinExecutionInvalidInput) {
		t.Fatalf("enabled snapshot without version error = %v, want invalid input", err)
	}
	if _, err := NewTwinExecutionService(fixture.queries, false).ValidateTaskUseSnapshot(fixture.ctx, fixture.workspaceID, canonical); !errors.Is(err, ErrTwinExecutionDisabled) {
		t.Fatalf("disabled snapshot validation error = %v, want kill switch", err)
	}
}

func TestPrepareOneOffPreviewPinsCurrentSignedVersion(t *testing.T) {
	fixture := newTwinServiceFixture(t)
	wiki := fixture.acceptedWiki(t, "Preview current Twin run")
	proposal, err := fixture.service.EnsureProposal(fixture.ctx, fixture.workspaceID, wiki.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("create schema-v2 Twin proposal: %v", err)
	}
	signed, err := fixture.service.AcceptProposal(fixture.ctx, fixture.workspaceID, proposal.Proposal.ID, fixture.actorID)
	if err != nil {
		t.Fatalf("sign schema-v2 Twin proposal: %v", err)
	}
	execution := NewTwinExecutionService(fixture.queries, true)
	runID := fixture.workspaceID.String()

	preview, err := execution.PrepareOneOffPreview(fixture.ctx, fixture.workspaceID, runID, TwinUsePreview, "")
	if err != nil {
		t.Fatalf("prepare one-off preview: %v", err)
	}
	if preview.RunID != runID || preview.State != TwinUsePreview || preview.TwinVersionID != signed.Version.ID {
		t.Fatalf("one-off preview = %#v", preview)
	}

	off, err := execution.PrepareOneOffPreview(fixture.ctx, fixture.workspaceID, runID, TwinUseOff, "")
	if err != nil || off.State != TwinUseOff || off.TwinVersionID.Valid {
		t.Fatalf("one-off off preview = %#v, err = %v", off, err)
	}
	if _, err := execution.PrepareOneOffPreview(fixture.ctx, fixture.workspaceID, runID, TwinUseOff, signed.Version.ID.String()); !errors.Is(err, ErrTwinExecutionInvalidInput) {
		t.Fatalf("off preview with version error = %v, want invalid input", err)
	}
	if _, err := execution.PrepareOneOffPreview(fixture.ctx, fixture.workspaceID, "not-a-uuid", TwinUseEnabled, ""); !errors.Is(err, ErrTwinExecutionInvalidInput) {
		t.Fatalf("invalid preview run error = %v, want invalid input", err)
	}
}
