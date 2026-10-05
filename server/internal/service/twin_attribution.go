package service

import (
	"fmt"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// TwinOneOffUsePolicyOverride is the optional task-level policy snapshot. It
// has highest precedence and is immutable for the claimed task; enabled may
// pin a signed version while off/preview never inject execution bytes.
type TwinOneOffUsePolicyOverride struct {
	State     TwinUsePolicyState
	VersionID string
}

// TwinBriefingClaimInput is the resolved, tenant-safe task identity passed to
// the Twin execution boundary. Request and Tags are bounded in-memory matching
// inputs and must not be persisted or sent to the daemon.
type TwinBriefingClaimInput struct {
	TaskID               string
	WorkspaceID          string
	AgentID              string
	ProjectID            string
	IssueID              string
	RunID                string
	Request              string
	Tags                 []string
	OneOffPolicy         *TwinOneOffUsePolicyOverride
	SupportsTwinBriefing bool
}

type TwinBriefingClaimResolution struct {
	Compiled TwinCompiledBriefing
}

// TwinClaimAttribution is the exact privacy-safe execution record that must be
// committed atomically with the task token before briefing bytes are released.
// It intentionally contains no raw Wiki evidence, task request, or local
// artifact identity.
type TwinClaimAttribution struct {
	Briefing             string
	VersionID            string
	BriefingDigest       string
	SelectedAssertionIDs []string
	CitationIDs          []string
	PolicyState          string
	PolicyScope          string
	PolicyScopeID        string
	PolicyBindingID      string
	CompilerVersion      string
	AuthorityOrder       []string
	ByteCount            int
	TokenCount           int
}

func twinTaskAttributionInput(
	task db.AgentTaskQueue,
	token db.CreateTaskTokenParams,
	attribution TwinClaimAttribution,
) (TwinTaskAttributionInput, error) {
	versionID, err := util.ParseUUID(attribution.VersionID)
	if err != nil {
		return TwinTaskAttributionInput{}, fmt.Errorf("finalize task claim: invalid Twin version id: %w", err)
	}
	policyScopeID, err := util.ParseUUID(attribution.PolicyScopeID)
	if err != nil {
		return TwinTaskAttributionInput{}, fmt.Errorf("finalize task claim: invalid Twin policy scope id: %w", err)
	}
	if token.TaskID != task.ID || token.AgentID != task.AgentID {
		return TwinTaskAttributionInput{}, fmt.Errorf("finalize task claim: Twin attribution task identity mismatch")
	}
	if !token.WorkspaceID.Valid {
		return TwinTaskAttributionInput{}, fmt.Errorf("finalize task claim: invalid Twin workspace id")
	}
	return TwinTaskAttributionInput{
		WorkspaceID:      token.WorkspaceID,
		TaskID:           task.ID,
		AgentID:          task.AgentID,
		RuntimeID:        task.RuntimeID,
		TaskDispatchedAt: task.DispatchedAt,
		TwinVersionID:    versionID,
		Briefing:         attribution.Briefing,
		BriefingDigest:   attribution.BriefingDigest,
		AssertionIDs:     append([]string(nil), attribution.SelectedAssertionIDs...),
		CitationKeys:     append([]string(nil), attribution.CitationIDs...),
		PolicyScopeType:  attribution.PolicyScope,
		PolicyScopeID:    policyScopeID,
		PolicyState:      attribution.PolicyState,
		CompilerVersion:  attribution.CompilerVersion,
	}, nil
}
