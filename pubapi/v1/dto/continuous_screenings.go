package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type matchPayload struct {
	Match      *bool                      `json:"match,omitempty"`
	Score      *float64                   `json:"score,omitempty"`
	Schema     *string                    `json:"schema,omitempty"`
	Caption    *string                    `json:"caption,omitempty"`
	Datasets   []string                   `json:"datasets,omitempty"`
	Properties map[string]json.RawMessage `json:"properties,omitempty"`
}

func filterMatchPayload(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	var p matchPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return raw
	}
	result, err := json.Marshal(p)
	if err != nil {
		return raw
	}
	return result
}

type ContinuousScreeningMatch struct {
	Id                   uuid.UUID       `json:"id"`
	ListProviderEntityId *string         `json:"list_provider_entity_id"`
	ObjectType           *string         `json:"object_type"`
	ObjectId             *string         `json:"object_id"`
	Status               string          `json:"status"`
	Payload              json.RawMessage `json:"payload"`
	ReviewedBy           *uuid.UUID      `json:"reviewed_by,omitempty"`
	CreatedAt            types.DateTime  `json:"created_at"`
	UpdatedAt            types.DateTime  `json:"updated_at"`
}

func AdaptContinuousScreeningMatch(
	triggerType models.ContinuousScreeningTriggerType,
	m models.ContinuousScreeningMatch,
) ContinuousScreeningMatch {
	dto := ContinuousScreeningMatch{
		Id:         m.Id,
		Status:     m.Status.String(),
		Payload:    filterMatchPayload(m.Payload),
		ReviewedBy: m.ReviewedBy,
		CreatedAt:  types.DateTime(m.CreatedAt),
		UpdatedAt:  types.DateTime(m.UpdatedAt),
	}

	switch triggerType {
	case models.ContinuousScreeningTriggerTypeObjectAdded, models.ContinuousScreeningTriggerTypeObjectUpdated:
		dto.ListProviderEntityId = utils.Ptr(m.OpenSanctionEntityId)
	case models.ContinuousScreeningTriggerTypeDatasetUpdated:
		if m.Metadata != nil {
			dto.ObjectType = utils.Ptr(m.Metadata.ObjectType)
			dto.ObjectId = utils.Ptr(m.Metadata.ObjectId)
		}
	}

	return dto
}

type ContinuousScreening struct {
	Id                                uuid.UUID                  `json:"id"`
	ContinuousScreeningConfigStableId uuid.UUID                  `json:"continuous_screening_config_stable_id"`
	CaseId                            *uuid.UUID                 `json:"case_id"`
	ObjectType                        *string                    `json:"object_type"`
	ObjectId                          *string                    `json:"object_id"`
	ListProviderEntityId              *string                    `json:"list_provider_entity_id"`
	Status                            string                     `json:"status"`
	TriggerType                       string                     `json:"trigger_type"`
	Partial                           bool                       `json:"partial"`
	NumberOfMatches                   int                        `json:"number_of_matches"`
	Matches                           []ContinuousScreeningMatch `json:"matches"`
	CreatedAt                         types.DateTime             `json:"created_at"`
	UpdatedAt                         types.DateTime             `json:"updated_at"`
}

func (ContinuousScreening) ApiVersion() string {
	return "v1beta"
}

func AdaptContinuousScreening(m models.ContinuousScreeningWithMatches) ContinuousScreening {
	return ContinuousScreening{
		Id:                                m.Id,
		ContinuousScreeningConfigStableId: m.ContinuousScreeningConfigStableId,
		CaseId:                            m.CaseId,
		ObjectType:                        m.ObjectType,
		ObjectId:                          m.ObjectId,
		ListProviderEntityId:              m.OpenSanctionEntityId,
		Status:                            m.Status.String(),
		TriggerType:                       m.TriggerType.String(),
		Partial:                           m.IsPartial,
		NumberOfMatches:                   m.NumberOfMatches,
		Matches: pure_utils.Map(m.Matches, func(match models.ContinuousScreeningMatch) ContinuousScreeningMatch {
			return AdaptContinuousScreeningMatch(m.TriggerType, match)
		}),
		CreatedAt: types.DateTime(m.CreatedAt),
		UpdatedAt: types.DateTime(m.UpdatedAt),
	}
}
