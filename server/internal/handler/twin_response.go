package handler

import (
	"encoding/json"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type twinProposalResponse struct {
	ID                   string                      `json:"id"`
	Kind                 string                      `json:"kind"`
	SourceWikiRevisionID string                      `json:"source_wiki_revision_id"`
	BaseTwinVersionID    *string                     `json:"base_twin_version_id"`
	SchemaVersion        int32                       `json:"schema_version"`
	Content              json.RawMessage             `json:"content"`
	ContentDigest        string                      `json:"content_digest"`
	RequestedByID        *string                     `json:"requested_by_id"`
	ReplacesProposalID   *string                     `json:"replaces_proposal_id"`
	CreatedAt            time.Time                   `json:"created_at"`
	Review               *twinProposalReviewResponse `json:"review"`
	SignedVersion        *twinVersionResponse        `json:"signed_version"`
}

type twinProposalReviewResponse struct {
	ID         string    `json:"id"`
	Decision   string    `json:"decision"`
	ReviewerID string    `json:"reviewer_id"`
	Reason     *string   `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

type twinVersionResponse struct {
	ID                   string          `json:"id"`
	VersionNumber        int64           `json:"version_number"`
	ProposalID           string          `json:"proposal_id"`
	SourceWikiRevisionID string          `json:"source_wiki_revision_id"`
	PriorVersionID       *string         `json:"prior_version_id"`
	SchemaVersion        int32           `json:"schema_version"`
	Content              json.RawMessage `json:"content"`
	ContentDigest        string          `json:"content_digest"`
	SignedOffByID        string          `json:"signed_off_by_id"`
	SignedOffAt          time.Time       `json:"signed_off_at"`
	CreatedAt            time.Time       `json:"created_at"`
}

type twinProposalDetailResponse struct {
	Proposal       twinProposalResponse     `json:"proposal"`
	SourceRevision lmWikiRevisionResponse   `json:"source_revision"`
	Citations      []lmWikiCitationResponse `json:"citations"`
	RunEvidence    *twinRunEvidenceResponse `json:"run_evidence,omitempty"`
}

type twinRunEvidenceResponse struct {
	TaskID            string         `json:"task_id"`
	BaseTwinVersionID string         `json:"base_twin_version_id"`
	EvidenceDigest    string         `json:"evidence_digest"`
	TaskStatus        string         `json:"task_status"`
	CompletedAt       *time.Time     `json:"completed_at"`
	FeedbackRating    *string        `json:"feedback_rating"`
	SafeMetadata      map[string]any `json:"safe_metadata"`
}

type twinVersionDetailResponse struct {
	Version        twinVersionResponse      `json:"version"`
	Proposal       twinProposalResponse     `json:"proposal"`
	SourceRevision lmWikiRevisionResponse   `json:"source_revision"`
	Citations      []lmWikiCitationResponse `json:"citations"`
}

type twinOverviewResponse struct {
	CurrentVersion  *twinVersionResponse   `json:"current_version"`
	PendingProposal *twinProposalResponse  `json:"pending_proposal"`
	Proposals       []twinProposalResponse `json:"proposals"`
	Versions        []twinVersionResponse  `json:"versions"`
	CanManage       bool                   `json:"can_manage"`
}

type twinProposalResultResponse struct {
	Created  bool                 `json:"created"`
	Proposal twinProposalResponse `json:"proposal"`
}

type twinVersionResultResponse struct {
	Created bool                `json:"created"`
	Version twinVersionResponse `json:"version"`
}

func mapTwinProposal(proposal db.TwinProposal, review *db.TwinProposalReview, version *db.TwinVersion) twinProposalResponse {
	response := twinProposalResponse{ID: uuidToString(proposal.ID), Kind: proposal.Kind, SourceWikiRevisionID: uuidToString(proposal.SourceWikiRevisionID), BaseTwinVersionID: optionalUUID(proposal.BaseTwinVersionID), SchemaVersion: proposal.SchemaVersion, Content: json.RawMessage(proposal.Content), ContentDigest: proposal.ContentDigest, RequestedByID: optionalUUID(proposal.RequestedByID), ReplacesProposalID: optionalUUID(proposal.ReplacesProposalID), CreatedAt: proposal.CreatedAt.Time}
	if review != nil {
		response.Review = &twinProposalReviewResponse{ID: uuidToString(review.ID), Decision: review.Decision, ReviewerID: uuidToString(review.ReviewerID), Reason: optionalText(review.Reason), CreatedAt: review.CreatedAt.Time}
	}
	if version != nil {
		mapped := mapTwinVersion(*version)
		response.SignedVersion = &mapped
	}
	return response
}

func mapTwinVersion(version db.TwinVersion) twinVersionResponse {
	return twinVersionResponse{ID: uuidToString(version.ID), VersionNumber: version.VersionNumber, ProposalID: uuidToString(version.ProposalID), SourceWikiRevisionID: uuidToString(version.SourceWikiRevisionID), PriorVersionID: optionalUUID(version.PriorVersionID), SchemaVersion: version.SchemaVersion, Content: json.RawMessage(version.Content), ContentDigest: version.ContentDigest, SignedOffByID: uuidToString(version.SignedOffByID), SignedOffAt: version.SignedOffAt.Time, CreatedAt: version.CreatedAt.Time}
}

func mapTwinProposalDetail(detail service.TwinProposalDetail) twinProposalDetailResponse {
	response := twinProposalDetailResponse{Proposal: mapTwinProposal(detail.Proposal, detail.Review, detail.Version), SourceRevision: mapLMWikiRevision(detail.SourceRevision, nil), Citations: mapTwinCitations(detail.Citations)}
	if detail.RunEvidence != nil {
		response.RunEvidence = &twinRunEvidenceResponse{
			TaskID: uuidToString(detail.RunEvidence.TaskID), BaseTwinVersionID: uuidToString(detail.RunEvidence.BaseTwinVersionID),
			EvidenceDigest: detail.RunEvidence.EvidenceDigest, TaskStatus: detail.RunEvidence.TaskStatus,
			CompletedAt: optionalTime(detail.RunEvidence.CompletedAt), FeedbackRating: optionalText(detail.RunEvidence.FeedbackRating),
			SafeMetadata: map[string]any{},
		}
	}
	return response
}

func mapTwinVersionDetail(detail service.TwinVersionDetail) twinVersionDetailResponse {
	return twinVersionDetailResponse{Version: mapTwinVersion(detail.Version), Proposal: mapTwinProposal(detail.Proposal, nil, &detail.Version), SourceRevision: mapLMWikiRevision(detail.SourceRevision, nil), Citations: mapTwinCitations(detail.Citations)}
}

func mapTwinCitations(citations []db.LmWikiCitation) []lmWikiCitationResponse {
	responses := make([]lmWikiCitationResponse, len(citations))
	for index, citation := range citations {
		responses[index] = lmWikiCitationResponse{ID: uuidToString(citation.ID), Ordinal: citation.Ordinal, CitationKey: citation.CitationKey, SourceType: citation.SourceType, SourceID: uuidToString(citation.SourceID), SourceUpdatedAt: optionalTime(citation.SourceUpdatedAt), Locator: citation.Locator, Label: citation.Label, SafeMetadata: json.RawMessage(citation.SafeMetadata), SourceDigest: citation.SourceDigest}
	}
	return responses
}
