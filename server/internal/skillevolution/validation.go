package skillevolution

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

var (
	ErrCandidateIdentity   = errors.New("invalid candidate identity")
	ErrCandidateSource     = errors.New("candidate is not workspace-owned")
	ErrFrontmatterMissing  = errors.New("candidate frontmatter is missing")
	ErrFrontmatterInvalid  = errors.New("candidate frontmatter is invalid")
	ErrFrontmatterMismatch = errors.New("candidate frontmatter does not match bundle metadata")
)

type OwnershipClass string

const (
	OwnershipWorkspace    OwnershipClass = "workspace"
	OwnershipPlugin       OwnershipClass = "plugin"
	OwnershipExternal     OwnershipClass = "external"
	OwnershipRuntimeLocal OwnershipClass = "runtime_local"
	OwnershipBuiltin      OwnershipClass = "builtin"
	OwnershipUnknown      OwnershipClass = "unknown"
)

type OwnershipReason string

const (
	OwnershipReasonManual             OwnershipReason = "manual_or_unattributed"
	OwnershipReasonPluginInstallation OwnershipReason = "plugin_installation"
	OwnershipReasonExternalOrigin     OwnershipReason = "external_origin"
	OwnershipReasonRuntimeLocalOrigin OwnershipReason = "runtime_local_origin"
	OwnershipReasonBuiltin            OwnershipReason = "builtin"
	OwnershipReasonUnknownOrigin      OwnershipReason = "unknown_origin"
	OwnershipReasonMalformedConfig    OwnershipReason = "malformed_config"
)

type OwnershipInput struct {
	Builtin                   bool
	PluginInstallationPresent bool
	Config                    json.RawMessage
}

type Ownership struct {
	Class           OwnershipClass
	Reason          OwnershipReason
	DirectEvolution bool
	ForkRequired    bool
}

func (c OwnershipClass) Valid() bool {
	switch c {
	case OwnershipWorkspace, OwnershipPlugin, OwnershipExternal,
		OwnershipRuntimeLocal, OwnershipBuiltin, OwnershipUnknown:
		return true
	default:
		return false
	}
}

// ClassifyOwnership uses persisted database ownership, never runtime bundle
// source. Any config shape that could conceal an origin fails closed.
func ClassifyOwnership(input OwnershipInput) Ownership {
	if input.Builtin {
		return Ownership{Class: OwnershipBuiltin, Reason: OwnershipReasonBuiltin}
	}
	if input.PluginInstallationPresent {
		return Ownership{Class: OwnershipPlugin, Reason: OwnershipReasonPluginInstallation, ForkRequired: true}
	}
	raw := bytes.TrimSpace(input.Config)
	if len(raw) == 0 {
		return workspaceOwnership()
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil || config == nil {
		return unknownOwnership(OwnershipReasonMalformedConfig)
	}
	originRaw, present := config["origin"]
	if !present {
		return workspaceOwnership()
	}
	var origin map[string]json.RawMessage
	if err := json.Unmarshal(originRaw, &origin); err != nil || origin == nil {
		return unknownOwnership(OwnershipReasonUnknownOrigin)
	}
	typeRaw, present := origin["type"]
	if !present {
		return unknownOwnership(OwnershipReasonUnknownOrigin)
	}
	var originType string
	if err := json.Unmarshal(typeRaw, &originType); err != nil || originType == "" || strings.TrimSpace(originType) != originType {
		return unknownOwnership(OwnershipReasonUnknownOrigin)
	}
	switch originType {
	case "github", "skills_sh", "clawhub":
		return Ownership{Class: OwnershipExternal, Reason: OwnershipReasonExternalOrigin, ForkRequired: true}
	case "runtime_local":
		return Ownership{Class: OwnershipRuntimeLocal, Reason: OwnershipReasonRuntimeLocalOrigin, ForkRequired: true}
	default:
		return unknownOwnership(OwnershipReasonUnknownOrigin)
	}
}

func workspaceOwnership() Ownership {
	return Ownership{Class: OwnershipWorkspace, Reason: OwnershipReasonManual, DirectEvolution: true}
}

func unknownOwnership(reason OwnershipReason) Ownership {
	return Ownership{Class: OwnershipUnknown, Reason: reason, ForkRequired: true}
}

// ValidateCandidateBundle applies portable bundle validation and strict
// candidate frontmatter. Existing live Skills may be snapshotted with
// skillbundle.BuildValidatedManifest without satisfying the candidate-only
// frontmatter rules.
func ValidateCandidateBundle(candidate skillbundle.Skill) (skillbundle.Manifest, error) {
	if candidate.ID == "" || candidate.Name == "" {
		return skillbundle.Manifest{}, ErrCandidateIdentity
	}
	if candidate.Source != skillbundle.SourceWorkspace {
		return skillbundle.Manifest{}, ErrCandidateSource
	}
	manifest, err := skillbundle.BuildValidatedManifest(candidate)
	if err != nil {
		return skillbundle.Manifest{}, err
	}
	name, description, err := strictFrontmatter(candidate.Content)
	if err != nil {
		return skillbundle.Manifest{}, err
	}
	if name != candidate.Name || description != candidate.Description {
		return skillbundle.Manifest{}, ErrFrontmatterMismatch
	}
	return manifest, nil
}

func strictFrontmatter(content string) (string, string, error) {
	start := 0
	switch {
	case strings.HasPrefix(content, "---\n"):
		start = len("---\n")
	case strings.HasPrefix(content, "---\r\n"):
		start = len("---\r\n")
	default:
		return "", "", ErrFrontmatterMissing
	}

	end := -1
	for cursor := start; cursor <= len(content); {
		next := strings.IndexByte(content[cursor:], '\n')
		lineEnd := len(content)
		advance := len(content) + 1
		if next >= 0 {
			lineEnd = cursor + next
			advance = lineEnd + 1
		}
		line := strings.TrimSuffix(content[cursor:lineEnd], "\r")
		if line == "---" {
			end = cursor
			break
		}
		if next < 0 {
			break
		}
		cursor = advance
	}
	if end < 0 {
		return "", "", ErrFrontmatterMissing
	}

	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(content[start:end]))
	if err := decoder.Decode(&document); err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrFrontmatterInvalid, err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", "", ErrFrontmatterInvalid
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", "", ErrFrontmatterInvalid
	}

	root := document.Content[0]
	seen := make(map[string]struct{}, len(root.Content)/2)
	name, description := "", ""
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return "", "", ErrFrontmatterInvalid
		}
		if _, exists := seen[key.Value]; exists {
			return "", "", ErrFrontmatterInvalid
		}
		seen[key.Value] = struct{}{}
		switch key.Value {
		case "name":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" || !validFrontmatterLabel(value.Value) {
				return "", "", ErrFrontmatterInvalid
			}
			name = value.Value
		case "description":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" || !validFrontmatterLabel(value.Value) {
				return "", "", ErrFrontmatterInvalid
			}
			description = value.Value
		}
	}
	if name == "" || description == "" {
		return "", "", ErrFrontmatterInvalid
	}
	return name, description, nil
}

func validFrontmatterLabel(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\t")
}
