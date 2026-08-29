package skillbundle

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildManifestStableAcrossFileOrder(t *testing.T) {
	a := BuildManifest(Skill{
		ID:      "skill-1",
		Source:  SourceWorkspace,
		Name:    "deploy",
		Content: "main",
		Files: []File{
			{Path: "b.md", Content: "b"},
			{Path: "a.md", Content: "a"},
		},
	})
	b := BuildManifest(Skill{
		ID:      "skill-1",
		Source:  SourceWorkspace,
		Name:    "deploy",
		Content: "main",
		Files: []File{
			{Path: "a.md", Content: "a"},
			{Path: "b.md", Content: "b"},
		},
	})
	if a.Hash != b.Hash {
		t.Fatalf("hash depends on file order: %s != %s", a.Hash, b.Hash)
	}
}

func TestBuildManifestChangesWhenContentChanges(t *testing.T) {
	a := BuildManifest(Skill{ID: "skill-1", Source: SourceWorkspace, Name: "deploy", Content: "main"})
	b := BuildManifest(Skill{ID: "skill-1", Source: SourceWorkspace, Name: "deploy", Content: "changed"})
	if a.Hash == b.Hash {
		t.Fatal("hash did not change when content changed")
	}
}

func TestBuildManifestCanonicalChangeMatrix(t *testing.T) {
	base := Skill{
		ID:          "skill-1",
		Source:      SourceWorkspace,
		Name:        "deploy",
		Description: "Deploy safely",
		Content:     "---\nname: deploy\ndescription: Deploy safely\n---\n",
		Files:       []File{{Path: "references/check.md", Content: "check"}},
	}
	baseManifest := BuildManifest(base)
	if got, want := baseManifest.Hash, "sha256:0a70216f3437051e924fc86720fc0432877f4f7809c0cd1b3f3f5127bf416934"; got != want {
		t.Fatalf("canonical bundle hash = %q, want %q", got, want)
	}
	for _, tc := range []struct {
		name   string
		change func(*Skill)
	}{
		{name: "rename", change: func(s *Skill) { s.Files[0].Path = "references/review.md" }},
		{name: "description", change: func(s *Skill) { s.Description = "Deploy carefully" }},
		{name: "missing file", change: func(s *Skill) { s.Files = nil }},
		{name: "unicode content", change: func(s *Skill) { s.Files[0].Content = "检查" }},
		{name: "empty file", change: func(s *Skill) { s.Files[0].Content = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.Files = append([]File(nil), base.Files...)
			tc.change(&candidate)
			if got := BuildManifest(candidate).Hash; got == baseManifest.Hash {
				t.Fatalf("hash did not change for %s", tc.name)
			}
		})
	}
}

func TestBuildManifestDuplicateOrderIsDeterministic(t *testing.T) {
	left := Skill{Files: []File{{Path: "a.md", Content: "first"}, {Path: "a.md", Content: "second"}}}
	right := Skill{Files: []File{{Path: "a.md", Content: "second"}, {Path: "a.md", Content: "first"}}}
	if BuildManifest(left).Hash != BuildManifest(right).Hash {
		t.Fatal("legacy manifest hash is input-order-dependent for duplicate paths")
	}
}

func TestValidatePortableBundleMatrix(t *testing.T) {
	valid := Skill{
		ID:      "skill-1",
		Source:  SourceWorkspace,
		Name:    "deploy",
		Content: "primary\n",
		Files: []File{
			{Path: "参考/检查.md", Content: ""},
			{Path: "references/runbook.md", Content: "line one\n\tline two"},
		},
	}
	if _, err := BuildValidatedManifest(valid); err != nil {
		t.Fatalf("valid Unicode bundle rejected: %v", err)
	}

	invalidUTF8 := string([]byte{'a', 0xff})
	for _, tc := range []struct {
		name string
		edit func(*Skill)
		want error
	}{
		{name: "empty path", edit: func(s *Skill) { s.Files[0].Path = "" }, want: ErrInvalidPath},
		{name: "absolute", edit: func(s *Skill) { s.Files[0].Path = "/tmp/a.md" }, want: ErrInvalidPath},
		{name: "backslash", edit: func(s *Skill) { s.Files[0].Path = `refs\a.md` }, want: ErrInvalidPath},
		{name: "windows separator character", edit: func(s *Skill) { s.Files[0].Path = "refs/a:b.md" }, want: ErrInvalidPath},
		{name: "windows device", edit: func(s *Skill) { s.Files[0].Path = "refs/CON.txt" }, want: ErrInvalidPath},
		{name: "windows numbered device", edit: func(s *Skill) { s.Files[0].Path = "refs/lpt9" }, want: ErrInvalidPath},
		{name: "windows trailing dot", edit: func(s *Skill) { s.Files[0].Path = "refs/a." }, want: ErrInvalidPath},
		{name: "windows trailing space", edit: func(s *Skill) { s.Files[0].Path = "refs/a.md " }, want: ErrInvalidPath},
		{name: "traversal", edit: func(s *Skill) { s.Files[0].Path = "../a.md" }, want: ErrInvalidPath},
		{name: "alias", edit: func(s *Skill) { s.Files[0].Path = "refs/../a.md" }, want: ErrInvalidPath},
		{name: "path control", edit: func(s *Skill) { s.Files[0].Path = "refs/\na.md" }, want: ErrInvalidPath},
		{name: "path invalid utf8", edit: func(s *Skill) { s.Files[0].Path = invalidUTF8 }, want: ErrInvalidPath},
		{name: "reserved root", edit: func(s *Skill) { s.Files[0].Path = "skill.md" }, want: ErrReservedPath},
		{name: "binary extension", edit: func(s *Skill) { s.Files[0].Path = "refs/image.png" }, want: ErrBinaryFile},
		{name: "content nul", edit: func(s *Skill) { s.Files[0].Content = "a\x00b" }, want: ErrInvalidText},
		{name: "content invalid utf8", edit: func(s *Skill) { s.Files[0].Content = invalidUTF8 }, want: ErrInvalidText},
		{name: "exact duplicate", edit: func(s *Skill) { s.Files[1].Path = s.Files[0].Path }, want: ErrDuplicatePath},
		{name: "portable case duplicate", edit: func(s *Skill) { s.Files[0].Path = "refs/A.md"; s.Files[1].Path = "refs/a.md" }, want: ErrDuplicatePath},
		{name: "portable unicode case duplicate", edit: func(s *Skill) { s.Files[0].Path = "refs/Ä.md"; s.Files[1].Path = "refs/ä.md" }, want: ErrDuplicatePath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			candidate.Files = append([]File(nil), valid.Files...)
			tc.edit(&candidate)
			if err := Validate(candidate); !errors.Is(err, tc.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateBundleLimitsAndAccounting(t *testing.T) {
	limits := Limits{
		MaxPrimaryContentBytes: 4,
		MaxSupportingFileBytes: 4,
		MaxSupportingFiles:     2,
		MaxSupportingPathBytes: 8,
		MaxBundleBytes:         18,
	}
	base := Skill{ID: "i", Source: "s", Name: "n", Content: "1234", Files: []File{{Path: "a.md", Content: "1234"}}}
	// 1 source + 1 ID + 1 name + 0 description + 4 primary + 4 path + 4 file = 15.
	if err := ValidateWithLimits(base, limits); err != nil {
		t.Fatalf("exactly bounded bundle rejected: %v", err)
	}
	checks := []struct {
		name string
		edit func(*Skill)
		want error
	}{
		{name: "primary", edit: func(s *Skill) { s.Content = "12345" }, want: ErrPrimaryTooLarge},
		{name: "file", edit: func(s *Skill) { s.Files[0].Content = "12345" }, want: ErrFileTooLarge},
		{name: "count", edit: func(s *Skill) { s.Files = append(s.Files, File{Path: "b.md"}, File{Path: "c.md"}) }, want: ErrTooManyFiles},
		{name: "path", edit: func(s *Skill) { s.Files[0].Path = "123456789" }, want: ErrInvalidPath},
		{name: "metadata counted in total", edit: func(s *Skill) { s.Description = "metadata" }, want: ErrBundleTooLarge},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.Files = append([]File(nil), base.Files...)
			tc.edit(&candidate)
			if err := ValidateWithLimits(candidate, limits); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateWithLimits() error = %v, want %v", err, tc.want)
			}
		})
	}

	atMax := base
	atMax.Description = strings.Repeat("x", 3)
	if err := ValidateWithLimits(atMax, limits); err != nil {
		t.Fatalf("bundle exactly at total cap rejected: %v", err)
	}
	over := atMax
	over.Description += "x"
	if err := ValidateWithLimits(over, limits); !errors.Is(err, ErrBundleTooLarge) {
		t.Fatalf("bundle above total cap error = %v, want %v", err, ErrBundleTooLarge)
	}
}

func TestValidateErrorIsStableAcrossInputOrder(t *testing.T) {
	a := Skill{Files: []File{{Path: "z/../bad.md"}, {Path: "a/../bad.md"}}}
	b := Skill{Files: []File{{Path: "a/../bad.md"}, {Path: "z/../bad.md"}}}
	errA := Validate(a)
	errB := Validate(b)
	if errA == nil || errB == nil || errA.Error() != errB.Error() {
		t.Fatalf("validation errors differ by file order: %v != %v", errA, errB)
	}
}
