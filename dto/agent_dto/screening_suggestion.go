package agent_dto

import (
	"bytes"
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

// ScreeningHitSuggestions is a version-aware list of suggestions.
// It marshals natively (each concrete element embeds its "version"), and unmarshals by
// dispatching per element on that "version" field — so decoding never depends on a single
// struct version, and a plain json.Unmarshal into a struct holding this type just works
// (the standard decoder cannot, on its own, unmarshal a JSON object into the interface).
type ScreeningHitSuggestions []AiScreeningHitSuggestionDto

func (s *ScreeningHitSuggestions) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*s = nil
		return nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}
	result := make(ScreeningHitSuggestions, 0, len(raws))
	for _, raw := range raws {
		var probe struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return err
		}
		version := probe.Version
		if version == "" {
			// Tolerate pre-versioning data: the only format ever written is v1.
			version = VersionScreeningHitSuggestionV1
		}
		dto, err := UnmarshalScreeningHitSuggestionDto(version, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		result = append(result, dto)
	}
	*s = result
	return nil
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
