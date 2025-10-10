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

const (
	versionCaseReviewV1 = "v1"
)

type AiCaseReviewDto interface {
	aiCaseReviewDto()
	GetVersion() string
}

type OriginName string

const (
	OriginNameDataModel OriginName = "data_model"
	OriginNameInternal  OriginName = "internal"
	OriginNameUnknown   OriginName = "unknown"
)

type CaseReviewProof struct {
	Id     string     `json:"id"`
	Type   string     `json:"type"`
	Origin OriginName `json:"origin"`
	Reason string     `json:"reason"`
}

func OriginNameFromString(s string) OriginName {
	switch s {
	case "data_model":
		return OriginNameDataModel
	case "internal":
		return OriginNameInternal
	default:
		return OriginNameUnknown
	}
}

type CaseReviewV1 struct {
	Ok               bool                     `json:"ok"`
	Output           string                   `json:"output"`
	SanityCheck      string                   `json:"sanity_check"`
	Thought          string                   `json:"thought"`
	Version          string                   `json:"version"`
	Proofs           []CaseReviewProof        `json:"proofs"`
	PivotEnrichments *KYCEnrichmentResultsDto `json:"pivot_enrichments"`
	ReviewLevel      *string                  `json:"review_level"`
}

func (c CaseReviewV1) aiCaseReviewDto() {}

func (c CaseReviewV1) GetVersion() string {
	return versionCaseReviewV1
}

func UnmarshalCaseReviewDto(version string, payload io.Reader) (AiCaseReviewDto, error) {
	switch version {
	case versionCaseReviewV1:
		var dto CaseReviewV1
		err := json.NewDecoder(payload).Decode(&dto)
		dto.Version = version

		if dto.Proofs == nil {
			dto.Proofs = []CaseReviewProof{}
		}

		return dto, err
	}

	return nil, errors.New("unsupported version")
}

// Extend the AiCaseReviewDto with the id and reaction fields for our API endpoints
type AiCaseReviewOutputDto struct {
	Id       uuid.UUID `json:"id"`
	Reaction *string   `json:"reaction"`
	Version  string    `json:"version"`

	Review AiCaseReviewDto `json:"review"`
}

type UpdateCaseReviewFeedbackDto struct {
	Reaction *string `json:"reaction"`
}

func (dto UpdateCaseReviewFeedbackDto) Validate() error {
	if dto.Reaction != nil {
		if models.AiCaseReviewReactionFromString(*dto.Reaction) == models.AiCaseReviewReactionUnknown {
			return errors.New("invalid reaction")
		}
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
