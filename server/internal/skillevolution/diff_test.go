package skillevolution

import (
	"fmt"
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestBuildBundleDiffIncludesMetadataPrimaryAndSupportingFiles(t *testing.T) {
	base := RevisionSnapshot{Revision: db.SkillEvolutionRevision{Name: "review", Description: "old", PrimaryContent: "one\ntwo\n"}, Files: []db.SkillEvolutionRevisionFile{{Path: "references/a.md", Content: "same"}, {Path: "references/removed.md", Content: "gone"}}}
	candidate := RevisionSnapshot{Revision: db.SkillEvolutionRevision{Name: "review", Description: "new", PrimaryContent: "one\nchanged\n"}, Files: []db.SkillEvolutionRevisionFile{{Path: "references/a.md", Content: "same"}, {Path: "references/new.md", Content: "new"}}}

	diff := BuildBundleDiff(base, candidate)
	if len(diff.Metadata) != 1 || diff.Metadata[0].Field != "description" {
		t.Fatalf("unexpected metadata diff: %+v", diff.Metadata)
	}
	if len(diff.Files) != 3 || diff.Files[0].Path != "SKILL.md" || diff.Files[1].Change != "added" || diff.Files[2].Change != "deleted" {
		t.Fatalf("unexpected file diff: %+v", diff.Files)
	}
	rows := diff.Files[0].Rows
	if len(rows) != 3 || rows[0].Kind != "context" || rows[1].Kind != "delete" || rows[2].Kind != "add" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if rows[1].OldLine == nil || *rows[1].OldLine != 2 || rows[2].NewLine == nil || *rows[2].NewLine != 2 {
		t.Fatalf("unexpected line numbers: %+v", rows)
	}

}

func TestBuildBundleDiffBoundsLargeAndMultiFileResponses(t *testing.T) {
	largeBefore := strings.Repeat("before\n", maxDiffLinesPerFile)
	largeAfter := strings.Repeat("after\n", maxDiffLinesPerFile)
	base := RevisionSnapshot{Revision: db.SkillEvolutionRevision{PrimaryContent: largeBefore}}
	candidate := RevisionSnapshot{Revision: db.SkillEvolutionRevision{PrimaryContent: largeAfter}}
	for index := range 12 {
		path := fmt.Sprintf("references/%02d.md", index)
		base.Files = append(base.Files, db.SkillEvolutionRevisionFile{Path: path, Content: largeBefore})
		candidate.Files = append(candidate.Files, db.SkillEvolutionRevisionFile{Path: path, Content: largeAfter})
	}

	diff := BuildBundleDiff(base, candidate)
	rows := 0
	for _, file := range diff.Files {
		rows += len(file.Rows)
		if len(file.Rows) > maxDiffRowsPerFile {
			t.Fatalf("file %q emitted %d rows", file.Path, len(file.Rows))
		}
	}
	if rows > maxDiffRowsTotal || !diff.Truncated || diff.OmittedRows == 0 {
		t.Fatalf("unbounded diff: rows=%d truncated=%v omitted=%d", rows, diff.Truncated, diff.OmittedRows)
	}
}
