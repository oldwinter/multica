package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	MaxWikiReviewedProposalSignals       int32 = 100
	maxWikiReviewedProposalEvidenceBytes       = 2 * 1024 * 1024
	maxWikiReviewedProposalCitationBytes       = 64 * 1024
)

var (
	ErrInvalidWikiReviewedProposalSignal = errors.New("invalid Wiki reviewed-proposal signal")
	ErrWikiReviewedProposalUnavailable   = errors.New("Wiki reviewed-proposal signal unavailable")
	ErrWikiReviewedProposalDrift         = errors.New("Wiki reviewed-proposal signal digest drift")
)

// WikiReviewedProposalSignalRef is the content-free index projection exposed
// to downstream consumers. LoadReviewedProposalSignal must be used before any
// proposal or revision content is consumed.
type WikiReviewedProposalSignalRef struct {
	WorkspaceID        pgtype.UUID
	ProposalID         pgtype.UUID
	AcceptedRevisionID pgtype.UUID
	Decision           string
	Digest             string
	ObservedAt         pgtype.Timestamptz
}

// WikiReviewedProposalEvidence is the bounded source-owned envelope returned
// after custody, terminal review state, accepted provenance, and digest have
// all been revalidated. For accepted proposals, the path, title, and content
// come from the actual accepted revision, including any human override.
type WikiReviewedProposalEvidence struct {
	Ref                           WikiReviewedProposalSignalRef
	PageID                        pgtype.UUID
	AgentID                       pgtype.UUID
	ReviewedByID                  pgtype.UUID
	Path                          string
	Title                         string
	Content                       string
	Rationale                     string
	ReviewReason                  string
	Citations                     []string
	ProposalContentDigest         string
	AcceptedRevisionContentDigest string
}

type WikiReviewedProposalSignals interface {
	ListReviewedProposalSignals(context.Context, pgtype.UUID, int32) ([]WikiReviewedProposalSignalRef, error)
	LoadReviewedProposalSignal(context.Context, WikiReviewedProposalSignalRef) (WikiReviewedProposalEvidence, error)
}

type wikiReviewedProposalSignalAdapter struct {
	store wikiReviewedProposalSignalStore
}

// NewWikiReviewedProposalSignalAdapter creates the narrow Wiki-owned reader.
// It uses dedicated bounded projections because the existing page proposal
// query is page-scoped, unbounded, and selects proposal bodies.
func NewWikiReviewedProposalSignalAdapter(database db.DBTX) WikiReviewedProposalSignals {
	return &wikiReviewedProposalSignalAdapter{store: wikiReviewedProposalDBStore{database: database}}
}

func (a *wikiReviewedProposalSignalAdapter) ListReviewedProposalSignals(ctx context.Context, workspaceID pgtype.UUID, limit int32) ([]WikiReviewedProposalSignalRef, error) {
	if a == nil || a.store == nil || !workspaceID.Valid || limit <= 0 || limit > MaxWikiReviewedProposalSignals {
		return nil, ErrInvalidWikiReviewedProposalSignal
	}
	rows, err := a.store.listReviewedProposalSignals(ctx, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Wiki reviewed-proposal signals: %w", err)
	}
	refs := make([]WikiReviewedProposalSignalRef, 0, len(rows))
	for _, row := range rows {
		if int32(len(refs)) == limit {
			break
		}
		if !eligibleWikiReviewedProposalRow(row) || !row.proposalDigestValid {
			continue
		}
		ref := wikiReviewedProposalRef(row)
		if !validWikiReviewedProposalRef(ref) {
			continue
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (a *wikiReviewedProposalSignalAdapter) LoadReviewedProposalSignal(ctx context.Context, expected WikiReviewedProposalSignalRef) (WikiReviewedProposalEvidence, error) {
	if a == nil || a.store == nil || !validWikiReviewedProposalRef(expected) {
		return WikiReviewedProposalEvidence{}, ErrInvalidWikiReviewedProposalSignal
	}
	row, err := a.store.loadReviewedProposalSignal(ctx, expected.WorkspaceID, expected.ProposalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalUnavailable
	}
	if err != nil {
		return WikiReviewedProposalEvidence{}, fmt.Errorf("load Wiki reviewed-proposal signal: %w", err)
	}
	if row.workspaceID != expected.WorkspaceID || row.proposalID != expected.ProposalID {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalUnavailable
	}
	if !eligibleWikiReviewedProposalRow(row) {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalUnavailable
	}
	current := wikiReviewedProposalRef(row)
	if !row.proposalDigestValid || !wikiReviewedProposalRefEqual(current, expected) {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalDrift
	}
	if !validWikiReviewedProposalEvidenceRow(row) {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalDrift
	}
	citations, err := parseWikiReviewedProposalCitations(row.evidenceRefs)
	if err != nil {
		return WikiReviewedProposalEvidence{}, ErrWikiReviewedProposalUnavailable
	}
	return WikiReviewedProposalEvidence{
		Ref: current, PageID: row.pageID, AgentID: row.agentID, ReviewedByID: row.reviewedByID,
		Path: row.evidencePath, Title: row.evidenceTitle, Content: row.evidenceContent,
		Rationale: row.rationale, ReviewReason: row.reviewReason, Citations: citations,
		ProposalContentDigest:         row.proposalContentDigest,
		AcceptedRevisionContentDigest: row.acceptedRevisionContentDigest,
	}, nil
}

type wikiReviewedProposalSignalStore interface {
	listReviewedProposalSignals(context.Context, pgtype.UUID, int32) ([]wikiReviewedProposalSignalRow, error)
	loadReviewedProposalSignal(context.Context, pgtype.UUID, pgtype.UUID) (wikiReviewedProposalSignalRow, error)
}

type wikiReviewedProposalDBStore struct {
	database db.DBTX
}

type wikiReviewedProposalSignalRow struct {
	workspaceID                   pgtype.UUID
	proposalID                    pgtype.UUID
	pageID                        pgtype.UUID
	agentID                       pgtype.UUID
	reviewedByID                  pgtype.UUID
	decision                      string
	proposalContentDigest         string
	acceptedRevisionID            pgtype.UUID
	acceptedRevisionContentDigest string
	acceptedRevisionSourceKind    string
	acceptedRevisionSourceRefID   pgtype.UUID
	reviewedAt                    pgtype.Timestamptz
	scope                         string
	proposalPathDigest            string
	proposalTitleDigest           string
	rationaleDigest               string
	evidenceRefsDigest            string
	reviewReasonDigest            string
	acceptedRevisionPathDigest    string
	acceptedRevisionTitleDigest   string
	proposalDigestValid           bool
	evidencePath                  string
	evidenceTitle                 string
	evidenceContent               string
	rationale                     string
	reviewReason                  string
	evidenceRefs                  []byte
}

// This projection deliberately omits proposal and revision bodies. Hashes for
// the remaining mutable fields let Load detect changes without turning list
// into a content read.
const listWikiReviewedProposalSignalsSQL = `
SELECT proposal.workspace_id,
       proposal.id,
       proposal.page_id,
       proposal.agent_id,
       proposal.reviewed_by_id,
       proposal.status,
       proposal.content_digest,
       proposal.accepted_revision_id,
       COALESCE(revision.content_digest, ''),
       COALESCE(revision.source_kind, ''),
       revision.source_ref_id,
       proposal.reviewed_at,
       page.scope,
       'sha256:' || encode(sha256(convert_to(proposal.proposed_path, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.proposed_title, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.rationale, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.evidence_refs::text, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(COALESCE(proposal.review_reason, ''), 'UTF8')), 'hex'),
       CASE WHEN revision.id IS NULL THEN '' ELSE 'sha256:' || encode(sha256(convert_to(revision.path, 'UTF8')), 'hex') END,
       CASE WHEN revision.id IS NULL THEN '' ELSE 'sha256:' || encode(sha256(convert_to(revision.title, 'UTF8')), 'hex') END,
       proposal.content_digest = 'sha256:' || encode(sha256(convert_to(proposal.proposed_content, 'UTF8')), 'hex')
FROM wiki_page_edit_proposal proposal
JOIN wiki_page page
  ON page.id = proposal.page_id
 AND page.workspace_id = proposal.workspace_id
 AND page.scope IN ('workspace', 'project')
 AND page.owner_user_id IS NULL
LEFT JOIN wiki_page_revision revision
  ON revision.id = proposal.accepted_revision_id
 AND revision.page_id = proposal.page_id
 AND revision.workspace_id = proposal.workspace_id
 AND revision.owner_user_id IS NULL
WHERE proposal.workspace_id = $1
  AND proposal.status IN ('accepted', 'rejected')
  AND proposal.reviewed_by_id IS NOT NULL
  AND proposal.reviewed_at IS NOT NULL
  AND proposal.content_digest = 'sha256:' || encode(sha256(convert_to(proposal.proposed_content, 'UTF8')), 'hex')
  AND (
      (proposal.status = 'rejected' AND proposal.accepted_revision_id IS NULL)
      OR
      (proposal.status = 'accepted'
       AND revision.id = proposal.accepted_revision_id
       AND revision.source_kind = 'agent_proposal'
       AND revision.source_ref_id = proposal.id
       AND revision.content_digest = 'sha256:' || encode(sha256(convert_to(revision.content, 'UTF8')), 'hex'))
  )
  AND octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.path ELSE proposal.proposed_path END)
    + octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.title ELSE proposal.proposed_title END)
    + octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.content ELSE proposal.proposed_content END)
    + octet_length(proposal.rationale)
    + octet_length(proposal.evidence_refs::text)
    + octet_length(COALESCE(proposal.review_reason, '')) <= $3
ORDER BY proposal.reviewed_at DESC, proposal.id DESC
LIMIT $2`

const loadWikiReviewedProposalSignalSQL = `
SELECT proposal.workspace_id,
       proposal.id,
       proposal.page_id,
       proposal.agent_id,
       proposal.reviewed_by_id,
       proposal.status,
       proposal.content_digest,
       proposal.accepted_revision_id,
       COALESCE(revision.content_digest, ''),
       COALESCE(revision.source_kind, ''),
       revision.source_ref_id,
       proposal.reviewed_at,
       page.scope,
       'sha256:' || encode(sha256(convert_to(proposal.proposed_path, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.proposed_title, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.rationale, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(proposal.evidence_refs::text, 'UTF8')), 'hex'),
       'sha256:' || encode(sha256(convert_to(COALESCE(proposal.review_reason, ''), 'UTF8')), 'hex'),
       CASE WHEN revision.id IS NULL THEN '' ELSE 'sha256:' || encode(sha256(convert_to(revision.path, 'UTF8')), 'hex') END,
       CASE WHEN revision.id IS NULL THEN '' ELSE 'sha256:' || encode(sha256(convert_to(revision.title, 'UTF8')), 'hex') END,
       proposal.content_digest = 'sha256:' || encode(sha256(convert_to(proposal.proposed_content, 'UTF8')), 'hex'),
       CASE WHEN proposal.status = 'accepted' THEN COALESCE(revision.path, '') ELSE proposal.proposed_path END,
       CASE WHEN proposal.status = 'accepted' THEN COALESCE(revision.title, '') ELSE proposal.proposed_title END,
       CASE WHEN proposal.status = 'accepted' THEN COALESCE(revision.content, '') ELSE proposal.proposed_content END,
       proposal.rationale,
       COALESCE(proposal.review_reason, ''),
       proposal.evidence_refs
FROM wiki_page_edit_proposal proposal
JOIN wiki_page page
  ON page.id = proposal.page_id
 AND page.workspace_id = proposal.workspace_id
 AND page.scope IN ('workspace', 'project')
 AND page.owner_user_id IS NULL
LEFT JOIN wiki_page_revision revision
  ON revision.id = proposal.accepted_revision_id
 AND revision.page_id = proposal.page_id
 AND revision.workspace_id = proposal.workspace_id
 AND revision.owner_user_id IS NULL
WHERE proposal.workspace_id = $1
  AND proposal.id = $2
  AND proposal.status IN ('accepted', 'rejected')
  AND proposal.reviewed_by_id IS NOT NULL
  AND proposal.reviewed_at IS NOT NULL
  AND (
      (proposal.status = 'rejected' AND proposal.accepted_revision_id IS NULL)
      OR
      (proposal.status = 'accepted'
       AND revision.id = proposal.accepted_revision_id
       AND revision.source_kind = 'agent_proposal'
       AND revision.source_ref_id = proposal.id)
  )
  AND octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.path ELSE proposal.proposed_path END)
    + octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.title ELSE proposal.proposed_title END)
    + octet_length(CASE WHEN proposal.status = 'accepted' THEN revision.content ELSE proposal.proposed_content END)
    + octet_length(proposal.rationale)
    + octet_length(proposal.evidence_refs::text)
    + octet_length(COALESCE(proposal.review_reason, '')) <= $3`

func (s wikiReviewedProposalDBStore) listReviewedProposalSignals(ctx context.Context, workspaceID pgtype.UUID, limit int32) ([]wikiReviewedProposalSignalRow, error) {
	if s.database == nil {
		return nil, ErrInvalidWikiReviewedProposalSignal
	}
	rows, err := s.database.Query(ctx, listWikiReviewedProposalSignalsSQL, workspaceID, limit, maxWikiReviewedProposalEvidenceBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]wikiReviewedProposalSignalRow, 0, limit)
	for rows.Next() {
		var row wikiReviewedProposalSignalRow
		if err := scanWikiReviewedProposalMetadata(rows, &row); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s wikiReviewedProposalDBStore) loadReviewedProposalSignal(ctx context.Context, workspaceID, proposalID pgtype.UUID) (wikiReviewedProposalSignalRow, error) {
	if s.database == nil {
		return wikiReviewedProposalSignalRow{}, ErrInvalidWikiReviewedProposalSignal
	}
	var row wikiReviewedProposalSignalRow
	scanner := s.database.QueryRow(ctx, loadWikiReviewedProposalSignalSQL, workspaceID, proposalID, maxWikiReviewedProposalEvidenceBytes)
	if err := scanWikiReviewedProposalEvidence(scanner, &row); err != nil {
		return wikiReviewedProposalSignalRow{}, err
	}
	return row, nil
}

type wikiRowScanner interface {
	Scan(...any) error
}

func scanWikiReviewedProposalMetadata(scanner wikiRowScanner, row *wikiReviewedProposalSignalRow) error {
	return scanner.Scan(
		&row.workspaceID, &row.proposalID, &row.pageID, &row.agentID, &row.reviewedByID,
		&row.decision, &row.proposalContentDigest, &row.acceptedRevisionID,
		&row.acceptedRevisionContentDigest, &row.acceptedRevisionSourceKind,
		&row.acceptedRevisionSourceRefID, &row.reviewedAt, &row.scope,
		&row.proposalPathDigest, &row.proposalTitleDigest, &row.rationaleDigest,
		&row.evidenceRefsDigest, &row.reviewReasonDigest, &row.acceptedRevisionPathDigest,
		&row.acceptedRevisionTitleDigest, &row.proposalDigestValid,
	)
}

func scanWikiReviewedProposalEvidence(scanner wikiRowScanner, row *wikiReviewedProposalSignalRow) error {
	return scanner.Scan(
		&row.workspaceID, &row.proposalID, &row.pageID, &row.agentID, &row.reviewedByID,
		&row.decision, &row.proposalContentDigest, &row.acceptedRevisionID,
		&row.acceptedRevisionContentDigest, &row.acceptedRevisionSourceKind,
		&row.acceptedRevisionSourceRefID, &row.reviewedAt, &row.scope,
		&row.proposalPathDigest, &row.proposalTitleDigest, &row.rationaleDigest,
		&row.evidenceRefsDigest, &row.reviewReasonDigest, &row.acceptedRevisionPathDigest,
		&row.acceptedRevisionTitleDigest, &row.proposalDigestValid,
		&row.evidencePath, &row.evidenceTitle, &row.evidenceContent, &row.rationale,
		&row.reviewReason, &row.evidenceRefs,
	)
}

func eligibleWikiReviewedProposalRow(row wikiReviewedProposalSignalRow) bool {
	if !row.workspaceID.Valid || !row.proposalID.Valid || !row.pageID.Valid || !row.agentID.Valid ||
		!row.reviewedByID.Valid || !row.reviewedAt.Valid || (row.scope != "workspace" && row.scope != "project") ||
		!validWikiReviewedProposalDigest(row.proposalContentDigest) ||
		!validWikiReviewedProposalDigest(row.proposalPathDigest) ||
		!validWikiReviewedProposalDigest(row.proposalTitleDigest) ||
		!validWikiReviewedProposalDigest(row.rationaleDigest) ||
		!validWikiReviewedProposalDigest(row.evidenceRefsDigest) ||
		!validWikiReviewedProposalDigest(row.reviewReasonDigest) {
		return false
	}
	switch row.decision {
	case "accepted":
		return row.acceptedRevisionID.Valid &&
			validWikiReviewedProposalDigest(row.acceptedRevisionContentDigest) &&
			validWikiReviewedProposalDigest(row.acceptedRevisionPathDigest) &&
			validWikiReviewedProposalDigest(row.acceptedRevisionTitleDigest) &&
			row.acceptedRevisionSourceKind == "agent_proposal" &&
			row.acceptedRevisionSourceRefID.Valid &&
			row.acceptedRevisionSourceRefID == row.proposalID
	case "rejected":
		return !row.acceptedRevisionID.Valid && row.acceptedRevisionContentDigest == "" &&
			row.acceptedRevisionSourceKind == "" && !row.acceptedRevisionSourceRefID.Valid &&
			row.acceptedRevisionPathDigest == "" && row.acceptedRevisionTitleDigest == ""
	default:
		return false
	}
}

func validWikiReviewedProposalEvidenceRow(row wikiReviewedProposalSignalRow) bool {
	if !utf8.ValidString(row.evidencePath) || !utf8.ValidString(row.evidenceTitle) ||
		!utf8.ValidString(row.evidenceContent) || !utf8.ValidString(row.rationale) ||
		!utf8.ValidString(row.reviewReason) || len(row.evidenceRefs) > maxWikiReviewedProposalCitationBytes ||
		len(row.evidencePath)+len(row.evidenceTitle)+len(row.evidenceContent)+len(row.rationale)+
			len(row.reviewReason)+len(row.evidenceRefs) > maxWikiReviewedProposalEvidenceBytes ||
		wikiReviewedProposalStringDigest(row.evidencePath) != wikiReviewedProposalEvidencePathDigest(row) ||
		wikiReviewedProposalStringDigest(row.evidenceTitle) != wikiReviewedProposalEvidenceTitleDigest(row) ||
		wikiReviewedProposalStringDigest(row.rationale) != row.rationaleDigest ||
		wikiReviewedProposalStringDigest(row.reviewReason) != row.reviewReasonDigest {
		return false
	}
	if row.decision == "accepted" {
		return wikiReviewedProposalStringDigest(row.evidenceContent) == row.acceptedRevisionContentDigest
	}
	return wikiReviewedProposalStringDigest(row.evidenceContent) == row.proposalContentDigest
}

func wikiReviewedProposalEvidencePathDigest(row wikiReviewedProposalSignalRow) string {
	if row.decision == "accepted" {
		return row.acceptedRevisionPathDigest
	}
	return row.proposalPathDigest
}

func wikiReviewedProposalEvidenceTitleDigest(row wikiReviewedProposalSignalRow) string {
	if row.decision == "accepted" {
		return row.acceptedRevisionTitleDigest
	}
	return row.proposalTitleDigest
}

func wikiReviewedProposalRef(row wikiReviewedProposalSignalRow) WikiReviewedProposalSignalRef {
	return WikiReviewedProposalSignalRef{
		WorkspaceID: row.workspaceID, ProposalID: row.proposalID,
		AcceptedRevisionID: row.acceptedRevisionID, Decision: row.decision,
		Digest: wikiReviewedProposalSignalDigest(row), ObservedAt: row.reviewedAt,
	}
}

func validWikiReviewedProposalRef(ref WikiReviewedProposalSignalRef) bool {
	if !ref.WorkspaceID.Valid || !ref.ProposalID.Valid || !ref.ObservedAt.Valid ||
		!validWikiReviewedProposalDigest(ref.Digest) {
		return false
	}
	return ref.Decision == "accepted" && ref.AcceptedRevisionID.Valid ||
		ref.Decision == "rejected" && !ref.AcceptedRevisionID.Valid
}

func wikiReviewedProposalRefEqual(left, right WikiReviewedProposalSignalRef) bool {
	return left.WorkspaceID == right.WorkspaceID && left.ProposalID == right.ProposalID &&
		left.AcceptedRevisionID == right.AcceptedRevisionID && left.Decision == right.Decision &&
		left.Digest == right.Digest && left.ObservedAt.Time.Equal(right.ObservedAt.Time)
}

func wikiReviewedProposalSignalDigest(row wikiReviewedProposalSignalRow) string {
	h := sha256.New()
	writeWikiReviewedProposalDigestPart(h, "wiki-reviewed-proposal-signal-v1")
	for _, value := range []string{
		wikiUUIDString(row.workspaceID), wikiUUIDString(row.proposalID), wikiUUIDString(row.pageID),
		wikiUUIDString(row.agentID), wikiUUIDString(row.reviewedByID), row.decision,
		row.proposalContentDigest, row.proposalPathDigest, row.proposalTitleDigest,
		row.rationaleDigest, row.evidenceRefsDigest, row.reviewReasonDigest,
		wikiUUIDString(row.acceptedRevisionID), row.acceptedRevisionContentDigest,
		row.acceptedRevisionPathDigest, row.acceptedRevisionTitleDigest,
		row.acceptedRevisionSourceKind, wikiUUIDString(row.acceptedRevisionSourceRefID),
		row.reviewedAt.Time.UTC().Format(time.RFC3339Nano),
	} {
		writeWikiReviewedProposalDigestPart(h, value)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func wikiReviewedProposalStringDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeWikiReviewedProposalDigestPart(h hash.Hash, value string) {
	_, _ = fmt.Fprintf(h, "%d:%s\n", len(value), value)
}

func validWikiReviewedProposalDigest(value string) bool {
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

func parseWikiReviewedProposalCitations(raw []byte) ([]string, error) {
	if len(raw) == 0 || len(raw) > maxWikiReviewedProposalCitationBytes {
		return nil, ErrWikiReviewedProposalUnavailable
	}
	var citations []string
	if err := json.Unmarshal(raw, &citations); err != nil || len(citations) == 0 || len(citations) > maxWikiEvidenceRefs {
		return nil, ErrWikiReviewedProposalUnavailable
	}
	for _, citation := range citations {
		kind, id, ok := strings.Cut(citation, ":")
		if !ok || (kind != "task" && kind != "room") || strings.Contains(id, ":") {
			return nil, ErrWikiReviewedProposalUnavailable
		}
		var parsed pgtype.UUID
		if err := parsed.Scan(id); err != nil || !parsed.Valid {
			return nil, ErrWikiReviewedProposalUnavailable
		}
	}
	return append([]string(nil), citations...), nil
}

func wikiUUIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return value.String()
}
