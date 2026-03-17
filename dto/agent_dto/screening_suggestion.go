package agent_dto

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// ⚠️⚠️⚠️
// If you introduce a new version of the DTO, remember to update the parsing in UnmarshalScreeningHitSuggestionDto.
// A "new version" means any breaking change on the DTO — adding fields is not a new version.
// ⚠️⚠️⚠️

const (
	VersionScreeningHitSuggestionV1 = "v1"
)

type AiScreeningHitSuggestionDto interface {
	aiScreeningHitSuggestionDto()
	GetVersion() string
}

// ScreeningHitSuggestionBlob is the versioned envelope stored in blob storage.
type ScreeningHitSuggestionBlob struct {
	Version string          `json:"version"`
	Content json.RawMessage `json:"content"`
}

type ScreeningHitSuggestionV1 struct {
	MatchId    string                        `json:"match_id"`
	EntityId   string                        `json:"entity_id"`
	Confidence models.ScreeningHitConfidence `json:"confidence"`
	Reason     string                        `json:"reason"`
	Version    string                        `json:"version"`
	CreatedAt  time.Time                     `json:"created_at"`
}

func (s ScreeningHitSuggestionV1) aiScreeningHitSuggestionDto() {}

func (s ScreeningHitSuggestionV1) GetVersion() string {
	return VersionScreeningHitSuggestionV1
}

func NewScreeningHitSuggestionBlob(dto AiScreeningHitSuggestionDto) (ScreeningHitSuggestionBlob, error) {
	content, err := json.Marshal(dto)
	if err != nil {
		return ScreeningHitSuggestionBlob{}, err
	}
	return ScreeningHitSuggestionBlob{
		Version: dto.GetVersion(),
		Content: content,
	}, nil
}

func UnmarshalScreeningHitSuggestionDto(version string, payload io.Reader) (AiScreeningHitSuggestionDto, error) {
	switch version {
	case VersionScreeningHitSuggestionV1:
		var dto ScreeningHitSuggestionV1
		err := json.NewDecoder(payload).Decode(&dto)
		return dto, err
	}
	return nil, errors.New("unsupported version")
}
