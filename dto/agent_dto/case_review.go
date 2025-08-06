package agent_dto

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// ⚠️⚠️⚠️
// If you introduce a new version of the DTO, remember to update the dto_version in the case_review_file_repository.go file
// Be "new version", I mean anything that is a breaking change on the DTO, so adding fields is not a new version.
// ⚠️⚠️⚠️

type AiCaseReviewDto interface {
	aiCaseReviewDto()
}

type CaseReviewProof struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	IsDataModel bool   `json:"is_data_model"`
	Reason      string `json:"reason"`
}

type CaseReviewV1 struct {
	Ok          bool              `json:"ok"`
	Output      string            `json:"output"`
	SanityCheck string            `json:"sanity_check"`
	Thought     string            `json:"thought"`
	Version     string            `json:"version"`
	Proofs      []CaseReviewProof `json:"proofs"`
}

func (c CaseReviewV1) aiCaseReviewDto() {}

func UnmarshalCaseReviewDto(version string, payload io.Reader) (AiCaseReviewDto, error) {
	switch version {
	case "v1":
		var dto CaseReviewV1
		err := json.NewDecoder(payload).Decode(&dto)
		dto.Version = version
		return dto, err
	}

	return nil, errors.New("unsupported version")
}

type AiCaseReviewWithFeedbackDto struct {
	Id          uuid.UUID `json:"id"`
	Ok          bool      `json:"ok"`
	Output      string    `json:"output"`
	SanityCheck string    `json:"sanity_check"`
	Thought     string    `json:"thought"`
	Version     string    `json:"version"`

	Reaction *string `json:"reaction"`
}

type UpdateCaseReviewFeedbackDto struct {
	Reaction *string `json:"reaction"`
}

func (dto UpdateCaseReviewFeedbackDto) Validate() error {
	if dto.Reaction != nil && *dto.Reaction != "ok" && *dto.Reaction != "ko" {
		return errors.New("invalid reaction")
	}
	return nil
}

func (dto UpdateCaseReviewFeedbackDto) Adapt() models.AiCaseReviewFeedback {
	var reaction *models.AiCaseReviewReaction
	if dto.Reaction != nil {
		reaction = utils.Ptr(models.AiCaseReviewReactionFromString(*dto.Reaction))
	}

	return models.AiCaseReviewFeedback{
		Reaction: reaction,
	}
}
