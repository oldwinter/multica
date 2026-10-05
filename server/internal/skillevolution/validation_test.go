package skillevolution

import (
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestClassifyOwnershipFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input OwnershipInput
		want  Ownership
	}{
		{name: "manual empty", input: OwnershipInput{}, want: workspaceOwnership()},
		{name: "manual object", input: OwnershipInput{Config: []byte(`{"custom":true}`)}, want: workspaceOwnership()},
		{name: "historical archive without origin", input: OwnershipInput{Config: []byte(`{}`)}, want: workspaceOwnership()},
		{name: "builtin", input: OwnershipInput{Builtin: true}, want: Ownership{Class: OwnershipBuiltin, Reason: OwnershipReasonBuiltin}},
		{name: "plugin wins over config", input: OwnershipInput{PluginInstallationPresent: true, Config: []byte(`{"origin":{"type":"github"}}`)}, want: Ownership{Class: OwnershipPlugin, Reason: OwnershipReasonPluginInstallation, ForkRequired: true}},
		{name: "github", input: OwnershipInput{Config: []byte(`{"origin":{"type":"github"}}`)}, want: Ownership{Class: OwnershipExternal, Reason: OwnershipReasonExternalOrigin, ForkRequired: true}},
		{name: "skills sh", input: OwnershipInput{Config: []byte(`{"origin":{"type":"skills_sh"}}`)}, want: Ownership{Class: OwnershipExternal, Reason: OwnershipReasonExternalOrigin, ForkRequired: true}},
		{name: "clawhub", input: OwnershipInput{Config: []byte(`{"origin":{"type":"clawhub"}}`)}, want: Ownership{Class: OwnershipExternal, Reason: OwnershipReasonExternalOrigin, ForkRequired: true}},
		{name: "runtime local", input: OwnershipInput{Config: []byte(`{"origin":{"type":"runtime_local"}}`)}, want: Ownership{Class: OwnershipRuntimeLocal, Reason: OwnershipReasonRuntimeLocalOrigin, ForkRequired: true}},
		{name: "unknown origin", input: OwnershipInput{Config: []byte(`{"origin":{"type":"future"}}`)}, want: unknownOwnership(OwnershipReasonUnknownOrigin)},
		{name: "missing origin type", input: OwnershipInput{Config: []byte(`{"origin":{"url":"x"}}`)}, want: unknownOwnership(OwnershipReasonUnknownOrigin)},
		{name: "origin not object", input: OwnershipInput{Config: []byte(`{"origin":"github"}`)}, want: unknownOwnership(OwnershipReasonUnknownOrigin)},
		{name: "invalid json", input: OwnershipInput{Config: []byte(`{`)}, want: unknownOwnership(OwnershipReasonMalformedConfig)},
		{name: "non-object config", input: OwnershipInput{Config: []byte(`null`)}, want: unknownOwnership(OwnershipReasonMalformedConfig)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyOwnership(tc.input); got != tc.want {
				t.Fatalf("ClassifyOwnership() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidateCandidateBundleStrictFrontmatter(t *testing.T) {
	valid := skillbundle.Skill{
		ID: "skill-1", Source: skillbundle.SourceWorkspace, Name: "deploy", Description: "Deploy safely",
		Content: "---\nname: deploy\ndescription: Deploy safely\nmetadata:\n  owner: platform\n---\nDo it.\n",
	}
	if _, err := ValidateCandidateBundle(valid); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}

	for _, tc := range []struct {
		name    string
		content string
		edit    func(*skillbundle.Skill)
		want    error
	}{
		{name: "missing", content: "Do it.", want: ErrFrontmatterMissing},
		{name: "unterminated", content: "---\nname: deploy\ndescription: Deploy safely\n", want: ErrFrontmatterMissing},
		{name: "yaml invalid", content: "---\nname: [\n---\n", want: ErrFrontmatterInvalid},
		{name: "mapping required", content: "---\n- name\n- description\n---\n", want: ErrFrontmatterInvalid},
		{name: "duplicate key", content: "---\nname: deploy\nname: other\ndescription: Deploy safely\n---\n", want: ErrFrontmatterInvalid},
		{name: "name must be string", content: "---\nname: 1\ndescription: Deploy safely\n---\n", want: ErrFrontmatterInvalid},
		{name: "description must be one line", content: "---\nname: deploy\ndescription: |\n  Deploy\n  safely\n---\n", want: ErrFrontmatterInvalid},
		{name: "description required", content: "---\nname: deploy\n---\n", want: ErrFrontmatterInvalid},
		{name: "metadata mismatch", content: "---\nname: other\ndescription: Deploy safely\n---\n", want: ErrFrontmatterMismatch},
		{name: "workspace only", content: valid.Content, edit: func(s *skillbundle.Skill) { s.Source = skillbundle.SourcePlugin }, want: ErrCandidateSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			candidate.Content = tc.content
			if tc.edit != nil {
				tc.edit(&candidate)
			}
			if _, err := ValidateCandidateBundle(candidate); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateCandidateBundle() error = %v, want %v", err, tc.want)
			}
		})
	}
}
