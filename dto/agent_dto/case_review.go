package agent_dto

import (
	"encoding/json"
	"errors"
	"io"
)

// ⚠️⚠️⚠️
// If you introduce a new version of the DTO, remember to update the dto_version in the case_review_file_repository.go file
// Be "new version", I mean anything that is a breaking change on the DTO, so adding fields is not a new version.
// ⚠️⚠️⚠️

type AiCaseReviewDto interface {
	aiCaseReviewDto()
}

type CaseReviewV1 struct {
	Ok          bool   `json:"ok"`
	Output      string `json:"output"`
	SanityCheck string `json:"sanity_check"`
	Thought     string `json:"thought"`
	Version     string `json:"version"`
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
