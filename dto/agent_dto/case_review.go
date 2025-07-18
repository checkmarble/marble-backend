package agent_dto

import "github.com/checkmarble/marble-backend/models"

// ⚠️⚠️⚠️
// If you introduce a new version of the DTO, remember to update the dto_version in the case_review_file_repository.go file
// Be "new version", I mean anything that is a breaking change on the DTO, so adding fields is not a new version.
// ⚠️⚠️⚠️

type CaseReviewV1 struct {
	Ok          bool   `json:"ok"`
	Output      string `json:"output"`
	SanityCheck string `json:"sanity_check"`
	Thought     string `json:"thought"`
}

func AdaptCaseReviewV1(caseReview models.AiCaseReview) CaseReviewV1 {
	return CaseReviewV1{
		Ok:          caseReview.Ok,
		Output:      caseReview.Output,
		SanityCheck: caseReview.SanityCheck,
		Thought:     caseReview.Thought,
	}
}
