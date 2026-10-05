package skillevolution

import (
	"sort"
	"strings"
)

const (
	maxDiffLinesPerFile = 12000
	maxDiffRowsPerFile  = 2000
	maxDiffRowsTotal    = 10000
)

type BundleDiff struct {
	Metadata    []MetadataDiff `json:"metadata"`
	Files       []FileDiff     `json:"files"`
	Truncated   bool           `json:"truncated"`
	OmittedRows int            `json:"omitted_rows"`
}

type MetadataDiff struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type FileDiff struct {
	Path        string    `json:"path"`
	Change      string    `json:"change"`
	Rows        []DiffRow `json:"rows"`
	Truncated   bool      `json:"truncated"`
	OmittedRows int       `json:"omitted_rows"`
}

type DiffRow struct {
	Kind    string `json:"kind"`
	OldLine *int   `json:"old_line,omitempty"`
	NewLine *int   `json:"new_line,omitempty"`
	Text    string `json:"text"`
}

func BuildBundleDiff(base, candidate RevisionSnapshot) BundleDiff {
	diff := BundleDiff{Metadata: make([]MetadataDiff, 0, 2), Files: []FileDiff{}}
	if base.Revision.Description != candidate.Revision.Description {
		diff.Metadata = append(diff.Metadata, MetadataDiff{Field: "description", Before: base.Revision.Description, After: candidate.Revision.Description})
	}
	if base.Revision.Name != candidate.Revision.Name {
		diff.Metadata = append(diff.Metadata, MetadataDiff{Field: "name", Before: base.Revision.Name, After: candidate.Revision.Name})
	}
	baseFiles := revisionFileMap(base)
	candidateFiles := revisionFileMap(candidate)
	paths := make([]string, 0, len(baseFiles)+len(candidateFiles))
	seen := make(map[string]struct{}, len(baseFiles)+len(candidateFiles))
	for path := range baseFiles {
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	for path := range candidateFiles {
		if _, ok := seen[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		before, beforeOK := baseFiles[path]
		after, afterOK := candidateFiles[path]
		if beforeOK && afterOK && before == after {
			continue
		}
		change := "modified"
		switch {
		case !beforeOK:
			change = "added"
		case !afterOK:
			change = "deleted"
		}
		remaining := maxDiffRowsTotal
		for _, file := range diff.Files {
			remaining -= len(file.Rows)
		}
		if remaining < 0 {
			remaining = 0
		}
		rows, omitted := lineDiff(before, after, min(maxDiffRowsPerFile, remaining))
		file := FileDiff{Path: path, Change: change, Rows: rows, Truncated: omitted > 0, OmittedRows: omitted}
		diff.Files = append(diff.Files, file)
		diff.Truncated = diff.Truncated || file.Truncated
		diff.OmittedRows += omitted
	}
	return diff
}

func revisionFileMap(snapshot RevisionSnapshot) map[string]string {
	files := make(map[string]string, len(snapshot.Files)+1)
	files["SKILL.md"] = snapshot.Revision.PrimaryContent
	for _, file := range snapshot.Files {
		files[file.Path] = file.Content
	}
	return files
}

func lineDiff(before, after string, maxRows int) ([]DiffRow, int) {
	oldLines := splitDiffLines(before)
	newLines := splitDiffLines(after)
	if maxRows <= 0 {
		return []DiffRow{}, len(oldLines) + len(newLines)
	}
	if len(oldLines)+len(newLines) > maxDiffLinesPerFile {
		return replacementRows(oldLines, newLines, maxRows)
	}
	// The dynamic-programming table is intentionally bounded above. It yields
	// stable, review-friendly rows while large files fall back to replacement
	// rows instead of allowing an HTTP read to allocate unbounded memory.
	if len(oldLines)*len(newLines) > 2_000_000 {
		return replacementRows(oldLines, newLines, maxRows)
	}
	lcs := make([][]uint32, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]uint32, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	rows := make([]DiffRow, 0, len(oldLines)+len(newLines))
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			oldLine, newLine := i+1, j+1
			rows = append(rows, DiffRow{Kind: "context", OldLine: &oldLine, NewLine: &newLine, Text: oldLines[i]})
			i++
			j++
		case j < len(newLines) && (i == len(oldLines) || lcs[i][j+1] > lcs[i+1][j]):
			newLine := j + 1
			rows = append(rows, DiffRow{Kind: "add", NewLine: &newLine, Text: newLines[j]})
			j++
		default:
			oldLine := i + 1
			rows = append(rows, DiffRow{Kind: "delete", OldLine: &oldLine, Text: oldLines[i]})
			i++
		}
	}
	return boundedDiffRows(rows, maxRows)
}

func replacementRows(oldLines, newLines []string, maxRows int) ([]DiffRow, int) {
	total := len(oldLines) + len(newLines)
	if maxRows <= 0 {
		return []DiffRow{}, total
	}
	oldBudget := min(len(oldLines), maxRows)
	newBudget := min(len(newLines), maxRows-oldBudget)
	if len(newLines) > 0 && len(oldLines) > 0 {
		oldBudget = min(len(oldLines), maxRows/2)
		newBudget = min(len(newLines), maxRows-oldBudget)
		oldBudget = min(len(oldLines), maxRows-newBudget)
	}
	rows := make([]DiffRow, 0, oldBudget+newBudget)
	for i := range oldBudget {
		line := i + 1
		rows = append(rows, DiffRow{Kind: "delete", OldLine: &line, Text: oldLines[i]})
	}
	for i := range newBudget {
		line := i + 1
		rows = append(rows, DiffRow{Kind: "add", NewLine: &line, Text: newLines[i]})
	}
	return rows, total - len(rows)
}

func boundedDiffRows(rows []DiffRow, maxRows int) ([]DiffRow, int) {
	if len(rows) <= maxRows {
		return rows, 0
	}
	if maxRows <= 0 {
		return []DiffRow{}, len(rows)
	}
	head := maxRows / 2
	tail := maxRows - head
	bounded := make([]DiffRow, 0, maxRows)
	bounded = append(bounded, rows[:head]...)
	bounded = append(bounded, rows[len(rows)-tail:]...)
	return bounded, len(rows) - maxRows
}

func splitDiffLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(value, "\n"), "\n")
}
