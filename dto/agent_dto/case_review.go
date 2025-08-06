package agent_dto

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"

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

// Extend the AiCaseReviewDto with the id and reaction fields for our API endpoints
// Didn't add in aiCaseReviewDto to avoid adding none related fields of AI agent analysis, also aiCaseReviewDto is
// stored in blob storage, so we don't need to add it here.
type AiCaseReviewOutputDto struct {
	Id       uuid.UUID `json:"id"`
	Reaction *string   `json:"reaction"`

	AiCaseReviewDto
}

// Custom MarshalJSON to handle the id and reaction fields and merge them with the aiCaseReviewDto fields
// Avoid having a AiCaseReviewDto field in the output and flatten the fields
func (dto AiCaseReviewOutputDto) MarshalJSON() ([]byte, error) {
	// Use reflection to automatically extract non-interface fields
	result := make(map[string]interface{})

	// Get the value and type of the struct
	val := reflect.ValueOf(dto)
	typ := reflect.TypeOf(dto)

	// Iterate through all fields
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip the embedded AiCaseReviewDto interface field using type assertion
		if _, ok := field.Interface().(AiCaseReviewDto); ok {
			continue
		}

		// Get the JSON tag name, or use the field name if no tag
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Handle comma-separated tags (e.g., "id,omitempty")
		tagName := jsonTag
		if commaIdx := len(jsonTag); commaIdx > 0 {
			for j, char := range jsonTag {
				if char == ',' {
					tagName = jsonTag[:j]
					break
				}
			}
		}

		// Add the field to our result map
		result[tagName] = field.Interface()
	}

	// Marshal the embedded AiCaseReviewDto and merge its fields
	reviewJSON, err := json.Marshal(dto.AiCaseReviewDto)
	if err != nil {
		return nil, err
	}

	var reviewMap map[string]interface{}
	if err := json.Unmarshal(reviewJSON, &reviewMap); err != nil {
		return nil, err
	}

	// Merge the review fields into the result map
	for key, value := range reviewMap {
		result[key] = value
	}

	return json.Marshal(result)
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
