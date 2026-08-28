package skillbundle

import (
	"encoding/json"
	"testing"
)

func resolvedExecutionSkill(source, id, content string) ResolvedSkill {
	bundle := Skill{Source: source, ID: id, Name: id, Content: content}
	return ResolvedSkill{Bundle: bundle, DeclaredHash: BuildManifest(bundle).Hash}
}

func TestBuildExecutionManifestUsesResolvedBundleHash(t *testing.T) {
	resolved := resolvedExecutionSkill(SourceWorkspace, "skill-1", "current content")
	manifest, err := BuildExecutionManifest([]ResolvedSkill{resolved})
	if err != nil {
		t.Fatalf("BuildExecutionManifest: %v", err)
	}
	if manifest.Version != ExecutionManifestVersion || len(manifest.Skills) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if got := manifest.Skills[0].BundleHash; got != resolved.DeclaredHash {
		t.Fatalf("bundle hash = %q, want resolved hash %q", got, resolved.DeclaredHash)
	}
}

func TestBuildExecutionManifestIsAllOrNothing(t *testing.T) {
	valid := resolvedExecutionSkill(SourceWorkspace, "skill-1", "content")
	duplicate := resolvedExecutionSkill(SourceWorkspace, "skill-1", "other content")
	mismatch := resolvedExecutionSkill(SourceWorkspace, "skill-2", "content")
	if mismatch.DeclaredHash[len(mismatch.DeclaredHash)-1] == 'a' {
		mismatch.DeclaredHash = mismatch.DeclaredHash[:len(mismatch.DeclaredHash)-1] + "b"
	} else {
		mismatch.DeclaredHash = mismatch.DeclaredHash[:len(mismatch.DeclaredHash)-1] + "a"
	}

	for _, tc := range []struct {
		name   string
		skills []ResolvedSkill
	}{
		{name: "empty set"},
		{name: "empty source", skills: []ResolvedSkill{{Bundle: Skill{ID: "skill-1"}, DeclaredHash: valid.DeclaredHash}}},
		{name: "empty id", skills: []ResolvedSkill{{Bundle: Skill{Source: SourceWorkspace}, DeclaredHash: valid.DeclaredHash}}},
		{name: "duplicate identity", skills: []ResolvedSkill{valid, duplicate}},
		{name: "declared hash mismatch", skills: []ResolvedSkill{valid, mismatch}},
		{name: "bad declared hash", skills: []ResolvedSkill{valid, {Bundle: Skill{Source: SourceWorkspace, ID: "skill-2"}, DeclaredHash: "nope"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if manifest, err := BuildExecutionManifest(tc.skills); err == nil {
				t.Fatalf("BuildExecutionManifest succeeded with %+v", manifest)
			}
		})
	}
}

func TestNormalizeExecutionManifestCompatibilityMatrix(t *testing.T) {
	hash := BuildManifest(Skill{Source: SourceWorkspace, ID: "skill-1", Name: "skill-1", Content: "content"}).Hash
	valid := `{"version":1,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"` + hash + `"}],"future_field":true}`
	for _, tc := range []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "valid with unknown field", raw: valid, valid: true},
		{name: "absent"},
		{name: "malformed shape", raw: `[]`},
		{name: "unknown version", raw: `{"version":2,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"` + hash + `"}]}`},
		{name: "empty skills", raw: `{"version":1,"skills":[]}`},
		{name: "incomplete item", raw: `{"version":1,"skills":[{"source":"workspace","skill_id":"","bundle_hash":"` + hash + `"}]}`},
		{name: "duplicate identity", raw: `{"version":1,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"` + hash + `"},{"source":"workspace","skill_id":"skill-1","bundle_hash":"` + hash + `"}]}`},
		{name: "bad hash", raw: `{"version":1,"skills":[{"source":"workspace","skill_id":"skill-1","bundle_hash":"bad"}]}`},
		{name: "nul identity", raw: `{"version":1,"skills":[{"source":"workspace\u0000","skill_id":"skill-1","bundle_hash":"` + hash + `"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := NormalizeExecutionManifest(json.RawMessage(tc.raw))
			if tc.valid && err != nil {
				t.Fatalf("NormalizeExecutionManifest: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("NormalizeExecutionManifest succeeded with %+v", manifest)
			}
		})
	}
}
