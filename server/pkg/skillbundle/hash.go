package skillbundle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	skillpkg "github.com/multica-ai/multica/server/internal/skill"
)

const (
	SourceWorkspace = "workspace"
	SourceBuiltin   = "builtin"
	SourcePlugin    = "plugin"

	MaxPrimaryContentBytes = 1 << 20
	MaxSupportingFileBytes = 1 << 20
	MaxSupportingFiles     = 256
	MaxSupportingPathBytes = 1024
	MaxBundleBytes         = 8 << 20
)

var (
	ErrInvalidText        = errors.New("skill bundle contains invalid text")
	ErrInvalidPath        = errors.New("skill bundle contains an invalid supporting file path")
	ErrDuplicatePath      = errors.New("skill bundle contains duplicate supporting file paths")
	ErrReservedPath       = errors.New("skill bundle contains the reserved primary content path")
	ErrBinaryFile         = errors.New("skill bundle contains a binary supporting file")
	ErrTooManyFiles       = errors.New("skill bundle contains too many supporting files")
	ErrPrimaryTooLarge    = errors.New("skill bundle primary content is too large")
	ErrFileTooLarge       = errors.New("skill bundle supporting file is too large")
	ErrBundleTooLarge     = errors.New("skill bundle is too large")
	ErrInvalidBundleLimit = errors.New("skill bundle validation limits are invalid")
)

// ValidationCode is a stable machine-readable reason for bundle rejection.
type ValidationCode string

const (
	ValidationInvalidText   ValidationCode = "invalid_text"
	ValidationInvalidPath   ValidationCode = "invalid_path"
	ValidationDuplicatePath ValidationCode = "duplicate_path"
	ValidationReservedPath  ValidationCode = "reserved_path"
	ValidationBinaryFile    ValidationCode = "binary_file"
	ValidationTooManyFiles  ValidationCode = "too_many_files"
	ValidationPrimarySize   ValidationCode = "primary_too_large"
	ValidationFileSize      ValidationCode = "file_too_large"
	ValidationBundleSize    ValidationCode = "bundle_too_large"
	ValidationInvalidLimits ValidationCode = "invalid_limits"
)

// ValidationError deliberately omits user-controlled values from Error().
// Callers can safely map Code to a typed API error without leaking paths or
// candidate content.
type ValidationError struct {
	Code   ValidationCode
	Field  string
	Limit  int64
	Actual int64
	cause  error
}

func (e *ValidationError) Error() string {
	if e.Limit > 0 {
		return fmt.Sprintf("skill bundle validation failed: %s (%s: %d > %d)", e.Code, e.Field, e.Actual, e.Limit)
	}
	return fmt.Sprintf("skill bundle validation failed: %s (%s)", e.Code, e.Field)
}

func (e *ValidationError) Unwrap() error { return e.cause }

// Limits controls strict bundle validation. MaxBundleBytes includes all
// unhashed input bytes: identity/source metadata, primary content, supporting
// paths, and supporting content. Manifest.SizeBytes retains its historical
// content-only meaning for protocol compatibility.
type Limits struct {
	MaxPrimaryContentBytes int64
	MaxSupportingFileBytes int64
	MaxSupportingFiles     int
	MaxSupportingPathBytes int
	MaxBundleBytes         int64
}

func DefaultLimits() Limits {
	return Limits{
		MaxPrimaryContentBytes: MaxPrimaryContentBytes,
		MaxSupportingFileBytes: MaxSupportingFileBytes,
		MaxSupportingFiles:     MaxSupportingFiles,
		MaxSupportingPathBytes: MaxSupportingPathBytes,
		MaxBundleBytes:         MaxBundleBytes,
	}
}

type File struct {
	Path    string
	Content string
}

type Skill struct {
	ID          string
	Source      string
	Name        string
	Description string
	Content     string
	Files       []File
}

type FileRef struct {
	Path      string
	SHA256    string
	SizeBytes int64
}

type Manifest struct {
	Hash      string
	SizeBytes int64
	FileCount int
	Files     []FileRef
}

func BuildManifest(skill Skill) Manifest {
	files := append([]File(nil), skill.Files...)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		return files[i].Content < files[j].Content
	})

	h := sha256.New()
	writeHashPart(h, "v1")
	writeHashPart(h, skill.Source)
	writeHashPart(h, skill.ID)
	writeHashPart(h, skill.Name)
	writeHashPart(h, skill.Description)
	writeHashPart(h, skill.Content)

	size := int64(len(skill.Content))
	refs := make([]FileRef, 0, len(files))
	for _, file := range files {
		fileHash := sha256.Sum256([]byte(file.Content))
		fileDigest := "sha256:" + hex.EncodeToString(fileHash[:])
		writeHashPart(h, file.Path)
		writeHashPart(h, fileDigest)
		writeHashPart(h, file.Content)
		size += int64(len(file.Content))
		refs = append(refs, FileRef{
			Path:      file.Path,
			SHA256:    fileDigest,
			SizeBytes: int64(len(file.Content)),
		})
	}

	return Manifest{
		Hash:      "sha256:" + hex.EncodeToString(h.Sum(nil)),
		SizeBytes: size,
		FileCount: len(files),
		Files:     refs,
	}
}

// BuildValidatedManifest validates the portable canonical bundle contract
// before hashing it. BuildManifest remains available for legacy runtime paths
// whose data was accepted before the strict contract existed.
func BuildValidatedManifest(skill Skill) (Manifest, error) {
	return BuildValidatedManifestWithLimits(skill, DefaultLimits())
}

func BuildValidatedManifestWithLimits(skill Skill, limits Limits) (Manifest, error) {
	if err := ValidateWithLimits(skill, limits); err != nil {
		return Manifest{}, err
	}
	return BuildManifest(skill), nil
}

func Validate(skill Skill) error {
	return ValidateWithLimits(skill, DefaultLimits())
}

// ValidateWithLimits applies the same checks in a deterministic order after
// sorting a private copy of supporting files. Reordering an invalid candidate
// therefore cannot change the first reported error.
func ValidateWithLimits(bundle Skill, limits Limits) error {
	if limits.MaxPrimaryContentBytes < 0 || limits.MaxSupportingFileBytes < 0 ||
		limits.MaxSupportingFiles < 0 || limits.MaxSupportingPathBytes < 0 || limits.MaxBundleBytes < 0 {
		return validationError(ValidationInvalidLimits, "limits", 0, 0, ErrInvalidBundleLimit)
	}

	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "source", value: bundle.Source},
		{name: "id", value: bundle.ID},
		{name: "name", value: bundle.Name},
		{name: "description", value: bundle.Description},
		{name: "primary_content", value: bundle.Content},
	} {
		if !portableText(field.value) {
			return validationError(ValidationInvalidText, field.name, 0, 0, ErrInvalidText)
		}
	}
	if int64(len(bundle.Content)) > limits.MaxPrimaryContentBytes {
		return validationError(ValidationPrimarySize, "primary_content", limits.MaxPrimaryContentBytes, int64(len(bundle.Content)), ErrPrimaryTooLarge)
	}
	if len(bundle.Files) > limits.MaxSupportingFiles {
		return validationError(ValidationTooManyFiles, "supporting_files", int64(limits.MaxSupportingFiles), int64(len(bundle.Files)), ErrTooManyFiles)
	}

	files := append([]File(nil), bundle.Files...)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Path != files[j].Path {
			return files[i].Path < files[j].Path
		}
		return files[i].Content < files[j].Content
	})
	seenPaths := make(map[string]struct{}, len(files))
	total := int64(len(bundle.Source) + len(bundle.ID) + len(bundle.Name) + len(bundle.Description) + len(bundle.Content))
	for _, file := range files {
		if err := validatePortablePath(file.Path, limits.MaxSupportingPathBytes); err != nil {
			return err
		}
		portableIdentity := strings.ToLower(file.Path)
		if _, exists := seenPaths[portableIdentity]; exists {
			return validationError(ValidationDuplicatePath, "supporting_file_path", 0, 0, ErrDuplicatePath)
		}
		seenPaths[portableIdentity] = struct{}{}
		if skillpkg.IsReservedContentPath(file.Path) {
			return validationError(ValidationReservedPath, "supporting_file_path", 0, 0, ErrReservedPath)
		}
		if skillpkg.IsLikelyBinaryFilePath(file.Path) {
			return validationError(ValidationBinaryFile, "supporting_file_path", 0, 0, ErrBinaryFile)
		}
		if !portableText(file.Content) {
			return validationError(ValidationInvalidText, "supporting_file_content", 0, 0, ErrInvalidText)
		}
		if int64(len(file.Content)) > limits.MaxSupportingFileBytes {
			return validationError(ValidationFileSize, "supporting_file_content", limits.MaxSupportingFileBytes, int64(len(file.Content)), ErrFileTooLarge)
		}
		total += int64(len(file.Path) + len(file.Content))
	}
	if total > limits.MaxBundleBytes {
		return validationError(ValidationBundleSize, "bundle", limits.MaxBundleBytes, total, ErrBundleTooLarge)
	}
	return nil
}

func validatePortablePath(value string, maxBytes int) error {
	if value == "" || !utf8.ValidString(value) ||
		strings.HasPrefix(value, "/") || strings.Contains(value, "\\") ||
		path.Clean(value) != value || value == "." || value == ".." ||
		strings.HasPrefix(value, "../") {
		return validationError(ValidationInvalidPath, "supporting_file_path", 0, 0, ErrInvalidPath)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return validationError(ValidationInvalidPath, "supporting_file_path", 0, 0, ErrInvalidPath)
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if !portablePathSegment(segment) {
			return validationError(ValidationInvalidPath, "supporting_file_path", 0, 0, ErrInvalidPath)
		}
	}
	if len(value) > maxBytes {
		return validationError(ValidationInvalidPath, "supporting_file_path", int64(maxBytes), int64(len(value)), ErrInvalidPath)
	}
	return nil
}

func portablePathSegment(segment string) bool {
	if segment == "" || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") ||
		strings.ContainsAny(segment, `<>:"|?*`) {
		return false
	}
	base := segment
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if len(base) == 4 && base[3] >= '1' && base[3] <= '9' &&
		(strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return false
	}
	return true
}

func portableText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func validationError(code ValidationCode, field string, limit, actual int64, cause error) error {
	return &ValidationError{Code: code, Field: field, Limit: limit, Actual: actual, cause: cause}
}

func writeHashPart(h interface{ Write([]byte) (int, error) }, value string) {
	_, _ = fmt.Fprintf(h, "%d:%s\n", len(value), value)
}
