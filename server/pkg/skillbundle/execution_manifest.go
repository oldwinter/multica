package skillbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const ExecutionManifestVersion = 1

var (
	ErrInvalidExecutionManifest   = errors.New("invalid skill execution manifest")
	ErrDuplicateExecutionIdentity = errors.New("duplicate skill execution identity")
)

// ExecutionManifest is the normalized, all-or-nothing resolved-bundle claim
// carried by a successful daemon task completion.
type ExecutionManifest struct {
	Version int               `json:"version"`
	Skills  []ExecutionRecord `json:"skills"`
}

type ExecutionRecord struct {
	Source     string `json:"source"`
	SkillID    string `json:"skill_id"`
	BundleHash string `json:"bundle_hash"`
	RevisionID string `json:"revision_id,omitempty"`
}

// ResolvedSkill is the daemon-resolved Multica bundle and the hash that
// accompanied its content. It does not claim that a provider successfully
// materialized the bundle on disk.
type ResolvedSkill struct {
	Bundle       Skill
	DeclaredHash string
}

// BuildExecutionManifest builds an all-or-nothing claim from the exact
// bundles resolved by the daemon. Callers omit the whole manifest on error.
func BuildExecutionManifest(skills []ResolvedSkill) (ExecutionManifest, error) {
	if len(skills) == 0 {
		return ExecutionManifest{}, ErrInvalidExecutionManifest
	}

	manifest := ExecutionManifest{
		Version: ExecutionManifestVersion,
		Skills:  make([]ExecutionRecord, 0, len(skills)),
	}
	seen := make(map[string]struct{}, len(skills))
	for _, resolved := range skills {
		if !validExecutionIdentity(resolved.Bundle.Source) || !validExecutionIdentity(resolved.Bundle.ID) {
			return ExecutionManifest{}, ErrInvalidExecutionManifest
		}
		identity := resolved.Bundle.Source + "\x00" + resolved.Bundle.ID
		if _, exists := seen[identity]; exists {
			return ExecutionManifest{}, ErrDuplicateExecutionIdentity
		}
		seen[identity] = struct{}{}

		if !validExecutionDigest(resolved.DeclaredHash) {
			return ExecutionManifest{}, ErrInvalidExecutionManifest
		}
		recomputed, err := BuildValidatedManifest(resolved.Bundle)
		if err != nil || recomputed.Hash != resolved.DeclaredHash {
			return ExecutionManifest{}, ErrInvalidExecutionManifest
		}
		manifest.Skills = append(manifest.Skills, ExecutionRecord{
			Source:     resolved.Bundle.Source,
			SkillID:    resolved.Bundle.ID,
			BundleHash: resolved.DeclaredHash,
		})
	}
	return manifest, nil
}

// NormalizeExecutionManifest validates optional completion input without
// trusting it. Unknown JSON fields are ignored for forward compatibility; the
// returned value contains only the normalized v1 contract.
func NormalizeExecutionManifest(raw json.RawMessage) (ExecutionManifest, error) {
	if len(raw) == 0 {
		return ExecutionManifest{}, ErrInvalidExecutionManifest
	}
	var manifest ExecutionManifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Version != ExecutionManifestVersion || len(manifest.Skills) == 0 {
		return ExecutionManifest{}, ErrInvalidExecutionManifest
	}

	seen := make(map[string]struct{}, len(manifest.Skills))
	for _, record := range manifest.Skills {
		if !validExecutionIdentity(record.Source) || !validExecutionIdentity(record.SkillID) ||
			(record.RevisionID != "" && !validExecutionIdentity(record.RevisionID)) ||
			!validExecutionDigest(record.BundleHash) {
			return ExecutionManifest{}, ErrInvalidExecutionManifest
		}

		identity := record.Source + "\x00" + record.SkillID
		if _, exists := seen[identity]; exists {
			return ExecutionManifest{}, ErrDuplicateExecutionIdentity
		}
		seen[identity] = struct{}{}
	}
	return manifest, nil
}

func validExecutionDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	raw := strings.TrimPrefix(value, "sha256:")
	if raw != strings.ToLower(raw) {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func validExecutionIdentity(value string) bool {
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
